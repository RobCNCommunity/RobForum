package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestTimeoutExceptStreams_bypasses_only_conversation_WebSocket_streams(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		path         string
		wantDeadline bool
	}{
		{name: "conversation stream", method: http.MethodGet, path: "/api/v1/conversations/42/stream", wantDeadline: false},
		{name: "unrelated stream", method: http.MethodGet, path: "/api/v1/reports/stream", wantDeadline: true},
		{name: "invalid conversation id", method: http.MethodGet, path: "/api/v1/conversations/not-a-number/stream", wantDeadline: true},
		{name: "zero conversation id", method: http.MethodGet, path: "/api/v1/conversations/0/stream", wantDeadline: true},
		{name: "non GET conversation stream", method: http.MethodPost, path: "/api/v1/conversations/42/stream", wantDeadline: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			handler := requestTimeoutExceptStreams(time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, hasDeadline := r.Context().Deadline()
				if hasDeadline != tt.wantDeadline {
					t.Errorf("context deadline = %v, want %v", hasDeadline, tt.wantDeadline)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			request := httptest.NewRequest(tt.method, tt.path, nil)
			recorder := httptest.NewRecorder()

			// When
			handler.ServeHTTP(recorder, request)

			// Then
			if recorder.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
			}
		})
	}
}
