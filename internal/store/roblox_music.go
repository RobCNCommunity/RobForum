package store

import (
	"database/sql"
	"time"

	"roblox-community/internal/domain"
)

func (s *Store) SaveRobloxMusicFavorite(userID int64, item domain.RobloxMusic) error {
	_, err := s.db.Exec(`INSERT INTO roblox_music_favorites (user_id, asset_id, name, description, creator_name, thumbnail_url, created_at) VALUES (?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE name=VALUES(name), description=VALUES(description), creator_name=VALUES(creator_name), thumbnail_url=VALUES(thumbnail_url)`, userID, item.AssetID, item.Name, item.Description, item.CreatorName, item.ThumbnailURL, time.Now().UTC())
	return err
}

func (s *Store) DeleteRobloxMusicFavorite(userID, assetID int64) error {
	result, err := s.db.Exec(`DELETE FROM roblox_music_favorites WHERE user_id = ? AND asset_id = ?`, userID, assetID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ListRobloxMusicFavorites(userID int64) ([]domain.RobloxMusic, error) {
	rows, err := s.db.Query(`SELECT asset_id, name, description, creator_name, thumbnail_url, created_at FROM roblox_music_favorites WHERE user_id = ? ORDER BY created_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.RobloxMusic, 0)
	for rows.Next() {
		var item domain.RobloxMusic
		if err := rows.Scan(&item.AssetID, &item.Name, &item.Description, &item.CreatorName, &item.ThumbnailURL, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Favorited = true
		items = append(items, item)
	}
	return items, rows.Err()
}
