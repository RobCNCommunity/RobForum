package app

import (
	"bytes"
	"image"
	_ "image/gif"
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

const maxAvatarFrameImageUpload = 5 << 20

var avatarFrameImageMIMEs = map[string]string{
	".gif":  "image/gif",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".png":  "image/png",
}

func (s *Server) uploadAvatarFrameImage(w http.ResponseWriter, r *http.Request) {
	canUpload, err := s.store.CanUploadAvatarFrame(currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "avatar_frame_settings_failed", "头像框上传设置加载失败")
		return
	}
	if !canUpload {
		writeError(w, http.StatusForbidden, "avatar_frame_upload_forbidden", "当前账号暂未开放头像框上传")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarFrameImageUpload+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "avatar_frame_image_invalid", "头像框图片过大或表单格式无效")
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "avatar_frame_image_required", "请选择头像框图片")
		return
	}
	defer file.Close()

	extension := strings.ToLower(filepath.Ext(filepath.Base(header.Filename)))
	expectedMIME := avatarFrameImageMIMEs[extension]
	if expectedMIME == "" {
		writeError(w, http.StatusBadRequest, "avatar_frame_image_type_invalid", "头像框图片仅支持 PNG、JPG 和 GIF")
		return
	}
	imageBytes, readErr := io.ReadAll(io.LimitReader(file, maxAvatarFrameImageUpload+1))
	if readErr != nil || len(imageBytes) < 1 || len(imageBytes) > maxAvatarFrameImageUpload {
		writeError(w, http.StatusBadRequest, "avatar_frame_image_read_failed", "头像框图片读取失败")
		return
	}
	if http.DetectContentType(imageBytes[:min(len(imageBytes), 512)]) != expectedMIME {
		writeError(w, http.StatusBadRequest, "avatar_frame_image_type_mismatch", "图片内容与文件扩展名不匹配")
		return
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(imageBytes))
	if err != nil || config.Width < 32 || config.Height < 32 || config.Width > 4096 || config.Height > 4096 {
		writeError(w, http.StatusBadRequest, "avatar_frame_image_dimensions_invalid", "头像框图片尺寸必须在 32 × 32 至 4096 × 4096 之间")
		return
	}

	storedName, err := randomStoredName(extension)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "avatar_frame_image_upload_failed", "头像框图片文件名生成失败")
		return
	}
	imageDir := filepath.Join(s.uploadDir, "avatar-frames")
	if err := os.MkdirAll(imageDir, 0750); err != nil {
		writeError(w, http.StatusInternalServerError, "avatar_frame_image_upload_failed", "头像框图片目录创建失败")
		return
	}
	targetPath := filepath.Join(imageDir, storedName)
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "avatar_frame_image_upload_failed", "头像框图片保存失败")
		return
	}
	written, copyErr := io.Copy(target, bytes.NewReader(imageBytes))
	syncErr := target.Sync()
	closeErr := target.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil || written == 0 || written > maxAvatarFrameImageUpload {
		_ = os.Remove(targetPath)
		writeError(w, http.StatusBadRequest, "avatar_frame_image_upload_failed", "头像框图片为空、过大或保存失败")
		return
	}
	moderationBytes, moderationErr := moderationImageBytes(expectedMIME, imageBytes)
	if moderationErr != nil {
		_ = os.Remove(targetPath)
		writeError(w, http.StatusBadRequest, "avatar_frame_image_invalid", "头像框图片无法解析")
		return
	}
	if !s.approveImageBytes(w, r, "avatar_frame", moderationBytes) {
		_ = os.Remove(targetPath)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"image_url": "/api/v1/media/avatar-frames/" + storedName})
}

func (s *Server) serveAvatarFrameImage(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "filename")
	if localAvatarFrameImageName("/api/v1/media/avatar-frames/"+name) == "" {
		writeError(w, http.StatusNotFound, "avatar_frame_image_not_found", "头像框图片不存在")
		return
	}
	path := filepath.Join(s.uploadDir, "avatar-frames", name)
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, "avatar_frame_image_not_found", "头像框图片不存在")
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	w.Header().Set("Content-Type", mime.TypeByExtension(strings.ToLower(filepath.Ext(name))))
	http.ServeFile(w, r, path)
}

func localAvatarFrameImageName(value string) string {
	const prefix = "/api/v1/media/avatar-frames/"
	if !strings.HasPrefix(value, prefix) {
		return ""
	}
	name := strings.TrimPrefix(value, prefix)
	extension := strings.ToLower(filepath.Ext(name))
	if filepath.Base(name) != name || avatarFrameImageMIMEs[extension] == "" {
		return ""
	}
	return name
}
