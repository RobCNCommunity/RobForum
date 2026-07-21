package payment

import (
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

type EPayInput struct {
	GatewayURL  string
	MerchantID  string
	Secret      string
	PayType     string
	OrderNo     string
	Name        string
	AmountCents int64
	NotifyURL   string
	ReturnURL   string
}

func BuildEPayURL(input EPayInput) (string, error) {
	gateway := strings.TrimSpace(input.GatewayURL)
	parsed, err := url.Parse(gateway)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("payment gateway URL is invalid")
	}
	if parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("payment gateway URL must use HTTP or HTTPS")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("payment gateway URL must not contain a query or fragment")
	}
	if input.AmountCents <= 0 {
		return "", fmt.Errorf("payment amount is invalid")
	}
	if strings.TrimSpace(input.MerchantID) == "" || strings.TrimSpace(input.Secret) == "" || strings.TrimSpace(input.OrderNo) == "" || strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.NotifyURL) == "" || strings.TrimSpace(input.ReturnURL) == "" {
		return "", fmt.Errorf("payment request is incomplete")
	}
	if !strings.HasSuffix(parsed.Path, "/submit.php") {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/submit.php"
	}
	values := map[string]string{
		"pid":          strings.TrimSpace(input.MerchantID),
		"type":         defaultPayType(input.PayType),
		"out_trade_no": strings.TrimSpace(input.OrderNo),
		"notify_url":   strings.TrimSpace(input.NotifyURL),
		"return_url":   strings.TrimSpace(input.ReturnURL),
		"name":         strings.TrimSpace(input.Name),
		"money":        fmt.Sprintf("%d.%02d", input.AmountCents/100, input.AmountCents%100),
	}
	query := parsed.Query()
	for key, value := range values {
		query.Set(key, value)
	}
	query.Set("sign", Sign(values, input.Secret))
	query.Set("sign_type", "MD5")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func VerifyCallback(values url.Values, secret string) bool {
	sign := strings.ToLower(strings.TrimSpace(values.Get("sign")))
	if sign == "" {
		return false
	}
	params := make(map[string]string)
	for key, list := range values {
		if key == "sign" || key == "sign_type" || len(list) == 0 || strings.TrimSpace(list[0]) == "" {
			continue
		}
		params[key] = list[0]
	}
	return subtleCompare(sign, Sign(params, secret))
}

func Sign(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for key, value := range params {
		if key != "sign" && key != "sign_type" && strings.TrimSpace(value) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+params[key])
	}
	sum := md5.Sum([]byte(strings.Join(parts, "&") + strings.TrimSpace(secret)))
	return hex.EncodeToString(sum[:])
}

func PaidStatus(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "trade_success" || value == "trade_finished" || value == "paid" || value == "success"
}
func OrderNo(values url.Values) string {
	return strings.TrimSpace(values.Get("out_trade_no"))
}
func defaultPayType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "alipay", "wxpay", "qqpay", "bank":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "alipay"
	}
}
func subtleCompare(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
