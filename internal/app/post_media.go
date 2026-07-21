package app

import (
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"roblox-community/internal/store"
)

const (
	maxPostMediaUpload = 5 << 20
	maxPostMediaCount  = 4
)

func (s *Server) createPostWithMedia(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPostMediaCount*maxPostMediaUpload+(4<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "post_upload_invalid", "帖子图片过大或表单格式无效")
		return
	}
	defer r.MultipartForm.RemoveAll()

	boardID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("board_id")), 10, 64)
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		files = r.MultipartForm.File["file"]
	}
	if len(files) > maxPostMediaCount {
		writeError(w, http.StatusBadRequest, "post_media_too_many", "每篇帖子最多上传 4 张图片")
		return
	}

	media := make([]store.PostMediaInput, 0, len(files))
	savedPaths := make([]string, 0, len(files))
	cleanup := func() {
		for _, path := range savedPaths {
			_ = os.Remove(path)
		}
	}
	for _, header := range files {
		item, path, err := s.savePostMedia(header)
		if err != nil {
			cleanup()
			writeError(w, http.StatusBadRequest, "post_media_invalid", err.Error())
			return
		}
		media = append(media, item)
		savedPaths = append(savedPaths, path)
	}

	item, err := s.store.CreatePostWithMedia(currentUser(r).ID, boardID, r.FormValue("title"), r.FormValue("content"), r.FormValue("post_type"), media)
	if err != nil {
		cleanup()
		writeError(w, http.StatusBadRequest, "post_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) savePostMedia(header *multipart.FileHeader) (store.PostMediaInput, string, error) {
	if header == nil || header.Size < 1 || header.Size > maxPostMediaUpload {
		return store.PostMediaInput{}, "", errors.New("图片为空或超过 5 MB")
	}
	extension := strings.ToLower(filepath.Ext(filepath.Base(header.Filename)))
	if extension != ".png" && extension != ".jpg" && extension != ".jpeg" {
		return store.PostMediaInput{}, "", errors.New("帖子图片仅支持 PNG 和 JPG")
	}
	file, err := header.Open()
	if err != nil {
		return store.PostMediaInput{}, "", errors.New("图片读取失败")
	}
	defer file.Close()
	firstBytes := make([]byte, 512)
	readCount, readErr := io.ReadFull(file, firstBytes)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return store.PostMediaInput{}, "", errors.New("图片读取失败")
	}
	firstBytes = firstBytes[:readCount]
	mimeType := http.DetectContentType(firstBytes)
	if (extension == ".png" && mimeType != "image/png") || ((extension == ".jpg" || extension == ".jpeg") && mimeType != "image/jpeg") {
		return store.PostMediaInput{}, "", errors.New("图片内容与扩展名不匹配")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return store.PostMediaInput{}, "", errors.New("图片无法重新读取")
	}
	config, _, err := image.DecodeConfig(io.LimitReader(file, maxPostMediaUpload+1))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 8192 || config.Height > 8192 {
		return store.PostMediaInput{}, "", errors.New("图片尺寸无效或超过 8192 × 8192")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return store.PostMediaInput{}, "", errors.New("图片无法重新读取")
	}
	storedName, err := randomStoredName(extension)
	if err != nil {
		return store.PostMediaInput{}, "", errors.New("图片文件名生成失败")
	}
	dir := filepath.Join(s.uploadDir, "posts")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return store.PostMediaInput{}, "", errors.New("图片目录创建失败")
	}
	targetPath := filepath.Join(dir, storedName)
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
	if err != nil {
		return store.PostMediaInput{}, "", errors.New("图片保存失败")
	}
	written, copyErr := io.Copy(target, io.LimitReader(file, maxPostMediaUpload+1))
	syncErr := target.Sync()
	closeErr := target.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil || written < 1 || written > maxPostMediaUpload {
		_ = os.Remove(targetPath)
		return store.PostMediaInput{}, "", errors.New("图片为空、过大或保存失败")
	}
	return store.PostMediaInput{StoredName: storedName, MIMEType: mimeType, Width: config.Width, Height: config.Height, SizeBytes: written}, targetPath, nil
}

func (s *Server) servePostMedia(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "filename")
	if localPostMediaName("/api/v1/media/posts/"+name) == "" {
		writeError(w, http.StatusNotFound, "post_media_not_found", "帖子图片不存在")
		return
	}
	viewerID := int64(0)
	isAdmin := false
	if user, _, err := s.userBySessionCookies(r); err == nil {
		viewerID = user.ID
		isAdmin = user.Role == "admin"
	}
	allowed, publiclyVisible, err := s.store.CanServePostMedia(name, viewerID, isAdmin)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("post media access check failed", "filename", name, "error", err)
		}
		writeError(w, http.StatusNotFound, "post_media_not_found", "帖子图片不存在")
		return
	}
	if !allowed {
		// Do not reveal whether a hidden or pending post exists.
		writeError(w, http.StatusNotFound, "post_media_not_found", "帖子图片不存在")
		return
	}
	path := filepath.Join(s.uploadDir, "posts", name)
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, "post_media_not_found", "帖子图片不存在")
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if publiclyVisible {
		w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	} else {
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Vary", "Cookie")
	}
	w.Header().Set("Content-Type", mime.TypeByExtension(strings.ToLower(filepath.Ext(name))))
	http.ServeFile(w, r, path)
}

func localPostMediaName(value string) string {
	const prefix = "/api/v1/media/posts/"
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
