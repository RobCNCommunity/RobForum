package app

import (
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

const maxVerificationBadgeUpload = 2 << 20

func (s *Server) uploadVerificationBadge(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxVerificationBadgeUpload+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "verification_badge_invalid", "认证标志文件过大或格式无效")
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "verification_badge_required", "请选择认证标志图片")
		return
	}
	defer file.Close()

	extension := strings.ToLower(filepath.Ext(filepath.Base(header.Filename)))
	if extension != ".png" && extension != ".jpg" && extension != ".jpeg" {
		writeError(w, http.StatusBadRequest, "verification_badge_type_invalid", "认证标志仅支持 PNG 和 JPG")
		return
	}
	firstBytes := make([]byte, 512)
	readCount, readErr := io.ReadFull(file, firstBytes)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		writeError(w, http.StatusBadRequest, "verification_badge_read_failed", "认证标志读取失败")
		return
	}
	firstBytes = firstBytes[:readCount]
	mimeType := http.DetectContentType(firstBytes)
	if (extension == ".png" && mimeType != "image/png") || ((extension == ".jpg" || extension == ".jpeg") && mimeType != "image/jpeg") {
		writeError(w, http.StatusBadRequest, "verification_badge_type_mismatch", "图片内容与文件扩展名不匹配")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, http.StatusBadRequest, "verification_badge_read_failed", "认证标志无法重新读取")
		return
	}
	config, _, err := image.DecodeConfig(io.LimitReader(file, maxVerificationBadgeUpload+1))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 2048 || config.Height > 2048 {
		writeError(w, http.StatusBadRequest, "verification_badge_dimensions_invalid", "认证标志尺寸无效或超过 2048 × 2048")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, http.StatusBadRequest, "verification_badge_read_failed", "认证标志无法重新读取")
		return
	}

	storedName, err := randomStoredName(extension)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "verification_badge_upload_failed", "认证标志文件名生成失败")
		return
	}
	assetDir := filepath.Join(s.uploadDir, "site-assets")
	if err := os.MkdirAll(assetDir, 0750); err != nil {
		writeError(w, http.StatusInternalServerError, "verification_badge_upload_failed", "认证标志目录创建失败")
		return
	}
	targetPath := filepath.Join(assetDir, storedName)
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "verification_badge_upload_failed", "认证标志保存失败")
		return
	}
	written, copyErr := io.Copy(target, io.LimitReader(file, maxVerificationBadgeUpload+1))
	syncErr := target.Sync()
	closeErr := target.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil || written == 0 || written > maxVerificationBadgeUpload {
		_ = os.Remove(targetPath)
		writeError(w, http.StatusBadRequest, "verification_badge_upload_failed", "认证标志为空、过大或保存失败")
		return
	}

	oldSettings, err := s.store.GetSiteSettings()
	if err != nil {
		_ = os.Remove(targetPath)
		writeError(w, http.StatusInternalServerError, "settings_unavailable", "站点配置暂时不可用")
		return
	}
	next := oldSettings
	next.VerificationBadgeURL = "/api/v1/media/site-assets/" + storedName
	updated, err := s.store.UpdateSiteSettings(currentUser(r).ID, next)
	if err != nil {
		_ = os.Remove(targetPath)
		writeError(w, http.StatusInternalServerError, "verification_badge_update_failed", "认证标志配置更新失败")
		return
	}
	if oldName := localSiteAssetName(oldSettings.VerificationBadgeURL); oldName != "" && oldName != storedName {
		if err := os.Remove(filepath.Join(assetDir, oldName)); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.logger.Error("failed to remove replaced verification badge", "filename", oldName, "error", err)
		}
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) deleteVerificationBadge(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.GetSiteSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "settings_unavailable", "站点配置暂时不可用")
		return
	}
	oldName := localSiteAssetName(settings.VerificationBadgeURL)
	settings.VerificationBadgeURL = ""
	updated, err := s.store.UpdateSiteSettings(currentUser(r).ID, settings)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "verification_badge_update_failed", "认证标志配置更新失败")
		return
	}
	if oldName != "" {
		if err := os.Remove(filepath.Join(s.uploadDir, "site-assets", oldName)); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.logger.Error("failed to remove verification badge", "filename", oldName, "error", err)
		}
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) serveSiteAsset(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "filename")
	if localSiteAssetName("/api/v1/media/site-assets/"+name) == "" {
		writeError(w, http.StatusNotFound, "site_asset_not_found", "站点资源不存在")
		return
	}
	path := filepath.Join(s.uploadDir, "site-assets", name)
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, "site_asset_not_found", "站点资源不存在")
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	w.Header().Set("Content-Type", mime.TypeByExtension(strings.ToLower(filepath.Ext(name))))
	http.ServeFile(w, r, path)
}

func localSiteAssetName(value string) string {
	const prefix = "/api/v1/media/site-assets/"
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
