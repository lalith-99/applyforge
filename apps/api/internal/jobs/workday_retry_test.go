package jobs

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type workdayRoundTripFunc func(*http.Request) (*http.Response, error)

func (f workdayRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func workdayTestResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

func TestWorkdayRetryTransportRetriesRateLimitAndReplaysBody(t *testing.T) {
	calls := 0
	var bodies []string
	var delays []time.Duration

	base := workdayRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		bodies = append(bodies, string(payload))

		if calls == 1 {
			resp := workdayTestResponse(req, http.StatusTooManyRequests, "slow down")
			resp.Header.Set("Retry-After", "2")
			return resp, nil
		}
		return workdayTestResponse(req, http.StatusOK, `{}`), nil
	})

	transport := &workdayRetryTransport{
		base:        base,
		maxAttempts: 2,
		sleep: func(_ context.Context, delay time.Duration) error {
			delays = append(delays, delay)
			return nil
		},
		now: func() time.Time {
			return time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
		},
	}

	const payload = `{"offset":0}`
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"https://acme.wd1.myworkdayjobs.com/wday/cxs/acme/External/jobs",
		bytes.NewReader([]byte(payload)),
	)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected successful retry, got %s", resp.Status)
	}
	if calls != 2 {
		t.Fatalf("expected one retry, got %d calls", calls)
	}
	if len(bodies) != 2 || bodies[0] != payload || bodies[1] != payload {
		t.Fatalf("POST body was not replayed exactly: %+v", bodies)
	}
	if len(delays) != 1 || delays[0] != 2*time.Second {
		t.Fatalf("expected Retry-After delay of 2s, got %+v", delays)
	}
}

func TestWorkdayRetryTransportRetriesUnexpectedEOF(t *testing.T) {
	calls := 0
	var delays []time.Duration

	base := workdayRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, io.ErrUnexpectedEOF
		}
		return workdayTestResponse(req, http.StatusOK, `{}`), nil
	})
	transport := &workdayRetryTransport{
		base:        base,
		maxAttempts: 2,
		sleep: func(_ context.Context, delay time.Duration) error {
			delays = append(delays, delay)
			return nil
		},
		now: time.Now,
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"https://acme.wd1.myworkdayjobs.com/wday/cxs/acme/External/job/R123",
		nil,
	)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if calls != 2 {
		t.Fatalf("expected one retry after unexpected EOF, got %d calls", calls)
	}
	if len(delays) != 1 || delays[0] != workdayRetryBaseDelay {
		t.Fatalf("expected base retry delay, got %+v", delays)
	}
}

func TestWorkdayRetryTransportDoesNotRetryPermanentStatus(t *testing.T) {
	calls := 0
	base := workdayRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return workdayTestResponse(req, http.StatusNotFound, "gone"), nil
	})
	transport := &workdayRetryTransport{
		base:        base,
		maxAttempts: 2,
		sleep: func(context.Context, time.Duration) error {
			t.Fatal("permanent status must not sleep for retry")
			return nil
		},
		now: time.Now,
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/job/R123", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound || calls != 1 {
		t.Fatalf("expected immediate 404 with one call, status=%d calls=%d", resp.StatusCode, calls)
	}
}

func TestParseWorkdayRetryAfterCapsDelay(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	delay, ok := parseWorkdayRetryAfter("60", now)
	if !ok || delay != workdayRetryMaxDelay {
		t.Fatalf("expected Retry-After to cap at %s, got %s ok=%v", workdayRetryMaxDelay, delay, ok)
	}

	delay, ok = parseWorkdayRetryAfter(now.Add(3*time.Second).Format(http.TimeFormat), now)
	if !ok || delay != 3*time.Second {
		t.Fatalf("expected HTTP-date Retry-After of 3s, got %s ok=%v", delay, ok)
	}
}

func TestNewWorkdaySourceUsesTransientRetryTransport(t *testing.T) {
	source, err := NewWorkdaySource("acme.wd1.myworkdayjobs.com|acme|External")
	if err != nil {
		t.Fatalf("NewWorkdaySource: %v", err)
	}
	if _, ok := source.http.Transport.(*workdayRetryTransport); !ok {
		t.Fatalf("expected Workday HTTP client to use retry transport, got %T", source.http.Transport)
	}
}
