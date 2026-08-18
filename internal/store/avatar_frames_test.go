package store

import (
	"testing"

	"roblox-community/internal/domain"
)

func TestAvatarFrameAllowedUsesExclusiveUserClass(t *testing.T) {
	frame := domain.AvatarFrame{AllowedRegular: true, AllowedMember: false, AllowedAdmin: false}
	if !avatarFrameAllowed(frame, avatarFrameUserContext{role: "user"}) {
		t.Fatal("regular user should be allowed")
	}
	if avatarFrameAllowed(frame, avatarFrameUserContext{role: "user", memberActive: true}) {
		t.Fatal("member should use the member permission, not the regular permission")
	}
	if avatarFrameAllowed(frame, avatarFrameUserContext{role: "admin", memberActive: true}) {
		t.Fatal("admin should use the admin permission")
	}
}

func TestAvatarFrameUploadAllowedOnlyForAdministrators(t *testing.T) {
	settings := domain.AvatarFrameUploadSettings{AllowRegularUpload: true, AllowMemberUpload: false}
	if avatarFrameUploadAllowed(settings, avatarFrameUserContext{role: "user"}) {
		t.Fatal("regular users should not have an avatar frame upload entry")
	}
	if avatarFrameUploadAllowed(settings, avatarFrameUserContext{role: "user", memberActive: true}) {
		t.Fatal("members should not have an avatar frame upload entry")
	}
	if !avatarFrameUploadAllowed(domain.AvatarFrameUploadSettings{}, avatarFrameUserContext{role: "admin", memberActive: true}) {
		t.Fatal("administrator upload should always be enabled")
	}
}

func TestNormalizeAvatarFrameRejectsInvalidConfiguration(t *testing.T) {
	base := domain.AvatarFrame{Name: "蓝色光环", Style: "glow", PrimaryColor: "#1d9bf0", SecondaryColor: "#8b5cf6", AllowedRegular: true, Enabled: true}
	if _, err := normalizeAvatarFrame(base); err != nil {
		t.Fatalf("valid avatar frame rejected: %v", err)
	}
	invalid := base
	invalid.PrimaryColor = "blue"
	if _, err := normalizeAvatarFrame(invalid); err == nil {
		t.Fatal("invalid color should be rejected")
	}
	invalid = base
	invalid.AllowedRegular = false
	if _, err := normalizeAvatarFrame(invalid); err == nil {
		t.Fatal("configuration with no eligible user class should be rejected")
	}
	invalid = base
	invalid.Style = "image"
	if _, err := normalizeAvatarFrame(invalid); err == nil {
		t.Fatal("image frame without an uploaded image should be rejected")
	}
	validImage := base
	validImage.Style = "image"
	validImage.ImageURL = "/api/v1/media/avatar-frames/0123456789abcdef0123456789abcdef.png"
	if _, err := normalizeAvatarFrame(validImage); err != nil {
		t.Fatalf("valid image frame rejected: %v", err)
	}
	absoluteImage := validImage
	absoluteImage.ImageURL = "https://robforum.cn/api/v1/media/avatar-frames/0123456789abcdef0123456789abcdef.png"
	if normalized, err := normalizeAvatarFrame(absoluteImage); err != nil || normalized.ImageURL != validImage.ImageURL {
		t.Fatalf("absolute local image URL was not normalized: %#v, %v", normalized, err)
	}
	invalidOffset := base
	invalidOffset.ImageOffsetX = 101
	if _, err := normalizeAvatarFrame(invalidOffset); err == nil {
		t.Fatal("out-of-range image offset should be rejected")
	}
}
