package app

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResourceFileValidation(t *testing.T) {
	if !isExecutableContent([]byte{'M', 'Z', 0, 0}, "application/octet-stream") {
		t.Fatal("PE executable signature was not rejected")
	}
	if !isExecutableContent([]byte{0x7f, 'E', 'L', 'F'}, "application/octet-stream") {
		t.Fatal("ELF executable signature was not rejected")
	}
	if !resourceMIMEAllowed(".png", "image/png") {
		t.Fatal("valid PNG was rejected")
	}
	if resourceMIMEAllowed(".png", "application/x-msdownload") {
		t.Fatal("executable content was accepted as PNG")
	}
	if resourceMIMEAllowed(".exe", "application/octet-stream") {
		t.Fatal("blocked extension was accepted")
	}
}

func TestVerifyFileSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resource.bin")
	content := []byte("resource-content")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	if err := verifyFileSHA256(path, hex.EncodeToString(sum[:])); err != nil {
		t.Fatalf("valid file rejected: %v", err)
	}
	if err := verifyFileSHA256(path, strings.Repeat("0", 64)); err == nil {
		t.Fatal("corrupted file accepted")
	}
}

func TestRandomStoredNamePreservesSafeExtension(t *testing.T) {
	name, err := randomStoredName(".zip")
	if err != nil {
		t.Fatalf("randomStoredName() error = %v", err)
	}
	if len(name) != 44 || name[len(name)-4:] != ".zip" {
		t.Fatalf("randomStoredName() = %q", name)
	}
}

func TestValidateZipArchive(t *testing.T) {
	safePath := filepath.Join(t.TempDir(), "safe.zip")
	writeTestZip(t, safePath, "assets/readme.txt", []byte("safe content"))
	if info, err := os.Stat(safePath); err != nil {
		t.Fatal(err)
	} else if err := validateZipArchive(safePath, info.Size()); err != nil {
		t.Fatalf("safe archive rejected: %v", err)
	}

	for _, name := range []string{"../outside.txt", "payload.exe", "nested/archive.zip"} {
		path := filepath.Join(t.TempDir(), "unsafe.zip")
		writeTestZip(t, path, name, []byte("content"))
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateZipArchive(path, info.Size()); err == nil {
			t.Fatalf("unsafe archive entry %q was accepted", name)
		}
	}
}

func writeTestZip(t *testing.T, filename, name string, content []byte) {
	t.Helper()
	file, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	entry, err := archive.Create(name)
	if err != nil {
		file.Close()
		t.Fatal(err)
	}
	if _, err := entry.Write(content); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
