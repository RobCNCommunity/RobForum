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
	maxResourceMediaCount = 5
	maxResourceMediaSize  = 5 << 20
)

type ResourceMediaInput struct {
	StoredName string
	MIMEType   string
	Width      int
	Height     int
	SizeBytes  int64
}

func validateResourceMedia(input ResourceMediaInput) error {
	input.StoredName = strings.TrimSpace(input.StoredName)
	input.MIMEType = strings.ToLower(strings.TrimSpace(input.MIMEType))
	if input.StoredName == "" || len(input.StoredName) > 255 || filepath.Base(input.StoredName) != input.StoredName || strings.ContainsAny(input.StoredName, `/\`) {
		return errors.New("resource media filename is invalid")
	}
	if input.MIMEType != "image/png" && input.MIMEType != "image/jpeg" {
		return errors.New("resource media type is invalid")
	}
	if input.Width < 1 || input.Height < 1 || input.Width > 8192 || input.Height > 8192 || input.SizeBytes < 1 || input.SizeBytes > maxResourceMediaSize {
		return errors.New("resource media dimensions or size are invalid")
	}
	return nil
}

func insertResourceMedia(queryer interface {
	Exec(query string, args ...any) (sql.Result, error)
}, resourceID int64, media []ResourceMediaInput, now time.Time) error {
	if len(media) > maxResourceMediaCount {
		return errors.New("too many resource preview images")
	}
	for index, item := range media {
		if err := validateResourceMedia(item); err != nil {
			return err
		}
		if _, err := queryer.Exec(`INSERT INTO resource_media (resource_id, stored_name, mime_type, width, height, size_bytes, sort_order, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, resourceID, item.StoredName, item.MIMEType, item.Width, item.Height, item.SizeBytes, index, now); err != nil {
			return err
		}
	}
	return nil
}

func getResourceMedia(queryer sqlQueryer, resourceID int64) ([]domain.ResourceMedia, error) {
	rows, err := queryer.Query(`SELECT id, stored_name, mime_type, width, height, size_bytes FROM resource_media WHERE resource_id = ? ORDER BY sort_order, id`, resourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.ResourceMedia, 0)
	for rows.Next() {
		var item domain.ResourceMedia
		var storedName string
		if err := rows.Scan(&item.ID, &storedName, &item.MIMEType, &item.Width, &item.Height, &item.SizeBytes); err != nil {
			return nil, err
		}
		item.URL = "/api/v1/media/resources/" + storedName
		result = append(result, item)
	}
	return result, rows.Err()
}

func attachResourceMedia(queryer sqlQueryer, resources []domain.Resource) error {
	if len(resources) == 0 {
		return nil
	}
	resourceIndex := make(map[int64]int, len(resources))
	placeholders := make([]string, 0, len(resources))
	args := make([]any, 0, len(resources))
	for index := range resources {
		resourceIndex[resources[index].ID] = index
		placeholders = append(placeholders, "?")
		args = append(args, resources[index].ID)
	}
	rows, err := queryer.Query(`SELECT id, resource_id, stored_name, mime_type, width, height, size_bytes FROM resource_media WHERE resource_id IN (`+strings.Join(placeholders, ",")+`) ORDER BY resource_id, sort_order, id`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item domain.ResourceMedia
		var resourceID int64
		var storedName string
		if err := rows.Scan(&item.ID, &resourceID, &storedName, &item.MIMEType, &item.Width, &item.Height, &item.SizeBytes); err != nil {
			return err
		}
		index, ok := resourceIndex[resourceID]
		if !ok {
			continue
		}
		item.URL = "/api/v1/media/resources/" + storedName
		resources[index].Media = append(resources[index].Media, item)
	}
	return rows.Err()
}

// ResourceMediaAccess returns the media metadata and whether it may be served.
// Draft/rejected media is visible only to the creator or an administrator.
func (s *Store) ResourceMediaAccess(storedName string, viewerID int64, isAdmin bool) (domain.ResourceMedia, bool, bool, error) {
	var item domain.ResourceMedia
	var resourceStatus, userStatus string
	var creatorID int64
	var stored string
	err := s.db.QueryRow(`SELECT rm.id, rm.resource_id, rm.stored_name, rm.mime_type, rm.width, rm.height, rm.size_bytes, r.status, r.creator_id, u.status FROM resource_media rm JOIN resources r ON r.id = rm.resource_id JOIN users u ON u.id = r.creator_id WHERE rm.stored_name = ?`, storedName).Scan(&item.ID, &item.ResourceID, &stored, &item.MIMEType, &item.Width, &item.Height, &item.SizeBytes, &resourceStatus, &creatorID, &userStatus)
	if err != nil {
		return item, false, false, err
	}
	item.URL = "/api/v1/media/resources/" + stored
	publiclyVisible := resourceStatus == "approved" && userStatus == "active"
	allowed := isAdmin || (viewerID > 0 && viewerID == creatorID) || publiclyVisible
	return item, allowed, publiclyVisible, nil
}
