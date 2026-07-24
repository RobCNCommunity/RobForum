package app

import (
	"bytes"
	"encoding/binary"
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

func TestDetectCommunityMediaType_recognizes_video_signatures(t *testing.T) {
	tests := []struct {
		name      string
		extension string
		content   []byte
		wantMIME  string
		wantOK    bool
	}{
		{name: "mp4", extension: ".mp4", content: []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}, wantMIME: "video/mp4", wantOK: true},
		{name: "webm", extension: ".webm", content: []byte{0x1a, 0x45, 0xdf, 0xa3}, wantMIME: "video/webm", wantOK: true},
		{name: "short mp4", extension: ".mp4", content: []byte{'f', 't', 'y', 'p'}, wantMIME: "video/mp4", wantOK: false},
		{name: "fake webm", extension: ".webm", content: []byte("not-webm"), wantMIME: "video/webm", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			gotMIME, gotOK := detectCommunityMediaType(tt.extension, tt.content)

			// Then
			if gotMIME != tt.wantMIME || gotOK != tt.wantOK {
				t.Fatalf("detectCommunityMediaType() = (%q, %v), want (%q, %v)", gotMIME, gotOK, tt.wantMIME, tt.wantOK)
			}
		})
	}
}

func TestValidateCommunityVideo_rejects_signature_only_files(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		content  []byte
	}{
		{name: "mp4", mimeType: "video/mp4", content: []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}},
		{name: "webm", mimeType: "video/webm", content: []byte{0x1a, 0x45, 0xdf, 0xa3, 0x80}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			reader := bytes.NewReader(test.content)

			// When
			err := validateCommunityVideo(reader, test.mimeType, int64(len(test.content)))

			// Then
			if err == nil {
				t.Fatal("signature-only video was accepted")
			}
		})
	}
}

func TestValidateCommunityVideo_accepts_bounded_video_containers(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		content  []byte
	}{
		{name: "mp4", mimeType: "video/mp4", content: minimalMP4Video()},
		{name: "webm", mimeType: "video/webm", content: minimalWebMVideo()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			reader := bytes.NewReader(test.content)

			// When
			err := validateCommunityVideo(reader, test.mimeType, int64(len(test.content)))

			// Then
			if err != nil {
				t.Fatalf("valid video container rejected: %v", err)
			}
		})
	}
}

func minimalMP4Video() []byte {
	ftypPayload := append([]byte("isom"), 0, 0, 0, 0)
	ftypPayload = append(ftypPayload, []byte("isom")...)
	hdlrPayload := make([]byte, 24)
	copy(hdlrPayload[8:12], "vide")
	stszPayload := make([]byte, 16)
	binary.BigEndian.PutUint32(stszPayload[8:12], 1)
	binary.BigEndian.PutUint32(stszPayload[12:16], 1)
	stbl := mp4TestBox("stbl", mp4TestBox("stsz", stszPayload))
	minf := mp4TestBox("minf", stbl)
	mdia := mp4TestBox("mdia", append(mp4TestBox("hdlr", hdlrPayload), minf...))
	trak := mp4TestBox("trak", mdia)
	return bytes.Join([][]byte{
		mp4TestBox("ftyp", ftypPayload),
		mp4TestBox("moov", trak),
		mp4TestBox("mdat", []byte{0x00}),
	}, nil)
}

func mp4TestBox(boxType string, payload []byte) []byte {
	box := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(box[:4], uint32(len(box)))
	copy(box[4:8], boxType)
	copy(box[8:], payload)
	return box
}

func minimalWebMVideo() []byte {
	header := ebmlTestElement([]byte{0x1a, 0x45, 0xdf, 0xa3}, ebmlTestElement([]byte{0x42, 0x82}, []byte("webm")))
	track := bytes.Join([][]byte{
		ebmlTestElement([]byte{0xd7}, []byte{0x01}),
		ebmlTestElement([]byte{0x73, 0xc5}, []byte{0x01}),
		ebmlTestElement([]byte{0x86}, []byte("V_VP8")),
		ebmlTestElement([]byte{0x83}, []byte{0x01}),
		ebmlTestElement([]byte{0xe0}, append(ebmlTestElement([]byte{0xb0}, []byte{0x10}), ebmlTestElement([]byte{0xba}, []byte{0x10})...)),
	}, nil)
	tracks := ebmlTestElement([]byte{0x16, 0x54, 0xae, 0x6b}, ebmlTestElement([]byte{0xae}, track))
	block := ebmlTestElement([]byte{0xa3}, []byte{0x81, 0x00, 0x00, 0x80, 0x00})
	cluster := ebmlTestElement([]byte{0x1f, 0x43, 0xb6, 0x75}, append(ebmlTestElement([]byte{0xe7}, []byte{0x00}), block...))
	segment := ebmlTestElement([]byte{0x18, 0x53, 0x80, 0x67}, append(tracks, cluster...))
	return append(header, segment...)
}

func ebmlTestElement(id, payload []byte) []byte {
	if len(payload) > 126 {
		panic("test EBML payload is too large")
	}
	result := make([]byte, 0, len(id)+1+len(payload))
	result = append(result, id...)
	result = append(result, byte(0x80|len(payload)))
	return append(result, payload...)
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
