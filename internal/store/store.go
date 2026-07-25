package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"net/url"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"roblox-community/internal/auth"
	"roblox-community/internal/domain"
)

//go:embed migrations/mysql.sql migrations/sqlite.sql
var migrationFiles embed.FS

var (
	ErrInvalidCredentials       = errors.New("invalid credentials")
	ErrCommentInvalid           = errors.New("comment is invalid")
	ErrCommentAuthorUnavailable = errors.New("comment author is not available")
	ErrParentCommentUnavailable = errors.New("parent comment is not available")
	ErrPostUnavailable          = errors.New("post is not available")
	dummyPasswordHash, _        = bcrypt.GenerateFromPassword([]byte("invalid-password-placeholder"), bcrypt.DefaultCost)
)

type emailVerificationError struct{ message string }

func (e *emailVerificationError) Error() string { return e.message }

type Store struct {
	db        *sql.DB
	driver    string
	masterKey string
}

type rowQueryer interface {
	QueryRow(query string, args ...any) *sql.Row
}

type sqlQueryer interface {
	rowQueryer
	Query(query string, args ...any) (*sql.Rows, error)
}

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func Open(driver, dsn, masterKey string) (*Store, error) {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if driver == "" {
		driver = "mysql"
	}
	if driver != "mysql" {
		return nil, errors.New("only MySQL/MariaDB is supported by the production store")
	}
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("MySQL DSN is required")
	}
	if len([]byte(masterKey)) < 32 {
		return nil, errors.New("master key must contain at least 32 bytes")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("database connection failed: %w", err)
	}
	store := &Store{db: db, driver: driver, masterKey: masterKey}
	if err := store.Migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Migrate() error {
	return s.withAdvisoryLock("roblox-community:migrate", s.migrate)
}

func (s *Store) migrate() error {
	content, err := migrationFiles.ReadFile("migrations/mysql.sql")
	if err != nil {
		return err
	}
	for _, statement := range strings.Split(string(content), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	if err := s.ensureColumnAbsent("posts", "post_type"); err != nil {
		return fmt.Errorf("migration column posts.post_type removal failed: %w", err)
	}
	// Backfill the immutable purchase ledger before any request can rely on it.
	// Historical duplicate paid orders intentionally map to the earliest order only.
	if _, err := s.db.Exec(`INSERT IGNORE INTO resource_purchases (user_id, resource_id, order_id, purchased_at)
		SELECT o.user_id, o.resource_id, o.id, COALESCE(o.paid_at, o.updated_at, o.created_at)
		FROM commerce_orders o
		JOIN (
			SELECT user_id, resource_id, MIN(id) AS order_id
			FROM commerce_orders
			WHERE status = 'paid'
			GROUP BY user_id, resource_id
		) first_paid ON first_paid.order_id = o.id`); err != nil {
		return fmt.Errorf("resource purchase backfill failed: %w", err)
	}
	for _, column := range []struct {
		table string
		name  string
		def   string
	}{
		{table: "resources", name: "price_cents", def: "BIGINT NOT NULL DEFAULT 0"},
		{table: "resources", name: "sales_count", def: "BIGINT NOT NULL DEFAULT 0"},
		{table: "users", name: "bio", def: "VARCHAR(500) NOT NULL DEFAULT ''"},
		{table: "users", name: "cover_url", def: "VARCHAR(500) NOT NULL DEFAULT ''"},
		{table: "users", name: "profile_status", def: "VARCHAR(24) NOT NULL DEFAULT 'active'"},
		{table: "users", name: "blue_verified", def: "TINYINT(1) NOT NULL DEFAULT 0"},
		{table: "users", name: "verification_label", def: "VARCHAR(80) NOT NULL DEFAULT ''"},
		{table: "users", name: "membership_tier_id", def: "BIGINT NULL"},
		{table: "users", name: "membership_started_at", def: "DATETIME(6) NULL"},
		{table: "users", name: "membership_expires_at", def: "DATETIME(6) NULL"},
		{table: "membership_settings", name: "default_withdrawal_fee_bps", def: "INT NOT NULL DEFAULT 300"},
		{table: "membership_settings", name: "default_service_fee_bps", def: "INT NOT NULL DEFAULT 500"},
		{table: "membership_orders", name: "membership_tier_id", def: "BIGINT NULL"},
		{table: "membership_orders", name: "tier_name", def: "VARCHAR(80) NOT NULL DEFAULT ''"},
		{table: "commerce_orders", name: "service_fee_bps", def: "INT NOT NULL DEFAULT 0"},
		{table: "commerce_orders", name: "service_fee_cents", def: "BIGINT NOT NULL DEFAULT 0"},
		{table: "commerce_orders", name: "creator_share_cents", def: "BIGINT NOT NULL DEFAULT 0"},
		{table: "commerce_orders", name: "seller_membership_tier_id", def: "BIGINT NULL"},
		{table: "commerce_orders", name: "seller_membership_tier_name", def: "VARCHAR(80) NOT NULL DEFAULT ''"},
		{table: "creator_payouts", name: "payout_method", def: "VARCHAR(32) NOT NULL DEFAULT 'alipay'"},
		{table: "creator_payouts", name: "payout_account", def: "VARCHAR(255) NOT NULL DEFAULT ''"},
		{table: "creator_payouts", name: "account_name", def: "VARCHAR(120) NOT NULL DEFAULT ''"},
		{table: "creator_payouts", name: "withdrawal_fee_bps", def: "INT NOT NULL DEFAULT 0"},
		{table: "creator_payouts", name: "withdrawal_fee_cents", def: "BIGINT NOT NULL DEFAULT 0"},
		{table: "creator_payouts", name: "net_amount_cents", def: "BIGINT NOT NULL DEFAULT 0"},
		{table: "creator_payouts", name: "membership_tier_id", def: "BIGINT NULL"},
		{table: "creator_payouts", name: "membership_tier_name", def: "VARCHAR(80) NOT NULL DEFAULT ''"},
		{table: "creator_payouts", name: "review_note", def: "VARCHAR(500) NOT NULL DEFAULT ''"},
		{table: "creator_payouts", name: "reviewed_by", def: "BIGINT NULL"},
		{table: "creator_payouts", name: "reviewed_at", def: "DATETIME(6) NULL"},
		{table: "site_settings", name: "banner_enabled", def: "TINYINT(1) NOT NULL DEFAULT 0"},
		{table: "site_settings", name: "banner_text", def: "VARCHAR(240) NOT NULL DEFAULT ''"},
		{table: "site_settings", name: "banner_link", def: "VARCHAR(500) NOT NULL DEFAULT ''"},
		{table: "site_settings", name: "verification_badge_url", def: "VARCHAR(500) NOT NULL DEFAULT ''"},
		{table: "site_settings", name: "post_review_required", def: "TINYINT(1) NOT NULL DEFAULT 1"},
		{table: "site_settings", name: "user_agreement_url", def: "VARCHAR(500) NOT NULL DEFAULT ''"},
		{table: "site_settings", name: "cookies_policy_url", def: "VARCHAR(500) NOT NULL DEFAULT ''"},
		{table: "conversation_members", name: "membership_status", def: "VARCHAR(16) NOT NULL DEFAULT 'accepted'"},
		{table: "conversation_members", name: "invited_by", def: "BIGINT NULL"},
		{table: "conversation_members", name: "responded_at", def: "DATETIME(6) NULL"},
		{table: "conversation_members", name: "last_notified_at", def: "DATETIME(6) NULL"},
		{table: "notifications", name: "conversation_id", def: "BIGINT NULL"},
		{table: "comments", name: "parent_id", def: "BIGINT NULL"},
		{table: "notices", name: "link_url", def: "VARCHAR(500) NOT NULL DEFAULT ''"},
	} {
		if err := s.ensureColumn(column.table, column.name, column.def); err != nil {
			return fmt.Errorf("migration column %s.%s failed: %w", column.table, column.name, err)
		}
	}
	for _, index := range []struct {
		table string
		name  string
		def   string
	}{
		{table: "password_resets", name: "idx_resets_user", def: "INDEX idx_resets_user (user_id)"},
		{table: "password_resets", name: "idx_resets_user_created", def: "INDEX idx_resets_user_created (user_id, created_at)"},
		{table: "password_resets", name: "idx_resets_expires", def: "INDEX idx_resets_expires (expires_at)"},
		{table: "email_verifications", name: "idx_email_verifications_expires", def: "INDEX idx_email_verifications_expires (expires_at)"},
		{table: "comments", name: "idx_comments_author", def: "INDEX idx_comments_author (author_id, status, post_id)"},
		{table: "comments", name: "idx_comments_parent", def: "INDEX idx_comments_parent (parent_id, status, created_at)"},
		{table: "users", name: "idx_users_status_created", def: "INDEX idx_users_status_created (status, created_at)"},
		{table: "users", name: "idx_users_profile_status_created", def: "INDEX idx_users_profile_status_created (profile_status, created_at)"},
		{table: "posts", name: "idx_posts_status_updated", def: "INDEX idx_posts_status_updated (status, updated_at)"},
		{table: "resources", name: "idx_resources_creator_status", def: "INDEX idx_resources_creator_status (creator_id, status, updated_at)"},
		{table: "commerce_orders", name: "idx_orders_user_resource_status", def: "INDEX idx_orders_user_resource_status (user_id, resource_id, status, created_at)"},
		{table: "conversation_members", name: "idx_conversation_members_user_status", def: "INDEX idx_conversation_members_user_status (user_id, membership_status, conversation_id)"},
		{table: "conversations", name: "idx_conversations_creator_kind_created", def: "INDEX idx_conversations_creator_kind_created (created_by, kind, created_at)"},
		{table: "notifications", name: "idx_notifications_conversation", def: "INDEX idx_notifications_conversation (conversation_id, created_at)"},
		{table: "users", name: "idx_users_membership_expires", def: "INDEX idx_users_membership_expires (membership_expires_at, status)"},
		{table: "users", name: "idx_users_membership_tier", def: "INDEX idx_users_membership_tier (membership_tier_id, membership_expires_at)"},
		{table: "membership_tiers", name: "idx_membership_tiers_enabled_sort", def: "INDEX idx_membership_tiers_enabled_sort (enabled, sort_order, id)"},
	} {
		if err := s.ensureIndex(index.table, index.name, index.def); err != nil {
			return fmt.Errorf("migration index %s.%s failed: %w", index.table, index.name, err)
		}
	}
	if err := s.ensureForeignKey("comments", "fk_comments_parent", "FOREIGN KEY (parent_id) REFERENCES comments(id) ON DELETE SET NULL"); err != nil {
		return fmt.Errorf("migration foreign key comments.fk_comments_parent failed: %w", err)
	}
	if _, err := s.db.Exec(`UPDATE creator_payouts SET net_amount_cents = amount_cents WHERE amount_cents > 0 AND net_amount_cents = 0 AND withdrawal_fee_cents = 0`); err != nil {
		return fmt.Errorf("creator payout fee backfill failed: %w", err)
	}
	return nil
}

func (s *Store) ensureColumn(table, column, definition string) error {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`, table, column).Scan(&count)
	if err != nil || count != 0 {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + definition)
	if isMigrationAlreadyApplied(err) {
		return nil
	}
	return err
}

func (s *Store) ensureColumnAbsent(table, column string) error {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`, table, column).Scan(&count)
	if err != nil || count == 0 {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` DROP COLUMN ` + column)
	return err
}

