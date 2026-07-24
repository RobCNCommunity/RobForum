package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

type ModerationErrorKind string

const (
	ModerationErrorInvalid   ModerationErrorKind = "invalid"
	ModerationErrorNotFound  ModerationErrorKind = "not_found"
	ModerationErrorForbidden ModerationErrorKind = "forbidden"
	ModerationErrorConflict  ModerationErrorKind = "conflict"
)

type ModerationError struct {
	Kind    ModerationErrorKind
	Message string
}

func (e *ModerationError) Error() string { return e.Message }

func moderationError(kind ModerationErrorKind, message string) error {
	return &ModerationError{Kind: kind, Message: message}
}

func (s *Store) ListAdminUsers(query, status string, limit int) ([]domain.AdminUser, error) {
	query = strings.TrimSpace(query)
	status = strings.ToLower(strings.TrimSpace(status))
	if len([]rune(query)) > 100 {
		return nil, moderationError(ModerationErrorInvalid, "用户搜索内容过长")
	}
	if status != "" && status != "active" && status != "banned" && status != "deleted" {
		return nil, moderationError(ModerationErrorInvalid, "用户状态筛选无效")
	}
	if limit < 1 || limit > 200 {
		limit = 100
	}
	where := "1 = 1"
	args := make([]any, 0, 5)
	if status != "" {
		where += " AND u.status = ?"
		args = append(args, status)
	}
	if query != "" {
		pattern := "%" + escapeLike(query) + "%"
		where += ` AND (u.display_name LIKE ? ESCAPE '\\' OR u.email LIKE ? ESCAPE '\\' OR CAST(u.id AS CHAR) = ?)`
		args = append(args, pattern, pattern, query)
	}
	args = append(args, limit)
	rows, err := s.db.Query(`SELECT u.id, u.email, u.display_name, u.avatar_url, u.role, u.status, u.blue_verified, u.verification_label, COALESCE(u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP(), 0), CASE WHEN u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP() THEN u.membership_tier_id ELSE 0 END, u.membership_expires_at,
		(SELECT COUNT(*) FROM posts p WHERE p.author_id = u.id AND p.status <> 'deleted'),
		(SELECT COUNT(*) FROM comments c WHERE c.author_id = u.id AND c.status = 'published'),
		(SELECT COUNT(*) FROM resources r WHERE r.creator_id = u.id AND r.status <> 'takedown'),
		u.created_at, u.updated_at
		FROM users u WHERE `+where+`
		ORDER BY CASE WHEN u.role = 'admin' THEN 0 ELSE 1 END, u.created_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.AdminUser, 0)
	for rows.Next() {
		var item domain.AdminUser
		var verified, memberActive int
		var membershipExpiresAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Email, &item.DisplayName, &item.AvatarURL, &item.Role, &item.Status, &verified, &item.VerificationLabel, &memberActive, &item.MembershipTierID, &membershipExpiresAt, &item.PostCount, &item.CommentCount, &item.ResourceCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.BlueVerified = verified != 0
		item.MemberActive = memberActive != 0
		if membershipExpiresAt.Valid {
			value := membershipExpiresAt.Time.UTC()
			item.MembershipExpiresAt = &value
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// SetAdminUserStatus preserves the historical API for callers that only need
// the updated user record. Administrative HTTP handlers should use the
// cleanup-aware variant so files do not survive a ban on disk.
func (s *Store) SetAdminUserStatus(actorID, targetID int64, status, reason string) (domain.User, error) {
	user, _, err := s.SetAdminUserStatusWithResourceCleanup(actorID, targetID, status, reason)
	return user, err
}

func (s *Store) SetAdminUserStatusWithResourceCleanup(actorID, targetID int64, status, reason string) (domain.User, []string, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "active" && status != "banned" {
		return domain.User{}, nil, moderationError(ModerationErrorInvalid, "账号状态无效")
	}
	reason, err := normalizeModerationReason(reason, true)
	if err != nil {
		return domain.User{}, nil, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.User{}, nil, err
	}
	defer tx.Rollback()
	_, currentStatus, _, err := lockAdminTarget(tx, actorID, targetID)
	if err != nil {
		return domain.User{}, nil, err
	}
	if currentStatus == "deleted" {
		return domain.User{}, nil, moderationError(ModerationErrorConflict, "已删除账号不能恢复或封禁")
	}
	if currentStatus == status {
		return domain.User{}, nil, moderationError(ModerationErrorConflict, "账号状态没有变化")
	}
	now := time.Now().UTC()
	result, err := tx.Exec(`UPDATE users SET status = ?, updated_at = ? WHERE id = ?`, status, now, targetID)
	if err != nil {
		return domain.User{}, nil, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return domain.User{}, nil, err
		}
		return domain.User{}, nil, moderationError(ModerationErrorNotFound, "用户不存在")
	}
	if _, err := tx.Exec(`DELETE FROM auth_sessions WHERE user_id = ?`, targetID); err != nil {
		return domain.User{}, nil, err
	}
	if _, err := tx.Exec(`DELETE FROM password_resets WHERE user_id = ?`, targetID); err != nil {
		return domain.User{}, nil, err
	}
	var resourceFiles []string
	if status == "banned" {
		if _, err := tx.Exec(`UPDATE posts SET status = 'hidden', pinned = 0, featured = 0, updated_at = ? WHERE author_id = ? AND status = 'published'`, now, targetID); err != nil {
			return domain.User{}, nil, err
		}
		if _, err := tx.Exec(`UPDATE comments SET status = 'deleted' WHERE author_id = ? AND status = 'published'`, targetID); err != nil {
			return domain.User{}, nil, err
		}
		if _, err := tx.Exec(`UPDATE resources SET status = 'takedown', review_reason = ?, updated_at = ? WHERE creator_id = ? AND status IN ('pending', 'approved')`, reason, now, targetID); err != nil {
			return domain.User{}, nil, err
		}
		resourceFiles, err = removeResourceFilesForCreator(tx, targetID)
		if err != nil {
			return domain.User{}, nil, err
		}
		if err := recalculateCommentCountsForAuthor(tx, targetID); err != nil {
			return domain.User{}, nil, err
		}
	}
	action := "user_unbanned"
	if status == "banned" {
		action = "user_banned"
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'user', ?, ?, ?, ?)`, actorID, targetID, action, reason, now); err != nil {
		return domain.User{}, nil, err
	}
	user, err := getUser(tx, targetID)
	if err != nil {
		return domain.User{}, nil, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, nil, err
	}
	return user, resourceFiles, nil
}

