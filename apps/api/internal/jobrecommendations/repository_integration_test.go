package jobrecommendations

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobs"
	"github.com/lalithlochan/applyforge/apps/api/internal/preferences"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

// Exercise real commits: a transaction-scoped test repository would hide the
// original bug, where DELETE and each INSERT committed independently.
func TestReplaceForUser_RollsBackIncompleteShortlist(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	ctx := context.Background()
	pool, err := database.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	user, err := users.NewRepository(pool).CreateWithPassword(ctx, "recommendations-"+uuid.NewString()+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID) }()
	jobsRepo := jobs.NewRepository(pool)
	companyID, err := jobsRepo.UpsertCompany(ctx, "Review Fixture", "review-"+uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(ctx, "DELETE FROM companies WHERE id = $1", companyID) }()
	var jobIDs []uuid.UUID
	for range 2 {
		j, err := jobsRepo.UpsertJob(ctx, jobs.Job{
			Source: "LEVER", ExternalID: uuid.NewString(), CompanyID: companyID,
			CompanyName: "Review Fixture", Title: "Java Developer", NormalizedTitle: "java developer",
			Description: "Java and Kafka", ContentHash: uuid.NewString(),
		})
		if err != nil {
			t.Fatal(err)
		}
		jobIDs = append(jobIDs, j.Job.ID)
	}
	defer func() { _, _ = pool.Exec(ctx, "DELETE FROM jobs WHERE company_id = $1", companyID) }()
	repo := NewRepository(pool)
	initial := []Recommendation{{JobID: jobIDs[0], FinalScore: 80}}
	if err := repo.ReplaceForUser(ctx, user.ID, initial); err != nil {
		t.Fatal(err)
	}
	// The first replacement INSERT succeeds; the second fails its foreign key.
	err = repo.ReplaceForUser(ctx, user.ID, []Recommendation{
		{JobID: jobIDs[1], FinalScore: 90}, {JobID: uuid.New(), FinalScore: 95},
	})
	if err == nil {
		t.Fatal("expected invalid job foreign key to reject replacement")
	}
	var got uuid.UUID
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM job_recommendations WHERE user_id = $1", user.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("previous shortlist must survive, got %d rows", count)
	}
	if err := pool.QueryRow(ctx, "SELECT job_id FROM job_recommendations WHERE user_id = $1", user.ID).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != jobIDs[0] {
		t.Fatalf("failed replacement leaked a partial shortlist: got %s", got)
	}
	if err := repo.ReplaceForUser(ctx, user.ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM job_recommendations WHERE user_id = $1", user.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("successful empty replacement must clear shortlist: count=%d error=%v", count, err)
	}
}

func TestListForUser_RevalidatesStrictFullTimeAndH1BEvidence(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	ctx := context.Background()
	pool, err := database.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	user, err := users.NewRepository(pool).CreateWithPassword(ctx, "recommendation-eligibility-"+uuid.NewString()+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID) }()
	if _, err := preferences.NewRepository(pool).Upsert(ctx, user.ID, preferences.UpsertInput{
		EmploymentTypes:     []string{"full_time"},
		ImmigrationStatus:   stringPtr("H-1B"),
		RequiresH1BTransfer: true,
	}); err != nil {
		t.Fatal(err)
	}

	jobsRepo := jobs.NewRepository(pool)
	companyID, err := jobsRepo.UpsertCompany(ctx, "Eligibility Read Fixture", "eligibility-read-"+uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(ctx, "DELETE FROM companies WHERE id = $1", companyID) }()

	now := time.Now().UTC()
	us := "US"
	fullTime := "FullTime"
	createJob := func(employmentType *string, supported bool) uuid.UUID {
		t.Helper()
		created, createErr := jobsRepo.UpsertJob(ctx, jobs.Job{
			Source: "LEVER", ExternalID: uuid.NewString(), CompanyID: companyID,
			CompanyName: "Eligibility Read Fixture", Title: "Backend Engineer", NormalizedTitle: "backend engineer",
			Description: "Build APIs", CountryCode: &us, EmploymentType: employmentType,
			PostedAt: &now, ContentHash: uuid.NewString(),
		})
		if createErr != nil {
			t.Fatal(createErr)
		}
		if signalErr := jobsRepo.UpdateExplicitSponsorshipSignals(ctx, created.Job.ID, false, supported); signalErr != nil {
			t.Fatal(signalErr)
		}
		return created.Job.ID
	}

	wantID := createJob(&fullTime, true)
	unknownEmploymentID := createJob(nil, true)
	noEvidenceID := createJob(&fullTime, false)
	repo := NewRepository(pool)
	if err := repo.ReplaceForUser(ctx, user.ID, []Recommendation{
		{JobID: wantID, FinalScore: 90},
		{JobID: unknownEmploymentID, FinalScore: 95},
		{JobID: noEvidenceID, FinalScore: 99},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := repo.ListForUser(ctx, user.ID, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].JobID != wantID {
		t.Fatalf("expected only explicit-support full-time recommendation %s, got %+v", wantID, got)
	}
}

func stringPtr(value string) *string { return &value }
