package app

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"roblox-community/internal/domain"
	"roblox-community/internal/payment"
)

func (s *Server) adminPayment(w http.ResponseWriter, _ *http.Request) {
	config, err := s.store.GetPaymentConfig()
	if err != nil {
		writeError(w, 500, "payment_config_failed", "支付配置读取失败")
		return
	}
	writeJSON(w, 200, config)
}

func (s *Server) updatePayment(w http.ResponseWriter, r *http.Request) {
	var input domain.PaymentConfig
	if !decodeJSON(w, r, &input) {
		return
	}
	config, err := s.store.UpdatePaymentConfig(currentUser(r).ID, input)
	if err != nil {
		writeError(w, 400, "payment_config_invalid", err.Error())
		return
	}
	writeJSON(w, 200, config)
}

func (s *Server) createResourceOrder(w http.ResponseWriter, r *http.Request) {
	resourceID, _ := strconv.ParseInt(chi.URLParam(r, "resourceID"), 10, 64)
	config, err := s.store.PaymentDeliveryConfig()
	if err != nil || !config.Enabled {
		writeError(w, 503, "payment_unavailable", "支付通道尚未配置")
		return
	}
	base, err := s.publicBaseURL()
	if err != nil {
		writeError(w, 503, "public_url_required", err.Error())
		return
	}
	order, err := s.store.CreateResourceOrder(currentUser(r).ID, resourceID)
	if err != nil {
		writeError(w, 400, "order_create_failed", err.Error())
		return
	}
	notifyURL := config.NotifyURL
	if notifyURL == "" {
		notifyURL = base + "/api/v1/payment/callback"
	}
	returnURL := config.ReturnURL
	if returnURL == "" {
		returnURL = base + "/resources"
	}
	order.PaymentURL, err = payment.BuildEPayURL(payment.EPayInput{GatewayURL: config.GatewayURL, MerchantID: config.MerchantID, Secret: config.Secret, PayType: config.PayType, OrderNo: order.OrderNo, Name: order.ResourceTitle, AmountCents: order.AmountCents, NotifyURL: notifyURL, ReturnURL: returnURL})
	if err != nil {
		if cancelErr := s.store.CancelPendingOrder(order.ID, currentUser(r).ID); cancelErr != nil {
			s.logger.Error("failed to cancel order after payment URL generation error", "order_id", order.ID, "error", cancelErr)
		}
		writeError(w, 500, "payment_url_failed", err.Error())
		return
	}
	writeJSON(w, 201, order)
}

func (s *Server) listMyOrders(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListCommerceOrders(currentUser(r).ID)
	if err != nil {
		writeError(w, 500, "orders_failed", "订单加载失败")
		return
	}
	writeJSON(w, 200, items)
}

func (s *Server) creatorBalance(w http.ResponseWriter, r *http.Request) {
	balance, err := s.store.CreatorBalance(currentUser(r).ID)
	if err != nil {
		writeError(w, 500, "creator_balance_failed", "创作者余额加载失败")
		return
	}
	writeJSON(w, 200, map[string]any{"available_cents": balance, "platform_fee_percent": 10})
}

func (s *Server) requestCreatorPayout(w http.ResponseWriter, r *http.Request) {
	var input struct {
		AmountCents   int64  `json:"amount_cents"`
		PayoutMethod  string `json:"payout_method"`
		PayoutAccount string `json:"payout_account"`
		AccountName   string `json:"account_name"`
		Note          string `json:"note"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.CreateCreatorPayout(currentUser(r).ID, input.AmountCents, input.PayoutMethod, input.PayoutAccount, input.AccountName, input.Note)
	if err != nil {
		writeError(w, 400, "payout_create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) listCreatorPayouts(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListCreatorPayouts(currentUser(r).ID)
	if err != nil {
		writeError(w, 500, "payouts_failed", "提现记录加载失败")
		return
	}
	writeJSON(w, 200, items)
}

func (s *Server) listAdminPayouts(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListAdminCreatorPayouts(r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, 400, "payouts_failed", err.Error())
		return
	}
	writeJSON(w, 200, items)
}

func (s *Server) reviewAdminPayout(w http.ResponseWriter, r *http.Request) {
	payoutID, _ := strconv.ParseInt(chi.URLParam(r, "payoutID"), 10, 64)
	var input struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.ReviewCreatorPayout(currentUser(r).ID, payoutID, input.Status, input.Note)
	if err != nil {
		writeError(w, 400, "payout_review_failed", err.Error())
		return
	}
	writeJSON(w, 200, item)
}

func (s *Server) paymentCallback(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "fail", http.StatusBadRequest)
		return
	}
	config, err := s.store.PaymentDeliveryConfig()
	if err != nil || !config.Enabled || !payment.VerifyCallback(r.Form, config.Secret) || strings.TrimSpace(r.Form.Get("pid")) != config.MerchantID {
		http.Error(w, "fail", http.StatusUnauthorized)
		return
	}
	if !payment.PaidStatus(r.Form.Get("trade_status")) {
		http.Error(w, "fail", http.StatusBadRequest)
		return
	}
	orderNo := payment.OrderNo(r.Form)
	amountCents, err := parseMoneyCents(r.Form.Get("money"))
	if err != nil {
		http.Error(w, "fail", http.StatusBadRequest)
		return
	}
	tradeNo := strings.TrimSpace(r.Form.Get("trade_no"))
	if _, _, err := s.store.CompleteResourceOrder(orderNo, tradeNo, amountCents); err != nil {
		http.Error(w, "fail", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("success"))
}

func (s *Server) publicBaseURL() (string, error) {
	settings, err := s.store.GetSiteSettings()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(settings.PublicURL) != "" {
		return strings.TrimRight(settings.PublicURL, "/"), nil
	}
	if s.publicURL != "" {
		return s.publicURL, nil
	}
	return "", errors.New("public URL must be configured before creating payment orders")
}

func parseMoneyCents(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" || strings.HasPrefix(value, "-") {
		return 0, errors.New("invalid money")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, errors.New("invalid money")
	}
	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, errors.New("invalid money")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 2 {
		return 0, errors.New("invalid money")
	}
	fraction += strings.Repeat("0", 2-len(fraction))
	cents := int64(0)
	if fraction != "" {
		cents, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, errors.New("invalid money")
		}
	}
	if yuan > (1<<63-1-cents)/100 {
		return 0, errors.New("money is too large")
	}
	return yuan*100 + cents, nil
}
