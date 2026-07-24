package app

import (
	"os"
	"path/filepath"
	"testing"

	"roblox-community/internal/store"
)

func TestCleanupDeletedCommunityMediaFiles_removes_files_before_records(t *testing.T) {
	// Given
	uploadDir := t.TempDir()
	items := []store.DeletedCommunityMedia{
		{Kind: store.CommunityMediaPost, StoredName: "post.png"},
		{Kind: store.CommunityMediaComment, StoredName: "comment.webm"},
		{Kind: store.CommunityMediaPost, StoredName: "../outside.png"},
	}
	for _, item := range items[:2] {
		directory := filepath.Join(uploadDir, string(item.Kind)+"s")
		if err := os.MkdirAll(directory, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, item.StoredName), []byte("media"), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	deletedRecords := make([]store.DeletedCommunityMedia, 0)

	// When
	errs := cleanupDeletedCommunityMediaFiles(uploadDir, items, func(item store.DeletedCommunityMedia) error {
		deletedRecords = append(deletedRecords, item)
		return nil
	})

	// Then
	if len(errs) != 1 {
		t.Fatalf("cleanup errors = %d, want 1", len(errs))
	}
	if len(deletedRecords) != 2 {
		t.Fatalf("deleted records = %d, want 2", len(deletedRecords))
	}
	for _, item := range items[:2] {
		path := filepath.Join(uploadDir, string(item.Kind)+"s", item.StoredName)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("media file still exists: %s", path)
		}
	}
}