func (s *Store) ensureIndex(table, index, definition string) error {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?`, table, index).Scan(&count)
	if err != nil || count != 0 {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD ` + definition)
	if isMigrationAlreadyApplied(err) {
		return nil
	}
	return err
}

func (s *Store) ensureForeignKey(table, constraint, definition string) error {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM information_schema.referential_constraints WHERE constraint_schema = DATABASE() AND table_name = ? AND constraint_name = ?`, table, constraint).Scan(&count)
	if err != nil || count != 0 {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD CONSTRAINT ` + constraint + ` ` + definition)
	if isMigrationAlreadyApplied(err) {
		return nil
	}
	return err
}

func (s *Store) EnsureDefaults(adminEmail, adminPassword string) error {
	return s.withAdvisoryLock("roblox-community:defaults", func() error {
		return s.ensureDefaults(adminEmail, adminPassword)
	})
}

func (s *Store) ensureDefaults(adminEmail, adminPassword string) error {
	now := time.Now().UTC()
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM site_settings`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		domains, _ := json.Marshal([]string{"qq.com", "163.com", "126.com", "gmail.com", "outlook.com", "hotmail.com", "icloud.com"})
		_, err := s.db.Exec(`INSERT IGNORE INTO site_settings (id, site_name, site_description, logo_url, avatar_url, primary_color, public_url, allow_register, require_email_verification, allowed_email_domains, updated_at) VALUES (1, ?, ?, '', '', '#1d9bf0', '', 1, 0, ?, ?)`, "罗布玩家社区", "Roblox 中国玩家的独立交流社区", string(domains), now)
		if err != nil {
			return err
		}
	}
	if err := s.ensureSingleRow(`captcha_settings`, `INSERT INTO captcha_settings (id, enabled, provider, site_key, endpoint, secret_ciphertext, updated_at) VALUES (1, 0, 'gt4', '', '', '', ?)`, now); err != nil {
		return err
	}
	if err := s.ensureSingleRow(`smtp_settings`, `INSERT INTO smtp_settings (id, enabled, host, port, username, password_ciphertext, from_email, from_name, tls_mode, updated_at) VALUES (1, 0, '', 587, '', '', '', '', 'starttls', ?)`, now); err != nil {
		return err
	}
	if err := s.ensureSingleRow(`payment_settings`, `INSERT INTO payment_settings (id, enabled, kind, gateway_url, merchant_id, secret_ciphertext, pay_type, notify_url, return_url, updated_at) VALUES (1, 0, 'epay', '', '', '', 'alipay', '', '', ?)`, now); err != nil {
		return err
	}
	if err := s.ensureSingleRow(`oauth_settings`, `INSERT INTO oauth_settings (id, enabled, provider_key, provider_name, client_id, client_secret_ciphertext, authorization_url, token_url, userinfo_url, scopes, token_auth_method, require_verified_email, updated_at) VALUES (1, 0, 'oidc', 'OAuth', '', '', '', '', '', 'openid email profile', 'client_secret_post', 1, ?)`, now); err != nil {
		return err
	}
	if err := s.ensureSingleRow(`membership_settings`, `INSERT INTO membership_settings (id, enabled, name, badge_label, badge_url, badge_color, monthly_price_cents, quarterly_price_cents, yearly_price_cents, post_review_exempt, updated_at) VALUES (1, 0, '社区会员', '会员', '', '#f59e0b', 0, 0, 0, 0, ?)`, now); err != nil {
		return err
	}
	if err := s.ensureMembershipTierDefaults(now); err != nil {
		return err
	}
	boards := []struct{ slug, name, description, icon string }{
		{"general", "综合讨论", "聊聊 Roblox、社区动态和玩家见闻", "message"},
		{"guides", "攻略教程", "游戏攻略、玩法心得和新手教程", "book"},
		{"resources", "资源分享", "地图、脚本、素材和实用工具投稿", "folder"},
		{"team-up", "组队招募", "找队友、找开发者和项目招募", "team"},
		{"trade", "交易交流", "玩家之间的交易与服务信息", "shop"},
	}
	for index, board := range boards {
		var exists int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM boards WHERE slug = ?`, board.slug).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			if _, err := s.db.Exec(`INSERT IGNORE INTO boards (slug, name, description, icon, sort_order, status, created_at) VALUES (?, ?, ?, ?, ?, 'active', ?)`, board.slug, board.name, board.description, board.icon, index+1, now); err != nil {
				return err
			}
		}
	}
	var admins int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&admins); err != nil {
		return err
	}
	if admins == 0 {
		if adminEmail == "" || adminPassword == "" {
			return errors.New("ROBLOX_ADMIN_EMAIL and ROBLOX_ADMIN_PASSWORD are required when no administrator exists")
		}
		adminEmail = strings.ToLower(strings.TrimSpace(adminEmail))
		parsedEmail, err := mail.ParseAddress(adminEmail)
		emailParts := strings.Split(adminEmail, "@")
		if err != nil || parsedEmail.Address != adminEmail || len(adminEmail) > 254 || len(emailParts) != 2 || len(emailParts[0]) < 1 || len([]byte(emailParts[0])) > 64 {
			return errors.New("ROBLOX_ADMIN_EMAIL is invalid")
		}
		if err := auth.ValidatePassword(adminPassword); err != nil {
			return errors.New("ROBLOX_ADMIN_PASSWORD is invalid")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if _, err = s.db.Exec(`INSERT IGNORE INTO users (email, password_hash, display_name, avatar_url, role, status, created_at, updated_at) VALUES (?, ?, ?, '', 'admin', 'active', ?, ?)`, adminEmail, string(hash), "社区管理员", now, now); err != nil {
			return err
		}
	}
	if err := s.ensureBadgeDefaults(now); err != nil {
		return err
	}
	if err := s.EnsureCommunitySeeds(); err != nil {
		return err
	}
	if err := s.cleanupExpiredAuthData(now); err != nil {
		return err
	}
	return s.reconcileDerivedData()
}

