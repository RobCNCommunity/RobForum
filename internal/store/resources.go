package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

const maxResourcePriceCents int64 = 1_000_000

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type ResourceFileInput struct {
	OriginalName string
	StoredName   string
	MIMEType     string
	SizeBytes    int64
	SHA256       string
}

func (s *Store) CreateResource(creatorID int64, title, description, game, version, resourceType string, priceCents int64, file ResourceFileInput) (domain.Resource, error) {
	return s.CreateResourceWithMedia(creatorID, title, description, game, version, resourceType, priceCents, file, nil)
}

func (s *Store) CreateResourceWithMedia(creatorID int64, title, description, game, version, resourceType string, priceCents int64, file ResourceFileInput, media []ResourceMediaInput) (domain.Resource, error) {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	game = strings.TrimSpace(game)
	version = strings.TrimSpace(version)
	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	if title == "" || len([]rune(title)) > 180 || description == "" || len([]rune(description)) > 20000 || game == "" || len([]rune(game)) > 120 || len([]rune(version)) > 80 || priceCents < 0 || priceCents > maxResourcePriceCents || file.StoredName == "" || containsControl(title+description+game+version) {
		return domain.Resource{}, errors.New("resource metadata is incomplete")
	}
	if resourceType != "map" && resourceType != "script" && resourceType != "asset" && resourceType != "guide" && resourceType != "other" {
		return domain.Resource{}, errors.New("resource type is invalid")
	}
	if len(media) > maxResourceMediaCount {
		return domain.Resource{}, errors.New("too many resource preview images")
	}
	file.OriginalName = strings.TrimSpace(file.OriginalName)
	file.StoredName = strings.TrimSpace(file.StoredName)
	file.MIMEType = strings.TrimSpace(file.MIMEType)
	file.SHA256 = strings.ToLower(strings.TrimSpace(file.SHA256))
	if len([]rune(file.OriginalName)) < 1 || len([]rune(file.OriginalName)) > 255 || file.StoredName == "" || len(file.StoredName) > 255 || filepath.Base(file.StoredName) != file.StoredName || strings.ContainsAny(file.StoredName, `/\\`) || len(file.MIMEType) > 160 || file.SizeBytes < 1 || !sha256Pattern.MatchString(file.SHA256) || containsControl(file.OriginalName+file.StoredName+file.MIMEType) {
		return domain.Resource{}, errors.New("resource file metadata is invalid")
	}
	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Resource{}, err
	}
	defer tx.Rollback()
	var creatorStatus string
	if err := tx.QueryRow(`SELECT status FROM users WHERE id = ? FOR UPDATE`, creatorID).Scan(&creatorStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Resource{}, errors.New("resource creator is not available")
		}
		return domain.Resource{}, err
	}
	if creatorStatus != "active" {
		return domain.Resource{}, errors.New("resource creator is not available")
	}
	result, err := tx.Exec(`INSERT INTO resources (creator_id, title, description, game, version, resource_type, price_cents, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?)`, creatorID, title, description, game, version, resourceType, priceCents, now, now)
	if err != nil {
		return domain.Resource{}, err
	}
	resourceID, err := result.LastInsertId()
	if err != nil {
		return domain.Resource{}, err
	}
	if _, err := tx.Exec(`INSERT INTO resource_files (resource_id, original_name, stored_name, mime_type, size_bytes, sha256, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, resourceID, file.OriginalName, file.StoredName, file.MIMEType, file.SizeBytes, file.SHA256, now); err != nil {
		return domain.Resource{}, err
	}
	if err := insertResourceMedia(tx, resourceID, media, now); err != nil {
		return domain.Resource{}, err
	}
	item, err := getResource(tx, resourceID, false)
	if err != nil {
		return domain.Resource{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Resource{}, err
	}
	return item, nil
}

func (s *Store) GetResource(id int64) (domain.Resource, error) {
	return getResource(s.db, id, false)
}

func (s *Store) GetPublicResource(id int64) (domain.Resource, error) {
	return getResource(s.db, id, true)
}

func getResource(queryer sqlQueryer, id int64, public bool) (domain.Resource, error) {
	where := `WHERE r.id = ?`
	if public {
		where += ` AND r.status = 'approved' AND u.status = 'active'`
	}
	row := queryer.QueryRow(`SELECT r.id, r.creator_id, u.display_name, u.blue_verified, u.verification_label, COALESCE(u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP(), 0), CASE WHEN u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP() THEN u.membership_tier_id ELSE 0 END, r.title, r.description, r.game, r.version, r.resource_type, r.price_cents, r.status, r.review_reason, r.download_count, r.sales_count, r.created_at, r.updated_at, f.id, f.original_name, f.mime_type, f.size_bytes, f.sha256, f.created_at FROM resources r JOIN users u ON u.id = r.creator_id LEFT JOIN resource_files f ON f.resource_id = r.id `+where, id)
	item, err := scanResource(row)
	if err != nil {
		return item, err
	}
	item.Media, err = getResourceMedia(queryer, id)
	return item, err
}

func (s *Store) ListResources(status string, creatorID int64, limit int) ([]domain.Resource, error) {
	return s.listResources(status, creatorID, "", limit)
}

func (s *Store) SearchPublicResources(query string, limit int) ([]domain.Resource, error) {
	return s.listResources("approved", 0, query, limit)
}

func (s *Store) listResources(status string, creatorID int64, query string, limit int) ([]domain.Resource, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "" && status != "pending" && status != "approved" && status != "rejected" && status != "takedown" {
		return nil, errors.New("resource status is invalid")
	}
	where := "1 = 1"
	args := make([]any, 0, 3)
	if status != "" {
		where += " AND r.status = ?"
		args = append(args, status)
	}
	if creatorID > 0 {
		where += " AND r.creator_id = ?"
		args = append(args, creatorID)
	}
	if status == "approved" && creatorID == 0 {
		where += " AND u.status = 'active'"
	}
	query = strings.TrimSpace(query)
	if len([]rune(query)) > 100 {
		return nil, errors.New("search query is too long")
	}
	if query != "" {
		pattern := "%" + escapeLike(query) + "%"
		where += ` AND (r.title LIKE ? ESCAPE '\\' OR r.description LIKE ? ESCAPE '\\' OR r.game LIKE ? ESCAPE '\\' OR r.version LIKE ? ESCAPE '\\' OR r.resource_type LIKE ? ESCAPE '\\' OR u.display_name LIKE ? ESCAPE '\\')`
		args = append(args, pattern, pattern, pattern, pattern, pattern, pattern)
	}
	args = append(args, limit)
	rows, err := s.db.Query(`SELECT r.id, r.creator_id, u.display_name, u.blue_verified, u.verification_label, COALESCE(u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP(), 0), CASE WHEN u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP() THEN u.membership_tier_id ELSE 0 END, r.title, r.description, r.game, r.version, r.resource_type, r.price_cents, r.status, r.review_reason, r.download_count, r.sales_count, r.created_at, r.updated_at, f.id, f.original_name, f.mime_type, f.size_bytes, f.sha256, f.created_at FROM resources r JOIN users u ON u.id = r.creator_id LEFT JOIN resource_files f ON f.resource_id = r.id WHERE `+where+` ORDER BY r.updated_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.Resource, 0)
	for rows.Next() {
		item, err := scanResource(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := attachResourceMedia(s.db, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) ReviewResource(actorID, resourceID int64, status, reason string) (domain.Resource, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "approved" && status != "rejected" && status != "takedown" {
		return domain.Resource{}, errors.New("unsupported moderation status")
	}
	reason = strings.TrimSpace(reason)
	if status != "approved" && reason == "" {
		return domain.Resource{}, errors.New("a moderation reason is required")
	}
	if len([]rune(reason)) > 500 {
		return domain.Resource{}, errors.New("moderation reason is too long")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Resource{}, err
	}
	defer tx.Rollback()
	var currentStatus string
	if err := tx.QueryRow(`SELECT status FROM resources WHERE id = ? FOR UPDATE`, resourceID).Scan(&currentStatus); err != nil {
		return domain.Resource{}, err
	}
	if status == "approved" {
		var fileID int64
		if err := tx.QueryRow(`SELECT id FROM resource_files WHERE resource_id = ? FOR UPDATE`, resourceID).Scan(&fileID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return domain.Resource{}, errors.New("resource file is missing and cannot be approved")
			}
			return domain.Resource{}, err
		}
	}
	now := time.Now().UTC()
	result, err := tx.Exec(`UPDATE resources SET status = ?, review_reason = ?, updated_at = ? WHERE id = ?`, status, reason, now, resourceID)
	if err != nil {
		return domain.Resource{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Resource{}, err
	}
	if affected == 0 {
		return domain.Resource{}, sql.ErrNoRows
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'resource', ?, ?, ?, ?)`, actorID, resourceID, status, reason, now); err != nil {
		return domain.Resource{}, err
	}
	item, err := getResource(tx, resourceID, false)
	if err != nil {
		return domain.Resource{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Resource{}, err
	}
	return item, nil
}

func (s *Store) ResourceDownload(id int64) (domain.Resource, string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Resource{}, "", err
	}
	defer tx.Rollback()
	var lockedID int64
	if err := tx.QueryRow(`SELECT r.id FROM resources r JOIN users u ON u.id = r.creator_id WHERE r.id = ? AND r.status = 'approved' AND u.status = 'active' FOR UPDATE`, id).Scan(&lockedID); err != nil {
		return domain.Resource{}, "", err
	}
	item, err := getResource(tx, id, true)
	if err != nil {
		return domain.Resource{}, "", err
	}
	var storedName string
	if err := tx.QueryRow(`SELECT stored_name FROM resource_files WHERE resource_id = ?`, id).Scan(&storedName); err != nil {
		return domain.Resource{}, "", err
	}
	if err := tx.Commit(); err != nil {
		return domain.Resource{}, "", err
	}
	return item, storedName, nil
}

// ResourceDownloadForAdmin returns a resource file regardless of moderation
// status. It is used only behind the admin middleware so pending files can be
// inspected without making them public or incrementing download counters.
func (s *Store) ResourceDownloadForAdmin(id int64) (domain.Resource, string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Resource{}, "", err
	}
	defer tx.Rollback()
	var lockedID int64
	if err := tx.QueryRow(`SELECT id FROM resources WHERE id = ? FOR UPDATE`, id).Scan(&lockedID); err != nil {
		return domain.Resource{}, "", err
	}
	item, err := getResource(tx, id, false)
	if err != nil {
		return domain.Resource{}, "", err
	}
	var storedName string
	if err := tx.QueryRow(`SELECT stored_name FROM resource_files WHERE resource_id = ?`, id).Scan(&storedName); err != nil {
		return domain.Resource{}, "", err
	}
	if err := tx.Commit(); err != nil {
		return domain.Resource{}, "", err
	}
	return item, storedName, nil
}

func (s *Store) IncrementResourceDownload(id int64) error {
	result, err := s.db.Exec(`UPDATE resources r JOIN users u ON u.id = r.creator_id SET r.download_count = r.download_count + 1 WHERE r.id = ? AND r.status = 'approved' AND u.status = 'active'`, id)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanResource(scanner rowScanner) (domain.Resource, error) {
	var item domain.Resource
	var creatorVerified, creatorMember int
	var fileID sql.NullInt64
	var originalName, mimeType, sha256 sql.NullString
	var sizeBytes sql.NullInt64
	var fileCreated sql.NullTime
	err := scanner.Scan(&item.ID, &item.CreatorID, &item.CreatorName, &creatorVerified, &item.CreatorVerificationLabel, &creatorMember, &item.CreatorMembershipTierID, &item.Title, &item.Description, &item.Game, &item.Version, &item.ResourceType, &item.PriceCents, &item.Status, &item.ReviewReason, &item.DownloadCount, &item.SalesCount, &item.CreatedAt, &item.UpdatedAt, &fileID, &originalName, &mimeType, &sizeBytes, &sha256, &fileCreated)
	if err != nil {
		return item, err
	}
	if fileID.Valid {
		item.File = &domain.ResourceFile{ID: fileID.Int64, ResourceID: item.ID, OriginalName: originalName.String, MIMEType: mimeType.String, SizeBytes: sizeBytes.Int64, SHA256: sha256.String, CreatedAt: fileCreated.Time}
	}
	item.CreatorVerified = creatorVerified != 0
	item.CreatorMember = creatorMember != 0
	return item, nil
}
