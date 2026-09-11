package jobs

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
)

func TestRepository_ResolveCompanyByDiscoveredSources_UsesVerifiedJobSource(t *testing.T) {
	q := testdb.OpenTx(t)
	repo := NewRepositoryFromQueries(q)
	ctx := context.Background()

	suffix := uuid.NewString()
	companyID, err := repo.UpsertCompany(ctx, "Deloitte Consulting LLP", "deloitte-consulting-"+suffix)
	if err != nil {
		t.Fatalf("UpsertCompany: %v", err)
	}
	boardToken := "deloitte-" + suffix
	if _, err := repo.CreateJobSource(ctx, "GREENHOUSE", companyID, boardToken, true); err != nil {
		t.Fatalf("CreateJobSource: %v", err)
	}

	resolvedID, resolvedName, ok, err := repo.ResolveCompanyByDiscoveredSources(ctx, []DiscoveredCompanySource{{
		SourceType: "GREENHOUSE",
		BoardToken: boardToken,
		SourceURL:  fmt.Sprintf("https://boards.greenhouse.io/%s", boardToken),
	}})
	if err != nil {
		t.Fatalf("ResolveCompanyByDiscoveredSources: %v", err)
	}
	if !ok {
		t.Fatal("expected verified source identity to resolve a company")
	}
	if resolvedID != companyID || resolvedName != "Deloitte Consulting LLP" {
		t.Fatalf("unexpected resolved company id=%s name=%q", resolvedID, resolvedName)
	}
}

func TestRepository_ResolveCompanyByDiscoveredSources_DoesNotGuessUnknownSource(t *testing.T) {
	q := testdb.OpenTx(t)
	repo := NewRepositoryFromQueries(q)

	resolvedID, resolvedName, ok, err := repo.ResolveCompanyByDiscoveredSources(context.Background(), []DiscoveredCompanySource{{
		SourceType: "GREENHOUSE",
		BoardToken: "unknown-" + uuid.NewString(),
	}})
	if err != nil {
		t.Fatalf("ResolveCompanyByDiscoveredSources: %v", err)
	}
	if ok || resolvedID != uuid.Nil || resolvedName != "" {
		t.Fatalf("unknown source must remain unresolved: id=%s name=%q ok=%v", resolvedID, resolvedName, ok)
	}
}
