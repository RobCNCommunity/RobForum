package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

func (s *Store) ListPublicAds() ([]domain.AdSlot, error) {
	rows, err := s.db.Query(`SELECT id, title, image_url, link_url, sort_order, enabled, created_at, updated_at FROM ad_slots WHERE enabled = 1 ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.AdSlot, 0)
	for rows.Next() {
		item, err := scanAd(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) ListAdminAds() ([]domain.AdSlot, error) {
	rows, err := s.db.Query(`SELECT id, title, image_url, link_url, sort_order, enabled, created_at, updated_at FROM ad_slots ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.AdSlot, 0)
	for rows.Next() {
		item, err := scanAd(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

type adScanner interface {
	Scan(dest ...any) error
}

func scanAd(row adScanner) (domain.AdSlot, error) {
	var item domain.AdSlot
	var enabled int
	err := row.Scan(&item.ID, &item.Title, &item.ImageURL, &item.LinkURL, &item.SortOrder, &enabled, &item.CreatedAt, &item.UpdatedAt)
	item.Enabled = enabled != 0
	if validateWebURL(item.ImageURL, true) != nil {
		item.ImageURL = ""
	}
	if validateWebURL(item.LinkURL, true) != nil {
		item.LinkURL = ""
	}
	return item, err
}

func (s *Store) CreateAd(actorID int64, title, imageURL, linkURL string, sortOrder int, enabled bool) (domain.AdSlot, error) {
	title = strings.TrimSpace(title)
	imageURL = strings.TrimSpace(imageURL)
	linkURL = strings.TrimSpace(linkURL)
	if imageURL == "" && title == "" {
		return domain.AdSlot{}, errors.New("广告标题或图片至少填一项")
	}
	if len([]rune(title)) > 120 || sortOrder < -10000 || sortOrder > 10000 || containsControl(title) || validateWebURL(imageURL, true) != nil || validateWebURL(linkURL, true) != nil {
		return domain.AdSlot{}, errors.New("广告内容或链接无效")
	}
	now := time.Now().UTC()
	en := 0
	if enabled {
		en = 1
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.AdSlot{}, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO ad_slots (title, image_url, link_url, sort_order, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, title, imageURL, linkURL, sortOrder, en, now, now)
	if err != nil {
		return domain.AdSlot{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.AdSlot{}, err
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'ad', ?, 'create', '', ?)`, actorID, id, now); err != nil {
		return domain.AdSlot{}, err
	}
	item, err := getAd(tx, id)
	if err != nil {
		return domain.AdSlot{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.AdSlot{}, err
	}
	return item, nil
}

func (s *Store) GetAd(id int64) (domain.AdSlot, error) {
	return getAd(s.db, id)
}

func getAd(queryer rowQueryer, id int64) (domain.AdSlot, error) {
	row := queryer.QueryRow(`SELECT id, title, image_url, link_url, sort_order, enabled, created_at, updated_at FROM ad_slots WHERE id = ?`, id)
	return scanAd(row)
}

func (s *Store) UpdateAd(actorID, id int64, title, imageURL, linkURL string, sortOrder int, enabled bool) (domain.AdSlot, error) {
	title = strings.TrimSpace(title)
	imageURL = strings.TrimSpace(imageURL)
	linkURL = strings.TrimSpace(linkURL)
	if imageURL == "" && title == "" {
		return domain.AdSlot{}, errors.New("广告标题或图片至少填一项")
	}
	if len([]rune(title)) > 120 || sortOrder < -10000 || sortOrder > 10000 || containsControl(title) || validateWebURL(imageURL, true) != nil || validateWebURL(linkURL, true) != nil {
		return domain.AdSlot{}, errors.New("广告内容或链接无效")
	}
	en := 0
	if enabled {
		en = 1
	}
	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return domain.AdSlot{}, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE ad_slots SET title = ?, image_url = ?, link_url = ?, sort_order = ?, enabled = ?, updated_at = ? WHERE id = ?`, title, imageURL, linkURL, sortOrder, en, now, id)
	if err != nil {
		return domain.AdSlot{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return domain.AdSlot{}, err
	}
	if n == 0 {
		return domain.AdSlot{}, errors.New("广告不存在")
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'ad', ?, 'update', '', ?)`, actorID, id, now); err != nil {
		return domain.AdSlot{}, err
	}
	item, err := getAd(tx, id)
	if err != nil {
		return domain.AdSlot{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.AdSlot{}, err
	}
	return item, nil
}

func (s *Store) DeleteAd(actorID, id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`DELETE FROM ad_slots WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("广告不存在")
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'ad', ?, 'delete', '', ?)`, actorID, id, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

const maxNoticeMediaCount = 4

var noticeMediaURLPattern = regexp.MustCompile(`^/api/v1/media/news/[0-9a-f]{32,40}\.(png|jpe?g)$`)

func (s *Store) ListPublicNotices(limit int) ([]domain.Notice, error) {
	if limit < 1 || limit > 50 {
		limit = 20
	}
	rows, err := s.db.Query(`SELECT id, title, content, link_url, media_json, level, pinned, enabled, COALESCE(created_by, 0), created_at, updated_at FROM notices WHERE enabled = 1 ORDER BY updated_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.Notice, 0)
	for rows.Next() {
		item, err := scanNotice(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) ListAdminNotices() ([]domain.Notice, error) {
	rows, err := s.db.Query(`SELECT id, title, content, link_url, media_json, level, pinned, enabled, COALESCE(created_by, 0), created_at, updated_at FROM notices ORDER BY pinned DESC, updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.Notice, 0)
	for rows.Next() {
		item, err := scanNotice(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func scanNotice(row adScanner) (domain.Notice, error) {
	var item domain.Notice
	var pinned, enabled int
	var mediaJSON string
	err := row.Scan(&item.ID, &item.Title, &item.Content, &item.LinkURL, &mediaJSON, &item.Level, &pinned, &enabled, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt)
	if err == nil && strings.TrimSpace(mediaJSON) != "" {
		if decodeErr := json.Unmarshal([]byte(mediaJSON), &item.Media); decodeErr != nil {
			return domain.Notice{}, decodeErr
		}
	}
	item.Pinned = pinned != 0
	item.Enabled = enabled != 0
	return item, err
}

func normalizeNoticeMedia(media []domain.NoticeMedia) ([]domain.NoticeMedia, string, error) {
	if len(media) > maxNoticeMediaCount {
		return nil, "", errors.New("新闻最多上传 4 张图片")
	}
	seen := make(map[string]struct{}, len(media))
	for index := range media {
		item := &media[index]
		item.URL = strings.TrimSpace(item.URL)
		item.MIMEType = strings.ToLower(strings.TrimSpace(item.MIMEType))
		if !noticeMediaURLPattern.MatchString(item.URL) || item.MIMEType != "image/png" && item.MIMEType != "image/jpeg" || item.Width < 16 || item.Height < 16 || item.SizeBytes < 1 {
			return nil, "", errors.New("新闻图片链接无效")
		}
		if _, ok := seen[item.URL]; ok {
			return nil, "", errors.New("新闻图片不能重复")
		}
		seen[item.URL] = struct{}{}
		item.ID = 0
	}
	if media == nil {
		media = []domain.NoticeMedia{}
	}
	encoded, err := json.Marshal(media)
	if err != nil {
		return nil, "", err
	}
	return media, string(encoded), nil
}

func (s *Store) CreateNotice(userID int64, title, content, linkURL, level string, pinned, enabled bool, media []domain.NoticeMedia) (domain.Notice, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	linkURL = strings.TrimSpace(linkURL)
	level = strings.TrimSpace(level)
	if title == "" || content == "" {
		return domain.Notice{}, errors.New("公告标题和内容不能为空")
	}
	if level == "" {
		level = "info"
	}
	if len([]rune(title)) > 160 || len([]rune(content)) > 10000 || containsControl(title) || containsControl(content) {
		return domain.Notice{}, errors.New("公告标题或内容过长")
	}
	if err := validateWebURL(linkURL, false); err != nil {
		return domain.Notice{}, errors.New("公告链接无效")
	}
	if level != "info" && level != "success" && level != "warning" && level != "error" {
		return domain.Notice{}, errors.New("公告级别无效")
	}
	media, mediaJSON, err := normalizeNoticeMedia(media)
	if err != nil {
		return domain.Notice{}, err
	}
	now := time.Now().UTC()
	p, e := 0, 0
	if pinned {
		p = 1
	}
	if enabled {
		e = 1
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Notice{}, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO notices (title, content, link_url, media_json, level, pinned, enabled, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, title, content, linkURL, mediaJSON, level, p, e, userID, now, now)
	if err != nil {
		return domain.Notice{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.Notice{}, err
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'notice', ?, 'create', '', ?)`, userID, id, now); err != nil {
		return domain.Notice{}, err
	}
	item, err := getNotice(tx, id)
	if err != nil {
		return domain.Notice{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Notice{}, err
	}
	return item, nil
}

func (s *Store) GetNotice(id int64) (domain.Notice, error) {
	return getNotice(s.db, id)
}

func getNotice(queryer rowQueryer, id int64) (domain.Notice, error) {
	row := queryer.QueryRow(`SELECT id, title, content, link_url, media_json, level, pinned, enabled, COALESCE(created_by, 0), created_at, updated_at FROM notices WHERE id = ?`, id)
	return scanNotice(row)
}

func (s *Store) UpdateNotice(actorID, id int64, title, content, linkURL, level string, pinned, enabled bool, media []domain.NoticeMedia) (domain.Notice, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	linkURL = strings.TrimSpace(linkURL)
	level = strings.TrimSpace(level)
	if title == "" || content == "" {
		return domain.Notice{}, errors.New("公告标题和内容不能为空")
	}
	if level == "" {
		level = "info"
	}
	if len([]rune(title)) > 160 || len([]rune(content)) > 10000 || containsControl(title) || containsControl(content) {
		return domain.Notice{}, errors.New("公告标题或内容过长")
	}
	if err := validateWebURL(linkURL, false); err != nil {
		return domain.Notice{}, errors.New("公告链接无效")
	}
	if level != "info" && level != "success" && level != "warning" && level != "error" {
		return domain.Notice{}, errors.New("公告级别无效")
	}
	media, mediaJSON, err := normalizeNoticeMedia(media)
	if err != nil {
		return domain.Notice{}, err
	}
	p, e := 0, 0
	if pinned {
		p = 1
	}
	if enabled {
		e = 1
	}
	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Notice{}, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE notices SET title = ?, content = ?, link_url = ?, media_json = ?, level = ?, pinned = ?, enabled = ?, updated_at = ? WHERE id = ?`, title, content, linkURL, mediaJSON, level, p, e, now, id)
	if err != nil {
		return domain.Notice{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return domain.Notice{}, err
	}
	if n == 0 {
		return domain.Notice{}, errors.New("公告不存在")
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'notice', ?, 'update', '', ?)`, actorID, id, now); err != nil {
		return domain.Notice{}, err
	}
	item, err := getNotice(tx, id)
	if err != nil {
		return domain.Notice{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Notice{}, err
	}
	return item, nil
}

func (s *Store) DeleteNotice(actorID, id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`DELETE FROM notices WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("公告不存在")
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'notice', ?, 'delete', '', ?)`, actorID, id, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SetPostPinned(actorID, id int64, pinned bool) (domain.Post, error) {
	val := 0
	if pinned {
		val = 1
	}
	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Post{}, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE posts SET pinned = ?, updated_at = ? WHERE id = ? AND status = 'published'`, val, now, id)
	if err != nil {
		return domain.Post{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return domain.Post{}, err
	}
	if n == 0 {
		return domain.Post{}, fmt.Errorf("帖子不存在")
	}
	action := "unpin"
	if pinned {
		action = "pin"
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'post', ?, ?, '', ?)`, actorID, id, action, now); err != nil {
		return domain.Post{}, err
	}
	item, err := getPost(tx, id)
	if err != nil {
		return domain.Post{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Post{}, err
	}
	return item, nil
}

func (s *Store) UserWalletSummary(userID int64, limit int) (domain.WalletSummary, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	available, err := s.WalletBalance(userID)
	if err != nil {
		return domain.WalletSummary{}, err
	}
	withdrawable, err := s.CreatorBalance(userID)
	if err != nil {
		return domain.WalletSummary{}, err
	}
	feePolicy, err := s.UserFeePolicy(userID)
	if err != nil {
		return domain.WalletSummary{}, err
	}
	rows, err := s.db.Query(`SELECT id, user_id, entry_type, amount_cents, reference_type, reference_id, note, created_at FROM wallet_ledgers WHERE user_id = ? ORDER BY created_at DESC, id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return domain.WalletSummary{}, err
	}
	defer rows.Close()
	entries := make([]domain.WalletEntry, 0)
	for rows.Next() {
		var item domain.WalletEntry
		if err := rows.Scan(&item.ID, &item.UserID, &item.EntryType, &item.AmountCents, &item.ReferenceType, &item.ReferenceID, &item.Note, &item.CreatedAt); err != nil {
			return domain.WalletSummary{}, err
		}
		entries = append(entries, item)
	}
	if err := rows.Err(); err != nil {
		return domain.WalletSummary{}, err
	}
	topUps, err := s.ListWalletTopUps(userID, 8)
	if err != nil {
		return domain.WalletSummary{}, err
	}
	return domain.WalletSummary{AvailableCents: available, WithdrawableCents: withdrawable, FeePolicy: feePolicy, Entries: entries, TopUps: topUps}, nil
}

func (s *Store) EnsureCommunitySeeds() error {
	now := time.Now().UTC()
	var adCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM ad_slots`).Scan(&adCount); err != nil {
		return err
	}
	if adCount == 0 {
		_, err := s.db.Exec(`INSERT INTO ad_slots (title, image_url, link_url, sort_order, enabled, created_at, updated_at) VALUES
			(?, '', '', 1, 1, ?, ?),
			(?, '', '', 2, 1, ?, ?)`,
			"欢迎加入罗布玩家社区", now, now,
			"投稿资源 · 组队交流 · 攻略分享", now, now)
		if err != nil {
			return err
		}
	}
	var noticeCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM notices`).Scan(&noticeCount); err != nil {
		return err
	}
	if noticeCount == 0 {
		_, err := s.db.Exec(`INSERT INTO notices (title, content, level, pinned, enabled, created_by, created_at, updated_at) VALUES (?, ?, 'info', 1, 1, NULL, ?, ?)`,
			"社区公告", "欢迎来到玩家社区。请友善交流，资源投稿需审核。本站为玩家社区，与 Roblox 官方无关。", now, now)
		if err != nil {
			return err
		}
	}
	var robloxNewsCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM roblox_news`).Scan(&robloxNewsCount); err != nil {
		return err
	}
	if robloxNewsCount == 0 {
		_, err := s.db.Exec(`INSERT INTO roblox_news (title, content, enabled, created_by, created_at, updated_at) VALUES (?, ?, 1, NULL, ?, ?)`,
			"欢迎来到新闻快报", "这里会发布 Roblox 平台动态、版本更新与官方活动消息。", now, now)
		if err != nil {
			return err
		}
	}
	return nil
}