func (s *Store) withAdvisoryLock(name string, action func() error) (resultErr error) {
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var acquired sql.NullInt64
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK(?, 30)`, name).Scan(&acquired); err != nil {
		return err
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		return fmt.Errorf("database advisory lock %q was not acquired", name)
	}
	defer func() {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer releaseCancel()
		var released sql.NullInt64
		releaseErr := conn.QueryRowContext(releaseCtx, `SELECT RELEASE_LOCK(?)`, name).Scan(&released)
		if resultErr == nil && releaseErr != nil {
			resultErr = releaseErr
		}
	}()
	return action()
}

func (s *Store) cleanupExpiredAuthData(now time.Time) error {
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`DELETE FROM auth_sessions WHERE expires_at <= ?`, []any{now}},
		{`DELETE FROM password_resets WHERE expires_at <= ? OR (used_at IS NOT NULL AND used_at <= ?)`, []any{now, now.Add(-24 * time.Hour)}},
		{`DELETE FROM email_verifications WHERE expires_at <= ?`, []any{now}},
		{`DELETE FROM oauth_login_states WHERE expires_at <= ?`, []any{now}},
	} {
		if _, err := s.db.Exec(statement.query, statement.args...); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) reconcileDerivedData() error {
	statements := []string{
		`UPDATE posts p LEFT JOIN (SELECT post_id, COUNT(*) AS total FROM comments WHERE status = 'published' GROUP BY post_id) c ON c.post_id = p.id SET p.comment_count = COALESCE(c.total, 0) WHERE p.comment_count <> COALESCE(c.total, 0)`,
		`UPDATE resources r LEFT JOIN (SELECT resource_id, COUNT(*) AS total FROM resource_purchases GROUP BY resource_id) o ON o.resource_id = r.id SET r.sales_count = COALESCE(o.total, 0) WHERE r.sales_count <> COALESCE(o.total, 0)`,
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ensureSingleRow(table, query string, now time.Time) error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		_, err := s.db.Exec(query, now)
		if isDuplicateKeyError(err) {
			return nil
		}
		return err
	}
	return nil
}

func (s *Store) GetSiteSettings() (domain.SiteSettings, error) {
	return getSiteSettings(s.db)
}

func getSiteSettings(queryer rowQueryer) (domain.SiteSettings, error) {
	var result domain.SiteSettings
	var allow, verify, postReviewRequired, bannerEnabled int
	var domainsJSON string
	err := queryer.QueryRow(`SELECT site_name, site_description, logo_url, avatar_url, verification_badge_url, primary_color, public_url, user_agreement_url, cookies_policy_url, allow_register, require_email_verification, post_review_required, allowed_email_domains, banner_enabled, banner_text, banner_link, updated_at FROM site_settings WHERE id = 1`).Scan(&result.SiteName, &result.SiteDescription, &result.LogoURL, &result.AvatarURL, &result.VerificationBadgeURL, &result.PrimaryColor, &result.PublicURL, &result.UserAgreementURL, &result.CookiesPolicyURL, &allow, &verify, &postReviewRequired, &domainsJSON, &bannerEnabled, &result.BannerText, &result.BannerLink, &result.UpdatedAt)
	if err != nil {
		return result, err
	}
	result.AllowRegister = allow != 0
	result.RequireEmailVerification = verify != 0
	result.PostReviewRequired = postReviewRequired != 0
	result.BannerEnabled = bannerEnabled != 0
	if err := json.Unmarshal([]byte(domainsJSON), &result.AllowedEmailDomains); err != nil {
		return result, fmt.Errorf("allowed email domains are corrupted: %w", err)
	}
	if !validPrimaryColor(result.PrimaryColor) {
		result.PrimaryColor = "#1677ff"
	}
	if validateWebURL(result.LogoURL, true) != nil {
		result.LogoURL = ""
	}
	if validateWebURL(result.AvatarURL, true) != nil {
		result.AvatarURL = ""
	}
	if validateWebURL(result.VerificationBadgeURL, true) != nil {
		result.VerificationBadgeURL = ""
	}
	if validateWebURL(result.PublicURL, false) != nil {
		result.PublicURL = ""
	}
	if validateWebURL(result.UserAgreementURL, true) != nil {
		result.UserAgreementURL = ""
	}
	if validateWebURL(result.CookiesPolicyURL, true) != nil {
		result.CookiesPolicyURL = ""
	}
	if validateWebURL(result.BannerLink, true) != nil {
		result.BannerLink = ""
	}
	result.AllowedEmailDomains, err = normalizeDomains(result.AllowedEmailDomains)
	if err != nil {
		return result, err
	}
	return result, nil
}

func (s *Store) UpdateSiteSettings(actorID int64, input domain.SiteSettings) (domain.SiteSettings, error) {
	input.SiteName = strings.TrimSpace(input.SiteName)
	input.SiteDescription = strings.TrimSpace(input.SiteDescription)
	input.LogoURL = strings.TrimSpace(input.LogoURL)
	input.AvatarURL = strings.TrimSpace(input.AvatarURL)
	input.VerificationBadgeURL = strings.TrimSpace(input.VerificationBadgeURL)
	input.PrimaryColor = strings.TrimSpace(input.PrimaryColor)
	input.PublicURL = strings.TrimRight(strings.TrimSpace(input.PublicURL), "/")
	input.UserAgreementURL = strings.TrimSpace(input.UserAgreementURL)
	input.CookiesPolicyURL = strings.TrimSpace(input.CookiesPolicyURL)
	input.BannerText = strings.TrimSpace(input.BannerText)
	input.BannerLink = strings.TrimSpace(input.BannerLink)
	if input.SiteName == "" || len([]rune(input.SiteName)) > 120 {
		return domain.SiteSettings{}, errors.New("site name is invalid")
	}
	if len([]rune(input.SiteDescription)) > 500 || len(input.LogoURL) > 500 || len(input.AvatarURL) > 500 || len(input.VerificationBadgeURL) > 500 || len(input.PublicURL) > 255 || containsControl(input.SiteName+input.SiteDescription+input.BannerText) {
		return domain.SiteSettings{}, errors.New("site text is invalid")
	}
	if input.PrimaryColor == "" {
		input.PrimaryColor = "#1677ff"
	}
	if !validPrimaryColor(input.PrimaryColor) {
		return domain.SiteSettings{}, errors.New("primary color must be a six-digit hex color")
	}
	for _, candidate := range []struct {
		value         string
		allowRelative bool
	}{{input.LogoURL, true}, {input.AvatarURL, true}, {input.VerificationBadgeURL, true}, {input.PublicURL, false}, {input.UserAgreementURL, true}, {input.CookiesPolicyURL, true}, {input.BannerLink, true}} {
		if err := validateWebURL(candidate.value, candidate.allowRelative); err != nil {
			return domain.SiteSettings{}, err
		}
	}
	if err := validateSecureWebURL(input.PublicURL); err != nil {
		return domain.SiteSettings{}, err
	}
	normalizedDomains, err := normalizeDomains(input.AllowedEmailDomains)
	if err != nil {
		return domain.SiteSettings{}, err
	}
	domains, err := json.Marshal(normalizedDomains)
	if err != nil {
		return domain.SiteSettings{}, err
	}
	now := time.Now().UTC()
	if len([]rune(input.BannerText)) > 240 || len(input.BannerLink) > 500 {
		return domain.SiteSettings{}, errors.New("banner settings are invalid")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.SiteSettings{}, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE site_settings SET site_name = ?, site_description = ?, logo_url = ?, avatar_url = ?, verification_badge_url = ?, primary_color = ?, public_url = ?, user_agreement_url = ?, cookies_policy_url = ?, allow_register = ?, require_email_verification = ?, post_review_required = ?, allowed_email_domains = ?, banner_enabled = ?, banner_text = ?, banner_link = ?, updated_at = ? WHERE id = 1`, input.SiteName, input.SiteDescription, input.LogoURL, input.AvatarURL, input.VerificationBadgeURL, input.PrimaryColor, input.PublicURL, input.UserAgreementURL, input.CookiesPolicyURL, boolInt(input.AllowRegister), boolInt(input.RequireEmailVerification), boolInt(input.PostReviewRequired), string(domains), boolInt(input.BannerEnabled), input.BannerText, input.BannerLink, now)
	if err != nil {
		return domain.SiteSettings{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return domain.SiteSettings{}, err
		}
		return domain.SiteSettings{}, errors.New("site settings row is missing")
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'site_settings', 1, 'update', '', ?)`, actorID, now); err != nil {
		return domain.SiteSettings{}, err
	}
	settings, err := getSiteSettings(tx)
	if err != nil {
		return domain.SiteSettings{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.SiteSettings{}, err
	}
	return settings, nil
}

func normalizeDomains(domains []string) ([]string, error) {
	if len(domains) > 50 {
		return nil, errors.New("allowed email domains must not exceed 50 entries")
	}
	seen := map[string]bool{}
	result := make([]string, 0, len(domains))
	for _, domain := range domains {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain == "" || !validEmailDomain(domain) {
			return nil, errors.New("allowed email domains are invalid")
		}
		if seen[domain] {
			continue
		}
		seen[domain] = true
		result = append(result, domain)
	}
	return result, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (s *Store) GetSMTPConfig() (domain.SMTPConfig, error) { return s.smtpConfig(false) }

func (s *Store) SMTPDeliveryConfig() (domain.SMTPConfig, error) { return s.smtpConfig(true) }

func (s *Store) smtpConfig(includePassword bool) (domain.SMTPConfig, error) {
	return smtpConfigWithQuery(s.db, includePassword, false, s.masterKey)
}

func smtpConfigWithQuery(queryer rowQueryer, includePassword, forUpdate bool, masterKey string) (domain.SMTPConfig, error) {
	var config domain.SMTPConfig
	var enabled int
	var cipherText string
	query := `SELECT enabled, host, port, username, password_ciphertext, from_email, from_name, tls_mode FROM smtp_settings WHERE id = 1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	err := queryer.QueryRow(query).Scan(&enabled, &config.Host, &config.Port, &config.Username, &cipherText, &config.FromEmail, &config.FromName, &config.TLSMode)
	if err != nil {
		return config, err
	}
	config.Enabled = enabled != 0
	config.HasPassword = cipherText != ""
	if cipherText != "" {
		password, err := auth.Decrypt(masterKey, cipherText)
		if err != nil {
			return config, err
		}
		config.MaskedPassword = auth.Mask(password)
		if includePassword {
			config.Password = password
		}
	}
	return config, nil
}

