package payment

import (
	"net/url"
	"testing"
)

func TestEPaySignatureAndURL(t *testing.T) {
	params := map[string]string{"pid": "1001", "money": "9.90", "out_trade_no": "RBC-1"}
	sign := Sign(params, "secret")
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	values.Set("sign", sign)
	values.Set("sign_type", "MD5")
	if !VerifyCallback(values, "secret") {
		t.Fatal("valid callback signature rejected")
	}
	values.Set("money", "99.90")
	if VerifyCallback(values, "secret") {
		t.Fatal("tampered callback accepted")
	}
	link, err := BuildEPayURL(EPayInput{GatewayURL: "https://pay.example.com", MerchantID: "1001", Secret: "secret", OrderNo: "RBC-1", Name: "Resource", AmountCents: 99, NotifyURL: "https://community.example.com/api/v1/payment/callback", ReturnURL: "https://community.example.com/resources"})
	if err != nil || link == "" {
		t.Fatalf("BuildEPayURL() = %q, %v", link, err)
	}
	if OrderNo(url.Values{"trade_no": {"provider-only"}}) != "" {
		t.Fatal("provider trade number was accepted as a local order number")
	}
	if _, err := BuildEPayURL(EPayInput{GatewayURL: "https://pay.example.com?unsafe=1", MerchantID: "1001", Secret: "secret", OrderNo: "RBC-1", Name: "Resource", AmountCents: 99, NotifyURL: "https://community.example.com/api/v1/payment/callback", ReturnURL: "https://community.example.com/resources"}); err == nil {
		t.Fatal("gateway URL with unsigned query parameters was accepted")
	}
}
