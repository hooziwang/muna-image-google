package cmd

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestMaskKey(t *testing.T) {
	if got := maskKey("short-key"); got != "short-key" {
		t.Fatalf("unexpected short key masking: %s", got)
	}

	key := "abcdefghijklmnop"
	got := maskKey(key)
	if got != "abcd...ijklmnop" {
		t.Fatalf("unexpected masked key: %s", got)
	}
}

func TestCheckKey_OK(t *testing.T) {
	t.Setenv("MUNA_IMAGE_GOOGLE_BASE_URL", "")

	originalTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "generativelanguage.googleapis.com" {
			t.Fatalf("unexpected host: %s", req.URL.Host)
		}
		if got := req.URL.Query().Get("key"); got != "k-ok" {
			t.Fatalf("unexpected key query: %s", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     make(http.Header),
		}, nil
	})
	defer func() { http.DefaultTransport = originalTransport }()

	ok, reason := checkKey(context.Background(), "k-ok", time.Second)
	if !ok {
		t.Fatalf("expected ok=true, got false reason=%s", reason)
	}
	if reason != "" {
		t.Fatalf("expected empty reason, got %s", reason)
	}
}

func TestCheckKey_UsesConfiguredBaseURL(t *testing.T) {
	t.Setenv("MUNA_IMAGE_GOOGLE_BASE_URL", "https://proxy.example.com/")

	originalTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.String(); got != "https://proxy.example.com/v1beta/models?key=k-ok" {
			t.Fatalf("unexpected request url: %s", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     make(http.Header),
		}, nil
	})
	defer func() { http.DefaultTransport = originalTransport }()

	ok, reason := checkKey(context.Background(), "k-ok", time.Second)
	if !ok {
		t.Fatalf("expected ok=true, got false reason=%s", reason)
	}
	if reason != "" {
		t.Fatalf("expected empty reason, got %s", reason)
	}
}

func TestCheckKey_APIErrorDetails(t *testing.T) {
	t.Setenv("MUNA_IMAGE_GOOGLE_BASE_URL", "")

	originalTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		body := `{"error":{"code":401,"message":"bad key","status":"UNAUTHENTICATED","details":[{"reason":"API_KEY_INVALID"}]}}`
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})
	defer func() { http.DefaultTransport = originalTransport }()

	ok, reason := checkKey(context.Background(), "k-bad", time.Second)
	if ok {
		t.Fatalf("expected ok=false")
	}
	if !strings.Contains(reason, "401") || !strings.Contains(reason, "API_KEY_INVALID") || !strings.Contains(reason, "bad key") {
		t.Fatalf("unexpected reason: %s", reason)
	}
}

func TestCheckKey_FallbackHTTPStatus(t *testing.T) {
	t.Setenv("MUNA_IMAGE_GOOGLE_BASE_URL", "")

	originalTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     make(http.Header),
		}, nil
	})
	defer func() { http.DefaultTransport = originalTransport }()

	ok, reason := checkKey(context.Background(), "k-limit", time.Second)
	if ok {
		t.Fatalf("expected ok=false")
	}
	if reason != "HTTP 429" {
		t.Fatalf("unexpected reason: %s", reason)
	}
}
