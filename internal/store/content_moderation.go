package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

var contentModerationTypes = map[string]struct{}{
	"comment":                  {},
	"conversation":             {},
	"message":                  {},
	"post":                     {},
	"profile":                  {},
	"resource":                 {},
	"verification_application": {},
}

var contentModerationDecisions = map[string]struct{}{
	"allowed":     {},
	"blocked":     {},
	"unavailable": {},
}

// RecordContentModeration stores a hash and decision only. It intentionally
// never persists the reviewed text, including private-message content.
func (s *Store) RecordContentModeration(actorID int64, contentType, content, decision, reason, model string) error {
	if actorID < 1 {
		return errors.New("content moderation actor is invalid")
	}
	contentType = strings.TrimSpace(contentType)
	if _, ok := contentModerationTypes[contentType]; !ok {
		return errors.New("content moderation type is invalid")
	}
	decision = strings.TrimSpace(decision)
	if _, ok := contentModerationDecisions[decision]; !ok {
		return errors.New("content moderation decision is invalid")
	}
	reason = strings.TrimSpace(reason)
	model = strings.TrimSpace(model)
	if model == "" || len([]rune(model)) > 128 || len([]rune(reason)) > 200 || containsControlCharacter(reason) || containsControlCharacter(model) {
		return errors.New("content moderation audit fields are invalid")
	}
	hash := sha256.Sum256([]byte(content))
	_, err := s.db.Exec(
		`INSERT INTO content_moderation_audits (actor_id, content_type, content_hash, decision, reason, model, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		actorID,
		contentType,
		hex.EncodeToString(hash[:]),
		decision,
		reason,
		model,
		time.Now().UTC(),
	)
	return err
}

func containsControlCharacter(value string) bool {
	return strings.IndexFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0
}
