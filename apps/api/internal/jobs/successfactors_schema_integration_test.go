package jobs

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
)

func TestRepository_CreateJobSource_AllowsSuccessFactorsAfterMigrations(t *testing.T) {
	q := testdb.OpenTx(t)
	repo := NewRepositoryFromQueries(q)
	ctx := context.Background()

	suffix := uuid.NewString()
	companyID, err := repo.UpsertCompany(ctx, "SuccessFactors Schema Test", "successfactors-schema-test-"+suffix)
	if err != nil {
		t.Fatalf("UpsertCompany: %v", err)
	}

	boardToken := "https://careers.example-" + suffix + ".com"
	sourceID, err := repo.CreateJobSource(ctx, "SUCCESSFACTORS", companyID, boardToken, true)
	if err != nil {
		t.Fatalf("CreateJobSource(SUCCESSFACTORS): %v", err)
	}
	if sourceID == uuid.Nil {
		t.Fatal("expected non-nil SuccessFactors job source id")
	}
}

func TestBuildSource_SuccessFactors(t *testing.T) {
	source, sourceName, err := BuildSource(JobSourceConfig{
		SourceType: "SUCCESSFACTORS",
		BoardToken: "https://careers.example.com",
	})
	if err != nil {
		t.Fatalf("BuildSource(SUCCESSFACTORS): %v", err)
	}
	if source == nil || sourceName != "SUCCESSFACTORS" {
		t.Fatalf("unexpected source=%T sourceName=%q", source, sourceName)
	}
}
