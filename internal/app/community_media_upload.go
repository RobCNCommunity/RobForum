package app

import (
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxCommunityImageUpload = 5 << 20
	maxCommunityVideoUpload = 50 << 20
)

type savedCommunityMedia struct {
	StoredName string
	MIMEType   string
	Width      int
	Height     int
	SizeBytes  int64
}

func saveCommunityMedia(header *multipart.FileHeader, directory string) (savedCommunityMedia, string, error) {
	if header == nil || header.Size < 1 || header.Size > maxCommunityVideoUpload {
		return savedCommunityMedia{}, "", errors.New("媒体为空或超过 50 MB")
	}
	extension := strings.ToLower(filepath.Ext(filepath.Base(header.Filename)))
	file, err := header.Open()
	if err != nil {
		return savedCommunityMedia{}, "", errors.New("媒体读取失败")
	}
	defer file.Close()

	firstBytes := make([]byte, 512)
	readCount, readErr := io.ReadFull(file, firstBytes)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return savedCommunityMedia{}, "", errors.New("媒体读取失败")
	}
	firstBytes = firstBytes[:readCount]
	mimeType, ok := detectCommunityMediaType(extension, firstBytes)
	if !ok {
		return savedCommunityMedia{}, "", errors.New("仅支持 PNG、JPG、MP4 和 WebM")
	}
	maxSize := int64(maxCommunityVideoUpload)
	if strings.HasPrefix(mimeType, "image/") {
		maxSize = maxCommunityImageUpload
	}
	if header.Size > maxSize {
		return savedCommunityMedia{}, "", errors.New("图片不能超过 5 MB，视频不能超过 50 MB")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return savedCommunityMedia{}, "", errors.New("媒体无法重新读取")
	}

	width, height := 0, 0
	if strings.HasPrefix(mimeType, "image/") {
		config, _, err := image.DecodeConfig(io.LimitReader(file, maxCommunityImageUpload+1))
		if err != nil || config.Width < 16 || config.Height < 16 || config.Width > 6000 || config.Height > 6000 {
			return savedCommunityMedia{}, "", errors.New("图片尺寸必须在 16 x 16 至 6000 x 6000 之间")
		}
		width, height = config.Width, config.Height
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return savedCommunityMedia{}, "", errors.New("媒体无法重新读取")
		}
	} else {
		if err := validateCommunityVideo(file, mimeType, header.Size); err != nil {
			return savedCommunityMedia{}, "", err
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return savedCommunityMedia{}, "", errors.New("媒体无法重新读取")
		}
	}

	storedName, err := randomStoredName(extension)
	if err != nil {
		return savedCommunityMedia{}, "", errors.New("媒体文件名生成失败")
	}
	if err := os.MkdirAll(directory, 0750); err != nil {
		return savedCommunityMedia{}, "", errors.New("媒体目录创建失败")
	}
	targetPath := filepath.Join(directory, storedName)
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
	if err != nil {
		return savedCommunityMedia{}, "", errors.New("媒体保存失败")
	}
	written, copyErr := io.Copy(target, io.LimitReader(file, maxSize+1))
	syncErr := target.Sync()
	closeErr := target.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil || written < 1 || written > maxSize {
		_ = os.Remove(targetPath)
		return savedCommunityMedia{}, "", errors.New("媒体为空、过大或保存失败")
	}
	return savedCommunityMedia{
		StoredName: storedName,
		MIMEType:   mimeType,
		Width:      width,
		Height:     height,
		SizeBytes:  written,
	}, targetPath, nil
}

func detectCommunityMediaType(extension string, firstBytes []byte) (string, bool) {
	detected := http.DetectContentType(firstBytes)
	switch extension {
	case ".png":
		return "image/png", detected == "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg", detected == "image/jpeg"
	case ".mp4":
		valid := len(firstBytes) >= 12 && string(firstBytes[4:8]) == "ftyp"
		return "video/mp4", valid
	case ".webm":
		valid := len(firstBytes) >= 4 && firstBytes[0] == 0x1a && firstBytes[1] == 0x45 && firstBytes[2] == 0xdf && firstBytes[3] == 0xa3
		return "video/webm", valid
	default:
		return "", false
	}
}

func isImageMedia(mimeType string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(mimeType)), "image/")
}

func communityMediaContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	default:
		return "application/octet-stream"
	}
}
