package store

import "testing"

func TestSearchCommunityEmptyQuery(t *testing.T) {
	result, err := (&Store{}).SearchCommunity("   ")
	if err != nil {
		t.Fatalf("empty search returned error: %v", err)
	}
	if result.Query != "" || len(result.Posts) != 0 || len(result.Resources) != 0 || len(result.Users) != 0 {
		t.Fatalf("empty search result = %#v", result)
	}
}
