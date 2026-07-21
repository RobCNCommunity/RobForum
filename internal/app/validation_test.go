package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateEmailAllowlist(t *testing.T) {
	if err := validateEmail("Player@GMAIL.com", []string{"gmail.com"}); err != nil {
		t.Fatalf("valid email rejected: %v", err)
	}
	if err := validateEmail("player@gmali.com", []string{"gmail.com"}); err == nil {
		t.Fatal("typo domain accepted")
	}
	if err := validateEmail("not-an-email", nil); err == nil {
		t.Fatal("malformed email accepted")
	}
	if err := validateEmail(strings.Repeat("a", 65)+"@gmail.com", []string{"gmail.com"}); err == nil {
		t.Fatal("overlong email local part accepted")
	}
}

func TestDecodeJSONRejectsUnknownAndTrailingValues(t *testing.T) {
	for _, body := range []string{`{"known":"ok","unknown":true}`, `{"known":"ok"} {"known":"again"}`} {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		var input struct {
			Known string `json:"known"`
		}
		if decodeJSON(recorder, request, &input) || recorder.Code != http.StatusBadRequest {
			t.Fatalf("invalid JSON accepted: %s", body)
		}
	}
}
