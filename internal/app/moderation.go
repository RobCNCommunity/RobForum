package app

import (
	"database/sql"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"roblox-community/internal/store"
)

func (s *Server) listAdminUsers(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListAdminUsers(r.URL.Query().Get("q"), r.URL.Query().Get("status"), 100)
	if err != nil {
		s.writeModerationStoreError(w, r, err, "admin_users_failed", "用户列表加载失败")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) updateAdminUserStatus(w http.ResponseWriter, r *http.Request) {
	targetID, err := parsePositiveID(chi.URLParam(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "user_id_invalid", "用户 ID 无效")
		return
	}
	var input struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, resourceFiles, err := s.store.SetAdminUserStatusWithResourceCleanup(currentUser(r).ID, targetID, input.Status, input.Reason)
	if err != nil {
		s.writeModerationStoreError(w, r, err, "user_status_update_failed", "用户状态更新失败")
		return
	}
	s.removeResourceUploadFiles(resourceFiles)
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) deleteAdminUser(w http.ResponseWriter, r *http.Request) {
	targetID, err := parsePositiveID(chi.URLParam(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "user_id_invalid", "用户 ID 无效")
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	resourceFiles, err := s.store.DeleteAdminUserWithResourceCleanup(currentUser(r).ID, targetID, input.Reason)
	if err != nil {
		s.writeModerationStoreError(w, r, err, "user_delete_failed", "用户删除失败")
		return
	}
	s.removeResourceUploadFiles(resourceFiles)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (s *Server) removeResourceUploadFiles(storedNames []string) {
	seen := make(map[string]struct{}, len(storedNames))
	for _, storedName := range storedNames {
		storedName = strings.TrimSpace(storedName)
		if storedName == "" || filepath.Base(storedName) != storedName {
			if s.logger != nil {
				s.logger.Error("refusing to remove invalid resource upload name", "stored_name", storedName)
			}
			continue
		}
		if _, exists := seen[storedName]; exists {
			continue
		}
		seen[storedName] = struct{}{}

		path := filepath.Join(s.uploadDir, storedName)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			if s.logger != nil {
				s.logger.Error("failed to validate resource upload for removal", "path", path, "error", err)
			}
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) && s.logger != nil {
			s.logger.Error("failed to remove banned user's resource upload", "path", path, "error", err)
		}
	}
}

func (s *Server) listAdminPosts(w http.ResponseWriter, r *http.Request) {
	postID := int64(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("post_id")); raw != "" {
		var err error
		postID, err = parsePositiveID(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "post_id_invalid", "帖子 ID 无效")
			return
		}
	}
	items, err := s.store.ListAdminPosts(r.URL.Query().Get("status"), r.URL.Query().Get("q"), postID, 100)
	if err != nil {
		s.writeModerationStoreError(w, r, err, "admin_posts_failed", "内容审核列表加载失败")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) moderateAdminPost(w http.ResponseWriter, r *http.Request) {
	postID, err := parsePositiveID(chi.URLParam(r, "postID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "post_id_invalid", "帖子 ID 无效")
		return
	}
	var input struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.ModeratePost(currentUser(r).ID, postID, input.Status, input.Reason)
	if err != nil {
		s.writeModerationStoreError(w, r, err, "post_moderation_failed", "帖子处理失败")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func parsePositiveID(raw string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

func (s *Server) writeModerationStoreError(w http.ResponseWriter, r *http.Request, err error, code, fallback string) {
	var clientError *store.ModerationError
	if errors.As(err, &clientError) {
		status := http.StatusBadRequest
		switch clientError.Kind {
		case store.ModerationErrorNotFound:
			status = http.StatusNotFound
		case store.ModerationErrorForbidden:
			status = http.StatusForbidden
		case store.ModerationErrorConflict:
			status = http.StatusConflict
		}
		writeError(w, status, code, clientError.Message)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, code, "记录不存在")
		return
	}
	if s.logger != nil {
		s.logger.Error("moderation request failed", "request_id", middleware.GetReqID(r.Context()), "code", code, "error", err)
	}
	writeError(w, http.StatusInternalServerError, code, fallback)
}
