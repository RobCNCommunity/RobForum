package app

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"roblox-community/internal/store"
)

func (s *Server) avatarFrameUploadSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.GetAvatarFrameUploadSettings(currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "avatar_frame_settings_failed", "头像框上传设置加载失败")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) adminAvatarFrameUploadSettings(w http.ResponseWriter, _ *http.Request) {
	settings, err := s.store.GetAvatarFrameUploadSettings(0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "avatar_frame_settings_failed", "头像框上传设置加载失败")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) updateAdminAvatarFrameUploadSettings(w http.ResponseWriter, r *http.Request) {
	var input struct {
		AllowRegularUpload bool `json:"allow_regular_upload"`
		AllowMemberUpload  bool `json:"allow_member_upload"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	settings, err := s.store.UpdateAvatarFrameUploadSettings(currentUser(r).ID, input.AllowRegularUpload, input.AllowMemberUpload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "avatar_frame_settings_update_failed", "头像框上传设置保存失败")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) createAvatarFrameSubmission(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ImageURL    string `json:"image_url"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.CreateAvatarFrameSubmission(currentUser(r).ID, input.Name, input.Description, input.ImageURL)
	if errors.Is(err, store.ErrAvatarFrameUploadForbidden) {
		writeError(w, http.StatusForbidden, "avatar_frame_upload_forbidden", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "avatar_frame_submission_invalid", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) listMyAvatarFrameSubmissions(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListMyAvatarFrameSubmissions(currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "avatar_frame_submissions_failed", "头像框投稿记录加载失败")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) listAdminAvatarFrameSubmissions(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListAdminAvatarFrameSubmissions(strings.TrimSpace(r.URL.Query().Get("status")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "avatar_frame_submissions_invalid", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) reviewAdminAvatarFrameSubmission(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "submissionID"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "avatar_frame_submission_invalid", "头像框投稿无效")
		return
	}
	var input struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.ReviewAvatarFrameSubmission(currentUser(r).ID, id, input.Status, input.Note)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "avatar_frame_submission_not_found", "头像框投稿不存在")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "avatar_frame_submission_review_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}
