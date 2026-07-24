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
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

const (
	maxAvatarUpload = 5 << 20
	maxCoverUpload  = 7 << 20
)

func (s *Server) getUserProfile(w http.ResponseWriter, r *http.Request) {
	userID, _ := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if userID < 1 {
		writeError(w, 404, "user_not_found", "用户不存在")
		return
	}
	profile, err := s.store.GetUserProfile(userID)
	if err != nil {
		writeError(w, 404, "user_not_found", "用户不存在")
		return
	}
	writeJSON(w, 200, profile)
}

func (s *Server) updateMyProfile(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DisplayName string `json:"display_name"`
		Bio         string `json:"bio"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if !s.approveContent(w, r, "profile", moderationText("昵称："+input.DisplayName, "简介："+input.Bio)) {
		return
	}
	user, err := s.store.UpdateUserProfile(currentUser(r).ID, input.DisplayName, input.Bio)
	if err != nil {
		writeError(w, 400, "profile_update_failed", err.Error())
		return
	}
	writeJSON(w, 200, user)
}

func (s *Server) uploadMyAvatar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarUpload+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, 400, "avatar_invalid", "头像文件过大或格式无效")
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "avatar_required", "请选择头像图片")
		return
	}
	defer file.Close()
	extension := strings.ToLower(filepath.Ext(filepath.Base(header.Filename)))
	expectedMIME, typeErr := expectedProfileImageMIME(extension, currentUser(r).MemberActive)
	if errors.Is(typeErr, errMemberGIFRequired) {
		writeError(w, http.StatusForbidden, "avatar_gif_membership_required", "GIF 动态头像仅限会员使用")
		return
	}
	if typeErr != nil {
		writeError(w, 400, "avatar_type_invalid", "头像仅支持 JPG、PNG，会员可使用 GIF")
		return
	}
	firstBytes := make([]byte, 512)
	readCount, readErr := io.ReadFull(file, firstBytes)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		writeError(w, 400, "avatar_read_failed", "头像读取失败")
		return
	}
	firstBytes = firstBytes[:readCount]
	mimeType := http.DetectContentType(firstBytes)
	if mimeType != expectedMIME {
		writeError(w, 400, "avatar_type_mismatch", "头像内容与文件扩展名不匹配")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, 400, "avatar_read_failed", "头像无法重新读取")
		return
	}
	config, _, err := image.DecodeConfig(io.LimitReader(file, maxAvatarUpload+1))
	if err != nil || config.Width < 16 || config.Height < 16 || config.Width > 6000 || config.Height > 6000 {
		writeError(w, 400, "avatar_dimensions_invalid", "头像尺寸必须在 16 × 16 至 6000 × 6000 之间")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, 400, "avatar_read_failed", "头像无法重新读取")
		return
	}
	storedName, err := randomStoredName(extension)
	if err != nil {
		writeError(w, 500, "avatar_upload_failed", "头像文件名生成失败")
		return
	}
	avatarDir := filepath.Join(s.uploadDir, "avatars")
	if err := os.MkdirAll(avatarDir, 0750); err != nil {
		writeError(w, 500, "avatar_upload_failed", "头像目录创建失败")
		return
	}
	targetPath := filepath.Join(avatarDir, storedName)
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
	if err != nil {
		writeError(w, 500, "avatar_upload_failed", "头像保存失败")
		return
	}
	written, copyErr := io.Copy(target, io.LimitReader(file, maxAvatarUpload+1))
	syncErr := target.Sync()
	closeErr := target.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil || written == 0 || written > maxAvatarUpload {
		_ = os.Remove(targetPath)
		writeError(w, 400, "avatar_upload_failed", "头像为空、过大或保存失败")
		return
	}
	if !s.approveImagePath(w, r, "profile", targetPath) {
		_ = os.Remove(targetPath)
		return
	}
	oldAvatar := currentUser(r).AvatarURL
	user, err := s.store.UpdateUserAvatar(currentUser(r).ID, "/api/v1/media/avatars/"+storedName)
	if err != nil {
		_ = os.Remove(targetPath)
		writeError(w, 500, "avatar_update_failed", "头像更新失败")
		return
	}
	if oldName := localAvatarName(oldAvatar); oldName != "" && oldName != storedName {
		if err := os.Remove(filepath.Join(avatarDir, oldName)); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.logger.Error("failed to remove replaced avatar", "filename", oldName, "error", err)
		}
	}
	writeJSON(w, 200, user)
}

func (s *Server) uploadMyCover(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCoverUpload+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "cover_invalid", "封面文件过大或格式无效")
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "cover_required", "请选择封面图片")
		return
	}
	defer file.Close()
	if header.Size < 1 || header.Size > maxCoverUpload {
		writeError(w, http.StatusBadRequest, "cover_size_invalid", "封面不能为空或超过 7 MB")
		return
	}
	extension := strings.ToLower(filepath.Ext(filepath.Base(header.Filename)))
	expectedMIME, typeErr := expectedProfileImageMIME(extension, currentUser(r).MemberActive)
	if errors.Is(typeErr, errMemberGIFRequired) {
		writeError(w, http.StatusForbidden, "cover_gif_membership_required", "GIF 动态背景仅限会员使用")
		return
	}
	if typeErr != nil {
		writeError(w, http.StatusBadRequest, "cover_type_invalid", "封面仅支持 JPG、PNG，会员可使用 GIF")
		return
	}
	firstBytes := make([]byte, 512)
	readCount, readErr := io.ReadFull(file, firstBytes)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		writeError(w, http.StatusBadRequest, "cover_read_failed", "封面读取失败")
		return
	}
	firstBytes = firstBytes[:readCount]
	mimeType := http.DetectContentType(firstBytes)
	if mimeType != expectedMIME {
		writeError(w, http.StatusBadRequest, "cover_type_mismatch", "封面内容与文件扩展名不匹配")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, http.StatusBadRequest, "cover_read_failed", "封面无法重新读取")
		return
	}
	config, _, err := image.DecodeConfig(io.LimitReader(file, maxCoverUpload+1))
	if err != nil || config.Width < 16 || config.Height < 16 || config.Width > 6000 || config.Height > 6000 {
		writeError(w, http.StatusBadRequest, "cover_dimensions_invalid", "封面尺寸必须在 16 × 16 至 6000 × 6000 之间")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, http.StatusBadRequest, "cover_read_failed", "封面无法重新读取")
		return
	}
	storedName, err := randomStoredName(extension)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cover_upload_failed", "封面文件名生成失败")
		return
	}
	coverDir := filepath.Join(s.uploadDir, "covers")
	if err := os.MkdirAll(coverDir, 0750); err != nil {
		writeError(w, http.StatusInternalServerError, "cover_upload_failed", "封面目录创建失败")
		return
	}
	targetPath := filepath.Join(coverDir, storedName)
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cover_upload_failed", "封面保存失败")
		return
	}
	written, copyErr := io.Copy(target, io.LimitReader(file, maxCoverUpload+1))
	syncErr := target.Sync()
	closeErr := target.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil || written == 0 || written > maxCoverUpload {
		_ = os.Remove(targetPath)
		writeError(w, http.StatusBadRequest, "cover_upload_failed", "封面为空、过大或保存失败")
		return
	}
	if !s.approveImagePath(w, r, "profile", targetPath) {
		_ = os.Remove(targetPath)
		return
	}
	oldCover := currentUser(r).CoverURL
	user, err := s.store.UpdateUserCover(currentUser(r).ID, "/api/v1/media/covers/"+storedName)
	if err != nil {
		_ = os.Remove(targetPath)
		writeError(w, http.StatusInternalServerError, "cover_update_failed", "封面更新失败")
		return
	}
	if oldName := localCoverName(oldCover); oldName != "" && oldName != storedName {
		if err := os.Remove(filepath.Join(coverDir, oldName)); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.logger.Error("failed to remove replaced cover", "filename", oldName, "error", err)
		}
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) deleteMyCover(w http.ResponseWriter, r *http.Request) {
	oldName := localCoverName(currentUser(r).CoverURL)
	user, err := s.store.UpdateUserCover(currentUser(r).ID, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cover_delete_failed", "封面移除失败")
		return
	}
	if oldName != "" {
		if err := os.Remove(filepath.Join(s.uploadDir, "covers", oldName)); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.logger.Error("failed to remove deleted cover", "filename", oldName, "error", err)
		}
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) serveAvatar(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "filename")
	if name == "" || filepath.Base(name) != name || localAvatarName("/api/v1/media/avatars/"+name) == "" {
		writeError(w, 404, "avatar_not_found", "头像不存在")
		return
	}
	path := filepath.Join(s.uploadDir, "avatars", name)
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		writeError(w, 404, "avatar_not_found", "头像不存在")
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	w.Header().Set("Content-Type", mime.TypeByExtension(strings.ToLower(filepath.Ext(name))))
	http.ServeFile(w, r, path)
}

func (s *Server) serveCover(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "filename")
	if name == "" || filepath.Base(name) != name || localCoverName("/api/v1/media/covers/"+name) == "" {
		writeError(w, http.StatusNotFound, "cover_not_found", "封面不存在")
		return
	}
	path := filepath.Join(s.uploadDir, "covers", name)
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, "cover_not_found", "封面不存在")
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	w.Header().Set("Content-Type", mime.TypeByExtension(strings.ToLower(filepath.Ext(name))))
	http.ServeFile(w, r, path)
}

func localAvatarName(value string) string {
	const prefix = "/api/v1/media/avatars/"
	if !strings.HasPrefix(value, prefix) {
		return ""
	}
	name := strings.TrimPrefix(value, prefix)
	extension := strings.ToLower(filepath.Ext(name))
	if filepath.Base(name) != name || (extension != ".jpg" && extension != ".jpeg" && extension != ".png" && extension != ".gif") {
		return ""
	}
	return name
}

func localCoverName(value string) string {
	const prefix = "/api/v1/media/covers/"
	if !strings.HasPrefix(value, prefix) {
		return ""
	}
	name := strings.TrimPrefix(value, prefix)
	extension := strings.ToLower(filepath.Ext(name))
	if filepath.Base(name) != name || (extension != ".jpg" && extension != ".jpeg" && extension != ".png" && extension != ".gif") {
		return ""
	}
	return name
}

func (s *Server) createVerificationApplication(w http.ResponseWriter, r *http.Request) {
	var input struct {
		VerificationType string `json:"verification_type"`
		RequestedLabel   string `json:"requested_label"`
		EvidenceURL      string `json:"evidence_url"`
		Statement        string `json:"statement"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if !s.approveContent(w, r, "verification_application", moderationText("认证名称："+input.RequestedLabel, "申请说明："+input.Statement)) {
		return
	}
	item, err := s.store.CreateVerificationApplication(currentUser(r).ID, input.VerificationType, input.RequestedLabel, input.EvidenceURL, input.Statement)
	if err != nil {
		writeError(w, 400, "verification_create_failed", err.Error())
		return
	}
	writeJSON(w, 201, item)
}

func (s *Server) listMyVerificationApplications(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListVerificationApplications(currentUser(r).ID, "")
	if err != nil {
		writeError(w, 500, "verification_list_failed", "认证申请加载失败")
		return
	}
	writeJSON(w, 200, items)
}

func (s *Server) listAdminVerificationApplications(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListVerificationApplications(0, r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, 400, "verification_list_failed", err.Error())
		return
	}
	writeJSON(w, 200, items)
}

func (s *Server) reviewVerificationApplication(w http.ResponseWriter, r *http.Request) {
	applicationID, _ := strconv.ParseInt(chi.URLParam(r, "applicationID"), 10, 64)
	var input struct {
		Status string `json:"status"`
		Label  string `json:"label"`
		Note   string `json:"note"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.ReviewVerificationApplication(currentUser(r).ID, applicationID, input.Status, input.Label, input.Note)
	if err != nil {
		writeError(w, 400, "verification_review_failed", err.Error())
		return
	}
	writeJSON(w, 200, item)
}
