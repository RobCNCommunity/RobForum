package app

import (
	"archive/zip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"roblox-community/internal/store"
)

const maxResourceUpload = 100 << 20

var allowedResourceExtensions = map[string]struct{}{
	".zip": {}, ".pdf": {}, ".txt": {}, ".md": {}, ".json": {},
	".png": {}, ".jpg": {}, ".jpeg": {}, ".webp": {}, ".rbxl": {}, ".rbxlx": {}, ".rbxm": {},
}

var blockedArchiveExtensions = map[string]struct{}{
	".7z": {}, ".apk": {}, ".app": {}, ".bat": {}, ".cmd": {}, ".com": {}, ".dll": {}, ".dmg": {},
	".exe": {}, ".gz": {}, ".iso": {}, ".jar": {}, ".msi": {}, ".ps1": {}, ".rar": {}, ".scr": {},
	".sh": {}, ".tar": {}, ".vbs": {}, ".xz": {}, ".zip": {},
}

func (s *Server) listPublicResources(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListResources("approved", 0, 50)
	if err != nil {
		writeError(w, 500, "resources_failed", "资源列表加载失败")
		return
	}
	writeJSON(w, 200, items)
}

func (s *Server) getPublicResource(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "resourceID"), 10, 64)
	item, err := s.store.GetPublicResource(id)
	if err != nil {
		writeError(w, 404, "resource_not_found", "资源不存在或尚未通过审核")
		return
	}
	writeJSON(w, 200, item)
}

func (s *Server) listMyResources(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListResources("", currentUser(r).ID, 100)
	if err != nil {
		writeError(w, 500, "resources_failed", "资源列表加载失败")
		return
	}
	writeJSON(w, 200, items)
}

func (s *Server) listAdminResources(w http.ResponseWriter, r *http.Request) {
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "" {
		status = "pending"
	}
	items, err := s.store.ListResources(status, 0, 100)
	if err != nil {
		writeError(w, 500, "resources_failed", "审核资源加载失败")
		return
	}
	writeJSON(w, 200, items)
}

func (s *Server) createResource(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxResourceUpload+(2<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, 400, "upload_invalid", "上传内容过大或格式无效")
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "file_required", "请选择要投稿的资源文件")
		return
	}
	defer file.Close()

	originalName := filepath.Base(strings.TrimSpace(header.Filename))
	extension := strings.ToLower(filepath.Ext(originalName))
	if originalName == "" || len([]rune(originalName)) > 255 {
		writeError(w, 400, "filename_invalid", "文件名无效")
		return
	}
	if _, allowed := allowedResourceExtensions[extension]; !allowed {
		writeError(w, 400, "file_type_blocked", "当前文件类型不允许投稿")
		return
	}

	firstBytes := make([]byte, 512)
	readCount, readErr := io.ReadFull(file, firstBytes)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		writeError(w, 400, "file_read_failed", "资源文件读取失败")
		return
	}
	firstBytes = firstBytes[:readCount]
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, 400, "file_read_failed", "资源文件无法重新读取")
		return
	}
	mimeType := http.DetectContentType(firstBytes)
	if isExecutableContent(firstBytes, mimeType) {
		writeError(w, 400, "unsafe_file", "检测到可执行文件或高风险内容")
		return
	}
	if !resourceMIMEAllowed(extension, mimeType) {
		writeError(w, 400, "file_type_mismatch", "文件内容与扩展名不匹配")
		return
	}

	storedName, err := randomStoredName(extension)
	if err != nil {
		writeError(w, 500, "upload_failed", "资源文件名生成失败")
		return
	}
	targetPath := filepath.Join(s.uploadDir, storedName)
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		writeError(w, 500, "upload_failed", "资源文件保存失败")
		return
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(target, hash), io.LimitReader(file, maxResourceUpload+1))
	syncErr := target.Sync()
	closeErr := target.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil || written == 0 || written > maxResourceUpload {
		_ = os.Remove(targetPath)
		writeError(w, 400, "upload_failed", "资源文件为空、过大或保存失败")
		return
	}
	if extension == ".zip" {
		if err := validateZipArchive(targetPath, written); err != nil {
			_ = os.Remove(targetPath)
			writeError(w, 400, "unsafe_archive", err.Error())
			return
		}
	}

	priceCents := int64(0)
	if rawPrice := strings.TrimSpace(r.FormValue("price_cents")); rawPrice != "" {
		priceCents, err = strconv.ParseInt(rawPrice, 10, 64)
		if err != nil || priceCents < 0 {
			_ = os.Remove(targetPath)
			writeError(w, 400, "price_invalid", "资源价格无效")
			return
		}
	}
	item, err := s.store.CreateResource(currentUser(r).ID, r.FormValue("title"), r.FormValue("description"), r.FormValue("game"), r.FormValue("version"), r.FormValue("resource_type"), priceCents, store.ResourceFileInput{
		OriginalName: originalName,
		StoredName:   storedName,
		MIMEType:     mimeType,
		SizeBytes:    written,
		SHA256:       hex.EncodeToString(hash.Sum(nil)),
	})
	if err != nil {
		_ = os.Remove(targetPath)
		writeError(w, 400, "resource_create_failed", err.Error())
		return
	}
	writeJSON(w, 201, item)
}

func (s *Server) reviewResource(w http.ResponseWriter, r *http.Request) {
	resourceID, _ := strconv.ParseInt(chi.URLParam(r, "resourceID"), 10, 64)
	var input struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.ReviewResource(currentUser(r).ID, resourceID, input.Status, input.Reason)
	if err != nil {
		writeError(w, 400, "resource_review_failed", err.Error())
		return
	}
	writeJSON(w, 200, item)
}

