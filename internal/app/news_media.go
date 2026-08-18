package app

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"roblox-community/internal/domain"
)

func (s *Server) uploadAdminNewsImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCommunityImageUpload+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "news_image_invalid", "新闻图片过大或表单格式无效")
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "news_image_required", "请选择新闻图片")
		return
	}
	defer file.Close()
	if header == nil || !strings.HasPrefix(strings.ToLower(header.Header.Get("Content-Type")), "image/") {
		writeError(w, http.StatusBadRequest, "news_image_type_invalid", "新闻仅支持图片")
		return
	}
	item, storedPath, err := saveCommunityMedia(header, filepath.Join(s.uploadDir, "news"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "news_image_invalid", err.Error())
		return
	}
	if item.MIMEType != "image/png" && item.MIMEType != "image/jpeg" {
		_ = os.Remove(storedPath)
		writeError(w, http.StatusBadRequest, "news_image_type_invalid", "新闻仅支持 PNG 和 JPG 图片")
		return
	}
	writeJSON(w, http.StatusCreated, domain.NoticeMedia{
		URL: "/api/v1/media/news/" + item.StoredName, MIMEType: item.MIMEType,
		Width: item.Width, Height: item.Height, SizeBytes: item.SizeBytes,
	})
}

func (s *Server) serveNewsImage(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "filename")
	if localNewsImageName("/api/v1/media/news/"+name) == "" {
		writeError(w, http.StatusNotFound, "news_image_not_found", "新闻图片不存在")
		return
	}
	path := filepath.Join(s.uploadDir, "news", name)
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, "news_image_not_found", "新闻图片不存在")
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	w.Header().Set("Content-Type", mime.TypeByExtension(strings.ToLower(filepath.Ext(name))))
	http.ServeFile(w, r, path)
}

func localNewsImageName(value string) string {
	const prefix = "/api/v1/media/news/"
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
