package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"roblox-community/internal/store"
)

const communityMediaCleanupBatch = 500

func cleanupDeletedCommunityMediaFiles(uploadDir string, items []store.DeletedCommunityMedia, deleteRecord func(store.DeletedCommunityMedia) error) []error {
	cleanupErrors := make([]error, 0)
	for _, item := range items {
		path, err := deletedCommunityMediaPath(uploadDir, item)
		if err != nil {
			cleanupErrors = append(cleanupErrors, err)
			continue
		}
		info, err := os.Lstat(path)
		if err == nil && !info.Mode().IsRegular() {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("community media is not a regular file: %s", path))
			continue
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("inspect community media: %w", err))
			continue
		}
		if err == nil {
			if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("remove community media: %w", removeErr))
				continue
			}
		}
		if err := deleteRecord(item); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete community media record: %w", err))
		}
	}
	return cleanupErrors
}

func deletedCommunityMediaPath(uploadDir string, item store.DeletedCommunityMedia) (string, error) {
	name := strings.TrimSpace(item.StoredName)
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\`) {
		return "", errors.New("community media filename is invalid")
	}
	var directory string
	switch item.Kind {
	case store.CommunityMediaPost:
		if localPostMediaName("/api/v1/media/posts/"+name) == "" {
			return "", errors.New("post media filename is invalid")
		}
		directory = "posts"
	case store.CommunityMediaComment:
		if localCommentMediaName(name) == "" {
			return "", errors.New("comment media filename is invalid")
		}
		directory = "comments"
	default:
		return "", errors.New("community media kind is invalid")
	}
	return filepath.Join(uploadDir, directory, name), nil
}

func (s *Server) cleanupDeletedCommunityMedia(items []store.DeletedCommunityMedia) []error {
	if s.store == nil {
		return nil
	}
	errs := cleanupDeletedCommunityMediaFiles(s.uploadDir, items, s.store.DeleteDeletedCommunityMediaRecord)
	for _, err := range errs {
		if s.logger != nil {
			s.logger.Error("community media cleanup failed", "error", err)
		}
	}
	return errs
}

func (s *Server) reconcileDeletedCommunityMedia() {
	if s.store == nil {
		return
	}
	for {
		items, err := s.store.ListDeletedCommunityMedia(communityMediaCleanupBatch)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("deleted community media reconciliation failed", "error", err)
			}
			return
		}
		if len(items) == 0 {
			return
		}
		if len(s.cleanupDeletedCommunityMedia(items)) > 0 || len(items) < communityMediaCleanupBatch {
			return
		}
	}
}
