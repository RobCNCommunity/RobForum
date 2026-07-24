package store

import (
	"reflect"
	"testing"
)

func TestNormalizeConversationMemberIDs(t *testing.T) {
	got := normalizeConversationMemberIDs(7, []int64{9, 0, 7, 3, 9, -1, 5})
	want := []int64{3, 5, 9}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeConversationMemberIDs() = %v, want %v", got, want)
	}
}

func TestConversationInviteTokenIsOpaqueAndHashable(t *testing.T) {
	token, storedHash, err := newConversationInviteToken()
	if err != nil {
		t.Fatalf("newConversationInviteToken() error = %v", err)
	}
	if len(token) < 32 {
		t.Fatalf("token is unexpectedly short: %d", len(token))
	}
	computedHash, err := conversationInviteTokenHash(token)
	if err != nil {
		t.Fatalf("conversationInviteTokenHash() error = %v", err)
	}
	if computedHash != storedHash {
		t.Fatalf("hash mismatch: got %q want %q", computedHash, storedHash)
	}
	if token == storedHash {
		t.Fatal("raw token must not be stored as its hash")
	}
}

func TestConversationInviteTokenRejectsMalformedValue(t *testing.T) {
	if _, err := conversationInviteTokenHash("short"); err == nil {
		t.Fatal("expected malformed invite token to be rejected")
	}
}
