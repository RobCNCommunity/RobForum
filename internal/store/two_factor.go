package store

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"roblox-community/internal/auth"
	"roblox-community/internal/domain"
)

var ErrInvalidTwoFactorCode = errors.New("invalid two-factor code")

type TwoFactorSetup struct {
	Secret string
	URI    string
}

func (s *Store) TwoFactorEnabled(userID int64) (bool, error) {
	var enabled int
	err := s.db.QueryRow(`SELECT enabled FROM user_two_factor_settings WHERE user_id = ?`, userID).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return enabled != 0, err
}

func (s *Store) BeginTwoFactorSetup(userID int64, issuer, account string) (TwoFactorSetup, error) {
	enabled, err := s.TwoFactorEnabled(userID)
	if err != nil {
		return TwoFactorSetup{}, err
	}
	if enabled {
		return TwoFactorSetup{}, errors.New("two-factor authentication is already enabled")
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: strings.TrimSpace(issuer), AccountName: strings.TrimSpace(account), Period: 30, SecretSize: 20, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
	if err != nil {
		return TwoFactorSetup{}, err
	}
	ciphertext, err := auth.Encrypt(s.masterKey, key.Secret())
	if err != nil {
		return TwoFactorSetup{}, err
	}
	now := time.Now().UTC()
	_, err = s.db.Exec(`INSERT INTO user_two_factor_settings (user_id, secret_ciphertext, enabled, confirmed_at, updated_at) VALUES (?, ?, 0, NULL, ?) ON DUPLICATE KEY UPDATE secret_ciphertext = VALUES(secret_ciphertext), enabled = 0, confirmed_at = NULL, updated_at = VALUES(updated_at)`, userID, ciphertext, now)
	if err != nil {
		return TwoFactorSetup{}, err
	}
	return TwoFactorSetup{Secret: key.Secret(), URI: key.URL()}, nil
}

func validateTOTP(secret, code string, now time.Time) bool {
	valid, err := totp.ValidateCustom(strings.TrimSpace(code), secret, now, totp.ValidateOpts{Period: 30, Skew: 1, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
	return err == nil && valid
}

func (s *Store) ConfirmTwoFactor(userID int64, code string) error {
	var ciphertext string
	if err := s.db.QueryRow(`SELECT secret_ciphertext FROM user_two_factor_settings WHERE user_id = ? AND enabled = 0`, userID).Scan(&ciphertext); err != nil {
		return errors.New("two-factor setup is missing or already enabled")
	}
	secret, err := auth.Decrypt(s.masterKey, ciphertext)
	if err != nil {
		return err
	}
	if !validateTOTP(secret, code, time.Now().UTC()) {
		return ErrInvalidTwoFactorCode
	}
	now := time.Now().UTC()
	result, err := s.db.Exec(`UPDATE user_two_factor_settings SET enabled = 1, confirmed_at = ?, updated_at = ? WHERE user_id = ? AND enabled = 0`, now, now, userID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return errors.New("two-factor setup changed")
	}
	return nil
}

func (s *Store) DisableTwoFactor(userID int64, code string) error {
	var ciphertext string
	if err := s.db.QueryRow(`SELECT secret_ciphertext FROM user_two_factor_settings WHERE user_id = ? AND enabled = 1`, userID).Scan(&ciphertext); err != nil {
		return errors.New("two-factor authentication is not enabled")
	}
	secret, err := auth.Decrypt(s.masterKey, ciphertext)
	if err != nil {
		return err
	}
	if !validateTOTP(secret, code, time.Now().UTC()) {
		return ErrInvalidTwoFactorCode
	}
	_, err = s.db.Exec(`DELETE FROM user_two_factor_settings WHERE user_id = ?`, userID)
	return err
}

func (s *Store) CreateTwoFactorLoginChallenge(userID int64) (string, error) {
	token, tokenHash, err := randomToken()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	_, err = s.db.Exec(`INSERT INTO two_factor_login_challenges (token_hash, user_id, attempts, expires_at, created_at) VALUES (?, ?, 0, ?, ?)`, tokenHash[:], userID, now.Add(5*time.Minute), now)
	return token, err
}

func (s *Store) CompleteTwoFactorLogin(challengeToken, code string) (domain.User, string, error) {
	hash := sha256.Sum256([]byte(strings.TrimSpace(challengeToken)))
	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return domain.User{}, "", err
	}
	defer tx.Rollback()
	var userID int64
	var attempts int
	var expires time.Time
	var ciphertext string
	err = tx.QueryRow(`SELECT c.user_id, c.attempts, c.expires_at, f.secret_ciphertext FROM two_factor_login_challenges c JOIN user_two_factor_settings f ON f.user_id = c.user_id AND f.enabled = 1 WHERE c.token_hash = ? FOR UPDATE`, hash[:]).Scan(&userID, &attempts, &expires, &ciphertext)
	if err != nil || !expires.After(now) || attempts >= 5 {
		return domain.User{}, "", errors.New("two-factor challenge is invalid or expired")
	}
	secret, err := auth.Decrypt(s.masterKey, ciphertext)
	if err != nil {
		return domain.User{}, "", err
	}
	if !validateTOTP(secret, code, now) {
		_, _ = tx.Exec(`UPDATE two_factor_login_challenges SET attempts = attempts + 1 WHERE token_hash = ?`, hash[:])
		if commitErr := tx.Commit(); commitErr != nil {
			return domain.User{}, "", commitErr
		}
		return domain.User{}, "", ErrInvalidTwoFactorCode
	}
	if _, err := tx.Exec(`DELETE FROM two_factor_login_challenges WHERE token_hash = ?`, hash[:]); err != nil {
		return domain.User{}, "", err
	}
	token, tokenHash, err := randomToken()
	if err != nil {
		return domain.User{}, "", err
	}
	if _, err := tx.Exec(`INSERT INTO auth_sessions (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`, tokenHash[:], userID, now.Add(30*24*time.Hour), now); err != nil {
		return domain.User{}, "", err
	}
	user, err := getUser(tx, userID)
	if err != nil {
		return domain.User{}, "", err
	}
	user.TwoFactorEnabled = true
	if err := tx.Commit(); err != nil {
		return domain.User{}, "", err
	}
	return user, token, nil
}
