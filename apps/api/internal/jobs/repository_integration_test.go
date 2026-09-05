package jobs

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
)

func TestRepository_UpsertJob_IsIdempotent(t *testing.T) {
	q := testdb.OpenTx(t)
	repo := &Repository{q: q}
	ctx := context.Background()

	externalID := uuid.NewString()
	companyID, err := repo.UpsertCompany(ctx, "Acme", fmt.Sprintf("acme-%s", externalID))
	if err != nil {
		t.Fatalf("UpsertCompany: %v", err)
	}

	job := Job{
		Source:          "GREENHOUSE",
		ExternalID:      externalID,
		CompanyID:       companyID,
		CompanyName:     "Acme",
		Title:           "Backend Engineer",
		NormalizedTitle: normalizeTitle("Backend Engineer"),
		Description:     "Build things",
		ContentHash:     contentHash("Acme", "Backend Engineer", "", "Build things"),
	}

	first, err := repo.UpsertJob(ctx, job)
	if err != nil {
		t.Fatalf("first UpsertJob: %v", err)
	}
	if !first.Inserted {
		t.Fatalf("expected first upsert to insert a new row")
	}

	second, err := repo.UpsertJob(ctx, job)
	if err != nil {
		t.Fatalf("second UpsertJob: %v", err)
	}
	if second.Inserted {
		t.Fatalf("expected second upsert (same source+external_id) to update, not insert")
	}
	if second.Job.ID != first.Job.ID {
		t.Fatalf("expected same job id across repeated polls")
	}
}

func TestRepository_ListAndGet(t *testing.T) {
	q := testdb.OpenTx(t)
	repo := &Repository{q: q}
	ctx := context.Background()

	externalID := uuid.NewString()
	companyID, err := repo.UpsertCompany(ctx, "Acme List Co", fmt.Sprintf("acme-list-%s", externalID))
	if err != nil {
		t.Fatalf("UpsertCompany: %v", err)
	}

	_, err = repo.UpsertJob(ctx, Job{
		Source:          "GREENHOUSE",
		ExternalID:      externalID,
		CompanyID:       companyID,
		CompanyName:     "Acme List Co",
		Title:           "Platform Engineer",
		NormalizedTitle: normalizeTitle("Platform Engineer"),
		Description:     "Build platforms",
		ContentHash:     contentHash("Acme List Co", "Platform Engineer", "", "Build platforms"),
	})
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}

	results, total, err := repo.List(ctx, ListFilter{Search: "Platform Engineer"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total < 1 || len(results) < 1 {
		t.Fatalf("expected at least 1 matching job, got total=%d len=%d", total, len(results))
	}

	fetched, err := repo.GetByID(ctx, results[0].ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fetched.Title != "Platform Engineer" {
		t.Fatalf("expected title %q, got %q", "Platform Engineer", fetched.Title)
	}
}
