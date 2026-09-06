package jobs

import (
	"context"
	"fmt"
	"testing"
	"time"

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

func TestRepository_CloseStaleJobs_ClosesUnseenAndRevivesOnReappear(t *testing.T) {
	q := testdb.OpenTx(t)
	repo := &Repository{q: q}
	ctx := context.Background()

	companyID, err := repo.UpsertCompany(ctx, "Acme Closure Co", fmt.Sprintf("acme-closure-%s", t.Name()))
	if err != nil {
		t.Fatalf("UpsertCompany: %v", err)
	}

	job := Job{
		Source:          "GREENHOUSE",
		ExternalID:      uuid.NewString(),
		CompanyID:       companyID,
		CompanyName:     "Acme Closure Co",
		Title:           "Backend Engineer",
		NormalizedTitle: normalizeTitle("Backend Engineer"),
		Description:     "Build things",
		ContentHash:     contentHash("Acme Closure Co", "Backend Engineer", "", "Build things"),
	}
	upserted, err := repo.UpsertJob(ctx, job)
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}

	// Simulate a poll that completed after this job's last_seen_at, i.e. the
	// board no longer listed it.
	cutoff := time.Now().Add(1 * time.Second)
	closed, err := repo.CloseStaleJobs(ctx, "GREENHOUSE", companyID, cutoff)
	if err != nil {
		t.Fatalf("CloseStaleJobs: %v", err)
	}
	if closed != 1 {
		t.Fatalf("expected 1 job closed, got %d", closed)
	}

	fetched, err := repo.GetByID(ctx, upserted.Job.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fetched.Status != "CLOSED" {
		t.Fatalf("expected status CLOSED, got %s", fetched.Status)
	}

	// The job reappears in a later poll - upserting it again should revive it.
	revived, err := repo.UpsertJob(ctx, job)
	if err != nil {
		t.Fatalf("UpsertJob (revive): %v", err)
	}
	if revived.Job.Status != "ACTIVE" {
		t.Fatalf("expected status ACTIVE after re-appearing in a poll, got %s", revived.Job.Status)
	}
}

func TestRepository_CrossSourceDedupe_LinksAndExcludesFromListing(t *testing.T) {
	q := testdb.OpenTx(t)
	repo := &Repository{q: q}
	ctx := context.Background()

	companyID, err := repo.UpsertCompany(ctx, "Acme Dedupe Co", fmt.Sprintf("acme-dedupe-%s", t.Name()))
	if err != nil {
		t.Fatalf("UpsertCompany: %v", err)
	}

	fp := buildFingerprint("Acme Dedupe Co", "Senior Backend Engineer", "remote")

	first, err := repo.UpsertJob(ctx, Job{
		Source:          "GREENHOUSE",
		ExternalID:      uuid.NewString(),
		CompanyID:       companyID,
		CompanyName:     "Acme Dedupe Co",
		Title:           "Senior Backend Engineer",
		NormalizedTitle: normalizeTitle("Senior Backend Engineer"),
		RemoteType:      strPtrTest("remote"),
		Description:     "Build things",
		ContentHash:     contentHash("Acme Dedupe Co", "Senior Backend Engineer", "", "Build things"),
		Fingerprint:     fp,
	})
	if err != nil {
		t.Fatalf("UpsertJob (source A): %v", err)
	}

	second, err := repo.UpsertJob(ctx, Job{
		Source:          "ARBEITNOW",
		ExternalID:      uuid.NewString(),
		CompanyID:       companyID,
		CompanyName:     "Acme Dedupe Co",
		Title:           "Senior Backend Engineer",
		NormalizedTitle: normalizeTitle("Senior Backend Engineer"),
		RemoteType:      strPtrTest("remote"),
		Description:     "Build things (aggregator copy)",
		ContentHash:     contentHash("Acme Dedupe Co", "Senior Backend Engineer", "", "Build things (aggregator copy)"),
		Fingerprint:     fp,
	})
	if err != nil {
		t.Fatalf("UpsertJob (source B): %v", err)
	}

	canonical, err := repo.FindCanonicalByFingerprint(ctx, fp, second.Job.ID)
	if err != nil {
		t.Fatalf("FindCanonicalByFingerprint: %v", err)
	}
	if canonical.ID != first.Job.ID {
		t.Fatalf("expected canonical match to be the first-seen job %s, got %s", first.Job.ID, canonical.ID)
	}

	if err := repo.SetCanonicalJobID(ctx, second.Job.ID, canonical.ID); err != nil {
		t.Fatalf("SetCanonicalJobID: %v", err)
	}

	dup, err := repo.GetByID(ctx, second.Job.ID)
	if err != nil {
		t.Fatalf("GetByID (dup): %v", err)
	}
	if dup.CanonicalJobID == nil || *dup.CanonicalJobID != first.Job.ID {
		t.Fatalf("expected duplicate job's CanonicalJobID to point at %s, got %v", first.Job.ID, dup.CanonicalJobID)
	}

	results, _, err := repo.List(ctx, ListFilter{Search: "Senior Backend Engineer"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, r := range results {
		if r.ID == second.Job.ID {
			t.Fatalf("expected duplicate job %s to be excluded from listings", second.Job.ID)
		}
	}
}

func strPtrTest(s string) *string { return &s }

func TestRepository_EmbeddingStorageAndSearch(t *testing.T) {
	q := testdb.OpenTx(t)
	repo := &Repository{q: q}
	ctx := context.Background()

	companyID, err := repo.UpsertCompany(ctx, "Acme Embed Co", fmt.Sprintf("acme-embed-%s", t.Name()))
	if err != nil {
		t.Fatalf("UpsertCompany: %v", err)
	}

	upserted, err := repo.UpsertJob(ctx, Job{
		Source:          "GREENHOUSE",
		ExternalID:      uuid.NewString(),
		CompanyID:       companyID,
		CompanyName:     "Acme Embed Co",
		Title:           "Senior Backend Engineer",
		NormalizedTitle: normalizeTitle("Senior Backend Engineer"),
		Description:     "Go, Kafka, Kubernetes",
		ContentHash:     contentHash("Acme Embed Co", "Senior Backend Engineer", "", "Go, Kafka, Kubernetes"),
	})
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}

	vector := make([]float32, 1536)
	vector[0] = 1.0
	if err := repo.UpdateEmbedding(ctx, upserted.Job.ID, vector, "text-embedding-3-small"); err != nil {
		t.Fatalf("UpdateEmbedding: %v", err)
	}

	matches, err := repo.SearchByEmbedding(ctx, vector, 5, EmbeddingSearchFilter{})
	if err != nil {
		t.Fatalf("SearchByEmbedding: %v", err)
	}
	found := false
	for _, m := range matches {
		if m.ID == upserted.Job.ID {
			found = true
			if m.Distance > 0.0001 {
				t.Fatalf("expected ~0 distance for an identical vector, got %f", m.Distance)
			}
		}
	}
	if !found {
		t.Fatalf("expected the embedded job to appear in SearchByEmbedding results")
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
