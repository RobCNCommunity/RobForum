package app

import (
	"fmt"
	"testing"
)

func TestChatConnectionLimiter_rejects_connection_above_user_limit(t *testing.T) {
	// Given
	limiter := newChatConnectionLimiter()
	for index := 0; index < maxChatConnectionsPerUser; index++ {
		if !limiter.acquire(42, fmt.Sprintf("198.51.100.%d", index+1)) {
			t.Fatalf("connection %d within user limit was rejected", index+1)
		}
	}

	// When
	allowed := limiter.acquire(42, "203.0.113.1")

	// Then
	if allowed {
		t.Fatal("connection above per-user limit was accepted")
	}
}

func TestChatConnectionLimiter_rejects_connection_above_IP_limit(t *testing.T) {
	// Given
	limiter := newChatConnectionLimiter()
	for index := 0; index < maxChatConnectionsPerIP; index++ {
		if !limiter.acquire(int64(index+1), "198.51.100.10") {
			t.Fatalf("connection %d within IP limit was rejected", index+1)
		}
	}

	// When
	allowed := limiter.acquire(int64(maxChatConnectionsPerIP+1), "198.51.100.10")

	// Then
	if allowed {
		t.Fatal("connection above per-IP limit was accepted")
	}
}

func TestChatConnectionLimiter_release_restores_capacity(t *testing.T) {
	// Given
	limiter := newChatConnectionLimiter()
	for index := 0; index < maxChatConnectionsPerUser; index++ {
		if !limiter.acquire(42, fmt.Sprintf("198.51.100.%d", index+1)) {
			t.Fatalf("connection %d within user limit was rejected", index+1)
		}
	}
	limiter.release(42, "198.51.100.1")

	// When
	allowed := limiter.acquire(42, "203.0.113.1")

	// Then
	if !allowed {
		t.Fatal("released capacity was not reusable")
	}
}
