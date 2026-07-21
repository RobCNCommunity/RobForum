package app

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (s *Server) searchUsers(w http.ResponseWriter, r *http.Request) {
	result, err := s.store.SearchUsers(r.URL.Query().Get("q"), 30, false)
	if err != nil {
		writeError(w, 500, "users_search_failed", "用户搜索失败")
		return
	}
	writeJSON(w, 200, result)
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

func (s *Server) createConversation(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Kind      string  `json:"kind"`
		Name      string  `json:"name"`
		MemberIDs []int64 `json:"member_ids"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.CreateConversation(currentUser(r).ID, input.Kind, input.Name, input.MemberIDs)
	if err != nil {
		writeError(w, 400, "conversation_create_failed", err.Error())
		return
	}
	writeJSON(w, 201, item)
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
	item, err := s.store.SendMessage(currentUser(r).ID, conversationID, input.Content)
	if err != nil {
		writeError(w, 400, "message_send_failed", err.Error())
		return
	}
	writeJSON(w, 201, item)
}
