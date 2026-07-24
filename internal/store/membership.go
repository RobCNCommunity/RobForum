package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

const (
	maxMembershipPriceCents int64 = 1_000_000
	maxFeeBPS                     = 10_000
)

func (s *Store) ensureMembershipTierDefaults(now time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Lock the singleton settings row first. Besides supplying the legacy
	// values used for the initial tier, this serializes concurrent startup
	// calls without relying on an aggregate SELECT ... FOR UPDATE, whose
	// locking semantics differ between MySQL and MariaDB versions.
	var name, badgeLabel, badgeURL, badgeColor string
	var monthly, quarterly, yearly int64
	var postReviewExempt, withdrawalFeeBPS, serviceFeeBPS int
	if err := tx.QueryRow(`SELECT name, badge_label, badge_url, badge_color, monthly_price_cents, quarterly_price_cents, yearly_price_cents, post_review_exempt, default_withdrawal_fee_bps, default_service_fee_bps FROM membership_settings WHERE id = 1 FOR UPDATE`).Scan(
		&name, &badgeLabel, &badgeURL, &badgeColor, &monthly, &quarterly, &yearly, &postReviewExempt, &withdrawalFeeBPS, &serviceFeeBPS,
	); err != nil {
		return err
	}
	withdrawalFeeBPS = normalizedFeeBPS(withdrawalFeeBPS, 300)
	serviceFeeBPS = normalizedFeeBPS(serviceFeeBPS, 500)

	var defaultTierID int64
	err = tx.QueryRow(`SELECT id FROM membership_tiers ORDER BY sort_order, id LIMIT 1 FOR UPDATE`).Scan(&defaultTierID)
	if errors.Is(err, sql.ErrNoRows) {
		result, err := tx.Exec(`INSERT INTO membership_tiers (enabled, name, badge_label, badge_url, badge_color, monthly_price_cents, quarterly_price_cents, yearly_price_cents, post_review_exempt, feed_priority, withdrawal_fee_bps, service_fee_bps, sort_order, created_at, updated_at) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, 10, ?, ?)`,
			name, badgeLabel, badgeURL, badgeColor, monthly, quarterly, yearly, postReviewExempt, withdrawalFeeBPS, serviceFeeBPS, now, now)
		if err != nil {
			return err
		}
		defaultTierID, err = result.LastInsertId()
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if defaultTierID > 0 {
		if _, err := tx.Exec(`UPDATE users SET membership_tier_id = ? WHERE membership_tier_id IS NULL AND membership_expires_at > ?`, defaultTierID, now); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE membership_orders SET membership_tier_id = ?, tier_name = COALESCE(NULLIF(tier_name, ''), (SELECT name FROM membership_tiers WHERE id = ?)) WHERE membership_tier_id IS NULL`, defaultTierID, defaultTierID); err != nil {
			return err
		}
	}
	if err := syncLegacyMembershipSettings(tx, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetMembershipSettings() (domain.MembershipSettings, error) {
	return getMembershipSettings(s.db, false)
}

func getMembershipSettings(queryer sqlQueryer, forUpdate bool) (domain.MembershipSettings, error) {
	var result domain.MembershipSettings
	var enabled int
	query := `SELECT enabled, default_withdrawal_fee_bps, default_service_fee_bps, updated_at FROM membership_settings WHERE id = 1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	if err := queryer.QueryRow(query).Scan(&enabled, &result.DefaultWithdrawalFeeBPS, &result.DefaultServiceFeeBPS, &result.UpdatedAt); err != nil {
		return result, err
	}
	result.Enabled = enabled != 0
	result.DefaultWithdrawalFeeBPS = normalizedFeeBPS(result.DefaultWithdrawalFeeBPS, 300)
	result.DefaultServiceFeeBPS = normalizedFeeBPS(result.DefaultServiceFeeBPS, 500)
	tiers, err := listMembershipTiers(queryer, forUpdate)
	if err != nil {
		return result, err
	}
	result.Tiers = tiers
	applyLegacyTierFields(&result)
	return result, nil
}

func (s *Store) UpdateMembershipSettings(actorID int64, input domain.MembershipSettings) (domain.MembershipSettings, error) {
	if err := validateFeeBPS(input.DefaultWithdrawalFeeBPS, "普通用户提现手续费"); err != nil {
		return domain.MembershipSettings{}, err
	}
	if err := validateFeeBPS(input.DefaultServiceFeeBPS, "普通用户服务费"); err != nil {
		return domain.MembershipSettings{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.MembershipSettings{}, err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, []int64{actorID}); err != nil {
		return domain.MembershipSettings{}, err
	}
	current, err := getMembershipSettings(tx, true)
	if err != nil {
		return domain.MembershipSettings{}, err
	}

	// Cached versions of the old settings page submit the former single-tier
	// fields. Keep that save path working while treating the first tier as the
	// compatibility target.
	if len(input.Tiers) == 0 && strings.TrimSpace(input.Name) != "" && len(current.Tiers) > 0 {
		// Those clients predate configurable transaction fees and therefore omit
		// both fields. Preserve the current defaults instead of interpreting the
		// JSON zero values as an intentional request for free withdrawals/sales.
		input.DefaultWithdrawalFeeBPS = current.DefaultWithdrawalFeeBPS
		input.DefaultServiceFeeBPS = current.DefaultServiceFeeBPS
		legacy := current.Tiers[0]
		legacy.Name = input.Name
		legacy.BadgeLabel = input.BadgeLabel
		legacy.BadgeURL = input.BadgeURL
		legacy.BadgeColor = input.BadgeColor
		legacy.MonthlyPriceCents = input.MonthlyPriceCents
		legacy.QuarterlyPriceCents = input.QuarterlyPriceCents
		legacy.YearlyPriceCents = input.YearlyPriceCents
		legacy.PostReviewExempt = input.PostReviewExempt
		if err := validateMembershipTier(legacy); err != nil {
			return domain.MembershipSettings{}, err
		}
		if err := updateMembershipTierTx(tx, legacy, time.Now().UTC()); err != nil {
			return domain.MembershipSettings{}, err
		}
		current.Tiers[0] = legacy
	}
	if input.Enabled && !hasPurchasableTier(current.Tiers) {
		return domain.MembershipSettings{}, errors.New("开放会员购买前至少需要一个已启用且有价格的阶层")
	}

	now := time.Now().UTC()
	if _, err := tx.Exec(`UPDATE membership_settings SET enabled = ?, default_withdrawal_fee_bps = ?, default_service_fee_bps = ?, updated_at = ? WHERE id = 1`, boolInt(input.Enabled), input.DefaultWithdrawalFeeBPS, input.DefaultServiceFeeBPS, now); err != nil {
		return domain.MembershipSettings{}, err
	}
	if err := syncLegacyMembershipSettings(tx, now); err != nil {
		return domain.MembershipSettings{}, err
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'membership_settings', 1, 'update', ?, ?)`, actorID, fmt.Sprintf("default_withdrawal_fee_bps=%d, default_service_fee_bps=%d", input.DefaultWithdrawalFeeBPS, input.DefaultServiceFeeBPS), now); err != nil {
		return domain.MembershipSettings{}, err
	}
	updated, err := getMembershipSettings(tx, false)
	if err != nil {
		return domain.MembershipSettings{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.MembershipSettings{}, err
	}
	return updated, nil
}

func (s *Store) CreateMembershipTier(actorID int64, input domain.MembershipTier) (domain.MembershipTier, error) {
	input.ID = 0
	input.CreatedAt = time.Time{}
	input.UpdatedAt = time.Time{}
	if err := normalizeAndValidateMembershipTier(&input); err != nil {
		return domain.MembershipTier{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.MembershipTier{}, err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, []int64{actorID}); err != nil {
		return domain.MembershipTier{}, err
	}
	if _, err := getMembershipSettings(tx, true); err != nil {
		return domain.MembershipTier{}, err
	}
	now := time.Now().UTC()
	result, err := tx.Exec(`INSERT INTO membership_tiers (enabled, name, badge_label, badge_url, badge_color, monthly_price_cents, quarterly_price_cents, yearly_price_cents, post_review_exempt, feed_priority, withdrawal_fee_bps, service_fee_bps, sort_order, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		boolInt(input.Enabled), input.Name, input.BadgeLabel, input.BadgeURL, input.BadgeColor, input.MonthlyPriceCents, input.QuarterlyPriceCents, input.YearlyPriceCents, boolInt(input.PostReviewExempt), boolInt(input.FeedPriority), input.WithdrawalFeeBPS, input.ServiceFeeBPS, input.SortOrder, now, now)
	if err != nil {
		return domain.MembershipTier{}, err
	}
	input.ID, err = result.LastInsertId()
	if err != nil {
		return domain.MembershipTier{}, err
	}
	if err := syncLegacyMembershipSettings(tx, now); err != nil {
		return domain.MembershipTier{}, err
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'membership_tier', ?, 'create', ?, ?)`, actorID, input.ID, input.Name, now); err != nil {
		return domain.MembershipTier{}, err
	}
	created, err := getMembershipTier(tx, input.ID, false)
	if err != nil {
		return domain.MembershipTier{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.MembershipTier{}, err
	}
	return created, nil
}

func (s *Store) GetMembershipTier(tierID int64) (domain.MembershipTier, error) {
	if tierID < 1 {
		return domain.MembershipTier{}, sql.ErrNoRows
	}
	return getMembershipTier(s.db, tierID, false)
}

func (s *Store) UpdateMembershipTier(actorID, tierID int64, input domain.MembershipTier) (domain.MembershipTier, error) {
	if tierID < 1 {
		return domain.MembershipTier{}, errors.New("会员阶层无效")
	}
	input.ID = tierID
	if err := normalizeAndValidateMembershipTier(&input); err != nil {
		return domain.MembershipTier{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.MembershipTier{}, err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, []int64{actorID}); err != nil {
		return domain.MembershipTier{}, err
	}
	settings, err := getMembershipSettings(tx, true)
	if err != nil {
		return domain.MembershipTier{}, err
	}
	found := false
	for index := range settings.Tiers {
		if settings.Tiers[index].ID == tierID {
			settings.Tiers[index] = input
			found = true
			break
		}
	}
	if !found {
		return domain.MembershipTier{}, errors.New("会员阶层不存在")
	}
	if settings.Enabled && !hasPurchasableTier(settings.Tiers) {
		return domain.MembershipTier{}, errors.New("会员购买已开放，至少需要保留一个已启用且有价格的阶层")
	}
	now := time.Now().UTC()
	if err := updateMembershipTierTx(tx, input, now); err != nil {
		return domain.MembershipTier{}, err
	}
	if err := syncLegacyMembershipSettings(tx, now); err != nil {
		return domain.MembershipTier{}, err
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'membership_tier', ?, 'update', ?, ?)`, actorID, tierID, input.Name, now); err != nil {
		return domain.MembershipTier{}, err
	}
	updated, err := getMembershipTier(tx, tierID, false)
	if err != nil {
		return domain.MembershipTier{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.MembershipTier{}, err
	}
	return updated, nil
}

func (s *Store) DeleteMembershipTier(actorID, tierID int64) (domain.MembershipTier, error) {
	if tierID < 1 {
		return domain.MembershipTier{}, errors.New("会员阶层无效")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.MembershipTier{}, err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, []int64{actorID}); err != nil {
		return domain.MembershipTier{}, err
	}
	settings, err := getMembershipSettings(tx, true)
	if err != nil {
		return domain.MembershipTier{}, err
	}
	var tier domain.MembershipTier
	found := false
	for _, candidate := range settings.Tiers {
		if candidate.ID == tierID {
			tier = candidate
			found = true
			break
		}
	}
	if !found {
		return domain.MembershipTier{}, errors.New("会员阶层不存在")
	}
	now := time.Now().UTC()
	var activeMemberID int64
	err = tx.QueryRow(`SELECT id FROM users WHERE membership_tier_id = ? AND status = 'active' AND membership_expires_at > ? ORDER BY id LIMIT 1 FOR UPDATE`, tierID, now).Scan(&activeMemberID)
	if err == nil {
		return domain.MembershipTier{}, errors.New("该阶层仍有有效会员，请先停用售卖并等待到期")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.MembershipTier{}, err
	}
	remaining := make([]domain.MembershipTier, 0, len(settings.Tiers)-1)
	for _, candidate := range settings.Tiers {
		if candidate.ID != tierID {
			remaining = append(remaining, candidate)
		}
	}
	if settings.Enabled && !hasPurchasableTier(remaining) {
		return domain.MembershipTier{}, errors.New("不能删除当前唯一可购买的会员阶层，请先暂停会员购买")
	}
	if _, err := tx.Exec(`UPDATE users SET membership_tier_id = NULL WHERE membership_tier_id = ?`, tierID); err != nil {
		return domain.MembershipTier{}, err
	}
	if _, err := tx.Exec(`DELETE FROM membership_tiers WHERE id = ?`, tierID); err != nil {
		return domain.MembershipTier{}, err
	}
	if err := syncLegacyMembershipSettings(tx, now); err != nil {
		return domain.MembershipTier{}, err
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'membership_tier', ?, 'delete', ?, ?)`, actorID, tierID, tier.Name, now); err != nil {
		return domain.MembershipTier{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.MembershipTier{}, err
	}
	return tier, nil
}

func listMembershipTiers(queryer sqlQueryer, forUpdate bool) ([]domain.MembershipTier, error) {
	query := `SELECT id, enabled, name, badge_label, badge_url, badge_color, monthly_price_cents, quarterly_price_cents, yearly_price_cents, post_review_exempt, feed_priority, withdrawal_fee_bps, service_fee_bps, sort_order, created_at, updated_at FROM membership_tiers ORDER BY sort_order, id`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	rows, err := queryer.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.MembershipTier, 0)
	for rows.Next() {
		item, err := scanMembershipTier(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func getMembershipTier(queryer rowQueryer, tierID int64, forUpdate bool) (domain.MembershipTier, error) {
	query := `SELECT id, enabled, name, badge_label, badge_url, badge_color, monthly_price_cents, quarterly_price_cents, yearly_price_cents, post_review_exempt, feed_priority, withdrawal_fee_bps, service_fee_bps, sort_order, created_at, updated_at FROM membership_tiers WHERE id = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	return scanMembershipTier(queryer.QueryRow(query, tierID))
}

type membershipTierScanner interface{ Scan(...any) error }

func scanMembershipTier(scanner membershipTierScanner) (domain.MembershipTier, error) {
	var item domain.MembershipTier
	var enabled, postReviewExempt, feedPriority int
	err := scanner.Scan(&item.ID, &enabled, &item.Name, &item.BadgeLabel, &item.BadgeURL, &item.BadgeColor, &item.MonthlyPriceCents, &item.QuarterlyPriceCents, &item.YearlyPriceCents, &postReviewExempt, &feedPriority, &item.WithdrawalFeeBPS, &item.ServiceFeeBPS, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, err
	}
	item.Enabled = enabled != 0
	item.PostReviewExempt = postReviewExempt != 0
	item.FeedPriority = feedPriority != 0
	if validateWebURL(item.BadgeURL, true) != nil {
		item.BadgeURL = ""
	}
	if !validPrimaryColor(item.BadgeColor) {
		item.BadgeColor = "#f59e0b"
	}
	item.WithdrawalFeeBPS = normalizedFeeBPS(item.WithdrawalFeeBPS, 300)
	item.ServiceFeeBPS = normalizedFeeBPS(item.ServiceFeeBPS, 500)
	return item, nil
}

func normalizeAndValidateMembershipTier(input *domain.MembershipTier) error {
	input.Name = strings.TrimSpace(input.Name)
	input.BadgeLabel = strings.TrimSpace(input.BadgeLabel)
	input.BadgeURL = strings.TrimSpace(input.BadgeURL)
	input.BadgeColor = strings.TrimSpace(input.BadgeColor)
	if input.BadgeColor == "" {
		input.BadgeColor = "#f59e0b"
	}
	return validateMembershipTier(*input)
}

func validateMembershipTier(input domain.MembershipTier) error {
	if len([]rune(input.Name)) < 1 || len([]rune(input.Name)) > 80 || len([]rune(input.BadgeLabel)) < 1 || len([]rune(input.BadgeLabel)) > 40 || containsControl(input.Name+input.BadgeLabel) {
		return errors.New("会员阶层名称或徽标文字无效")
	}
	if len(input.BadgeURL) > 500 || validateWebURL(input.BadgeURL, true) != nil {
		return errors.New("会员徽标地址无效")
	}
	if !validPrimaryColor(input.BadgeColor) {
		return errors.New("会员徽标颜色必须是六位十六进制颜色")
	}
	for _, price := range []int64{input.MonthlyPriceCents, input.QuarterlyPriceCents, input.YearlyPriceCents} {
		if price < 0 || price > maxMembershipPriceCents {
			return errors.New("会员价格无效")
		}
	}
	if err := validateFeeBPS(input.WithdrawalFeeBPS, "会员提现手续费"); err != nil {
		return err
	}
	if err := validateFeeBPS(input.ServiceFeeBPS, "会员服务费"); err != nil {
		return err
	}
	if input.SortOrder < -10000 || input.SortOrder > 10000 {
		return errors.New("会员阶层排序值无效")
	}
	return nil
}

func validateFeeBPS(value int, label string) error {
	if value < 0 || value > maxFeeBPS {
		return fmt.Errorf("%s必须在 0%% 到 100%% 之间", label)
	}
	return nil
}

func normalizedFeeBPS(value, fallback int) int {
	if value < 0 || value > maxFeeBPS {
		return fallback
	}
	return value
}

func updateMembershipTierTx(tx *sql.Tx, input domain.MembershipTier, now time.Time) error {
	_, err := tx.Exec(`UPDATE membership_tiers SET enabled = ?, name = ?, badge_label = ?, badge_url = ?, badge_color = ?, monthly_price_cents = ?, quarterly_price_cents = ?, yearly_price_cents = ?, post_review_exempt = ?, feed_priority = ?, withdrawal_fee_bps = ?, service_fee_bps = ?, sort_order = ?, updated_at = ? WHERE id = ?`,
		boolInt(input.Enabled), input.Name, input.BadgeLabel, input.BadgeURL, input.BadgeColor, input.MonthlyPriceCents, input.QuarterlyPriceCents, input.YearlyPriceCents, boolInt(input.PostReviewExempt), boolInt(input.FeedPriority), input.WithdrawalFeeBPS, input.ServiceFeeBPS, input.SortOrder, now, input.ID)
	return err
}

func syncLegacyMembershipSettings(tx *sql.Tx, now time.Time) error {
	var tier domain.MembershipTier
	var err error
	row := tx.QueryRow(`SELECT id, enabled, name, badge_label, badge_url, badge_color, monthly_price_cents, quarterly_price_cents, yearly_price_cents, post_review_exempt, feed_priority, withdrawal_fee_bps, service_fee_bps, sort_order, created_at, updated_at FROM membership_tiers ORDER BY sort_order, id LIMIT 1`)
	tier, err = scanMembershipTier(row)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.Exec(`UPDATE membership_settings SET name = '社区会员', badge_label = '会员', badge_url = '', badge_color = '#f59e0b', monthly_price_cents = 0, quarterly_price_cents = 0, yearly_price_cents = 0, post_review_exempt = 0, updated_at = ? WHERE id = 1`, now)
		return err
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE membership_settings SET name = ?, badge_label = ?, badge_url = ?, badge_color = ?, monthly_price_cents = ?, quarterly_price_cents = ?, yearly_price_cents = ?, post_review_exempt = ?, updated_at = ? WHERE id = 1`, tier.Name, tier.BadgeLabel, tier.BadgeURL, tier.BadgeColor, tier.MonthlyPriceCents, tier.QuarterlyPriceCents, tier.YearlyPriceCents, boolInt(tier.PostReviewExempt), now)
	return err
}

func applyLegacyTierFields(config *domain.MembershipSettings) {
	if len(config.Tiers) == 0 {
		config.Name = "社区会员"
		config.BadgeLabel = "会员"
		config.BadgeColor = "#f59e0b"
		return
	}
	tier := config.Tiers[0]
	config.Name = tier.Name
	config.BadgeLabel = tier.BadgeLabel
	config.BadgeURL = tier.BadgeURL
	config.BadgeColor = tier.BadgeColor
	config.MonthlyPriceCents = tier.MonthlyPriceCents
	config.QuarterlyPriceCents = tier.QuarterlyPriceCents
	config.YearlyPriceCents = tier.YearlyPriceCents
	config.PostReviewExempt = tier.PostReviewExempt
}

func hasPurchasableTier(tiers []domain.MembershipTier) bool {
	for _, tier := range tiers {
		if tier.Enabled && (tier.MonthlyPriceCents > 0 || tier.QuarterlyPriceCents > 0 || tier.YearlyPriceCents > 0) {
			return true
		}
	}
	return false
}

func (s *Store) GetMembershipSummary(userID int64) (domain.MembershipSummary, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.MembershipSummary{}, err
	}
	defer tx.Rollback()
	var status string
	var tierID sql.NullInt64
	var startedAt, expiresAt sql.NullTime
	if err := tx.QueryRow(`SELECT status, membership_tier_id, membership_started_at, membership_expires_at FROM users WHERE id = ?`, userID).Scan(&status, &tierID, &startedAt, &expiresAt); err != nil {
		return domain.MembershipSummary{}, err
	}
	if status != "active" {
		return domain.MembershipSummary{}, errors.New("用户不存在或已停用")
	}
	config, err := getMembershipSettings(tx, false)
	if err != nil {
		return domain.MembershipSummary{}, err
	}
	available, err := walletBalance(tx, userID)
	if err != nil {
		return domain.MembershipSummary{}, err
	}
	orders, err := listMembershipOrders(tx, userID, 30)
	if err != nil {
		return domain.MembershipSummary{}, err
	}
	summary := membershipSummary(config, status, tierID, startedAt, expiresAt, available, orders)
	if err := tx.Commit(); err != nil {
		return domain.MembershipSummary{}, err
	}
	return summary, nil
}

func (s *Store) SubscribeMembership(userID, tierID int64, plan string) (domain.MembershipSummary, error) {
	plan = strings.ToLower(strings.TrimSpace(plan))
	if tierID < 1 || (plan != "monthly" && plan != "quarterly" && plan != "yearly") {
		return domain.MembershipSummary{}, errors.New("会员套餐无效")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.MembershipSummary{}, err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, []int64{userID}); err != nil {
		return domain.MembershipSummary{}, err
	}
	var currentTierID sql.NullInt64
	var startedAt, expiresAt sql.NullTime
	if err := tx.QueryRow(`SELECT membership_tier_id, membership_started_at, membership_expires_at FROM users WHERE id = ? FOR UPDATE`, userID).Scan(&currentTierID, &startedAt, &expiresAt); err != nil {
		return domain.MembershipSummary{}, err
	}
	config, err := getMembershipSettings(tx, true)
	if err != nil {
		return domain.MembershipSummary{}, err
	}
	if !config.Enabled {
		return domain.MembershipSummary{}, errors.New("会员暂未开放购买")
	}
	tier, ok := membershipTierByID(config.Tiers, tierID)
	if !ok || !tier.Enabled {
		return domain.MembershipSummary{}, errors.New("该会员阶层暂不可购买")
	}
	price, months := membershipPlan(tier, plan)
	if price <= 0 || months <= 0 {
		return domain.MembershipSummary{}, errors.New("该会员套餐暂不可购买")
	}
	now := time.Now().UTC()
	if expiresAt.Valid && expiresAt.Time.After(now) && currentTierID.Valid && currentTierID.Int64 != tierID {
		return domain.MembershipSummary{}, errors.New("当前会员未到期，只能续费同一阶层")
	}
	available, err := walletBalance(tx, userID)
	if err != nil {
		return domain.MembershipSummary{}, err
	}
	if available < price {
		return domain.MembershipSummary{}, errors.New("钱包余额不足，请先充值")
	}

	periodStart := now
	membershipStarted := now
	if expiresAt.Valid && expiresAt.Time.After(now) {
		periodStart = expiresAt.Time.UTC()
		if startedAt.Valid {
			membershipStarted = startedAt.Time.UTC()
		}
	}
	newExpiresAt := periodStart.AddDate(0, months, 0)
	orderNo, err := randomMembershipOrderNo()
	if err != nil {
		return domain.MembershipSummary{}, err
	}
	result, err := tx.Exec(`INSERT INTO membership_orders (order_no, user_id, membership_tier_id, tier_name, plan, amount_cents, duration_months, started_at, expires_at, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'paid', ?, ?)`, orderNo, userID, tier.ID, tier.Name, plan, price, months, periodStart, newExpiresAt, now, now)
	if err != nil {
		return domain.MembershipSummary{}, err
	}
	orderID, err := result.LastInsertId()
	if err != nil {
		return domain.MembershipSummary{}, err
	}
	if _, err := tx.Exec(`INSERT INTO wallet_ledgers (user_id, entry_type, amount_cents, reference_type, reference_id, note, created_at) VALUES (?, 'membership_purchase', ?, 'membership_order', ?, ?, ?)`, userID, -price, orderID, fmt.Sprintf("%s %s", tier.Name, membershipPlanLabel(plan)), now); err != nil {
		return domain.MembershipSummary{}, err
	}
	update, err := tx.Exec(`UPDATE users SET membership_tier_id = ?, membership_started_at = ?, membership_expires_at = ?, updated_at = ? WHERE id = ? AND status = 'active'`, tier.ID, membershipStarted, newExpiresAt, now, userID)
	if err != nil {
		return domain.MembershipSummary{}, err
	}
	if affected, err := update.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return domain.MembershipSummary{}, err
		}
		return domain.MembershipSummary{}, errors.New("会员状态更新失败")
	}
	available -= price
	orders, err := listMembershipOrders(tx, userID, 30)
	if err != nil {
		return domain.MembershipSummary{}, err
	}
	startedAt = sql.NullTime{Time: membershipStarted, Valid: true}
	expiresAt = sql.NullTime{Time: newExpiresAt, Valid: true}
	currentTierID = sql.NullInt64{Int64: tier.ID, Valid: true}
	summary := membershipSummary(config, "active", currentTierID, startedAt, expiresAt, available, orders)
	if err := tx.Commit(); err != nil {
		return domain.MembershipSummary{}, err
	}
	return summary, nil
}

func membershipSummary(config domain.MembershipSettings, userStatus string, tierID sql.NullInt64, startedAt, expiresAt sql.NullTime, available int64, orders []domain.MembershipOrder) domain.MembershipSummary {
	result := domain.MembershipSummary{Config: config, AvailableCents: available, Orders: orders}
	if startedAt.Valid {
		value := startedAt.Time.UTC()
		result.StartedAt = &value
	}
	if expiresAt.Valid {
		value := expiresAt.Time.UTC()
		result.ExpiresAt = &value
		if tierID.Valid && userStatus == "active" && value.After(time.Now().UTC()) {
			if tier, ok := membershipTierByID(config.Tiers, tierID.Int64); ok {
				result.Active = true
				copy := tier
				result.CurrentTier = &copy
			}
		}
	}
	return result
}

func listMembershipOrders(queryer sqlQueryer, userID int64, limit int) ([]domain.MembershipOrder, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	rows, err := queryer.Query(`SELECT id, order_no, user_id, COALESCE(membership_tier_id, 0), tier_name, plan, amount_cents, duration_months, started_at, expires_at, status, created_at, updated_at FROM membership_orders WHERE user_id = ? ORDER BY created_at DESC, id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.MembershipOrder, 0)
	for rows.Next() {
		var item domain.MembershipOrder
		if err := rows.Scan(&item.ID, &item.OrderNo, &item.UserID, &item.MembershipTierID, &item.TierName, &item.Plan, &item.AmountCents, &item.DurationMonths, &item.StartedAt, &item.ExpiresAt, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func membershipTierByID(tiers []domain.MembershipTier, id int64) (domain.MembershipTier, bool) {
	for _, tier := range tiers {
		if tier.ID == id {
			return tier, true
		}
	}
	return domain.MembershipTier{}, false
}

func membershipPlan(tier domain.MembershipTier, plan string) (int64, int) {
	switch plan {
	case "monthly":
		return tier.MonthlyPriceCents, 1
	case "quarterly":
		return tier.QuarterlyPriceCents, 3
	case "yearly":
		return tier.YearlyPriceCents, 12
	default:
		return 0, 0
	}
}

func membershipPlanLabel(plan string) string {
	switch plan {
	case "monthly":
		return "月度会员"
	case "quarterly":
		return "季度会员"
	case "yearly":
		return "年度会员"
	default:
		return "会员"
	}
}

func (s *Store) UserFeePolicy(userID int64) (domain.FeePolicy, error) {
	return feePolicyWithQuery(s.db, userID, false)
}

func feePolicyWithQuery(queryer rowQueryer, userID int64, forUpdate bool) (domain.FeePolicy, error) {
	var policy domain.FeePolicy
	var userStatus string
	var tierID sql.NullInt64
	var expiresAt sql.NullTime
	var tierName sql.NullString
	var tierWithdrawal, tierService sql.NullInt64
	query := `SELECT ms.default_withdrawal_fee_bps, ms.default_service_fee_bps, u.status, u.membership_tier_id, u.membership_expires_at, t.name, t.withdrawal_fee_bps, t.service_fee_bps FROM users u JOIN membership_settings ms ON ms.id = 1 LEFT JOIN membership_tiers t ON t.id = u.membership_tier_id WHERE u.id = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	if err := queryer.QueryRow(query, userID).Scan(&policy.WithdrawalFeeBPS, &policy.ServiceFeeBPS, &userStatus, &tierID, &expiresAt, &tierName, &tierWithdrawal, &tierService); err != nil {
		return domain.FeePolicy{}, err
	}
	policy.WithdrawalFeeBPS = normalizedFeeBPS(policy.WithdrawalFeeBPS, 300)
	policy.ServiceFeeBPS = normalizedFeeBPS(policy.ServiceFeeBPS, 500)
	if userStatus == "active" && tierID.Valid && expiresAt.Valid && expiresAt.Time.After(time.Now().UTC()) && tierName.Valid && tierWithdrawal.Valid && tierService.Valid {
		policy.MembershipTierID = tierID.Int64
		policy.MembershipTierName = tierName.String
		policy.WithdrawalFeeBPS = normalizedFeeBPS(int(tierWithdrawal.Int64), policy.WithdrawalFeeBPS)
		policy.ServiceFeeBPS = normalizedFeeBPS(int(tierService.Int64), policy.ServiceFeeBPS)
	}
	return policy, nil
}

func feeAmountCents(amountCents int64, feeBPS int) int64 {
	if amountCents <= 0 || feeBPS <= 0 {
		return 0
	}
	if feeBPS >= maxFeeBPS {
		return amountCents
	}
	// Split the quotient and remainder so very large valid ledger balances do
	// not overflow int64 before division. The remainder term is rounded up to
	// a whole cent, matching the displayed and persisted fee.
	whole := (amountCents / maxFeeBPS) * int64(feeBPS)
	remainder := amountCents % maxFeeBPS
	return whole + (remainder*int64(feeBPS)+maxFeeBPS-1)/maxFeeBPS
}

func nullablePositiveID(id int64) any {
	if id > 0 {
		return id
	}
	return nil
}

func randomMembershipOrderNo() (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "RBM" + time.Now().UTC().Format("20060102150405") + strings.ToUpper(hex.EncodeToString(raw)), nil
}

func applyUserMembership(user *domain.User, tierID sql.NullInt64, startedAt, expiresAt sql.NullTime) {
	if startedAt.Valid {
		value := startedAt.Time.UTC()
		user.MembershipStartedAt = &value
	}
	if expiresAt.Valid {
		value := expiresAt.Time.UTC()
		user.MembershipExpiresAt = &value
		user.MemberActive = user.Status == "active" && tierID.Valid && value.After(time.Now().UTC())
		if user.MemberActive {
			user.MembershipTierID = tierID.Int64
		}
	}
}
