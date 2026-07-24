package store

import (
	"database/sql"
	"errors"
	"time"

	"roblox-community/internal/domain"
)

func (s *Store) GetFollowStatus(viewerID, targetID int64) (domain.FollowStatus, error) {
	if targetID <= 0 {
		return domain.FollowStatus{}, errors.New("invalid user")
	}
	var active int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id = ? AND status = 'active'`, targetID).Scan(&active); err != nil {
		return domain.FollowStatus{}, err
	}
	if active != 1 {
		return domain.FollowStatus{}, sql.ErrNoRows
	}
	var result domain.FollowStatus
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM user_follows WHERE followed_id = ?`, targetID).Scan(&result.FollowerCount); err != nil {
		return result, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM user_follows WHERE follower_id = ?`, targetID).Scan(&result.FollowingCount); err != nil {
		return result, err
	}
	if viewerID > 0 && viewerID != targetID {
		var following int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM user_follows WHERE follower_id = ? AND followed_id = ?`, viewerID, targetID).Scan(&following); err != nil {
			return result, err
		}
		result.Following = following != 0
	}
	return result, nil
}

func (s *Store) SetUserFollow(followerID, followedID int64, following bool) (domain.FollowStatus, error) {
	if followerID <= 0 || followedID <= 0 || followerID == followedID {
		return domain.FollowStatus{}, errors.New("cannot follow this user")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.FollowStatus{}, err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, []int64{followerID, followedID}); err != nil {
		return domain.FollowStatus{}, err
	}
	blocked, err := blockedEitherWayQuery(tx, followerID, followedID)
	if err != nil {
		return domain.FollowStatus{}, err
	}
	if blocked {
		return domain.FollowStatus{}, errors.New("user interaction is blocked")
	}
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM user_follows WHERE follower_id = ? AND followed_id = ?`, followerID, followedID).Scan(&exists); err != nil {
		return domain.FollowStatus{}, err
	}
	now := time.Now().UTC()
	if following && exists == 0 {
		if _, err := tx.Exec(`INSERT INTO user_follows (follower_id, followed_id, created_at) VALUES (?, ?, ?)`, followerID, followedID, now); err != nil {
			return domain.FollowStatus{}, err
		}
		if err := createNotification(tx, followedID, followerID, "follow", nil, nil); err != nil {
			return domain.FollowStatus{}, err
		}
	} else if !following && exists != 0 {
		if _, err := tx.Exec(`DELETE FROM user_follows WHERE follower_id = ? AND followed_id = ?`, followerID, followedID); err != nil {
			return domain.FollowStatus{}, err
		}
	}
	result, err := followStatusTx(tx, followerID, followedID)
	if err != nil {
		return domain.FollowStatus{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.FollowStatus{}, err
	}
	return result, nil
}

func followStatusTx(tx *sql.Tx, viewerID, targetID int64) (domain.FollowStatus, error) {
	var result domain.FollowStatus
	if err := tx.QueryRow(`SELECT COUNT(*) FROM user_follows WHERE followed_id = ?`, targetID).Scan(&result.FollowerCount); err != nil {
		return result, err
	}
	if err := tx.QueryRow(`SELECT COUNT(*) FROM user_follows WHERE follower_id = ?`, targetID).Scan(&result.FollowingCount); err != nil {
		return result, err
	}
	var following int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM user_follows WHERE follower_id = ? AND followed_id = ?`, viewerID, targetID).Scan(&following); err != nil {
		return result, err
	}
	result.Following = following != 0
	return result, nil
}

func createNotification(tx *sql.Tx, userID, actorID int64, kind string, postID, commentID any) error {
	if userID <= 0 || actorID <= 0 || userID == actorID {
		return nil
	}
	_, err := tx.Exec(`INSERT INTO notifications (user_id, actor_id, kind, post_id, comment_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`, userID, actorID, kind, postID, commentID, time.Now().UTC())
	return err
}

func createConversationNotification(tx *sql.Tx, userID, actorID int64, kind string, conversationID int64, createdAt time.Time) error {
	if userID <= 0 || actorID <= 0 || conversationID <= 0 || userID == actorID {
		return nil
	}
	_, err := tx.Exec(`INSERT INTO notifications (user_id, actor_id, kind, conversation_id, created_at) VALUES (?, ?, ?, ?, ?)`, userID, actorID, kind, conversationID, createdAt)
	return err
}

