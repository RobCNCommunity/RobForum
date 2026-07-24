package contentmoderation

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLiveStrictModeration(t *testing.T) {
	if os.Getenv("ROBLOX_CONTENT_MODERATION_LIVE_TEST") != "1" {
		t.Skip("set ROBLOX_CONTENT_MODERATION_LIVE_TEST=1 to call the configured provider")
	}
	client, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !client.Enabled() {
		t.Fatal("live moderation client is disabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	result, err := client.Review(ctx, "post", "这件事不能明说，说出来会出事，懂的都懂：\n广阔天空风很轻\n场边树影慢慢移\n事情留给后来人\n件件往事不敢提")
	if err != nil {
		t.Fatal(err)
	}
	if result.Model == "local_policy" {
		t.Fatal("live sample unexpectedly matched the local policy; provider was not exercised")
	}
	if !result.Violation {
		t.Fatalf("provider allowed covert sensitive content: %#v", result)
	}
}
