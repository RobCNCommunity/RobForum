package store

import (
	"database/sql"
	"errors"
	"time"

	"roblox-community/internal/domain"
)

var qualificationLevels = []struct {
	name string
	min  int64
}{
	{name: "新芽", min: 0},
	{name: "熟面孔", min: 100},
	{name: "活跃玩家", min: 300},
	{name: "资深玩家", min: 800},
	{name: "社区达人", min: 1600},
	{name: "社区元老", min: 3000},
}

var checkinLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

func progressForExperience(experience int64) domain.UserProgress {
	if experience < 0 {
		experience = 0
	}
	index := 0
	for i := range qualificationLevels {
		if experience >= qualificationLevels[i].min {
			index = i
		}
	}
	level := qualificationLevels[index]
	result := domain.UserProgress{
		Experience:         experience,
		Level:              index + 1,
		LevelName:          level.name,
		LevelMinExperience: level.min,
		LevelProgress:      100,
	}
	if index+1 < len(qualificationLevels) {
		next := qualificationLevels[index+1].min
		result.NextLevelExperience = next
		span := next - level.min
		if span > 0 {
			result.LevelProgress = int((experience - level.min) * 100 / span)
			if result.LevelProgress < 0 {
				result.LevelProgress = 0
			}
			if result.LevelProgress > 100 {
				result.LevelProgress = 100
			}
		}
	}
	return result
}

func (s *Store) ensureBadgeDefaults(now time.Time) error {
	badges := []struct {
		slug, name, description, icon, color string
	}{
		{"new_member", "初来乍到", "加入社区，开启玩家旅程。", "user", "#1d9bf0"},
		{"first_checkin", "第一次签到", "完成首次每日签到。", "calendar", "#00a86b"},
		{"streak_7", "七日同行", "连续签到 7 天。", "flame", "#f97316"},
		{"checkin_30", "签到达人", "累计签到 30 天。", "award", "#8b5cf6"},
		{"creator_10", "内容创作者", "在社区发布 10 篇内容。", "edit", "#e11d48"},
	}
	for _, badge := range badges {
		if _, err := s.db.Exec(`INSERT INTO badges (slug, name, description, icon, color, created_at) VALUES (?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE name = VALUES(name), description = VALUES(description), icon = VALUES(icon), color = VALUES(color)`, badge.slug, badge.name, badge.description, badge.icon, badge.color, now); err != nil {
			return err
		}
	}
	_, err := s.db.Exec(`INSERT IGNORE INTO user_badges (user_id, badge_id, awarded_at) SELECT u.id, b.id, u.created_at FROM users u JOIN badges b ON b.slug = 'new_member'`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT IGNORE INTO user_badges (user_id, badge_id, awarded_at) SELECT p.author_id, b.id, ? FROM (SELECT author_id FROM posts WHERE status IN ('published', 'pending') GROUP BY author_id HAVING COUNT(*) >= 10) p JOIN badges b ON b.slug = 'creator_10'`, now)
	return err
}

func awardBadgeTx(tx *sql.Tx, userID int64, slug string, awardedAt time.Time) (bool, error) {
	result, err := tx.Exec(`INSERT IGNORE INTO user_badges (user_id, badge_id, awarded_at) SELECT ?, id, ? FROM badges WHERE slug = ?`, userID, awardedAt, slug)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *Store) GetUserProgress(userID int64) (domain.UserProgress, error) {
	var experience int64
	var total, current, longest int
	var last sql.NullTime
	err := s.db.QueryRow(`SELECT experience, total_checkins, current_streak, longest_streak, last_checkin_date FROM user_progress WHERE user_id = ?`, userID).Scan(&experience, &total, &current, &longest, &last)
	if errors.Is(err, sql.ErrNoRows) {
		return progressForExperience(0), nil
	}
	if err != nil {
		return domain.UserProgress{}, err
	}
	result := progressForExperience(experience)
	result.TotalCheckins = total
	result.CurrentStreak = current
	result.LongestStreak = longest
	if last.Valid {
		result.LastCheckinDate = last.Time.In(checkinLocation).Format("2006-01-02")
		result.CheckedInToday = result.LastCheckinDate == time.Now().In(checkinLocation).Format("2006-01-02")
	}
	return result, nil
}

func (s *Store) ListUserBadges(userID int64) ([]domain.Badge, error) {
	rows, err := s.db.Query(`SELECT b.id, b.slug, b.name, b.description, b.icon, b.color, ub.awarded_at FROM user_badges ub JOIN badges b ON b.id = ub.badge_id WHERE ub.user_id = ? ORDER BY ub.awarded_at DESC, b.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.Badge, 0)
	for rows.Next() {
		var badge domain.Badge
		if err := rows.Scan(&badge.ID, &badge.Slug, &badge.Name, &badge.Description, &badge.Icon, &badge.Color, &badge.AwardedAt); err != nil {
			return nil, err
		}
		result = append(result, badge)
	}
	return result, rows.Err()
}

