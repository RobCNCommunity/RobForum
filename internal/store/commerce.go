package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"roblox-community/internal/auth"
	"roblox-community/internal/domain"
)

func (s *Store) GetPaymentConfig() (domain.PaymentConfig, error)      { return s.paymentConfig(false) }
func (s *Store) PaymentDeliveryConfig() (domain.PaymentConfig, error) { return s.paymentConfig(true) }

func (s *Store) paymentConfig(includeSecret bool) (domain.PaymentConfig, error) {
	return paymentConfigWithQuery(s.db, includeSecret, false, s.masterKey)
}

func paymentConfigWithQuery(queryer rowQueryer, includeSecret, forUpdate bool, masterKey string) (domain.PaymentConfig, error) {
	var config domain.PaymentConfig
	var enabled int
	var secretCiphertext string
	query := `SELECT enabled, kind, gateway_url, merchant_id, secret_ciphertext, pay_type, notify_url, return_url FROM payment_settings WHERE id = 1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	err := queryer.QueryRow(query).Scan(&enabled, &config.Kind, &config.GatewayURL, &config.MerchantID, &secretCiphertext, &config.PayType, &config.NotifyURL, &config.ReturnURL)
	if err != nil {
		return config, err
	}
	config.Enabled = enabled != 0
	config.HasSecret = secretCiphertext != ""
	if secretCiphertext != "" {
		secret, err := auth.Decrypt(masterKey, secretCiphertext)
		if err != nil {
			return config, err
		}
		config.MaskedSecret = auth.Mask(secret)
		if includeSecret {
			config.Secret = secret
		}
	}
	return config, nil
}

func (s *Store) UpdatePaymentConfig(actorID int64, input domain.PaymentConfig) (domain.PaymentConfig, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.PaymentConfig{}, err
	}
	defer tx.Rollback()
	current, err := paymentConfigWithQuery(tx, true, true, s.masterKey)
	if err != nil {
		return domain.PaymentConfig{}, err
	}
	input.Kind = strings.ToLower(strings.TrimSpace(input.Kind))
	input.GatewayURL = strings.TrimRight(strings.TrimSpace(input.GatewayURL), "/")
	input.MerchantID = strings.TrimSpace(input.MerchantID)
	input.PayType = strings.ToLower(strings.TrimSpace(input.PayType))
	input.NotifyURL = strings.TrimSpace(input.NotifyURL)
	input.ReturnURL = strings.TrimSpace(input.ReturnURL)
	if input.Kind == "" {
		input.Kind = "epay"
	}
	if input.Kind != "epay" {
		return domain.PaymentConfig{}, errors.New("only EPay-compatible gateways are currently supported")
	}
	if input.PayType == "" {
		input.PayType = "alipay"
	}
	if input.PayType != "alipay" && input.PayType != "wxpay" && input.PayType != "qqpay" && input.PayType != "bank" {
		return domain.PaymentConfig{}, errors.New("unsupported payment type")
	}
	if len(input.GatewayURL) > 500 || len(input.MerchantID) > 255 || len(input.NotifyURL) > 500 || len(input.ReturnURL) > 500 || containsControl(input.MerchantID) {
		return domain.PaymentConfig{}, errors.New("payment configuration is too long or invalid")
	}
	for _, candidate := range []string{input.GatewayURL, input.NotifyURL, input.ReturnURL} {
		if candidate == "" {
			continue
		}
		if err := validateWebURL(candidate, false); err != nil {
			return domain.PaymentConfig{}, errors.New("payment URL is invalid")
		}
	}
	if input.Enabled {
		for _, candidate := range []string{input.GatewayURL, input.NotifyURL, input.ReturnURL} {
			if err := validateSecureWebURL(candidate); err != nil {
				return domain.PaymentConfig{}, errors.New("enabled payment URLs must use HTTPS")
			}
		}
	}
	if parsedGateway, err := url.Parse(input.GatewayURL); err == nil && (parsedGateway.RawQuery != "" || parsedGateway.Fragment != "") {
		return domain.PaymentConfig{}, errors.New("payment gateway URL must not contain query parameters or fragments")
	}
	secret := current.Secret
	if strings.TrimSpace(input.Secret) != "" && !strings.HasPrefix(input.Secret, "••••") {
		secret = strings.TrimSpace(input.Secret)
	}
	if input.Enabled && (input.GatewayURL == "" || input.MerchantID == "" || secret == "") {
		return domain.PaymentConfig{}, errors.New("enabled payment gateway requires URL, merchant ID, and secret")
	}
	if input.Enabled && len([]byte(secret)) < 12 {
		return domain.PaymentConfig{}, errors.New("payment secret must contain at least 12 bytes")
	}
	if len([]byte(secret)) > 4096 {
		return domain.PaymentConfig{}, errors.New("payment secret is too long")
	}
	ciphertext := ""
	if secret != "" {
		ciphertext, err = auth.Encrypt(s.masterKey, secret)
		if err != nil {
			return domain.PaymentConfig{}, err
		}
	}
	now := time.Now().UTC()
	result, err := tx.Exec(`UPDATE payment_settings SET enabled = ?, kind = ?, gateway_url = ?, merchant_id = ?, secret_ciphertext = ?, pay_type = ?, notify_url = ?, return_url = ?, updated_at = ? WHERE id = 1`, boolInt(input.Enabled), input.Kind, input.GatewayURL, input.MerchantID, ciphertext, input.PayType, input.NotifyURL, input.ReturnURL, now)
	if err != nil {
		return domain.PaymentConfig{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return domain.PaymentConfig{}, err
		}
		return domain.PaymentConfig{}, errors.New("payment settings row is missing")
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'payment_settings', 1, 'update', '', ?)`, actorID, now); err != nil {
		return domain.PaymentConfig{}, err
	}
	config, err := paymentConfigWithQuery(tx, false, false, s.masterKey)
	if err != nil {
		return domain.PaymentConfig{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.PaymentConfig{}, err
	}
	return config, nil
}

