package store

import (
	"database/sql"
	"errors"
	"net/url"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

var robloxMusicSubmissionColumns = `s.id, s.user_id, u.display_name, u.avatar_url, s.asset_id, s.name, s.image_url, s.status, s.review_note, s.reviewed_by, s.reviewed_at, s.created_at, s.updated_at`

func normalizeRobloxMusicSubmission(assetID int64, name, imageURL string) (string, string, error) {
	name = strings.TrimSpace(name)
	imageURL = strings.TrimSpace(imageURL)
	if assetID <= 0 {
		return "", "", errors.New("Roblox 音乐 ID 无效")
	}
	if name == "" || len([]rune(name)) > 120 || containsControl(name) {
		return "", "", errors.New("音乐名称不能为空且不能超过 120 个字符")
	}
	if len(imageURL) > 500 || containsControl(imageURL) {
		return "", "", errors.New("音乐图片地址无效")
	}
	parsed, err := url.Parse(imageURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", "", errors.New("音乐图片必须使用 http 或 https 地址")
	}
	return name, imageURL, nil
}

func scanRobloxMusicSubmission(scanner rowScanner) (domain.RobloxMusicSubmission, error) {
	var item domain.RobloxMusicSubmission
	var reviewedBy sql.NullInt64
	var reviewedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.UserID, &item.UserName, &item.UserAvatar, &item.AssetID, &item.Name, &item.ImageURL, &item.Status, &item.ReviewNote, &reviewedBy, &reviewedAt, &item.CreatedAt, &item.UpdatedAt)
	if reviewedBy.Valid {
		item.ReviewedBy = reviewedBy.Int64
	}
	if reviewedAt.Valid {
		value := reviewedAt.Time
		item.ReviewedAt = &value
	}
	return item, err
}

