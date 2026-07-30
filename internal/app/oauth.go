package app

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"roblox-community/internal/domain"
	"roblox-community/internal/store"
)

const oauthStateCookieName = "roblox_oauth_state"

type oauthIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	AvatarURL     string
}

func (s *Server) publicOAuthConfig(w http.ResponseWriter, _ *http.Request) {
	configs, err := s.store.ListOAuthConfigs(true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "oauth_config_unavailable", "OAuth 登录配置暂不可用")
		return
	}
	publicConfigs := make([]map[string]any, 0, len(configs))
	for _, config := range configs {
		publicConfigs = append(publicConfigs, map[string]any{"id": config.ID, "enabled": true, "provider_key": config.ProviderKey, "provider_name": config.ProviderName})
	}
	writeJSON(w, http.StatusOK, publicConfigs)
}

func (s *Server) adminOAuthConfig(w http.ResponseWriter, _ *http.Request) {
	config, err := s.store.ListOAuthConfigs(false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "oauth_config_failed", "OAuth 配置读取失败")
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (s *Server) createOAuthConfig(w http.ResponseWriter, r *http.Request) {
	var input domain.OAuthConfig
	if !decodeJSON(w, r, &input) {
		return
	}
	config, err := s.store.CreateOAuthConfig(currentUser(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "oauth_config_invalid", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, config)
}

func (s *Server) updateOAuthConfig(w http.ResponseWriter, r *http.Request) {
	var input domain.OAuthConfig
	if !decodeJSON(w, r, &input) {
		return
	}
	input.ID, _ = strconv.ParseInt(chi.URLParam(r, "providerID"), 10, 64)
	config, err := s.store.UpdateOAuthConfig(currentUser(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "oauth_config_invalid", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (s *Server) deleteOAuthConfig(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "providerID"), 10, 64)
	if id < 1 || s.store.DeleteOAuthConfig(currentUser(r).ID, id) != nil {
		writeError(w, http.StatusBadRequest, "oauth_delete_failed", "OAuth 提供商删除失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) oauthStart(w http.ResponseWriter, r *http.Request) {
	providerKey := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider")))
	config, err := s.store.OAuthDeliveryConfig(providerKey)
	if err != nil || !config.Enabled {
		s.redirectOAuthFailure(w, r, "unavailable")
		return
	}
	state, err := randomURLToken(32)
	if err != nil {
		s.redirectOAuthFailure(w, r, "state")
		return
	}
	verifier, err := randomURLToken(48)
	if err != nil {
		s.redirectOAuthFailure(w, r, "state")
		return
	}
	returnTo := safeOAuthReturnTo(r.URL.Query().Get("return_to"))
	stateHash := sha256.Sum256([]byte(state))
	if err := s.store.CreateOAuthLoginState(stateHash, verifier, config.ProviderKey, returnTo, time.Now().UTC().Add(10*time.Minute)); err != nil {
		s.logger.Error("create OAuth state failed", "error", err)
		s.redirectOAuthFailure(w, r, "state")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		Path:     "/api/v1/oauth",
		MaxAge:   10 * 60,
		HttpOnly: true,
		Secure:   s.secureCookies(r),
		SameSite: http.SameSiteLaxMode,
	})
	authorizationURL, err := url.Parse(config.AuthorizationURL)
	if err != nil {
		s.redirectOAuthFailure(w, r, "configuration")
		return
	}
	challengeHash := sha256.Sum256([]byte(verifier))
	query := authorizationURL.Query()
	query.Set("response_type", "code")
	query.Set("client_id", config.ClientID)
	query.Set("redirect_uri", s.oauthCallbackURL(r))
	query.Set("scope", config.Scopes)
	query.Set("state", state)
	query.Set("code_challenge", base64.RawURLEncoding.EncodeToString(challengeHash[:]))
	query.Set("code_challenge_method", "S256")
	authorizationURL.RawQuery = query.Encode()
	http.Redirect(w, r, authorizationURL.String(), http.StatusSeeOther)
}

func (s *Server) oauthCallback(w http.ResponseWriter, r *http.Request) {
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	cookie, _ := r.Cookie(oauthStateCookieName)
	s.clearOAuthStateCookie(w, r)
	if state == "" || cookie == nil || len(state) != len(cookie.Value) || subtle.ConstantTimeCompare([]byte(state), []byte(cookie.Value)) != 1 {
		s.redirectOAuthFailure(w, r, "state")
		return
	}
	loginState, err := s.store.ConsumeOAuthLoginState(state)
	if err != nil {
		s.redirectOAuthFailure(w, r, "state")
		return
	}
	if providerError := strings.TrimSpace(r.URL.Query().Get("error")); providerError != "" {
		s.redirectOAuthFailure(w, r, "denied")
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" || len(code) > 4096 {
		s.redirectOAuthFailure(w, r, "code")
		return
	}
	config, err := s.store.OAuthDeliveryConfig(loginState.ProviderKey)
	if err != nil || !config.Enabled {
		s.redirectOAuthFailure(w, r, "configuration")
		return
	}
	accessToken, err := exchangeOAuthCode(config, code, loginState.CodeVerifier, s.oauthCallbackURL(r))
	if err != nil {
		s.logger.Warn("OAuth token exchange failed", "error", err)
		s.redirectOAuthFailure(w, r, "exchange")
		return
	}
	identity, err := fetchOAuthIdentity(config, accessToken)
	if err != nil {
		s.logger.Warn("OAuth user info failed", "error", err)
		s.redirectOAuthFailure(w, r, "identity")
		return
	}
	if config.RequireVerifiedEmail && !identity.EmailVerified {
		s.redirectOAuthFailure(w, r, "email_unverified")
		return
	}
	_, sessionToken, err := s.store.OAuthLogin(config.ProviderKey, identity.Subject, identity.Email, identity.Name, identity.AvatarURL, identity.EmailVerified)
	if err != nil {
		s.logger.Warn("OAuth account login failed", "error", err)
		if errors.Is(err, store.ErrOAuthRegistrationDisabled) {
			s.redirectOAuthFailure(w, r, "registration_disabled")
			return
		}
		s.redirectOAuthFailure(w, r, "account")
		return
	}
	s.setSessionCookie(w, r, sessionToken)
	http.Redirect(w, r, safeOAuthReturnTo(loginState.ReturnTo), http.StatusSeeOther)
}

func exchangeOAuthCode(config domain.OAuthConfig, code, verifier, redirectURI string) (string, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {config.ClientID},
		"code_verifier": {verifier},
	}
	if config.TokenAuthMethod == "client_secret_post" && config.ClientSecret != "" {
		form.Set("client_secret", config.ClientSecret)
	}
	request, err := http.NewRequest(http.MethodPost, config.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if config.TokenAuthMethod == "client_secret_basic" {
		request.SetBasicAuth(config.ClientID, config.ClientSecret)
	}
	response, err := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("token endpoint returned HTTP %d", response.StatusCode)
	}
	var accessToken string
	if strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "application/x-www-form-urlencoded") {
		values, err := url.ParseQuery(string(body))
		if err != nil {
			return "", err
		}
		accessToken = values.Get("access_token")
	} else {
		var payload struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return "", err
		}
		accessToken = payload.AccessToken
	}
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" || len(accessToken) > 8192 {
		return "", errors.New("token endpoint response has no usable access token")
	}
	return accessToken, nil
}

