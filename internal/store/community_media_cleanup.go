package store

import (
	"errors"
)

type CommunityMediaKind string

const (
	CommunityMediaPost    CommunityMediaKind = "post"
	CommunityMediaComment CommunityMediaKind = "comment"
)

type DeletedCommunityMedia struct {
	Kind       CommunityMediaKind
	StoredName string
}

func (s *Store) ListDeletedCommunityMedia(limit int) ([]DeletedCommunityMedia, error) {
	if limit < 1 || limit > 1000 {
		limit = 500
	}
	rows, err := s.db.Query(`SELECT media_kind, stored_name FROM (
		SELECT 'post' AS media_kind, pm.id AS media_id, pm.stored_name
		FROM post_media pm JOIN posts p ON p.id = pm.post_id
		WHERE p.status = 'deleted'
		UNION ALL
		SELECT 'comment' AS media_kind, cm.id AS media_id, cm.stored_name
		FROM comment_media cm
		JOIN comments c ON c.id = cm.comment_id
		JOIN posts p ON p.id = c.post_id
		WHERE c.status = 'deleted' OR p.status = 'deleted'
	) deleted_media ORDER BY media_kind, media_id LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]DeletedCommunityMedia, 0)
	for rows.Next() {
		var item DeletedCommunityMedia
		if err := rows.Scan(&item.Kind, &item.StoredName); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) DeleteDeletedCommunityMediaRecord(item DeletedCommunityMedia) error {
	switch item.Kind {
	case CommunityMediaPost:
		_, err := s.db.Exec(`DELETE FROM post_media WHERE stored_name = ? AND post_id IN (SELECT id FROM posts WHERE status = 'deleted')`, item.StoredName)
		return err
	case CommunityMediaComment:
		_, err := s.db.Exec(`DELETE FROM comment_media WHERE stored_name = ? AND comment_id IN (
			SELECT c.id FROM comments c JOIN posts p ON p.id = c.post_id WHERE c.status = 'deleted' OR p.status = 'deleted'
		)`, item.StoredName)
		return err
	default:
		return errors.New("community media kind is invalid")
	}
}
