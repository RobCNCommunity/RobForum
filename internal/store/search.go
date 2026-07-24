package store

import (
	"errors"
	"strings"

	"roblox-community/internal/domain"
)

const communitySearchLimit = 12

func (s *Store) SearchCommunity(query string) (domain.CommunitySearchResult, error) {
	query = strings.TrimSpace(query)
	result := domain.CommunitySearchResult{
		Query:     query,
		Posts:     make([]domain.Post, 0),
		Resources: make([]domain.Resource, 0),
		Users:     make([]domain.UserSearchResult, 0),
	}
	if query == "" {
		return result, nil
	}
	if len([]rune(query)) > 80 {
		return result, errors.New("search query is too long")
	}

	var err error
	if result.Posts, err = s.SearchPosts(query, communitySearchLimit); err != nil {
		return result, err
	}
	if result.Resources, err = s.SearchPublicResources(query, communitySearchLimit); err != nil {
		return result, err
	}
	if result.Users, err = s.SearchUsers(query, communitySearchLimit, false); err != nil {
		return result, err
	}
	return result, nil
}
