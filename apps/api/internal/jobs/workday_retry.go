package jobs

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	workdayHTTPMaxAttempts = 2
	workdayRetryBaseDelay  = 250 * time.Millisecond
	workdayRateLimitDelay  = time.Second
	workdayRetryMaxDelay   = 5 * time.Second
)

type workdayRetryTransport struct {
	base        http.RoundTripper
	maxAttempts int
	sleep       func(context.Context, time.Duration) error
	now         func() time.Time
}

func newWorkdayRetryTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &workdayRetryTransport{
		base:        base,
		maxAttempts: workdayHTTPMaxAttempts,
		sleep:       sleepWorkdayRetry,
		now:         time.Now,
	}
}

func (t *workdayRetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	attempts := t.maxAttempts
	if attempts < 1 {
		attempts = 1
	}

	for attempt := 0; attempt < attempts; attempt++ {
		current, err := cloneWorkdayRequest(req)
		if err != nil {
			return nil, err
		}

		resp, err := t.base.RoundTrip(current)
		if err == nil && !retryableWorkdayStatus(resp.StatusCode) {
			return resp, nil
		}

		if err != nil {
			if req.Context().Err() != nil || !retryableWorkdayTransportError(err) || attempt == attempts-1 {
				return resp, err
			}
		} else if attempt == attempts-1 {
			return resp, nil
		}

		delay := workdayRetryBaseDelay << attempt
		if resp != nil {
			delay = workdayRetryDelay(resp, attempt, t.now())
			if resp.Body != nil {
				_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
				_ = resp.Body.Close()
			}
		}

		if err := t.sleep(req.Context(), delay); err != nil {
			return nil, err
		}
	}

	return nil, errors.New("Workday request retry loop exhausted")
}

func cloneWorkdayRequest(req *http.Request) (*http.Request, error) {
	clone := req.Clone(req.Context())
	if req.Body == nil || req.Body == http.NoBody {
		return clone, nil
	}
	if req.GetBody == nil {
		return nil, errors.New("Workday request body cannot be replayed safely")
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	clone.Body = body
	return clone, nil
}

func retryableWorkdayStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func retryableWorkdayTransportError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}

	text := strings.ToLower(err.Error())
	return strings.Contains(text, "connection reset") ||
		strings.Contains(text, "server closed idle connection")
}

func workdayRetryDelay(resp *http.Response, attempt int, now time.Time) time.Duration {
	if retryAfter, ok := parseWorkdayRetryAfter(resp.Header.Get("Retry-After"), now); ok {
		return retryAfter
	}

	delay := workdayRetryBaseDelay << attempt
	if resp.StatusCode == http.StatusTooManyRequests && delay < workdayRateLimitDelay {
		delay = workdayRateLimitDelay
	}
	if delay > workdayRetryMaxDelay {
		return workdayRetryMaxDelay
	}
	return delay
}

func parseWorkdayRetryAfter(raw string, now time.Time) (time.Duration, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(raw); err == nil && seconds >= 0 {
		return capWorkdayRetryDelay(time.Duration(seconds) * time.Second), true
	}
	if retryAt, err := http.ParseTime(raw); err == nil {
		delay := retryAt.Sub(now)
		if delay < 0 {
			delay = 0
		}
		return capWorkdayRetryDelay(delay), true
	}
	return 0, false
}

func capWorkdayRetryDelay(delay time.Duration) time.Duration {
	if delay > workdayRetryMaxDelay {
		return workdayRetryMaxDelay
	}
	return delay
}

func sleepWorkdayRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
