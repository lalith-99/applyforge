package aiclient

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

func TestNewUsesDedicatedLongTimeoutForTailoring(t *testing.T) {
	client := New("http://ai-worker:8000")

	if client.http.Timeout != defaultAIWorkerTimeout {
		t.Fatalf("default timeout = %s, want %s", client.http.Timeout, defaultAIWorkerTimeout)
	}
	if client.tailoringHTTP.Timeout != tailoringAIWorkerTimeout {
		t.Fatalf("tailoring timeout = %s, want %s", client.tailoringHTTP.Timeout, tailoringAIWorkerTimeout)
	}
	if client.http.Timeout != 60*time.Second {
		t.Fatalf("default timeout changed unexpectedly: %s", client.http.Timeout)
	}
	if client.tailoringHTTP.Timeout != 5*time.Minute {
		t.Fatalf("tailoring timeout = %s, want 5m", client.tailoringHTTP.Timeout)
	}
}

func TestSuggestTailoringUsesDedicatedHTTPClient(t *testing.T) {
	defaultCalled := false
	tailoringCalled := false

	client := &Client{
		baseURL: "http://ai-worker:8000",
		http: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			defaultCalled = true
			t.Fatalf("SuggestTailoring used the default 60-second HTTP client")
			return nil, nil
		})},
		tailoringHTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			tailoringCalled = true
			if req.URL.Path != "/v1/tailoring/suggest" {
				t.Fatalf("path = %q, want /v1/tailoring/suggest", req.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Request:    req,
			}, nil
		})},
	}

	if _, err := client.SuggestTailoring(context.Background(), TailoringRequest{}); err != nil {
		t.Fatalf("SuggestTailoring returned error: %v", err)
	}
	if defaultCalled {
		t.Fatal("default HTTP client was used")
	}
	if !tailoringCalled {
		t.Fatal("dedicated tailoring HTTP client was not used")
	}
}