func (s *Store) UpdateSMTPConfig(actorID int64, input domain.SMTPConfig) (domain.SMTPConfig, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.SMTPConfig{}, err
	}
	defer tx.Rollback()
	current, err := smtpConfigWithQuery(tx, true, true, s.masterKey)
	if err != nil {
		return domain.SMTPConfig{}, err
	}
	input.Host = strings.TrimSpace(input.Host)
	input.Username = strings.TrimSpace(input.Username)
	input.FromEmail = strings.ToLower(strings.TrimSpace(input.FromEmail))
	input.FromName = strings.TrimSpace(input.FromName)
	input.TLSMode = strings.ToLower(strings.TrimSpace(input.TLSMode))
	if input.Port == 0 {
		input.Port = 587
	}
	if input.TLSMode == "" {
		input.TLSMode = "starttls"
	}
	if input.Port < 1 || input.Port > 65535 || (input.TLSMode != "starttls" && input.TLSMode != "tls" && input.TLSMode != "none") {
		return domain.SMTPConfig{}, errors.New("SMTP port or TLS mode is invalid")
	}
	if (input.Host != "" && !validSMTPHost(input.Host)) || strings.ContainsAny(input.FromName+input.Username, "\r\n") || len(input.Host) > 255 || len(input.Username) > 255 || len([]rune(input.FromName)) > 120 || containsControl(input.FromName+input.Username) {
		return domain.SMTPConfig{}, errors.New("SMTP host or sender name is invalid")
	}
	if input.FromEmail != "" {
		parsed, parseErr := mail.ParseAddress(input.FromEmail)
		parts := strings.Split(input.FromEmail, "@")
		if parseErr != nil || !strings.EqualFold(parsed.Address, input.FromEmail) || strings.ContainsAny(input.FromEmail, "\r\n") || len(input.FromEmail) > 254 || len(parts) != 2 || len([]byte(parts[0])) > 64 {
			return domain.SMTPConfig{}, errors.New("SMTP sender email is invalid")
		}
	}
	if input.Enabled && (input.Host == "" || input.FromEmail == "") {
		return domain.SMTPConfig{}, errors.New("enabled SMTP requires host and sender email")
	}
	password := current.Password
	if strings.TrimSpace(input.Password) != "" && !strings.HasPrefix(input.Password, "••••") {
		password = strings.TrimSpace(input.Password)
	}
	if len([]byte(password)) > 4096 {
		return domain.SMTPConfig{}, errors.New("SMTP password is too long")
	}
	cipherText := ""
	if password != "" {
		cipherText, err = auth.Encrypt(s.masterKey, password)
		if err != nil {
			return domain.SMTPConfig{}, err
		}
	}
	if input.Enabled && input.Username != "" && password == "" {
		return domain.SMTPConfig{}, errors.New("SMTP username requires a password")
	}
	if input.TLSMode == "none" && (input.Username != "" || password != "") {
		return domain.SMTPConfig{}, errors.New("SMTP authentication requires TLS")
	}
	now := time.Now().UTC()
	result, err := tx.Exec(`UPDATE smtp_settings SET enabled = ?, host = ?, port = ?, username = ?, password_ciphertext = ?, from_email = ?, from_name = ?, tls_mode = ?, updated_at = ? WHERE id = 1`, boolInt(input.Enabled), input.Host, input.Port, input.Username, cipherText, input.FromEmail, input.FromName, input.TLSMode, now)
	if err != nil {
		return domain.SMTPConfig{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return domain.SMTPConfig{}, err
		}
		return domain.SMTPConfig{}, errors.New("SMTP settings row is missing")
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'smtp_settings', 1, 'update', '', ?)`, actorID, now); err != nil {
		return domain.SMTPConfig{}, err
	}
	config, err := smtpConfigWithQuery(tx, false, false, s.masterKey)
	if err != nil {
		return domain.SMTPConfig{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.SMTPConfig{}, err
	}
	return config, nil
}

func (s *Store) GetCaptchaConfig() (domain.CaptchaConfig, error) {
	return s.captchaConfig(false)
}

func (s *Store) CaptchaDeliveryConfig() (domain.CaptchaConfig, error) {
	return s.captchaConfig(true)
}

func (s *Store) captchaConfig(includeSecret bool) (domain.CaptchaConfig, error) {
	var config domain.CaptchaConfig
	var enabled int
	var cipherText string
	err := s.db.QueryRow(`SELECT enabled, provider, site_key, endpoint, secret_ciphertext FROM captcha_settings WHERE id = 1`).Scan(&enabled, &config.Provider, &config.SiteKey, &config.Endpoint, &cipherText)
	if err != nil {
		return config, err
	}
	config.Enabled = enabled != 0
	config.HasSecret = cipherText != ""
	if cipherText != "" {
		secret, err := auth.Decrypt(s.masterKey, cipherText)
		if err != nil {
			return config, err
		}
		config.MaskedSecret = "••••••••"
		if includeSecret {
			config.Secret = secret
		}
	}
	if !config.Enabled {
		config.AdapterStatus = "disabled"
	} else if config.Endpoint == "" || !config.HasSecret || config.SiteKey == "" {
		config.AdapterStatus = "incomplete"
	} else if config.Provider == "gt4" {
		config.AdapterStatus = "ready"
	} else {
		config.AdapterStatus = "unsupported"
	}
	return config, nil
}

func (s *Store) UpdateCaptchaConfig(actorID int64, input domain.CaptchaConfig) (domain.CaptchaConfig, error) {
	input.Provider = strings.ToLower(strings.TrimSpace(input.Provider))
	if input.Provider != "gt4" && input.Provider != "gt3" && input.Provider != "aliyun" {
		return domain.CaptchaConfig{}, errors.New("unsupported captcha provider")
	}
	input.SiteKey = strings.TrimSpace(input.SiteKey)
	input.Endpoint = strings.TrimSpace(input.Endpoint)
	input.MaskedSecret = strings.TrimSpace(input.MaskedSecret)
	if input.Provider == "gt4" && input.Endpoint == "" {
		input.Endpoint = "https://gcaptcha4.geetest.com/validate"
	}
	if len(input.SiteKey) > 255 || len(input.Endpoint) > 500 || len([]byte(input.MaskedSecret)) > 4096 || containsControl(input.SiteKey+input.Endpoint) {
		return domain.CaptchaConfig{}, errors.New("captcha settings are invalid")
	}
	if err := validateSecureWebURL(input.Endpoint); err != nil {
		return domain.CaptchaConfig{}, errors.New("captcha endpoint must use HTTPS")
	}
	if input.Endpoint != "" {
		parsed, err := url.Parse(input.Endpoint)
		if err != nil || !strings.EqualFold(parsed.Scheme, "https") || !strings.EqualFold(parsed.Hostname(), "gcaptcha4.geetest.com") || parsed.Port() != "" || parsed.Path != "/validate" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
			return domain.CaptchaConfig{}, errors.New("captcha endpoint must be the official GT4 validation endpoint")
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.CaptchaConfig{}, err
	}
	defer tx.Rollback()
	var current string
	if err := tx.QueryRow(`SELECT secret_ciphertext FROM captcha_settings WHERE id = 1 FOR UPDATE`).Scan(&current); err != nil {
		return domain.CaptchaConfig{}, err
	}
	secret := current
	if input.MaskedSecret != "" && input.MaskedSecret != "••••••••" {
		var err error
		secret, err = auth.Encrypt(s.masterKey, strings.TrimSpace(input.MaskedSecret))
		if err != nil {
			return domain.CaptchaConfig{}, err
		}
	}
	if input.Enabled {
		if input.Provider != "gt4" {
			return domain.CaptchaConfig{}, errors.New("only GT4 captcha is currently supported")
		}
		if input.SiteKey == "" || secret == "" || input.Endpoint == "" {
			return domain.CaptchaConfig{}, errors.New("GT4 captcha ID, key, and endpoint are required")
		}
	}
	if strings.TrimSpace(input.Endpoint) == "" {
		input.Endpoint = ""
	}
	now := time.Now().UTC()
	result, err := tx.Exec(`UPDATE captcha_settings SET enabled = ?, provider = ?, site_key = ?, endpoint = ?, secret_ciphertext = ?, updated_at = ? WHERE id = 1`, boolInt(input.Enabled), input.Provider, strings.TrimSpace(input.SiteKey), strings.TrimSpace(input.Endpoint), secret, now)
	if err != nil {
		return domain.CaptchaConfig{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return domain.CaptchaConfig{}, err
		}
		return domain.CaptchaConfig{}, errors.New("captcha settings row is missing")
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'captcha_settings', 1, 'update', '', ?)`, actorID, now); err != nil {
		return domain.CaptchaConfig{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.CaptchaConfig{}, err
	}
	return s.GetCaptchaConfig()
}

