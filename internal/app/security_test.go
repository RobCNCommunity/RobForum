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

func TestRequestClientIPOnlyTrustsLoopbackProxy(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	request.RemoteAddr = "198.51.100.20:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.10")
	if got := requestClientIP(request); got != "198.51.100.20" {
		t.Fatalf("untrusted forwarding header accepted: %q", got)
	}
	request.RemoteAddr = "127.0.0.1:1234"
	if got := requestClientIP(request); got != "203.0.113.10" {
		t.Fatalf("trusted local proxy header rejected: %q", got)
	}
}

func TestSameOriginRequest(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "https://community.example.com/api/v1/posts", nil)
	request.Host = "community.example.com"
	request.Header.Set("Origin", "https://community.example.com")
	present, valid := sameOriginRequest(request, "https://community.example.com")
	if !present || !valid {
		t.Fatal("same-origin request rejected")
	}
	request.Header.Set("Origin", "https://evil.example")
	if _, valid := sameOriginRequest(request, "https://community.example.com"); valid {
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
