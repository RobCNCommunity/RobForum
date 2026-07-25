package store

func validateCommentReplyTarget(parentPostID, postID int64) error {
	if parentPostID != postID {
		return ErrParentCommentUnavailable
	}
	return nil
}
