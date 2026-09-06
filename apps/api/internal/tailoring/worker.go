package tailoring

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/background"
)

// JobTypeProcess is the background job type enqueued after a tailoring run
// is created (Phase J: async multi-pass tailoring).
const JobTypeProcess = "process_tailoring_run"

// ProcessPayload is the JSON payload enqueued for a process_tailoring_run job.
type ProcessPayload struct {
	RunID string `json:"run_id"`
}

// Worker processes process_tailoring_run background jobs by running the
// service's multi-pass pipeline (see Service.ProcessRun).
type Worker struct {
	svc *Service
}

// NewWorker builds a Worker.
func NewWorker(svc *Service) *Worker {
	return &Worker{svc: svc}
}

// Handle implements background.Handler for JobTypeProcess.
func (w *Worker) Handle(ctx context.Context, job background.Job) error {
	var payload ProcessPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}

	runID, err := uuid.Parse(payload.RunID)
	if err != nil {
		return fmt.Errorf("invalid run id: %w", err)
	}

	return w.svc.ProcessRun(ctx, runID)
}
