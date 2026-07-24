package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

const maxConversationMembers = 50

func normalizeConversationMemberIDs(ownerID int64, memberIDs []int64) []int64 {
	seen := map[int64]struct{}{ownerID: {}}
	result := make([]int64, 0, len(memberIDs))
	for _, memberID := range memberIDs {
		if memberID <= 0 {
			continue
		}
		if _, exists := seen[memberID]; exists {
			continue
		}
		seen[memberID] = struct{}{}
		result = append(result, memberID)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func lockOwnedGroup(tx *sql.Tx, ownerID, conversationID int64) error {
	var kind string
	var createdBy int64
	if err := tx.QueryRow(`SELECT kind, created_by FROM conversations WHERE id = ? FOR UPDATE`, conversationID).Scan(&kind, &createdBy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("群聊不存在")
		}
		return err
	}
	if kind != "group" || createdBy != ownerID {
		return errors.New("只有群主可以管理群聊")
	}
	var membershipStatus string
	if err := tx.QueryRow(`SELECT membership_status FROM conversation_members WHERE conversation_id = ? AND user_id = ? FOR UPDATE`, conversationID, ownerID).Scan(&membershipStatus); err != nil {
		return errors.New("群聊不存在或无权访问")
	}
	if membershipStatus != "accepted" {
		return errors.New("群主状态无效")
	}
	return nil
}

func (s *Store) ListConversationMembers(userID, conversationID int64) ([]domain.ConversationMember, error) {
	var kind, membershipStatus string
	if err := s.db.QueryRow(`SELECT c.kind, cm.membership_status FROM conversations c JOIN conversation_members cm ON cm.conversation_id = c.id WHERE c.id = ? AND cm.user_id = ?`, conversationID, userID).Scan(&kind, &membershipStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("群聊不存在或无权访问")
		}
		return nil, err
	}
	if kind != "group" || membershipStatus != "accepted" {
		return nil, errors.New("群聊不存在或无权访问")
	}
	rows, err := s.db.Query(`SELECT u.id, u.display_name, u.avatar_url, u.cover_url, u.bio, u.blue_verified, u.verification_label, COALESCE(u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP(), 0), CASE WHEN u.membership_tier_id IS NOT NULL AND u.membership_expires_at > UTC_TIMESTAMP() THEN u.membership_tier_id ELSE 0 END, u.roblox_name, u.roblox_verified, u.created_at, cm.membership_status, cm.invited_by, cm.joined_at, cm.responded_at FROM conversation_members cm JOIN users u ON u.id = cm.user_id WHERE cm.conversation_id = ? AND cm.membership_status IN ('accepted', 'pending') AND u.status = 'active' ORDER BY cm.membership_status = 'accepted' DESC, cm.joined_at, u.id`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.ConversationMember, 0)
	for rows.Next() {
		var item domain.ConversationMember
		var blueVerified, memberActive, robloxVerified int
		var invitedBy sql.NullInt64
		var respondedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.DisplayName, &item.AvatarURL, &item.CoverURL, &item.Bio, &blueVerified, &item.VerificationLabel, &memberActive, &item.MembershipTierID, &item.RobloxName, &robloxVerified, &item.CreatedAt, &item.MembershipStatus, &invitedBy, &item.JoinedAt, &respondedAt); err != nil {
			return nil, err
		}
		item.BlueVerified = blueVerified != 0
		item.MemberActive = memberActive != 0
		item.RobloxVerified = robloxVerified != 0
		if invitedBy.Valid {
			value := invitedBy.Int64
			item.InvitedBy = &value
		}
		if respondedAt.Valid {
			value := respondedAt.Time
			item.RespondedAt = &value
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) InviteConversationMembers(ownerID, conversationID int64, memberIDs []int64) (domain.Conversation, int64, error) {
	members := normalizeConversationMemberIDs(ownerID, memberIDs)
	if len(members) == 0 || len(members) >= maxConversationMembers {
		return domain.Conversation{}, 0, errors.New("请选择要邀请的用户")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Conversation{}, 0, err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, append([]int64{ownerID}, members...)); err != nil {
		return domain.Conversation{}, 0, err
	}
	if err := lockOwnedGroup(tx, ownerID, conversationID); err != nil {
		return domain.Conversation{}, 0, err
	}

	rows, err := tx.Query(`SELECT user_id, membership_status FROM conversation_members WHERE conversation_id = ? ORDER BY user_id FOR UPDATE`, conversationID)
	if err != nil {
		return domain.Conversation{}, 0, err
	}
	statuses := make(map[int64]string)
	acceptedMembers := make([]int64, 0)
	activeCount := 0
	for rows.Next() {
		var memberID int64
		var status string
		if err := rows.Scan(&memberID, &status); err != nil {
			rows.Close()
			return domain.Conversation{}, 0, err
		}
		statuses[memberID] = status
		if status == "accepted" || status == "pending" {
			activeCount++
		}
		if status == "accepted" {
			acceptedMembers = append(acceptedMembers, memberID)
		}
	}
	if err := rows.Close(); err != nil {
		return domain.Conversation{}, 0, err
	}

	eligible := make([]int64, 0, len(members))
	for _, memberID := range members {
		if status := statuses[memberID]; status == "accepted" || status == "pending" {
			continue
		}
		for _, acceptedMemberID := range acceptedMembers {
			blocked, err := blockedEitherWayQuery(tx, memberID, acceptedMemberID)
			if err != nil {
				return domain.Conversation{}, 0, err
			}
			if blocked {
				return domain.Conversation{}, 0, errors.New("邀请列表中存在与群成员互相拉黑的用户")
			}
		}
		eligible = append(eligible, memberID)
	}
	if activeCount+len(eligible) > maxConversationMembers {
		return domain.Conversation{}, 0, errors.New("群聊最多允许 50 位已加入或待确认成员")
	}

	now := time.Now().UTC()
	for _, memberID := range eligible {
		if statuses[memberID] == "declined" {
			if _, err := tx.Exec(`UPDATE conversation_members SET membership_status = 'pending', invited_by = ?, joined_at = ?, last_read_at = NULL, responded_at = NULL, last_notified_at = ? WHERE conversation_id = ? AND user_id = ?`, ownerID, now, now, conversationID, memberID); err != nil {
				return domain.Conversation{}, 0, err
			}
		} else if _, err := tx.Exec(`INSERT INTO conversation_members (conversation_id, user_id, membership_status, invited_by, joined_at, last_read_at, responded_at, last_notified_at) VALUES (?, ?, 'pending', ?, ?, NULL, NULL, ?)`, conversationID, memberID, ownerID, now, now); err != nil {
			return domain.Conversation{}, 0, err
		}
		if err := createConversationNotification(tx, memberID, ownerID, "group_invite", conversationID, now); err != nil {
			return domain.Conversation{}, 0, err
		}
	}
	if len(eligible) > 0 {
		if _, err := tx.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, now, conversationID); err != nil {
			return domain.Conversation{}, 0, err
		}
	}
	conversation, err := getConversation(tx, ownerID, conversationID)
	if err != nil {
		return domain.Conversation{}, 0, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Conversation{}, 0, err
	}
	return conversation, int64(len(eligible)), nil
}

