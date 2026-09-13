package jobs

import (
	"context"
	"errors"
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

func TestRepository_SourceScopedSnapshotsDoNotCloseSiblingBoards(t *testing.T) {
	q, tx := testdb.OpenTxWithTransaction(t)
	repo := NewRepositoryFromTransaction(q, tx)
	ctx := context.Background()

	companyID, err := repo.UpsertCompany(ctx, "Acme Multi Board", fmt.Sprintf("acme-multi-%s", uuid.NewString()))
	if err != nil {
		t.Fatalf("UpsertCompany: %v", err)
	}
	sourceA, err := repo.CreateJobSource(ctx, "GREENHOUSE", companyID, "acme-engineering-"+uuid.NewString(), true)
	if err != nil {
		t.Fatalf("CreateJobSource A: %v", err)
	}
	sourceB, err := repo.CreateJobSource(ctx, "GREENHOUSE", companyID, "acme-product-"+uuid.NewString(), true)
	if err != nil {
		t.Fatalf("CreateJobSource B: %v", err)
	}

	newJob := func(externalID, title string) Job {
		return Job{
			Source:          "GREENHOUSE",
			ExternalID:      externalID,
			CompanyID:       companyID,
			CompanyName:     "Acme Multi Board",
			Title:           title,
			NormalizedTitle: normalizeTitle(title),
			Description:     "Build reliable systems",
			ContentHash:     contentHash("Acme Multi Board", title, "", "Build reliable systems"),
		}
	}

	a1, err := repo.UpsertJob(ctx, newJob("a1-"+uuid.NewString(), "Backend Engineer"))
	if err != nil {
		t.Fatalf("UpsertJob A1: %v", err)
	}
	a2, err := repo.UpsertJob(ctx, newJob("a2-"+uuid.NewString(), "Platform Engineer"))
	if err != nil {
		t.Fatalf("UpsertJob A2: %v", err)
	}
	b1, err := repo.UpsertJob(ctx, newJob("b1-"+uuid.NewString(), "Java Engineer"))
	if err != nil {
		t.Fatalf("UpsertJob B1: %v", err)
	}

	observeSnapshot := func(sourceID uuid.UUID, postings map[string]uuid.UUID) {
		t.Helper()
		generation, beginErr := repo.BeginSourcePoll(ctx, sourceID)
		if beginErr != nil {
			t.Fatalf("BeginSourcePoll: %v", beginErr)
		}
		startedAt := time.Now().UTC()
		ids := make([]string, 0, len(postings))
		for externalID, jobID := range postings {
			ids = append(ids, externalID)
			if observeErr := repo.ObserveSourcePosting(ctx, sourceID, generation, externalID, jobID, startedAt); observeErr != nil {
				t.Fatalf("ObserveSourcePosting: %v", observeErr)
			}
		}
		if _, finalizeErr := repo.FinalizeSourceSnapshot(ctx, sourceID, generation, ids, startedAt); finalizeErr != nil {
			t.Fatalf("FinalizeSourceSnapshot: %v", finalizeErr)
		}
	}

	observeSnapshot(sourceA, map[string]uuid.UUID{a1.Job.ExternalID: a1.Job.ID, a2.Job.ExternalID: a2.Job.ID})
	observeSnapshot(sourceB, map[string]uuid.UUID{b1.Job.ExternalID: b1.Job.ID})

	// Board A drops A2. Only A2 may close; board B's B1 membership is outside
	// the finalized source boundary even though provider and company match.
	generationA, err := repo.BeginSourcePoll(ctx, sourceA)
	if err != nil {
		t.Fatalf("BeginSourcePoll A2: %v", err)
	}
	pollStart := time.Now().UTC()
	if err := repo.ObserveSourcePosting(ctx, sourceA, generationA, a1.Job.ExternalID, a1.Job.ID, pollStart); err != nil {
		t.Fatalf("ObserveSourcePosting A1: %v", err)
	}
	closed, err := repo.FinalizeSourceSnapshot(ctx, sourceA, generationA, []string{a1.Job.ExternalID}, pollStart)
	if err != nil {
		t.Fatalf("FinalizeSourceSnapshot A: %v", err)
	}
	if closed != 1 {
		t.Fatalf("expected exactly A2 to close, got %d closures", closed)
	}

	for _, expectation := range []struct {
		id     uuid.UUID
		status string
	}{
		{a1.Job.ID, "ACTIVE"},
		{a2.Job.ID, "CLOSED"},
		{b1.Job.ID, "ACTIVE"},
	} {
		job, getErr := repo.GetByID(ctx, expectation.id)
		if getErr != nil {
			t.Fatalf("GetByID: %v", getErr)
		}
		if job.Status != expectation.status {
			t.Fatalf("job %s: expected %s, got %s", expectation.id, expectation.status, job.Status)
		}
	}

	// A later generation fences an older worker, and an anomalous empty board
	// cannot erase source B's previously active inventory.
	staleGeneration, err := repo.BeginSourcePoll(ctx, sourceA)
	if err != nil {
		t.Fatalf("BeginSourcePoll stale: %v", err)
	}
	if _, err := repo.BeginSourcePoll(ctx, sourceA); err != nil {
		t.Fatalf("BeginSourcePoll current: %v", err)
	}
	if _, err := repo.FinalizeSourceSnapshot(ctx, sourceA, staleGeneration, []string{a1.Job.ExternalID}, time.Now().UTC()); !errors.Is(err, ErrStaleSourcePoll) {
		t.Fatalf("expected ErrStaleSourcePoll, got %v", err)
	}

	emptyGeneration, err := repo.BeginSourcePoll(ctx, sourceB)
	if err != nil {
		t.Fatalf("BeginSourcePoll empty: %v", err)
	}
	if _, err := repo.FinalizeSourceSnapshot(ctx, sourceB, emptyGeneration, nil, time.Now().UTC()); !errors.Is(err, ErrSuspiciousEmptySnapshot) {
		t.Fatalf("expected ErrSuspiciousEmptySnapshot, got %v", err)
	}
	stillActive, err := repo.GetByID(ctx, b1.Job.ID)
	if err != nil {
		t.Fatalf("GetByID B1: %v", err)
	}
	if stillActive.Status != "ACTIVE" {
		t.Fatalf("expected B1 to remain ACTIVE after empty snapshot, got %s", stillActive.Status)
	}
}

type staticJobSource struct {
	jobs []RawJob
}

func (s staticJobSource) Name() string { return "GREENHOUSE" }
func (s staticJobSource) Fetch(context.Context, *Cursor) ([]RawJob, *Cursor, error) {
	return s.jobs, nil, nil
}

func TestIngestionService_PartialPersistenceDoesNotPublishClosure(t *testing.T) {
	q, tx := testdb.OpenTxWithTransaction(t)
	repo := NewRepositoryFromTransaction(q, tx)
	service := NewIngestionService(repo, nil)
	ctx := context.Background()

	companyID, err := repo.UpsertCompany(ctx, "Acme Partial Poll", fmt.Sprintf("acme-partial-%s", uuid.NewString()))
	if err != nil {
		t.Fatalf("UpsertCompany: %v", err)
	}
	sourceID, err := repo.CreateJobSource(ctx, "GREENHOUSE", companyID, "acme-partial-"+uuid.NewString(), true)
	if err != nil {
		t.Fatalf("CreateJobSource: %v", err)
	}
	cfg := JobSourceConfig{ID: sourceID, SourceType: "GREENHOUSE", CompanyID: companyID, CompanyName: "Acme Partial Poll"}
	firstID := "first-" + uuid.NewString()
	secondID := "second-" + uuid.NewString()

	initial := staticJobSource{jobs: []RawJob{
		{ExternalID: firstID, Title: "Backend Engineer", Description: "Build APIs"},
		{ExternalID: secondID, Title: "Platform Engineer", Description: "Build platforms"},
	}}
	if _, err := service.Ingest(ctx, cfg, "GREENHOUSE", initial); err != nil {
		t.Fatalf("initial Ingest: %v", err)
	}

	// The empty external ID makes source-membership persistence fail after the
	// first posting succeeds. The previous second posting must remain active
	// because this incomplete poll is never finalized.
	partial := staticJobSource{jobs: []RawJob{
		{ExternalID: firstID, Title: "Backend Engineer", Description: "Build APIs"},
		{ExternalID: "", Title: "Invalid Posting", Description: "Missing provider identity"},
	}}
	if _, err := service.Ingest(ctx, cfg, "GREENHOUSE", partial); err == nil {
		t.Fatal("expected partial persistence error")
	}

	var secondStatus string
	if err := tx.QueryRow(ctx, `SELECT status FROM jobs WHERE source = 'GREENHOUSE' AND external_id = $1`, secondID).Scan(&secondStatus); err != nil {
		t.Fatalf("query second job: %v", err)
	}
	if secondStatus != "ACTIVE" {
		t.Fatalf("expected previously seen job to remain ACTIVE after partial poll, got %s", secondStatus)
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

	fp := buildFingerprint("Acme Dedupe Co", "Senior Backend Engineer", "Remote - US", "Build things")

	first, err := repo.UpsertJob(ctx, Job{
		Source:          "GREENHOUSE",
		ExternalID:      uuid.NewString(),
		CompanyID:       companyID,
		CompanyName:     "Acme Dedupe Co",
		Title:           "Senior Backend Engineer",
		NormalizedTitle: normalizeTitle("Senior Backend Engineer"),
		RemoteType:      strPtrTest("remote"),
		LocationText:    strPtrTest("Remote - US"),
		Description:     "Build things",
		ContentHash:     contentHash("Acme Dedupe Co", "Senior Backend Engineer", "Remote - US", "Build things"),
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
		LocationText:    strPtrTest("Remote - US"),
		Description:     "Build things",
		ContentHash:     contentHash("Acme Dedupe Co", "Senior Backend Engineer", "Remote - US", "Build things"),
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

func TestRepository_List_UsesExactCountryAndStrictPostedAt(t *testing.T) {
	q := testdb.OpenTx(t)
	repo := &Repository{q: q}
	ctx := context.Background()

	testID := uuid.NewString()
	companyID, err := repo.UpsertCompany(ctx, "Location Filter Co", "location-filter-"+testID)
	if err != nil {
		t.Fatalf("UpsertCompany: %v", err)
	}
	title := "Location Filter Backend Engineer " + testID
	now := time.Now().UTC()
	us := "US"
	australia := "AU"
	base := Job{
		Source:          "GREENHOUSE",
		CompanyID:       companyID,
		CompanyName:     "Location Filter Co",
		Title:           title,
		NormalizedTitle: normalizeTitle(title),
		Description:     "Build location-safe filters",
		ContentHash:     contentHash("Location Filter Co", title, "", "Build location-safe filters"),
	}

	for _, job := range []Job{
		{ExternalID: uuid.NewString(), CountryCode: &us, PostedAt: &now},
		{ExternalID: uuid.NewString(), CountryCode: &australia, PostedAt: &now},
		{ExternalID: uuid.NewString(), CountryCode: &us},
	} {
		job.Source = base.Source
		job.CompanyID = base.CompanyID
		job.CompanyName = base.CompanyName
		job.Title = base.Title
		job.NormalizedTitle = base.NormalizedTitle
		job.Description = base.Description
		job.ContentHash = base.ContentHash + job.ExternalID
		if _, err := repo.UpsertJob(ctx, job); err != nil {
			t.Fatalf("UpsertJob: %v", err)
		}
	}

	cutoff := now.Add(-time.Hour)
	jobs, total, err := repo.List(ctx, ListFilter{Search: testID, CountryCode: "US", PostedAfter: &cutoff})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 || len(jobs) != 1 || jobs[0].CountryCode == nil || *jobs[0].CountryCode != "US" {
		t.Fatalf("expected exactly the current US job, got total=%d jobs=%+v", total, jobs)
	}
}
