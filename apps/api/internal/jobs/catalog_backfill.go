package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/lalithlochan/applyforge/apps/api/internal/background"
	"github.com/lalithlochan/applyforge/apps/api/internal/database"
)

const JobTypeCatalogBackfill = "backfill_job_catalog"

type CatalogBackfillPayload struct {
	AfterCreatedAt string `json:"after_created_at,omitempty"`
	AfterID        string `json:"after_id,omitempty"`
	BatchSize      int    `json:"batch_size,omitempty"`
}

type catalogBackfillRow struct {
	ID                           uuid.UUID
	Source                       string
	CompanyName                  string
	Title                        string
	Description                  string
	Country                      string
	State                        string
	City                         string
	LocationText                 string
	CountryCode                  string
	StateCode                    string
	WorkplaceType                string
	RemoteScope                  string
	EligibleCountryCodes         []string
	LocationConfidence           string
	RemoteType                   string
	EmploymentType               string
	PostedAt                     *time.Time
	CreatedAt                    time.Time
	JobFamily                    string
	RoleClassification           string
	RoleClassificationConfidence float32
}

type CatalogBackfillWorker struct {
	repo  *Repository
	queue *background.Queue
}

func NewCatalogBackfillWorker(repo *Repository, queue *background.Queue) *CatalogBackfillWorker {
	return &CatalogBackfillWorker{repo: repo, queue: queue}
}

func (w *CatalogBackfillWorker) Handle(ctx context.Context, job background.Job) error {
	var payload CatalogBackfillPayload
	if len(job.Payload) > 0 {
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("decode catalog backfill payload: %w", err)
		}
	}
	if payload.BatchSize <= 0 || payload.BatchSize > 1000 {
		payload.BatchSize = 250
	}

	var afterCreatedAt *time.Time
	var afterID uuid.UUID
	if payload.AfterCreatedAt != "" {
		parsed, err := time.Parse(time.RFC3339Nano, payload.AfterCreatedAt)
		if err != nil {
			return fmt.Errorf("invalid backfill cursor time: %w", err)
		}
		afterCreatedAt = &parsed
		if payload.AfterID == "" {
			return errors.New("backfill cursor id is required with cursor time")
		}
		afterID, err = uuid.Parse(payload.AfterID)
		if err != nil {
			return fmt.Errorf("invalid backfill cursor id: %w", err)
		}
	}

	rows, err := w.repo.listCatalogBackfillBatch(ctx, afterCreatedAt, afterID, payload.BatchSize)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	fingerprints := make(map[string]struct{})
	now := time.Now().UTC()
	for _, row := range rows {
		classification := classifyTitle(row.Title)
		if classification.Classification == "UNKNOWN" && row.RoleClassification != "" && row.RoleClassification != "UNKNOWN" {
			classification = RoleClassification{
				Family:         row.JobFamily,
				Classification: row.RoleClassification,
				Confidence:     row.RoleClassificationConfidence,
			}
		}

		raw := RawJob{
			Title:          row.Title,
			Description:    row.Description,
			Country:        row.Country,
			State:          row.State,
			City:           row.City,
			LocationText:   row.LocationText,
			RemoteType:     row.RemoteType,
			EmploymentType: row.EmploymentType,
			PostedAt:       row.PostedAt,
		}
		location := normalizeLocation(raw)
		if location.CountryCode == "" && row.CountryCode != "" {
			location.CountryCode = row.CountryCode
			location.EligibleCountryCodes = row.EligibleCountryCodes
			location.LocationConfidence = row.LocationConfidence
		}
		if location.StateCode == "" && row.StateCode != "" {
			location.StateCode = row.StateCode
		}
		if row.RemoteType == "" && row.WorkplaceType != "" {
			location.WorkplaceType = row.WorkplaceType
		}
		if location.RemoteScope == "UNKNOWN" && row.RemoteScope != "" {
			location.RemoteScope = row.RemoteScope
		}

		country := row.Country
		if country == "" && location.CountryCode == "US" {
			country = "United States"
		}
		state := firstNonEmpty(row.State, location.StateCode)
		city := firstNonEmpty(row.City, location.City)
		fingerprint := buildFingerprint(row.CompanyName, row.Title, row.LocationText, row.Description)

		if err := w.repo.updateCatalogBackfillMetadata(ctx, row.ID, catalogMetadataUpdate{
			NormalizedTitle:              normalizeTitle(row.Title),
			Country:                      country,
			State:                        state,
			City:                         city,
			CountryCode:                  location.CountryCode,
			StateCode:                    location.StateCode,
			WorkplaceType:                location.WorkplaceType,
			RemoteScope:                  location.RemoteScope,
			EligibleCountryCodes:         location.EligibleCountryCodes,
			LocationConfidence:           location.LocationConfidence,
			EmploymentType:               normalizeEmploymentType(row.EmploymentType),
			JobFamily:                    classification.Family,
			RoleClassification:           classification.Classification,
			RoleClassificationConfidence: classification.Confidence,
			ContentHash:                  contentHash(row.CompanyName, row.Title, row.LocationText, row.Description),
			Fingerprint:                  fingerprint,
		}); err != nil {
			return fmt.Errorf("backfill job %s: %w", row.ID, err)
		}

		if fingerprint != "" {
			fingerprints[fingerprint] = struct{}{}
		}

		if w.queue != nil &&
			location.CountryCode == "US" &&
			classification.Classification == "UNKNOWN" &&
			strings.TrimSpace(row.Description) != "" &&
			isFreshForEagerAI(row.PostedAt, now) {
			if err := w.queue.Enqueue(ctx, JobTypeClassifyRole, ClassifyRolePayload{JobID: row.ID.String()}, 3); err != nil {
				return fmt.Errorf("enqueue role classification during backfill: %w", err)
			}
		}
	}

	orderedFingerprints := make([]string, 0, len(fingerprints))
	for fingerprint := range fingerprints {
		orderedFingerprints = append(orderedFingerprints, fingerprint)
	}
	sort.Strings(orderedFingerprints)
	for _, fingerprint := range orderedFingerprints {
		if err := w.repo.ReconcileFingerprint(ctx, fingerprint); err != nil {
			return fmt.Errorf("reconcile fingerprint: %w", err)
		}
	}

	if len(rows) == payload.BatchSize && w.queue != nil {
		last := rows[len(rows)-1]
		next := CatalogBackfillPayload{
			AfterCreatedAt: last.CreatedAt.UTC().Format(time.RFC3339Nano),
			AfterID:        last.ID.String(),
			BatchSize:      payload.BatchSize,
		}
		if err := w.queue.Enqueue(ctx, JobTypeCatalogBackfill, next, 3); err != nil {
			return fmt.Errorf("enqueue next catalog backfill batch: %w", err)
		}
	}
	return nil
}

