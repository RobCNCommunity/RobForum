package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecommendedPostsRequestAlwaysUsesAllBoards(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/posts?feed=for-you&board=general", nil)
	if !recommendedPostsRequested(request) {
		t.Fatal("for-you feed was not recognized")
	}
	if board := requestedPostBoard(request); board != "" {
		t.Fatalf("for-you feed retained board filter %q", board)
	}
}

func TestRequestedPostBoardKeepsBoardPageFilter(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/posts?board=%20guides%20", nil)
	if recommendedPostsRequested(request) {
		t.Fatal("board page was treated as recommended feed")
	}
	if board := requestedPostBoard(request); board != "guides" {
		t.Fatalf("board filter = %q, want guides", board)
	}
}

func TestRequestedPostBoardAllAliasRemovesFilter(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/posts?board=all", nil)
	if board := requestedPostBoard(request); board != "" {
		t.Fatalf("all alias retained board filter %q", board)
	}
}
