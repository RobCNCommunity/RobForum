package store

import "testing"

func TestSearchCommunityEmptyQuery(t *testing.T) {
	result, err := (&Store{}).SearchCommunity("   ")
	if err != nil {
		t.Fatalf("empty search returned error: %v", err)
	}
	if result.Query != "" || len(result.Posts) != 0 || len(result.Guides) != 0 || len(result.Resources) != 0 || len(result.Users) != 0 {
		t.Fatalf("empty search result = %#v", result)
	}
}

func TestSearchPostsRejectsUnknownCategory(t *testing.T) {
	if _, err := (&Store{}).SearchPosts("keyword", "unknown", 10); err == nil {
		t.Fatal("unknown search category was accepted")
	}
}
