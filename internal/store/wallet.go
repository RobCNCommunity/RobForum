package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

const (
	minWalletTopUpCents int64 = 100
	maxWalletTopUpCents int64 = 500_000
	maxRedeemCodeCents  int64 = 1_000_000
)

func (s *Store) CreateWalletTopUp(userID, amountCents int64) (domain.WalletTopUpOrder, error) {
	if amountCents < minWalletTopUpCents || amountCents > maxWalletTopUpCents {
		return domain.WalletTopUpOrder{}, fmt.Errorf("充值金额需在 %.2f 到 %.2f 元之间", float64(minWalletTopUpCents)/100, float64(maxWalletTopUpCents)/100)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.WalletTopUpOrder{}, err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, []int64{userID}); err != nil {
		return domain.WalletTopUpOrder{}, err
	}
	var existingID int64
	if err := tx.QueryRow(`SELECT id FROM wallet_topup_orders WHERE user_id = ? AND amount_cents = ? AND status = 'pending' AND created_at > ? ORDER BY id DESC LIMIT 1 FOR UPDATE`, userID, amountCents, time.Now().UTC().Add(-20*time.Minute)).Scan(&existingID); err == nil {
		item, err := getWalletTopUpOrder(tx, existingID)
		if err != nil {
			return domain.WalletTopUpOrder{}, err
		}
		if err := tx.Rollback(); err != nil {
			return domain.WalletTopUpOrder{}, err
		}
		return item, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return domain.WalletTopUpOrder{}, err
	}
	orderNo, err := randomWalletTopUpOrderNo()
	if err != nil {
		return domain.WalletTopUpOrder{}, err
	}
	now := time.Now().UTC()
	result, err := tx.Exec(`INSERT INTO wallet_topup_orders (order_no, user_id, amount_cents, status, created_at, updated_at) VALUES (?, ?, ?, 'pending', ?, ?)`, orderNo, userID, amountCents, now, now)
	if err != nil {
		return domain.WalletTopUpOrder{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.WalletTopUpOrder{}, err
	}
	item, err := getWalletTopUpOrder(tx, id)
	if err != nil {
		return domain.WalletTopUpOrder{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.WalletTopUpOrder{}, err
	}
	return item, nil
}

func (s *Store) CancelWalletTopUp(orderID, userID int64) error {
	result, err := s.db.Exec(`UPDATE wallet_topup_orders SET status = 'cancelled', updated_at = ? WHERE id = ? AND user_id = ? AND status = 'pending'`, time.Now().UTC(), orderID, userID)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) CompleteWalletTopUp(orderNo, gatewayTradeNo string, amountCents int64) (domain.WalletTopUpOrder, bool, error) {
	orderNo = strings.TrimSpace(orderNo)
	gatewayTradeNo = strings.TrimSpace(gatewayTradeNo)
	if !strings.HasPrefix(orderNo, "RWT") || gatewayTradeNo == "" || len(orderNo) > 64 || len(gatewayTradeNo) > 255 || amountCents < minWalletTopUpCents || amountCents > maxWalletTopUpCents {
		return domain.WalletTopUpOrder{}, false, errors.New("充值回调参数无效")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.WalletTopUpOrder{}, false, err
	}
	defer tx.Rollback()
	var item domain.WalletTopUpOrder
	var paidAt sql.NullTime
	var tradeNo sql.NullString
	err = tx.QueryRow(`SELECT id, order_no, user_id, amount_cents, status, gateway_trade_no, paid_at, created_at FROM wallet_topup_orders WHERE order_no = ? FOR UPDATE`, orderNo).Scan(&item.ID, &item.OrderNo, &item.UserID, &item.AmountCents, &item.Status, &tradeNo, &paidAt, &item.CreatedAt)
	if err != nil {
		return domain.WalletTopUpOrder{}, false, err
	}
	if tradeNo.Valid {
		item.GatewayTradeNo = tradeNo.String
	}
	if paidAt.Valid {
		item.PaidAt = &paidAt.Time
	}
	if err := lockActiveUsers(tx, []int64{item.UserID}); err != nil {
		return domain.WalletTopUpOrder{}, false, err
	}
	var resourceOrderID int64
	if err := tx.QueryRow(`SELECT id FROM commerce_orders WHERE gateway_trade_no = ? AND gateway_trade_no <> '' LIMIT 1 FOR UPDATE`, gatewayTradeNo).Scan(&resourceOrderID); err == nil {
		return domain.WalletTopUpOrder{}, false, errors.New("网关流水已用于其他订单")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return domain.WalletTopUpOrder{}, false, err
	}
	var otherTopUpID int64
	if err := tx.QueryRow(`SELECT id FROM wallet_topup_orders WHERE gateway_trade_no = ? AND id <> ? LIMIT 1 FOR UPDATE`, gatewayTradeNo, item.ID).Scan(&otherTopUpID); err == nil {
		return domain.WalletTopUpOrder{}, false, errors.New("网关流水已用于其他充值订单")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return domain.WalletTopUpOrder{}, false, err
	}
	if item.Status == "paid" {
		if item.AmountCents != amountCents || item.GatewayTradeNo != gatewayTradeNo {
			return domain.WalletTopUpOrder{}, false, errors.New("已支付充值订单与回调不一致")
		}
		if err := tx.Commit(); err != nil {
			return domain.WalletTopUpOrder{}, false, err
		}
		return item, true, nil
	}
	if item.Status != "pending" || item.AmountCents != amountCents {
		return domain.WalletTopUpOrder{}, false, errors.New("充值订单金额或状态无效")
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`UPDATE wallet_topup_orders SET status = 'paid', gateway_trade_no = ?, paid_at = ?, updated_at = ? WHERE id = ?`, gatewayTradeNo, now, now, item.ID); err != nil {
		return domain.WalletTopUpOrder{}, false, err
	}
	if _, err := tx.Exec(`INSERT INTO wallet_ledgers (user_id, entry_type, amount_cents, reference_type, reference_id, note, created_at) VALUES (?, 'wallet_top_up', ?, 'wallet_topup_order', ?, '钱包充值', ?)`, item.UserID, item.AmountCents, item.ID, now); err != nil {
		return domain.WalletTopUpOrder{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return domain.WalletTopUpOrder{}, false, err
	}
	item.Status = "paid"
	item.GatewayTradeNo = gatewayTradeNo
	item.PaidAt = &now
	return item, false, nil
}

func (s *Store) ListWalletTopUps(userID int64, limit int) ([]domain.WalletTopUpOrder, error) {
	if limit < 1 || limit > 50 {
		limit = 12
	}
	rows, err := s.db.Query(`SELECT id FROM wallet_topup_orders WHERE user_id = ? ORDER BY created_at DESC, id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.WalletTopUpOrder, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		item, err := getWalletTopUpOrder(s.db, id)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func getWalletTopUpOrder(queryer rowQueryer, id int64) (domain.WalletTopUpOrder, error) {
	var item domain.WalletTopUpOrder
	var paidAt sql.NullTime
	var tradeNo sql.NullString
	err := queryer.QueryRow(`SELECT id, order_no, user_id, amount_cents, status, gateway_trade_no, paid_at, created_at FROM wallet_topup_orders WHERE id = ?`, id).Scan(&item.ID, &item.OrderNo, &item.UserID, &item.AmountCents, &item.Status, &tradeNo, &paidAt, &item.CreatedAt)
	if tradeNo.Valid {
		item.GatewayTradeNo = tradeNo.String
	}
	if paidAt.Valid {
		item.PaidAt = &paidAt.Time
	}
	return item, err
}

func (s *Store) WalletBalance(userID int64) (int64, error) {
	return walletBalance(s.db, userID)
}

func walletBalance(queryer rowQueryer, userID int64) (int64, error) {
	var balance, reserved int64
	if err := queryer.QueryRow(`SELECT COALESCE(SUM(amount_cents), 0) FROM wallet_ledgers WHERE user_id = ?`, userID).Scan(&balance); err != nil {
		return 0, err
	}
	if err := queryer.QueryRow(`SELECT COALESCE(SUM(amount_cents), 0) FROM creator_payouts WHERE creator_id = ? AND status IN ('pending', 'paid')`, userID).Scan(&reserved); err != nil {
		return 0, err
	}
	balance -= reserved
	if balance < 0 {
		return 0, nil
	}
	return balance, nil
}

func (s *Store) RedeemWalletCode(userID int64, rawCode string) (domain.RedeemCode, error) {
	code := normalizeRedeemCode(rawCode)
	if len(code) < 8 || len(code) > 64 || containsControl(code) {
		return domain.RedeemCode{}, errors.New("兑换码格式无效")
	}
	hash := sha256.Sum256([]byte(code))
	tx, err := s.db.Begin()
	if err != nil {
		return domain.RedeemCode{}, err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, []int64{userID}); err != nil {
		return domain.RedeemCode{}, err
	}
	item, err := getRedeemCodeByHash(tx, hash[:], true)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.RedeemCode{}, errors.New("兑换码不存在或已失效")
		}
		return domain.RedeemCode{}, err
	}
	if item.RedeemedAt != nil || item.RevokedAt != nil || (item.ExpiresAt != nil && !item.ExpiresAt.After(time.Now().UTC())) {
		return domain.RedeemCode{}, errors.New("兑换码不存在或已失效")
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`UPDATE redeem_codes SET redeemed_by = ?, redeemed_at = ? WHERE id = ? AND redeemed_at IS NULL AND revoked_at IS NULL`, userID, now, item.ID); err != nil {
		return domain.RedeemCode{}, err
	}
	if _, err := tx.Exec(`INSERT INTO wallet_ledgers (user_id, entry_type, amount_cents, reference_type, reference_id, note, created_at) VALUES (?, 'redeem_code', ?, 'redeem_code', ?, '兑换码到账', ?)`, userID, item.AmountCents, item.ID, now); err != nil {
		return domain.RedeemCode{}, err
	}
	item.RedeemedBy = &userID
	item.RedeemedAt = &now
	item.Status = "redeemed"
	if err := tx.Commit(); err != nil {
		return domain.RedeemCode{}, err
	}
	return item, nil
}

func (s *Store) CreateRedeemCode(actorID, amountCents int64, expiresAt *time.Time) (domain.RedeemCode, error) {
	if amountCents < 1 || amountCents > maxRedeemCodeCents {
		return domain.RedeemCode{}, errors.New("兑换码金额无效")
	}
	if expiresAt != nil {
		expires := expiresAt.UTC()
		if !expires.After(time.Now().UTC()) || expires.After(time.Now().UTC().Add(366*24*time.Hour)) {
			return domain.RedeemCode{}, errors.New("兑换码有效期无效")
		}
		expiresAt = &expires
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.RedeemCode{}, err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, []int64{actorID}); err != nil {
		return domain.RedeemCode{}, err
	}
	var rawCode string
	var hash [32]byte
	for attempt := 0; attempt < 4; attempt++ {
		rawCode, err = randomRedeemCode()
		if err != nil {
			return domain.RedeemCode{}, err
		}
		hash = sha256.Sum256([]byte(rawCode))
		var existing int64
		err = tx.QueryRow(`SELECT id FROM redeem_codes WHERE code_hash = ? FOR UPDATE`, hash[:]).Scan(&existing)
		if errors.Is(err, sql.ErrNoRows) {
			break
		}
		if err != nil {
			return domain.RedeemCode{}, err
		}
		if attempt == 3 {
			return domain.RedeemCode{}, errors.New("兑换码生成冲突，请重试")
		}
	}
	now := time.Now().UTC()
	codeHint := codeHint(rawCode)
	result, err := tx.Exec(`INSERT INTO redeem_codes (code_hash, code_hint, amount_cents, expires_at, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?)`, hash[:], codeHint, amountCents, expiresAt, actorID, now)
	if err != nil {
		return domain.RedeemCode{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.RedeemCode{}, err
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'redeem_code', ?, 'create', ?, ?)`, actorID, id, fmt.Sprintf("%d", amountCents), now); err != nil {
		return domain.RedeemCode{}, err
	}
	item, err := getRedeemCode(tx, id, false)
	if err != nil {
		return domain.RedeemCode{}, err
	}
	item.Code = rawCode
	if err := tx.Commit(); err != nil {
		return domain.RedeemCode{}, err
	}
	return item, nil
}

func (s *Store) ListRedeemCodes(limit int) ([]domain.RedeemCode, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id FROM redeem_codes ORDER BY created_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.RedeemCode, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		item, err := getRedeemCode(s.db, id, false)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) RevokeRedeemCode(actorID, codeID int64) (domain.RedeemCode, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.RedeemCode{}, err
	}
	defer tx.Rollback()
	item, err := getRedeemCode(tx, codeID, true)
	if err != nil {
		return domain.RedeemCode{}, err
	}
	if item.RedeemedAt != nil {
		return domain.RedeemCode{}, errors.New("已兑换的兑换码不能停用")
	}
	if item.RevokedAt != nil {
		return domain.RedeemCode{}, errors.New("兑换码已停用")
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`UPDATE redeem_codes SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`, now, codeID); err != nil {
		return domain.RedeemCode{}, err
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'redeem_code', ?, 'revoke', '', ?)`, actorID, codeID, now); err != nil {
		return domain.RedeemCode{}, err
	}
	item.RevokedAt = &now
	item.Status = "revoked"
	if err := tx.Commit(); err != nil {
		return domain.RedeemCode{}, err
	}
	return item, nil
}

func getRedeemCode(queryer rowQueryer, id int64, forUpdate bool) (domain.RedeemCode, error) {
	query := `SELECT r.id, r.code_hint, r.amount_cents, r.expires_at, r.redeemed_by, COALESCE(u.display_name, ''), r.redeemed_at, r.revoked_at, r.created_by, r.created_at FROM redeem_codes r LEFT JOIN users u ON u.id = r.redeemed_by WHERE r.id = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	return scanRedeemCode(queryer.QueryRow(query, id))
}

func getRedeemCodeByHash(queryer rowQueryer, hash []byte, forUpdate bool) (domain.RedeemCode, error) {
	query := `SELECT r.id, r.code_hint, r.amount_cents, r.expires_at, r.redeemed_by, COALESCE(u.display_name, ''), r.redeemed_at, r.revoked_at, r.created_by, r.created_at FROM redeem_codes r LEFT JOIN users u ON u.id = r.redeemed_by WHERE r.code_hash = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	return scanRedeemCode(queryer.QueryRow(query, hash))
}

type redeemCodeScanner interface {
	Scan(dest ...any) error
}

func scanRedeemCode(scanner redeemCodeScanner) (domain.RedeemCode, error) {
	var item domain.RedeemCode
	var expiresAt, redeemedAt, revokedAt sql.NullTime
	var redeemedBy sql.NullInt64
	err := scanner.Scan(&item.ID, &item.CodeHint, &item.AmountCents, &expiresAt, &redeemedBy, &item.RedeemedByName, &redeemedAt, &revokedAt, &item.CreatedBy, &item.CreatedAt)
	if err != nil {
		return item, err
	}
	if expiresAt.Valid {
		item.ExpiresAt = &expiresAt.Time
	}
	if redeemedBy.Valid {
		item.RedeemedBy = &redeemedBy.Int64
	}
	if redeemedAt.Valid {
		item.RedeemedAt = &redeemedAt.Time
	}
	if revokedAt.Valid {
		item.RevokedAt = &revokedAt.Time
	}
	item.Status = redeemCodeStatus(item)
	return item, nil
}

func redeemCodeStatus(item domain.RedeemCode) string {
	if item.RedeemedAt != nil {
		return "redeemed"
	}
	if item.RevokedAt != nil {
		return "revoked"
	}
	if item.ExpiresAt != nil && !item.ExpiresAt.After(time.Now().UTC()) {
		return "expired"
	}
	return "active"
}

func (s *Store) CreateWalletResourceOrder(userID, resourceID int64) (domain.CommerceOrder, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.CommerceOrder{}, err
	}
	defer tx.Rollback()
	var discoveredCreatorID int64
	if err := tx.QueryRow(`SELECT creator_id FROM resources WHERE id = ?`, resourceID).Scan(&discoveredCreatorID); err != nil {
		return domain.CommerceOrder{}, errors.New("资源不可购买")
	}
	if err := lockActiveUsers(tx, []int64{userID, discoveredCreatorID}); err != nil {
		return domain.CommerceOrder{}, errors.New("购买者或创作者不可用")
	}
	var creatorID, priceCents int64
	var resourceStatus string
	if err := tx.QueryRow(`SELECT creator_id, price_cents, status FROM resources WHERE id = ? FOR UPDATE`, resourceID).Scan(&creatorID, &priceCents, &resourceStatus); err != nil {
		return domain.CommerceOrder{}, err
	}
	if resourceStatus != "approved" || creatorID != discoveredCreatorID || priceCents <= 0 || priceCents > maxResourcePriceCents {
		return domain.CommerceOrder{}, errors.New("资源不可购买")
	}
	if creatorID == userID {
		return domain.CommerceOrder{}, errors.New("不能购买自己的资源")
	}
	feePolicy, err := feePolicyWithQuery(tx, creatorID, true)
	if err != nil {
		return domain.CommerceOrder{}, err
	}
	serviceFeeCents := feeAmountCents(priceCents, feePolicy.ServiceFeeBPS)
	creatorShare := priceCents - serviceFeeCents
	var fileID int64
	if err := tx.QueryRow(`SELECT id FROM resource_files WHERE resource_id = ? FOR UPDATE`, resourceID).Scan(&fileID); err != nil {
		return domain.CommerceOrder{}, errors.New("资源文件不可用")
	}
	if purchasedOrderID, err := resourcePurchaseOrderForUpdate(tx, userID, resourceID); err == nil {
		item, err := getCommerceOrder(tx, purchasedOrderID)
		if err != nil {
			return domain.CommerceOrder{}, err
		}
		if err := tx.Rollback(); err != nil {
			return domain.CommerceOrder{}, err
		}
		return item, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return domain.CommerceOrder{}, err
	}
	available, err := walletBalance(tx, userID)
	if err != nil {
		return domain.CommerceOrder{}, err
	}
	if available < priceCents {
		return domain.CommerceOrder{}, errors.New("钱包余额不足")
	}
	var orderID int64
	if err := tx.QueryRow(`SELECT id FROM commerce_orders WHERE user_id = ? AND resource_id = ? AND status = 'pending' ORDER BY id DESC LIMIT 1 FOR UPDATE`, userID, resourceID).Scan(&orderID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return domain.CommerceOrder{}, err
	}
	now := time.Now().UTC()
	if orderID == 0 {
		orderNo, err := randomOrderNo()
		if err != nil {
			return domain.CommerceOrder{}, err
		}
		result, err := tx.Exec(`INSERT INTO commerce_orders (order_no, user_id, resource_id, amount_cents, service_fee_bps, service_fee_cents, creator_share_cents, seller_membership_tier_id, seller_membership_tier_name, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?)`, orderNo, userID, resourceID, priceCents, feePolicy.ServiceFeeBPS, serviceFeeCents, creatorShare, nullablePositiveID(feePolicy.MembershipTierID), feePolicy.MembershipTierName, now, now)
		if err != nil {
			return domain.CommerceOrder{}, err
		}
		orderID, err = result.LastInsertId()
		if err != nil {
			return domain.CommerceOrder{}, err
		}
	} else if _, err := tx.Exec(`UPDATE commerce_orders SET service_fee_bps = ?, service_fee_cents = ?, creator_share_cents = ?, seller_membership_tier_id = ?, seller_membership_tier_name = ?, updated_at = ? WHERE id = ? AND service_fee_bps = 0 AND service_fee_cents = 0 AND creator_share_cents = 0`, feePolicy.ServiceFeeBPS, serviceFeeCents, creatorShare, nullablePositiveID(feePolicy.MembershipTierID), feePolicy.MembershipTierName, now, orderID); err != nil {
		return domain.CommerceOrder{}, err
	}
	order, err := getCommerceOrder(tx, orderID)
	if err != nil {
		return domain.CommerceOrder{}, err
	}
	if order.AmountCents != priceCents || order.Status != "pending" {
		return domain.CommerceOrder{}, errors.New("订单状态已变化，请重试")
	}
	if _, err := tx.Exec(`UPDATE commerce_orders SET status = 'paid', gateway_trade_no = ?, paid_at = ?, updated_at = ? WHERE id = ? AND status = 'pending'`, "wallet:"+order.OrderNo, now, now, order.ID); err != nil {
		return domain.CommerceOrder{}, err
	}
	if err := createResourcePurchase(tx, userID, resourceID, order.ID, now); err != nil {
		return domain.CommerceOrder{}, err
	}
	if _, err := tx.Exec(`INSERT INTO wallet_ledgers (user_id, entry_type, amount_cents, reference_type, reference_id, note, created_at) VALUES (?, 'resource_purchase', ?, 'commerce_order', ?, ?, ?)`, userID, -priceCents, order.ID, "资源余额支付", now); err != nil {
		return domain.CommerceOrder{}, err
	}
	if _, err := tx.Exec(`UPDATE resources SET sales_count = sales_count + 1 WHERE id = ?`, resourceID); err != nil {
		return domain.CommerceOrder{}, err
	}
	if _, err := tx.Exec(`INSERT INTO wallet_ledgers (user_id, entry_type, amount_cents, reference_type, reference_id, note, created_at) VALUES (?, 'resource_sale', ?, 'commerce_order', ?, ?, ?)`, creatorID, creatorShare, order.ID, fmt.Sprintf("资源销售收入，服务费 %d 分（%.2f%%）", serviceFeeCents, float64(feePolicy.ServiceFeeBPS)/100), now); err != nil {
		return domain.CommerceOrder{}, err
	}
	if err := cancelOtherPendingResourceOrders(tx, userID, resourceID, order.ID, now); err != nil {
		return domain.CommerceOrder{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.CommerceOrder{}, err
	}
	order.Status = "paid"
	order.GatewayTradeNo = "wallet:" + order.OrderNo
	order.PaidAt = &now
	return order, nil
}

func randomWalletTopUpOrderNo() (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "RWT" + time.Now().UTC().Format("20060102150405") + strings.ToUpper(hex.EncodeToString(raw)), nil
}

func randomRedeemCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	parts := make([]string, 4)
	for group := range parts {
		chunk := make([]byte, 4)
		for index := range chunk {
			chunk[index] = alphabet[int(buf[group*4+index])%len(alphabet)]
		}
		parts[group] = string(chunk)
	}
	return "RBF-" + strings.Join(parts, "-"), nil
}

func normalizeRedeemCode(value string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
}

func codeHint(code string) string {
	if len(code) < 4 {
		return "****"
	}
	return "RBF-****-****-****-" + code[len(code)-4:]
}