func (s *Store) TogglePostBookmark(userID, postID int64) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, []int64{userID}); err != nil {
		return false, err
	}
	if ok, err := lockLikeTarget(tx, "post", postID); err != nil || !ok {
		if err != nil {
			return false, err
		}
		return false, errors.New("post not found")
	}
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM post_bookmarks WHERE post_id = ? AND user_id = ?`, postID, userID).Scan(&exists); err != nil {
		return false, err
	}
	bookmarked := exists == 0
	if bookmarked {
		_, err = tx.Exec(`INSERT INTO post_bookmarks (post_id, user_id, created_at) VALUES (?, ?, ?)`, postID, userID, time.Now().UTC())
	} else {
		_, err = tx.Exec(`DELETE FROM post_bookmarks WHERE post_id = ? AND user_id = ?`, postID, userID)
	}
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return bookmarked, nil
}

func (s *Store) BookmarkStatus(userID, postID int64) (bool, error) {
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM post_bookmarks pb JOIN posts p ON p.id = pb.post_id WHERE pb.post_id = ? AND pb.user_id = ? AND p.status = 'published'`, postID, userID).Scan(&exists); err != nil {
		return false, err
	}
	return exists != 0, nil
}

func (s *Store) TogglePostRepost(userID, postID int64) (bool, int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, []int64{userID}); err != nil {
		return false, 0, err
	}
	if ok, err := lockLikeTarget(tx, "post", postID); err != nil || !ok {
		if err != nil {
			return false, 0, err
		}
		return false, 0, errors.New("post not found")
	}
	var authorID int64
	if err := tx.QueryRow(`SELECT author_id FROM posts WHERE id = ? FOR UPDATE`, postID).Scan(&authorID); err != nil {
		return false, 0, err
	}
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM post_reposts WHERE post_id = ? AND user_id = ?`, postID, userID).Scan(&exists); err != nil {
		return false, 0, err
	}
	reposted := exists == 0
	if reposted {
		if _, err := tx.Exec(`INSERT INTO post_reposts (post_id, user_id, created_at) VALUES (?, ?, ?)`, postID, userID, time.Now().UTC()); err != nil {
			return false, 0, err
		}
		if err := createNotification(tx, authorID, userID, "repost", postID, nil); err != nil {
			return false, 0, err
		}
	} else if _, err := tx.Exec(`DELETE FROM post_reposts WHERE post_id = ? AND user_id = ?`, postID, userID); err != nil {
		return false, 0, err
	}
	var count int64
	if err := tx.QueryRow(`SELECT COUNT(*) FROM post_reposts WHERE post_id = ?`, postID).Scan(&count); err != nil {
		return false, 0, err
	}
	if err := tx.Commit(); err != nil {
		return false, 0, err
	}
	return reposted, count, nil
}

func (s *Store) RepostStatus(userID, postID int64) (bool, int64, error) {
	var reposted, count int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM post_reposts WHERE post_id = ? AND user_id = ?`, postID, userID).Scan(&reposted); err != nil {
		return false, 0, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM post_reposts WHERE post_id = ?`, postID).Scan(&count); err != nil {
		return false, 0, err
	}
	return reposted != 0, count, nil
}

func (s *Store) ListFollowingPosts(userID int64, limit int) ([]domain.Post, error) {
	page, err := s.ListFollowingPostsPage(userID, limit, 0)
	return page.Items, err
}

func (s *Store) ListFollowingPostsPage(userID int64, limit, offset int) (domain.PostPage, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	if offset < 0 || offset > 100000 {
		offset = 0
	}
	rows, err := s.db.Query(`SELECT p.id, p.board_id, b.name, p.author_id, u.display_name, u.avatar_url, u.blue_verified, u.verification_label, COALESCE(u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP(), 0), CASE WHEN u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP() THEN u.membership_tier_id ELSE 0 END, p.title, p.content, p.post_type, p.status, p.pinned, p.featured, p.views, p.comment_count, (SELECT COUNT(*) FROM post_likes pl WHERE pl.post_id = p.id), (SELECT COUNT(*) FROM post_reposts pr WHERE pr.post_id = p.id), p.created_at, p.updated_at FROM posts p JOIN boards b ON b.id = p.board_id JOIN users u ON u.id = p.author_id WHERE p.status = 'published' AND b.status = 'active' AND u.status = 'active' AND (p.author_id = ? OR EXISTS (SELECT 1 FROM user_follows uf WHERE uf.follower_id = ? AND uf.followed_id = p.author_id)) AND NOT EXISTS (SELECT 1 FROM user_blocks ub WHERE (ub.blocker_id = ? AND ub.blocked_id = p.author_id) OR (ub.blocker_id = p.author_id AND ub.blocked_id = ?)) ORDER BY p.pinned DESC, p.updated_at DESC LIMIT ? OFFSET ?`, userID, userID, userID, userID, limit+1, offset)
	if err != nil {
		return domain.PostPage{}, err
	}
	defer rows.Close()
	result := make([]domain.Post, 0, limit+1)
	for rows.Next() {
		var item domain.Post
		var authorVerified, authorMember, pinned, featured int
		if err := rows.Scan(&item.ID, &item.BoardID, &item.BoardName, &item.AuthorID, &item.AuthorName, &item.AuthorAvatar, &authorVerified, &item.AuthorVerificationLabel, &authorMember, &item.AuthorMembershipTierID, &item.Title, &item.Content, &item.PostType, &item.Status, &pinned, &featured, &item.Views, &item.CommentCount, &item.LikeCount, &item.RepostCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return domain.PostPage{}, err
		}
		item.AuthorVerified = authorVerified != 0
		item.AuthorMember = authorMember != 0
		item.Pinned = pinned != 0
		item.Featured = featured != 0
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return domain.PostPage{}, err
	}
	hasMore := len(result) > limit
	if hasMore {
		result = result[:limit]
	}
	if err := attachPostMedia(s.db, result); err != nil {
		return domain.PostPage{}, err
	}
	if err := s.MarkPostsLikedBy(userID, result); err != nil {
		return domain.PostPage{}, err
	}
	return domain.PostPage{Items: result, NextOffset: offset + len(result), HasMore: hasMore}, nil
}

func (s *Store) ListBookmarkedPosts(userID int64, limit int) ([]domain.Post, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	rows, err := s.db.Query(`SELECT p.id, p.board_id, b.name, p.author_id, u.display_name, u.avatar_url, u.blue_verified, u.verification_label, COALESCE(u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP(), 0), CASE WHEN u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP() THEN u.membership_tier_id ELSE 0 END, p.title, p.content, p.post_type, p.status, p.pinned, p.featured, p.views, p.comment_count, (SELECT COUNT(*) FROM post_likes pl WHERE pl.post_id = p.id), (SELECT COUNT(*) FROM post_reposts pr WHERE pr.post_id = p.id), p.created_at, p.updated_at FROM post_bookmarks pb JOIN posts p ON p.id = pb.post_id JOIN boards b ON b.id = p.board_id JOIN users u ON u.id = p.author_id WHERE pb.user_id = ? AND p.status = 'published' AND b.status = 'active' AND u.status = 'active' ORDER BY pb.created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.Post, 0)
	for rows.Next() {
		var item domain.Post
		var authorVerified, authorMember, pinned, featured int
		if err := rows.Scan(&item.ID, &item.BoardID, &item.BoardName, &item.AuthorID, &item.AuthorName, &item.AuthorAvatar, &authorVerified, &item.AuthorVerificationLabel, &authorMember, &item.AuthorMembershipTierID, &item.Title, &item.Content, &item.PostType, &item.Status, &pinned, &featured, &item.Views, &item.CommentCount, &item.LikeCount, &item.RepostCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.AuthorVerified = authorVerified != 0
		item.AuthorMember = authorMember != 0
		item.Pinned = pinned != 0
		item.Featured = featured != 0
		item.Bookmarked = true
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

func (s *Store) ListNotifications(userID int64, limit int) ([]domain.Notification, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT n.id, n.kind, n.actor_id, u.display_name, u.avatar_url, n.post_id, n.comment_id, n.conversation_id, COALESCE(p.title, ''), COALESCE(c.name, ''), n.read_at, n.created_at FROM notifications n JOIN users u ON u.id = n.actor_id LEFT JOIN posts p ON p.id = n.post_id LEFT JOIN conversations c ON c.id = n.conversation_id WHERE n.user_id = ? AND u.status = 'active' ORDER BY n.created_at DESC, n.id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.Notification, 0)
	for rows.Next() {
		var item domain.Notification
		var postID, commentID, conversationID sql.NullInt64
		var readAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Kind, &item.ActorID, &item.ActorName, &item.ActorAvatar, &postID, &commentID, &conversationID, &item.PostTitle, &item.ConversationName, &readAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		if postID.Valid {
			item.PostID = &postID.Int64
		}
		if commentID.Valid {
			item.CommentID = &commentID.Int64
		}
		if conversationID.Valid {
			item.ConversationID = &conversationID.Int64
		}
		item.Read = readAt.Valid
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) MarkNotificationsRead(userID int64) error {
	_, err := s.db.Exec(`UPDATE notifications SET read_at = COALESCE(read_at, ?) WHERE user_id = ?`, time.Now().UTC(), userID)
	return err
}

func (s *Store) UnreadNotificationCount(userID int64) (int64, error) {
	var count int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id = ? AND read_at IS NULL`, userID).Scan(&count)
	return count, err
}