func (s *Store) CreateResourceOrder(userID, resourceID int64) (domain.CommerceOrder, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.CommerceOrder{}, err
	}
	defer tx.Rollback()
	var discoveredCreatorID int64
	if err := tx.QueryRow(`SELECT creator_id FROM resources WHERE id = ?`, resourceID).Scan(&discoveredCreatorID); err != nil {
		return domain.CommerceOrder{}, errors.New("resource is not available")
	}
	if err := lockActiveUsers(tx, []int64{userID, discoveredCreatorID}); err != nil {
		return domain.CommerceOrder{}, errors.New("user or resource creator is not available")
	}
	var creatorID, priceCents int64
	var resourceStatus string
	if err := tx.QueryRow(`SELECT creator_id, price_cents, status FROM resources WHERE id = ? FOR UPDATE`, resourceID).Scan(&creatorID, &priceCents, &resourceStatus); err != nil {
		return domain.CommerceOrder{}, err
	}
	if resourceStatus != "approved" {
		return domain.CommerceOrder{}, errors.New("resource is not available")
	}
	if creatorID != discoveredCreatorID {
		return domain.CommerceOrder{}, errors.New("resource creator changed during order creation")
	}
	if priceCents <= 0 || priceCents > maxResourcePriceCents {
		return domain.CommerceOrder{}, errors.New("resource price is invalid")
	}
	if creatorID == userID {
		return domain.CommerceOrder{}, errors.New("creators cannot buy their own resources")
	}
	var resourceFileID int64
	if err := tx.QueryRow(`SELECT id FROM resource_files WHERE resource_id = ? FOR UPDATE`, resourceID).Scan(&resourceFileID); err != nil {
		return domain.CommerceOrder{}, errors.New("resource file is not available")
	}
	var existingID int64
	if err := tx.QueryRow(`SELECT id FROM commerce_orders WHERE user_id = ? AND resource_id = ? AND status = 'paid' ORDER BY id DESC LIMIT 1`, userID, resourceID).Scan(&existingID); err == nil {
		item, err := getCommerceOrder(tx, existingID)
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
	if err := tx.QueryRow(`SELECT id FROM commerce_orders WHERE user_id = ? AND resource_id = ? AND status = 'pending' AND created_at > ? ORDER BY id DESC LIMIT 1`, userID, resourceID, time.Now().UTC().Add(-30*time.Minute)).Scan(&existingID); err == nil {
		item, err := getCommerceOrder(tx, existingID)
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
	orderNo, err := randomOrderNo()
	if err != nil {
		return domain.CommerceOrder{}, err
	}
	now := time.Now().UTC()
	result, err := tx.Exec(`INSERT INTO commerce_orders (order_no, user_id, resource_id, amount_cents, status, created_at, updated_at) VALUES (?, ?, ?, ?, 'pending', ?, ?)`, orderNo, userID, resourceID, priceCents, now, now)
	if err != nil {
		return domain.CommerceOrder{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.CommerceOrder{}, err
	}
	item, err := getCommerceOrder(tx, id)
	if err != nil {
		return domain.CommerceOrder{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.CommerceOrder{}, err
	}
	return item, nil
}

func (s *Store) CancelPendingOrder(orderID, userID int64) error {
	result, err := s.db.Exec(`UPDATE commerce_orders SET status = 'cancelled', updated_at = ? WHERE id = ? AND user_id = ? AND status = 'pending'`, time.Now().UTC(), orderID, userID)
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

func (s *Store) GetCommerceOrder(id int64) (domain.CommerceOrder, error) {
	return getCommerceOrder(s.db, id)
}

func getCommerceOrder(queryer rowQueryer, id int64) (domain.CommerceOrder, error) {
	var item domain.CommerceOrder
	var paidAt sql.NullTime
	err := queryer.QueryRow(`SELECT o.id, o.order_no, o.user_id, o.resource_id, r.title, o.amount_cents, o.status, o.gateway_trade_no, o.paid_at, o.created_at FROM commerce_orders o JOIN resources r ON r.id = o.resource_id WHERE o.id = ?`, id).Scan(&item.ID, &item.OrderNo, &item.UserID, &item.ResourceID, &item.ResourceTitle, &item.AmountCents, &item.Status, &item.GatewayTradeNo, &paidAt, &item.CreatedAt)
	if paidAt.Valid {
		item.PaidAt = &paidAt.Time
	}
	return item, err
}

func (s *Store) ListCommerceOrders(userID int64) ([]domain.CommerceOrder, error) {
	rows, err := s.db.Query(`SELECT o.id, o.order_no, o.user_id, o.resource_id, r.title, o.amount_cents, o.status, o.gateway_trade_no, o.paid_at, o.created_at FROM commerce_orders o JOIN resources r ON r.id = o.resource_id WHERE o.user_id = ? ORDER BY o.created_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.CommerceOrder, 0)
	for rows.Next() {
		var item domain.CommerceOrder
		var paidAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.OrderNo, &item.UserID, &item.ResourceID, &item.ResourceTitle, &item.AmountCents, &item.Status, &item.GatewayTradeNo, &paidAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		if paidAt.Valid {
			item.PaidAt = &paidAt.Time
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) CompleteResourceOrder(orderNo, gatewayTradeNo string, amountCents int64) (domain.CommerceOrder, bool, error) {
	orderNo = strings.TrimSpace(orderNo)
	gatewayTradeNo = strings.TrimSpace(gatewayTradeNo)
	if orderNo == "" || gatewayTradeNo == "" || len(orderNo) > 64 || len(gatewayTradeNo) > 255 || amountCents <= 0 || amountCents > maxResourcePriceCents {
		return domain.CommerceOrder{}, false, errors.New("payment callback is incomplete")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.CommerceOrder{}, false, err
	}
	var order domain.CommerceOrder
	var creatorID int64
	var paidAt sql.NullTime
	defer tx.Rollback()
	err = tx.QueryRow(`SELECT o.id, o.order_no, o.user_id, o.resource_id, r.title, o.amount_cents, o.status, o.gateway_trade_no, o.paid_at, o.created_at, r.creator_id FROM commerce_orders o JOIN resources r ON r.id = o.resource_id WHERE o.order_no = ? FOR UPDATE`, orderNo).Scan(&order.ID, &order.OrderNo, &order.UserID, &order.ResourceID, &order.ResourceTitle, &order.AmountCents, &order.Status, &order.GatewayTradeNo, &paidAt, &order.CreatedAt, &creatorID)
	if err != nil {
		return domain.CommerceOrder{}, false, err
	}
	var duplicateOrderID int64
	duplicateErr := tx.QueryRow(`SELECT id FROM commerce_orders WHERE gateway_trade_no = ? AND gateway_trade_no <> '' AND id <> ? LIMIT 1 FOR UPDATE`, gatewayTradeNo, order.ID).Scan(&duplicateOrderID)
	if duplicateErr == nil {
		return domain.CommerceOrder{}, false, errors.New("gateway transaction is already assigned to another order")
	}
	if duplicateErr != nil && !errors.Is(duplicateErr, sql.ErrNoRows) {
		return domain.CommerceOrder{}, false, duplicateErr
	}
	if paidAt.Valid {
		order.PaidAt = &paidAt.Time
	}
	if order.Status == "paid" {
		if amountCents != order.AmountCents || order.GatewayTradeNo != gatewayTradeNo {
			return domain.CommerceOrder{}, false, errors.New("paid order callback does not match the original transaction")
		}
		if err := claimPaymentTransaction(tx, order.ID, gatewayTradeNo, amountCents); err != nil {
			return domain.CommerceOrder{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return domain.CommerceOrder{}, false, err
		}
		return order, true, nil
	}
	if order.Status != "pending" || amountCents != order.AmountCents {
		return domain.CommerceOrder{}, false, errors.New("order amount or status is invalid")
	}
	if err := claimPaymentTransaction(tx, order.ID, gatewayTradeNo, amountCents); err != nil {
		return domain.CommerceOrder{}, false, err
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`UPDATE commerce_orders SET status = 'paid', gateway_trade_no = ?, paid_at = ?, updated_at = ? WHERE id = ?`, gatewayTradeNo, now, now, order.ID); err != nil {
		return domain.CommerceOrder{}, false, err
	}
	if _, err := tx.Exec(`UPDATE resources SET sales_count = sales_count + 1 WHERE id = ?`, order.ResourceID); err != nil {
		return domain.CommerceOrder{}, false, err
	}
	creatorShare := order.AmountCents/100*90 + order.AmountCents%100*90/100
	if _, err := tx.Exec(`INSERT INTO wallet_ledgers (user_id, entry_type, amount_cents, reference_type, reference_id, note, created_at) VALUES (?, 'resource_sale', ?, 'commerce_order', ?, ?, ?)`, creatorID, creatorShare, order.ID, fmt.Sprintf("资源销售收入，平台抽成 %d 分", order.AmountCents-creatorShare), now); err != nil {
		return domain.CommerceOrder{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return domain.CommerceOrder{}, false, err
	}
	order.Status = "paid"
	order.GatewayTradeNo = gatewayTradeNo
	order.PaidAt = &now
	return order, false, nil
}

func claimPaymentTransaction(tx *sql.Tx, orderID int64, gatewayTradeNo string, amountCents int64) error {
	var existingOrderID, existingAmount int64
	err := tx.QueryRow(`SELECT order_id, amount_cents FROM payment_transactions WHERE gateway_trade_no = ? FOR UPDATE`, gatewayTradeNo).Scan(&existingOrderID, &existingAmount)
	if err == nil {
		if existingOrderID != orderID || existingAmount != amountCents {
			return errors.New("gateway transaction is already assigned to another order")
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var existingTradeNo string
	err = tx.QueryRow(`SELECT gateway_trade_no FROM payment_transactions WHERE order_id = ? FOR UPDATE`, orderID).Scan(&existingTradeNo)
	if err == nil {
		if existingTradeNo != gatewayTradeNo {
			return errors.New("order is already assigned to another gateway transaction")
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO payment_transactions (gateway_trade_no, order_id, amount_cents, created_at) VALUES (?, ?, ?, ?)`, gatewayTradeNo, orderID, amountCents, time.Now().UTC()); err != nil {
		return fmt.Errorf("payment transaction claim failed: %w", err)
	}
	return nil
}

func (s *Store) HasPurchasedResource(userID, resourceID int64) (bool, error) {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM commerce_orders WHERE user_id = ? AND resource_id = ? AND status = 'paid'`, userID, resourceID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) CreatorBalance(userID int64) (int64, error) {
	var earned sql.NullInt64
	if err := s.db.QueryRow(`SELECT COALESCE(SUM(amount_cents), 0) FROM wallet_ledgers WHERE user_id = ?`, userID).Scan(&earned); err != nil {
		return 0, err
	}
	var reserved sql.NullInt64
	if err := s.db.QueryRow(`SELECT COALESCE(SUM(amount_cents), 0) FROM creator_payouts WHERE creator_id = ? AND status IN ('pending', 'paid')`, userID).Scan(&reserved); err != nil {
		return 0, err
	}
	available := earned.Int64 - reserved.Int64
	if available < 0 {
		available = 0
	}
	return available, nil
}

func (s *Store) CreateCreatorPayout(userID, amountCents int64, method, account, accountName, note string) (domain.CreatorPayout, error) {
	method = strings.ToLower(strings.TrimSpace(method))
	account = strings.TrimSpace(account)
	accountName = strings.TrimSpace(accountName)
	note = strings.TrimSpace(note)
	if amountCents < 1000 {
		return domain.CreatorPayout{}, errors.New("minimum payout is 1000 cents")
	}
	if method != "alipay" && method != "bank" && method != "paypal" && method != "other" {
		return domain.CreatorPayout{}, errors.New("unsupported payout method")
	}
	if account == "" || len([]rune(account)) > 255 || accountName == "" || len([]rune(accountName)) > 120 || len([]rune(note)) > 500 || containsControl(account+accountName+note) {
		return domain.CreatorPayout{}, errors.New("payout account information is invalid")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.CreatorPayout{}, err
	}
	defer tx.Rollback()
	var lockedID int64
	if err := tx.QueryRow(`SELECT id FROM users WHERE id = ? AND status = 'active' FOR UPDATE`, userID).Scan(&lockedID); err != nil {
		return domain.CreatorPayout{}, err
	}
	var earned, reserved int64
	if err := tx.QueryRow(`SELECT COALESCE(SUM(amount_cents), 0) FROM wallet_ledgers WHERE user_id = ?`, userID).Scan(&earned); err != nil {
		return domain.CreatorPayout{}, err
	}
	if err := tx.QueryRow(`SELECT COALESCE(SUM(amount_cents), 0) FROM creator_payouts WHERE creator_id = ? AND status IN ('pending', 'paid')`, userID).Scan(&reserved); err != nil {
		return domain.CreatorPayout{}, err
	}
	if amountCents > earned-reserved {
		return domain.CreatorPayout{}, errors.New("insufficient creator balance")
	}
	now := time.Now().UTC()
	result, err := tx.Exec(`INSERT INTO creator_payouts (creator_id, amount_cents, status, payout_method, payout_account, account_name, note, review_note, created_at, updated_at) VALUES (?, ?, 'pending', ?, ?, ?, ?, '', ?, ?)`, userID, amountCents, method, account, accountName, note, now, now)
	if err != nil {
		return domain.CreatorPayout{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.CreatorPayout{}, err
	}
	item, err := getCreatorPayoutWithQuery(tx, `WHERE p.id = ?`, id)
	if err != nil {
		return domain.CreatorPayout{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.CreatorPayout{}, err
	}
	return item, nil
}

func (s *Store) GetCreatorPayout(id int64) (domain.CreatorPayout, error) {
	return s.getCreatorPayout(`WHERE p.id = ?`, id)
}

func (s *Store) ListCreatorPayouts(creatorID int64) ([]domain.CreatorPayout, error) {
	return s.listCreatorPayouts(`WHERE p.creator_id = ?`, creatorID)
}

func (s *Store) ListAdminCreatorPayouts(status string) ([]domain.CreatorPayout, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "pending" && status != "paid" && status != "rejected" && status != "" {
		return nil, errors.New("invalid payout status")
	}
	if status == "" {
		return s.listCreatorPayouts("")
	}
	return s.listCreatorPayouts(`WHERE p.status = ?`, status)
}

func (s *Store) listCreatorPayouts(where string, args ...any) ([]domain.CreatorPayout, error) {
	query := `SELECT p.id, p.creator_id, u.display_name, u.email, p.amount_cents, p.status, p.payout_method, p.payout_account, p.account_name, p.note, p.review_note, p.reviewed_by, p.reviewed_at, p.created_at, p.updated_at FROM creator_payouts p JOIN users u ON u.id = p.creator_id ` + where + ` ORDER BY p.created_at DESC LIMIT 100`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.CreatorPayout, 0)
	for rows.Next() {
		item, err := scanCreatorPayout(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) getCreatorPayout(where string, args ...any) (domain.CreatorPayout, error) {
	return getCreatorPayoutWithQuery(s.db, where, args...)
}

func getCreatorPayoutWithQuery(queryer rowQueryer, where string, args ...any) (domain.CreatorPayout, error) {
	row := queryer.QueryRow(`SELECT p.id, p.creator_id, u.display_name, u.email, p.amount_cents, p.status, p.payout_method, p.payout_account, p.account_name, p.note, p.review_note, p.reviewed_by, p.reviewed_at, p.created_at, p.updated_at FROM creator_payouts p JOIN users u ON u.id = p.creator_id `+where, args...)
	return scanCreatorPayout(row)
}

type creatorPayoutScanner interface{ Scan(...any) error }

func scanCreatorPayout(scanner creatorPayoutScanner) (domain.CreatorPayout, error) {
	var item domain.CreatorPayout
	var reviewedBy sql.NullInt64
	var reviewedAt sql.NullTime
	if err := scanner.Scan(&item.ID, &item.CreatorID, &item.CreatorName, &item.CreatorEmail, &item.AmountCents, &item.Status, &item.PayoutMethod, &item.PayoutAccount, &item.AccountName, &item.Note, &item.ReviewNote, &reviewedBy, &reviewedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return item, err
	}
	if reviewedBy.Valid {
		item.ReviewedBy = &reviewedBy.Int64
	}
	if reviewedAt.Valid {
		item.ReviewedAt = &reviewedAt.Time
	}
	return item, nil
}

func (s *Store) ReviewCreatorPayout(actorID, payoutID int64, status, reviewNote string) (domain.CreatorPayout, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	reviewNote = strings.TrimSpace(reviewNote)
	if status != "paid" && status != "rejected" {
		return domain.CreatorPayout{}, errors.New("invalid payout review status")
	}
	if len([]rune(reviewNote)) > 500 || containsControl(reviewNote) {
		return domain.CreatorPayout{}, errors.New("review note is too long")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.CreatorPayout{}, err
	}
	defer tx.Rollback()
	var currentStatus string
	if err := tx.QueryRow(`SELECT status FROM creator_payouts WHERE id = ? FOR UPDATE`, payoutID).Scan(&currentStatus); err != nil {
		return domain.CreatorPayout{}, err
	}
	if currentStatus != "pending" {
		return domain.CreatorPayout{}, errors.New("only pending payouts can be reviewed")
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`UPDATE creator_payouts SET status = ?, review_note = ?, reviewed_by = ?, reviewed_at = ?, updated_at = ? WHERE id = ?`, status, reviewNote, actorID, now, now, payoutID); err != nil {
		return domain.CreatorPayout{}, err
	}
	action := "payout_rejected"
	if status == "paid" {
		action = "payout_paid"
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'creator_payout', ?, ?, ?, ?)`, actorID, payoutID, action, reviewNote, now); err != nil {
		return domain.CreatorPayout{}, err
	}
	item, err := getCreatorPayoutWithQuery(tx, `WHERE p.id = ?`, payoutID)
	if err != nil {
		return domain.CreatorPayout{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.CreatorPayout{}, err
	}
	return item, nil
}

func randomOrderNo() (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "RBC" + time.Now().UTC().Format("20060102150405") + strings.ToUpper(hex.EncodeToString(raw)), nil
}
