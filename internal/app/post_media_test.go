package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func multipartImageHeader(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = request.MultipartForm.RemoveAll() })
	return request.MultipartForm.File["files"][0]
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	imageData := image.NewRGBA(image.Rect(0, 0, 32, 24))
	imageData.Set(1, 1, color.RGBA{R: 29, G: 155, B: 240, A: 255})
	var content bytes.Buffer
	if err := png.Encode(&content, imageData); err != nil {
		t.Fatal(err)
	}
	return content.Bytes()
}

func TestSavePostMedia(t *testing.T) {
	server := &Server{uploadDir: t.TempDir()}
	input, path, err := server.savePostMedia(multipartImageHeader(t, "preview.png", testPNG(t)))
	if err != nil {
		t.Fatalf("valid image rejected: %v", err)
	}
	if input.MIMEType != "image/png" || input.Width != 32 || input.Height != 24 || input.SizeBytes < 1 {
		t.Fatalf("unexpected media metadata: %+v", input)
	}
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("saved image missing: %v", err)
	}
}

func TestSavePostMediaRejectsMismatchedExtension(t *testing.T) {
	server := &Server{uploadDir: t.TempDir()}
	if _, _, err := server.savePostMedia(multipartImageHeader(t, "preview.jpg", testPNG(t))); err == nil {
		t.Fatal("mismatched image extension accepted")
	}
}

func TestLocalPostMediaName(t *testing.T) {
	if got := localPostMediaName("/api/v1/media/posts/0123456789abcdef.png"); got != "0123456789abcdef.png" {
		t.Fatalf("valid media path rejected: %q", got)
	}
	for _, value := range []string{
		"/api/v1/media/posts/../secret.jpg",
		"/api/v1/media/posts/file.svg",
		"/media/posts/file.png",
	} {
		if got := localPostMediaName(value); got != "" {
			t.Fatalf("unsafe media path accepted: %q", value)
		}
	}
}
