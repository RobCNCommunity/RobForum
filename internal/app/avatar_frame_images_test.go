package app

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"testing"
)

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
