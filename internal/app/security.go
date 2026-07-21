package app

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const csrfCookieName = "roblox_csrf"

type rateEntry struct {
	started time.Time
	count   int
}

type requestLimiter struct {
	mu          sync.Mutex
	entries     map[string]rateEntry
	lastCleanup time.Time
}

func newRequestLimiter() *requestLimiter {
	return &requestLimiter{entries: make(map[string]rateEntry), lastCleanup: time.Now()}
}

func (l *requestLimiter) allow(key string, limit int, window time.Duration, now time.Time) (bool, time.Duration) {
	if l == nil || limit < 1 || window <= 0 {
		return true, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastCleanup) >= time.Minute || len(l.entries) > 10000 {
		for entryKey, entry := range l.entries {
			if now.Sub(entry.started) >= window || len(l.entries) > 9000 {
				delete(l.entries, entryKey)
			}
		}
		l.lastCleanup = now
	}
	entry, exists := l.entries[key]
	if !exists || now.Sub(entry.started) >= window {
		l.entries[key] = rateEntry{started: now, count: 1}
		return true, 0
	}
	if entry.count >= limit {
		return false, window - now.Sub(entry.started)
	}
	entry.count++
	l.entries[key] = entry
	return true, 0
}

func (s *Server) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		limit, window, scope := 360, time.Minute, "api"
		path := r.URL.Path
		switch {
		case path == "/api/v1/auth/login":
			limit, window, scope = 10, 10*time.Minute, "login"
		case path == "/api/v1/auth/register":
			limit, window, scope = 5, 10*time.Minute, "register"
		case path == "/api/v1/auth/register/verification":
			limit, window, scope = 5, 10*time.Minute, "register-verification"
		case path == "/api/v1/auth/forgot-password":
			limit, window, scope = 5, 15*time.Minute, "forgot-password"
		case path == "/api/v1/auth/reset-password":
			limit, window, scope = 10, 15*time.Minute, "reset-password"
		case path == "/api/v1/oauth/start" || path == "/api/v1/oauth/callback":
			limit, window, scope = 20, 10*time.Minute, "oauth"
		case path == "/api/v1/payment/callback":
			limit, window, scope = 240, time.Minute, "payment-callback"
		case r.Method == http.MethodPost && path == "/api/v1/resources":
			limit, window, scope = 12, time.Hour, "resource-upload"
		case r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions:
			limit, window, scope = 120, time.Minute, "write"
		}
		key := scope + "|" + requestClientIP(r)
		if token := sessionToken(r); token != "" && scope == "write" {
			hash := sha256.Sum256([]byte(token))
			key += "|" + hex.EncodeToString(hash[:6])
		}
		allowed, retry := s.limiter.allow(key, limit, window, time.Now())
		if !allowed {
			seconds := int(retry.Seconds())
			if seconds < 1 {
				seconds = 1
			}
			w.Header().Set("Retry-After", strconvItoa(seconds))
			writeError(w, http.StatusTooManyRequests, "rate_limited", "请求过于频繁，请稍后再试")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestClientIP(r *http.Request) string {
	remote := strings.TrimSpace(r.RemoteAddr)
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	if parsed := net.ParseIP(host); parsed != nil && parsed.IsLoopback() {
		if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); net.ParseIP(forwarded) != nil {
			return forwarded
		}
	}
	if net.ParseIP(host) != nil {
		return host
	}
	return "unknown"
}

func remotePeerIsLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}
	parsed := net.ParseIP(host)
	return parsed != nil && parsed.IsLoopback()
}

func (s *Server) csrfProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			if r.Method != http.MethodOptions {
				if _, err := r.Cookie(csrfCookieName); err != nil {
					s.setCSRFCookie(w, r)
				}
			}
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/v1/payment/callback" {
			next.ServeHTTP(w, r)
			return
		}
		originPresent, originOK := sameOriginRequest(r, s.publicURL)
		if originPresent && !originOK {
			writeError(w, http.StatusForbidden, "csrf_origin_invalid", "请求来源无效")
			return
		}
		header := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
		cookieValues := make([]string, 0, 1)
		for _, cookie := range r.Cookies() {
			if cookie.Name == csrfCookieName && cookie.Value != "" {
				cookieValues = append(cookieValues, cookie.Value)
			}
		}
		if len(cookieValues) == 0 || header == "" {
			writeError(w, http.StatusForbidden, "csrf_token_required", "缺少 CSRF 令牌")
			return
		}
		matched := false
		for _, cookieValue := range cookieValues {
			matched = subtle.ConstantTimeCompare([]byte(cookieValue), []byte(header)) == 1 || matched
		}
		if !matched {
			writeError(w, http.StatusForbidden, "csrf_token_invalid", "CSRF 令牌无效")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sameOriginRequest(r *http.Request, publicURL string) (bool, bool) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		origin = refererOrigin(r.Header.Get("Referer"))
	}
	if origin == "" {
		return false, true
	}
	if origin == "null" {
		return true, false
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return true, false
	}
	if strings.EqualFold(parsed.Host, r.Host) {
		return true, strings.EqualFold(parsed.Scheme, requestScheme(r))
	}
	for _, candidate := range []string{"http://127.0.0.1:5173", "http://localhost:5173", strings.TrimRight(publicURL, "/")} {
		if candidate == "" {
			continue
		}
		if expected, parseErr := url.Parse(candidate); parseErr == nil && strings.EqualFold(expected.Scheme, parsed.Scheme) && strings.EqualFold(expected.Host, parsed.Host) {
			return true, true
		}
	}
	return true, false
}

func refererOrigin(referer string) string {
	parsed, err := url.Parse(strings.TrimSpace(referer))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func requestScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if remotePeerIsLoopback(r) && strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		return "https"
	}
	return "http"
}

func (s *Server) setCSRFCookie(w http.ResponseWriter, r *http.Request) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return
	}
	value := base64.RawURLEncoding.EncodeToString(raw)
	http.SetCookie(w, &http.Cookie{Name: csrfCookieName, Value: value, Path: "/", MaxAge: 30 * 24 * 3600, Secure: s.secureCookies(r), SameSite: http.SameSiteLaxMode})
}

func (s *Server) secureCookies(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if remotePeerIsLoopback(r) && strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		return true
	}
	if parsed, err := url.Parse(strings.TrimSpace(s.publicURL)); err == nil && strings.EqualFold(parsed.Scheme, "https") {
		return true
	}
	return false
}

func strconvItoa(value int) string {
	if value <= 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[i:])
}
