package app

import "testing"

func TestLocalAvatarFrameImageName(t *testing.T) {
	valid := "/api/v1/media/avatar-frames/0123456789abcdef0123456789abcdef.png"
	if got := localAvatarFrameImageName(valid); got != "0123456789abcdef0123456789abcdef.png" {
		t.Fatalf("valid avatar frame image path rejected: %q", got)
	}
	for _, value := range []string{
		"/api/v1/media/avatar-frames/../secret.png",
		"/api/v1/media/avatar-frames/frame.svg",
		"/api/v1/media/avatar-frames/folder/frame.gif",
		"/api/v1/media/site-assets/frame.png",
		"",
	} {
		if got := localAvatarFrameImageName(value); got != "" {
			t.Fatalf("unsafe avatar frame image path accepted: %q", value)
		}
	}
}
