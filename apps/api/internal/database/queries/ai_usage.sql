-- name: RecordAIUsage :exec
INSERT INTO ai_usage (operation, status, latency_ms, cache_hit, error_message)
VALUES ($1, $2, $3, $4, $5);
