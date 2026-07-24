package contentmoderation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestBaiduReviewImagePostsBase64(t *testing.T) {
	imageBytes := []byte("safe-test-image-bytes")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "image-token", "expires_in": 3600})
		case "/image":
			if r.URL.Query().Get("access_token") != "image-token" {
				t.Fatal("image request did not use OAuth token")
			}
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			decoded, err := base64.StdEncoding.DecodeString(r.Form.Get("image"))
			if err != nil || string(decoded) != string(imageBytes) {
				t.Fatalf("unexpected image payload: %q, %v", decoded, err)
			}
			if r.Form.Get("strategyId") != "image-strategy" {
				t.Fatalf("unexpected image strategy: %q", r.Form.Get("strategyId"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"conclusionType": 1})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewBaidu(BaiduConfig{
		ImageEnabled:    true,
		APIKey:          "test-ak",
		SecretKey:       "test-sk",
		AuthURL:         server.URL + "/oauth/token",
		ImageEndpoint:   server.URL + "/image",
		ImageStrategyID: "image-strategy",
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.Enabled() || !client.ImageEnabled() {
		t.Fatalf("text/image enablement was not independent")
	}
	result, err := client.ReviewImage(context.Background(), "post", imageBytes)
	if err != nil || result.Violation || result.Model != baiduImageModel {
		t.Fatalf("unexpected image result: %#v, %v", result, err)
	}
}

func TestBaiduReviewGetsTokenAndPostsForm(t *testing.T) {
	var tokenCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			tokenCalls.Add(1)
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("grant_type") != "client_credentials" || r.Form.Get("client_id") != "test-ak" || r.Form.Get("client_secret") != "test-sk" {
				t.Fatalf("unexpected token form: %#v", r.Form)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token-one", "expires_in": 3600})
		case "/text":
			if r.URL.Query().Get("access_token") != "token-one" {
				t.Fatalf("unexpected token query")
			}
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("text") != "普通内容" || r.Form.Get("strategyId") != "strict-strategy" {
				t.Fatalf("unexpected censor form: %#v", r.Form)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"conclusion": "合规", "conclusionType": 1})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewBaidu(BaiduConfig{
		Enabled:    true,
		APIKey:     "test-ak",
		SecretKey:  "test-sk",
		AuthURL:    server.URL + "/oauth/token",
		Endpoint:   server.URL + "/text",
		StrategyID: "strict-strategy",
	})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		result, err := client.Review(context.Background(), "message", "普通内容")
		if err != nil || result.Violation || result.Model != "baidu" {
			t.Fatalf("unexpected result: %#v, %v", result, err)
		}
	}
	if tokenCalls.Load() != 1 {
		t.Fatalf("token was not cached: %d calls", tokenCalls.Load())
	}
}

func TestBaiduReviewStrictDecisionMapping(t *testing.T) {
	for conclusionType, wantViolation := range map[int]bool{1: false, 2: true, 3: true} {
		body, _ := json.Marshal(map[string]any{"conclusionType": conclusionType})
		result, err := parseBaiduDecision(body)
		if err != nil || result.Violation != wantViolation {
			t.Fatalf("type %d: result=%#v err=%v", conclusionType, result, err)
		}
		if wantViolation && strings.TrimSpace(result.Reason) == "" {
			t.Fatalf("type %d did not include a safe reason", conclusionType)
		}
	}
	nested, _ := json.Marshal(map[string]any{"data": []any{map[string]any{"conclusionType": 1}}})
	if result, err := parseBaiduDecision(nested); err != nil || result.Violation {
		t.Fatalf("nested Baidu decision was not accepted: %#v, %v", result, err)
	}
	for _, payload := range [][]byte{
		[]byte(`{"conclusionType":4}`),
		[]byte(`{"error_code":18,"error_msg":"quota"}`),
		[]byte(`{"conclusion":"unknown"}`),
		[]byte(`not-json`),
	} {
		if _, err := parseBaiduDecision(payload); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("expected fail-closed error for %q, got %v", payload, err)
		}
	}
}

