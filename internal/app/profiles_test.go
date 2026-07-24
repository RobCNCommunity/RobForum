package app

import (
	"errors"
	"testing"
)

func TestLocalCoverName(t *testing.T) {
	if got := localCoverName("/api/v1/media/covers/0123456789abcdef.png"); got != "0123456789abcdef.png" {
		t.Fatalf("valid cover path rejected: %q", got)
	}
	for _, value := range []string{
		"/api/v1/media/covers/../secret.jpg",
		"/api/v1/media/covers/file.svg",
		"/api/v1/media/covers/folder/file.png",
		"/media/covers/file.png",
		"",
	} {
		if got := localCoverName(value); got != "" {
			t.Fatalf("unsafe cover path accepted: %q", value)
		}
	}
}

func TestExpectedProfileImageMIME_allows_GIF_only_for_active_members(t *testing.T) {
	// Given / When
	nonMemberMIME, nonMemberErr := expectedProfileImageMIME(".gif", false)
	memberMIME, memberErr := expectedProfileImageMIME(".GIF", true)

	// Then
	if nonMemberMIME != "" || !errors.Is(nonMemberErr, errMemberGIFRequired) {
		t.Fatalf("non-member result = (%q, %v), want membership error", nonMemberMIME, nonMemberErr)
	}
	if memberErr != nil || memberMIME != "image/gif" {
		t.Fatalf("member result = (%q, %v), want image/gif", memberMIME, memberErr)
	}
}
