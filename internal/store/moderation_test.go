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

func TestCanDeletePost(t *testing.T) {
	tests := []struct {
		name              string
		actorID, authorID int64
		status            string
		allowed           bool
	}{
		{name: "published author", actorID: 7, authorID: 7, status: "published", allowed: true},
		{name: "pending author", actorID: 7, authorID: 7, status: "pending", allowed: true},
		{name: "other user", actorID: 8, authorID: 7, status: "published", allowed: false},
		{name: "deleted post", actorID: 7, authorID: 7, status: "deleted", allowed: false},
		{name: "invalid actor", actorID: 0, authorID: 7, status: "published", allowed: false},
	}
	for _, test := range tests {
		if got := canDeletePost(test.actorID, test.authorID, test.status); got != test.allowed {
			t.Errorf("%s: canDeletePost(%d, %d, %q) = %v, want %v", test.name, test.actorID, test.authorID, test.status, got, test.allowed)
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
		memberExempt   bool
		want           string
	}{
		{true, "user", false, "pending"},
		{true, "creator", false, "pending"},
		{true, "admin", false, "published"},
		{true, "user", true, "published"},
		{true, "creator", true, "published"},
		{false, "user", false, "published"},
	}
	for _, test := range tests {
		if got := initialPostStatus(test.reviewRequired, test.role, test.memberExempt); got != test.want {
			t.Errorf("initialPostStatus(%v, %q, %v) = %q, want %q", test.reviewRequired, test.role, test.memberExempt, got, test.want)
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

func TestNormalizeContentReportReason(t *testing.T) {
	if got, err := normalizeContentReportReason("  冒充官方账号  "); err != nil || got != "冒充官方账号" {
		t.Fatalf("normalize report reason = %q, %v", got, err)
	}
	for _, reason := range []string{"", "a", "违规\x00内容", "删除符\x7f"} {
		if _, err := normalizeContentReportReason(reason); err == nil {
			t.Fatalf("report reason %q was accepted", reason)
		}
	}
}

func TestContentReportValidation(t *testing.T) {
	for _, targetType := range []string{contentReportTargetPost, contentReportTargetComment, contentReportTargetProfile} {
		if !validContentReportTargetType(targetType) {
			t.Errorf("target type %q rejected", targetType)
		}
	}
	if validContentReportTargetType("resource") {
		t.Fatal("unsupported report target type accepted")
	}
	for _, status := range []string{"", "pending", "accepted", "rejected"} {
		if !validContentReportStatusFilter(status) {
			t.Errorf("report status %q rejected", status)
		}
	}
	if validContentReportStatusFilter("hidden") {
		t.Fatal("unsupported report status accepted")
	}
}
