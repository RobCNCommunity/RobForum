package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

const (
	contentReportTargetPost    = "post"
	contentReportTargetComment = "comment"
	contentReportTargetProfile = "profile"
)

func (s *Store) CreateContentReport(reporterID int64, targetType string, targetID int64, reason string) (domain.ContentReport, error) {
	targetType = strings.ToLower(strings.TrimSpace(targetType))
	if reporterID <= 0 || targetID <= 0 || !validContentReportTargetType(targetType) {
		return domain.ContentReport{}, moderationError(ModerationErrorInvalid, "举报对象无效")
	}
	reason, err := normalizeContentReportReason(reason)
	if err != nil {
		return domain.ContentReport{}, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return domain.ContentReport{}, err
	}
	defer tx.Rollback()

	// Resolve first, then acquire account locks in a stable order. This avoids
	// a deadlock when two users report each other's public profiles at once.
	targetUserID, err := reportTargetUserID(tx, targetType, targetID, false)
	if err != nil {
		return domain.ContentReport{}, err
	}
	if targetUserID == reporterID {
		return domain.ContentReport{}, moderationError(ModerationErrorInvalid, "不能举报自己的内容或个人资料")
	}
	if err := lockActiveUsers(tx, []int64{reporterID, targetUserID}); err != nil {
		return domain.ContentReport{}, moderationError(ModerationErrorNotFound, "举报对象不存在或已不可见")
	}
	if _, err := reportTargetUserID(tx, targetType, targetID, true); err != nil {
		return domain.ContentReport{}, err
	}

	var existingID int64
	err = tx.QueryRow(`SELECT id FROM content_reports WHERE reporter_id = ? AND target_type = ? AND target_id = ? AND status = 'pending' LIMIT 1 FOR UPDATE`, reporterID, targetType, targetID).Scan(&existingID)
	if err == nil {
		return domain.ContentReport{}, moderationError(ModerationErrorConflict, "你已经举报过该内容，正在等待审核")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.ContentReport{}, err
	}

	now := time.Now().UTC()
	result, err := tx.Exec(`INSERT INTO content_reports (reporter_id, target_type, target_id, target_user_id, reason, status, review_note, created_at) VALUES (?, ?, ?, ?, ?, 'pending', '', ?)`, reporterID, targetType, targetID, targetUserID, reason, now)
	if err != nil {
		return domain.ContentReport{}, err
	}
	reportID, err := result.LastInsertId()
	if err != nil {
		return domain.ContentReport{}, err
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, ?, ?, 'report_submitted', ?, ?)`, reporterID, targetType, targetID, reason, now); err != nil {
		return domain.ContentReport{}, err
	}
	item, err := getContentReport(tx, reportID)
	if err != nil {
		return domain.ContentReport{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ContentReport{}, err
	}
	return item, nil
}

func (s *Store) ListAdminContentReports(status string, limit int) ([]domain.ContentReport, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if !validContentReportStatusFilter(status) {
		return nil, moderationError(ModerationErrorInvalid, "举报状态筛选无效")
	}
	if limit < 1 || limit > 200 {
		limit = 100
	}
	query := contentReportSelect
	args := make([]any, 0, 2)
	if status != "" {
		query += ` WHERE r.status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY CASE r.status WHEN 'pending' THEN 0 WHEN 'accepted' THEN 1 ELSE 2 END, r.created_at DESC, r.id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.ContentReport, 0)
	for rows.Next() {
		item, err := scanContentReport(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ReviewContentReport(actorID, reportID int64, status, note string) (domain.ContentReport, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if reportID <= 0 || (status != "accepted" && status != "rejected") {
		return domain.ContentReport{}, moderationError(ModerationErrorInvalid, "举报审核状态无效")
	}
	note, err := normalizeModerationReason(note, true)
	if err != nil {
		return domain.ContentReport{}, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return domain.ContentReport{}, err
	}
	defer tx.Rollback()
	if err := lockActiveAdminReviewer(tx, actorID); err != nil {
		return domain.ContentReport{}, err
	}

	var currentStatus string
	if err := tx.QueryRow(`SELECT status FROM content_reports WHERE id = ? FOR UPDATE`, reportID).Scan(&currentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ContentReport{}, moderationError(ModerationErrorNotFound, "举报记录不存在")
		}
		return domain.ContentReport{}, err
	}
	if currentStatus != "pending" {
		return domain.ContentReport{}, moderationError(ModerationErrorConflict, "该举报已处理")
	}
	report, err := getContentReport(tx, reportID)
	if err != nil {
		return domain.ContentReport{}, err
	}

	now := time.Now().UTC()
	if status == "accepted" {
		if err := applyAcceptedContentReport(tx, actorID, report, note, now); err != nil {
			return domain.ContentReport{}, err
		}
	}
	result, err := tx.Exec(`UPDATE content_reports SET status = ?, review_note = ?, reviewed_by = ?, reviewed_at = ? WHERE id = ? AND status = 'pending'`, status, note, actorID, now, reportID)
	if err != nil {
		return domain.ContentReport{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return domain.ContentReport{}, err
		}
		return domain.ContentReport{}, moderationError(ModerationErrorConflict, "该举报已处理")
	}
	action := "report_rejected"
	if status == "accepted" {
		action = "report_accepted"
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'report', ?, ?, ?, ?)`, actorID, reportID, action, note, now); err != nil {
		return domain.ContentReport{}, err
	}

	postID, commentID := contentReportNotificationReferences(report)
	if err := createNotification(tx, report.ReporterID, actorID, action, postID, commentID); err != nil {
		return domain.ContentReport{}, err
	}
	if status == "accepted" {
		if err := createNotification(tx, report.TargetUserID, actorID, "content_report_accepted", postID, commentID); err != nil {
			return domain.ContentReport{}, err
		}
	}
	item, err := getContentReport(tx, reportID)
	if err != nil {
		return domain.ContentReport{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ContentReport{}, err
	}
	return item, nil
}

func reportTargetUserID(tx *sql.Tx, targetType string, targetID int64, lock bool) (int64, error) {
	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE"
	}
	var userID int64
	var err error
	switch targetType {
	case contentReportTargetPost:
		err = tx.QueryRow(`SELECT p.author_id FROM posts p JOIN boards b ON b.id = p.board_id JOIN users u ON u.id = p.author_id WHERE p.id = ? AND p.status = 'published' AND b.status = 'active' AND u.status = 'active'`+lockClause, targetID).Scan(&userID)
	case contentReportTargetComment:
		err = tx.QueryRow(`SELECT c.author_id FROM comments c JOIN posts p ON p.id = c.post_id JOIN boards b ON b.id = p.board_id JOIN users author ON author.id = c.author_id JOIN users post_author ON post_author.id = p.author_id WHERE c.id = ? AND c.status = 'published' AND p.status = 'published' AND b.status = 'active' AND author.status = 'active' AND post_author.status = 'active'`+lockClause, targetID).Scan(&userID)
	case contentReportTargetProfile:
		err = tx.QueryRow(`SELECT id FROM users WHERE id = ? AND status = 'active' AND profile_status = 'active'`+lockClause, targetID).Scan(&userID)
	default:
		return 0, moderationError(ModerationErrorInvalid, "举报对象无效")
	}
	if errors.Is(err, sql.ErrNoRows) {
		return 0, moderationError(ModerationErrorNotFound, "举报对象不存在或已不可见")
	}
	return userID, err
}

func applyAcceptedContentReport(tx *sql.Tx, actorID int64, report domain.ContentReport, note string, now time.Time) error {
	action := ""
	switch report.TargetType {
	case contentReportTargetPost:
		if _, err := tx.Exec(`UPDATE posts SET status = 'hidden', pinned = 0, featured = 0, updated_at = ? WHERE id = ? AND status IN ('pending', 'published', 'rejected', 'hidden')`, now, report.TargetID); err != nil {
			return err
		}
		action = "report_post_hidden"
	case contentReportTargetComment:
		var postID int64
		if err := tx.QueryRow(`SELECT post_id FROM comments WHERE id = ? FOR UPDATE`, report.TargetID).Scan(&postID); err == nil {
			if _, err := tx.Exec(`UPDATE comments SET status = 'hidden' WHERE id = ? AND status = 'published'`, report.TargetID); err != nil {
				return err
			}
			if _, err := tx.Exec(`UPDATE posts SET comment_count = (SELECT COUNT(*) FROM comments WHERE post_id = ? AND status = 'published'), updated_at = ? WHERE id = ?`, postID, now, postID); err != nil {
				return err
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		action = "report_comment_hidden"
	case contentReportTargetProfile:
		if _, err := tx.Exec(`UPDATE users SET profile_status = 'hidden', updated_at = ? WHERE id = ? AND status = 'active'`, now, report.TargetUserID); err != nil {
			return err
		}
		action = "report_profile_hidden"
	default:
		return moderationError(ModerationErrorInvalid, "举报对象无效")
	}
	_, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, ?, ?, ?, ?, ?)`, actorID, report.TargetType, report.TargetID, action, note, now)
	return err
}

