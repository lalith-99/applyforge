package jobs

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
)

func TestSuccessFactorsMigration_AllowsFirstClassRegistryRows(t *testing.T) {
	q := testdb.OpenTx(t)
	ctx := context.Background()

	companyID := uuid.New()
	if _, err := q.Exec(ctx, `
		INSERT INTO companies (id, name, normalized_name)
		VALUES ($1, $2, $3)
	`, companyID, "SuccessFactors Registry Test", "successfactors-registry-test-"+uuid.NewString()); err != nil {
		t.Fatalf("insert company: %v", err)
	}

	if _, err := q.Exec(ctx, `
		INSERT INTO company_source_registry (
			company_id, source_type, board_token, source_url,
			discovery_method, confidence, monitorable
		) VALUES ($1, 'SUCCESSFACTORS', $2, $2, 'MANUAL', 0.99, false)
	`, companyID, "https://careers.example.com"); err != nil {
		t.Fatalf("insert SUCCESSFACTORS registry row: %v", err)
	}
}
