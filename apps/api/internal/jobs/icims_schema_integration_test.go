package jobs

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
)

func TestRepository_CreateJobSource_AllowsICIMSAfterMigrations(t *testing.T) {
	q := testdb.OpenTx(t)
	repo := NewRepositoryFromQueries(q)
	ctx := context.Background()

	suffix := uuid.NewString()
	companyID, err := repo.UpsertCompany(ctx, "iCIMS Schema Test", "icims-schema-test-"+suffix)
	if err != nil {
		t.Fatalf("UpsertCompany: %v", err)
	}

	sourceID, err := repo.CreateJobSource(ctx, "ICIMS", companyID, "careers-"+suffix+".icims.com", true)
	if err != nil {
		t.Fatalf("CreateJobSource(ICIMS): %v", err)
	}
	if sourceID == uuid.Nil {
		t.Fatal("expected non-nil iCIMS job source id")
	}
}
