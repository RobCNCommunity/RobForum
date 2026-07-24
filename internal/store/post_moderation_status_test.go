package store

import "testing"

func TestMachineApprovedPostPublishesWithoutManualReview(t *testing.T) {
	if got := machineModeratedPostStatus(true); got != "published" {
		t.Fatalf("machine-approved post status = %q, want published", got)
	}
}

func TestMachineReviewFailureWaitsForManualReview(t *testing.T) {
	if got := machineModeratedPostStatus(false); got != "pending" {
		t.Fatalf("machine-review failure status = %q, want pending", got)
	}
}
