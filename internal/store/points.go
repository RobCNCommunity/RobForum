package store

import (
	cryptorand "crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

var (
	ErrPointsInsufficient = errors.New("积分不足")
	ErrProductUnavailable = errors.New("商品不存在或已售罄")
	ErrLotteryUnavailable = errors.New("抽奖活动未开始或已结束")
	ErrLotteryLimit       = errors.New("已达到抽奖次数限制")
)

func ensurePointAccount(q interface{ QueryRow(string, ...any) *sql.Row }, exec interface {
	Exec(string, ...any) (sql.Result, error)
}, userID int64, now time.Time, lock bool) (domain.PointAccount, error) {
	if _, err := exec.Exec(`INSERT IGNORE INTO point_accounts (user_id, balance, total_earned, total_spent, updated_at) VALUES (?, 0, 0, 0, ?)`, userID, now); err != nil {
		return domain.PointAccount{}, err
	}
	query := `SELECT user_id, balance, total_earned, total_spent, updated_at FROM point_accounts WHERE user_id = ?`
	if lock {
		query += ` FOR UPDATE`
	}
	var a domain.PointAccount
	err := q.QueryRow(query, userID).Scan(&a.UserID, &a.Balance, &a.TotalEarned, &a.TotalSpent, &a.UpdatedAt)
	return a, err
}

func (s *Store) PointSummary(userID int64, limit int) (domain.PointSummary, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	a, err := ensurePointAccount(s.db, s.db, userID, time.Now().UTC(), false)
	if err != nil {
		return domain.PointSummary{}, err
	}
	rows, err := s.db.Query(`SELECT id, user_id, amount, entry_type, source_id, balance_after, note, created_at FROM point_ledgers WHERE user_id = ? ORDER BY id DESC LIMIT `+fmt.Sprint(limit), userID)
	if err != nil {
		return domain.PointSummary{}, err
	}
	defer rows.Close()
	entries := make([]domain.PointLedger, 0)
	for rows.Next() {
		var e domain.PointLedger
		if err := rows.Scan(&e.ID, &e.UserID, &e.Amount, &e.EntryType, &e.SourceID, &e.BalanceAfter, &e.Note, &e.CreatedAt); err != nil {
			return domain.PointSummary{}, err
		}
		entries = append(entries, e)
	}
	return domain.PointSummary{Account: a, Entries: entries}, rows.Err()
}

func (s *Store) ListPointAccounts(limit int) ([]domain.PointAccount, error) {
	if limit < 1 || limit > 500 {
		limit = 200
	}
	rows, err := s.db.Query(`SELECT a.user_id,a.balance,a.total_earned,a.total_spent,a.updated_at,u.display_name,u.avatar_url FROM point_accounts a JOIN users u ON u.id=a.user_id ORDER BY a.updated_at DESC LIMIT ` + fmt.Sprint(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.PointAccount, 0)
	for rows.Next() {
		var a domain.PointAccount
		if err := rows.Scan(&a.UserID, &a.Balance, &a.TotalEarned, &a.TotalSpent, &a.UpdatedAt, &a.UserName, &a.UserAvatar); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func applyPointsTx(tx *sql.Tx, userID, amount, sourceID int64, entryType, note string, now time.Time) (int64, error) {
	a, err := ensurePointAccount(tx, tx, userID, now, true)
	if err != nil {
		return 0, err
	}
	if amount == 0 {
		return a.Balance, nil
	}
	if amount < 0 && a.Balance < -amount {
		return 0, ErrPointsInsufficient
	}
	balance := a.Balance + amount
	if _, err := tx.Exec(`UPDATE point_accounts SET balance = ?, total_earned = total_earned + ?, total_spent = total_spent + ?, updated_at = ? WHERE user_id = ?`, balance, maxInt64(amount, 0), maxInt64(-amount, 0), now, userID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`INSERT INTO point_ledgers (user_id, amount, entry_type, source_id, balance_after, note, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, userID, amount, entryType, sourceID, balance, note, now); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`INSERT INTO notifications (user_id, actor_id, kind, created_at) VALUES (?, ?, ?, ?)`, userID, userID, "points_changed", now); err != nil {
		return 0, err
	}
	return balance, nil
}

func maxInt64(v, floor int64) int64 {
	if v > floor {
		return v
	}
	return floor
}

func (s *Store) AddPoints(userID, amount, sourceID int64, entryType, note string) (domain.PointAccount, error) {
	if amount <= 0 || strings.TrimSpace(entryType) == "" {
		return domain.PointAccount{}, errors.New("积分参数无效")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.PointAccount{}, err
	}
	defer tx.Rollback()
	if _, err = applyPointsTx(tx, userID, amount, sourceID, entryType, strings.TrimSpace(note), time.Now().UTC()); err != nil {
		return domain.PointAccount{}, err
	}
	a, err := ensurePointAccount(tx, tx, userID, time.Now().UTC(), false)
	if err != nil {
		return domain.PointAccount{}, err
	}
	if err = tx.Commit(); err != nil {
		return domain.PointAccount{}, err
	}
	return a, nil
}

func (s *Store) ListPointProducts(includeDraft bool) ([]domain.PointProduct, error) {
	query := `SELECT id,name,image_url,points_required,shipping_points,stock,physical,description,status,sort_order,created_at,updated_at FROM point_products`
	if !includeDraft {
		query += ` WHERE status = 'on_sale'`
	}
	query += ` ORDER BY sort_order ASC, id DESC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.PointProduct, 0)
	for rows.Next() {
		var p domain.PointProduct
		var physical int
		if err := rows.Scan(&p.ID, &p.Name, &p.ImageURL, &p.PointsRequired, &p.ShippingPoints, &p.Stock, &physical, &p.Description, &p.Status, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Physical = physical != 0
		items = append(items, p)
	}
	return items, rows.Err()
}

func normalizeProduct(p domain.PointProduct) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || len([]byte(p.Name)) > 120 || p.PointsRequired < 1 || p.ShippingPoints < 0 || p.Stock < 0 || p.Status != "on_sale" && p.Status != "off_sale" && p.Status != "draft" && p.Status != "sold_out" {
		return errors.New("商品参数无效")
	}
	return nil
}
func (s *Store) SavePointProduct(id int64, p domain.PointProduct) (domain.PointProduct, error) {
	if err := normalizeProduct(p); err != nil {
		return domain.PointProduct{}, err
	}
	now := time.Now().UTC()
	var err error
	if id == 0 {
		var r sql.Result
		r, err = s.db.Exec(`INSERT INTO point_products (name,image_url,points_required,shipping_points,stock,physical,description,status,sort_order,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, p.Name, p.ImageURL, p.PointsRequired, p.ShippingPoints, p.Stock, p.Physical, p.Description, p.Status, p.SortOrder, now, now)
		if err == nil {
			id, err = r.LastInsertId()
		}
	} else {
		_, err = s.db.Exec(`UPDATE point_products SET name=?,image_url=?,points_required=?,shipping_points=?,stock=?,physical=?,description=?,status=?,sort_order=?,updated_at=? WHERE id=?`, p.Name, p.ImageURL, p.PointsRequired, p.ShippingPoints, p.Stock, p.Physical, p.Description, p.Status, p.SortOrder, now, id)
	}
	if err != nil {
		return domain.PointProduct{}, err
	}
	return s.getPointProduct(id)
}
func (s *Store) getPointProduct(id int64) (domain.PointProduct, error) {
	var p domain.PointProduct
	var physical int
	err := s.db.QueryRow(`SELECT id,name,image_url,points_required,shipping_points,stock,physical,description,status,sort_order,created_at,updated_at FROM point_products WHERE id=?`, id).Scan(&p.ID, &p.Name, &p.ImageURL, &p.PointsRequired, &p.ShippingPoints, &p.Stock, &physical, &p.Description, &p.Status, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt)
	p.Physical = physical != 0
	return p, err
}
func (s *Store) DeletePointProduct(id int64) error {
	r, err := s.db.Exec(`UPDATE point_products SET status='off_sale',updated_at=? WHERE id=?`, time.Now().UTC(), id)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) RedeemPointProduct(userID, productID int64, address *domain.ShippingAddress) (domain.PointOrder, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.PointOrder{}, err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	var p domain.PointProduct
	var physical int
	err = tx.QueryRow(`SELECT id,name,image_url,points_required,shipping_points,stock,physical,description,status,sort_order,created_at,updated_at FROM point_products WHERE id=? FOR UPDATE`, productID).Scan(&p.ID, &p.Name, &p.ImageURL, &p.PointsRequired, &p.ShippingPoints, &p.Stock, &physical, &p.Description, &p.Status, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt)
	p.Physical = physical != 0
	if errors.Is(err, sql.ErrNoRows) || p.Status != "on_sale" || p.Stock < 1 {
		return domain.PointOrder{}, ErrProductUnavailable
	}
	if err != nil {
		return domain.PointOrder{}, err
	}
	if p.Physical && address == nil {
		return domain.PointOrder{}, errors.New("实物商品需要收货地址")
	}
	if p.Physical && (strings.TrimSpace(address.Name) == "" || strings.TrimSpace(address.Phone) == "" || strings.TrimSpace(address.Province) == "" || strings.TrimSpace(address.City) == "" || strings.TrimSpace(address.Detail) == "") {
		return domain.PointOrder{}, errors.New("请完整填写收货地址")
	}
	if _, err = tx.Exec(`UPDATE point_products SET stock=stock-1, status=IF(stock<=1,'sold_out',status), updated_at=? WHERE id=? AND stock>0`, now, productID); err != nil {
		return domain.PointOrder{}, err
	}
	addrJSON := []byte(nil)
	recipientName, recipientPhone := "", ""
	if address != nil {
		addrJSON, _ = json.Marshal(address)
		recipientName = address.Name
		recipientPhone = address.Phone
	}
	orderNo, err := randomPointOrderNo()
	if err != nil {
		return domain.PointOrder{}, err
	}
	total := p.PointsRequired + p.ShippingPoints
	balance, err := applyPointsTx(tx, userID, -total, 0, "redeem_product", p.Name, now)
	if err != nil {
		return domain.PointOrder{}, err
	}
	orderStatus := "completed"
	if p.Physical {
		orderStatus = "pending"
	}
	r, err := tx.Exec(`INSERT INTO point_orders (order_no,user_id,product_id,product_name,product_image,points_spent,shipping_points,physical,recipient_name,recipient_phone,address_json,status,ordered_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, orderNo, userID, p.ID, p.Name, p.ImageURL, p.PointsRequired, p.ShippingPoints, p.Physical, recipientName, recipientPhone, addrJSON, orderStatus, now, now)
	if err != nil {
		return domain.PointOrder{}, err
	}
	id, _ := r.LastInsertId()
	if err = tx.Commit(); err != nil {
		return domain.PointOrder{}, err
	}
	o, err := s.GetPointOrder(id, userID, false)
	if err != nil {
		return domain.PointOrder{}, err
	}
	o.UserID = userID
	_ = balance
	return o, nil
}
func randomPointOrderNo() (string, error) { return randomOrderNo() }
func (s *Store) GetPointOrder(id, userID int64, admin bool) (domain.PointOrder, error) {
	q := `SELECT o.id,o.order_no,o.user_id,o.product_id,o.product_name,o.product_image,o.points_spent,o.shipping_points,o.physical,o.recipient_name,o.recipient_phone,o.address_json,o.carrier,o.tracking_no,o.status,o.note,o.ordered_at,o.updated_at,u.display_name FROM point_orders o JOIN users u ON u.id=o.user_id WHERE o.id=?`
	args := []any{id}
	if !admin {
		q += ` AND o.user_id=?`
		args = append(args, userID)
	}
	var o domain.PointOrder
	var physical int
	var addr []byte
	var name sql.NullString
	var recipientName, recipientPhone string
	if err := s.db.QueryRow(q, args...).Scan(&o.ID, &o.OrderNo, &o.UserID, &o.ProductID, &o.ProductName, &o.ProductImage, &o.PointsSpent, &o.ShippingPoints, &physical, &recipientName, &recipientPhone, &addr, &o.Carrier, &o.TrackingNo, &o.Status, &o.Note, &o.OrderedAt, &o.UpdatedAt, &name); err != nil {
		return o, err
	}
	o.Physical = physical != 0
	o.UserName = name.String
	if len(addr) > 0 {
		o.Address = &domain.ShippingAddress{}
		_ = json.Unmarshal(addr, o.Address)
	}
	return o, nil
}
func (s *Store) ListPointOrders(userID int64, admin bool) ([]domain.PointOrder, error) {
	q := `SELECT id FROM point_orders`
	args := []any{}
	if !admin {
		q += ` WHERE user_id=?`
		args = append(args, userID)
	}
	q += ` ORDER BY ordered_at DESC LIMIT 200`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.PointOrder, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		o, err := s.GetPointOrder(id, userID, admin)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
func (s *Store) ShipPointOrder(id int64, carrier, tracking string) (domain.PointOrder, error) {
	carrier = strings.TrimSpace(carrier)
	tracking = strings.TrimSpace(tracking)
	if carrier == "" || tracking == "" {
		return domain.PointOrder{}, errors.New("物流信息不能为空")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.PointOrder{}, err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	var userID int64
	if err := tx.QueryRow(`SELECT user_id FROM point_orders WHERE id=? AND status='pending' FOR UPDATE`, id).Scan(&userID); err != nil {
		return domain.PointOrder{}, err
	}
	r, err := tx.Exec(`UPDATE point_orders SET carrier=?,tracking_no=?,status='shipped',updated_at=? WHERE id=? AND status='pending'`, carrier, tracking, now, id)
	if err != nil {
		return domain.PointOrder{}, err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return domain.PointOrder{}, sql.ErrNoRows
	}
	if _, err := tx.Exec(`INSERT INTO notifications (user_id, actor_id, kind, created_at) VALUES (?, ?, 'points_order_shipped', ?)`, userID, userID, now); err != nil {
		return domain.PointOrder{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.PointOrder{}, err
	}
	return s.GetPointOrder(id, 0, true)
}

func (s *Store) ListLotteryActivities(activeOnly bool) ([]domain.LotteryActivity, error) {
	q := `SELECT id,name,cover_url,description,style,cost_points,daily_limit,total_limit,daily_free_attempts,starts_at,ends_at,status,sort_order,created_at,updated_at FROM lottery_activities`
	if activeOnly {
		q += ` WHERE status='on_sale' AND starts_at<=UTC_TIMESTAMP() AND ends_at>=UTC_TIMESTAMP()`
	}
	q += ` ORDER BY sort_order ASC,id DESC`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	out := make([]domain.LotteryActivity, 0)
	for rows.Next() {
		var a domain.LotteryActivity
		if err := rows.Scan(&a.ID, &a.Name, &a.CoverURL, &a.Description, &a.Style, &a.CostPoints, &a.DailyLimit, &a.TotalLimit, &a.DailyFreeAttempts, &a.StartsAt, &a.EndsAt, &a.Status, &a.SortOrder, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for index := range out {
		prizes, err := s.ListLotteryPrizes(out[index].ID)
		if err != nil {
			return nil, err
		}
		out[index].Prizes = prizes
	}
	return out, nil
}
func (s *Store) SaveLotteryActivity(id int64, a domain.LotteryActivity) (domain.LotteryActivity, error) {
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" || a.CostPoints < 0 || !a.EndsAt.After(a.StartsAt) || a.Style != "wheel" && a.Style != "scratch" || a.DailyLimit < 0 || a.TotalLimit < 0 || a.DailyFreeAttempts < 0 || a.Status != "draft" && a.Status != "on_sale" && a.Status != "off_sale" {
		return domain.LotteryActivity{}, errors.New("抽奖活动参数无效")
	}
	now := time.Now().UTC()
	var err error
	if id == 0 {
		r, e := s.db.Exec(`INSERT INTO lottery_activities (name,cover_url,description,style,cost_points,daily_limit,total_limit,daily_free_attempts,starts_at,ends_at,status,sort_order,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.Name, a.CoverURL, a.Description, a.Style, a.CostPoints, a.DailyLimit, a.TotalLimit, a.DailyFreeAttempts, a.StartsAt, a.EndsAt, a.Status, a.SortOrder, now, now)
		err = e
		if e == nil {
			id, _ = r.LastInsertId()
		}
	} else {
		_, err = s.db.Exec(`UPDATE lottery_activities SET name=?,cover_url=?,description=?,style=?,cost_points=?,daily_limit=?,total_limit=?,daily_free_attempts=?,starts_at=?,ends_at=?,status=?,sort_order=?,updated_at=? WHERE id=?`, a.Name, a.CoverURL, a.Description, a.Style, a.CostPoints, a.DailyLimit, a.TotalLimit, a.DailyFreeAttempts, a.StartsAt, a.EndsAt, a.Status, a.SortOrder, now, id)
	}
	if err != nil {
		return domain.LotteryActivity{}, err
	}
	items, _ := s.ListLotteryActivities(false)
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return domain.LotteryActivity{}, sql.ErrNoRows
}
func (s *Store) DeleteLotteryActivity(id int64) error {
	r, e := s.db.Exec(`UPDATE lottery_activities SET status='off_sale',updated_at=? WHERE id=?`, time.Now().UTC(), id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) SaveLotteryPrize(id int64, p domain.LotteryPrize) (domain.LotteryPrize, error) {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || p.ProbabilityBP < 0 || p.ProbabilityBP > 10000 || p.Stock < 0 {
		return domain.LotteryPrize{}, errors.New("奖品参数无效")
	}
	if p.ConfigJSON == "" {
		p.ConfigJSON = "{}"
	}
	if id > 0 {
		var existingActivityID int64
		if err := s.db.QueryRow(`SELECT activity_id FROM lottery_prizes WHERE id=?`, id).Scan(&existingActivityID); err != nil {
			return domain.LotteryPrize{}, err
		}
		if existingActivityID != p.ActivityID {
			return domain.LotteryPrize{}, errors.New("奖品不属于当前活动")
		}
	}
	if p.PrizeType == "physical" {
		p.Physical = true
	} else {
		p.Physical = false
	}
	if err := s.validateLotteryPrizeConfig(p); err != nil {
		return domain.LotteryPrize{}, err
	}
	var otherTotal int
	if err := s.db.QueryRow(`SELECT COALESCE(SUM(probability_bp), 0) FROM lottery_prizes WHERE activity_id=? AND id<>?`, p.ActivityID, id).Scan(&otherTotal); err != nil {
		return domain.LotteryPrize{}, err
	}
	if otherTotal+p.ProbabilityBP > 10000 {
		return domain.LotteryPrize{}, errors.New("奖品中奖概率合计不能超过 100%")
	}
	now := time.Now().UTC()
	var err error
	if id == 0 {
		r, e := s.db.Exec(`INSERT INTO lottery_prizes (activity_id,name,image_url,probability_bp,stock,prize_type,bound_id,config_json,physical,sort_order,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, p.ActivityID, p.Name, p.ImageURL, p.ProbabilityBP, p.Stock, p.PrizeType, p.BoundID, p.ConfigJSON, p.Physical, p.SortOrder, now, now)
		err = e
		if e == nil {
			id, _ = r.LastInsertId()
		}
	} else {
		_, err = s.db.Exec(`UPDATE lottery_prizes SET name=?,image_url=?,probability_bp=?,stock=?,prize_type=?,bound_id=?,config_json=?,physical=?,sort_order=?,updated_at=? WHERE id=?`, p.Name, p.ImageURL, p.ProbabilityBP, p.Stock, p.PrizeType, p.BoundID, p.ConfigJSON, p.Physical, p.SortOrder, now, id)
	}
	if err != nil {
		return domain.LotteryPrize{}, err
	}
	return s.getLotteryPrize(id)
}
func (s *Store) ListLotteryPrizes(activityID int64) ([]domain.LotteryPrize, error) {
	rows, err := s.db.Query(`SELECT id FROM lottery_prizes WHERE activity_id=? ORDER BY sort_order ASC,id ASC`, activityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.LotteryPrize, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		item, err := s.getLotteryPrize(id)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) DeleteLotteryPrize(id int64) error {
	result, err := s.db.Exec(`DELETE FROM lottery_prizes WHERE id=?`, id)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
func (s *Store) getLotteryPrize(id int64) (domain.LotteryPrize, error) {
	var p domain.LotteryPrize
	var physical int
	err := s.db.QueryRow(`SELECT id,activity_id,name,image_url,probability_bp,stock,prize_type,bound_id,config_json,physical,sort_order,created_at,updated_at FROM lottery_prizes WHERE id=?`, id).Scan(&p.ID, &p.ActivityID, &p.Name, &p.ImageURL, &p.ProbabilityBP, &p.Stock, &p.PrizeType, &p.BoundID, &p.ConfigJSON, &physical, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt)
	p.Physical = physical != 0
	return p, err
}

func (s *Store) DrawLottery(userID, activityID int64) (domain.LotteryDrawResult, error) {
	tx, e := s.db.Begin()
	if e != nil {
		return domain.LotteryDrawResult{}, e
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	var a domain.LotteryActivity
	e = tx.QueryRow(`SELECT id,name,cover_url,description,style,cost_points,daily_limit,total_limit,daily_free_attempts,starts_at,ends_at,status,sort_order,created_at,updated_at FROM lottery_activities WHERE id=? FOR UPDATE`, activityID).Scan(&a.ID, &a.Name, &a.CoverURL, &a.Description, &a.Style, &a.CostPoints, &a.DailyLimit, &a.TotalLimit, &a.DailyFreeAttempts, &a.StartsAt, &a.EndsAt, &a.Status, &a.SortOrder, &a.CreatedAt, &a.UpdatedAt)
	if e != nil {
		return domain.LotteryDrawResult{}, e
	}
	if a.Status != "on_sale" || now.Before(a.StartsAt) || now.After(a.EndsAt) {
		return domain.LotteryDrawResult{}, ErrLotteryUnavailable
	}
	var totalCount, dailyCount int
	e = tx.QueryRow(`SELECT COUNT(*), COALESCE(SUM(created_at >= UTC_DATE()), 0) FROM lottery_draws WHERE user_id=? AND activity_id=?`, userID, activityID).Scan(&totalCount, &dailyCount)
	if e != nil {
		return domain.LotteryDrawResult{}, e
	}
	if a.TotalLimit > 0 && totalCount >= a.TotalLimit || a.DailyLimit > 0 && dailyCount >= a.DailyLimit {
		return domain.LotteryDrawResult{}, ErrLotteryLimit
	}
	charge := a.CostPoints
	if dailyCount < a.DailyFreeAttempts {
		charge = 0
	}
	if charge > 0 {
		if _, e = applyPointsTx(tx, userID, -charge, activityID, "lottery_draw", a.Name, now); e != nil {
			return domain.LotteryDrawResult{}, e
		}
	}
	rows, e := tx.Query(`SELECT id,activity_id,name,image_url,probability_bp,stock,prize_type,bound_id,config_json,physical,sort_order,created_at,updated_at FROM lottery_prizes WHERE activity_id=? AND stock<>0 ORDER BY sort_order ASC,id ASC FOR UPDATE`, activityID)
	if e != nil {
		return domain.LotteryDrawResult{}, e
	}
	prizes := make([]domain.LotteryPrize, 0)
	for rows.Next() {
		var p domain.LotteryPrize
		var physical int
		if e = rows.Scan(&p.ID, &p.ActivityID, &p.Name, &p.ImageURL, &p.ProbabilityBP, &p.Stock, &p.PrizeType, &p.BoundID, &p.ConfigJSON, &physical, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt); e != nil {
			return domain.LotteryDrawResult{}, e
		}
		p.Physical = physical != 0
		prizes = append(prizes, p)
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		return domain.LotteryDrawResult{}, e
	}
	rows.Close()
	var winner *domain.LotteryPrize
	pickValue, randomErr := cryptorand.Int(cryptorand.Reader, big.NewInt(10000))
	if randomErr != nil {
		return domain.LotteryDrawResult{}, randomErr
	}
	pick := int(pickValue.Int64())
	for i := range prizes {
		if pick < prizes[i].ProbabilityBP {
			winner = &prizes[i]
			break
		}
		pick -= prizes[i].ProbabilityBP
	}
	var prizeID any
	if winner != nil {
		prizeID = winner.ID
		if _, e = tx.Exec(`UPDATE lottery_prizes SET stock=IF(stock>0,stock-1,stock),updated_at=? WHERE id=?`, now, winner.ID); e != nil {
			return domain.LotteryDrawResult{}, e
		}
	}
	r, e := tx.Exec(`INSERT INTO lottery_draws (user_id,activity_id,prize_id,points_charged,created_at) VALUES (?,?,?,?,?)`, userID, activityID, prizeID, charge, now)
	if e != nil {
		return domain.LotteryDrawResult{}, e
	}
	drawID, _ := r.LastInsertId()
	result := domain.LotteryDrawResult{Won: false, PointsCharged: charge}
	if winner != nil {
		result.Won = true
		result.Prize = winner
		status := "pending"
		wr, e := tx.Exec(`INSERT INTO lottery_wins (user_id,activity_id,prize_id,prize_name,prize_type,physical,status,won_at) VALUES (?,?,?,?,?,?,?,?)`, userID, activityID, winner.ID, winner.Name, winner.PrizeType, winner.Physical, status, now)
		if e != nil {
			return domain.LotteryDrawResult{}, e
		}
		winID, _ := wr.LastInsertId()
		status, e = deliverLotteryPrizeTx(tx, userID, winID, *winner, now)
		if e != nil {
			return domain.LotteryDrawResult{}, e
		}
		result.Win = &domain.LotteryWin{ID: winID, UserID: userID, ActivityID: activityID, PrizeID: winner.ID, PrizeName: winner.Name, PrizeType: winner.PrizeType, Physical: winner.Physical, Status: status, WonAt: now}
	}
	if e = tx.Commit(); e != nil {
		return domain.LotteryDrawResult{}, e
	}
	summary, _ := s.PointSummary(userID, 1)
	result.Balance = summary.Account.Balance
	_ = drawID
	return result, nil
}
func (s *Store) ListLotteryWins(userID int64) ([]domain.LotteryWin, error) {
	rows, e := s.db.Query(`SELECT w.id,w.user_id,w.activity_id,a.name,w.prize_id,w.prize_name,w.prize_type,w.physical,w.status,w.order_id,w.address_json,w.delivered_at,w.won_at FROM lottery_wins w JOIN lottery_activities a ON a.id=w.activity_id WHERE w.user_id=? ORDER BY w.won_at DESC LIMIT 100`, userID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := make([]domain.LotteryWin, 0)
	for rows.Next() {
		var w domain.LotteryWin
		var physical int
		var orderID sql.NullInt64
		var addr []byte
		var delivered sql.NullTime
		if e = rows.Scan(&w.ID, &w.UserID, &w.ActivityID, &w.ActivityName, &w.PrizeID, &w.PrizeName, &w.PrizeType, &physical, &w.Status, &orderID, &addr, &delivered, &w.WonAt); e != nil {
			return nil, e
		}
		w.Physical = physical != 0
		if orderID.Valid {
			w.OrderID = orderID.Int64
		}
		if delivered.Valid {
			v := delivered.Time
			w.DeliveredAt = &v
		}
		if len(addr) > 0 {
			w.Address = &domain.ShippingAddress{}
			_ = json.Unmarshal(addr, w.Address)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
