package app

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"roblox-community/internal/domain"
)

func (s *Server) listPublicRobloxNews(w http.ResponseWriter, _ *http.Request) {
	result, err := s.store.ListPublicRobloxNews(20)
	if err != nil {
		writeError(w, 500, "roblox_news_failed", "新闻加载失败")
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) getPublicRobloxNews(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "newsID")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, 400, "invalid_id", "新闻 ID 无效")
		return
	}
	item, err := s.store.GetRobloxNews(id)
	if err != nil {
		writeError(w, 404, "not_found", "新闻不存在或已删除")
		return
	}
	if !item.Enabled {
		writeError(w, 404, "not_found", "新闻不存在或已删除")
		return
	}
	writeJSON(w, 200, item)
}

func (s *Server) listAdminRobloxNews(w http.ResponseWriter, _ *http.Request) {
	result, err := s.store.ListAdminRobloxNews()
	if err != nil {
		writeError(w, 500, "roblox_news_failed", "新闻加载失败")
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) createAdminRobloxNews(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title   string                   `json:"title"`
		Content string                   `json:"content"`
		LinkURL string                   `json:"link_url"`
		Media   []domain.RobloxNewsMedia `json:"media"`
		Enabled bool                     `json:"enabled"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.CreateRobloxNews(currentUser(r).ID, input.Title, input.Content, input.LinkURL, input.Enabled, input.Media)
	if err != nil {
		writeError(w, 400, "roblox_news_create_failed", err.Error())
		return
	}
	writeJSON(w, 201, item)
}

func (s *Server) updateAdminRobloxNews(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "newsID"), 10, 64)
	var input struct {
		Title   string                   `json:"title"`
		Content string                   `json:"content"`
		LinkURL string                   `json:"link_url"`
		Media   []domain.RobloxNewsMedia `json:"media"`
		Enabled bool                     `json:"enabled"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.UpdateRobloxNews(currentUser(r).ID, id, input.Title, input.Content, input.LinkURL, input.Enabled, input.Media)
	if err != nil {
		writeError(w, 400, "roblox_news_update_failed", err.Error())
		return
	}
	writeJSON(w, 200, item)
}

func (s *Server) deleteAdminRobloxNews(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "newsID"), 10, 64)
	if err := s.store.DeleteRobloxNews(currentUser(r).ID, id); err != nil {
		writeError(w, 400, "roblox_news_delete_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"deleted": true})
}

func (s *Server) uploadAdminRobloxNewsImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCommunityImageUpload+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "roblox_news_image_invalid", "新闻图片过大或表单格式无效")
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "roblox_news_image_required", "请选择新闻图片")
		return
	}
	defer file.Close()
	if header == nil || !strings.HasPrefix(strings.ToLower(header.Header.Get("Content-Type")), "image/") {
		writeError(w, http.StatusBadRequest, "roblox_news_image_type_invalid", "新闻仅支持图片")
		return
	}
	item, storedPath, err := saveCommunityMedia(header, filepath.Join(s.uploadDir, "roblox-news"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "roblox_news_image_invalid", err.Error())
		return
	}
	if item.MIMEType != "image/png" && item.MIMEType != "image/jpeg" {
		_ = os.Remove(storedPath)
		writeError(w, http.StatusBadRequest, "roblox_news_image_type_invalid", "新闻仅支持 PNG 和 JPG 图片")
		return
	}
	writeJSON(w, http.StatusCreated, domain.RobloxNewsMedia{
		URL: "/api/v1/media/roblox-news/" + item.StoredName, MIMEType: item.MIMEType,
		Width: item.Width, Height: item.Height, SizeBytes: item.SizeBytes,
	})
}

func (s *Server) serveRobloxNewsImage(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "filename")
	if localRobloxNewsImageName("/api/v1/media/roblox-news/"+name) == "" {
		writeError(w, http.StatusNotFound, "roblox_news_image_not_found", "新闻图片不存在")
		return
	}
	path := filepath.Join(s.uploadDir, "roblox-news", name)
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, "roblox_news_image_not_found", "新闻图片不存在")
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	w.Header().Set("Content-Type", mime.TypeByExtension(strings.ToLower(filepath.Ext(name))))
	http.ServeFile(w, r, path)
}

func localRobloxNewsImageName(value string) string {
	const prefix = "/api/v1/media/roblox-news/"
	if !strings.HasPrefix(value, prefix) {
		return ""
	}
	name := strings.TrimPrefix(value, prefix)
	extension := strings.ToLower(filepath.Ext(name))
	if filepath.Base(name) != name || (extension != ".png" && extension != ".jpg" && extension != ".jpeg") {
		return ""
	}
	return name
}
