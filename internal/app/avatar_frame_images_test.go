package app

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestAvatarFrameUploadRoutesAreAdminOnly(t *testing.T) {
	routes, ok := New(nil, "", t.TempDir(), "", nil).Router().(chi.Routes)
	if !ok {
		t.Fatal("router does not expose chi routes")
	}

	registered := make(map[string]bool)
	if err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		registered[method+" "+route] = true
		return nil
	}); err != nil {
		t.Fatalf("walk routes: %v", err)
	}

	if !registered["POST /api/v1/admin/avatar-frames/image"] {
		t.Fatal("admin avatar frame image upload route is missing")
	}
	for _, route := range []string{
		"GET /api/v1/avatar-frame-upload-settings",
		"POST /api/v1/avatar-frames/image",
		"POST /api/v1/avatar-frame-submissions",
		"GET /api/v1/me/avatar-frame-submissions",
		"GET /api/v1/admin/avatar-frame-upload-settings",
		"PUT /api/v1/admin/avatar-frame-upload-settings",
		"GET /api/v1/admin/avatar-frame-submissions",
		"PATCH /api/v1/admin/avatar-frame-submissions/{submissionID}",
	} {
		if registered[route] {
			t.Fatalf("removed avatar frame submission route is still registered: %s", route)
		}
	}
}

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

func TestModerationImageBytesConvertsGIFFirstFrameToPNG(t *testing.T) {
	frame := image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.Transparent, color.RGBA{R: 255, A: 255}})
	frame.SetColorIndex(0, 0, 1)
	var source bytes.Buffer
	if err := gif.Encode(&source, frame, nil); err != nil {
		t.Fatal(err)
	}
	converted, err := moderationImageBytes("image/gif", source.Bytes())
	if err != nil {
		t.Fatalf("convert gif: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(converted)); err != nil {
		t.Fatalf("moderation payload is not PNG: %v", err)
	}
	if bytes.Equal(source.Bytes(), converted) {
		t.Fatal("GIF was passed to moderation without conversion")
	}
}