func (s *Store) RegisterUser(email, password, displayName, emailCode string, requireVerification bool) (domain.User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	displayName = strings.TrimSpace(displayName)
	parsed, emailErr := mail.ParseAddress(email)
	emailParts := strings.Split(email, "@")
	if emailErr != nil || parsed.Address != email || len(email) > 254 || len(emailParts) != 2 || len(emailParts[0]) < 1 || len([]byte(emailParts[0])) > 64 || len([]rune(displayName)) < 2 || len([]rune(displayName)) > 80 || containsControl(displayName) {
		return domain.User{}, "", errors.New("email, display name, or password is invalid")
	}
	if err := auth.ValidatePassword(password); err != nil {
		return domain.User{}, "", err
	}
	if requireVerification && strings.TrimSpace(emailCode) == "" {
		return domain.User{}, "", &emailVerificationError{message: "请先获取并填写邮箱验证码"}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, "", err
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
	if requireVerification {
		if err := verifyEmailCodeTx(tx, email, emailCode); err != nil {
			var verificationErr *emailVerificationError
			if errors.As(err, &verificationErr) {
				if commitErr := tx.Commit(); commitErr != nil {
					return domain.User{}, "", commitErr
				}
				return domain.User{}, "", err
			}
			return domain.User{}, "", err
		}
	}
	result, err := tx.Exec(`INSERT INTO users (email, password_hash, display_name, avatar_url, role, status, created_at, updated_at) VALUES (?, ?, ?, '', 'user', 'active', ?, ?)`, email, string(hash), displayName, now, now)
	if err != nil {
		if isDuplicateKeyError(err) {
			return domain.User{}, "", errors.New("email is already registered")
		}
		return domain.User{}, "", err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.User{}, "", err
	}
	if _, err := awardBadgeTx(tx, id, "new_member", now); err != nil {
		return domain.User{}, "", err
	}
	user, err := getUser(tx, id)
	if err != nil {
		return domain.User{}, "", err
	}
	if _, err := tx.Exec(`INSERT INTO auth_sessions (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`, tokenHash[:], id, now.Add(30*24*time.Hour), now); err != nil {
		return domain.User{}, "", err
	}
	if requireVerification {
		result, err := tx.Exec(`DELETE FROM email_verifications WHERE email = ?`, email)
		if err != nil {
			return domain.User{}, "", err
		}
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			if err != nil {
				return domain.User{}, "", err
			}
			return domain.User{}, "", errors.New("邮箱验证码状态更新失败")
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, "", err
	}
	return user, token, nil
}

func (s *Store) GetUser(id int64) (domain.User, error) {
	return getUser(s.db, id)
}

func getUser(queryer rowQueryer, id int64) (domain.User, error) {
	var user domain.User
	var blueVerified, robloxVerified int
	var membershipTierID sql.NullInt64
	var membershipStartedAt, membershipExpiresAt sql.NullTime
	err := queryer.QueryRow(`SELECT id, email, display_name, avatar_url, cover_url, bio, role, status, blue_verified, verification_label, membership_tier_id, membership_started_at, membership_expires_at, roblox_name, roblox_id, roblox_verified, created_at FROM users WHERE id = ?`, id).Scan(&user.ID, &user.Email, &user.DisplayName, &user.AvatarURL, &user.CoverURL, &user.Bio, &user.Role, &user.Status, &blueVerified, &user.VerificationLabel, &membershipTierID, &membershipStartedAt, &membershipExpiresAt, &user.RobloxName, &user.RobloxID, &robloxVerified, &user.CreatedAt)
	user.BlueVerified = blueVerified != 0
	user.RobloxVerified = robloxVerified != 0
	applyUserMembership(&user, membershipTierID, membershipStartedAt, membershipExpiresAt)
	if err == nil {
		user.AvatarFrame, err = attachAvatarFrameToUser(queryer, user.ID)
	}
	return user, err
}

func isDuplicateKeyError(err error) bool {
	var mysqlError *mysql.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}

func isMigrationAlreadyApplied(err error) bool {
	var mysqlError *mysql.MySQLError
	return errors.As(err, &mysqlError) && (mysqlError.Number == 1060 || mysqlError.Number == 1061)
}

func (s *Store) GetUserByEmail(email string) (domain.User, error) {
	var user domain.User
	var blueVerified, robloxVerified int
	var membershipTierID sql.NullInt64
	var membershipStartedAt, membershipExpiresAt sql.NullTime
	err := s.db.QueryRow(`SELECT id, email, display_name, avatar_url, cover_url, bio, role, status, blue_verified, verification_label, membership_tier_id, membership_started_at, membership_expires_at, roblox_name, roblox_id, roblox_verified, created_at FROM users WHERE email = ?`, strings.ToLower(strings.TrimSpace(email))).Scan(&user.ID, &user.Email, &user.DisplayName, &user.AvatarURL, &user.CoverURL, &user.Bio, &user.Role, &user.Status, &blueVerified, &user.VerificationLabel, &membershipTierID, &membershipStartedAt, &membershipExpiresAt, &user.RobloxName, &user.RobloxID, &robloxVerified, &user.CreatedAt)
	user.BlueVerified = blueVerified != 0
	user.RobloxVerified = robloxVerified != 0
	applyUserMembership(&user, membershipTierID, membershipStartedAt, membershipExpiresAt)
	if err == nil {
		user.AvatarFrame, err = attachAvatarFrameToUser(s.db, user.ID)
	}
	return user, err
}

func (s *Store) VerifyPassword(email, password string) (domain.User, error) {
	var user domain.User
	var hash string
	var blueVerified, robloxVerified int
	var membershipTierID sql.NullInt64
	var membershipStartedAt, membershipExpiresAt sql.NullTime
	queryErr := s.db.QueryRow(`SELECT id, email, password_hash, display_name, avatar_url, cover_url, bio, role, status, blue_verified, verification_label, membership_tier_id, membership_started_at, membership_expires_at, roblox_name, roblox_id, roblox_verified, created_at FROM users WHERE email = ?`, strings.ToLower(strings.TrimSpace(email))).Scan(&user.ID, &user.Email, &hash, &user.DisplayName, &user.AvatarURL, &user.CoverURL, &user.Bio, &user.Role, &user.Status, &blueVerified, &user.VerificationLabel, &membershipTierID, &membershipStartedAt, &membershipExpiresAt, &user.RobloxName, &user.RobloxID, &robloxVerified, &user.CreatedAt)
	passwordHash := dummyPasswordHash
	if queryErr == nil {
		passwordHash = []byte(hash)
	}
	candidate := []byte(password)
	if len(candidate) > 72 {
		candidate = []byte("invalid-password-placeholder")
	}
	passwordErr := bcrypt.CompareHashAndPassword(passwordHash, candidate)
	if queryErr != nil && !errors.Is(queryErr, sql.ErrNoRows) {
		return domain.User{}, queryErr
	}
	if queryErr != nil || user.Status != "active" || passwordErr != nil {
		return domain.User{}, ErrInvalidCredentials
	}
	user.BlueVerified = blueVerified != 0
	user.RobloxVerified = robloxVerified != 0
	applyUserMembership(&user, membershipTierID, membershipStartedAt, membershipExpiresAt)
	avatarFrame, err := attachAvatarFrameToUser(s.db, user.ID)
	if err != nil {
		return domain.User{}, err
	}
	user.AvatarFrame = avatarFrame
	return user, nil
}

func (s *Store) IssueSession(userID int64) (string, error) {
	token, hash, err := randomToken()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	result, err := s.db.Exec(`INSERT INTO auth_sessions (token_hash, user_id, expires_at, created_at) SELECT ?, id, ?, ? FROM users WHERE id = ? AND status = 'active'`, hash[:], now.Add(30*24*time.Hour), now, userID)
	if err != nil {
		return "", err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return "", err
		}
		return "", errors.New("user is not active")
	}
	return token, nil
}

