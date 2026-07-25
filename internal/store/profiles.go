package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

var ErrProfileUnavailable = errors.New("个人资料正在审核中，暂时无法修改")

func (s *Store) UpdateUserProfile(userID int64, displayName, bio string) (domain.User, error) {
	displayName = strings.TrimSpace(displayName)
	bio = strings.TrimSpace(bio)
	if len([]rune(displayName)) < 2 || len([]rune(displayName)) > 80 || containsControl(displayName) {
		return domain.User{}, errors.New("昵称长度需要在 2 到 80 个字符之间")
	}
	if len([]rune(bio)) > 500 || containsControl(bio) {
		return domain.User{}, errors.New("个人简介不能超过 500 个字符")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE users SET display_name = ?, bio = ?, updated_at = ? WHERE id = ? AND status = 'active' AND profile_status = 'active'`, displayName, bio, time.Now().UTC(), userID)
	if err != nil {
		return domain.User{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return domain.User{}, err
		}
		return domain.User{}, ErrProfileUnavailable
	}
	user, err := getUser(tx, userID)
	if err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *Store) UpdateUserAvatar(userID int64, avatarURL string) (domain.User, error) {
	avatarURL = strings.TrimSpace(avatarURL)
	if !strings.HasPrefix(avatarURL, "/api/v1/media/avatars/") || len(avatarURL) > 500 {
		return domain.User{}, errors.New("头像地址无效")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE users SET avatar_url = ?, updated_at = ? WHERE id = ? AND status = 'active' AND profile_status = 'active'`, avatarURL, time.Now().UTC(), userID)
	if err != nil {
		return domain.User{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return domain.User{}, err
		}
		return domain.User{}, ErrProfileUnavailable
	}
	user, err := getUser(tx, userID)
	if err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *Store) UpdateUserCover(userID int64, coverURL string) (domain.User, error) {
	coverURL = strings.TrimSpace(coverURL)
	if coverURL != "" && (!strings.HasPrefix(coverURL, "/api/v1/media/covers/") || len(coverURL) > 500) {
		return domain.User{}, errors.New("封面地址无效")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE users SET cover_url = ?, updated_at = ? WHERE id = ? AND status = 'active' AND profile_status = 'active'`, coverURL, time.Now().UTC(), userID)
	if err != nil {
		return domain.User{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return domain.User{}, err
		}
		return domain.User{}, ErrProfileUnavailable
	}
	user, err := getUser(tx, userID)
	if err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *Store) GetPublicUser(userID int64) (domain.PublicUser, error) {
	var user domain.PublicUser
	var blueVerified, memberActive, robloxVerified int
	err := s.db.QueryRow(`SELECT id, display_name, avatar_url, cover_url, bio, blue_verified, verification_label, COALESCE(membership_tier_id IS NOT NULL AND membership_expires_at > UTC_TIMESTAMP(), 0), CASE WHEN membership_tier_id IS NOT NULL AND membership_expires_at > UTC_TIMESTAMP() THEN membership_tier_id ELSE 0 END, roblox_name, roblox_verified, created_at FROM users WHERE id = ? AND status = 'active' AND profile_status = 'active'`, userID).Scan(&user.ID, &user.DisplayName, &user.AvatarURL, &user.CoverURL, &user.Bio, &blueVerified, &user.VerificationLabel, &memberActive, &user.MembershipTierID, &user.RobloxName, &robloxVerified, &user.CreatedAt)
	if err != nil {
		return user, err
	}
	user.BlueVerified = blueVerified != 0
	user.MemberActive = memberActive != 0
	user.RobloxVerified = robloxVerified != 0
	user.Progress, err = s.GetUserProgress(userID)
	if err != nil {
		return user, err
	}
	user.Badges, err = s.ListUserBadges(userID)
	return user, err
}

func (s *Store) GetUserProfile(userID int64) (domain.UserProfile, error) {
	user, err := s.GetPublicUser(userID)
	if err != nil {
		return domain.UserProfile{}, err
	}
	posts, err := s.ListUserPosts(userID, 30)
	if err != nil {
		return domain.UserProfile{}, err
	}
	resources, err := s.ListResources("approved", userID, 30)
	if err != nil {
		return domain.UserProfile{}, err
	}
	var followerCount, followingCount int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM user_follows WHERE followed_id = ?`, userID).Scan(&followerCount); err != nil {
		return domain.UserProfile{}, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM user_follows WHERE follower_id = ?`, userID).Scan(&followingCount); err != nil {
		return domain.UserProfile{}, err
	}
	return domain.UserProfile{User: user, Posts: posts, Resources: resources, FollowerCount: followerCount, FollowingCount: followingCount}, nil
}

func (s *Store) ListUserPosts(userID int64, limit int) ([]domain.Post, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	rows, err := s.db.Query(`SELECT p.id, p.board_id, b.name, p.author_id, u.display_name, u.avatar_url, u.blue_verified, u.verification_label, COALESCE(u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP(), 0), CASE WHEN u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP() THEN u.membership_tier_id ELSE 0 END, p.title, p.content, p.status, p.pinned, p.featured, p.views, p.comment_count, (SELECT COUNT(*) FROM post_likes pl WHERE pl.post_id = p.id), (SELECT COUNT(*) FROM post_reposts pr WHERE pr.post_id = p.id), p.created_at, p.updated_at FROM posts p JOIN boards b ON b.id = p.board_id JOIN users u ON u.id = p.author_id WHERE p.author_id = ? AND p.status = 'published' AND b.status = 'active' AND u.status = 'active' AND u.profile_status = 'active' ORDER BY p.updated_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.Post, 0)
	for rows.Next() {
		var item domain.Post
		var authorVerified, authorMember, pinned, featured int
		if err := rows.Scan(&item.ID, &item.BoardID, &item.BoardName, &item.AuthorID, &item.AuthorName, &item.AuthorAvatar, &authorVerified, &item.AuthorVerificationLabel, &authorMember, &item.AuthorMembershipTierID, &item.Title, &item.Content, &item.Status, &pinned, &featured, &item.Views, &item.CommentCount, &item.LikeCount, &item.RepostCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.AuthorVerified = authorVerified != 0
		item.AuthorMember = authorMember != 0
		item.Pinned = pinned != 0
		item.Featured = featured != 0
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := attachPostMedia(s.db, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) CreateVerificationApplication(userID int64, verificationType, requestedLabel, evidenceURL, statement string) (domain.VerificationApplication, error) {
	verificationType = strings.ToLower(strings.TrimSpace(verificationType))
	requestedLabel = strings.TrimSpace(requestedLabel)
	evidenceURL = strings.TrimSpace(evidenceURL)
	statement = strings.TrimSpace(statement)
	if verificationType != "creator" && verificationType != "developer" && verificationType != "organization" && verificationType != "community" {
		return domain.VerificationApplication{}, errors.New("请选择有效的认证类型")
	}
	if len([]rune(requestedLabel)) < 2 || len([]rune(requestedLabel)) > 80 || containsControl(requestedLabel) {
		return domain.VerificationApplication{}, errors.New("认证名称长度需要在 2 到 80 个字符之间")
	}
	if err := validateWebURL(evidenceURL, false); err != nil {
		return domain.VerificationApplication{}, errors.New("证明材料链接无效")
	}
	if len([]rune(statement)) < 30 || len([]rune(statement)) > 2000 || containsControl(statement) {
		return domain.VerificationApplication{}, errors.New("申请说明长度需要在 30 到 2000 个字符之间")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.VerificationApplication{}, err
	}
	defer tx.Rollback()
	var blueVerified int
	if err := tx.QueryRow(`SELECT blue_verified FROM users WHERE id = ? AND status = 'active' FOR UPDATE`, userID).Scan(&blueVerified); err != nil {
		return domain.VerificationApplication{}, err
	}
	if blueVerified != 0 {
		return domain.VerificationApplication{}, errors.New("账号已经完成蓝微认证")
	}
	var pending int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM verification_applications WHERE user_id = ? AND status = 'pending'`, userID).Scan(&pending); err != nil {
		return domain.VerificationApplication{}, err
	}
	if pending > 0 {
		return domain.VerificationApplication{}, errors.New("已有待审核的认证申请")
	}
	now := time.Now().UTC()
	result, err := tx.Exec(`INSERT INTO verification_applications (user_id, verification_type, requested_label, evidence_url, statement, status, review_note, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 'pending', '', ?, ?)`, userID, verificationType, requestedLabel, evidenceURL, statement, now, now)
	if err != nil {
		return domain.VerificationApplication{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.VerificationApplication{}, err
	}
	item, err := getVerificationApplication(tx, id)
	if err != nil {
		return domain.VerificationApplication{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.VerificationApplication{}, err
	}
	return item, nil
}

func (s *Store) GetVerificationApplication(id int64) (domain.VerificationApplication, error) {
	return getVerificationApplication(s.db, id)
}

func getVerificationApplication(queryer rowQueryer, id int64) (domain.VerificationApplication, error) {
	row := queryer.QueryRow(`SELECT a.id, a.user_id, u.display_name, u.email, a.verification_type, a.requested_label, a.evidence_url, a.statement, a.status, a.review_note, a.reviewed_by, a.reviewed_at, a.created_at, a.updated_at FROM verification_applications a JOIN users u ON u.id = a.user_id WHERE a.id = ?`, id)
	return scanVerificationApplication(row)
}

func (s *Store) ListVerificationApplications(userID int64, status string) ([]domain.VerificationApplication, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	where := "1 = 1"
	args := make([]any, 0, 2)
	if userID > 0 {
		where += " AND a.user_id = ?"
		args = append(args, userID)
	}
	if status != "" {
		if status != "pending" && status != "approved" && status != "rejected" {
			return nil, errors.New("认证申请状态无效")
		}
		where += " AND a.status = ?"
		args = append(args, status)
	}
	rows, err := s.db.Query(`SELECT a.id, a.user_id, u.display_name, u.email, a.verification_type, a.requested_label, a.evidence_url, a.statement, a.status, a.review_note, a.reviewed_by, a.reviewed_at, a.created_at, a.updated_at FROM verification_applications a JOIN users u ON u.id = a.user_id WHERE `+where+` ORDER BY a.created_at DESC LIMIT 100`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.VerificationApplication, 0)
	for rows.Next() {
		item, err := scanVerificationApplication(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) ReviewVerificationApplication(actorID, applicationID int64, status, label, note string) (domain.VerificationApplication, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	label = strings.TrimSpace(label)
	note = strings.TrimSpace(note)
	if status != "approved" && status != "rejected" {
		return domain.VerificationApplication{}, errors.New("认证审核状态无效")
	}
	if len([]rune(label)) > 80 || len([]rune(note)) > 500 || containsControl(label) || containsControl(note) {
		return domain.VerificationApplication{}, errors.New("认证名称或审核说明过长")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.VerificationApplication{}, err
	}
	defer tx.Rollback()
	var userID int64
	var currentStatus, requestedLabel string
	if err := tx.QueryRow(`SELECT user_id, status, requested_label FROM verification_applications WHERE id = ? FOR UPDATE`, applicationID).Scan(&userID, &currentStatus, &requestedLabel); err != nil {
		return domain.VerificationApplication{}, err
	}
	var userStatus string
	if err := tx.QueryRow(`SELECT status FROM users WHERE id = ? FOR UPDATE`, userID).Scan(&userStatus); err != nil {
		return domain.VerificationApplication{}, err
	}
	if status == "approved" && userStatus != "active" {
		return domain.VerificationApplication{}, errors.New("认证用户不存在或已停用")
	}
	revokingApproved := currentStatus == "approved" && status == "rejected"
	if currentStatus != "pending" && !revokingApproved {
		return domain.VerificationApplication{}, errors.New("只有待审核申请可以处理，已通过申请只能撤销认证")
	}
	if revokingApproved && note == "" {
		return domain.VerificationApplication{}, errors.New("撤销认证时必须填写原因")
	}
	if status == "approved" {
		if label == "" {
			label = requestedLabel
		}
		if len([]rune(label)) < 2 {
			return domain.VerificationApplication{}, errors.New("认证名称无效")
		}
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`UPDATE verification_applications SET status = ?, requested_label = ?, review_note = ?, reviewed_by = ?, reviewed_at = ?, updated_at = ? WHERE id = ?`, status, labelOrRequested(label, requestedLabel), note, actorID, now, now, applicationID); err != nil {
		return domain.VerificationApplication{}, err
	}
	if status == "approved" {
		if _, err := tx.Exec(`UPDATE users SET blue_verified = 1, verification_label = ?, updated_at = ? WHERE id = ?`, label, now, userID); err != nil {
			return domain.VerificationApplication{}, err
		}
	} else if revokingApproved {
		var replacementLabel string
		err := tx.QueryRow(`SELECT requested_label FROM verification_applications WHERE user_id = ? AND status = 'approved' AND id <> ? ORDER BY reviewed_at DESC, id DESC LIMIT 1`, userID, applicationID).Scan(&replacementLabel)
		if errors.Is(err, sql.ErrNoRows) {
			if _, err := tx.Exec(`UPDATE users SET blue_verified = 0, verification_label = '', updated_at = ? WHERE id = ?`, now, userID); err != nil {
				return domain.VerificationApplication{}, err
			}
		} else if err != nil {
			return domain.VerificationApplication{}, err
		} else {
			if _, err := tx.Exec(`UPDATE users SET blue_verified = 1, verification_label = ?, updated_at = ? WHERE id = ?`, replacementLabel, now, userID); err != nil {
				return domain.VerificationApplication{}, err
			}
		}
	}
	action := status
	if revokingApproved {
		action = "revoked"
	}
	if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'verification', ?, ?, ?, ?)`, actorID, applicationID, action, note, now); err != nil {
		return domain.VerificationApplication{}, err
	}
	item, err := getVerificationApplication(tx, applicationID)
	if err != nil {
		return domain.VerificationApplication{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.VerificationApplication{}, err
	}
	return item, nil
}

func labelOrRequested(label, requested string) string {
	if label != "" {
		return label
	}
	return requested
}

func scanVerificationApplication(scanner rowScanner) (domain.VerificationApplication, error) {
	var item domain.VerificationApplication
	var reviewedBy sql.NullInt64
	var reviewedAt sql.NullTime
	if err := scanner.Scan(&item.ID, &item.UserID, &item.UserName, &item.UserEmail, &item.VerificationType, &item.RequestedLabel, &item.EvidenceURL, &item.Statement, &item.Status, &item.ReviewNote, &reviewedBy, &reviewedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return item, err
	}
	if reviewedBy.Valid {
		item.ReviewedBy = &reviewedBy.Int64
	}
	if reviewedAt.Valid {
		item.ReviewedAt = &reviewedAt.Time
	}
	return item, nil
}
