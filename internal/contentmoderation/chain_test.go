package contentmoderation

import (
	"context"
	"errors"
	"testing"
)

type stubModerationService struct {
	enabled      bool
	result       Result
	err          error
	calls        int
	imageEnabled bool
	imageResult  Result
	imageErr     error
	imageCalls   int
}

func (s *stubModerationService) Enabled() bool { return s.enabled }

func (s *stubModerationService) Review(context.Context, string, string) (Result, error) {
	s.calls++
	return s.result, s.err
}

func (s *stubModerationService) ImageEnabled() bool { return s.imageEnabled }

func (s *stubModerationService) ReviewImage(context.Context, string, []byte) (Result, error) {
	s.imageCalls++
	return s.imageResult, s.imageErr
}

func TestChainRequiresEveryProviderToPass(t *testing.T) {
	first := &stubModerationService{enabled: true, result: Result{Model: "grok"}}
	second := &stubModerationService{enabled: true, result: Result{Violation: true, Reason: "blocked", Model: "baidu"}}
	chain := NewChain(first, second)
	result, err := chain.Review(context.Background(), "post", "content")
	if err != nil || !result.Violation || result.Model != "grok+baidu" || first.calls != 1 || second.calls != 1 {
		t.Fatalf("unexpected chain result: %#v err=%v calls=%d/%d", result, err, first.calls, second.calls)
	}
}

func TestChainFailsClosedOnProviderError(t *testing.T) {
	providerErr := errors.New("provider failed")
	first := &stubModerationService{enabled: true, result: Result{Model: "grok"}}
	second := &stubModerationService{enabled: true, result: Result{Model: "baidu"}, err: providerErr}
	chain := NewChain(first, second)
	result, err := chain.Review(context.Background(), "message", "content")
	if !errors.Is(err, providerErr) || result.Model != "grok+baidu" {
		t.Fatalf("unexpected chain error: %#v %v", result, err)
	}
}

func TestChainIgnoresDisabledProviders(t *testing.T) {
	disabled := &stubModerationService{enabled: false, result: Result{Violation: true, Model: "disabled"}}
	enabled := &stubModerationService{enabled: true, result: Result{Model: "baidu"}}
	chain := NewChain(disabled, enabled)
	result, err := chain.Review(context.Background(), "comment", "content")
	if err != nil || result.Violation || result.Model != "baidu" || disabled.calls != 0 || enabled.calls != 1 {
		t.Fatalf("unexpected chain result: %#v err=%v calls=%d/%d", result, err, disabled.calls, enabled.calls)
	}
}

func TestChainRunsOnlyImageProvidersForImages(t *testing.T) {
	textOnly := &stubModerationService{enabled: true, result: Result{Violation: true, Model: "grok"}}
	imageProvider := &stubModerationService{imageEnabled: true, imageResult: Result{Model: baiduImageModel}}
	chain := NewChain(textOnly, imageProvider)
	if !chain.Enabled() || !chain.ImageEnabled() {
		t.Fatal("chain did not preserve independent text and image providers")
	}
	result, err := chain.ReviewImage(context.Background(), "post", []byte("image"))
	if err != nil || result.Violation || result.Model != baiduImageModel {
		t.Fatalf("unexpected image chain result: %#v, %v", result, err)
	}
	if textOnly.calls != 0 || textOnly.imageCalls != 0 || imageProvider.calls != 0 || imageProvider.imageCalls != 1 {
		t.Fatalf("unexpected calls: text=%d/%d image=%d/%d", textOnly.calls, textOnly.imageCalls, imageProvider.calls, imageProvider.imageCalls)
	}
}

func TestChainImageFailsClosed(t *testing.T) {
	providerErr := errors.New("image provider failed")
	provider := &stubModerationService{imageEnabled: true, imageResult: Result{Model: baiduImageModel}, imageErr: providerErr}
	chain := NewChain(provider)
	result, err := chain.ReviewImage(context.Background(), "comment", []byte("image"))
	if !errors.Is(err, providerErr) || result.Model != baiduImageModel {
		t.Fatalf("unexpected image chain error: %#v, %v", result, err)
	}
}
