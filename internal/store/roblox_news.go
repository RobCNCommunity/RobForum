package store

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

const maxRobloxNewsMediaCount = 4

var robloxNewsMediaURLPattern = regexp.MustCompile(`^/api/v1/media/roblox-news/[0-9a-f]{32,40}\.(png|jpe?g)$`)

func (s *Store) ListPublicRobloxNews(limit int) ([]domain.RobloxNews, error) {
	if limit < 1 || limit > 50 {
		limit = 20
	}
	rows, err := s.db.Query(`SELECT id, title, content, link_url, media_json, enabled, COALESCE(created_by, 0), created_at, updated_at FROM roblox_news WHERE enabled = 1 ORDER BY updated_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.RobloxNews, 0)
	for rows.Next() {
		item, err := scanRobloxNews(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) ListAdminRobloxNews() ([]domain.RobloxNews, error) {
	rows, err := s.db.Query(`SELECT id, title, content, link_url, media_json, enabled, COALESCE(created_by, 0), created_at, updated_at FROM roblox_news ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.RobloxNews, 0)
	for rows.Next() {
		item, err := scanRobloxNews(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func scanRobloxNews(row adScanner) (domain.RobloxNews, error) {
	var item domain.RobloxNews
	var enabled int
	var mediaJSON string
	err := row.Scan(&item.ID, &item.Title, &item.Content, &item.LinkURL, &mediaJSON, &enabled, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt)
	if err == nil && strings.TrimSpace(mediaJSON) != "" {
		if decodeErr := json.Unmarshal([]byte(mediaJSON), &item.Media); decodeErr != nil {
			return domain.RobloxNews{}, decodeErr
		}
	}
	item.Enabled = enabled != 0
	return item, err
}

func normalizeRobloxNewsMedia(media []domain.RobloxNewsMedia) ([]domain.RobloxNewsMedia, string, error) {
	if len(media) > maxRobloxNewsMediaCount {
		return nil, "", errors.New("新闻最多上传 4 张图片")
	}
	seen := make(map[string]struct{}, len(media))
	for index := range media {
		item := &media[index]
		item.URL = strings.TrimSpace(item.URL)
		item.MIMEType = strings.ToLower(strings.TrimSpace(item.MIMEType))
		if !robloxNewsMediaURLPattern.MatchString(item.URL) || item.MIMEType != "image/png" && item.MIMEType != "image/jpeg" || item.Width < 16 || item.Height < 16 || item.SizeBytes < 1 {
			return nil, "", errors.New("新闻图片链接无效")
		}
		if _, ok := seen[item.URL]; ok {
			return nil, "", errors.New("新闻图片不能重复")
		}
		seen[item.URL] = struct{}{}
		item.ID = 0
	}
	if media == nil {
		media = []domain.RobloxNewsMedia{}
	}
	encoded, err := json.Marshal(media)
	if err != nil {
		return nil, "", err
	}
	return media, string(encoded), nil
}

func (s *Store) CreateRobloxNews(userID int64, title, content, linkURL string, enabled bool, media []domain.RobloxNewsMedia) (domain.RobloxNews, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	linkURL = strings.TrimSpace(linkURL)
	if title == "" || content == "" {
		return domain.RobloxNews{}, errors.New("新闻标题和内容不能为空")
	}
	if len([]rune(title)) > 160 || len([]rune(content)) > 10000 || containsControl(title) || containsControl(content) {
		return domain.RobloxNews{}, errors.New("新闻标题或内容过长")
	}
	if err := validateWebURL(linkURL, false); err != nil {
		return domain.RobloxNews{}, errors.New("新闻链接无效")
	}
	media, mediaJSON, err := normalizeRobloxNewsMedia(media)
	if err != nil {
		return domain.RobloxNews{}, err
	}
	now := time.Now().UTC()
	e := 0
	if enabled {
		e = 1
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.RobloxNews{}, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO roblox_news (title, content, link_url, media_json, enabled, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, title, content, linkURL, mediaJSON, e, userID, now, now)
	if err != nil {
		return domain.RobloxNews{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.RobloxNews{}, err
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'roblox_news', ?, 'create', '', ?)`, userID, id, now); err != nil {
		return domain.RobloxNews{}, err
	}
	item, err := getRobloxNews(tx, id)
	if err != nil {
		return domain.RobloxNews{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.RobloxNews{}, err
	}
	return item, nil
}

func (s *Store) GetRobloxNews(id int64) (domain.RobloxNews, error) {
	return getRobloxNews(s.db, id)
}

func getRobloxNews(queryer rowQueryer, id int64) (domain.RobloxNews, error) {
	row := queryer.QueryRow(`SELECT id, title, content, link_url, media_json, enabled, COALESCE(created_by, 0), created_at, updated_at FROM roblox_news WHERE id = ?`, id)
	return scanRobloxNews(row)
}

func (s *Store) UpdateRobloxNews(actorID, id int64, title, content, linkURL string, enabled bool, media []domain.RobloxNewsMedia) (domain.RobloxNews, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	linkURL = strings.TrimSpace(linkURL)
	if title == "" || content == "" {
		return domain.RobloxNews{}, errors.New("新闻标题和内容不能为空")
	}
	if len([]rune(title)) > 160 || len([]rune(content)) > 10000 || containsControl(title) || containsControl(content) {
		return domain.RobloxNews{}, errors.New("新闻标题或内容过长")
	}
	if err := validateWebURL(linkURL, false); err != nil {
		return domain.RobloxNews{}, errors.New("新闻链接无效")
	}
	media, mediaJSON, err := normalizeRobloxNewsMedia(media)
	if err != nil {
		return domain.RobloxNews{}, err
	}
	e := 0
	if enabled {
		e = 1
	}
	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return domain.RobloxNews{}, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE roblox_news SET title = ?, content = ?, link_url = ?, media_json = ?, enabled = ?, updated_at = ? WHERE id = ?`, title, content, linkURL, mediaJSON, e, now, id)
	if err != nil {
		return domain.RobloxNews{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return domain.RobloxNews{}, err
	}
	if n == 0 {
		return domain.RobloxNews{}, errors.New("新闻不存在")
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'roblox_news', ?, 'update', '', ?)`, actorID, id, now); err != nil {
		return domain.RobloxNews{}, err
	}
	item, err := getRobloxNews(tx, id)
	if err != nil {
		return domain.RobloxNews{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.RobloxNews{}, err
	}
	return item, nil
}

func (s *Store) DeleteRobloxNews(actorID, id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`DELETE FROM roblox_news WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("新闻不存在")
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'roblox_news', ?, 'delete', '', ?)`, actorID, id, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}
