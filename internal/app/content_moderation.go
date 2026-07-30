package app

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
	"roblox-community/internal/contentmoderation"
)

const maxModeratedImageBytes = 7 << 20

func moderationText(parts ...string) string {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return strings.Join(values, "\n")
}

func postModerationText(title, content string, tags []string) string {
	parts := []string{"标题：" + title, "正文：" + content}
	for _, tag := range tags {
		if tag = strings.TrimSpace(tag); tag != "" {
			parts = append(parts, "标签："+tag)
		}
	}
	return moderationText(parts...)
}

// approveContent runs before any user content is committed. A provider error,
// malformed response, or failed audit write is fail-closed.
func (s *Server) approveContent(w http.ResponseWriter, r *http.Request, contentType, content string) bool {
	if s.moderator == nil || !s.moderator.Enabled() {
		return true
	}
	user := currentUser(r)
	result, reviewErr := s.moderator.Review(r.Context(), contentType, content)
	decision := "allowed"
	reason := result.Reason
	if reviewErr != nil {
		decision = "unavailable"
		reason = "provider_unavailable"
	} else if result.Violation {
		decision = "blocked"
	}
	if err := s.store.RecordContentModeration(user.ID, contentType, content, decision, reason, result.Model); err != nil {
		if s.logger != nil {
			s.logger.Error("content moderation audit failed", "request_id", middleware.GetReqID(r.Context()), "user_id", user.ID, "content_type", contentType, "error", err)
		}
		writeError(w, http.StatusServiceUnavailable, "content_moderation_unavailable", "内容审核暂不可用，请稍后重试")
		return false
	}
	if reviewErr != nil {
		if s.logger != nil {
			s.logger.Warn("content moderation unavailable", "request_id", middleware.GetReqID(r.Context()), "user_id", user.ID, "content_type", contentType, "error", reviewErr)
		}
		status := http.StatusServiceUnavailable
		if !errors.Is(reviewErr, contentmoderation.ErrUnavailable) {
			status = http.StatusInternalServerError
		}
		writeError(w, status, "content_moderation_unavailable", "内容审核暂不可用，请稍后重试")
		return false
	}
	if result.Violation {
		writeError(w, http.StatusUnprocessableEntity, "content_blocked", "内容未通过安全审核，无法发送或发布")
		return false
	}
	return true
}

func (s *Server) postMachineApproved(r *http.Request, content string) bool {
	if s.moderator == nil || !s.moderator.Enabled() {
		return false
	}
	user := currentUser(r)
	result, reviewErr := s.moderator.Review(r.Context(), "post", content)
	decision := "allowed"
	reason := result.Reason
	if reviewErr != nil {
		decision = "unavailable"
		reason = "provider_unavailable"
	} else if result.Violation {
		decision = "blocked"
	}
	if strings.TrimSpace(result.Model) == "" {
		result.Model = "content_moderation"
	}
	if err := s.store.RecordContentModeration(user.ID, "post", content, decision, reason, result.Model); err != nil {
		if s.logger != nil {
			s.logger.Error("post moderation audit failed", "request_id", middleware.GetReqID(r.Context()), "user_id", user.ID, "error", err)
		}
		return false
	}
	if reviewErr != nil && s.logger != nil {
		s.logger.Warn("post moderation unavailable; queued for manual review", "request_id", middleware.GetReqID(r.Context()), "user_id", user.ID, "error", reviewErr)
	}
	return reviewErr == nil && !result.Violation
}