func fetchOAuthIdentity(config domain.OAuthConfig, accessToken string) (oauthIdentity, error) {
	request, err := http.NewRequest(http.MethodGet, config.UserInfoURL, nil)
	if err != nil {
		return oauthIdentity{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if err != nil {
		return oauthIdentity{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return oauthIdentity{}, fmt.Errorf("user info endpoint returned HTTP %d", response.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	decoder.UseNumber()
	var payload struct {
		Subject           any    `json:"sub"`
		ID                any    `json:"id"`
		Email             string `json:"email"`
		EmailVerified     any    `json:"email_verified"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
		Login             string `json:"login"`
		Picture           string `json:"picture"`
		AvatarURL         string `json:"avatar_url"`
	}
	if err := decoder.Decode(&payload); err != nil {
		return oauthIdentity{}, err
	}
	identity := oauthIdentity{
		Subject:       oauthScalarString(payload.Subject),
		Email:         strings.ToLower(strings.TrimSpace(payload.Email)),
		EmailVerified: oauthBoolean(payload.EmailVerified),
		Name:          firstNonEmpty(payload.Name, payload.PreferredUsername, payload.Login),
		AvatarURL:     firstNonEmpty(payload.Picture, payload.AvatarURL),
	}
	if identity.Subject == "" {
		identity.Subject = oauthScalarString(payload.ID)
	}
	if identity.Subject == "" || identity.Email == "" {
		return oauthIdentity{}, errors.New("user info response is missing subject or email")
	}
	return identity, nil
}

func oauthScalarString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return ""
	}
}

func oauthBoolean(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, _ := strconv.ParseBool(strings.TrimSpace(typed))
		return parsed
	case json.Number:
		return typed.String() == "1"
	case float64:
		return typed == 1
	default:
		return false
	}
}

func randomURLToken(size int) (string, error) {
	if size < 32 || size > 96 {
		return "", errors.New("token size is invalid")
	}
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func safeOAuthReturnTo(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 255 || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.ContainsAny(value, "\\\r\n") {
		return "/"
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func (s *Server) oauthCallbackURL(r *http.Request) string {
	base := s.publicURL
	if settings, err := s.store.GetSiteSettings(); err == nil && settings.PublicURL != "" {
		base = settings.PublicURL
	}
	if base == "" {
		base = requestScheme(r, s.trustedProxies) + "://" + r.Host
	}
	return strings.TrimRight(base, "/") + "/api/v1/oauth/callback"
}

func (s *Server) clearOAuthStateCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: oauthStateCookieName, Value: "", Path: "/api/v1/oauth", MaxAge: -1, HttpOnly: true, Secure: s.secureCookies(r), SameSite: http.SameSiteLaxMode})
}

func (s *Server) redirectOAuthFailure(w http.ResponseWriter, r *http.Request, code string) {
	http.Redirect(w, r, "/login?oauth_error="+url.QueryEscape(code), http.StatusSeeOther)
}
