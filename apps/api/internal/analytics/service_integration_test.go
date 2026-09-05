package analytics_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/analytics"
	"github.com/lalithlochan/applyforge/apps/api/internal/applications"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobs"
	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

func fixtureJob(t *testing.T, ctx context.Context, jobsRepo *jobs.Repository, suffix string) uuid.UUID {
	t.Helper()
	companyID, err := jobsRepo.UpsertCompany(ctx, "Acme", fmt.Sprintf("acme-analytics-%s", suffix))
	if err != nil {
		t.Fatalf("UpsertCompany: %v", err)
	}
	upserted, err := jobsRepo.UpsertJob(ctx, jobs.Job{
		Source:          "GREENHOUSE",
		ExternalID:      suffix,
		CompanyID:       companyID,
		CompanyName:     "Acme",
		Title:           "Backend Engineer",
		NormalizedTitle: "backend engineer",
		Description:     "Go",
		ContentHash:     "analytics-hash-" + suffix,
	})
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	return upserted.Job.ID
}

func TestService_Dashboard_FunnelAndResponseRate(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	jobsRepo := jobs.NewRepositoryFromQueries(q)
	appsRepo := applications.NewRepositoryFromQueries(q)
	appsSvc := applications.NewService(appsRepo)
	analyticsRepo := analytics.NewRepositoryFromQueries(q)
	svc := analytics.NewService(analyticsRepo, appsRepo)
	ctx := context.Background()

	user, err := userRepo.CreateWithPassword(ctx, fmt.Sprintf("analytics-%s@example.com", uuid.NewString()), "hash")
	if err != nil {
		t.Fatalf("create fixture user: %v", err)
	}

	// Two applications: one progresses to RECRUITER_SCREEN (a "response"),
	// the other stays at APPLIED (no response yet).
	scoreA := int32(85)
	jobA := fixtureJob(t, ctx, jobsRepo, uuid.NewString())
	appA, err := appsRepo.Create(ctx, user.ID, jobA, nil, applications.StatusSaved, &scoreA)
	if err != nil {
		t.Fatalf("create appA: %v", err)
	}
	if _, err := appsSvc.ChangeStatus(ctx, user.ID, appA.ID, applications.StatusApplied, nil); err != nil {
		t.Fatalf("advance appA to APPLIED: %v", err)
	}
	if _, err := appsSvc.ChangeStatus(ctx, user.ID, appA.ID, applications.StatusRecruiterScreen, nil); err != nil {
		t.Fatalf("advance appA to RECRUITER_SCREEN: %v", err)
	}

	scoreB := int32(65)
	jobB := fixtureJob(t, ctx, jobsRepo, uuid.NewString())
	appB, err := appsRepo.Create(ctx, user.ID, jobB, nil, applications.StatusSaved, &scoreB)
	if err != nil {
		t.Fatalf("create appB: %v", err)
	}
	if _, err := appsSvc.ChangeStatus(ctx, user.ID, appB.ID, applications.StatusApplied, nil); err != nil {
		t.Fatalf("advance appB to APPLIED: %v", err)
	}

	dashboard, err := svc.Dashboard(ctx, user.ID)
	if err != nil {
		t.Fatalf("Dashboard: %v", err)
	}

	if dashboard.TotalApplications != 2 {
		t.Fatalf("expected 2 total applications, got %d", dashboard.TotalApplications)
	}

	var savedCount, appliedCount, screenCount int64
	for _, stage := range dashboard.Funnel {
		switch stage.Status {
		case applications.StatusSaved:
			savedCount = stage.Count
		case applications.StatusApplied:
			appliedCount = stage.Count
		case applications.StatusRecruiterScreen:
			screenCount = stage.Count
		}
	}
	if savedCount != 2 {
		t.Fatalf("expected funnel SAVED stage to count all tracked applications (2), got %d", savedCount)
	}
	if appliedCount != 2 {
		t.Fatalf("expected funnel APPLIED stage to be 2, got %d", appliedCount)
	}
	if screenCount != 1 {
		t.Fatalf("expected funnel RECRUITER_SCREEN stage to be 1, got %d", screenCount)
	}

	if dashboard.ResponseRatePercent != 50 {
		t.Fatalf("expected response rate 50%% (1 of 2 applied reached recruiter screen), got %v", dashboard.ResponseRatePercent)
	}

	if dashboard.AverageMatchScore == nil || *dashboard.AverageMatchScore != 75 {
		t.Fatalf("expected average match score 75 ((85+65)/2), got %v", dashboard.AverageMatchScore)
	}
}

func TestService_Dashboard_NoApplicationsHasZeroValuesNotErrors(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	appsRepo := applications.NewRepositoryFromQueries(q)
	analyticsRepo := analytics.NewRepositoryFromQueries(q)
	svc := analytics.NewService(analyticsRepo, appsRepo)
	ctx := context.Background()

	user, err := userRepo.CreateWithPassword(ctx, fmt.Sprintf("analytics-empty-%s@example.com", uuid.NewString()), "hash")
	if err != nil {
		t.Fatalf("create fixture user: %v", err)
	}

	dashboard, err := svc.Dashboard(ctx, user.ID)
	if err != nil {
		t.Fatalf("Dashboard: %v", err)
	}
	if dashboard.TotalApplications != 0 {
		t.Fatalf("expected 0 total applications, got %d", dashboard.TotalApplications)
	}
	if dashboard.ResponseRatePercent != 0 {
		t.Fatalf("expected 0%% response rate with no applications, got %v", dashboard.ResponseRatePercent)
	}
	if dashboard.AverageMatchScore != nil {
		t.Fatalf("expected nil average match score with no scored applications, got %v", *dashboard.AverageMatchScore)
	}
}
