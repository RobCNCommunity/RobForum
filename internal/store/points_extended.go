package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

func (s *Store) validateLotteryPrizeConfig(prize domain.LotteryPrize) error {
	switch prize.PrizeType {
	case "points":
		var config struct {
			Points int64 `json:"points"`
		}
		if json.Unmarshal([]byte(prize.ConfigJSON), &config) != nil || config.Points < 1 {
			return errors.New("积分奖品配置无效")
		}
	case "membership":
		var config struct {
			DurationDays int    `json:"duration_days"`
			DurationUnit string `json:"duration_unit"`
		}
		if json.Unmarshal([]byte(prize.ConfigJSON), &config) != nil || prize.BoundID < 1 || config.DurationDays < 1 || config.DurationDays > 3650 || config.DurationUnit != "" && config.DurationUnit != "days" && config.DurationUnit != "months" {
			return errors.New("会员奖品需要会员等级和 1 至 3650 天有效期")
		}
		var enabled int
		if err := s.db.QueryRow(`SELECT enabled FROM membership_tiers WHERE id=?`, prize.BoundID).Scan(&enabled); err != nil || enabled == 0 {
			return errors.New("绑定的会员等级不存在或未启用")
		}
	case "wallet":
		var config struct {
			AmountCents int64 `json:"amount_cents"`
		}
		if json.Unmarshal([]byte(prize.ConfigJSON), &config) != nil || config.AmountCents < 1 || config.AmountCents > maxRedeemCodeCents {
			return errors.New("钱包奖品金额无效")
		}
	case "physical":
		if prize.BoundID < 1 {
			return errors.New("实物奖品必须关联积分商品")
		}
		var physical int
		if err := s.db.QueryRow(`SELECT physical FROM point_products WHERE id=?`, prize.BoundID).Scan(&physical); err != nil || physical == 0 {
			return errors.New("实物奖品必须关联现有实物商品")
		}
	case "custom":
		var config map[string]any
		if json.Unmarshal([]byte(prize.ConfigJSON), &config) != nil {
			return errors.New("自定义奖品配置 JSON 无效")
		}
	default:
		return errors.New("奖品类型无效")
	}
	return nil
}