// DeleteAdminUser retains the original return shape for non-HTTP callers.
func (s *Store) DeleteAdminUser(actorID, targetID int64, reason string) error {
	_, err := s.DeleteAdminUserWithResourceCleanup(actorID, targetID, reason)
	return err
}

func (s *Store) DeleteAdminUserWithResourceCleanup(actorID, targetID int64, reason string) ([]string, error) {
	reason, err := normalizeModerationReason(reason, true)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	_, currentStatus, currentEmail, err := lockAdminTarget(tx, actorID, targetID)
	if err != nil {
		return nil, err
	}
	if currentStatus == "deleted" {
		return nil, moderationError(ModerationErrorConflict, "账号已经删除")
	}
	now := time.Now().UTC()
	anonymizedEmail := fmt.Sprintf("deleted+%d@invalid.local", targetID)
	result, err := tx.Exec(`UPDATE users SET email = ?, password_hash = ?, display_name = '已删除用户', avatar_url = '', bio = '', status = 'deleted', blue_verified = 0, verification_label = '', roblox_name = '', roblox_id = '', roblox_verified = 0, updated_at = ? WHERE id = ?`, anonymizedEmail, string(dummyPasswordHash), now, targetID)
	if err != nil {
		return nil, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return nil, err
		}
		return nil, moderationError(ModerationErrorNotFound, "用户不存在")
	}
	statements := []struct {
		query string
		args  []any
	}{
		{`DELETE FROM auth_sessions WHERE user_id = ?`, []any{targetID}},
		{`DELETE FROM password_resets WHERE user_id = ?`, []any{targetID}},
		{`DELETE FROM oauth_accounts WHERE user_id = ?`, []any{targetID}},
		{`DELETE FROM email_verifications WHERE email = ?`, []any{currentEmail}},
		{`DELETE FROM user_follows WHERE follower_id = ? OR followed_id = ?`, []any{targetID, targetID}},
		{`DELETE FROM user_blocks WHERE blocker_id = ? OR blocked_id = ?`, []any{targetID, targetID}},
		{`UPDATE posts SET status = 'deleted', pinned = 0, featured = 0, comment_count = 0, updated_at = ? WHERE author_id = ? AND status <> 'deleted'`, []any{now, targetID}},
		{`UPDATE comments SET status = 'deleted' WHERE author_id = ?`, []any{targetID}},
		{`UPDATE comments c JOIN posts p ON p.id = c.post_id SET c.status = 'deleted' WHERE p.author_id = ?`, []any{targetID}},
		{`UPDATE resources SET status = 'takedown', review_reason = ?, updated_at = ? WHERE creator_id = ? AND status <> 'takedown'`, []any{reason, now, targetID}},
		{`UPDATE verification_applications SET status = 'rejected', review_note = ?, reviewed_by = ?, reviewed_at = ?, updated_at = ? WHERE user_id = ? AND status = 'pending'`, []any{"账号已删除：" + reason, actorID, now, now, targetID}},
	}
	for _, statement := range statements {
		if _, err := tx.Exec(statement.query, statement.args...); err != nil {
			return nil, err
		}
	}
	resourceFiles, err := removeResourceFilesForCreator(tx, targetID)
	if err != nil {
		return nil, err
	}
	if err := recalculateCommentCountsForAuthor(tx, targetID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'user', ?, 'user_deleted', ?, ?)`, actorID, targetID, reason, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return resourceFiles, nil
}

func removeResourceFilesForCreator(tx *sql.Tx, creatorID int64) ([]string, error) {
	rows, err := tx.Query(`SELECT f.stored_name FROM resource_files f JOIN resources r ON r.id = f.resource_id WHERE r.creator_id = ? FOR UPDATE`, creatorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	storedNames := make([]string, 0)
	for rows.Next() {
		var storedName string
		if err := rows.Scan(&storedName); err != nil {
			return nil, err
		}
		storedNames = append(storedNames, storedName)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`DELETE f FROM resource_files f JOIN resources r ON r.id = f.resource_id WHERE r.creator_id = ?`, creatorID); err != nil {
		return nil, err
	}
	return storedNames, nil
}

func lockAdminTarget(tx *sql.Tx, actorID, targetID int64) (role, status, email string, err error) {
	if actorID <= 0 || targetID <= 0 {
		return "", "", "", moderationError(ModerationErrorNotFound, "用户不存在")
	}
	if actorID == targetID {
		return "", "", "", moderationError(ModerationErrorForbidden, "管理员不能处理自己的账号")
	}
	var actorRole, actorStatus string
	if err := tx.QueryRow(`SELECT role, status FROM users WHERE id = ? FOR UPDATE`, actorID).Scan(&actorRole, &actorStatus); err != nil {
		return "", "", "", err
	}
	if actorRole != "admin" || actorStatus != "active" {
		return "", "", "", moderationError(ModerationErrorForbidden, "需要有效的管理员账号")
	}
	if err := tx.QueryRow(`SELECT role, status, email FROM users WHERE id = ? FOR UPDATE`, targetID).Scan(&role, &status, &email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", "", moderationError(ModerationErrorNotFound, "用户不存在")
		}
		return "", "", "", err
	}
	if role == "admin" {
		return "", "", "", moderationError(ModerationErrorForbidden, "管理员账号受保护，不能在此处封禁或删除")
	}
	return role, status, email, nil
}

func recalculateCommentCountsForAuthor(tx *sql.Tx, authorID int64) error {
	_, err := tx.Exec(`UPDATE posts p SET comment_count = (SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id AND c.status = 'published') WHERE EXISTS (SELECT 1 FROM comments authored WHERE authored.post_id = p.id AND authored.author_id = ?)`, authorID)
	return err
}

func (s *Store) ListAdminPosts(status, query string, postID int64, limit int) ([]domain.Post, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	query = strings.TrimSpace(query)
	if !validPostStatusFilter(status) {
		return nil, moderationError(ModerationErrorInvalid, "帖子状态筛选无效")
	}
	if len([]rune(query)) > 100 {
		return nil, moderationError(ModerationErrorInvalid, "帖子搜索内容过长")
	}
	if limit < 1 || limit > 200 {
		limit = 100
	}
	where := "1 = 1"
	args := make([]any, 0, 6)
	if status != "" {
		where += " AND p.status = ?"
		args = append(args, status)
	}
	if postID > 0 {
		where += " AND p.id = ?"
		args = append(args, postID)
	}
	if query != "" {
		pattern := "%" + escapeLike(query) + "%"
		where += ` AND (p.title LIKE ? ESCAPE '\\' OR p.content LIKE ? ESCAPE '\\' OR u.display_name LIKE ? ESCAPE '\\')`
		args = append(args, pattern, pattern, pattern)
	}
	args = append(args, limit)
	rows, err := s.db.Query(`SELECT p.id, p.board_id, b.name, p.author_id, u.display_name, u.avatar_url, u.blue_verified, u.verification_label, COALESCE(u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP(), 0), CASE WHEN u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP() THEN u.membership_tier_id ELSE 0 END, p.title, p.content, p.status, p.pinned, p.featured, p.views, p.comment_count, (SELECT COUNT(*) FROM post_likes pl WHERE pl.post_id = p.id), (SELECT COUNT(*) FROM post_reposts pr WHERE pr.post_id = p.id), p.created_at, p.updated_at
		FROM posts p JOIN boards b ON b.id = p.board_id JOIN users u ON u.id = p.author_id
		WHERE `+where+`
		ORDER BY CASE p.status WHEN 'pending' THEN 0 WHEN 'published' THEN 1 WHEN 'hidden' THEN 2 WHEN 'rejected' THEN 3 ELSE 4 END, p.updated_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Post, 0)
	for rows.Next() {
		item, err := scanModerationPost(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := attachPostMedia(s.db, items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ModeratePost(actorID, postID int64, status, reason string) (domain.Post, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	reason, err := normalizeModerationReason(reason, status != "published")
	if err != nil {
		return domain.Post{}, err
	}
	if status != "published" && status != "rejected" && status != "hidden" && status != "deleted" {
		return domain.Post{}, moderationError(ModerationErrorInvalid, "帖子审核状态无效")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Post{}, err
	}
	defer tx.Rollback()
	var actorRole, actorStatus string
	if err := tx.QueryRow(`SELECT role, status FROM users WHERE id = ? FOR UPDATE`, actorID).Scan(&actorRole, &actorStatus); err != nil {
		return domain.Post{}, err
	}
	if actorRole != "admin" || actorStatus != "active" {
		return domain.Post{}, moderationError(ModerationErrorForbidden, "需要有效的管理员账号")
	}
	var currentStatus string
	if err := tx.QueryRow(`SELECT status FROM posts WHERE id = ? FOR UPDATE`, postID).Scan(&currentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Post{}, moderationError(ModerationErrorNotFound, "帖子不存在")
		}
		return domain.Post{}, err
	}
	if !canModeratePost(currentStatus, status) {
		return domain.Post{}, moderationError(ModerationErrorConflict, "当前帖子状态不能执行该操作")
	}
	now := time.Now().UTC()
	result, err := tx.Exec(`UPDATE posts SET status = ?, pinned = CASE WHEN ? = 'published' THEN pinned ELSE 0 END, featured = CASE WHEN ? = 'published' THEN featured ELSE 0 END, updated_at = ? WHERE id = ?`, status, status, status, now, postID)
	if err != nil {
		return domain.Post{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return domain.Post{}, err
		}
		return domain.Post{}, moderationError(ModerationErrorNotFound, "帖子不存在")
	}
	if status == "deleted" {
		if _, err := tx.Exec(`UPDATE comments SET status = 'deleted' WHERE post_id = ?`, postID); err != nil {
			return domain.Post{}, err
		}
		if _, err := tx.Exec(`UPDATE posts SET comment_count = 0 WHERE id = ?`, postID); err != nil {
			return domain.Post{}, err
		}
	}
	action := map[string]string{"published": "post_published", "rejected": "post_rejected", "hidden": "post_hidden", "deleted": "post_deleted"}[status]
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'post', ?, ?, ?, ?)`, actorID, postID, action, reason, now); err != nil {
		return domain.Post{}, err
	}
	item, err := getPostForModeration(tx, postID)
	if err != nil {
		return domain.Post{}, err
	}
	item.Media, err = getPostMedia(tx, postID)
	if err != nil {
		return domain.Post{}, err
	}
	item.Tags, err = getPostTags(tx, postID)
	if err != nil {
		return domain.Post{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Post{}, err
	}
	return item, nil
}

func getPostForModeration(queryer rowQueryer, id int64) (domain.Post, error) {
	row := queryer.QueryRow(`SELECT p.id, p.board_id, b.name, p.author_id, u.display_name, u.avatar_url, u.blue_verified, u.verification_label, COALESCE(u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP(), 0), CASE WHEN u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP() THEN u.membership_tier_id ELSE 0 END, p.title, p.content, p.status, p.pinned, p.featured, p.views, p.comment_count, (SELECT COUNT(*) FROM post_likes pl WHERE pl.post_id = p.id), (SELECT COUNT(*) FROM post_reposts pr WHERE pr.post_id = p.id), p.created_at, p.updated_at FROM posts p JOIN boards b ON b.id = p.board_id JOIN users u ON u.id = p.author_id WHERE p.id = ?`, id)
	return scanModerationPost(row)
}

func scanModerationPost(scanner rowScanner) (domain.Post, error) {
	var item domain.Post
	var authorVerified, authorMember, pinned, featured int
	err := scanner.Scan(&item.ID, &item.BoardID, &item.BoardName, &item.AuthorID, &item.AuthorName, &item.AuthorAvatar, &authorVerified, &item.AuthorVerificationLabel, &authorMember, &item.AuthorMembershipTierID, &item.Title, &item.Content, &item.Status, &pinned, &featured, &item.Views, &item.CommentCount, &item.LikeCount, &item.RepostCount, &item.CreatedAt, &item.UpdatedAt)
	item.AuthorVerified = authorVerified != 0
	item.AuthorMember = authorMember != 0
	item.Pinned = pinned != 0
	item.Featured = featured != 0
	return item, err
}

func (s *Store) CanServePostMedia(storedName string, viewerID int64, admin bool) (allowed, public bool, err error) {
	var postStatus, boardStatus, authorStatus string
	var authorID int64
	err = s.db.QueryRow(`SELECT p.status, p.author_id, b.status, u.status FROM post_media pm JOIN posts p ON p.id = pm.post_id JOIN boards b ON b.id = p.board_id JOIN users u ON u.id = p.author_id WHERE pm.stored_name = ?`, storedName).Scan(&postStatus, &authorID, &boardStatus, &authorStatus)
	if err != nil {
		return false, false, err
	}
	if postStatus == "published" && boardStatus == "active" && authorStatus == "active" {
		return true, true, nil
	}
	if admin {
		return true, false, nil
	}
	return viewerID > 0 && viewerID == authorID && authorStatus == "active" && postStatus != "deleted", false, nil
}

func normalizeModerationReason(reason string, required bool) (string, error) {
	reason = strings.TrimSpace(reason)
	if required && len([]rune(reason)) < 2 {
		return "", moderationError(ModerationErrorInvalid, "请填写至少 2 个字符的处理原因")
	}
	if len([]rune(reason)) > 500 || containsControl(reason) {
		return "", moderationError(ModerationErrorInvalid, "处理原因不能超过 500 个字符且不能包含控制字符")
	}
	return reason, nil
}

func validPostStatusFilter(status string) bool {
	return status == "" || status == "pending" || status == "published" || status == "hidden" || status == "rejected" || status == "deleted"
}

func canModeratePost(current, next string) bool {
	if current == next || current == "deleted" {
		return false
	}
	switch current {
	case "pending":
		return next == "published" || next == "rejected" || next == "hidden" || next == "deleted"
	case "published":
		return next == "hidden" || next == "rejected" || next == "deleted"
	case "hidden":
		return next == "published" || next == "rejected" || next == "deleted"
	case "rejected":
		return next == "published" || next == "deleted"
	default:
		return false
	}
}
