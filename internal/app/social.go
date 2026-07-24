package app

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"roblox-community/internal/store"
)

func paginationParams(r *http.Request, defaultLimit, maxLimit int) (int, int) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > maxLimit {
		limit = defaultLimit
	}
	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil || offset < 0 || offset > 100000 {
		offset = 0
	}
	return limit, offset
}

func (s *Server) searchUsers(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("paged") == "1" {
		limit, offset := paginationParams(r, 30, 50)
		result, err := s.store.SearchUsersPage(r.URL.Query().Get("q"), limit, offset, false)
		if err != nil {
			writeError(w, 500, "users_search_failed", "用户搜索失败")
			return
		}
		writeJSON(w, 200, result)
		return
	}
	result, err := s.store.SearchUsers(r.URL.Query().Get("q"), 30, false)
	if err != nil {
		writeError(w, 500, "users_search_failed", "用户搜索失败")
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) searchCommunity(w http.ResponseWriter, r *http.Request) {
	result, err := s.store.SearchCommunity(r.URL.Query().Get("q"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "search_invalid", "搜索关键词无效")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) hotUsers(w http.ResponseWriter, _ *http.Request) {
	result, err := s.store.SearchUsers("", 20, true)
	if err != nil {
		writeError(w, 500, "hot_users_failed", "热门用户加载失败")
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) toggleUserBlock(w http.ResponseWriter, r *http.Request) {
	targetID, _ := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	blocked := true
	if strings.EqualFold(r.Method, http.MethodDelete) {
		blocked = false
	}
	if err := s.store.SetUserBlocked(currentUser(r).ID, targetID, blocked); err != nil {
		writeError(w, 400, "block_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"blocked": blocked})
}

func (s *Server) userBlockStatus(w http.ResponseWriter, r *http.Request) {
	targetID, _ := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	blocked, err := s.store.IsUserBlocked(currentUser(r).ID, targetID)
	if err != nil {
		writeError(w, 500, "block_status_failed", "拉黑状态读取失败")
		return
	}
	writeJSON(w, 200, map[string]any{"blocked": blocked})
}

func (s *Server) followStatus(w http.ResponseWriter, r *http.Request) {
	targetID, _ := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	status, err := s.store.GetFollowStatus(currentUser(r).ID, targetID)
	if err != nil {
		writeError(w, 404, "follow_status_failed", "关注状态获取失败")
		return
	}
	writeJSON(w, 200, status)
}

func (s *Server) followUser(w http.ResponseWriter, r *http.Request) {
	targetID, _ := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	status, err := s.store.SetUserFollow(currentUser(r).ID, targetID, true)
	if err != nil {
		writeError(w, 400, "follow_failed", err.Error())
		return
	}
	writeJSON(w, 200, status)
}

func (s *Server) unfollowUser(w http.ResponseWriter, r *http.Request) {
	targetID, _ := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	status, err := s.store.SetUserFollow(currentUser(r).ID, targetID, false)
	if err != nil {
		writeError(w, 400, "unfollow_failed", err.Error())
		return
	}
	writeJSON(w, 200, status)
}

func (s *Server) toggleBookmark(w http.ResponseWriter, r *http.Request) {
	postID, _ := strconv.ParseInt(chi.URLParam(r, "postID"), 10, 64)
	bookmarked, err := s.store.TogglePostBookmark(currentUser(r).ID, postID)
	if err != nil {
		writeError(w, 400, "bookmark_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"bookmarked": bookmarked})
}

func (s *Server) bookmarkStatus(w http.ResponseWriter, r *http.Request) {
	postID, _ := strconv.ParseInt(chi.URLParam(r, "postID"), 10, 64)
	bookmarked, err := s.store.BookmarkStatus(currentUser(r).ID, postID)
	if err != nil {
		writeError(w, 400, "bookmark_status_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"bookmarked": bookmarked})
}

func (s *Server) toggleRepost(w http.ResponseWriter, r *http.Request) {
	postID, _ := strconv.ParseInt(chi.URLParam(r, "postID"), 10, 64)
	reposted, count, err := s.store.TogglePostRepost(currentUser(r).ID, postID)
	if err != nil {
		writeError(w, 400, "repost_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"reposted": reposted, "repost_count": count})
}

func (s *Server) repostStatus(w http.ResponseWriter, r *http.Request) {
	postID, _ := strconv.ParseInt(chi.URLParam(r, "postID"), 10, 64)
	reposted, count, err := s.store.RepostStatus(currentUser(r).ID, postID)
	if err != nil {
		writeError(w, 400, "repost_status_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"reposted": reposted, "repost_count": count})
}

func (s *Server) followingFeed(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("paged") == "1" {
		limit, offset := paginationParams(r, 30, 50)
		result, err := s.store.ListFollowingPostsPage(currentUser(r).ID, limit, offset)
		if err != nil {
			writeError(w, 500, "following_feed_failed", "关注动态加载失败")
			return
		}
		writeJSON(w, 200, result)
		return
	}
	result, err := s.store.ListFollowingPosts(currentUser(r).ID, 30)
	if err != nil {
		writeError(w, 500, "following_feed_failed", "关注动态加载失败")
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) myBookmarks(w http.ResponseWriter, r *http.Request) {
	result, err := s.store.ListBookmarkedPosts(currentUser(r).ID, 50)
	if err != nil {
		writeError(w, 500, "bookmarks_failed", "收藏加载失败")
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) notifications(w http.ResponseWriter, r *http.Request) {
	result, err := s.store.ListNotifications(currentUser(r).ID, 50)
	if err != nil {
		writeError(w, 500, "notifications_failed", "通知加载失败")
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) unreadNotifications(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.UnreadNotificationCount(currentUser(r).ID)
	if err != nil {
		writeError(w, 500, "notifications_count_failed", "通知数量获取失败")
		return
	}
	writeJSON(w, 200, map[string]any{"count": count})
}

func (s *Server) markNotificationsRead(w http.ResponseWriter, r *http.Request) {
	if err := s.store.MarkNotificationsRead(currentUser(r).ID); err != nil {
		writeError(w, 500, "notifications_read_failed", "通知状态更新失败")
		return
	}
	writeJSON(w, 200, map[string]any{"read": true})
}

func (s *Server) deleteComment(w http.ResponseWriter, r *http.Request) {
	commentID, _ := strconv.ParseInt(chi.URLParam(r, "commentID"), 10, 64)
	if err := s.store.DeleteComment(currentUser(r).ID, currentUser(r).Role == "admin", commentID); err != nil {
		writeError(w, 400, "comment_delete_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"deleted": true})
}

func (s *Server) togglePostLike(w http.ResponseWriter, r *http.Request) {
	postID, _ := strconv.ParseInt(chi.URLParam(r, "postID"), 10, 64)
	liked, count, err := s.store.TogglePostLike(currentUser(r).ID, postID)
	if err != nil {
		writeError(w, 400, "post_like_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"liked": liked, "like_count": count})
}

func (s *Server) postLikeStatus(w http.ResponseWriter, r *http.Request) {
	postID, _ := strconv.ParseInt(chi.URLParam(r, "postID"), 10, 64)
	liked, count, err := s.store.LikeStatus("post", currentUser(r).ID, postID)
	if err != nil {
		writeError(w, 400, "post_like_status_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"liked": liked, "like_count": count})
}

func (s *Server) toggleCommentLike(w http.ResponseWriter, r *http.Request) {
	commentID, _ := strconv.ParseInt(chi.URLParam(r, "commentID"), 10, 64)
	liked, count, err := s.store.ToggleCommentLike(currentUser(r).ID, commentID)
	if err != nil {
		writeError(w, 400, "comment_like_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"liked": liked, "like_count": count})
}

func (s *Server) commentLikeStatus(w http.ResponseWriter, r *http.Request) {
	commentID, _ := strconv.ParseInt(chi.URLParam(r, "commentID"), 10, 64)
	liked, count, err := s.store.LikeStatus("comment", currentUser(r).ID, commentID)
	if err != nil {
		writeError(w, 400, "comment_like_status_failed", "评论点赞状态读取失败")
		return
	}
	writeJSON(w, 200, map[string]any{"liked": liked, "like_count": count})
}

func (s *Server) listConversations(w http.ResponseWriter, r *http.Request) {
	result, err := s.store.ListConversations(currentUser(r).ID)
	if err != nil {
		writeError(w, 500, "conversations_failed", "会话加载失败")
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) listConversationInvites(w http.ResponseWriter, r *http.Request) {
	result, err := s.store.ListConversationInvites(currentUser(r).ID)
	if err != nil {
		writeError(w, 500, "conversation_invites_failed", "群聊邀请加载失败")
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) createConversation(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Kind      string  `json:"kind"`
		Name      string  `json:"name"`
		MemberIDs []int64 `json:"member_ids"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Name) != "" && !s.approveContent(w, r, "conversation", input.Name) {
		return
	}
	item, err := s.store.CreateConversation(currentUser(r).ID, input.Kind, input.Name, input.MemberIDs)
	if err != nil {
		if errors.Is(err, store.ErrGroupCreationRateLimited) {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "group_creation_rate_limited", err.Error())
			return
		}
		writeError(w, 400, "conversation_create_failed", err.Error())
		return
	}
	writeJSON(w, 201, item)
}

func (s *Server) respondConversationInvite(w http.ResponseWriter, r *http.Request) {
	conversationID, _ := strconv.ParseInt(chi.URLParam(r, "conversationID"), 10, 64)
	var input struct {
		Accept bool `json:"accept"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.RespondConversationInvite(currentUser(r).ID, conversationID, input.Accept)
	if err != nil {
		writeError(w, 400, "conversation_invite_response_failed", err.Error())
		return
	}
	writeJSON(w, 200, item)
}

func (s *Server) listConversationMembers(w http.ResponseWriter, r *http.Request) {
	conversationID, _ := strconv.ParseInt(chi.URLParam(r, "conversationID"), 10, 64)
	items, err := s.store.ListConversationMembers(currentUser(r).ID, conversationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "conversation_members_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) inviteConversationMembers(w http.ResponseWriter, r *http.Request) {
	conversationID, _ := strconv.ParseInt(chi.URLParam(r, "conversationID"), 10, 64)
	var input struct {
		MemberIDs []int64 `json:"member_ids"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	conversation, invited, err := s.store.InviteConversationMembers(currentUser(r).ID, conversationID, input.MemberIDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, "conversation_invite_members_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversation": conversation, "invited": invited})
}

func (s *Server) removeConversationMember(w http.ResponseWriter, r *http.Request) {
	conversationID, _ := strconv.ParseInt(chi.URLParam(r, "conversationID"), 10, 64)
	memberID, _ := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err := s.store.RemoveConversationMember(currentUser(r).ID, conversationID, memberID); err != nil {
		writeError(w, http.StatusBadRequest, "conversation_member_remove_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": true})
}

func (s *Server) updateConversation(w http.ResponseWriter, r *http.Request) {
	conversationID, _ := strconv.ParseInt(chi.URLParam(r, "conversationID"), 10, 64)
	var input struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if !s.approveContent(w, r, "conversation", input.Name) {
		return
	}
	conversation, err := s.store.UpdateConversationName(currentUser(r).ID, conversationID, input.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, "conversation_update_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, conversation)
}

func (s *Server) leaveConversation(w http.ResponseWriter, r *http.Request) {
	conversationID, _ := strconv.ParseInt(chi.URLParam(r, "conversationID"), 10, 64)
	if err := s.store.LeaveConversation(currentUser(r).ID, conversationID); err != nil {
		writeError(w, http.StatusBadRequest, "conversation_leave_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"left": true})
}

func (s *Server) deleteConversation(w http.ResponseWriter, r *http.Request) {
	conversationID, _ := strconv.ParseInt(chi.URLParam(r, "conversationID"), 10, 64)
	if err := s.store.DeleteConversation(currentUser(r).ID, conversationID); err != nil {
		writeError(w, http.StatusBadRequest, "conversation_delete_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (s *Server) createConversationInviteLink(w http.ResponseWriter, r *http.Request) {
	conversationID, _ := strconv.ParseInt(chi.URLParam(r, "conversationID"), 10, 64)
	item, err := s.store.CreateConversationInviteLink(currentUser(r).ID, conversationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "conversation_invite_link_failed", err.Error())
		return
	}
	baseURL := strings.TrimRight(s.publicURL, "/")
	joinURL := "/messages/join?token=" + url.QueryEscape(item.Token)
	if baseURL != "" {
		joinURL = baseURL + joinURL
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": item.Token, "join_url": joinURL, "created_at": item.CreatedAt, "expires_at": item.ExpiresAt})
}

func (s *Server) revokeConversationInviteLink(w http.ResponseWriter, r *http.Request) {
	conversationID, _ := strconv.ParseInt(chi.URLParam(r, "conversationID"), 10, 64)
	if err := s.store.RevokeConversationInviteLinks(currentUser(r).ID, conversationID); err != nil {
		writeError(w, http.StatusBadRequest, "conversation_invite_link_revoke_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"revoked": true})
}

func (s *Server) joinConversationByInvite(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	conversation, err := s.store.JoinConversationByInvite(currentUser(r).ID, input.Token)
	if err != nil {
		writeError(w, http.StatusBadRequest, "conversation_invite_join_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, conversation)
}

func (s *Server) remindGroupInvitees(w http.ResponseWriter, r *http.Request) {
	conversationID, _ := strconv.ParseInt(chi.URLParam(r, "conversationID"), 10, 64)
	count, err := s.store.RemindGroupInvitees(currentUser(r).ID, conversationID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrGroupInviteReminderRateLimited):
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "group_invite_reminder_rate_limited", err.Error())
		case errors.Is(err, store.ErrNoPendingGroupInvites):
			writeError(w, http.StatusConflict, "group_invites_already_resolved", err.Error())
		case errors.Is(err, store.ErrGroupInviteReminderNotPermitted):
			writeError(w, http.StatusForbidden, "group_invite_reminder_forbidden", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "group_invite_reminder_failed", "提醒发送失败")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reminded": count})
}

func (s *Server) listMessages(w http.ResponseWriter, r *http.Request) {
	conversationID, _ := strconv.ParseInt(chi.URLParam(r, "conversationID"), 10, 64)
	result, err := s.store.ListMessages(currentUser(r).ID, conversationID, 100)
	if err != nil {
		writeError(w, 403, "messages_forbidden", err.Error())
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) createMessage(w http.ResponseWriter, r *http.Request) {
	conversationID, _ := strconv.ParseInt(chi.URLParam(r, "conversationID"), 10, 64)
	var input struct {
		Content string `json:"content"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if !s.approveContent(w, r, "message", input.Content) {
		return
	}
	item, err := s.store.SendMessage(currentUser(r).ID, conversationID, input.Content)
	if err != nil {
		writeError(w, 400, "message_send_failed", err.Error())
		return
	}
	writeJSON(w, 201, item)
}
