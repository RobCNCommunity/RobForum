package store

import "database/sql"

func validateCommentReplyTarget(parentPostID, postID int64, parentParentID sql.NullInt64) error {
	if parentPostID != postID || parentParentID.Valid {
		return ErrParentCommentUnavailable
	}
	return nil
}
