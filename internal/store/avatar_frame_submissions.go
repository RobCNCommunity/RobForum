package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

var ErrAvatarFrameUploadForbidden = errors.New("当前账号暂未开放头像框上传")

const avatarFrameSubmissionColumns = `s.id, s.user_id, u.display_name, u.avatar_url, s.name, s.description, s.image_url, s.status, s.review_note, s.reviewed_by, s.reviewed_at, s.approved_frame_id, s.created_at, s.updated_at`

func (s *Store) GetAvatarFrameUploadSettings(userID int64) (domain.AvatarFrameUploadSettings, error) {
	now := time.Now().UTC()
	if _, err := s.db.Exec(`INSERT IGNORE INTO avatar_frame_upload_settings (id, allow_regular_upload, allow_member_upload, updated_at) VALUES (1, 0, 0, ?)`, now); err != nil {
		return domain.AvatarFrameUploadSettings{}, err
	}
	var settings domain.AvatarFrameUploadSettings
	var regular, member int
	if err := s.db.QueryRow(`SELECT allow_regular_upload, allow_member_upload, updated_at FROM avatar_frame_upload_settings WHERE id = 1`).Scan(&regular, &member, &settings.UpdatedAt); err != nil {
		return settings, err
	}
	settings.AllowRegularUpload = regular != 0
	settings.AllowMemberUpload = member != 0
	if userID > 0 {
		context, err := avatarFrameContext(s.db, userID, false)
		if err != nil {
			return settings, err
		}
		settings.CanUpload = avatarFrameUploadAllowed(settings, context)
	}
	return settings, nil
}

func avatarFrameUploadAllowed(settings domain.AvatarFrameUploadSettings, context avatarFrameUserContext) bool {
	if context.role == "admin" {
		return true
	}
	if context.memberActive {
		return settings.AllowMemberUpload
	}
	return settings.AllowRegularUpload
}

