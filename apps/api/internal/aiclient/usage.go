package aiclient

import (
	"context"
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
