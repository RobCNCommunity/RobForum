package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

type PostMediaInput struct {
	StoredName string
	MIMEType   string
	Width      int
	Height     int
	SizeBytes  int64
}

const maxPostMediaSize = 50 << 20

const maxPostImageSize = 5 << 20

func validatePostMedia(input PostMediaInput) error {
	input.StoredName = strings.TrimSpace(input.StoredName)
	input.MIMEType = strings.ToLower(strings.TrimSpace(input.MIMEType))
	if input.StoredName == "" || len(input.StoredName) > 255 || filepath.Base(input.StoredName) != input.StoredName || strings.ContainsAny(input.StoredName, `/\`) {
		return errors.New("post media filename is invalid")
	}
	isImage := input.MIMEType == "image/png" || input.MIMEType == "image/jpeg"
	isVideo := input.MIMEType == "video/mp4" || input.MIMEType == "video/webm"
	if !isImage && !isVideo {
		return errors.New("post media type is invalid")
	}
	if isImage && (input.Width < 1 || input.Height < 1 || input.Width > 8192 || input.Height > 8192) {
		return errors.New("post media dimensions or size are invalid")
	}
	if isVideo && (input.Width != 0 || input.Height != 0) {
		return errors.New("post media dimensions or size are invalid")
	}
	maxSize := int64(maxPostMediaSize)
	if isImage {
		maxSize = maxPostImageSize
	}
	if input.SizeBytes < 1 || input.SizeBytes > maxSize {
		return errors.New("post media dimensions or size are invalid")
	}
	return nil
}

func insertPostMedia(queryer interface {
	Exec(query string, args ...any) (sql.Result, error)
}, postID int64, media []PostMediaInput, now time.Time) error {
	for index, item := range media {
		if err := validatePostMedia(item); err != nil {
			return err
		}
		if _, err := queryer.Exec(`INSERT INTO post_media (post_id, stored_name, mime_type, width, height, size_bytes, sort_order, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, postID, item.StoredName, item.MIMEType, item.Width, item.Height, item.SizeBytes, index, now); err != nil {
			return err
		}
	}
	return nil
}

func getPostMedia(queryer sqlQueryer, postID int64) ([]domain.PostMedia, error) {
	rows, err := queryer.Query(`SELECT id, stored_name, mime_type, width, height, size_bytes FROM post_media WHERE post_id = ? ORDER BY sort_order, id`, postID)
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
		item.URL = "/api/v1/media/posts/" + storedName
		result = append(result, item)
	}
	return result, rows.Err()
}

func attachPostMedia(queryer sqlQueryer, posts []domain.Post) error {
	if len(posts) == 0 {
		return nil
	}
	postIndex := make(map[int64]int, len(posts))
	placeholders := make([]string, 0, len(posts))
	args := make([]any, 0, len(posts))
	for index := range posts {
		postIndex[posts[index].ID] = index
		placeholders = append(placeholders, "?")
		args = append(args, posts[index].ID)
	}
	rows, err := queryer.Query(`SELECT id, post_id, stored_name, mime_type, width, height, size_bytes FROM post_media WHERE post_id IN (`+strings.Join(placeholders, ",")+`) ORDER BY post_id, sort_order, id`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item domain.PostMedia
		var postID int64
		var storedName string
		if err := rows.Scan(&item.ID, &postID, &storedName, &item.MIMEType, &item.Width, &item.Height, &item.SizeBytes); err != nil {
			return err
		}
		index, ok := postIndex[postID]
		if !ok {
			continue
		}
		item.URL = "/api/v1/media/posts/" + storedName
		posts[index].Media = append(posts[index].Media, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := attachPostTags(queryer, posts); err != nil {
		return err
	}
	return attachAvatarFramesToPosts(queryer, posts)
}
