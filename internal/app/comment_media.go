package app

import (
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"roblox-community/internal/store"
)

const (
	maxCommentMediaUpload = maxCommunityVideoUpload
	maxCommentMediaCount  = 4
)

func (s *Server) saveCommentMedia(header *multipart.FileHeader) (store.CommentMediaInput, string, error) {
	item, path, err := saveCommunityMedia(header, filepath.Join(s.uploadDir, "comments"))
	if err != nil {
		return store.CommentMediaInput{}, "", err
	}
	return store.CommentMediaInput{
		StoredName: item.StoredName,
		MIMEType:   item.MIMEType,
		Width:      item.Width,
		Height:     item.Height,
		SizeBytes:  item.SizeBytes,
	}, path, nil
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
	w.Header().Set("Content-Type", media.MIMEType)
	http.ServeFile(w, r, path)
}

func localCommentMediaName(name string) string {
	name = strings.TrimSpace(name)
	extension := strings.ToLower(filepath.Ext(name))
	if filepath.Base(name) != name || (extension != ".png" && extension != ".jpg" && extension != ".jpeg" && extension != ".mp4" && extension != ".webm") {
		return ""
	}
	return name
}

func (s *Server) cleanupDeletedCommentMedia(storedNames []string) {
	items := make([]store.DeletedCommunityMedia, 0, len(storedNames))
	for _, storedName := range storedNames {
		items = append(items, store.DeletedCommunityMedia{Kind: store.CommunityMediaComment, StoredName: storedName})
	}
	s.cleanupDeletedCommunityMedia(items)
}