func (s *Store) UserBySession(token string) (domain.User, error) {
	hash := sha256.Sum256([]byte(token))
	var user domain.User
	var blueVerified, robloxVerified int
	var membershipTierID sql.NullInt64
	var membershipStartedAt, membershipExpiresAt sql.NullTime
	err := s.db.QueryRow(`SELECT u.id, u.email, u.display_name, u.avatar_url, u.cover_url, u.bio, u.role, u.status, u.blue_verified, u.verification_label, u.membership_tier_id, u.membership_started_at, u.membership_expires_at, u.roblox_name, u.roblox_id, u.roblox_verified, u.created_at FROM auth_sessions a JOIN users u ON u.id = a.user_id WHERE a.token_hash = ? AND a.expires_at > ? AND u.status = 'active'`, hash[:], time.Now().UTC()).Scan(&user.ID, &user.Email, &user.DisplayName, &user.AvatarURL, &user.CoverURL, &user.Bio, &user.Role, &user.Status, &blueVerified, &user.VerificationLabel, &membershipTierID, &membershipStartedAt, &membershipExpiresAt, &user.RobloxName, &user.RobloxID, &robloxVerified, &user.CreatedAt)
	if err != nil {
		return domain.User{}, errors.New("session expired")
	}
	user.BlueVerified = blueVerified != 0
	user.RobloxVerified = robloxVerified != 0
	applyUserMembership(&user, membershipTierID, membershipStartedAt, membershipExpiresAt)
	user.AvatarFrame, err = attachAvatarFrameToUser(s.db, user.ID)
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *Store) DeleteSession(token string) error {
	hash := sha256.Sum256([]byte(token))
	_, err := s.db.Exec(`DELETE FROM auth_sessions WHERE token_hash = ?`, hash[:])
	return err
}

func (s *Store) CreateResetToken(email string) (string, domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	token, hash, err := randomToken()
	if err != nil {
		return "", domain.User{}, err
	}
	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return "", domain.User{}, err
	}
	defer tx.Rollback()
	var user domain.User
	var blueVerified, robloxVerified int
	var membershipTierID sql.NullInt64
	var membershipStartedAt, membershipExpiresAt sql.NullTime
	if err := tx.QueryRow(`SELECT id, email, display_name, avatar_url, cover_url, bio, role, status, blue_verified, verification_label, membership_tier_id, membership_started_at, membership_expires_at, roblox_name, roblox_id, roblox_verified, created_at FROM users WHERE email = ? FOR UPDATE`, email).Scan(&user.ID, &user.Email, &user.DisplayName, &user.AvatarURL, &user.CoverURL, &user.Bio, &user.Role, &user.Status, &blueVerified, &user.VerificationLabel, &membershipTierID, &membershipStartedAt, &membershipExpiresAt, &user.RobloxName, &user.RobloxID, &robloxVerified, &user.CreatedAt); err != nil {
		return "", domain.User{}, err
	}
	if user.Status != "active" {
		return "", domain.User{}, errors.New("user is not active")
	}
	user.BlueVerified = blueVerified != 0
	user.RobloxVerified = robloxVerified != 0
	applyUserMembership(&user, membershipTierID, membershipStartedAt, membershipExpiresAt)
	var lastCreated time.Time
	if err := tx.QueryRow(`SELECT created_at FROM password_resets WHERE user_id = ? ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, user.ID).Scan(&lastCreated); err == nil {
		if now.Before(lastCreated.Add(time.Minute)) {
			return "", domain.User{}, errors.New("password reset was requested too recently")
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", domain.User{}, err
	}
	if _, err = tx.Exec(`DELETE FROM password_resets WHERE user_id = ?`, user.ID); err != nil {
		return "", domain.User{}, err
	}
	if _, err = tx.Exec(`INSERT INTO password_resets (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`, hash[:], user.ID, now.Add(30*time.Minute), now); err != nil {
		return "", domain.User{}, err
	}
	err = tx.Commit()
	return token, user, err
}

func (s *Store) RevokeResetToken(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	hash := sha256.Sum256([]byte(token))
	_, err := s.db.Exec(`DELETE FROM password_resets WHERE token_hash = ?`, hash[:])
	return err
}

func (s *Store) ResetPassword(token, password string) error {
	if err := auth.ValidatePassword(password); err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" || len(token) > 128 {
		return errors.New("reset link is invalid or expired")
	}
	hash := sha256.Sum256([]byte(token))
	newHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID int64
	if err := tx.QueryRow(`SELECT user_id FROM password_resets WHERE token_hash = ? AND used_at IS NULL AND expires_at > ? FOR UPDATE`, hash[:], time.Now().UTC()).Scan(&userID); err != nil {
		return errors.New("reset link is invalid or expired")
	}
	now := time.Now().UTC()
	result, err := tx.Exec(`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ? AND status = 'active'`, string(newHash), now, userID)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return err
		}
		return errors.New("reset link is invalid or expired")
	}
	resetResult, err := tx.Exec(`UPDATE password_resets SET used_at = ? WHERE user_id = ? AND used_at IS NULL`, now, userID)
	if err != nil {
		return err
	}
	if affected, err := resetResult.RowsAffected(); err != nil || affected < 1 {
		if err != nil {
			return err
		}
		return errors.New("reset link is invalid or expired")
	}
	if _, err = tx.Exec(`DELETE FROM auth_sessions WHERE user_id = ?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CreateEmailVerification(email string) (string, time.Duration, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	parsed, err := mail.ParseAddress(email)
	emailParts := strings.Split(email, "@")
	if err != nil || parsed.Address != email || len(email) > 254 || len(emailParts) != 2 || len(emailParts[0]) < 1 || len([]byte(emailParts[0])) > 64 {
		return "", 0, errors.New("请输入有效的邮箱地址")
	}
	now := time.Now().UTC()
	number, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", 0, err
	}
	code := fmt.Sprintf("%06d", number.Int64())
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", 0, err
	}
	expires := now.Add(10 * time.Minute)
	tx, err := s.db.Begin()
	if err != nil {
		return "", 0, err
	}
	defer tx.Rollback()
	var sentAt time.Time
	err = tx.QueryRow(`SELECT sent_at FROM email_verifications WHERE email = ? FOR UPDATE`, email).Scan(&sentAt)
	if err == nil && now.Before(sentAt.Add(time.Minute)) {
		return "", 0, errors.New("请稍后再获取验证码")
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", 0, err
	}
	if _, err := tx.Exec(`INSERT INTO email_verifications (email, code_hash, expires_at, sent_at, attempts) VALUES (?, ?, ?, ?, 0) ON DUPLICATE KEY UPDATE code_hash = VALUES(code_hash), expires_at = VALUES(expires_at), sent_at = VALUES(sent_at), attempts = 0`, email, string(hash), expires, now); err != nil {
		return "", 0, err
	}
	if err := tx.Commit(); err != nil {
		return "", 0, err
	}
	return code, 10 * time.Minute, nil
}

func (s *Store) ConsumeEmailVerification(email, code string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := verifyEmailCodeTx(tx, email, code); err != nil {
		var verificationErr *emailVerificationError
		if errors.As(err, &verificationErr) {
			if commitErr := tx.Commit(); commitErr != nil {
				return commitErr
			}
			return err
		}
		return err
	}
	result, err := tx.Exec(`DELETE FROM email_verifications WHERE email = ?`, email)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return err
		}
		return errors.New("邮箱验证码状态更新失败")
	}
	return tx.Commit()
}

func verifyEmailCodeTx(tx *sql.Tx, email, code string) error {
	var hash string
	var expires time.Time
	var attempts int
	if err := tx.QueryRow(`SELECT code_hash, expires_at, attempts FROM email_verifications WHERE email = ? FOR UPDATE`, email).Scan(&hash, &expires, &attempts); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &emailVerificationError{message: "请先获取邮箱验证码"}
		}
		return err
	}
	if time.Now().UTC().After(expires) {
		if _, err := tx.Exec(`DELETE FROM email_verifications WHERE email = ?`, email); err != nil {
			return err
		}
		return &emailVerificationError{message: "邮箱验证码已过期"}
	}
	if attempts >= 5 {
		if _, err := tx.Exec(`DELETE FROM email_verifications WHERE email = ?`, email); err != nil {
			return err
		}
		return &emailVerificationError{message: "验证码错误次数过多"}
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(strings.TrimSpace(code))); err != nil {
		if _, err := tx.Exec(`UPDATE email_verifications SET attempts = attempts + 1 WHERE email = ?`, email); err != nil {
			return err
		}
		return &emailVerificationError{message: "邮箱验证码错误"}
	}
	return nil
}

func (s *Store) DiscardEmailVerification(email string) error {
	_, err := s.db.Exec(`DELETE FROM email_verifications WHERE email = ?`, strings.ToLower(strings.TrimSpace(email)))
	return err
}

func randomToken() (string, [32]byte, error) {
	var hash [32]byte
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", hash, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash = sha256.Sum256([]byte(token))
	return token, hash, nil
}