func (s *Store) GetCheckinSummary(userID int64) (domain.CheckinSummary, error) {
	progress, err := s.GetUserProgress(userID)
	if err != nil {
		return domain.CheckinSummary{}, err
	}
	badges, err := s.ListUserBadges(userID)
	if err != nil {
		return domain.CheckinSummary{}, err
	}
	rows, err := s.db.Query(`SELECT checkin_date, experience_awarded, streak, created_at FROM user_checkins WHERE user_id = ? ORDER BY checkin_date DESC LIMIT 35`, userID)
	if err != nil {
		return domain.CheckinSummary{}, err
	}
	defer rows.Close()
	history := make([]domain.CheckinRecord, 0)
	for rows.Next() {
		var record domain.CheckinRecord
		var date time.Time
		if err := rows.Scan(&date, &record.ExperienceAwarded, &record.Streak, &record.CreatedAt); err != nil {
			return domain.CheckinSummary{}, err
		}
		record.Date = date.In(checkinLocation).Format("2006-01-02")
		history = append(history, record)
	}
	if err := rows.Err(); err != nil {
		return domain.CheckinSummary{}, err
	}
	return domain.CheckinSummary{Progress: progress, History: history, Badges: badges, NewlyAwarded: []domain.Badge{}}, nil
}

func (s *Store) Checkin(userID int64) (domain.CheckinSummary, error) {
	now := time.Now().UTC()
	localNow := now.In(checkinLocation)
	today := localNow.Format("2006-01-02")
	yesterday := localNow.AddDate(0, 0, -1).Format("2006-01-02")

	tx, err := s.db.Begin()
	if err != nil {
		return domain.CheckinSummary{}, err
	}
	defer tx.Rollback()
	var status string
	if err := tx.QueryRow(`SELECT status FROM users WHERE id = ? FOR UPDATE`, userID).Scan(&status); err != nil || status != "active" {
		if err != nil {
			return domain.CheckinSummary{}, err
		}
		return domain.CheckinSummary{}, errors.New("账号当前不可签到")
	}
	var experience int64
	var total, current, longest int
	var last sql.NullTime
	err = tx.QueryRow(`SELECT experience, total_checkins, current_streak, longest_streak, last_checkin_date FROM user_progress WHERE user_id = ? FOR UPDATE`, userID).Scan(&experience, &total, &current, &longest, &last)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := tx.Exec(`INSERT INTO user_progress (user_id, experience, total_checkins, current_streak, longest_streak, last_checkin_date, updated_at) VALUES (?, 0, 0, 0, 0, NULL, ?)`, userID, now); err != nil {
			return domain.CheckinSummary{}, err
		}
		last = sql.NullTime{}
	} else if err != nil {
		return domain.CheckinSummary{}, err
	}
	lastDate := ""
	if last.Valid {
		lastDate = last.Time.In(checkinLocation).Format("2006-01-02")
	}
	if lastDate == today {
		_ = tx.Rollback()
		return s.GetCheckinSummary(userID)
	}
	if lastDate == yesterday {
		current++
	} else {
		current = 1
	}
	if current > longest {
		longest = current
	}
	total++
	bonusDays := current - 1
	if bonusDays > 6 {
		bonusDays = 6
	}
	awardedExperience := 10 + bonusDays*2
	experience += int64(awardedExperience)
	if _, err := tx.Exec(`INSERT INTO user_checkins (user_id, checkin_date, experience_awarded, streak, created_at) VALUES (?, ?, ?, ?, ?)`, userID, today, awardedExperience, current, now); err != nil {
		return domain.CheckinSummary{}, err
	}
	if _, err := tx.Exec(`UPDATE user_progress SET experience = ?, total_checkins = ?, current_streak = ?, longest_streak = ?, last_checkin_date = ?, updated_at = ? WHERE user_id = ?`, experience, total, current, longest, today, now, userID); err != nil {
		return domain.CheckinSummary{}, err
	}
	newSlugs := make([]string, 0, 3)
	for _, candidate := range []struct {
		slug string
		met  bool
	}{
		{slug: "first_checkin", met: total >= 1},
		{slug: "streak_7", met: current >= 7},
		{slug: "checkin_30", met: total >= 30},
	} {
		if !candidate.met {
			continue
		}
		awarded, err := awardBadgeTx(tx, userID, candidate.slug, now)
		if err != nil {
			return domain.CheckinSummary{}, err
		}
		if awarded {
			newSlugs = append(newSlugs, candidate.slug)
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.CheckinSummary{}, err
	}
	summary, err := s.GetCheckinSummary(userID)
	if err != nil {
		return domain.CheckinSummary{}, err
	}
	for _, badge := range summary.Badges {
		for _, slug := range newSlugs {
			if badge.Slug == slug {
				summary.NewlyAwarded = append(summary.NewlyAwarded, badge)
			}
		}
	}
	return summary, nil
}
