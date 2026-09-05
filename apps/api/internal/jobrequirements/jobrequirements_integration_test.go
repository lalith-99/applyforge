package jobrequirements_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobrequirements"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobs"
	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
)

func TestRepository_UpsertAndGet(t *testing.T) {
	q := testdb.OpenTx(t)
	jobsRepo := jobs.NewRepositoryFromQueries(q)
	repo := jobrequirements.NewRepositoryFromQueries(q)
	ctx := context.Background()

	externalID := uuid.NewString()
	companyID, err := jobsRepo.UpsertCompany(ctx, "Acme", fmt.Sprintf("acme-jr-%s", externalID))
	if err != nil {
		t.Fatalf("UpsertCompany: %v", err)
	}
	upserted, err := jobsRepo.UpsertJob(ctx, jobs.Job{
		Source:          "GREENHOUSE",
		ExternalID:      externalID,
		CompanyID:       companyID,
		CompanyName:     "Acme",
		Title:           "Backend Engineer",
		NormalizedTitle: "backend engineer",
		Description:     "Go and PostgreSQL",
		ContentHash:     "hash-v1",
	})
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}

	if _, err := repo.Get(ctx, upserted.Job.ID); err != jobrequirements.ErrNotFound {
		t.Fatalf("expected ErrNotFound before first parse, got %v", err)
	}

	parsed := aiclient.JobRequirements{
		Keywords:       []string{"Go", "PostgreSQL"},
		RequiredSkills: []aiclient.SkillRequirement{{NormalizedName: "Go", Importance: "required"}},
	}
	created, err := repo.Upsert(ctx, upserted.Job.ID, "hash-v1", parsed)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if len(created.Keywords) != 2 {
		t.Fatalf("expected 2 keywords, got %d", len(created.Keywords))
	}

	fetched, err := repo.Get(ctx, upserted.Job.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fetched.ContentHash != "hash-v1" {
		t.Fatalf("expected content hash to persist")
	}
	if len(fetched.RequiredSkills) != 1 || fetched.RequiredSkills[0].NormalizedName != "Go" {
		t.Fatalf("expected required skills to round-trip, got %+v", fetched.RequiredSkills)
	}
}