func lockActiveAdminReviewer(tx *sql.Tx, actorID int64) error {
	if actorID <= 0 {
		return moderationError(ModerationErrorForbidden, "需要有效的管理员账号")
	}
	var role, status string
	if err := tx.QueryRow(`SELECT role, status FROM users WHERE id = ? FOR UPDATE`, actorID).Scan(&role, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return moderationError(ModerationErrorForbidden, "需要有效的管理员账号")
		}
		return err
	}
	if role != "admin" || status != "active" {
		return moderationError(ModerationErrorForbidden, "需要有效的管理员账号")
	}
	return nil
}

func getContentReport(queryer rowQueryer, reportID int64) (domain.ContentReport, error) {
	return scanContentReport(queryer.QueryRow(contentReportSelect+` WHERE r.id = ?`, reportID))
}

const contentReportSelect = `SELECT
	r.id, r.reporter_id, reporter.display_name, reporter.avatar_url,
	r.target_type, r.target_id, r.target_user_id, target_user.display_name, target_user.avatar_url,
	CASE r.target_type WHEN 'post' THEN p.id WHEN 'comment' THEN c.post_id ELSE NULL END,
	CASE r.target_type WHEN 'post' THEN COALESCE(p.title, '已删除帖子') WHEN 'comment' THEN CONCAT('评论 #', r.target_id) ELSE target_user.display_name END,
	CASE r.target_type WHEN 'post' THEN COALESCE(p.content, '') WHEN 'comment' THEN COALESCE(c.content, '') ELSE CONCAT('昵称：', target_user.display_name, CASE WHEN target_user.bio = '' THEN '' ELSE CONCAT('\n简介：', target_user.bio) END) END,
	CASE r.target_type WHEN 'post' THEN COALESCE(p.status, 'deleted') WHEN 'comment' THEN COALESCE(c.status, 'deleted') ELSE target_user.profile_status END,
	r.reason, r.status, r.review_note, r.reviewed_by, COALESCE(reviewer.display_name, ''), r.reviewed_at, r.created_at
	FROM content_reports r
	JOIN users reporter ON reporter.id = r.reporter_id
	JOIN users target_user ON target_user.id = r.target_user_id
	LEFT JOIN users reviewer ON reviewer.id = r.reviewed_by
	LEFT JOIN posts p ON r.target_type = 'post' AND p.id = r.target_id
	LEFT JOIN comments c ON r.target_type = 'comment' AND c.id = r.target_id`