type catalogMetadataUpdate struct {
	NormalizedTitle              string
	Country                      string
	State                        string
	City                         string
	CountryCode                  string
	StateCode                    string
	WorkplaceType                string
	RemoteScope                  string
	EligibleCountryCodes         []string
	LocationConfidence           string
	EmploymentType               string
	JobFamily                    string
	RoleClassification           string
	RoleClassificationConfidence float32
	ContentHash                  string
	Fingerprint                  string
}

func (r *Repository) listCatalogBackfillBatch(
	ctx context.Context,
	afterCreatedAt *time.Time,
	afterID uuid.UUID,
	limit int,
) ([]catalogBackfillRow, error) {
	if r.pool == nil {
		return nil, errors.New("catalog backfill requires a repository backed by a database pool")
	}

	query := `
		SELECT
			id, source, company_name, title, description,
			country, state, city, location_text, country_code, state_code,
			workplace_type, remote_scope, eligible_country_codes, location_confidence,
			remote_type, employment_type, posted_at, created_at,
			job_family, role_classification, role_classification_confidence
		FROM jobs
	`
	args := []any{}
	if afterCreatedAt != nil {
		query += " WHERE (created_at, id) > ($1, $2)"
		args = append(args, *afterCreatedAt, afterID)
	}
	query += " ORDER BY created_at ASC, id ASC LIMIT $" + fmt.Sprint(len(args)+1)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]catalogBackfillRow, 0, limit)
	for rows.Next() {
		var (
			item           catalogBackfillRow
			country        pgtype.Text
			state          pgtype.Text
			city           pgtype.Text
			locationText   pgtype.Text
			countryCode    pgtype.Text
			stateCode      pgtype.Text
			remoteType     pgtype.Text
			employmentType pgtype.Text
			postedAt       pgtype.Timestamptz
		)
		if err := rows.Scan(
			&item.ID,
			&item.Source,
			&item.CompanyName,
			&item.Title,
			&item.Description,
			&country,
			&state,
			&city,
			&locationText,
			&countryCode,
			&stateCode,
			&item.WorkplaceType,
			&item.RemoteScope,
			&item.EligibleCountryCodes,
			&item.LocationConfidence,
			&remoteType,
			&employmentType,
			&postedAt,
			&item.CreatedAt,
			&item.JobFamily,
			&item.RoleClassification,
			&item.RoleClassificationConfidence,
		); err != nil {
			return nil, err
		}
		item.Country = valueOrEmpty(country)
		item.State = valueOrEmpty(state)
		item.City = valueOrEmpty(city)
		item.LocationText = valueOrEmpty(locationText)
		item.CountryCode = valueOrEmpty(countryCode)
		item.StateCode = valueOrEmpty(stateCode)
		item.RemoteType = valueOrEmpty(remoteType)
		item.EmploymentType = valueOrEmpty(employmentType)
		item.PostedAt = database.TimeOrNil(postedAt)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) updateCatalogBackfillMetadata(ctx context.Context, id uuid.UUID, update catalogMetadataUpdate) error {
	if r.pool == nil {
		return errors.New("catalog backfill requires a repository backed by a database pool")
	}
	eligible := update.EligibleCountryCodes
	if eligible == nil {
		eligible = []string{}
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE jobs
		SET normalized_title = $2,
			country = $3,
			state = $4,
			city = $5,
			country_code = $6,
			state_code = $7,
			workplace_type = $8,
			remote_scope = $9,
			eligible_country_codes = $10,
			location_confidence = $11,
			employment_type = $12,
			job_family = $13,
			role_classification = $14,
			role_classification_confidence = $15,
			content_hash = $16,
			fingerprint = $17,
			updated_at = now()
		WHERE id = $1
	`,
		id,
		update.NormalizedTitle,
		database.PGText(strOrNil(update.Country)),
		database.PGText(strOrNil(update.State)),
		database.PGText(strOrNil(update.City)),
		database.PGText(strOrNil(update.CountryCode)),
		database.PGText(strOrNil(update.StateCode)),
		firstNonEmpty(update.WorkplaceType, "ONSITE"),
		firstNonEmpty(update.RemoteScope, "UNKNOWN"),
		eligible,
		firstNonEmpty(update.LocationConfidence, "LOW"),
		database.PGText(strOrNil(update.EmploymentType)),
		firstNonEmpty(update.JobFamily, "UNKNOWN"),
		firstNonEmpty(update.RoleClassification, "UNKNOWN"),
		update.RoleClassificationConfidence,
		update.ContentHash,
		update.Fingerprint,
	)
	return err
}

func valueOrEmpty(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

type fingerprintMember struct {
	ID          uuid.UUID
	Source      string
	Status      string
	FirstSeenAt time.Time
}

// ReconcileFingerprint chooses one canonical row for the complete duplicate
// cluster. ACTIVE rows beat closed rows; within that group employer-direct
// ATS sources beat broad providers/aggregators; ties use earliest discovery.
func (r *Repository) ReconcileFingerprint(ctx context.Context, fingerprint string) error {
	if r.pool == nil || strings.TrimSpace(fingerprint) == "" {
		return nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, source, status, first_seen_at
		FROM jobs
		WHERE fingerprint = $1
		ORDER BY first_seen_at ASC, id ASC
	`, fingerprint)
	if err != nil {
		return err
	}
	defer rows.Close()

	var members []fingerprintMember
	for rows.Next() {
		var item fingerprintMember
		if err := rows.Scan(&item.ID, &item.Source, &item.Status, &item.FirstSeenAt); err != nil {
			return err
		}
		members = append(members, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(members) < 2 {
		return nil
	}

	best := members[0]
	for _, candidate := range members[1:] {
		if betterCanonicalCandidate(candidate, best) {
			best = candidate
		}
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		"UPDATE jobs SET canonical_job_id = NULL, updated_at = now() WHERE id = $1",
		best.ID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE jobs
		SET canonical_job_id = $2, updated_at = now()
		WHERE fingerprint = $1 AND id <> $2
	`, fingerprint, best.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func betterCanonicalCandidate(candidate, current fingerprintMember) bool {
	candidateActive := candidate.Status == "ACTIVE"
	currentActive := current.Status == "ACTIVE"
	if candidateActive != currentActive {
		return candidateActive
	}
	candidatePriority := sourcePriority(candidate.Source)
	currentPriority := sourcePriority(current.Source)
	if candidatePriority != currentPriority {
		return candidatePriority > currentPriority
	}
	return candidate.FirstSeenAt.Before(current.FirstSeenAt)
}
