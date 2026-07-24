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
	"strings"

	"github.com/go-chi/chi/v5"
	"roblox-community/internal/store"
)

const (
	maxCommentMediaUpload = 5 << 20
	maxCommentMediaCount  = 4
)

func (s *Server) saveCommentMedia(header *multipart.FileHeader) (store.CommentMediaInput, string, error) {
	if header == nil || header.Size < 1 || header.Size > maxCommentMediaUpload {
		return store.CommentMediaInput{}, "", errors.New("图片为空或超过 5 MB")
	}
	file, err := header.Open()
	if err != nil {
		return store.CommentMediaInput{}, "", errors.New("图片读取失败")
	}
	defer file.Close()
	firstBytes := make([]byte, 512)
	readCount, readErr := io.ReadFull(file, firstBytes)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return store.CommentMediaInput{}, "", errors.New("图片读取失败")
	}
	firstBytes = firstBytes[:readCount]
	mimeType := http.DetectContentType(firstBytes)
	if mimeType != "image/png" && mimeType != "image/jpeg" {
		return store.CommentMediaInput{}, "", errors.New("评论图片仅支持 PNG 和 JPG")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return store.CommentMediaInput{}, "", errors.New("图片无法重新读取")
	}
	config, _, err := image.DecodeConfig(io.LimitReader(file, maxCommentMediaUpload+1))
	if err != nil || config.Width < 16 || config.Height < 16 || config.Width > 6000 || config.Height > 6000 {
		return store.CommentMediaInput{}, "", errors.New("图片尺寸必须在 16 × 16 至 6000 × 6000 之间")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return store.CommentMediaInput{}, "", errors.New("图片无法重新读取")
	}
	extension := ".jpg"
	if mimeType == "image/png" {
		extension = ".png"
	}
	storedName, err := randomStoredName(extension)
	if err != nil {
		return store.CommentMediaInput{}, "", errors.New("图片文件名生成失败")
	}
	dir := filepath.Join(s.uploadDir, "comments")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return store.CommentMediaInput{}, "", errors.New("图片目录创建失败")
	}
	targetPath := filepath.Join(dir, storedName)
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
	if err != nil {
		return store.CommentMediaInput{}, "", errors.New("图片保存失败")
	}
	written, copyErr := io.Copy(target, io.LimitReader(file, maxCommentMediaUpload+1))
	syncErr := target.Sync()
	closeErr := target.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil || written < 1 || written > maxCommentMediaUpload {
		_ = os.Remove(targetPath)
		return store.CommentMediaInput{}, "", errors.New("图片为空、过大或保存失败")
	}
	return store.CommentMediaInput{StoredName: storedName, MIMEType: mimeType, Width: config.Width, Height: config.Height, SizeBytes: written}, targetPath, nil
}

func (s *Server) serveCommentMedia(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "filename")
	if localCommentMediaName(name) == "" {
		writeError(w, http.StatusNotFound, "comment_media_not_found", "评论图片不存在")
		return
	}
	viewerID := int64(0)
	isAdmin := false
	if user, _, err := s.userBySessionCookies(r); err == nil {
		viewerID = user.ID
		isAdmin = user.Role == "admin"
	}
	media, allowed, publiclyVisible, err := s.store.CommentMediaAccess(name, viewerID, isAdmin)
	if err != nil || !allowed {
		writeError(w, http.StatusNotFound, "comment_media_not_found", "评论图片不存在")
		return
	}
	path := filepath.Join(s.uploadDir, "comments", name)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != media.SizeBytes {
		writeError(w, http.StatusNotFound, "comment_media_not_found", "评论图片不存在")
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

func localCommentMediaName(name string) string {
	name = strings.TrimSpace(name)
	extension := strings.ToLower(filepath.Ext(name))
	if filepath.Base(name) != name || (extension != ".png" && extension != ".jpg") {
		return ""
	}
	return name
}