// approveImagePath reviews a newly saved, not-yet-committed user image. The
// caller remains responsible for deleting the temporary file when this
// method returns false. Only a domain-separated SHA-256 input is passed to the
// audit store; image bytes are never persisted in the moderation audit table.
func (s *Server) approveImagePath(w http.ResponseWriter, r *http.Request, contentType, path string) bool {
	imageModerator, ok := s.moderator.(contentmoderation.ImageService)
	if !ok || !imageModerator.ImageEnabled() {
		return true
	}
	file, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "image_moderation_unavailable", "图片审核暂不可用，请稍后重试")
		return false
	}
	info, statErr := file.Stat()
	if statErr != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxModeratedImageBytes {
		_ = file.Close()
		writeError(w, http.StatusUnprocessableEntity, "image_moderation_size_invalid", "图片不能为空或超过 7 MB")
		return false
	}
	imageBytes, readErr := io.ReadAll(io.LimitReader(file, maxModeratedImageBytes+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || len(imageBytes) < 1 || len(imageBytes) > maxModeratedImageBytes {
		writeError(w, http.StatusServiceUnavailable, "image_moderation_unavailable", "图片审核暂不可用，请稍后重试")
		return false
	}

	return s.approveImageBytes(w, r, contentType, imageBytes)
}

func moderationImageBytes(mimeType string, imageBytes []byte) ([]byte, error) {
	if mimeType != "image/gif" {
		return imageBytes, nil
	}
	frame, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return nil, err
	}
	var converted bytes.Buffer
	if err := png.Encode(&converted, frame); err != nil {
		return nil, err
	}
	return converted.Bytes(), nil
}

func (s *Server) approveImageBytes(w http.ResponseWriter, r *http.Request, contentType string, imageBytes []byte) bool {
	imageModerator, ok := s.moderator.(contentmoderation.ImageService)
	if !ok || !imageModerator.ImageEnabled() {
		return true
	}
	if len(imageBytes) < 1 || len(imageBytes) > maxModeratedImageBytes {
		writeError(w, http.StatusUnprocessableEntity, "image_moderation_size_invalid", "图片不能为空或超过 7 MB")
		return false
	}
	user := currentUser(r)
	result, reviewErr := imageModerator.ReviewImage(r.Context(), contentType, imageBytes)
	decision := "allowed"
	reason := result.Reason
	if reviewErr != nil {
		decision = "unavailable"
		reason = "provider_unavailable"
	} else if result.Violation {
		decision = "blocked"
	}
	if strings.TrimSpace(result.Model) == "" {
		result.Model = "image_moderation"
	}
	auditContent := "image\x00" + string(imageBytes)
	if err := s.store.RecordContentModeration(user.ID, contentType, auditContent, decision, reason, result.Model); err != nil {
		if s.logger != nil {
			s.logger.Error("image moderation audit failed", "request_id", middleware.GetReqID(r.Context()), "user_id", user.ID, "content_type", contentType, "error", err)
		}
		writeError(w, http.StatusServiceUnavailable, "image_moderation_unavailable", "图片审核暂不可用，请稍后重试")
		return false
	}
	if reviewErr != nil {
		if s.logger != nil {
			s.logger.Warn("image moderation unavailable", "request_id", middleware.GetReqID(r.Context()), "user_id", user.ID, "content_type", contentType, "error", reviewErr)
		}
		status := http.StatusServiceUnavailable
		if !errors.Is(reviewErr, contentmoderation.ErrUnavailable) {
			status = http.StatusInternalServerError
		}
		writeError(w, status, "image_moderation_unavailable", "图片审核暂不可用，请稍后重试")
		return false
	}
	if result.Violation {
		writeError(w, http.StatusUnprocessableEntity, "image_blocked", "图片未通过安全审核，无法上传")
		return false
	}
	return true
}

type responseSink struct {
	header http.Header
}

func (w *responseSink) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (*responseSink) WriteHeader(int) {}

func (*responseSink) Write(content []byte) (int, error) {
	return len(content), nil
}

func (s *Server) postImageMachineApproved(r *http.Request, path string) bool {
	imageModerator, ok := s.moderator.(contentmoderation.ImageService)
	if !ok || !imageModerator.ImageEnabled() {
		return false
	}
	return s.approveImagePath(&responseSink{}, r, "post", path)
}
