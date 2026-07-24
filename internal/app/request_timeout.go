package app

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

func requestTimeoutExceptStreams(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		timed := middleware.Timeout(timeout)(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isConversationStreamRequest(r) {
				next.ServeHTTP(w, r)
				return
			}
			timed.ServeHTTP(w, r)
		})
	}
}

func isConversationStreamRequest(r *http.Request) bool {
	const prefix = "/api/v1/conversations/"
	const suffix = "/stream"
	if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, prefix) || !strings.HasSuffix(r.URL.Path, suffix) {
		return false
	}
	conversationID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), suffix)
	id, err := strconv.ParseUint(conversationID, 10, 64)
	return err == nil && id > 0
}
