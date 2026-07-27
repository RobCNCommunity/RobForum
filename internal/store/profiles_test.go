package store

import "testing"

func TestNormalizeCustomUID(t *testing.T) {
	got, err := normalizeCustomUID("  Roblox_Player88  ")
	if err != nil {
		t.Fatalf("normalizeCustomUID returned error: %v", err)
	}
	if got != "roblox_player88" {
		t.Fatalf("normalizeCustomUID = %q, want %q", got, "roblox_player88")
	}
}

func TestNormalizeCustomUIDRejectsInvalidValues(t *testing.T) {
	tests := []string{
		"abc",
		"user-name",
		"用户1234",
		"contains space",
		"abcdefghijklmnopqrstuvwxyz1234567",
	}
	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			if _, err := normalizeCustomUID(value); err == nil {
				t.Fatalf("normalizeCustomUID(%q) unexpectedly succeeded", value)
			}
		})
	}
}
