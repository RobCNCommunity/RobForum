package store

import (
	"database/sql"
	"errors"
	"testing"
)

func TestValidateCommentReplyTarget_allows_only_root_comments_on_the_same_post(t *testing.T) {
	tests := []struct {
		name           string
		parentPostID   int64
		postID         int64
		parentParentID sql.NullInt64
		wantErr        bool
	}{
		{name: "root comment", parentPostID: 7, postID: 7},
		{name: "comment from another post", parentPostID: 8, postID: 7, wantErr: true},
		{name: "reply to a reply", parentPostID: 7, postID: 7, parentParentID: sql.NullInt64{Int64: 4, Valid: true}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			err := validateCommentReplyTarget(tt.parentPostID, tt.postID, tt.parentParentID)

			// Then
			if tt.wantErr && !errors.Is(err, ErrParentCommentUnavailable) {
				t.Fatalf("error = %v, want %v", err, ErrParentCommentUnavailable)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("error = %v, want nil", err)
			}
		})
	}
}