func (s *Store) UpdateAvatarFrameUploadSettings(actorID int64, allowRegular, allowMember bool) (domain.AvatarFrameUploadSettings, error) {
	now := time.Now().UTC()
	_, err := s.db.Exec(`INSERT INTO avatar_frame_upload_settings (id, allow_regular_upload, allow_member_upload, updated_at) VALUES (1, ?, ?, ?) ON DUPLICATE KEY UPDATE allow_regular_upload = VALUES(allow_regular_upload), allow_member_upload = VALUES(allow_member_upload), updated_at = VALUES(updated_at)`, allowRegular, allowMember, now)
	if err != nil {
		return domain.AvatarFrameUploadSettings{}, err
	}
	_, _ = s.db.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'avatar_frame_settings', 1, 'updated', ?, ?)`, actorID, "普通用户上传="+boolLabel(allowRegular)+"，会员上传="+boolLabel(allowMember), now)
	return s.GetAvatarFrameUploadSettings(0)
}

func boolLabel(value bool) string {
	if value {
		return "开启"
	}
	return "关闭"
}

func (s *Store) CanUploadAvatarFrame(userID int64) (bool, error) {
	settings, err := s.GetAvatarFrameUploadSettings(userID)
	return settings.CanUpload, err
}

func normalizeAvatarFrameSubmission(name, description, imageURL string) (string, string, string, error) {
	frame, err := normalizeAvatarFrame(domain.AvatarFrame{
		Name: name, Description: description, Style: "image", ImageURL: imageURL,
		PrimaryColor: "#1d9bf0", SecondaryColor: "#8b5cf6",
		AllowedRegular: true, AllowedMember: true, AllowedAdmin: true,
	})
	return frame.Name, frame.Description, frame.ImageURL, err
}

func scanAvatarFrameSubmission(scanner rowScanner) (domain.AvatarFrameSubmission, error) {
	var item domain.AvatarFrameSubmission
	var reviewedBy, approvedFrameID sql.NullInt64
	var reviewedAt sql.NullTime
	err := scanner.Scan(
		&item.ID, &item.UserID, &item.UserName, &item.UserAvatar, &item.Name, &item.Description,
		&item.ImageURL, &item.Status, &item.ReviewNote, &reviewedBy, &reviewedAt, &approvedFrameID,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if reviewedBy.Valid {
		item.ReviewedBy = reviewedBy.Int64
	}
	if reviewedAt.Valid {
		value := reviewedAt.Time
		item.ReviewedAt = &value
	}
	if approvedFrameID.Valid {
		item.ApprovedFrameID = approvedFrameID.Int64
	}
	return item, err
}

func (s *Store) CreateAvatarFrameSubmission(userID int64, name, description, imageURL string) (domain.AvatarFrameSubmission, error) {
	canUpload, err := s.CanUploadAvatarFrame(userID)
	if err != nil {
		return domain.AvatarFrameSubmission{}, err
	}
	if !canUpload {
		return domain.AvatarFrameSubmission{}, ErrAvatarFrameUploadForbidden
	}
	name, description, imageURL, err = normalizeAvatarFrameSubmission(name, description, imageURL)
	if err != nil {
		return domain.AvatarFrameSubmission{}, err
	}
	now := time.Now().UTC()
	result, err := s.db.Exec(`INSERT INTO avatar_frame_submissions (user_id, name, description, image_url, status, review_note, created_at, updated_at) VALUES (?, ?, ?, ?, 'pending', '', ?, ?)`, userID, name, description, imageURL, now, now)
	if err != nil {
		return domain.AvatarFrameSubmission{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.AvatarFrameSubmission{}, err
	}
	return s.avatarFrameSubmissionByID(s.db, id, false)
}

func (s *Store) avatarFrameSubmissionByID(queryer rowQueryer, id int64, forUpdate bool) (domain.AvatarFrameSubmission, error) {
	query := `SELECT ` + avatarFrameSubmissionColumns + ` FROM avatar_frame_submissions s JOIN users u ON u.id = s.user_id WHERE s.id = ?`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	return scanAvatarFrameSubmission(queryer.QueryRow(query, id))
}

func (s *Store) ListMyAvatarFrameSubmissions(userID int64) ([]domain.AvatarFrameSubmission, error) {
	return s.listAvatarFrameSubmissions(`WHERE s.user_id = ?`, userID)
}

func (s *Store) ListAdminAvatarFrameSubmissions(status string) ([]domain.AvatarFrameSubmission, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "" && status != "pending" && status != "approved" && status != "rejected" {
		return nil, errors.New("头像框投稿状态无效")
	}
	if status == "" {
		return s.listAvatarFrameSubmissions("")
	}
	return s.listAvatarFrameSubmissions(`WHERE s.status = ?`, status)
}

func (s *Store) listAvatarFrameSubmissions(where string, args ...any) ([]domain.AvatarFrameSubmission, error) {
	rows, err := s.db.Query(`SELECT `+avatarFrameSubmissionColumns+` FROM avatar_frame_submissions s JOIN users u ON u.id = s.user_id `+where+` ORDER BY s.created_at DESC LIMIT 200`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.AvatarFrameSubmission, 0)
	for rows.Next() {
		item, err := scanAvatarFrameSubmission(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ReviewAvatarFrameSubmission(actorID, submissionID int64, status, note string) (domain.AvatarFrameSubmission, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	note = strings.TrimSpace(note)
	if status != "approved" && status != "rejected" {
		return domain.AvatarFrameSubmission{}, errors.New("头像框审核状态无效")
	}
	if status == "rejected" && note == "" {
		return domain.AvatarFrameSubmission{}, errors.New("请填写驳回原因")
	}
	if len([]rune(note)) > 500 || containsControl(note) {
		return domain.AvatarFrameSubmission{}, errors.New("审核备注不能超过 500 个字符")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.AvatarFrameSubmission{}, err
	}
	defer tx.Rollback()
	item, err := s.avatarFrameSubmissionByID(tx, submissionID, true)
	if err != nil {
		return domain.AvatarFrameSubmission{}, err
	}
	if item.Status != "pending" {
		return domain.AvatarFrameSubmission{}, errors.New("该头像框投稿已处理")
	}
	now := time.Now().UTC()
	var approvedFrameID any
	if status == "approved" {
		result, err := tx.Exec(`INSERT INTO avatar_frames (name, description, style, image_url, primary_color, secondary_color, price_cents, allowed_regular, allowed_member, allowed_admin, enabled, sort_order, sales_count, created_at, updated_at) VALUES (?, ?, 'image', ?, '#1d9bf0', '#8b5cf6', 0, 1, 1, 1, 1, 0, 0, ?, ?)`, item.Name, item.Description, item.ImageURL, now, now)
		if err != nil {
			return domain.AvatarFrameSubmission{}, err
		}
		frameID, err := result.LastInsertId()
		if err != nil {
			return domain.AvatarFrameSubmission{}, err
		}
		approvedFrameID = frameID
		_, _ = tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'avatar_frame', ?, 'submission_approved', ?, ?)`, actorID, frameID, item.Name, now)
	}
	if _, err := tx.Exec(`UPDATE avatar_frame_submissions SET status = ?, review_note = ?, reviewed_by = ?, reviewed_at = ?, approved_frame_id = ?, updated_at = ? WHERE id = ?`, status, note, actorID, now, approvedFrameID, now, submissionID); err != nil {
		return domain.AvatarFrameSubmission{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.AvatarFrameSubmission{}, err
	}
	return s.avatarFrameSubmissionByID(s.db, submissionID, false)
}