func deliverLotteryPrizeTx(tx *sql.Tx, userID, winID int64, prize domain.LotteryPrize, now time.Time) (string, error) {
	switch prize.PrizeType {
	case "points":
		var config struct {
			Points int64 `json:"points"`
		}
		if json.Unmarshal([]byte(prize.ConfigJSON), &config) != nil || config.Points < 1 {
			return "", errors.New("积分奖品配置无效")
		}
		if _, err := applyPointsTx(tx, userID, config.Points, winID, "lottery_win", prize.Name, now); err != nil {
			return "", err
		}
	case "wallet":
		var config struct {
			AmountCents int64 `json:"amount_cents"`
		}
		if json.Unmarshal([]byte(prize.ConfigJSON), &config) != nil || config.AmountCents < 1 {
			return "", errors.New("钱包奖品配置无效")
		}
		if _, err := tx.Exec(`INSERT INTO wallet_ledgers (user_id,entry_type,amount_cents,reference_type,reference_id,note,created_at) VALUES (?,'lottery_win',?,'lottery_win',?,?,?)`, userID, config.AmountCents, winID, prize.Name, now); err != nil {
			return "", err
		}
	case "membership":
		var config struct {
			DurationDays int    `json:"duration_days"`
			DurationUnit string `json:"duration_unit"`
		}
		if json.Unmarshal([]byte(prize.ConfigJSON), &config) != nil || prize.BoundID < 1 || config.DurationDays < 1 {
			return "", errors.New("会员奖品配置无效")
		}
		var tierName string
		var enabled int
		if err := tx.QueryRow(`SELECT name,enabled FROM membership_tiers WHERE id=?`, prize.BoundID).Scan(&tierName, &enabled); err != nil || enabled == 0 {
			return "", errors.New("绑定的会员等级不可用")
		}
		var currentTier sql.NullInt64
		var startedAt, expiresAt sql.NullTime
		if err := tx.QueryRow(`SELECT membership_tier_id,membership_started_at,membership_expires_at FROM users WHERE id=? AND status='active' FOR UPDATE`, userID).Scan(&currentTier, &startedAt, &expiresAt); err != nil {
			return "", err
		}
		membershipStart := now
		periodStart := now
		if currentTier.Valid && currentTier.Int64 == prize.BoundID && expiresAt.Valid && expiresAt.Time.After(now) {
			periodStart = expiresAt.Time.UTC()
			if startedAt.Valid {
				membershipStart = startedAt.Time.UTC()
			}
		}
		newExpiry := periodStart.Add(time.Duration(config.DurationDays) * 24 * time.Hour)
		if config.DurationUnit == "months" {
			newExpiry = periodStart.AddDate(0, config.DurationDays, 0)
		}
		if _, err := tx.Exec(`UPDATE users SET membership_tier_id=?,membership_started_at=?,membership_expires_at=?,updated_at=? WHERE id=?`, prize.BoundID, membershipStart, newExpiry, now, userID); err != nil {
			return "", err
		}
		_ = tierName
	case "physical", "custom":
		return "pending", nil
	default:
		return "", errors.New("奖品类型无效")
	}
	if _, err := tx.Exec(`UPDATE lottery_wins SET status='delivered',delivered_at=? WHERE id=?`, now, winID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(`INSERT INTO notifications (user_id, actor_id, kind, created_at) VALUES (?, ?, 'lottery_win', ?)`, userID, userID, now); err != nil {
		return "", err
	}
	return "delivered", nil
}

func validateShippingAddress(address domain.ShippingAddress) error {
	if strings.TrimSpace(address.Name) == "" || strings.TrimSpace(address.Phone) == "" || strings.TrimSpace(address.Province) == "" || strings.TrimSpace(address.City) == "" || strings.TrimSpace(address.Detail) == "" {
		return errors.New("请完整填写收货地址")
	}
	if len([]byte(address.Name)) > 80 || len([]byte(address.Phone)) > 32 || len([]byte(address.Province)) > 50 || len([]byte(address.City)) > 50 || len([]byte(address.District)) > 50 || len([]byte(address.Detail)) > 300 {
		return errors.New("收货地址内容过长")
	}
	return nil
}

func (s *Store) ClaimPhysicalLotteryWin(userID, winID int64, address domain.ShippingAddress) (domain.LotteryWin, error) {
	if err := validateShippingAddress(address); err != nil {
		return domain.LotteryWin{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.LotteryWin{}, err
	}
	defer tx.Rollback()
	var prizeID, productID int64
	var prizeName, prizeImage, status string
	err = tx.QueryRow(`SELECT w.prize_id,w.prize_name,p.image_url,w.status,p.bound_id FROM lottery_wins w JOIN lottery_prizes p ON p.id=w.prize_id WHERE w.id=? AND w.user_id=? AND w.physical=1 FOR UPDATE`, winID, userID).Scan(&prizeID, &prizeName, &prizeImage, &status, &productID)
	if err != nil {
		return domain.LotteryWin{}, err
	}
	if status != "pending" || productID < 1 {
		return domain.LotteryWin{}, errors.New("该奖品不可领取或已领取")
	}
	var physical int
	var productName, productImage string
	var stock int64
	if err := tx.QueryRow(`SELECT name,image_url,stock,physical FROM point_products WHERE id=? FOR UPDATE`, productID).Scan(&productName, &productImage, &stock, &physical); err != nil || physical == 0 || stock < 1 {
		return domain.LotteryWin{}, errors.New("奖品关联商品不可用")
	}
	addressJSON, err := json.Marshal(address)
	if err != nil {
		return domain.LotteryWin{}, err
	}
	orderNo, err := randomPointOrderNo()
	if err != nil {
		return domain.LotteryWin{}, err
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`UPDATE point_products SET stock=stock-1,status=IF(stock<=1,'sold_out',status),updated_at=? WHERE id=? AND stock>0`, now, productID); err != nil {
		return domain.LotteryWin{}, err
	}
	if prizeName == "" {
		prizeName = productName
	}
	if prizeImage == "" {
		prizeImage = productImage
	}
	result, err := tx.Exec(`INSERT INTO point_orders (order_no,user_id,product_id,product_name,product_image,points_spent,shipping_points,physical,recipient_name,recipient_phone,address_json,status,note,ordered_at,updated_at) VALUES (?,?,?,?,?,0,0,1,?,?,?,'pending','抽奖实物奖品',?,?)`, orderNo, userID, productID, productName, productImage, address.Name, address.Phone, addressJSON, now, now)
	if err != nil {
		return domain.LotteryWin{}, err
	}
	orderID, err := result.LastInsertId()
	if err != nil {
		return domain.LotteryWin{}, err
	}
	if _, err := tx.Exec(`UPDATE lottery_wins SET status='claimed',order_id=?,address_json=? WHERE id=?`, orderID, addressJSON, winID); err != nil {
		return domain.LotteryWin{}, err
	}
	if _, err := tx.Exec(`INSERT INTO notifications (user_id, actor_id, kind, created_at) VALUES (?, ?, 'lottery_win_claimed', ?)`, userID, userID, now); err != nil {
		return domain.LotteryWin{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.LotteryWin{}, err
	}
	wins, err := s.ListLotteryWins(userID)
	if err != nil {
		return domain.LotteryWin{}, err
	}
	for _, win := range wins {
		if win.ID == winID {
			return win, nil
		}
	}
	return domain.LotteryWin{}, sql.ErrNoRows
}

func (s *Store) DeliverLotteryWin(winID int64) (domain.LotteryWin, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.LotteryWin{}, err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	var userID int64
	if err := tx.QueryRow(`SELECT user_id FROM lottery_wins WHERE id=? AND prize_type='custom' AND status='pending' FOR UPDATE`, winID).Scan(&userID); err != nil {
		return domain.LotteryWin{}, err
	}
	result, err := tx.Exec(`UPDATE lottery_wins SET status='delivered',delivered_at=? WHERE id=? AND prize_type='custom' AND status='pending'`, now, winID)
	if err != nil {
		return domain.LotteryWin{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return domain.LotteryWin{}, errors.New("中奖记录不存在或当前状态不可发放")
	}
	if _, err := tx.Exec(`INSERT INTO notifications (user_id, actor_id, kind, created_at) VALUES (?, ?, 'lottery_win_delivered', ?)`, userID, userID, now); err != nil {
		return domain.LotteryWin{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.LotteryWin{}, err
	}
	return s.getAdminLotteryWin(winID)
}

func (s *Store) getAdminLotteryWin(winID int64) (domain.LotteryWin, error) {
	rows, err := s.listLotteryWinsQuery(`WHERE w.id=?`, winID)
	if err != nil {
		return domain.LotteryWin{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return domain.LotteryWin{}, sql.ErrNoRows
	}
	return scanLotteryWin(rows)
}

func (s *Store) ListAdminLotteryWins() ([]domain.LotteryWin, error) {
	rows, err := s.listLotteryWinsQuery(`ORDER BY w.won_at DESC LIMIT 300`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.LotteryWin, 0)
	for rows.Next() {
		item, err := scanLotteryWin(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) listLotteryWinsQuery(suffix string, args ...any) (*sql.Rows, error) {
	return s.db.Query(`SELECT w.id,w.user_id,u.display_name,w.activity_id,a.name,w.prize_id,w.prize_name,w.prize_type,w.physical,w.status,w.order_id,w.address_json,w.delivered_at,w.won_at FROM lottery_wins w JOIN users u ON u.id=w.user_id JOIN lottery_activities a ON a.id=w.activity_id `+suffix, args...)
}

func scanLotteryWin(scanner interface{ Scan(...any) error }) (domain.LotteryWin, error) {
	var item domain.LotteryWin
	var physical int
	var orderID sql.NullInt64
	var addressJSON []byte
	var deliveredAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.UserID, &item.UserName, &item.ActivityID, &item.ActivityName, &item.PrizeID, &item.PrizeName, &item.PrizeType, &physical, &item.Status, &orderID, &addressJSON, &deliveredAt, &item.WonAt)
	if err != nil {
		return item, err
	}
	item.Physical = physical != 0
	if orderID.Valid {
		item.OrderID = orderID.Int64
	}
	if deliveredAt.Valid {
		value := deliveredAt.Time
		item.DeliveredAt = &value
	}
	if len(addressJSON) > 0 {
		item.Address = &domain.ShippingAddress{}
		_ = json.Unmarshal(addressJSON, item.Address)
	}
	return item, nil
}

func (s *Store) SearchPointOrders(filter domain.PointOrderFilter) ([]domain.PointOrder, error) {
	query := `SELECT o.id FROM point_orders o WHERE 1=1`
	args := make([]any, 0, 5)
	if filter.Status != "" {
		query += ` AND o.status=?`
		args = append(args, filter.Status)
	}
	if filter.ProductID > 0 {
		query += ` AND o.product_id=?`
		args = append(args, filter.ProductID)
	}
	if filter.UserID > 0 {
		query += ` AND o.user_id=?`
		args = append(args, filter.UserID)
	}
	if filter.StartedAt != nil {
		query += ` AND o.ordered_at>=?`
		args = append(args, filter.StartedAt.UTC())
	}
	if filter.EndedAt != nil {
		query += ` AND o.ordered_at<?`
		args = append(args, filter.EndedAt.UTC())
	}
	query += ` ORDER BY o.ordered_at DESC LIMIT 300`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items := make([]domain.PointOrder, 0, len(ids))
	for _, id := range ids {
		item, err := s.GetPointOrder(id, 0, true)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Store) CompletePointOrder(id, userID int64) (domain.PointOrder, error) {
	result, err := s.db.Exec(`UPDATE point_orders SET status='completed',updated_at=? WHERE id=? AND user_id=? AND status='shipped'`, time.Now().UTC(), id, userID)
	if err != nil {
		return domain.PointOrder{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return domain.PointOrder{}, errors.New("订单不存在或当前状态不可确认")
	}
	return s.GetPointOrder(id, userID, false)
}

func (s *Store) PointOrderTracking(id, userID int64, admin bool) (domain.PointTracking, error) {
	order, err := s.GetPointOrder(id, userID, admin)
	if err != nil {
		return domain.PointTracking{}, err
	}
	if order.TrackingNo == "" || order.Carrier == "" {
		return domain.PointTracking{}, errors.New("订单暂未录入物流信息")
	}
	return domain.PointTracking{
		OrderID: order.ID, OrderNo: order.OrderNo, Carrier: order.Carrier, TrackingNo: order.TrackingNo,
		OrderStatus: order.Status, QueryURL: trackingQueryURL(order.Carrier, order.TrackingNo), UpdatedAt: order.UpdatedAt,
	}, nil
}

func trackingQueryURL(carrier, trackingNo string) string {
	trackingNo = url.QueryEscape(strings.TrimSpace(trackingNo))
	carrierLower := strings.ToLower(strings.TrimSpace(carrier))
	switch {
	case strings.Contains(carrierLower, "顺丰") || strings.Contains(carrierLower, "sf"):
		return "https://www.sf-express.com/chn/sc/dynamic_function/waybill/#search/bill-number/" + trackingNo
	case strings.Contains(carrierLower, "京东") || strings.Contains(carrierLower, "jd"):
		return "https://www.jdl.com/order/search?waybillCodes=" + trackingNo
	default:
		return "https://t.17track.net/zh-cn#nums=" + trackingNo
	}
}

func (s *Store) PointsDashboard() (domain.PointsDashboard, error) {
	var result domain.PointsDashboard
	if err := s.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(balance),0),COALESCE(SUM(total_earned),0),COALESCE(SUM(total_spent),0) FROM point_accounts`).Scan(&result.AccountCount, &result.TotalBalance, &result.TotalEarned, &result.TotalSpent); err != nil {
		return result, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM point_products`).Scan(&result.ProductCount); err != nil {
		return result, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(status='pending'),0) FROM point_orders`).Scan(&result.OrderCount, &result.PendingOrders); err != nil {
		return result, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(prize_id IS NOT NULL),0) FROM lottery_draws`).Scan(&result.DrawCount, &result.WinCount); err != nil {
		return result, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM lottery_wins WHERE status IN ('pending','claimed')`).Scan(&result.PendingWins); err != nil {
		return result, err
	}
	rows, err := s.db.Query(`SELECT a.id,a.name,COUNT(d.id),COUNT(DISTINCT d.user_id),COALESCE(SUM(d.prize_id IS NOT NULL),0),COALESCE(SUM(d.points_charged),0),(SELECT COUNT(*) FROM lottery_wins w WHERE w.activity_id=a.id AND w.status='delivered'),(SELECT COUNT(*) FROM lottery_wins w WHERE w.activity_id=a.id AND w.status IN ('pending','claimed')) FROM lottery_activities a LEFT JOIN lottery_draws d ON d.activity_id=a.id GROUP BY a.id,a.name ORDER BY a.id DESC`)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	result.Activities = make([]domain.LotteryActivityStats, 0)
	for rows.Next() {
		var item domain.LotteryActivityStats
		if err := rows.Scan(&item.ActivityID, &item.ActivityName, &item.DrawCount, &item.UniqueUsers, &item.WinCount, &item.PointsSpent, &item.DeliveredCount, &item.PendingCount); err != nil {
			return result, err
		}
		if item.DrawCount > 0 {
			item.WinRateBP = item.WinCount * 10000 / item.DrawCount
		}
		result.Activities = append(result.Activities, item)
	}
	return result, rows.Err()
}

func (s *Store) PointOrderFilterFromValues(status string, productID, userID int64, start, end *time.Time) (domain.PointOrderFilter, error) {
	status = strings.TrimSpace(status)
	if status != "" && status != "pending" && status != "shipped" && status != "completed" && status != "cancelled" {
		return domain.PointOrderFilter{}, fmt.Errorf("订单状态无效")
	}
	return domain.PointOrderFilter{Status: status, ProductID: productID, UserID: userID, StartedAt: start, EndedAt: end}, nil
}