func (s *Server) downloadResource(w http.ResponseWriter, r *http.Request) {
	resourceID, _ := strconv.ParseInt(chi.URLParam(r, "resourceID"), 10, 64)
	item, storedName, err := s.store.ResourceDownload(resourceID)
	if err != nil || item.Status != "approved" || item.File == nil {
		writeError(w, 404, "resource_not_found", "资源不存在或尚未通过审核")
		return
	}
	if storedName == "" || filepath.Base(storedName) != storedName {
		writeError(w, 410, "resource_file_invalid", "资源文件路径异常")
		return
	}
	if item.PriceCents > 0 {
		user, _, authErr := s.userBySessionCookies(r)
		if authErr != nil {
			writeError(w, http.StatusPaymentRequired, "resource_purchase_required", "购买资源后才能下载")
			return
		}
		if user.ID != item.CreatorID {
			purchased, purchaseErr := s.store.HasPurchasedResource(user.ID, resourceID)
			if purchaseErr != nil {
				writeError(w, 500, "purchase_check_failed", "购买状态校验失败")
				return
			}
			if !purchased {
				writeError(w, http.StatusPaymentRequired, "resource_purchase_required", "购买资源后才能下载")
				return
			}
		}
	}
	path := filepath.Join(s.uploadDir, storedName)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != item.File.SizeBytes {
		writeError(w, 410, "resource_file_missing", "资源文件不存在或校验异常")
		return
	}
	if err := verifyFileSHA256(path, item.File.SHA256); err != nil {
		writeError(w, 410, "resource_file_corrupted", "资源文件完整性校验失败")
		return
	}
	w.Header().Set("Content-Type", item.File.MIMEType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": item.File.OriginalName}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if err := s.store.IncrementResourceDownload(resourceID); err != nil {
		s.logger.Error("failed to increment resource download count", "resource_id", resourceID, "error", err)
	}
	http.ServeFile(w, r, path)
}

func verifyFileSHA256(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), strings.TrimSpace(expected)) {
		return errors.New("sha256 mismatch")
	}
	return nil
}

func randomStoredName(extension string) (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw) + extension, nil
}

func isExecutableContent(header []byte, mimeType string) bool {
	if len(header) >= 2 && header[0] == 'M' && header[1] == 'Z' {
		return true
	}
	if len(header) >= 4 && header[0] == 0x7f && string(header[1:4]) == "ELF" {
		return true
	}
	lower := strings.ToLower(mimeType)
	return strings.Contains(lower, "x-msdownload") || strings.Contains(lower, "x-executable") || strings.Contains(lower, "x-sh")
}

func resourceMIMEAllowed(extension, mimeType string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	switch extension {
	case ".png":
		return mimeType == "image/png"
	case ".jpg", ".jpeg":
		return mimeType == "image/jpeg"
	case ".webp":
		return mimeType == "image/webp"
	case ".pdf":
		return mimeType == "application/pdf"
	case ".zip":
		return mimeType == "application/zip" || mimeType == "application/x-zip-compressed" || mimeType == "application/octet-stream"
	case ".rbxl", ".rbxm":
		return mimeType == "application/octet-stream"
	case ".txt", ".md", ".json", ".rbxlx":
		return strings.HasPrefix(mimeType, "text/") || mimeType == "application/json" || mimeType == "application/xml" || mimeType == "application/octet-stream"
	default:
		return false
	}
}

func validateZipArchive(filename string, size int64) error {
	if size <= 0 || size > maxResourceUpload {
		return errors.New("压缩包大小无效")
	}
	archive, err := zip.OpenReader(filename)
	if err != nil {
		return errors.New("压缩包格式无效")
	}
	defer archive.Close()
	if len(archive.File) > 2000 {
		return errors.New("压缩包文件数量过多")
	}
	var totalUncompressed uint64
	for _, entry := range archive.File {
		name := strings.ReplaceAll(entry.Name, "\\", "/")
		pathName := strings.TrimSuffix(name, "/")
		cleanName := pathpkg.Clean("/" + pathName)
		if pathName == "" || strings.ContainsRune(name, 0) || strings.Contains(name, ":") || strings.HasPrefix(name, "/") || cleanName != "/"+pathName || strings.HasPrefix(cleanName, "/../") {
			return errors.New("压缩包包含非法路径")
		}
		if entry.Flags&0x1 != 0 {
			return errors.New("不允许加密压缩包")
		}
		if entry.FileInfo().Mode()&os.ModeSymlink != 0 {
			return errors.New("压缩包不允许包含符号链接")
		}
		extension := strings.ToLower(filepath.Ext(name))
		if _, blocked := blockedArchiveExtensions[extension]; blocked {
			return errors.New("压缩包包含高风险文件")
		}
		if entry.UncompressedSize64 > 500<<20 || totalUncompressed > (500<<20)-entry.UncompressedSize64 {
			return errors.New("压缩包解压后体积过大")
		}
		totalUncompressed += entry.UncompressedSize64
		if entry.CompressedSize64 > 0 && entry.UncompressedSize64 > 10<<20 && entry.UncompressedSize64/entry.CompressedSize64 > 200 {
			return errors.New("压缩包压缩比异常")
		}
		if entry.FileInfo().IsDir() {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			return errors.New("压缩包内容无法读取")
		}
		header := make([]byte, 512)
		readCount, readErr := io.ReadFull(reader, header)
		closeErr := reader.Close()
		if closeErr != nil || (readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF)) {
			return errors.New("压缩包内容无法读取")
		}
		if isExecutableContent(header[:readCount], http.DetectContentType(header[:readCount])) {
			return errors.New("压缩包包含可执行内容")
		}
	}
	return nil
}
