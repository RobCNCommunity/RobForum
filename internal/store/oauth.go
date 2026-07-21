package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"roblox-community/internal/auth"
	"roblox-community/internal/domain"
)

var ErrOAuthRegistrationDisabled = errors.New("OAuth registration is disabled")

type OAuthLoginState struct {
	CodeVerifier string
	ReturnTo     string
}

func (s *Store) GetOAuthConfig() (domain.OAuthConfig, error) {
	return s.oauthConfig(false)
}

func (s *Store) OAuthDeliveryConfig() (domain.OAuthConfig, error) {
	return s.oauthConfig(true)
}

func (s *Store) oauthConfig(includeSecret bool) (domain.OAuthConfig, error) {
	return oauthConfigWithQuery(s.db, includeSecret, false, s.masterKey)
}

func oauthConfigWithQuery(queryer rowQueryer, includeSecret, forUpdate bool, masterKey string) (domain.OAuthConfig, error) {
	var config domain.OAuthConfig
	var enabled, requireVerified int
	var cipherText string
	query := `SELECT enabled, provider_key, provider_name, client_id, client_secret_ciphertext, authorization_url, token_url, userinfo_url, scopes, token_auth_method, require_verified_email, updated_at FROM oauth_settings WHERE id = 1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	if err := queryer.QueryRow(query).Scan(&enabled, &config.ProviderKey, &config.ProviderName, &config.ClientID, &cipherText, &config.AuthorizationURL, &config.TokenURL, &config.UserInfoURL, &config.Scopes, &config.TokenAuthMethod, &requireVerified, &config.UpdatedAt); err != nil {
		return config, err
	}
	config.Enabled = enabled != 0
	config.RequireVerifiedEmail = requireVerified != 0
	config.HasClientSecret = cipherText != ""
	if cipherText != "" {
		secret, err := auth.Decrypt(masterKey, cipherText)
		if err != nil {
			return config, err
		}
		config.MaskedClientSecret = auth.Mask(secret)
		if includeSecret {
			config.ClientSecret = secret
		}
	}
	return config, nil
}

func (s *Store) UpdateOAuthConfig(actorID int64, input domain.OAuthConfig) (domain.OAuthConfig, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.OAuthConfig{}, err
	}
	defer tx.Rollback()
	current, err := oauthConfigWithQuery(tx, true, true, s.masterKey)
	if err != nil {
		return domain.OAuthConfig{}, err
	}

	input.ProviderKey = strings.ToLower(strings.TrimSpace(input.ProviderKey))
	input.ProviderName = strings.TrimSpace(input.ProviderName)
	input.ClientID = strings.TrimSpace(input.ClientID)
	input.AuthorizationURL = strings.TrimSpace(input.AuthorizationURL)
	input.TokenURL = strings.TrimSpace(input.TokenURL)
	input.UserInfoURL = strings.TrimSpace(input.UserInfoURL)
	input.Scopes = strings.Join(strings.Fields(input.Scopes), " ")
	input.TokenAuthMethod = strings.ToLower(strings.TrimSpace(input.TokenAuthMethod))
	if input.ProviderKey == "" {
		input.ProviderKey = "oidc"
	}
	if input.ProviderName == "" {
		input.ProviderName = "OAuth"
	}
	if input.Scopes == "" {
		input.Scopes = "openid email profile"
	}
	if input.TokenAuthMethod == "" {
		input.TokenAuthMethod = "client_secret_post"
	}
	if !validOAuthProviderKey(input.ProviderKey) || len([]rune(input.ProviderName)) > 80 || containsControl(input.ProviderName+input.ClientID+input.Scopes) {
		return domain.OAuthConfig{}, errors.New("OAuth provider identity is invalid")
	}
	if len(input.ClientID) > 255 || len(input.Scopes) > 500 || (input.TokenAuthMethod != "client_secret_post" && input.TokenAuthMethod != "client_secret_basic") {
		return domain.OAuthConfig{}, errors.New("OAuth client settings are invalid")
	}
	for _, endpoint := range []string{input.AuthorizationURL, input.TokenURL, input.UserInfoURL} {
		if err := validateSecureWebURL(endpoint); err != nil {
			return domain.OAuthConfig{}, errors.New("OAuth endpoints must use HTTPS outside local development")
		}
	}

	secret := current.ClientSecret
	if input.ClearClientSecret {
		secret = ""
	} else if strings.TrimSpace(input.ClientSecret) != "" && input.ClientSecret != current.MaskedClientSecret {
		secret = strings.TrimSpace(input.ClientSecret)
	}
	if len([]byte(secret)) > 4096 {
		return domain.OAuthConfig{}, errors.New("OAuth client secret is too long")
	}
	if input.Enabled && (input.ClientID == "" || input.AuthorizationURL == "" || input.TokenURL == "" || input.UserInfoURL == "") {
		return domain.OAuthConfig{}, errors.New("enabled OAuth requires client ID and all provider endpoints")
	}
	cipherText := ""
	if secret != "" {
		cipherText, err = auth.Encrypt(s.masterKey, secret)
		if err != nil {
			return domain.OAuthConfig{}, err
		}
	}

	now := time.Now().UTC()
	result, err := tx.Exec(`UPDATE oauth_settings SET enabled = ?, provider_key = ?, provider_name = ?, client_id = ?, client_secret_ciphertext = ?, authorization_url = ?, token_url = ?, userinfo_url = ?, scopes = ?, token_auth_method = ?, require_verified_email = ?, updated_at = ? WHERE id = 1`, boolInt(input.Enabled), input.ProviderKey, input.ProviderName, input.ClientID, cipherText, input.AuthorizationURL, input.TokenURL, input.UserInfoURL, input.Scopes, input.TokenAuthMethod, boolInt(input.RequireVerifiedEmail), now)
	if err != nil {
		return domain.OAuthConfig{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return domain.OAuthConfig{}, err
		}
		return domain.OAuthConfig{}, errors.New("OAuth settings row is missing")
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'oauth_settings', 1, 'update', '', ?)`, actorID, now); err != nil {
		return domain.OAuthConfig{}, err
	}
	config, err := oauthConfigWithQuery(tx, false, false, s.masterKey)
	if err != nil {
		return domain.OAuthConfig{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.OAuthConfig{}, err
	}
	return config, nil
}

