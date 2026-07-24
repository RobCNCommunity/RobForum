package app

import (
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
	maxPostMediaUpload = maxCommunityVideoUpload
	maxPostMediaCount  = 4
)

func (s *Server) createPostWithMedia(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPostMediaCount*maxPostMediaUpload+(4<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "post_upload_invalid", "帖子媒体过大或表单格式无效")
		return
	}
	defer r.MultipartForm.RemoveAll()

	boardID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("board_id")), 10, 64)
	title := r.FormValue("title")
	content := r.FormValue("content")
	tags := r.MultipartForm.Value["tags"]
	machineApproved := s.postMachineApproved(r, moderationText("标题："+title, "正文："+content))
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		files = r.MultipartForm.File["file"]
	}
	if len(files) > maxPostMediaCount {
		writeError(w, http.StatusBadRequest, "post_media_too_many", "每篇帖子最多上传 4 个媒体文件")
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
		savedPaths = append(savedPaths, path)
		if isImageMedia(item.MIMEType) {
			imageApproved := s.postImageMachineApproved(r, path)
			machineApproved = machineApproved && imageApproved
		}
		media = append(media, item)
	}

	item, err := s.store.CreateMachineModeratedPostWithTagsAndMedia(currentUser(r).ID, boardID, title, content, tags, media, machineApproved)
	if err != nil {
		cleanup()
		writeError(w, http.StatusBadRequest, "post_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) savePostMedia(header *multipart.FileHeader) (store.PostMediaInput, string, error) {
	item, path, err := saveCommunityMedia(header, filepath.Join(s.uploadDir, "posts"))
	if err != nil {
		return store.PostMediaInput{}, "", err
	}
	return store.PostMediaInput{
		StoredName: item.StoredName,
		MIMEType:   item.MIMEType,
		Width:      item.Width,
		Height:     item.Height,
		SizeBytes:  item.SizeBytes,
	}, path, nil
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
	w.Header().Set("Content-Type", communityMediaContentType(name))
	http.ServeFile(w, r, path)
}

func localPostMediaName(value string) string {
	const prefix = "/api/v1/media/posts/"
	if !strings.HasPrefix(value, prefix) {
		return ""
	}
	name := strings.TrimPrefix(value, prefix)
	extension := strings.ToLower(filepath.Ext(name))
	if filepath.Base(name) != name || (extension != ".png" && extension != ".jpg" && extension != ".jpeg" && extension != ".mp4" && extension != ".webm") {
		return ""
	}
	return name
}
