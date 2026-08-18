package app

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"roblox-community/internal/domain"
)

func (s *Server) listPublicAds(w http.ResponseWriter, _ *http.Request) {
	result, err := s.store.ListPublicAds()
	if err != nil {
		writeError(w, 500, "ads_failed", "广告加载失败")
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) listAdminAds(w http.ResponseWriter, _ *http.Request) {
	result, err := s.store.ListAdminAds()
	if err != nil {
		writeError(w, 500, "ads_failed", "广告加载失败")
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) createAdminAd(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title     string `json:"title"`
		ImageURL  string `json:"image_url"`
		LinkURL   string `json:"link_url"`
		SortOrder int    `json:"sort_order"`
		Enabled   bool   `json:"enabled"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.CreateAd(currentUser(r).ID, input.Title, input.ImageURL, input.LinkURL, input.SortOrder, input.Enabled)
	if err != nil {
		writeError(w, 400, "ad_create_failed", err.Error())
		return
	}
	writeJSON(w, 201, item)
}

func (s *Server) updateAdminAd(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "adID"), 10, 64)
	var input struct {
		Title     string `json:"title"`
		ImageURL  string `json:"image_url"`
		LinkURL   string `json:"link_url"`
		SortOrder int    `json:"sort_order"`
		Enabled   bool   `json:"enabled"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.UpdateAd(currentUser(r).ID, id, input.Title, input.ImageURL, input.LinkURL, input.SortOrder, input.Enabled)
	if err != nil {
		writeError(w, 400, "ad_update_failed", err.Error())
		return
	}
	writeJSON(w, 200, item)
}

func (s *Server) deleteAdminAd(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "adID"), 10, 64)
	if err := s.store.DeleteAd(currentUser(r).ID, id); err != nil {
		writeError(w, 400, "ad_delete_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"deleted": true})
}

func (s *Server) listPublicNotices(w http.ResponseWriter, _ *http.Request) {
	result, err := s.store.ListPublicNotices(20)
	if err != nil {
		writeError(w, 500, "notices_failed", "公告加载失败")
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) listAdminNotices(w http.ResponseWriter, _ *http.Request) {
	result, err := s.store.ListAdminNotices()
	if err != nil {
		writeError(w, 500, "notices_failed", "公告加载失败")
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) createAdminNotice(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title   string               `json:"title"`
		Content string               `json:"content"`
		LinkURL string               `json:"link_url"`
		Media   []domain.NoticeMedia `json:"media"`
		Level   string               `json:"level"`
		Pinned  bool                 `json:"pinned"`
		Enabled bool                 `json:"enabled"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.CreateNotice(currentUser(r).ID, input.Title, input.Content, input.LinkURL, input.Level, input.Pinned, input.Enabled, input.Media)
	if err != nil {
		writeError(w, 400, "notice_create_failed", err.Error())
		return
	}
	writeJSON(w, 201, item)
}

func (s *Server) updateAdminNotice(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "noticeID"), 10, 64)
	var input struct {
		Title   string               `json:"title"`
		Content string               `json:"content"`
		LinkURL string               `json:"link_url"`
		Media   []domain.NoticeMedia `json:"media"`
		Level   string               `json:"level"`
		Pinned  bool                 `json:"pinned"`
		Enabled bool                 `json:"enabled"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.UpdateNotice(currentUser(r).ID, id, input.Title, input.Content, input.LinkURL, input.Level, input.Pinned, input.Enabled, input.Media)
	if err != nil {
		writeError(w, 400, "notice_update_failed", err.Error())
		return
	}
	writeJSON(w, 200, item)
}

func (s *Server) deleteAdminNotice(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "noticeID"), 10, 64)
	if err := s.store.DeleteNotice(currentUser(r).ID, id); err != nil {
		writeError(w, 400, "notice_delete_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"deleted": true})
}

func (s *Server) pinPost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "postID"), 10, 64)
	var input struct {
		Pinned bool `json:"pinned"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.SetPostPinned(currentUser(r).ID, id, input.Pinned)
	if err != nil {
		writeError(w, 400, "pin_failed", err.Error())
		return
	}
	writeJSON(w, 200, item)
}

func (s *Server) myWallet(w http.ResponseWriter, r *http.Request) {
	result, err := s.store.UserWalletSummary(currentUser(r).ID, 40)
	if err != nil {
		writeError(w, 500, "wallet_failed", "钱包加载失败")
		return
	}
	writeJSON(w, 200, result)
}
