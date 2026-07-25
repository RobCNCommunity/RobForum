package domain

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestPostJSON_omits_legacy_post_type(t *testing.T) {
	// Given
	post := Post{}

	// When
	payload, err := json.Marshal(post)
	if err != nil {
		t.Fatalf("marshal post: %v", err)
	}

	// Then
	if bytes.Contains(payload, []byte(`"post_type"`)) {
		t.Fatalf("post JSON still exposes legacy type: %s", payload)
	}
}

func TestCommunitySearchResultJSON_omits_legacy_guide_partition(t *testing.T) {
	// Given
	result := CommunitySearchResult{}

	// When
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal community search result: %v", err)
	}

	// Then
	if bytes.Contains(payload, []byte(`"guides"`)) {
		t.Fatalf("community search JSON still exposes legacy guide type: %s", payload)
	}
}

func TestSiteSettingsJSON_exposes_login_policy_links(t *testing.T) {
	// Given
	settings := SiteSettings{
		UserAgreementURL: "/user-agreement",
		CookiesPolicyURL: "https://example.com/cookies",
	}

	// When
	payload, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal site settings: %v", err)
	}

	// Then
	if !bytes.Contains(payload, []byte(`"user_agreement_url":"/user-agreement"`)) {
		t.Fatalf("site settings JSON omits the user agreement URL: %s", payload)
	}
	if !bytes.Contains(payload, []byte(`"cookies_policy_url":"https://example.com/cookies"`)) {
		t.Fatalf("site settings JSON omits the cookies policy URL: %s", payload)
	}
}
