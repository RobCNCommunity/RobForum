package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"roblox-community/internal/domain"
)

func TestRequestLimiter(t *testing.T) {
	limiter := newRequestLimiter()
	now := time.Unix(100, 0)
	for i := 0; i < 2; i++ {
		if allowed, _ := limiter.allow("client", 2, time.Minute, now); !allowed {
			t.Fatal("request was limited too early")
		}
	}
	if allowed, retry := limiter.allow("client", 2, time.Minute, now); allowed || retry <= 0 {
		t.Fatal("request limit was not enforced")
	}
	if allowed, _ := limiter.allow("client", 2, time.Minute, now.Add(time.Minute)); !allowed {
		t.Fatal("request limit did not reset")
	}
}

func TestRateLimit_keeps_chat_stream_available_when_API_bucket_is_exhausted(t *testing.T) {
	// Given
	server := &Server{limiter: newRequestLimiter()}
	handler := server.rateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for index := 0; index < 360; index++ {
		request := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/posts", nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("generic API request %d was limited early: %d", index+1, recorder.Code)
		}
	}

	// When
	request := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/conversations/42/stream", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	// Then
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("chat stream shared the exhausted generic API bucket: %d", recorder.Code)
	}
}

func TestRequestClientIP_requires_explicit_trusted_proxy(t *testing.T) {
	// Given
	request := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.10")

	// When
	got := requestClientIP(request, nil)

	// Then
	if got != "127.0.0.1" {
		t.Fatalf("forwarding header was trusted without configuration: %q", got)
	}
}

func TestRequestClientIP_uses_rightmost_untrusted_forwarded_hop(t *testing.T) {
	// Given
	trusted, err := parseTrustedProxyCIDRs("127.0.0.1/32, 10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.99, 203.0.113.10, 10.1.2.3")

	// When
	got := requestClientIP(request, trusted)

	// Then
	if got != "203.0.113.10" {
		t.Fatalf("spoofed leftmost hop was accepted: %q", got)
	}
}

func TestRequestScheme_only_trusts_configured_proxy(t *testing.T) {
	// Given
	request := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Forwarded-Proto", "https")

	// When / Then
	if got := requestScheme(request, nil); got != "http" {
		t.Fatalf("forwarded scheme was trusted without configuration: %q", got)
	}
	trusted, err := parseTrustedProxyCIDRs("127.0.0.1/32")
	if err != nil {
		t.Fatal(err)
	}
	if got := requestScheme(request, trusted); got != "https" {
		t.Fatalf("configured proxy scheme was rejected: %q", got)
	}
	if _, err := parseTrustedProxyCIDRs("127.0.0.1/32,not-a-cidr"); err == nil {
		t.Fatal("invalid trusted proxy configuration was accepted")
	}
}

func TestSameOriginRequest(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "https://community.example.com/api/v1/posts", nil)
	request.Host = "community.example.com"
	request.Header.Set("Origin", "https://community.example.com")
	present, valid := sameOriginRequest(request, "https://community.example.com", nil)
	if !present || !valid {
		t.Fatal("same-origin request rejected")
	}
	request.Header.Set("Origin", "https://evil.example")
	if _, valid := sameOriginRequest(request, "https://community.example.com", nil); valid {
		t.Fatal("cross-origin request accepted")
	}
}

func TestCSRFProtection(t *testing.T) {
	server := &Server{publicURL: "https://community.example.com", limiter: newRequestLimiter()}
	handler := server.csrfProtection(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))

	valid := httptest.NewRequest(http.MethodPost, "https://community.example.com/api/v1/posts", strings.NewReader(`{}`))
	valid.Host = "community.example.com"
	valid.Header.Set("Origin", "https://community.example.com")
	valid.Header.Set("X-CSRF-Token", "token")
	valid.AddCookie(&http.Cookie{Name: "roblox_session", Value: "session"})
	valid.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "token"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, valid)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("valid CSRF token rejected: %d", recorder.Code)
	}

	duplicate := httptest.NewRequest(http.MethodPost, "https://community.example.com/api/v1/posts", strings.NewReader(`{}`))
	duplicate.Host = "community.example.com"
	duplicate.Header.Set("Origin", "https://community.example.com")
	duplicate.Header.Set("X-CSRF-Token", "current-token")
	duplicate.Header.Add("Cookie", csrfCookieName+"=stale-domain-token; "+csrfCookieName+"=current-token")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, duplicate)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("matching CSRF token among duplicate cookies rejected: %d", recorder.Code)
	}

	invalid := httptest.NewRequest(http.MethodPost, "https://community.example.com/api/v1/posts", strings.NewReader(`{}`))
	invalid.Host = "community.example.com"
	invalid.Header.Set("Origin", "https://evil.example")
	invalid.AddCookie(&http.Cookie{Name: "roblox_session", Value: "session"})
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, invalid)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("cross-origin mutation accepted: %d", recorder.Code)
	}

	missingToken := httptest.NewRequest(http.MethodPost, "https://community.example.com/api/v1/auth/login", strings.NewReader(`{}`))
	missingToken.Host = "community.example.com"
	missingToken.Header.Set("Origin", "https://community.example.com")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, missingToken)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("authentication mutation without CSRF token accepted: %d", recorder.Code)
	}

	bootstrap := httptest.NewRequest(http.MethodGet, "https://community.example.com/login", nil)
	bootstrap.Host = "community.example.com"
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, bootstrap)
	if len(recorder.Result().Cookies()) == 0 || recorder.Result().Cookies()[0].Name != csrfCookieName {
		t.Fatal("GET request did not bootstrap a CSRF cookie")
	}
}

func TestResolveSessionUserSkipsStaleDuplicateCookie(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "https://community.example.com/me", nil)
	request.Header.Add("Cookie", "roblox_session=stale-token; roblox_session=current-token")
	want := domain.User{ID: 42, DisplayName: "tester"}
	user, token, err := resolveSessionUser(request, func(candidate string) (domain.User, error) {
		if candidate == "current-token" {
			return want, nil
		}
		return domain.User{}, errors.New("expired")
	})
	if err != nil {
		t.Fatalf("valid duplicate session was rejected: %v", err)
	}
	if token != "current-token" || user.ID != want.ID {
		t.Fatalf("unexpected resolved session: token=%q user=%+v", token, user)
	}
}
