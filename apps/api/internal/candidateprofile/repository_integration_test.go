package candidateprofile_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/candidateprofile"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

func createTestUser(t *testing.T, q *db.Queries) uuid.UUID {
	t.Helper()
	userRepo := users.NewRepositoryFromQueries(q)
	user, err := userRepo.CreateWithPassword(context.Background(), fmt.Sprintf("candidateprofile-%s@example.com", uuid.NewString()), "hash")
	if err != nil {
		t.Fatalf("create fixture user: %v", err)
	}
	return user.ID
}

func TestRepository_CreateAndGetLatest_VersionsIncrement(t *testing.T) {
	q := testdb.OpenTx(t)
	repo := candidateprofile.NewRepositoryFromQueries(q)
	ctx := context.Background()

	userID := createTestUser(t, q)

	first, err := repo.Create(ctx, userID, candidateprofile.Profile{
		TargetRoles:       []string{"Backend Engineer"},
		CoreSkills:        []string{"Go", "PostgreSQL"},
		Summary:           "v1 summary",
		SourceContentHash: "hash-v1",
	})
	if err != nil {
		t.Fatalf("Create (v1): %v", err)
	}
	if first.Version != 1 {
		t.Fatalf("expected version 1, got %d", first.Version)
	}

	second, err := repo.Create(ctx, userID, candidateprofile.Profile{
		TargetRoles: []string{"Senior Backend Engineer"},
		CoreSkills:  []string{"Go", "PostgreSQL", "Kafka"},
		TransferableSkills: []candidateprofile.TransferableSkill{
			{Skill: "Kubernetes", Evidence: "Deployed services via Helm", Strength: "TRANSFERABLE"},
		},
		Summary:           "v2 summary",
		SourceContentHash: "hash-v2",
	})
	if err != nil {
		t.Fatalf("Create (v2): %v", err)
	}
	if second.Version != 2 {
		t.Fatalf("expected version 2, got %d", second.Version)
	}

	latest, err := repo.GetLatest(ctx, userID)
	if err != nil {
		t.Fatalf("GetLatest: %v", err)
	}
	if latest.Version != 2 || latest.Summary != "v2 summary" {
		t.Fatalf("expected latest to be v2, got %+v", latest)
	}
	if len(latest.TransferableSkills) != 1 || latest.TransferableSkills[0].Skill != "Kubernetes" {
		t.Fatalf("expected transferable skills to round-trip, got %+v", latest.TransferableSkills)
	}

	vector := make([]float32, 1536)
	vector[0] = 1.0
	if err := repo.UpdateEmbedding(ctx, latest.ID, vector, "text-embedding-3-small"); err != nil {
		t.Fatalf("UpdateEmbedding: %v", err)
	}
}

func TestRepository_GetLatest_NotFound(t *testing.T) {
	q := testdb.OpenTx(t)
	repo := candidateprofile.NewRepositoryFromQueries(q)
	ctx := context.Background()

	if _, err := repo.GetLatest(ctx, uuid.New()); err != candidateprofile.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
