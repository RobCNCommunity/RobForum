package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"roblox-community/internal/domain"
)

func TestValidateGT4(t *testing.T) {
	const secret = "test-secret"
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("captcha_id") != "captcha-id" {
			t.Error("captcha ID query is missing")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatal(err)
		}
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte("lot"))
		if form.Get("sign_token") != hex.EncodeToString(mac.Sum(nil)) {
			t.Error("invalid GT4 signature")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"result":"success"}`)
	}))
	defer provider.Close()

	config := domain.CaptchaConfig{Provider: "gt4", SiteKey: "captcha-id", Secret: secret, Endpoint: provider.URL}
	token := `{"captcha_id":"captcha-id","lot_number":"lot","captcha_output":"output","pass_token":"pass","gen_time":"time"}`
	if err := validateGT4(context.Background(), provider.Client(), config, token); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
}

func TestValidateGT4RejectsInvalidInput(t *testing.T) {
	config := domain.CaptchaConfig{SiteKey: "id", Secret: "secret", Endpoint: "https://example.com"}
	if err := validateGT4(context.Background(), http.DefaultClient, config, `{}`); err == nil || !strings.Contains(err.Error(), "fields") {
		t.Fatalf("invalid token accepted: %v", err)
	}
	token := `{"captcha_id":"other-id","lot_number":"lot","captcha_output":"output","pass_token":"pass","gen_time":"time"}`
	if err := validateGT4(context.Background(), http.DefaultClient, config, token); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched captcha ID accepted: %v", err)
	}
}

func TestRegistrationCaptchaRequirement(t *testing.T) {
	if !registrationCaptchaRequired(domain.SiteSettings{RequireEmailVerification: false}) {
		t.Fatal("registration without email verification must require CAPTCHA")
	}
	if registrationCaptchaRequired(domain.SiteSettings{RequireEmailVerification: true}) {
		t.Fatal("email-code registration must not require a second CAPTCHA")
	}
}