func (s *Store) UpdateConversationName(ownerID, conversationID int64, name string) (domain.Conversation, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 120 || containsControl(name) {
		return domain.Conversation{}, errors.New("群聊名称无效")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Conversation{}, err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, []int64{ownerID}); err != nil {
		return domain.Conversation{}, err
	}
	if err := lockOwnedGroup(tx, ownerID, conversationID); err != nil {
		return domain.Conversation{}, err
	}
	if _, err := tx.Exec(`UPDATE conversations SET name = ?, updated_at = ? WHERE id = ?`, name, time.Now().UTC(), conversationID); err != nil {
		return domain.Conversation{}, err
	}
	conversation, err := getConversation(tx, ownerID, conversationID)
	if err != nil {
		return domain.Conversation{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Conversation{}, err
	}
	return conversation, nil
}

func (s *Store) RemoveConversationMember(ownerID, conversationID, memberID int64) error {
	if memberID <= 0 || memberID == ownerID {
		return errors.New("不能移除群主")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, []int64{ownerID}); err != nil {
		return err
	}
	if err := lockOwnedGroup(tx, ownerID, conversationID); err != nil {
		return err
	}
	var status string
	if err := tx.QueryRow(`SELECT membership_status FROM conversation_members WHERE conversation_id = ? AND user_id = ? FOR UPDATE`, conversationID, memberID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("该用户不在群聊中")
		}
		return err
	}
	if status != "accepted" && status != "pending" {
		return errors.New("该用户不在群聊中")
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`DELETE FROM conversation_members WHERE conversation_id = ? AND user_id = ?`, conversationID, memberID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, now, conversationID); err != nil {
		return err
	}
	if err := createConversationNotification(tx, memberID, ownerID, "group_removed", conversationID, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) LeaveConversation(userID, conversationID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, []int64{userID}); err != nil {
		return err
	}
	var kind string
	var ownerID int64
	if err := tx.QueryRow(`SELECT kind, created_by FROM conversations WHERE id = ? FOR UPDATE`, conversationID).Scan(&kind, &ownerID); err != nil {
		return errors.New("群聊不存在")
	}
	if kind != "group" {
		return errors.New("私信会话不能退出")
	}
	if ownerID == userID {
		return errors.New("群主需要先解散群聊")
	}
	var status string
	if err := tx.QueryRow(`SELECT membership_status FROM conversation_members WHERE conversation_id = ? AND user_id = ? FOR UPDATE`, conversationID, userID).Scan(&status); err != nil || status != "accepted" {
		return errors.New("你不在该群聊中")
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`DELETE FROM conversation_members WHERE conversation_id = ? AND user_id = ?`, conversationID, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, now, conversationID); err != nil {
		return err
	}
	if err := createConversationNotification(tx, ownerID, userID, "group_left", conversationID, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteConversation(ownerID, conversationID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, []int64{ownerID}); err != nil {
		return err
	}
	if err := lockOwnedGroup(tx, ownerID, conversationID); err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT user_id FROM conversation_members WHERE conversation_id = ? AND user_id <> ? AND membership_status IN ('accepted', 'pending') ORDER BY user_id FOR UPDATE`, conversationID, ownerID)
	if err != nil {
		return err
	}
	members := make([]int64, 0)
	for rows.Next() {
		var memberID int64
		if err := rows.Scan(&memberID); err != nil {
			rows.Close()
			return err
		}
		members = append(members, memberID)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, memberID := range members {
		if err := createConversationNotification(tx, memberID, ownerID, "group_dissolved", conversationID, now); err != nil {
			return err
		}
	}
	result, err := tx.Exec(`DELETE FROM conversations WHERE id = ? AND created_by = ?`, conversationID, ownerID)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return err
		}
		return errors.New("群聊不存在")
	}
	return tx.Commit()
}

func newConversationInviteToken() (string, string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buffer)
	hash := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(hash[:]), nil
}

func conversationInviteTokenHash(token string) (string, error) {
	token = strings.TrimSpace(token)
	if len(token) < 32 || len(token) > 128 {
		return "", errors.New("群聊邀请链接无效")
	}
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:]), nil
}

func (s *Store) CreateConversationInviteLink(ownerID, conversationID int64) (domain.ConversationInviteLink, error) {
	token, tokenHash, err := newConversationInviteToken()
	if err != nil {
		return domain.ConversationInviteLink{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.ConversationInviteLink{}, err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, []int64{ownerID}); err != nil {
		return domain.ConversationInviteLink{}, err
	}
	if err := lockOwnedGroup(tx, ownerID, conversationID); err != nil {
		return domain.ConversationInviteLink{}, err
	}
	now := time.Now().UTC()
	expiresAt := now.Add(7 * 24 * time.Hour)
	if _, err := tx.Exec(`UPDATE conversation_invite_links SET revoked_at = ? WHERE conversation_id = ? AND revoked_at IS NULL`, now, conversationID); err != nil {
		return domain.ConversationInviteLink{}, err
	}
	if _, err := tx.Exec(`INSERT INTO conversation_invite_links (conversation_id, token_hash, created_by, created_at, expires_at, revoked_at) VALUES (?, ?, ?, ?, ?, NULL)`, conversationID, tokenHash, ownerID, now, expiresAt); err != nil {
		return domain.ConversationInviteLink{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ConversationInviteLink{}, err
	}
	return domain.ConversationInviteLink{Token: token, CreatedAt: now, ExpiresAt: expiresAt}, nil
}

func (s *Store) RevokeConversationInviteLinks(ownerID, conversationID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockActiveUsers(tx, []int64{ownerID}); err != nil {
		return err
	}
	if err := lockOwnedGroup(tx, ownerID, conversationID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE conversation_invite_links SET revoked_at = ? WHERE conversation_id = ? AND revoked_at IS NULL`, time.Now().UTC(), conversationID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) JoinConversationByInvite(userID int64, token string) (domain.Conversation, error) {
	tokenHash, err := conversationInviteTokenHash(token)
	if err != nil {
		return domain.Conversation{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Conversation{}, err
	}
	defer tx.Rollback()
	var conversationID, ownerID int64
	var kind string
	var expiresAt time.Time
	var revokedAt sql.NullTime
	if err := tx.QueryRow(`SELECT l.conversation_id, c.created_by, c.kind, l.expires_at, l.revoked_at FROM conversation_invite_links l JOIN conversations c ON c.id = l.conversation_id WHERE l.token_hash = ? FOR UPDATE`, tokenHash).Scan(&conversationID, &ownerID, &kind, &expiresAt, &revokedAt); err != nil {
		return domain.Conversation{}, errors.New("群聊邀请链接无效或已过期")
	}
	if kind != "group" || revokedAt.Valid || !time.Now().UTC().Before(expiresAt) {
		return domain.Conversation{}, errors.New("群聊邀请链接无效或已过期")
	}
	if err := lockActiveUsers(tx, []int64{userID, ownerID}); err != nil {
		return domain.Conversation{}, err
	}
	rows, err := tx.Query(`SELECT user_id, membership_status FROM conversation_members WHERE conversation_id = ? ORDER BY user_id FOR UPDATE`, conversationID)
	if err != nil {
		return domain.Conversation{}, err
	}
	acceptedMembers := make([]int64, 0)
	activeCount := 0
	currentStatus := ""
	for rows.Next() {
		var memberID int64
		var status string
		if err := rows.Scan(&memberID, &status); err != nil {
			rows.Close()
			return domain.Conversation{}, err
		}
		if status == "accepted" || status == "pending" {
			activeCount++
		}
		if status == "accepted" {
			acceptedMembers = append(acceptedMembers, memberID)
		}
		if memberID == userID {
			currentStatus = status
		}
	}
	if err := rows.Close(); err != nil {
		return domain.Conversation{}, err
	}
	if currentStatus == "accepted" {
		conversation, err := getConversation(tx, userID, conversationID)
		if err != nil {
			return domain.Conversation{}, err
		}
		if err := tx.Commit(); err != nil {
			return domain.Conversation{}, err
		}
		return conversation, nil
	}
	if currentStatus != "pending" && activeCount >= maxConversationMembers {
		return domain.Conversation{}, errors.New("群聊人数已满")
	}
	for _, memberID := range acceptedMembers {
		if memberID == userID {
			continue
		}
		blocked, err := blockedEitherWayQuery(tx, userID, memberID)
		if err != nil {
			return domain.Conversation{}, err
		}
		if blocked {
			return domain.Conversation{}, errors.New("无法加入存在拉黑关系的群聊")
		}
	}
	now := time.Now().UTC()
	if currentStatus == "pending" || currentStatus == "declined" {
		if _, err := tx.Exec(`UPDATE conversation_members SET membership_status = 'accepted', last_read_at = ?, responded_at = ?, last_notified_at = NULL WHERE conversation_id = ? AND user_id = ?`, now, now, conversationID, userID); err != nil {
			return domain.Conversation{}, err
		}
	} else if _, err := tx.Exec(`INSERT INTO conversation_members (conversation_id, user_id, membership_status, invited_by, joined_at, last_read_at, responded_at, last_notified_at) VALUES (?, ?, 'accepted', NULL, ?, ?, ?, NULL)`, conversationID, userID, now, now, now); err != nil {
		return domain.Conversation{}, err
	}
	if _, err := tx.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, now, conversationID); err != nil {
		return domain.Conversation{}, err
	}
	if err := createConversationNotification(tx, ownerID, userID, "group_joined", conversationID, now); err != nil {
		return domain.Conversation{}, err
	}
	conversation, err := getConversation(tx, userID, conversationID)
	if err != nil {
		return domain.Conversation{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Conversation{}, err
	}
	return conversation, nil
}
