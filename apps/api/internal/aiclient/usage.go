package aiclient

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// UsageRecorder is invoked after every AI-worker call completes.
type UsageRecorder func(ctx context.Context, operation string, latencyMS int64, status string, errMsg *string)

// SetUsageRecorder wires a recorder invoked after every subsequent call.
// Left nil (the default), tracking is a no-op - safe for tests/tools that
// construct a Client directly.
func (c *Client) SetUsageRecorder(fn UsageRecorder) {
	c.usageRecorder = fn
}

// track starts timing an operation and returns a func to call (typically via
// defer) with the final error, which records latency/status.
func (c *Client) track(ctx context.Context, operation string) func(errPtr *error) {
	start := time.Now()
	return func(errPtr *error) {
		if c.usageRecorder == nil {
			return
		}
		status := "SUCCESS"
		var errMsg *string
		if errPtr != nil && *errPtr != nil {
			status = "ERROR"
			msg := (*errPtr).Error()
			errMsg = &msg
		}
		c.usageRecorder(ctx, operation, time.Since(start).Milliseconds(), status, errMsg)
	}
}


type DetailedUsage struct {
	Operation        string
	LatencyMS        int64
	Status           string
	ErrorMessage     *string
	Provider         string
	Model            string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	EstimatedCostUSD *float64
}

type DetailedUsageRecorder func(ctx context.Context, usage DetailedUsage)

func (c *Client) SetDetailedUsageRecorder(fn DetailedUsageRecorder) {
	c.detailedUsageRecorder = fn
}

func (c *Client) trackDetailed(ctx context.Context, operation string, headers *http.Header) func(errPtr *error) {
	start := time.Now()
	return func(errPtr *error) {
		if c.detailedUsageRecorder == nil {
			return
		}
		status := "SUCCESS"
		var errMsg *string
		if errPtr != nil && *errPtr != nil {
			status = "ERROR"
			msg := (*errPtr).Error()
			errMsg = &msg
		}

		var h http.Header
		if headers != nil {
			h = *headers
		}
		event := DetailedUsage{
			Operation:    operation,
			LatencyMS:    time.Since(start).Milliseconds(),
			Status:       status,
			ErrorMessage: errMsg,
			Provider:     headerString(h, "X-ApplyForge-AI-Provider"),
			Model:        headerString(h, "X-ApplyForge-AI-Model"),
			PromptTokens: headerInt(h, "X-ApplyForge-AI-Prompt-Tokens"),
			CompletionTokens: headerInt(h, "X-ApplyForge-AI-Completion-Tokens"),
			TotalTokens:      headerInt(h, "X-ApplyForge-AI-Total-Tokens"),
		}
		if raw := headerString(h, "X-ApplyForge-AI-Estimated-Cost-USD"); raw != "" {
			if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed >= 0 {
				event.EstimatedCostUSD = &parsed
			}
		}
		c.detailedUsageRecorder(ctx, event)
	}
}

func headerString(h http.Header, key string) string {
	if h == nil {
		return ""
	}
	return h.Get(key)
}

func headerInt(h http.Header, key string) int {
	raw := headerString(h, key)
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0
	}
	return value
}
