package store

import (
	"database/sql"
	"errors"
	"strings"
	"unicode"

	"roblox-community/internal/domain"
)

const maxPostTagCount = 5

var errPostTagsInvalid = errors.New("post tags are invalid")

func normalizePostTags(tags []string) ([]string, error) {
	if len(tags) > maxPostTagCount {
		return nil, errPostTagsInvalid
	}
	seen := make(map[string]struct{}, len(tags))
	result := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := strings.ToLower(strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(raw), "#")))
		runes := []rune(tag)
		if len(runes) == 0 || len(runes) > 24 {
			return nil, errPostTagsInvalid
		}
		for _, value := range runes {
			if unicode.IsControl(value) || unicode.IsSpace(value) || value == '#' {
				return nil, errPostTagsInvalid
			}
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	return result, nil
}

func insertPostTags(queryer interface {
	Exec(query string, args ...any) (sql.Result, error)
}, postID int64, tags []string) error {
	normalized, err := normalizePostTags(tags)
	if err != nil {
		return err
	}
	for index, tag := range normalized {
		if _, err := queryer.Exec(`INSERT INTO post_tags (post_id, tag, sort_order) VALUES (?, ?, ?)`, postID, tag, index); err != nil {
			return err
		}
	}
	return nil
}

func getPostTags(queryer sqlQueryer, postID int64) ([]string, error) {
	rows, err := queryer.Query(`SELECT tag FROM post_tags WHERE post_id = ? ORDER BY sort_order, tag`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		result = append(result, tag)
	}
	return result, rows.Err()
}

func attachPostTags(queryer sqlQueryer, posts []domain.Post) error {
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
	rows, err := queryer.Query(`SELECT post_id, tag FROM post_tags WHERE post_id IN (`+strings.Join(placeholders, ",")+`) ORDER BY post_id, sort_order, tag`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var postID int64
		var tag string
		if err := rows.Scan(&postID, &tag); err != nil {
			return err
		}
		if index, ok := postIndex[postID]; ok {
			posts[index].Tags = append(posts[index].Tags, tag)
		}
	}
	return rows.Err()
}
