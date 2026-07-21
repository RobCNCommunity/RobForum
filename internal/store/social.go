package store

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

func (s *Store) SearchUsers(query string, limit int, hotOnly bool) ([]domain.UserSearchResult, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) > 80 {
		return nil, errors.New("search query is too long")
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	where := `u.status = 'active'`
	args := make([]any, 0, 3)
	if query != "" {
		where += ` AND (u.display_name LIKE ? ESCAPE '\\' OR u.roblox_name LIKE ? ESCAPE '\\')`
		pattern := "%" + escapeLike(query) + "%"
		args = append(args, pattern, pattern)
	}
	order := `hot_score DESC, u.created_at DESC`
	if !hotOnly && query != "" {
		order = `CASE WHEN u.display_name = ? THEN 0 ELSE 1 END, hot_score DESC, u.created_at DESC`
		args = append(args, query)
	}
	args = append(args, limit)
	rows, err := s.db.Query(`SELECT
		u.id, u.display_name, u.avatar_url, u.cover_url, u.bio, u.blue_verified, u.verification_label,
		u.roblox_name, u.roblox_verified, u.created_at,
		(SELECT COUNT(*) FROM posts p JOIN boards pb ON pb.id = p.board_id WHERE p.author_id = u.id AND p.status = 'published' AND pb.status = 'active') AS post_count,
		(SELECT COUNT(*) FROM resources r WHERE r.creator_id = u.id AND r.status = 'approved') AS resource_count,
		((SELECT COUNT(*) FROM posts p JOIN boards pb ON pb.id = p.board_id WHERE p.author_id = u.id AND p.status = 'published' AND pb.status = 'active') * 5 +
		 (SELECT COUNT(*) FROM resources r WHERE r.creator_id = u.id AND r.status = 'approved') * 8 +
		 (SELECT COUNT(*) FROM comments c JOIN posts cp ON cp.id = c.post_id JOIN boards cb ON cb.id = cp.board_id WHERE c.author_id = u.id AND c.status = 'published' AND cp.status = 'published' AND cb.status = 'active') +
		 (SELECT COUNT(*) FROM post_likes pl JOIN posts p2 ON p2.id = pl.post_id JOIN boards pb2 ON pb2.id = p2.board_id WHERE p2.author_id = u.id AND p2.status = 'published' AND pb2.status = 'active') * 2 +
		 (SELECT COUNT(*) FROM comment_likes cl JOIN comments c2 ON c2.id = cl.comment_id JOIN posts cp2 ON cp2.id = c2.post_id JOIN boards cb2 ON cb2.id = cp2.board_id WHERE c2.author_id = u.id AND c2.status = 'published' AND cp2.status = 'published' AND cb2.status = 'active')) AS hot_score
		FROM users u WHERE `+where+` ORDER BY `+order+` LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.UserSearchResult, 0)
	for rows.Next() {
		var item domain.UserSearchResult
		var blue, roblox int
		if err := rows.Scan(&item.ID, &item.DisplayName, &item.AvatarURL, &item.CoverURL, &item.Bio, &blue, &item.VerificationLabel, &item.RobloxName, &roblox, &item.CreatedAt, &item.PostCount, &item.ResourceCount, &item.HotScore); err != nil {
			return nil, err
		}
		item.BlueVerified = blue != 0
		item.RobloxVerified = roblox != 0
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) SetUserBlocked(blockerID, blockedID int64, blocked bool) error {
	if blockerID <= 0 || blockedID <= 0 || blockerID == blockedID {
		return errors.New("不能拉黑自己")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if blocked {
		if err := lockActiveUsers(tx, []int64{blockerID, blockedID}); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO user_blocks (blocker_id, blocked_id, created_at) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE created_at = created_at`, blockerID, blockedID, time.Now().UTC()); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM user_follows WHERE (follower_id = ? AND followed_id = ?) OR (follower_id = ? AND followed_id = ?)`, blockerID, blockedID, blockedID, blockerID); err != nil {
			return err
		}
	} else {
		if err := lockActiveUsers(tx, []int64{blockerID}); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM user_blocks WHERE blocker_id = ? AND blocked_id = ?`, blockerID, blockedID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) IsUserBlocked(blockerID, blockedID int64) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM user_blocks WHERE blocker_id = ? AND blocked_id = ?`, blockerID, blockedID).Scan(&count)
	return count > 0, err
}

func (s *Store) blockedEitherWay(userA, userB int64) (bool, error) {
	return blockedEitherWayQuery(s.db, userA, userB)
}

func blockedEitherWayQuery(queryer rowQueryer, userA, userB int64) (bool, error) {
	var count int
	err := queryer.QueryRow(`SELECT COUNT(*) FROM user_blocks WHERE (blocker_id = ? AND blocked_id = ?) OR (blocker_id = ? AND blocked_id = ?)`, userA, userB, userB, userA).Scan(&count)
	return count > 0, err
}

func lockActiveUsers(tx *sql.Tx, userIDs []int64) error {
	unique := make(map[int64]struct{}, len(userIDs))
	ordered := make([]int64, 0, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 {
			return errors.New("用户不存在或已停用")
		}
		if _, exists := unique[userID]; exists {
			continue
		}
		unique[userID] = struct{}{}
		ordered = append(ordered, userID)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	for _, userID := range ordered {
		var status string
		if err := tx.QueryRow(`SELECT status FROM users WHERE id = ? FOR UPDATE`, userID).Scan(&status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errors.New("用户不存在或已停用")
			}
			return err
		}
		if status != "active" {
			return errors.New("用户不存在或已停用")
		}
	}
	return nil
}

func lockConversationMembers(tx *sql.Tx, userID, conversationID int64) (string, []int64, error) {
	var kind string
	if err := tx.QueryRow(`SELECT kind FROM conversations WHERE id = ? FOR UPDATE`, conversationID).Scan(&kind); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil, errors.New("会话不存在或无权访问")
		}
		return "", nil, err
	}
	rows, err := tx.Query(`SELECT user_id FROM conversation_members WHERE conversation_id = ? ORDER BY user_id FOR UPDATE`, conversationID)
	if err != nil {
		return "", nil, err
	}
	members := make([]int64, 0)
	allowed := false
	for rows.Next() {
		var memberID int64
		if err := rows.Scan(&memberID); err != nil {
			rows.Close()
			return "", nil, err
		}
		members = append(members, memberID)
		allowed = allowed || memberID == userID
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", nil, err
	}
	if err := rows.Close(); err != nil {
		return "", nil, err
	}
	if !allowed {
		return "", nil, errors.New("会话不存在或无权访问")
	}
	if kind != "direct" && kind != "group" {
		return "", nil, errors.New("会话类型无效")
	}
	return kind, members, nil
}

func (s *Store) DeleteComment(actorID int64, admin bool, commentID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var authorID, postID int64
	var status string
	if err := tx.QueryRow(`SELECT author_id, post_id, status FROM comments WHERE id = ? FOR UPDATE`, commentID).Scan(&authorID, &postID, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("评论不存在")
		}
		return err
	}
	if status != "published" {
		return errors.New("评论已删除")
	}
	if !admin && actorID != authorID {
		return errors.New("只能删除自己的评论")
	}
	if _, err := tx.Exec(`UPDATE comments SET status = 'deleted' WHERE id = ?`, commentID); err != nil {
		return err
	}
	postUpdate, err := tx.Exec(`UPDATE posts SET comment_count = GREATEST(comment_count - 1, 0), updated_at = ? WHERE id = ?`, time.Now().UTC(), postID)
	if err != nil {
		return err
	}
	if affected, err := postUpdate.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return err
		}
		return errors.New("post comment count update failed")
	}
	if admin && actorID != authorID {
		if _, err := tx.Exec(`INSERT INTO moderation_actions (actor_id, target_type, target_id, action, reason, created_at) VALUES (?, 'comment', ?, 'delete', '', ?)`, actorID, commentID, time.Now().UTC()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) TogglePostLike(userID, postID int64) (bool, int64, error) {
	return s.toggleLike("post", userID, postID)
}

func (s *Store) ToggleCommentLike(userID, commentID int64) (bool, int64, error) {
	return s.toggleLike("comment", userID, commentID)
}

func (s *Store) LikeStatus(kind string, userID, targetID int64) (bool, int64, error) {
	table, targetColumn, _ := likeTables(kind)
	if table == "" {
		return false, 0, errors.New("点赞类型无效")
	}
	exists, err := likeTargetExists(s.db, kind, targetID)
	if err != nil || !exists {
		return false, 0, errors.New("内容不存在")
	}
	var liked, count int64
	if err := s.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s = ? AND user_id = ?`, table, targetColumn), targetID, userID).Scan(&liked); err != nil {
		return false, 0, err
	}
	if err := s.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s = ?`, table, targetColumn), targetID).Scan(&count); err != nil {
		return false, 0, err
	}
	return liked > 0, count, nil
}

func (s *Store) toggleLike(kind string, userID, targetID int64) (bool, int64, error) {
	table, targetColumn, _ := likeTables(kind)
	if table == "" {
		return false, 0, errors.New("点赞类型无效")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, []int64{userID}); err != nil {
		return false, 0, err
	}
	targetExists, err := lockLikeTarget(tx, kind, targetID)
	if err != nil {
		return false, 0, err
	}
	if !targetExists {
		return false, 0, errors.New("内容不存在")
	}
	ownerID, postID, err := likeTargetOwner(tx, kind, targetID)
	if err != nil {
		return false, 0, err
	}
	var likeExists int
	if err := tx.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s = ? AND user_id = ?`, table, targetColumn), targetID, userID).Scan(&likeExists); err != nil {
		return false, 0, err
	}
	liked := likeExists == 0
	if liked {
		if _, err := tx.Exec(fmt.Sprintf(`INSERT INTO %s (%s, user_id, created_at) VALUES (?, ?, ?)`, table, targetColumn), targetID, userID, time.Now().UTC()); err != nil {
			return false, 0, err
		}
		if kind == "post" {
			if err := createNotification(tx, ownerID, userID, "like", postID, nil); err != nil {
				return false, 0, err
			}
		} else if err := createNotification(tx, ownerID, userID, "comment_like", postID, targetID); err != nil {
			return false, 0, err
		}
	} else if _, err := tx.Exec(fmt.Sprintf(`DELETE FROM %s WHERE %s = ? AND user_id = ?`, table, targetColumn), targetID, userID); err != nil {
		return false, 0, err
	}
	var count int64
	if err := tx.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s = ?`, table, targetColumn), targetID).Scan(&count); err != nil {
		return false, 0, err
	}
	if err := tx.Commit(); err != nil {
		return false, 0, err
	}
	return liked, count, nil
}

func likeTargetOwner(tx *sql.Tx, kind string, targetID int64) (int64, int64, error) {
	var ownerID, postID int64
	var query string
	switch kind {
	case "post":
		query = `SELECT p.author_id, p.id FROM posts p JOIN boards b ON b.id = p.board_id JOIN users u ON u.id = p.author_id WHERE p.id = ? AND p.status = 'published' AND b.status = 'active' AND u.status = 'active' FOR UPDATE`
	case "comment":
		query = `SELECT c.author_id, c.post_id FROM comments c JOIN posts p ON p.id = c.post_id JOIN boards b ON b.id = p.board_id JOIN users u ON u.id = c.author_id JOIN users pu ON pu.id = p.author_id WHERE c.id = ? AND c.status = 'published' AND p.status = 'published' AND b.status = 'active' AND u.status = 'active' AND pu.status = 'active' FOR UPDATE`
	default:
		return 0, 0, errors.New("invalid like target")
	}
	if err := tx.QueryRow(query, targetID).Scan(&ownerID, &postID); err != nil {
		return 0, 0, err
	}
	return ownerID, postID, nil
}

func likeTargetExists(queryer rowQueryer, kind string, targetID int64) (bool, error) {
	var count int
	var query string
	switch kind {
	case "post":
		query = `SELECT COUNT(*) FROM posts p JOIN boards b ON b.id = p.board_id JOIN users u ON u.id = p.author_id WHERE p.id = ? AND p.status = 'published' AND b.status = 'active' AND u.status = 'active'`
	case "comment":
		query = `SELECT COUNT(*) FROM comments c JOIN posts p ON p.id = c.post_id JOIN boards b ON b.id = p.board_id JOIN users u ON u.id = c.author_id JOIN users pu ON pu.id = p.author_id WHERE c.id = ? AND c.status = 'published' AND p.status = 'published' AND b.status = 'active' AND u.status = 'active' AND pu.status = 'active'`
	default:
		return false, errors.New("invalid like target")
	}
	if err := queryer.QueryRow(query, targetID).Scan(&count); err != nil {
		return false, err
	}
	return count == 1, nil
}

func lockLikeTarget(tx *sql.Tx, kind string, targetID int64) (bool, error) {
	var id int64
	var query string
	switch kind {
	case "post":
		query = `SELECT p.id FROM posts p JOIN boards b ON b.id = p.board_id JOIN users u ON u.id = p.author_id WHERE p.id = ? AND p.status = 'published' AND b.status = 'active' AND u.status = 'active' FOR UPDATE`
	case "comment":
		query = `SELECT c.id FROM comments c JOIN posts p ON p.id = c.post_id JOIN boards b ON b.id = p.board_id JOIN users u ON u.id = c.author_id JOIN users pu ON pu.id = p.author_id WHERE c.id = ? AND c.status = 'published' AND p.status = 'published' AND b.status = 'active' AND u.status = 'active' AND pu.status = 'active' FOR UPDATE`
	default:
		return false, errors.New("invalid like target")
	}
	if err := tx.QueryRow(query, targetID).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func likeTables(kind string) (string, string, string) {
	switch kind {
	case "post":
		return "post_likes", "post_id", "posts"
	case "comment":
		return "comment_likes", "comment_id", "comments"
	default:
		return "", "", ""
	}
}

func (s *Store) CreateConversation(creatorID int64, kind, name string, memberIDs []int64) (domain.Conversation, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	name = strings.TrimSpace(name)
	if kind != "direct" && kind != "group" {
		return domain.Conversation{}, errors.New("会话类型无效")
	}
	seen := map[int64]bool{creatorID: true}
	members := []int64{creatorID}
	for _, id := range memberIDs {
		if id > 0 && !seen[id] {
			seen[id] = true
			members = append(members, id)
		}
	}
	if (kind == "direct" && len(members) != 2) || (kind == "group" && (len(members) < 3 || len(members) > 50)) {
		return domain.Conversation{}, errors.New("私信需要 2 人，群聊需要 3 到 50 人")
	}
	if kind == "group" && (name == "" || len([]rune(name)) > 120 || containsControl(name)) {
		return domain.Conversation{}, errors.New("群聊名称无效")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Conversation{}, err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, members); err != nil {
		return domain.Conversation{}, err
	}
	for _, id := range members {
		if id == creatorID {
			continue
		}
		blocked, err := blockedEitherWayQuery(tx, creatorID, id)
		if err != nil {
			return domain.Conversation{}, err
		}
		if blocked {
			return domain.Conversation{}, errors.New("无法与已拉黑或拉黑你的用户创建会话")
		}
	}
	if kind == "direct" {
		var existing int64
		err := tx.QueryRow(`SELECT c.id FROM conversations c JOIN conversation_members cm ON cm.conversation_id = c.id WHERE c.kind = 'direct' GROUP BY c.id HAVING COUNT(*) = 2 AND SUM(cm.user_id IN (?, ?)) = 2 LIMIT 1`, members[0], members[1]).Scan(&existing)
		if err == nil {
			item, err := getConversation(tx, creatorID, existing)
			if err != nil {
				return domain.Conversation{}, err
			}
			if err := tx.Rollback(); err != nil {
				return domain.Conversation{}, err
			}
			return item, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return domain.Conversation{}, err
		}
	}
	now := time.Now().UTC()
	res, err := tx.Exec(`INSERT INTO conversations (kind, name, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, kind, name, creatorID, now, now)
	if err != nil {
		return domain.Conversation{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.Conversation{}, err
	}
	for _, memberID := range members {
		if _, err := tx.Exec(`INSERT INTO conversation_members (conversation_id, user_id, joined_at, last_read_at) VALUES (?, ?, ?, ?)`, id, memberID, now, now); err != nil {
			return domain.Conversation{}, err
		}
	}
	item, err := getConversation(tx, creatorID, id)
	if err != nil {
		return domain.Conversation{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Conversation{}, err
	}
	return item, nil
}

func (s *Store) ListConversations(userID int64) ([]domain.Conversation, error) {
	rows, err := s.db.Query(`SELECT c.id FROM conversations c JOIN conversation_members cm ON cm.conversation_id = c.id WHERE cm.user_id = ? ORDER BY c.updated_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	result := make([]domain.Conversation, 0, len(ids))
	for _, id := range ids {
		item, err := s.GetConversation(userID, id)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) GetConversation(userID, conversationID int64) (domain.Conversation, error) {
	return getConversation(s.db, userID, conversationID)
}

func getConversation(queryer sqlQueryer, userID, conversationID int64) (domain.Conversation, error) {
	var item domain.Conversation
	var lastRead sql.NullTime
	err := queryer.QueryRow(`SELECT c.id, c.kind, c.name, c.created_by, c.created_at, c.updated_at, cm.last_read_at FROM conversations c JOIN conversation_members cm ON cm.conversation_id = c.id WHERE c.id = ? AND cm.user_id = ?`, conversationID, userID).Scan(&item.ID, &item.Kind, &item.Name, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt, &lastRead)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return item, errors.New("会话不存在或无权访问")
		}
		return item, err
	}
	rows, err := queryer.Query(`SELECT u.id, u.display_name, u.avatar_url, u.cover_url, u.bio, u.blue_verified, u.verification_label, u.roblox_name, u.roblox_verified, u.created_at FROM conversation_members cm JOIN users u ON u.id = cm.user_id WHERE cm.conversation_id = ? ORDER BY cm.joined_at`, conversationID)
	if err != nil {
		return item, err
	}
	defer rows.Close()
	item.Members = make([]domain.PublicUser, 0)
	for rows.Next() {
		var member domain.PublicUser
		var blue, roblox int
		if err := rows.Scan(&member.ID, &member.DisplayName, &member.AvatarURL, &member.CoverURL, &member.Bio, &blue, &member.VerificationLabel, &member.RobloxName, &roblox, &member.CreatedAt); err != nil {
			return item, err
		}
		member.BlueVerified = blue != 0
		member.RobloxVerified = roblox != 0
		item.Members = append(item.Members, member)
	}
	if err := rows.Err(); err != nil {
		return item, err
	}
	if err := rows.Close(); err != nil {
		return item, err
	}
	var last domain.Message
	err = queryer.QueryRow(`SELECT m.id, m.conversation_id, m.sender_id, u.display_name, u.avatar_url, m.content, m.created_at FROM messages m JOIN users u ON u.id = m.sender_id WHERE m.conversation_id = ? AND m.deleted_at IS NULL ORDER BY m.id DESC LIMIT 1`, conversationID).Scan(&last.ID, &last.ConversationID, &last.SenderID, &last.SenderName, &last.SenderAvatar, &last.Content, &last.CreatedAt)
	if err == nil {
		item.LastMessage = &last
	} else if !errors.Is(err, sql.ErrNoRows) {
		return item, err
	}
	if lastRead.Valid {
		err = queryer.QueryRow(`SELECT COUNT(*) FROM messages WHERE conversation_id = ? AND sender_id <> ? AND deleted_at IS NULL AND created_at > ?`, conversationID, userID, lastRead.Time).Scan(&item.UnreadCount)
	} else {
		err = queryer.QueryRow(`SELECT COUNT(*) FROM messages WHERE conversation_id = ? AND sender_id <> ? AND deleted_at IS NULL`, conversationID, userID).Scan(&item.UnreadCount)
	}
	if err != nil {
		return item, err
	}
	return item, nil
}

func (s *Store) ListMessages(userID, conversationID int64, limit int) ([]domain.Message, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, _, err := lockConversationMembers(tx, userID, conversationID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(`SELECT m.id, m.conversation_id, m.sender_id, u.display_name, u.avatar_url, m.content, m.created_at FROM messages m JOIN users u ON u.id = m.sender_id WHERE m.conversation_id = ? AND m.deleted_at IS NULL ORDER BY m.id DESC LIMIT ?`, conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reversed := make([]domain.Message, 0)
	for rows.Next() {
		var item domain.Message
		if err := rows.Scan(&item.ID, &item.ConversationID, &item.SenderID, &item.SenderName, &item.SenderAvatar, &item.Content, &item.CreatedAt); err != nil {
			return nil, err
		}
		reversed = append(reversed, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	result := make([]domain.Message, len(reversed))
	for i := range reversed {
		result[len(reversed)-1-i] = reversed[i]
	}
	update, err := tx.Exec(`UPDATE conversation_members SET last_read_at = ? WHERE conversation_id = ? AND user_id = ?`, time.Now().UTC(), conversationID, userID)
	if err != nil {
		return nil, err
	}
	if affected, err := update.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("会话成员状态更新失败")
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) SendMessage(userID, conversationID int64, content string) (domain.Message, error) {
	content = strings.TrimSpace(content)
	if content == "" || len([]rune(content)) > 5000 || containsControl(content) {
		return domain.Message{}, errors.New("消息内容无效")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Message{}, err
	}
	defer tx.Rollback()
	kind, members, err := readConversationMembers(tx, userID, conversationID)
	if err != nil {
		return domain.Message{}, err
	}
	if kind == "direct" {
		if err := lockActiveUsers(tx, members); err != nil {
			return domain.Message{}, err
		}
		for _, memberID := range members {
			if memberID == userID {
				continue
			}
			blocked, err := blockedEitherWayQuery(tx, userID, memberID)
			if err != nil {
				return domain.Message{}, err
			}
			if blocked {
				return domain.Message{}, errors.New("会话中存在拉黑关系，无法发送消息")
			}
		}
	} else if err := lockActiveUsers(tx, []int64{userID}); err != nil {
		return domain.Message{}, err
	}
	lockedKind, lockedMembers, err := lockConversationMembers(tx, userID, conversationID)
	if err != nil {
		return domain.Message{}, err
	}
	if lockedKind != kind || !sameInt64s(lockedMembers, members) {
		return domain.Message{}, errors.New("会话成员已发生变化，请重试")
	}
	var senderName, senderAvatar string
	if err := tx.QueryRow(`SELECT display_name, avatar_url FROM users WHERE id = ?`, userID).Scan(&senderName, &senderAvatar); err != nil {
		return domain.Message{}, err
	}
	now := time.Now().UTC()
	res, err := tx.Exec(`INSERT INTO messages (conversation_id, sender_id, content, created_at) VALUES (?, ?, ?, ?)`, conversationID, userID, content, now)
	if err != nil {
		return domain.Message{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.Message{}, err
	}
	conversationUpdate, err := tx.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, now, conversationID)
	if err != nil {
		return domain.Message{}, err
	}
	if affected, err := conversationUpdate.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return domain.Message{}, err
		}
		return domain.Message{}, errors.New("会话状态更新失败")
	}
	memberUpdate, err := tx.Exec(`UPDATE conversation_members SET last_read_at = ? WHERE conversation_id = ? AND user_id = ?`, now, conversationID, userID)
	if err != nil {
		return domain.Message{}, err
	}
	if affected, err := memberUpdate.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return domain.Message{}, err
		}
		return domain.Message{}, errors.New("会话成员状态更新失败")
	}
	if err := tx.Commit(); err != nil {
		return domain.Message{}, err
	}
	return domain.Message{
		ID:             id,
		ConversationID: conversationID,
		SenderID:       userID,
		SenderName:     senderName,
		SenderAvatar:   senderAvatar,
		Content:        content,
		CreatedAt:      now,
	}, nil
}

func readConversationMembers(queryer sqlQueryer, userID, conversationID int64) (string, []int64, error) {
	var kind string
	if err := queryer.QueryRow(`SELECT kind FROM conversations WHERE id = ?`, conversationID).Scan(&kind); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil, errors.New("会话不存在或无权访问")
		}
		return "", nil, err
	}
	rows, err := queryer.Query(`SELECT user_id FROM conversation_members WHERE conversation_id = ? ORDER BY user_id`, conversationID)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	members := make([]int64, 0)
	allowed := false
	for rows.Next() {
		var memberID int64
		if err := rows.Scan(&memberID); err != nil {
			return "", nil, err
		}
		members = append(members, memberID)
		allowed = allowed || memberID == userID
	}
	if err := rows.Err(); err != nil {
		return "", nil, err
	}
	if !allowed || (kind != "direct" && kind != "group") {
		return "", nil, errors.New("会话不存在或无权访问")
	}
	return kind, members, nil
}

func sameInt64s(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
