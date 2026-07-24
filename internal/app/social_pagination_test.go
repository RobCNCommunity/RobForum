package app

import (
	"net/http/httptest"
	"testing"
)

func TestPaginationParams(t *testing.T) {
	request := httptest.NewRequest("GET", "/?limit=30&offset=60", nil)
	limit, offset := paginationParams(request, 20, 50)
	if limit != 30 || offset != 60 {
		t.Fatalf("paginationParams() = (%d, %d), want (30, 60)", limit, offset)
	}
}

func TestPaginationParamsClampsInvalidInput(t *testing.T) {
	request := httptest.NewRequest("GET", "/?limit=999&offset=-1", nil)
	limit, offset := paginationParams(request, 30, 50)
	if limit != 30 || offset != 0 {
		t.Fatalf("paginationParams() = (%d, %d), want (30, 0)", limit, offset)
	}
}
