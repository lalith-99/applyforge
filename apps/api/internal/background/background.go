// Package background implements a PostgreSQL-backed job queue: workers claim
// rows with SELECT ... FOR UPDATE SKIP LOCKED, with bounded retry/backoff and
// a dead-letter status after max_attempts is exceeded.
package background

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// ErrNoJobAvailable is returned when no job is ready to be claimed.
var ErrNoJobAvailable = errors.New("no job available")

// Job is a claimed unit of background work.
type Job struct {
	ID       string
	JobType  string
	Payload  []byte
	Attempts int32
}

// Handler processes a single job type. Returning an error causes a retry
// (with backoff) up to the job's max_attempts, after which it is dead-lettered.
type Handler func(ctx context.Context, job Job) error

// Queue provides enqueue/claim operations over background_jobs.
type Queue struct {
	q *db.Queries
}

// NewQueue builds a Queue from a database pool.
func NewQueue(pool *database.Pool) *Queue {
	return &Queue{q: pool.Queries()}
}

// NewQueueFromQueries builds a Queue from an existing sqlc Queries value
// (e.g. bound to a transaction). Primarily for other packages' integration
// tests that need to enqueue/claim without a full pool.
func NewQueueFromQueries(q *db.Queries) *Queue {
	return &Queue{q: q}
}

// Enqueue schedules a new job of the given type with a JSON-serializable payload.
func (queue *Queue) Enqueue(ctx context.Context, jobType string, payload any, maxAttempts int32) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = queue.q.EnqueueJob(ctx, db.EnqueueJobParams{
		JobType:     jobType,
		Payload:     body,
		MaxAttempts: maxAttempts,
	})
	return err
}

// EnqueueDebounced schedules a job unless an equivalent one is already
// pending/running or was enqueued within minInterval. This is useful for
// fan-in events such as dozens of source polls all changing the same user's
// recommendation catalog.
func (queue *Queue) EnqueueDebounced(
	ctx context.Context,
	jobType string,
	payload any,
	maxAttempts int32,
	minInterval time.Duration,
) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	row, findErr := queue.q.FindJobByTypeAndPayload(ctx, db.FindJobByTypeAndPayloadParams{
		JobType:      jobType,
		MatchPayload: body,
	})
	if findErr == nil {
		if row.Status == "PENDING" || row.Status == "RUNNING" {
			return nil
		}
		if minInterval > 0 && row.CreatedAt.Valid && time.Since(row.CreatedAt.Time) < minInterval {
			return nil
		}
	} else if !errors.Is(findErr, pgx.ErrNoRows) {
		return findErr
	}

	_, err = queue.q.EnqueueJob(ctx, db.EnqueueJobParams{
		JobType:     jobType,
		Payload:     body,
		MaxAttempts: maxAttempts,
	})
	if err == nil {
		return nil
	}

	// background_jobs_active_sync_source_idx is the race-safe backstop for
	// concurrent scheduler/admin enqueue attempts. The optimistic lookup above
	// keeps the normal path cheap; if another caller wins between lookup and
	// insert, treat that exact uniqueness conflict as a successful debounce.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == "background_jobs_active_sync_source_idx" {
		return nil
	}
	return err
}

// FindByTypeAndPayload returns the most recent job of jobType whose payload
// contains matchPayload (JSONB containment), regardless of status. Intended
// for tests asserting a specific enqueue happened, without racing other
// pending jobs on shared job_type prefixes.
func (queue *Queue) FindByTypeAndPayload(ctx context.Context, jobType string, matchPayload any) (Job, error) {
	body, err := json.Marshal(matchPayload)
	if err != nil {
		return Job{}, err
	}
	row, err := queue.q.FindJobByTypeAndPayload(ctx, db.FindJobByTypeAndPayloadParams{
		JobType:      jobType,
		MatchPayload: body,
	})
	if err != nil {
		return Job{}, err
	}
	return Job{
		ID:       database.PGToUUID(row.ID).String(),
		JobType:  row.JobType,
		Payload:  row.Payload,
		Attempts: row.Attempts,
	}, nil
}

// Worker polls the queue and dispatches claimed jobs to registered handlers.
type Worker struct {
	queue      *Queue
	workerName string
	handlers   map[string]Handler
	backoff    func(attempt int32) time.Duration
}

// NewWorker builds a Worker with the given identity (used for locked_by).
func NewWorker(queue *Queue, workerName string) *Worker {
	return &Worker{
		queue:      queue,
		workerName: workerName,
		handlers:   map[string]Handler{},
		backoff:    defaultBackoff,
	}
}

func defaultBackoff(attempt int32) time.Duration {
	seconds := 1 << attempt // 2, 4, 8, 16, 32...
	if seconds > 300 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}

// Register associates a job type with a handler.
func (w *Worker) Register(jobType string, handler Handler) {
	w.handlers[jobType] = handler
}

// PollOnce claims and processes a single job, if one is available. ClaimNextJob
// also reaps RUNNING leases older than two hours, so a crashed/restarted worker
// cannot strand queue rows forever.
func (w *Worker) PollOnce(ctx context.Context) error {
	row, err := w.queue.q.ClaimNextJob(ctx, database.PGText(&w.workerName))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoJobAvailable
		}
		return err
	}

	job := Job{
		ID:       database.PGToUUID(row.ID).String(),
		JobType:  row.JobType,
		Payload:  row.Payload,
		Attempts: row.Attempts,
	}

	handler, ok := w.handlers[job.JobType]
	if !ok {
		slog.Warn("no handler registered for job type", "job_type", job.JobType, "job_id", job.ID)
		return w.queue.q.FailJob(ctx, db.FailJobParams{
			ID:        row.ID,
			LastError: database.PGText(strPtr("no handler registered")),
			Column3:   0,
		})
	}

	start := time.Now()
	handleErr := handler(ctx, job)
	duration := time.Since(start)

	if handleErr != nil {
		slog.Error("background job failed", "job_id", job.ID, "job_type", job.JobType, "attempt", job.Attempts, "duration", duration, "error", handleErr)
		msg := handleErr.Error()
		backoffSeconds := int32(w.backoff(job.Attempts).Seconds())
		return w.queue.q.FailJob(ctx, db.FailJobParams{
			ID:        row.ID,
			LastError: database.PGText(&msg),
			Column3:   backoffSeconds,
		})
	}

	slog.Info("background job completed", "job_id", job.ID, "job_type", job.JobType, "duration", duration)
	return w.queue.q.CompleteJob(ctx, row.ID)
}

// Run polls continuously until ctx is cancelled, sleeping pollInterval
// between empty polls for graceful, low-overhead idling.
func (w *Worker) Run(ctx context.Context, pollInterval time.Duration) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := w.PollOnce(ctx)
		switch {
		case err == nil:
			continue
		case errors.Is(err, ErrNoJobAvailable):
			select {
			case <-ctx.Done():
				return
			case <-time.After(pollInterval):
			}
		default:
			slog.Error("worker poll error", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(pollInterval):
			}
		}
	}
}

func strPtr(s string) *string { return &s }
