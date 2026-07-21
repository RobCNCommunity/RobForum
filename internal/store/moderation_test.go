package store

import "testing"

func TestNormalizeModerationReason(t *testing.T) {
	if got, err := normalizeModerationReason("  垃圾广告  ", true); err != nil || got != "垃圾广告" {
		t.Fatalf("normalize reason = %q, %v", got, err)
	}
	for _, reason := range []string{"", "a", "非法\x00字符", "删除符\x7f"} {
		if _, err := normalizeModerationReason(reason, true); err == nil {
			t.Fatalf("reason %q was accepted", reason)
		}
	}
	if _, err := normalizeModerationReason("", false); err != nil {
		t.Fatalf("optional empty reason rejected: %v", err)
	}
}

func TestCanModeratePost(t *testing.T) {
	tests := []struct {
		current string
		next    string
		allowed bool
	}{
		{"pending", "published", true},
		{"pending", "rejected", true},
		{"published", "hidden", true},
		{"hidden", "published", true},
		{"rejected", "published", true},
		{"deleted", "published", false},
		{"published", "published", false},
		{"unknown", "deleted", false},
	}
	for _, test := range tests {
		if got := canModeratePost(test.current, test.next); got != test.allowed {
			t.Errorf("canModeratePost(%q, %q) = %v, want %v", test.current, test.next, got, test.allowed)
		}
	}
}

func TestValidPostStatusFilter(t *testing.T) {
	for _, status := range []string{"", "pending", "published", "hidden", "rejected", "deleted"} {
		if !validPostStatusFilter(status) {
			t.Errorf("status %q rejected", status)
		}
	}
	if validPostStatusFilter("approved") {
		t.Fatal("unsupported status accepted")
	}
}

func TestInitialPostStatus(t *testing.T) {
	tests := []struct {
		reviewRequired bool
		role           string
		want           string
	}{
		{true, "user", "pending"},
		{true, "creator", "pending"},
		{true, "admin", "published"},
		{false, "user", "published"},
	}
	for _, test := range tests {
		if got := initialPostStatus(test.reviewRequired, test.role); got != test.want {
			t.Errorf("initialPostStatus(%v, %q) = %q, want %q", test.reviewRequired, test.role, got, test.want)
		}
	}
}

func TestModerationErrorKind(t *testing.T) {
	err := moderationError(ModerationErrorForbidden, "受保护账号")
	typed, ok := err.(*ModerationError)
	if !ok || typed.Kind != ModerationErrorForbidden || typed.Error() != "受保护账号" {
		t.Fatalf("unexpected moderation error: %#v", err)
	}
}
