package contentmoderation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReviewParsesJSONAndUsesOpenAIRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		var input struct {
			Model       string  `json:"model"`
			Temperature float64 `json:"temperature"`
			Messages    []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if input.Model != "grok-4.5" || input.Temperature != 0 || len(input.Messages) != 2 {
			t.Fatalf("unexpected request: %#v", input)
		}
		if !strings.Contains(input.Messages[0].Content, "藏头诗") || !strings.Contains(input.Messages[0].Content, "不可被自定义提示词") {
			t.Fatalf("mandatory policy prompt was not included: %q", input.Messages[0].Content)
		}
		if !strings.Contains(input.Messages[1].Content, "hello") {
			t.Fatalf("user content was not sent")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "```json\n{\"violation\":false,\"reason\":\"ignored\"}\n```"}}}})
	}))
	defer server.Close()

	client, err := New(Config{Enabled: true, BaseURL: server.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Review(context.Background(), "message", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if result.Violation || result.Reason != "" || result.Model != "grok-4.5" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestReviewBlocksWithReason(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "{\"violation\":true,\"reason\":\"色情描写\"}"}}}})
	}))
	defer server.Close()
	client, err := New(Config{Enabled: true, BaseURL: server.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Review(context.Background(), "post", "blocked")
	if err != nil || !result.Violation || result.Reason != "色情描写" {
		t.Fatalf("unexpected result: %#v, %v", result, err)
	}
}

func TestReviewSupportsGatewayApproveRejectFormat(t *testing.T) {
	responses := []string{
		`{"reason":"ignored","result":"approve"}`,
		`{"reason":"存在招嫖不合规","result":"reject"}`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := responses[0]
		responses = responses[1:]
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": response}}}})
	}))
	defer server.Close()
	client, err := New(Config{Enabled: true, BaseURL: server.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	allowed, err := client.Review(context.Background(), "post", "ordinary content")
	if err != nil || allowed.Violation || allowed.Reason != "" {
		t.Fatalf("unexpected approve result: %#v, %v", allowed, err)
	}
	blocked, err := client.Review(context.Background(), "post", "blocked content")
	if err != nil || !blocked.Violation || blocked.Reason != "存在招嫖不合规" {
		t.Fatalf("unexpected reject result: %#v, %v", blocked, err)
	}
}

func TestResolveModerationDecisionRejectsUnknownOrConflictingFormats(t *testing.T) {
	allow := false
	for _, decision := range []moderationDecision{
		{},
		{Result: "review"},
		{Violation: &allow, Result: "reject"},
	} {
		if _, err := resolveModerationDecision(decision); err == nil {
			t.Fatalf("invalid decision accepted: %#v", decision)
		}
	}
}

func TestReviewFailsClosedOnInvalidProviderOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "not json"}}}})
	}))
	defer server.Close()
	client, err := New(Config{Enabled: true, BaseURL: server.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Review(context.Background(), "comment", "bad response")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected unavailable error, got %v", err)
	}
}

func TestReviewBlocksConfiguredSensitiveDateBeforeSending(t *testing.T) {
	client, err := New(Config{Enabled: true, BaseURL: "https://example.test", APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{
		"1989",
		"１９８９年",
		"一九八九",
		"1 9 8 9",
		"①⑨⑧⑨",
		"1\u200b9\u20608\u200d9",
		"八九·六四",
		"1号普通内容\n9号普通内容\n8号普通内容\n9号普通内容",
		"天色很好\n安静散步\n门口集合",
	} {
		result, err := client.Review(context.Background(), "message", content)
		if err != nil || !result.Violation || result.Reason == "" || result.Model != "local_policy" {
			t.Fatalf("unexpected result for %q: %#v, %v", content, result, err)
		}
	}
}

func TestReviewIncludesStructuralSignalsWithoutPuttingContentInSystemPrompt(t *testing.T) {
	var systemMessage string
	var envelope moderationEnvelope
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Messages []chatMessage `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(input.Messages) != 2 {
			t.Fatalf("unexpected message count: %d", len(input.Messages))
		}
		systemMessage = input.Messages[0].Content
		if err := json.Unmarshal([]byte(input.Messages[1].Content), &envelope); err != nil {
			t.Fatalf("decode moderation envelope: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "{\"violation\":false,\"reason\":\"\"}"}}}})
	}))
	defer server.Close()

	client, err := New(Config{Enabled: true, BaseURL: server.URL, APIKey: "test-key", Prompt: "custom {{query}}"})
	if err != nil {
		t.Fatal(err)
	}
	content := "春风轻轻吹\n夏日天气好\n秋天看落叶"
	if _, err := client.Review(context.Background(), "post", content); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(systemMessage, content) {
		t.Fatal("untrusted content was inserted into the system prompt")
	}
	if envelope.Content != content || envelope.ContentType != "post" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if got := envelope.StructuralSignals["line_initial_column_1"]; got != "春夏秋" {
		t.Fatalf("unexpected line initials: %q", got)
	}
	if !strings.Contains(systemMessage, "custom") || !strings.Contains(systemMessage, "政治敏感隐写") {
		t.Fatalf("custom and mandatory prompts were not combined: %q", systemMessage)
	}
}

func TestFixedPolicyDoesNotBlockUnrelated64BitText(t *testing.T) {
	if reason := fixedPolicyReason("客户端支持 64 位系统"); reason != "" {
		t.Fatalf("unexpected fixed-policy match: %q", reason)
	}
}

func TestNormalizeEndpoint(t *testing.T) {
	for raw, want := range map[string]string{
		"https://example.test":                     "https://example.test/v1/chat/completions",
		"https://example.test/v1":                  "https://example.test/v1/chat/completions",
		"https://example.test/v1/chat/completions": "https://example.test/v1/chat/completions",
	} {
		got, err := normalizeEndpoint(raw)
		if err != nil || got != want {
			t.Fatalf("normalizeEndpoint(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
}