func (s *Store) ListBoards() ([]domain.Board, error) {
	rows, err := s.db.Query(`SELECT b.id, b.slug, b.name, b.description, b.icon, COUNT(p.id) FROM boards b LEFT JOIN posts p ON p.board_id = b.id AND p.status = 'published' WHERE b.status = 'active' GROUP BY b.id, b.slug, b.name, b.description, b.icon ORDER BY b.sort_order, b.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.Board, 0)
	for rows.Next() {
		var item domain.Board
		if err := rows.Scan(&item.ID, &item.Slug, &item.Name, &item.Description, &item.Icon, &item.PostCount); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) ListPosts(boardSlug, query string, limit int) ([]domain.Post, error) {
	return s.listPosts(boardSlug, query, limit, false)
}

func (s *Store) ListPostsPage(boardSlug, query string, limit, offset int) (domain.PostPage, error) {
	return s.listPostsPage(boardSlug, query, limit, offset, false)
}

// ListRecommendedPosts and ListRecommendedPostsPage intentionally have no
// board/category filter.  Keeping this as a separate store contract prevents
// the home timeline from accidentally inheriting a board filter when the
// caller is navigating between a board page and the home page.
func (s *Store) ListRecommendedPosts(limit int) ([]domain.Post, error) {
	return s.listPosts("", "", limit, true)
}

func (s *Store) ListRecommendedPostsPage(limit, offset int) (domain.PostPage, error) {
	return s.listPostsPage("", "", limit, offset, true)
}

func (s *Store) SearchPosts(query string, limit int) ([]domain.Post, error) {
	return s.listPosts("", query, limit, false)
}

func (s *Store) listPosts(boardSlug, query string, limit int, prioritizeMembers bool) ([]domain.Post, error) {
	page, err := s.listPostsPage(boardSlug, query, limit, 0, prioritizeMembers)
	return page.Items, err
}

func (s *Store) listPostsPage(boardSlug, query string, limit, offset int, prioritizeMembers bool) (domain.PostPage, error) {
	boardSlug = strings.TrimSpace(boardSlug)
	if strings.EqualFold(boardSlug, "all") {
		boardSlug = ""
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if offset < 0 || offset > 100000 {
		offset = 0
	}
	args := []any{}
	where := `p.status = 'published' AND b.status = 'active' AND u.status = 'active'`
	if boardSlug != "" {
		where += ` AND b.slug = ?`
		args = append(args, boardSlug)
	}
	query = strings.TrimSpace(query)
	if len([]rune(query)) > 100 {
		return domain.PostPage{}, errors.New("search query is too long")
	}
	if query != "" {
		where += ` AND (p.title LIKE ? ESCAPE '\\' OR p.content LIKE ? ESCAPE '\\')`
		pattern := "%" + escapeLike(query) + "%"
		args = append(args, pattern, pattern)
	}
	args = append(args, limit+1, offset)
	orderBy := `p.pinned DESC, p.featured DESC, p.updated_at DESC, p.id DESC`
	if prioritizeMembers {
		// A priority tier receives a six-hour recency boost instead of a
		// permanent top bucket, so paid posts cannot starve newer community
		// content indefinitely.
		orderBy = `p.pinned DESC, p.featured DESC, (UNIX_TIMESTAMP(p.updated_at) + CASE WHEN u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP() AND mt.feed_priority = 1 THEN 21600 ELSE 0 END) DESC, p.id DESC`
	}
	rows, err := s.db.Query(`SELECT p.id, p.board_id, b.name, p.author_id, u.display_name, u.avatar_url, u.blue_verified, u.verification_label, COALESCE(u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP(), 0), CASE WHEN u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP() THEN u.membership_tier_id ELSE 0 END, p.title, p.content, p.status, p.pinned, p.featured, p.views, p.comment_count, (SELECT COUNT(*) FROM post_likes pl WHERE pl.post_id = p.id), (SELECT COUNT(*) FROM post_reposts pr WHERE pr.post_id = p.id), p.created_at, p.updated_at FROM posts p JOIN boards b ON b.id = p.board_id JOIN users u ON u.id = p.author_id LEFT JOIN membership_tiers mt ON mt.id = u.membership_tier_id WHERE `+where+` ORDER BY `+orderBy+` LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return domain.PostPage{}, err
	}
	defer rows.Close()
	result := make([]domain.Post, 0, limit+1)
	for rows.Next() {
		var item domain.Post
		var authorVerified, authorMember, pinned, featured int
		if err := rows.Scan(&item.ID, &item.BoardID, &item.BoardName, &item.AuthorID, &item.AuthorName, &item.AuthorAvatar, &authorVerified, &item.AuthorVerificationLabel, &authorMember, &item.AuthorMembershipTierID, &item.Title, &item.Content, &item.Status, &pinned, &featured, &item.Views, &item.CommentCount, &item.LikeCount, &item.RepostCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return domain.PostPage{}, err
		}
		item.AuthorVerified = authorVerified != 0
		item.AuthorMember = authorMember != 0
		item.Pinned = pinned != 0
		item.Featured = featured != 0
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return domain.PostPage{}, err
	}
	hasMore := len(result) > limit
	if hasMore {
		result = result[:limit]
	}
	if err := attachPostMedia(s.db, result); err != nil {
		return domain.PostPage{}, err
	}
	return domain.PostPage{Items: result, NextOffset: offset + len(result), HasMore: hasMore}, nil
}

