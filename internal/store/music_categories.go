package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

const musicCategoryColumns = `id, name, sort_order, enabled, created_at, updated_at`

func scanMusicCategory(scanner rowScanner) (domain.MusicCategory, error) {
	var item domain.MusicCategory
	var enabled int
	err := scanner.Scan(&item.ID, &item.Name, &item.SortOrder, &enabled, &item.CreatedAt, &item.UpdatedAt)
	item.Enabled = enabled != 0
	return item, err
}

func normalizeMusicCategory(name string, sortOrder int) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 40 || containsControl(name) {
		return "", errors.New("分区名称不能为空且不能超过 40 个字符")
	}
	if sortOrder < -9999 || sortOrder > 9999 {
		return "", errors.New("分区排序值必须在 -9999 到 9999 之间")
	}
	return name, nil
}

func (s *Store) ListMusicCategories(publicOnly bool) ([]domain.MusicCategory, error) {
	query := `SELECT ` + musicCategoryColumns + ` FROM music_categories`
	if publicOnly {
		query += ` WHERE enabled = 1`
	}
	query += ` ORDER BY sort_order ASC, id ASC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.MusicCategory, 0)
	for rows.Next() {
		item, err := scanMusicCategory(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) musicCategoryByID(queryer rowQueryer, id int64) (domain.MusicCategory, error) {
	return scanMusicCategory(queryer.QueryRow(`SELECT `+musicCategoryColumns+` FROM music_categories WHERE id = ?`, id))
}

func (s *Store) activeMusicCategory(id int64) (domain.MusicCategory, error) {
	item, err := s.musicCategoryByID(s.db, id)
	if err != nil {
		return domain.MusicCategory{}, err
	}
	if !item.Enabled {
		return domain.MusicCategory{}, errors.New("所选音乐分区已下架")
	}
	return item, nil
}

func (s *Store) CreateMusicCategory(actorID int64, name string, sortOrder int, enabled bool) (domain.MusicCategory, error) {
	name, err := normalizeMusicCategory(name, sortOrder)
	if err != nil {
		return domain.MusicCategory{}, err
	}
	now := time.Now().UTC()
	value := 0
	if enabled {
		value = 1
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.MusicCategory{}, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`INSERT INTO music_categories (name, sort_order, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, name, sortOrder, value, now, now)
	if err != nil {
		return domain.MusicCategory{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.MusicCategory{}, err
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'music_category', ?, 'create', ?, ?)`, actorID, id, name, now); err != nil {
		return domain.MusicCategory{}, err
	}
	item, err := s.musicCategoryByID(tx, id)
	if err != nil {
		return domain.MusicCategory{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.MusicCategory{}, err
	}
	return item, nil
}

func (s *Store) UpdateMusicCategory(actorID, id int64, name string, sortOrder int, enabled bool) (domain.MusicCategory, error) {
	name, err := normalizeMusicCategory(name, sortOrder)
	if err != nil {
		return domain.MusicCategory{}, err
	}
	now := time.Now().UTC()
	value := 0
	if enabled {
		value = 1
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.MusicCategory{}, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE music_categories SET name = ?, sort_order = ?, enabled = ?, updated_at = ? WHERE id = ?`, name, sortOrder, value, now, id)
	if err != nil {
		return domain.MusicCategory{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return domain.MusicCategory{}, err
	}
	if count == 0 {
		return domain.MusicCategory{}, sql.ErrNoRows
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'music_category', ?, 'update', ?, ?)`, actorID, id, name, now); err != nil {
		return domain.MusicCategory{}, err
	}
	item, err := s.musicCategoryByID(tx, id)
	if err != nil {
		return domain.MusicCategory{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.MusicCategory{}, err
	}
	return item, nil
}

func (s *Store) DeleteMusicCategory(actorID, id int64) error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM roblox_music_submissions WHERE category_id = ?`, id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return errors.New("该分区仍有关联音乐，请先下架分区或调整音乐分区")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`DELETE FROM music_categories WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'music_category', ?, 'delete', '', ?)`, actorID, id, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}