func TestBaiduRetriesTransientTokenAndModerationFailures(t *testing.T) {
	var tokenCalls atomic.Int32
	var censorCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			if tokenCalls.Add(1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "retry-token", "expires_in": 3600})
		case "/text":
			if censorCalls.Add(1) == 1 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"conclusionType": 1})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewBaidu(BaiduConfig{Enabled: true, APIKey: "ak", SecretKey: "sk", AuthURL: server.URL + "/oauth/token", Endpoint: server.URL + "/text"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Review(context.Background(), "post", "普通内容")
	if err != nil || result.Violation {
		t.Fatalf("transient failures were not recovered: %#v, %v", result, err)
	}
	if tokenCalls.Load() != 2 || censorCalls.Load() != 2 {
		t.Fatalf("unexpected retries: token=%d censor=%d", tokenCalls.Load(), censorCalls.Load())
	}
}

func TestBaiduReviewRefreshesRejectedToken(t *testing.T) {
	var tokenCalls atomic.Int32
	var censorCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			call := tokenCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token-" + string(rune('0'+call)), "expires_in": 3600})
		case "/text":
			call := censorCalls.Add(1)
			if call == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if r.URL.Query().Get("access_token") != "token-2" {
				t.Fatalf("refreshed token was not used")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"conclusionType": 1})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewBaidu(BaiduConfig{Enabled: true, APIKey: "ak", SecretKey: "sk", AuthURL: server.URL + "/oauth/token", Endpoint: server.URL + "/text"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Review(context.Background(), "post", "普通内容"); err != nil {
		t.Fatal(err)
	}
	if tokenCalls.Load() != 2 || censorCalls.Load() != 2 {
		t.Fatalf("unexpected retries: token=%d censor=%d", tokenCalls.Load(), censorCalls.Load())
	}
}

func TestBaiduReviewBlocksLocalPolicyBeforeNetwork(t *testing.T) {
	client, err := NewBaidu(BaiduConfig{Enabled: true, APIKey: "ak", SecretKey: "sk", AuthURL: "https://example.test/token", Endpoint: "https://example.test/text"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Review(context.Background(), "message", "1 9 8 9")
	if err != nil || !result.Violation || result.Model != "local_policy" {
		t.Fatalf("unexpected result: %#v, %v", result, err)
	}
}

func TestBaiduRejectsInvalidConfiguration(t *testing.T) {
	if _, err := NewBaidu(BaiduConfig{Enabled: true, APIKey: "", SecretKey: "sk"}); err == nil {
		t.Fatal("expected missing API key error")
	}
	if _, err := NewBaidu(BaiduConfig{Enabled: true, APIKey: "ak", SecretKey: "sk", Endpoint: "http://example.com/text"}); err == nil {
		t.Fatal("expected HTTPS validation error")
	}
	if _, err := url.Parse(defaultBaiduTextURL); err != nil {
		t.Fatal(err)
	}
}

func TestBaiduImageEnablementInheritsTextSetting(t *testing.T) {
	t.Setenv("ROBLOX_BAIDU_CONTENT_MODERATION_ENABLED", "true")
	t.Setenv("ROBLOX_BAIDU_IMAGE_MODERATION_ENABLED", "")
	t.Setenv("ROBLOX_BAIDU_CONTENT_MODERATION_API_KEY", "ak")
	t.Setenv("ROBLOX_BAIDU_CONTENT_MODERATION_SECRET_KEY", "sk")
	t.Setenv("ROBLOX_BAIDU_CONTENT_MODERATION_AUTH_URL", "https://example.test/token")
	t.Setenv("ROBLOX_BAIDU_CONTENT_MODERATION_URL", "https://example.test/text")
	t.Setenv("ROBLOX_BAIDU_IMAGE_MODERATION_URL", "https://example.test/image")
	client, err := BaiduFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !client.Enabled() || !client.ImageEnabled() {
		t.Fatal("image moderation did not inherit enabled text moderation")
	}
}
