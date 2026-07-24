package app

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (s *Server) createPostReport(w http.ResponseWriter, r *http.Request) {
	s.createContentReport(w, r, "post", chi.URLParam(r, "postID"))
}

func (s *Server) createCommentReport(w http.ResponseWriter, r *http.Request) {
	s.createContentReport(w, r, "comment", chi.URLParam(r, "commentID"))
}

func (s *Server) createProfileReport(w http.ResponseWriter, r *http.Request) {
	s.createContentReport(w, r, "profile", chi.URLParam(r, "userID"))
}

func (s *Server) createContentReport(w http.ResponseWriter, r *http.Request, targetType, rawTargetID string) {
	targetID, err := parsePositiveID(rawTargetID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "report_target_invalid", "举报对象无效")
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.CreateContentReport(currentUser(r).ID, targetType, targetID, input.Reason)
	if err != nil {
		s.writeModerationStoreError(w, r, err, "report_create_failed", "举报提交失败")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) listAdminContentReports(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListAdminContentReports(r.URL.Query().Get("status"), 100)
	if err != nil {
		s.writeModerationStoreError(w, r, err, "admin_reports_failed", "举报审核列表加载失败")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) reviewAdminContentReport(w http.ResponseWriter, r *http.Request) {
	reportID, err := parsePositiveID(chi.URLParam(r, "reportID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "report_id_invalid", "举报记录 ID 无效")
		return
	}
	var input struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.ReviewContentReport(currentUser(r).ID, reportID, strings.TrimSpace(input.Status), input.Note)
	if err != nil {
		s.writeModerationStoreError(w, r, err, "report_review_failed", "举报审核失败")
		return
	}
	writeJSON(w, http.StatusOK, item)
}