func validOAuthProviderKey(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func (s *Store) CreateOAuthLoginState(stateHash [32]byte, codeVerifier, returnTo string, expiresAt time.Time) error {
	codeVerifier = strings.TrimSpace(codeVerifier)
	returnTo = strings.TrimSpace(returnTo)
	if len(codeVerifier) < 43 || len(codeVerifier) > 128 || !validInternalReturnTo(returnTo) || !expiresAt.After(time.Now().UTC()) {
		return errors.New("OAuth login state is invalid")
	}
	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM oauth_login_states WHERE expires_at <= ?`, now); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO oauth_login_states (state_hash, code_verifier, return_to, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`, stateHash[:], codeVerifier, returnTo, expiresAt, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ConsumeOAuthLoginState(state string) (OAuthLoginState, error) {
	state = strings.TrimSpace(state)
	if state == "" || len(state) > 128 {
		return OAuthLoginState{}, errors.New("OAuth login state is invalid")
	}
	hash := sha256.Sum256([]byte(state))
	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return OAuthLoginState{}, err
	}
	defer tx.Rollback()
	var result OAuthLoginState
	var expiresAt time.Time
	if err := tx.QueryRow(`SELECT code_verifier, return_to, expires_at FROM oauth_login_states WHERE state_hash = ? FOR UPDATE`, hash[:]).Scan(&result.CodeVerifier, &result.ReturnTo, &expiresAt); err != nil {
		return OAuthLoginState{}, errors.New("OAuth login state is invalid or expired")
	}
	if _, err := tx.Exec(`DELETE FROM oauth_login_states WHERE state_hash = ?`, hash[:]); err != nil {
		return OAuthLoginState{}, err
	}
	if !expiresAt.After(now) {
		if err := tx.Commit(); err != nil {
			return OAuthLoginState{}, err
		}
		return OAuthLoginState{}, errors.New("OAuth login state is invalid or expired")
	}
	if err := tx.Commit(); err != nil {
		return OAuthLoginState{}, err
	}
	return result, nil
}

func validInternalReturnTo(value string) bool {
	return strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") && !strings.Contains(value, "\\") && len(value) <= 255 && !containsControl(value)
}

func (s *Store) OAuthLogin(providerKey, subject, email, displayName, avatarURL string, emailVerified bool) (domain.User, string, error) {
	providerKey = strings.ToLower(strings.TrimSpace(providerKey))
	subject = strings.TrimSpace(subject)
	email = strings.ToLower(strings.TrimSpace(email))
	displayName = normalizeOAuthDisplayName(displayName, email)
	avatarURL = strings.TrimSpace(avatarURL)
	parsedEmail, emailErr := mail.ParseAddress(email)
	parts := strings.Split(email, "@")
	if !validOAuthProviderKey(providerKey) || subject == "" || len(subject) > 191 || containsControl(subject) || emailErr != nil || parsedEmail.Address != email || len(parts) != 2 || len([]byte(parts[0])) > 64 {
		return domain.User{}, "", errors.New("OAuth identity is invalid")
	}
	if err := validateSecureWebURL(avatarURL); err != nil {
		avatarURL = ""
	}

	token, tokenHash, err := randomToken()
	if err != nil {
		return domain.User{}, "", err
	}

	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return domain.User{}, "", err
	}
	defer tx.Rollback()

	var userID int64
	linked := true
	err = tx.QueryRow(`SELECT user_id FROM oauth_accounts WHERE provider_key = ? AND subject = ? FOR UPDATE`, providerKey, subject).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		linked = false
		err = tx.QueryRow(`SELECT id FROM users WHERE email = ? FOR UPDATE`, email).Scan(&userID)
		if errors.Is(err, sql.ErrNoRows) {
			var allowRegister int
			var domainsJSON string
			if err := tx.QueryRow(`SELECT allow_register, allowed_email_domains FROM site_settings WHERE id = 1 FOR UPDATE`).Scan(&allowRegister, &domainsJSON); err != nil {
				return domain.User{}, "", err
			}
			if allowRegister == 0 {
				return domain.User{}, "", ErrOAuthRegistrationDisabled
			}
			var domains []string
			if err := json.Unmarshal([]byte(domainsJSON), &domains); err != nil {
				return domain.User{}, "", err
			}
			if len(domains) > 0 {
				allowed := false
				for _, domain := range domains {
					if strings.EqualFold(strings.TrimSpace(domain), parts[1]) {
						allowed = true
						break
					}
				}
				if !allowed {
					return domain.User{}, "", errors.New("OAuth email domain is not allowed")
				}
			}
			randomPassword := make([]byte, 32)
			if _, err := rand.Read(randomPassword); err != nil {
				return domain.User{}, "", err
			}
			passwordHash, err := bcrypt.GenerateFromPassword([]byte(base64.RawURLEncoding.EncodeToString(randomPassword)), bcrypt.DefaultCost)
			if err != nil {
				return domain.User{}, "", err
			}
			result, err := tx.Exec(`INSERT INTO users (email, password_hash, display_name, avatar_url, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, 'user', 'active', ?, ?)`, email, string(passwordHash), displayName, avatarURL, now, now)
			if err != nil {
				return domain.User{}, "", err
			}
			userID, err = result.LastInsertId()
			if err != nil {
				return domain.User{}, "", err
			}
		} else if err != nil {
			return domain.User{}, "", err
		} else if !emailVerified {
			return domain.User{}, "", errors.New("verified OAuth email is required to link an existing account")
		}
		if _, err := tx.Exec(`INSERT INTO oauth_accounts (provider_key, subject, user_id, created_at, last_login_at) VALUES (?, ?, ?, ?, ?)`, providerKey, subject, userID, now, now); err != nil {
			return domain.User{}, "", err
		}
	} else if err != nil {
		return domain.User{}, "", err
	}
	if linked {
		if _, err := tx.Exec(`UPDATE oauth_accounts SET last_login_at = ? WHERE provider_key = ? AND subject = ?`, now, providerKey, subject); err != nil {
			return domain.User{}, "", err
		}
	}
	var status string
	if err := tx.QueryRow(`SELECT status FROM users WHERE id = ? FOR UPDATE`, userID).Scan(&status); err != nil || status != "active" {
		if err != nil {
			return domain.User{}, "", err
		}
		return domain.User{}, "", errors.New("OAuth user is not active")
	}
	if avatarURL != "" {
		if _, err := tx.Exec(`UPDATE users SET avatar_url = ?, updated_at = ? WHERE id = ? AND avatar_url = ''`, avatarURL, now, userID); err != nil {
			return domain.User{}, "", err
		}
	}
	user, err := getUser(tx, userID)
	if err != nil {
		return domain.User{}, "", err
	}
	if _, err := tx.Exec(`INSERT INTO auth_sessions (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`, tokenHash[:], userID, now.Add(30*24*time.Hour), now); err != nil {
		return domain.User{}, "", err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, "", err
	}
	return user, token, nil
}

func normalizeOAuthDisplayName(value, email string) string {
	value = strings.TrimSpace(value)
	if containsControl(value) {
		value = ""
	}
	characters := []rune(value)
	if len(characters) > 80 {
		value = string(characters[:80])
	}
	if len([]rune(value)) < 2 {
		value = strings.Split(email, "@")[0]
	}
	if len([]rune(value)) < 2 || containsControl(value) {
		return "OAuth 用户"
	}
	characters = []rune(value)
	if len(characters) > 80 {
		return string(characters[:80])
	}
	return value
}
