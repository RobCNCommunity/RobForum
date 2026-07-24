package store

import (
	"errors"
	"slices"
	"testing"
)

func TestNormalizePostTags_normalizes_and_deduplicates_valid_tags(t *testing.T) {
	// Given
	input := []string{" #Roblox ", "攻略", "roblox", "Lua"}

	// When
	got, err := normalizePostTags(input)

	// Then
	if err != nil {
		t.Fatalf("normalize valid tags: %v", err)
	}
	want := []string{"roblox", "攻略", "lua"}
	if !slices.Equal(got, want) {
		t.Fatalf("normalized tags = %v, want %v", got, want)
	}
}

func TestNormalizePostTags_rejects_invalid_tags(t *testing.T) {
	tests := []struct {
		name string
		tags []string
	}{
		{name: "more than five", tags: []string{"a", "b", "c", "d", "e", "f"}},
		{name: "embedded whitespace", tags: []string{"role play"}},
		{name: "embedded hash", tags: []string{"roblox#studio"}},
		{name: "empty after prefix", tags: []string{"###"}},
		{name: "longer than twenty four runes", tags: []string{"一二三四五六七八九十一二三四五六七八九十一二三四五"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			_, err := normalizePostTags(tt.tags)

			// Then
			if !errors.Is(err, errPostTagsInvalid) {
				t.Fatalf("error = %v, want %v", err, errPostTagsInvalid)
			}
		})
	}
}