func (s *Store) ListApprovedRobloxMusic(query string) ([]domain.RobloxMusic, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) > 100 {
		query = string([]rune(query)[:100])
	}
	base := `SELECT s.asset_id, s.name, s.image_url, s.created_at FROM roblox_music_submissions s WHERE s.status = 'approved'`
	args := []any{}
	if query != "" {
		base += ` AND (s.name LIKE ? OR CAST(s.asset_id AS CHAR) LIKE ?)`
		like := "%" + query + "%"
		args = append(args, like, like)
	}
	base += ` ORDER BY s.created_at DESC, s.id DESC LIMIT 200`
	rows, err := s.db.Query(base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.RobloxMusic, 0)
	for rows.Next() {
		var item domain.RobloxMusic
		if err := rows.Scan(&item.AssetID, &item.Name, &item.ThumbnailURL, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateRobloxMusicSubmission(userID, assetID int64, name, imageURL string) (domain.RobloxMusicSubmission, error) {
	name, imageURL, err := normalizeRobloxMusicSubmission(assetID, name, imageURL)
	if err != nil {
		return domain.RobloxMusicSubmission{}, err
	}
	var existingID, ownerID int64
	var status string
	err = s.db.QueryRow(`SELECT id, user_id, status FROM roblox_music_submissions WHERE asset_id = ?`, assetID).Scan(&existingID, &ownerID, &status)
	now := time.Now().UTC()
	if err == nil {
		if status == "approved" {
			return domain.RobloxMusicSubmission{}, errors.New("该音乐已在音乐库中")
		}
		if status == "pending" {
			return domain.RobloxMusicSubmission{}, errors.New("该音乐正在审核中")
		}
		if ownerID != userID {
			return domain.RobloxMusicSubmission{}, errors.New("该音乐投稿已被驳回，请勿重复投稿")
		}
		if _, err := s.db.Exec(`UPDATE roblox_music_submissions SET name = ?, image_url = ?, status = 'pending', review_note = '', reviewed_by = NULL, reviewed_at = NULL, updated_at = ? WHERE id = ?`, name, imageURL, now, existingID); err != nil {
			return domain.RobloxMusicSubmission{}, err
		}
		return s.robloxMusicSubmissionByID(s.db, existingID, false)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.RobloxMusicSubmission{}, err
	}
	result, err := s.db.Exec(`INSERT INTO roblox_music_submissions (user_id, asset_id, name, image_url, status, review_note, created_at, updated_at) VALUES (?, ?, ?, ?, 'pending', '', ?, ?)`, userID, assetID, name, imageURL, now, now)
	if err != nil {
		return domain.RobloxMusicSubmission{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.RobloxMusicSubmission{}, err
	}
	return s.robloxMusicSubmissionByID(s.db, id, false)
}

func (s *Store) robloxMusicSubmissionByID(queryer rowQueryer, id int64, forUpdate bool) (domain.RobloxMusicSubmission, error) {
	query := `SELECT ` + robloxMusicSubmissionColumns + ` FROM roblox_music_submissions s JOIN users u ON u.id = s.user_id WHERE s.id = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	return scanRobloxMusicSubmission(queryer.QueryRow(query, id))
}

func (s *Store) ListMyRobloxMusicSubmissions(userID int64) ([]domain.RobloxMusicSubmission, error) {
	return s.listRobloxMusicSubmissions(`WHERE s.user_id = ?`, userID)
}

func (s *Store) ListAdminRobloxMusicSubmissions(status string) ([]domain.RobloxMusicSubmission, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "" && status != "pending" && status != "approved" && status != "rejected" {
		return nil, errors.New("音乐投稿状态无效")
	}
	if status == "" {
		return s.listRobloxMusicSubmissions("")
	}
	return s.listRobloxMusicSubmissions(`WHERE s.status = ?`, status)
}

func (s *Store) listRobloxMusicSubmissions(where string, args ...any) ([]domain.RobloxMusicSubmission, error) {
	rows, err := s.db.Query(`SELECT `+robloxMusicSubmissionColumns+` FROM roblox_music_submissions s JOIN users u ON u.id = s.user_id `+where+` ORDER BY s.created_at DESC, s.id DESC LIMIT 200`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.RobloxMusicSubmission, 0)
	for rows.Next() {
		item, err := scanRobloxMusicSubmission(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ReviewRobloxMusicSubmission(actorID, submissionID int64, status, note string) (domain.RobloxMusicSubmission, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	note = strings.TrimSpace(note)
	if status != "approved" && status != "rejected" {
		return domain.RobloxMusicSubmission{}, errors.New("音乐投稿审核状态无效")
	}
	if status == "rejected" && note == "" {
		return domain.RobloxMusicSubmission{}, errors.New("请填写驳回原因")
	}
	if len([]rune(note)) > 500 || containsControl(note) {
		return domain.RobloxMusicSubmission{}, errors.New("审核备注不能超过 500 个字符")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.RobloxMusicSubmission{}, err
	}
	defer tx.Rollback()
	item, err := s.robloxMusicSubmissionByID(tx, submissionID, true)
	if err != nil {
		return domain.RobloxMusicSubmission{}, err
	}
	if item.Status != "pending" {
		return domain.RobloxMusicSubmission{}, errors.New("该音乐投稿已处理")
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`UPDATE roblox_music_submissions SET status = ?, review_note = ?, reviewed_by = ?, reviewed_at = ?, updated_at = ? WHERE id = ?`, status, note, actorID, now, now, submissionID); err != nil {
		return domain.RobloxMusicSubmission{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.RobloxMusicSubmission{}, err
	}
	return s.robloxMusicSubmissionByID(s.db, submissionID, false)
}

func (s *Store) ApprovedRobloxMusic(assetID int64) (domain.RobloxMusic, error) {
	var item domain.RobloxMusic
	err := s.db.QueryRow(`SELECT asset_id, name, image_url, created_at FROM roblox_music_submissions WHERE asset_id = ? AND status = 'approved'`, assetID).Scan(&item.AssetID, &item.Name, &item.ThumbnailURL, &item.CreatedAt)
	return item, err
}