func scanContentReport(scanner rowScanner) (domain.ContentReport, error) {
	var item domain.ContentReport
	var targetPostID, reviewedBy sql.NullInt64
	var reviewedAt sql.NullTime
	err := scanner.Scan(
		&item.ID, &item.ReporterID, &item.ReporterName, &item.ReporterAvatar,
		&item.TargetType, &item.TargetID, &item.TargetUserID, &item.TargetUserName, &item.TargetUserAvatar,
		&targetPostID, &item.TargetTitle, &item.TargetContent, &item.TargetStatus,
		&item.Reason, &item.Status, &item.ReviewNote, &reviewedBy, &item.ReviewerName, &reviewedAt, &item.CreatedAt,
	)
	if targetPostID.Valid {
		item.TargetPostID = &targetPostID.Int64
	}
	if reviewedBy.Valid {
		item.ReviewedBy = &reviewedBy.Int64
	}
	if reviewedAt.Valid {
		item.ReviewedAt = &reviewedAt.Time
	}
	return item, err
}

func contentReportNotificationReferences(report domain.ContentReport) (any, any) {
	switch report.TargetType {
	case contentReportTargetPost:
		return report.TargetID, nil
	case contentReportTargetComment:
		if report.TargetPostID != nil {
			return *report.TargetPostID, report.TargetID
		}
		return nil, report.TargetID
	default:
		return nil, nil
	}
}

func normalizeContentReportReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) < 2 {
		return "", moderationError(ModerationErrorInvalid, "请填写至少 2 个字符的举报原因")
	}
	if len([]rune(reason)) > 500 || containsControl(reason) {
		return "", moderationError(ModerationErrorInvalid, "举报原因不能超过 500 个字符且不能包含控制字符")
	}
	return reason, nil
}

func validContentReportTargetType(targetType string) bool {
	return targetType == contentReportTargetPost || targetType == contentReportTargetComment || targetType == contentReportTargetProfile
}

func validContentReportStatusFilter(status string) bool {
	return status == "" || status == "pending" || status == "accepted" || status == "rejected"
}
