package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

const (
	maxCommentMediaCount = 4
	maxCommentMediaSize  = 50 << 20
	maxCommentImageSize  = 5 << 20
)

type CommentMediaInput struct {
	StoredName string
	MIMEType   string
	Width      int
	Height     int
	SizeBytes  int64
}

func validateCommentMedia(input CommentMediaInput) error {
	input.StoredName = strings.TrimSpace(input.StoredName)
	input.MIMEType = strings.ToLower(strings.TrimSpace(input.MIMEType))
	if input.StoredName == "" || len(input.StoredName) > 255 || filepath.Base(input.StoredName) != input.StoredName || strings.ContainsAny(input.StoredName, `/\`) {
		return errors.New("comment media filename is invalid")
	}
	isImage := input.MIMEType == "image/png" || input.MIMEType == "image/jpeg"
	isVideo := input.MIMEType == "video/mp4" || input.MIMEType == "video/webm"
	if !isImage && !isVideo {
		return errors.New("comment media type is invalid")
	}
	if isImage && (input.Width < 1 || input.Height < 1 || input.Width > 8192 || input.Height > 8192) {
		return errors.New("comment media dimensions or size are invalid")
	}
	if isVideo && (input.Width != 0 || input.Height != 0) {
		return errors.New("comment media dimensions or size are invalid")
	}
	maxSize := int64(maxCommentMediaSize)
	if isImage {
		maxSize = maxCommentImageSize
	}
	if input.SizeBytes < 1 || input.SizeBytes > maxSize {
		return errors.New("comment media dimensions or size are invalid")
	}
	return nil
}

func insertCommentMedia(queryer interface {
	Exec(query string, args ...any) (sql.Result, error)
}, commentID int64, media []CommentMediaInput, now time.Time) error {
	if len(media) > maxCommentMediaCount {
		return errors.New("too many comment images")
	}
	for index, item := range media {
		if err := validateCommentMedia(item); err != nil {
			return err
		}
		if _, err := queryer.Exec(`INSERT INTO comment_media (comment_id, stored_name, mime_type, width, height, size_bytes, sort_order, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, commentID, item.StoredName, item.MIMEType, item.Width, item.Height, item.SizeBytes, index, now); err != nil {
			return err
		}
	}
	return nil
}

func getCommentMedia(queryer sqlQueryer, commentID int64) ([]domain.PostMedia, error) {
	rows, err := queryer.Query(`SELECT id, stored_name, mime_type, width, height, size_bytes FROM comment_media WHERE comment_id = ? ORDER BY sort_order, id`, commentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.PostMedia, 0)
	for rows.Next() {
		var item domain.PostMedia
		var storedName string
		if err := rows.Scan(&item.ID, &storedName, &item.MIMEType, &item.Width, &item.Height, &item.SizeBytes); err != nil {
			return nil, err
		}
		item.URL = "/api/v1/media/comments/" + storedName
		result = append(result, item)
	}
	return result, rows.Err()
}

func attachCommentMedia(queryer sqlQueryer, comments []domain.Comment) error {
	if len(comments) == 0 {
		return nil
	}
	commentIndex := make(map[int64]int, len(comments))
	placeholders := make([]string, 0, len(comments))
	args := make([]any, 0, len(comments))
	for index := range comments {
		commentIndex[comments[index].ID] = index
		placeholders = append(placeholders, "?")
		args = append(args, comments[index].ID)
	}
	rows, err := queryer.Query(`SELECT id, comment_id, stored_name, mime_type, width, height, size_bytes FROM comment_media WHERE comment_id IN (`+strings.Join(placeholders, ",")+`) ORDER BY comment_id, sort_order, id`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item domain.PostMedia
		var commentID int64
		var storedName string
		if err := rows.Scan(&item.ID, &commentID, &storedName, &item.MIMEType, &item.Width, &item.Height, &item.SizeBytes); err != nil {
			return err
		}
		index, ok := commentIndex[commentID]
		if !ok {
			continue
		}
		item.URL = "/api/v1/media/comments/" + storedName
		comments[index].Media = append(comments[index].Media, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	return attachAvatarFramesToComments(queryer, comments)
}

func (s *Store) CommentMediaAccess(storedName string, viewerID int64, isAdmin bool) (domain.PostMedia, bool, bool, error) {
	var item domain.PostMedia
	var storedNameDB, commentStatus, postStatus, boardStatus, authorStatus string
	var commentID, ignoredPostID, authorID int64
	err := s.db.QueryRow(`SELECT cm.id, cm.comment_id, cm.stored_name, cm.mime_type, cm.width, cm.height, cm.size_bytes, c.post_id, c.author_id, c.status, p.status, b.status, u.status FROM comment_media cm JOIN comments c ON c.id = cm.comment_id JOIN posts p ON p.id = c.post_id JOIN boards b ON b.id = p.board_id JOIN users u ON u.id = c.author_id WHERE cm.stored_name = ?`, storedName).Scan(&item.ID, &commentID, &storedNameDB, &item.MIMEType, &item.Width, &item.Height, &item.SizeBytes, &ignoredPostID, &authorID, &commentStatus, &postStatus, &boardStatus, &authorStatus)
	if err != nil {
		return item, false, false, err
	}
	item.URL = "/api/v1/media/comments/" + storedNameDB
	publiclyVisible := commentStatus == "published" && postStatus == "published" && boardStatus == "active" && authorStatus == "active"
	allowed := isAdmin || (viewerID > 0 && viewerID == authorID) || publiclyVisible
	_ = commentID
	_ = ignoredPostID
	return item, allowed, publiclyVisible, nil
}

func (s *Store) ListDeletedCommentMediaNames(limit int) ([]string, error) {
	if limit < 1 || limit > 1000 {
		limit = 500
	}
	rows, err := s.db.Query(`SELECT cm.stored_name FROM comment_media cm JOIN comments c ON c.id = cm.comment_id WHERE c.status = 'deleted' ORDER BY cm.id LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var storedName string
		if err := rows.Scan(&storedName); err != nil {
			return nil, err
		}
		result = append(result, storedName)
	}
	return result, rows.Err()
}

func (s *Store) DeleteDeletedCommentMediaRecord(storedName string) error {
	_, err := s.db.Exec(`DELETE FROM comment_media WHERE stored_name = ? AND comment_id IN (SELECT id FROM comments WHERE status = 'deleted')`, storedName)
	return err
}
