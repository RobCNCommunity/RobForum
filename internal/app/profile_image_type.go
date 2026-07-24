package app

import (
	"errors"
	_ "image/gif"
	"strings"
)

var errMemberGIFRequired = errors.New("active membership is required for GIF profile images")

func expectedProfileImageMIME(extension string, memberActive bool) (string, error) {
	switch strings.ToLower(strings.TrimSpace(extension)) {
	case ".jpg", ".jpeg":
		return "image/jpeg", nil
	case ".png":
		return "image/png", nil
	case ".gif":
		if !memberActive {
			return "", errMemberGIFRequired
		}
		return "image/gif", nil
	default:
		return "", errors.New("unsupported profile image type")
	}
}