func (s *Store) GetPost(id int64) (domain.Post, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Post{}, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE posts p JOIN boards b ON b.id = p.board_id JOIN users u ON u.id = p.author_id SET p.views = p.views + 1 WHERE p.id = ? AND p.status = 'published' AND b.status = 'active' AND u.status = 'active'`, id)
	if err != nil {
		return domain.Post{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return domain.Post{}, err
		}
		return domain.Post{}, sql.ErrNoRows
	}
	item, err := getPost(tx, id)
	if err != nil {
		return domain.Post{}, err
	}
	item.Media, err = getPostMedia(tx, id)
	if err != nil {
		return domain.Post{}, err
	}
	item.Tags, err = getPostTags(tx, id)
	if err != nil {
		return domain.Post{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Post{}, err
	}
	return item, nil
}

func getPost(queryer rowQueryer, id int64) (domain.Post, error) {
	var item domain.Post
	var pinned, featured int
	var authorVerified, authorMember int
	err := queryer.QueryRow(`SELECT p.id, p.board_id, b.name, p.author_id, u.display_name, u.avatar_url, u.blue_verified, u.verification_label, COALESCE(u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP(), 0), CASE WHEN u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP() THEN u.membership_tier_id ELSE 0 END, p.title, p.content, p.status, p.pinned, p.featured, p.views, p.comment_count, (SELECT COUNT(*) FROM post_likes pl WHERE pl.post_id = p.id), (SELECT COUNT(*) FROM post_reposts pr WHERE pr.post_id = p.id), p.created_at, p.updated_at FROM posts p JOIN boards b ON b.id = p.board_id JOIN users u ON u.id = p.author_id WHERE p.id = ? AND p.status = 'published' AND b.status = 'active' AND u.status = 'active'`, id).Scan(&item.ID, &item.BoardID, &item.BoardName, &item.AuthorID, &item.AuthorName, &item.AuthorAvatar, &authorVerified, &item.AuthorVerificationLabel, &authorMember, &item.AuthorMembershipTierID, &item.Title, &item.Content, &item.Status, &pinned, &featured, &item.Views, &item.CommentCount, &item.LikeCount, &item.RepostCount, &item.CreatedAt, &item.UpdatedAt)
	item.AuthorVerified = authorVerified != 0
	item.AuthorMember = authorMember != 0
	item.Pinned = pinned != 0
	item.Featured = featured != 0
	if err == nil {
		item.AuthorAvatarFrame, err = attachAvatarFrameToUser(queryer, item.AuthorID)
	}
	return item, err
}

func (s *Store) CreatePost(userID, boardID int64, title, content string) (domain.Post, error) {
	return s.CreatePostWithTagsAndMedia(userID, boardID, title, content, nil, nil)
}

func (s *Store) CreatePostWithMedia(userID, boardID int64, title, content string, media []PostMediaInput) (domain.Post, error) {
	return s.CreatePostWithTagsAndMedia(userID, boardID, title, content, nil, media)
}

func (s *Store) CreatePostWithTagsAndMedia(userID, boardID int64, title, content string, tags []string, media []PostMediaInput) (domain.Post, error) {
	return s.createPostWithTagsAndMedia(userID, boardID, title, content, tags, media, "")
}

func (s *Store) CreateMachineModeratedPostWithTagsAndMedia(userID, boardID int64, title, content string, tags []string, media []PostMediaInput, machineApproved bool) (domain.Post, error) {
	return s.createPostWithTagsAndMedia(userID, boardID, title, content, tags, media, machineModeratedPostStatus(machineApproved))
}

func (s *Store) createPostWithTagsAndMedia(userID, boardID int64, title, content string, tags []string, media []PostMediaInput, moderatedStatus string) (domain.Post, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if len([]rune(title)) > 180 || content == "" || len([]rune(content)) > 50000 || containsControl(title) || containsControl(content) {
		return domain.Post{}, errors.New("title or content is invalid")
	}
	normalizedTags, err := normalizePostTags(tags)
	if err != nil {
		return domain.Post{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Post{}, err
	}
	defer tx.Rollback()
	var authorStatus, authorRole string
	var membershipTierID sql.NullInt64
	var membershipExpiresAt sql.NullTime
	if err := tx.QueryRow(`SELECT status, role, membership_tier_id, membership_expires_at FROM users WHERE id = ? FOR UPDATE`, userID).Scan(&authorStatus, &authorRole, &membershipTierID, &membershipExpiresAt); err != nil || authorStatus != "active" {
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return domain.Post{}, err
		}
		return domain.Post{}, errors.New("author is not available")
	}
	var reviewRequired int
	if err := tx.QueryRow(`SELECT post_review_required FROM site_settings WHERE id = 1`).Scan(&reviewRequired); err != nil {
		return domain.Post{}, err
	}
	memberExempt := false
	if membershipTierID.Valid && membershipExpiresAt.Valid && membershipExpiresAt.Time.After(time.Now().UTC()) {
		tier, err := getMembershipTier(tx, membershipTierID.Int64, false)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return domain.Post{}, err
		}
		memberExempt = err == nil && tier.PostReviewExempt
	}
	postStatus := moderatedStatus
	if postStatus == "" {
		postStatus = initialPostStatus(reviewRequired != 0, authorRole, memberExempt)
	}
	var boardName string
	if err := tx.QueryRow(`SELECT name FROM boards WHERE id = ? AND status = 'active' FOR UPDATE`, boardID).Scan(&boardName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Post{}, errors.New("board is not available")
		}
		return domain.Post{}, err
	}
	now := time.Now().UTC()
	result, err := tx.Exec(`INSERT INTO posts (board_id, author_id, title, content, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, boardID, userID, title, content, postStatus, now, now)
	if err != nil {
		return domain.Post{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.Post{}, err
	}
	if err := insertPostMedia(tx, id, media, now); err != nil {
		return domain.Post{}, err
	}
	if err := insertPostTags(tx, id, normalizedTags); err != nil {
		return domain.Post{}, err
	}
	var authoredCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM posts WHERE author_id = ? AND status IN ('published', 'pending')`, userID).Scan(&authoredCount); err != nil {
		return domain.Post{}, err
	}
	if authoredCount >= 10 {
		if _, err := awardBadgeTx(tx, userID, "creator_10", now); err != nil {
			return domain.Post{}, err
		}
	}
	item, err := getPostForModeration(tx, id)
	if err != nil {
		return domain.Post{}, err
	}
	item.Media, err = getPostMedia(tx, id)
	if err != nil {
		return domain.Post{}, err
	}
	item.Tags, err = getPostTags(tx, id)
	if err != nil {
		return domain.Post{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Post{}, err
	}
	return item, nil
}

func initialPostStatus(reviewRequired bool, authorRole string, membershipExempt ...bool) string {
	exempt := len(membershipExempt) > 0 && membershipExempt[0]
	if reviewRequired && authorRole != "admin" && !exempt {
		return "pending"
	}
	return "published"
}

func machineModeratedPostStatus(approved bool) string {
	if approved {
		return "published"
	}
	return "pending"
}

func (s *Store) ListComments(postID int64) ([]domain.Comment, error) {
	rows, err := s.db.Query(`SELECT c.id, c.post_id, c.parent_id, c.author_id, u.display_name, u.avatar_url, u.blue_verified, u.verification_label, COALESCE(u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP(), 0), CASE WHEN u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP() THEN u.membership_tier_id ELSE 0 END, c.content, (SELECT COUNT(*) FROM comment_likes cl WHERE cl.comment_id = c.id), c.created_at FROM comments c JOIN users u ON u.id = c.author_id JOIN posts p ON p.id = c.post_id JOIN boards b ON b.id = p.board_id JOIN users pu ON pu.id = p.author_id WHERE c.post_id = ? AND c.status = 'published' AND p.status = 'published' AND b.status = 'active' AND u.status = 'active' AND pu.status = 'active' ORDER BY c.created_at ASC LIMIT 500`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.Comment, 0)
	for rows.Next() {
		var item domain.Comment
		var authorVerified, authorMember int
		if err := rows.Scan(&item.ID, &item.PostID, &item.ParentID, &item.AuthorID, &item.AuthorName, &item.AuthorAvatar, &authorVerified, &item.AuthorVerificationLabel, &authorMember, &item.AuthorMembershipTierID, &item.Content, &item.LikeCount, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.AuthorVerified = authorVerified != 0
		item.AuthorMember = authorMember != 0
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := attachCommentMedia(s.db, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) CreateComment(userID, postID int64, content string) (domain.Comment, error) {
	return s.CreateCommentWithMediaAndParent(userID, postID, nil, content, nil)
}

func (s *Store) CreateCommentWithMedia(userID, postID int64, content string, media []CommentMediaInput) (domain.Comment, error) {
	return s.CreateCommentWithMediaAndParent(userID, postID, nil, content, media)
}

func (s *Store) CreateCommentWithMediaAndParent(userID, postID int64, parentID *int64, content string, media []CommentMediaInput) (domain.Comment, error) {
	content = strings.TrimSpace(content)
	if (content == "" && len(media) == 0) || len([]rune(content)) > 5000 || containsControl(content) || len(media) > maxCommentMediaCount || (parentID != nil && *parentID < 1) {
		return domain.Comment{}, ErrCommentInvalid
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Comment{}, err
	}
	defer tx.Rollback()
	var authorStatus string
	if err := tx.QueryRow(`SELECT status FROM users WHERE id = ? FOR UPDATE`, userID).Scan(&authorStatus); err != nil || authorStatus != "active" {
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return domain.Comment{}, err
		}
		return domain.Comment{}, ErrCommentAuthorUnavailable
	}
	var targetID, postAuthorID int64
	if err := tx.QueryRow(`SELECT p.id, p.author_id FROM posts p JOIN boards b ON b.id = p.board_id JOIN users u ON u.id = p.author_id WHERE p.id = ? AND p.status = 'published' AND b.status = 'active' AND u.status = 'active' FOR UPDATE`, postID).Scan(&targetID, &postAuthorID); err != nil {
		return domain.Comment{}, ErrPostUnavailable
	}
	notificationUserID := postAuthorID
	if parentID != nil {
		var parentPostID int64
		if err := tx.QueryRow(`SELECT post_id, author_id FROM comments WHERE id = ? AND status = 'published' FOR UPDATE`, *parentID).Scan(&parentPostID, &notificationUserID); err != nil {
			return domain.Comment{}, ErrParentCommentUnavailable
		}
		if err := validateCommentReplyTarget(parentPostID, postID); err != nil {
			return domain.Comment{}, err
		}
	}
	now := time.Now().UTC()
	result, err := tx.Exec(`INSERT INTO comments (post_id, parent_id, author_id, content, status, created_at) VALUES (?, ?, ?, ?, 'published', ?)`, postID, parentID, userID, content, now)
	if err != nil {
		return domain.Comment{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.Comment{}, err
	}
	if err := insertCommentMedia(tx, id, media, now); err != nil {
		return domain.Comment{}, err
	}
	update, err := tx.Exec(`UPDATE posts SET comment_count = comment_count + 1, updated_at = ? WHERE id = ?`, now, postID)
	if err != nil {
		return domain.Comment{}, err
	}
	if affected, err := update.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return domain.Comment{}, err
		}
		return domain.Comment{}, errors.New("post comment count update failed")
	}
	if err := createNotification(tx, notificationUserID, userID, "comment", postID, id); err != nil {
		return domain.Comment{}, err
	}
	item, err := getComment(tx, id)
	if err != nil {
		return domain.Comment{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Comment{}, err
	}
	return item, nil
}

func getComment(queryer sqlQueryer, id int64) (domain.Comment, error) {
	var item domain.Comment
	var authorVerified, authorMember int
	err := queryer.QueryRow(`SELECT c.id, c.post_id, c.parent_id, c.author_id, u.display_name, u.avatar_url, u.blue_verified, u.verification_label, COALESCE(u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP(), 0), CASE WHEN u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP() THEN u.membership_tier_id ELSE 0 END, c.content, (SELECT COUNT(*) FROM comment_likes cl WHERE cl.comment_id = c.id), c.created_at FROM comments c JOIN users u ON u.id = c.author_id JOIN posts p ON p.id = c.post_id JOIN boards b ON b.id = p.board_id JOIN users pu ON pu.id = p.author_id WHERE c.id = ? AND c.status = 'published' AND u.status = 'active' AND p.status = 'published' AND b.status = 'active' AND pu.status = 'active'`, id).Scan(&item.ID, &item.PostID, &item.ParentID, &item.AuthorID, &item.AuthorName, &item.AuthorAvatar, &authorVerified, &item.AuthorVerificationLabel, &authorMember, &item.AuthorMembershipTierID, &item.Content, &item.LikeCount, &item.CreatedAt)
	item.AuthorVerified = authorVerified != 0
	item.AuthorMember = authorMember != 0
	if err == nil {
		item.Media, err = getCommentMedia(queryer, id)
	}
	if err == nil {
		item.AuthorAvatarFrame, err = attachAvatarFrameToUser(queryer, item.AuthorID)
	}
	return item, err
}
