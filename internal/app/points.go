package app

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"roblox-community/internal/domain"
	"roblox-community/internal/store"
)

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	return id, err == nil && id > 0
}

func (s *Server) myPoints(w http.ResponseWriter, r *http.Request) {
	result, err := s.store.PointSummary(currentUser(r).ID, 100)
	if err != nil {
		writeError(w, 500, "points_failed", "积分信息加载失败")
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) listPointProducts(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListPointProducts(false)
	if err != nil {
		writeError(w, 500, "products_failed", "积分商品加载失败")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) redeemPointProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "productID")
	if !ok {
		writeError(w, 400, "product_invalid", "商品无效")
		return
	}
	var input struct {
		Address *domain.ShippingAddress `json:"address"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.RedeemPointProduct(currentUser(r).ID, id, input.Address)
	if err != nil {
		status := 400
		if errors.Is(err, store.ErrProductUnavailable) {
			status = 404
		}
		writeError(w, status, "redeem_failed", err.Error())
		return
	}
	writeJSON(w, 201, item)
}
func (s *Server) myPointOrders(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListPointOrders(currentUser(r).ID, false)
	if err != nil {
		writeError(w, 500, "orders_failed", "兑换订单加载失败")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) completeMyPointOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "orderID")
	if !ok {
		writeError(w, 400, "order_invalid", "订单无效")
		return
	}
	item, err := s.store.CompletePointOrder(id, currentUser(r).ID)
	if err != nil {
		writeError(w, 400, "complete_failed", err.Error())
		return
	}
	writeJSON(w, 200, item)
}
func (s *Server) myPointOrderTracking(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "orderID")
	if !ok {
		writeError(w, 400, "order_invalid", "订单无效")
		return
	}
	item, err := s.store.PointOrderTracking(id, currentUser(r).ID, false)
	if err != nil {
		writeError(w, 404, "tracking_unavailable", err.Error())
		return
	}
	writeJSON(w, 200, item)
}
func (s *Server) listLotteryActivities(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListLotteryActivities(true)
	if err != nil {
		writeError(w, 500, "lottery_failed", "抽奖活动加载失败")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) drawLottery(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "activityID")
	if !ok {
		writeError(w, 400, "activity_invalid", "活动无效")
		return
	}
	result, err := s.store.DrawLottery(currentUser(r).ID, id)
	if err != nil {
		status := 400
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, store.ErrLotteryUnavailable) {
			status = 404
		}
		writeError(w, status, "lottery_draw_failed", err.Error())
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) myLotteryWins(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListLotteryWins(currentUser(r).ID)
	if err != nil {
		writeError(w, 500, "wins_failed", "中奖记录加载失败")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) claimLotteryWin(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "winID")
	if !ok {
		writeError(w, 400, "win_invalid", "中奖记录无效")
		return
	}
	var input struct {
		Address domain.ShippingAddress `json:"address"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.ClaimPhysicalLotteryWin(currentUser(r).ID, id, input.Address)
	if err != nil {
		writeError(w, 400, "claim_failed", err.Error())
		return
	}
	writeJSON(w, 200, item)
}

func (s *Server) adminPointAccounts(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListPointAccounts(300)
	if err != nil {
		writeError(w, 500, "points_failed", "用户积分加载失败")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) adminPointProducts(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListPointProducts(true)
	if err != nil {
		writeError(w, 500, "products_failed", "商品加载失败")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) saveAdminPointProduct(w http.ResponseWriter, r *http.Request) {
	var input domain.PointProduct
	if !decodeJSON(w, r, &input) {
		return
	}
	id := int64(0)
	if raw := chi.URLParam(r, "productID"); raw != "" {
		var ok bool
		id, ok = pathID(r, "productID")
		if !ok {
			writeError(w, 400, "product_invalid", "商品无效")
			return
		}
	}
	item, err := s.store.SavePointProduct(id, input)
	if err != nil {
		writeError(w, 400, "product_invalid", err.Error())
		return
	}
	status := 200
	if id == 0 {
		status = 201
	}
	writeJSON(w, status, item)
}
func (s *Server) deleteAdminPointProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "productID")
	if !ok {
		writeError(w, 400, "product_invalid", "商品无效")
		return
	}
	if err := s.store.DeletePointProduct(id); err != nil {
		writeError(w, 404, "product_not_found", "商品不存在")
		return
	}
	writeJSON(w, 200, map[string]bool{"disabled": true})
}
func (s *Server) adminPointOrders(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	productID, err := optionalPositiveID(query.Get("product_id"))
	if err != nil {
		writeError(w, 400, "filter_invalid", "商品筛选无效")
		return
	}
	userID, err := optionalPositiveID(query.Get("user_id"))
	if err != nil {
		writeError(w, 400, "filter_invalid", "用户筛选无效")
		return
	}
	start, err := optionalDate(query.Get("start"))
	if err != nil {
		writeError(w, 400, "filter_invalid", "开始日期无效")
		return
	}
	end, err := optionalDate(query.Get("end"))
	if err != nil {
		writeError(w, 400, "filter_invalid", "结束日期无效")
		return
	}
	if end != nil {
		value := end.Add(24 * time.Hour)
		end = &value
	}
	filter, err := s.store.PointOrderFilterFromValues(query.Get("status"), productID, userID, start, end)
	if err != nil {
		writeError(w, 400, "filter_invalid", err.Error())
		return
	}
	items, err := s.store.SearchPointOrders(filter)
	if err != nil {
		writeError(w, 500, "orders_failed", "订单加载失败")
		return
	}
	writeJSON(w, 200, items)
}
func optionalPositiveID(raw string) (int64, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("ID 无效")
	}
	return id, nil
}
func optionalDate(raw string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	value, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}
func (s *Server) shipAdminPointOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "orderID")
	if !ok {
		writeError(w, 400, "order_invalid", "订单无效")
		return
	}
	var input struct {
		Carrier    string `json:"carrier"`
		TrackingNo string `json:"tracking_no"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.ShipPointOrder(id, input.Carrier, input.TrackingNo)
	if err != nil {
		writeError(w, 400, "ship_failed", err.Error())
		return
	}
	writeJSON(w, 200, item)
}
func (s *Server) adminLotteryActivities(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListLotteryActivities(false)
	if err != nil {
		writeError(w, 500, "lottery_failed", "活动加载失败")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) saveAdminLotteryActivity(w http.ResponseWriter, r *http.Request) {
	var input domain.LotteryActivity
	if !decodeJSON(w, r, &input) {
		return
	}
	id := int64(0)
	if chi.URLParam(r, "activityID") != "" {
		var ok bool
		id, ok = pathID(r, "activityID")
		if !ok {
			writeError(w, 400, "activity_invalid", "活动无效")
			return
		}
	}
	item, err := s.store.SaveLotteryActivity(id, input)
	if err != nil {
		writeError(w, 400, "activity_invalid", err.Error())
		return
	}
	status := 200
	if id == 0 {
		status = 201
	}
	writeJSON(w, status, item)
}
func (s *Server) deleteAdminLotteryActivity(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "activityID")
	if !ok {
		writeError(w, 400, "activity_invalid", "活动无效")
		return
	}
	if err := s.store.DeleteLotteryActivity(id); err != nil {
		writeError(w, 404, "activity_not_found", "活动不存在")
		return
	}
	writeJSON(w, 200, map[string]bool{"disabled": true})
}
func (s *Server) saveAdminLotteryPrize(w http.ResponseWriter, r *http.Request) {
	activityID, ok := pathID(r, "activityID")
	if !ok {
		writeError(w, 400, "activity_invalid", "活动无效")
		return
	}
	var input domain.LotteryPrize
	if !decodeJSON(w, r, &input) {
		return
	}
	input.ActivityID = activityID
	id := int64(0)
	if chi.URLParam(r, "prizeID") != "" {
		id, ok = pathID(r, "prizeID")
		if !ok {
			writeError(w, 400, "prize_invalid", "奖品无效")
			return
		}
	}
	item, err := s.store.SaveLotteryPrize(id, input)
	if err != nil {
		writeError(w, 400, "prize_invalid", err.Error())
		return
	}
	status := 200
	if id == 0 {
		status = 201
	}
	writeJSON(w, status, item)
}
func (s *Server) deleteAdminLotteryPrize(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "prizeID")
	if !ok {
		writeError(w, 400, "prize_invalid", "奖品无效")
		return
	}
	if err := s.store.DeleteLotteryPrize(id); err != nil {
		writeError(w, 400, "prize_delete_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"deleted": true})
}
func (s *Server) adminPointsDashboard(w http.ResponseWriter, _ *http.Request) {
	item, err := s.store.PointsDashboard()
	if err != nil {
		writeError(w, 500, "dashboard_failed", "统计数据加载失败")
		return
	}
	writeJSON(w, 200, item)
}
func (s *Server) adminLotteryWins(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListAdminLotteryWins()
	if err != nil {
		writeError(w, 500, "wins_failed", "中奖记录加载失败")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) deliverAdminLotteryWin(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "winID")
	if !ok {
		writeError(w, 400, "win_invalid", "中奖记录无效")
		return
	}
	item, err := s.store.DeliverLotteryWin(id)
	if err != nil {
		writeError(w, 400, "deliver_failed", err.Error())
		return
	}
	writeJSON(w, 200, item)
}
