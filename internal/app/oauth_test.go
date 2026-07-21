package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"roblox-community/internal/domain"
)

func TestSafeOAuthReturnTo(t *testing.T) {
	tests := map[string]string{
		"":                         "/",
		"/posts/12?from=oauth":     "/posts/12?from=oauth",
		"//attacker.example/path":  "/",
		"https://attacker.example": "/",
		"/\\attacker.example":      "/",
		"/ok\r\nLocation: bad":     "/",
	}
	for input, expected := range tests {
		if actual := safeOAuthReturnTo(input); actual != expected {
			t.Fatalf("safeOAuthReturnTo(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestExchangeOAuthCodeAndFetchIdentity(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse token form: %v", err)
			}
			if r.Form.Get("code") != "authorization-code" || r.Form.Get("code_verifier") != "pkce-verifier" {
				t.Errorf("token form did not preserve code and PKCE verifier: %#v", r.Form)
			}
			if r.Form.Get("client_secret") != "client-secret" {
				t.Errorf("client secret post authentication is missing")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"Bearer"}`))
		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer access-token" {
				t.Errorf("userinfo bearer token is missing")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"sub":"subject-1","email":"USER@example.com","email_verified":true,"preferred_username":"Builder","picture":"https://cdn.example.com/avatar.png"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	config := domain.OAuthConfig{
		ClientID:        "client-id",
		ClientSecret:    "client-secret",
		TokenURL:        server.URL + "/token",
		UserInfoURL:     server.URL + "/userinfo",
		TokenAuthMethod: "client_secret_post",
	}
	token, err := exchangeOAuthCode(config, "authorization-code", "pkce-verifier", "https://community.example.com/api/v1/oauth/callback")
	if err != nil {
		t.Fatalf("exchange OAuth code: %v", err)
	}
	identity, err := fetchOAuthIdentity(config, token)
	if err != nil {
		t.Fatalf("fetch OAuth identity: %v", err)
	}
	if identity.Subject != "subject-1" || identity.Email != "user@example.com" || !identity.EmailVerified || identity.Name != "Builder" || identity.AvatarURL == "" {
		t.Fatalf("unexpected OAuth identity: %#v", identity)
	}
}

func TestOAuthIdentityRequiresSubjectAndEmail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"Missing identity"}`))
	}))
	defer server.Close()
	_, err := fetchOAuthIdentity(domain.OAuthConfig{UserInfoURL: server.URL}, "token")
	if err == nil {
		t.Fatal("userinfo without subject and email was accepted")
	}
}
