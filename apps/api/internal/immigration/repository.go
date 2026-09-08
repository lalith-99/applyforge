package immigration

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
)

const (
	ProgramLCAH1B = "LCA_H1B"
	ProgramPERM   = "PERM"
)

var (
	nonEmployerWord = regexp.MustCompile(`[^a-z0-9]+`)
	legalSuffixes   = map[string]bool{
		"inc": true, "incorporated": true, "llc": true, "corp": true, "corporation": true,
		"ltd": true, "limited": true, "pllc": true, "lp": true, "llp": true, "co": true, "company": true,
	}
)

type EvidenceRow struct {
	EmployerName       string     `json:"employer_name"`
	Program            string     `json:"program"`
	FiscalYear         int        `json:"fiscal_year"`
	CertifiedCount     int        `json:"certified_count"`
	DeniedCount        int        `json:"denied_count"`
	WithdrawnCount     int        `json:"withdrawn_count"`
	OtherCount         int        `json:"other_count"`
	TotalCount         int        `json:"total_count"`
	LatestDecisionDate *time.Time `json:"latest_decision_date,omitempty"`
}

type ImportBatch struct {
	SourceRelease string        `json:"source_release"`
	SourceURL     string        `json:"source_url"`
	Rows          []EvidenceRow `json:"rows"`
}

type CompanyEvidence struct {
	H1BCertified     int      `json:"h1b_certified"`
	H1BTotal         int      `json:"h1b_total"`
	PERMCertified    int      `json:"perm_certified"`
	PERMTotal        int      `json:"perm_total"`
	LatestFiscalYear int      `json:"latest_fiscal_year"`
	MatchedEmployers []string `json:"matched_employers"`
}

type Repository struct {
	pool *database.Pool
}

func NewRepository(pool *database.Pool) *Repository {
	return &Repository{pool: pool}
}

func NormalizeEmployerName(name string) string {
	words := strings.Fields(nonEmployerWord.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), " "))
	for len(words) > 1 && legalSuffixes[words[len(words)-1]] {
		words = words[:len(words)-1]
	}
	return strings.Join(words, " ")
}

func (r *Repository) Import(ctx context.Context, batch ImportBatch) (int, error) {
	if strings.TrimSpace(batch.SourceRelease) == "" {
		return 0, errors.New("source_release is required")
	}
	if strings.TrimSpace(batch.SourceURL) == "" {
		return 0, errors.New("source_url is required")
	}
	if len(batch.Rows) == 0 {
		return 0, nil
	}
	if len(batch.Rows) > 2000 {
		return 0, errors.New("immigration evidence import batch exceeds 2000 rows")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	imported := 0
	for _, row := range batch.Rows {
		if err := validateEvidenceRow(row); err != nil {
			return 0, err
		}
		normalized := NormalizeEmployerName(row.EmployerName)
		if normalized == "" {
			continue
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO company_immigration_evidence (
				employer_name,
				employer_normalized_name,
				program,
				fiscal_year,
				certified_count,
				denied_count,
				withdrawn_count,
				other_count,
				total_count,
				latest_decision_date,
				source_release,
				source_url,
				imported_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now()
			)
			ON CONFLICT (employer_normalized_name, program, fiscal_year, source_release)
			DO UPDATE SET
				employer_name = EXCLUDED.employer_name,
				certified_count = EXCLUDED.certified_count,
				denied_count = EXCLUDED.denied_count,
				withdrawn_count = EXCLUDED.withdrawn_count,
				other_count = EXCLUDED.other_count,
				total_count = EXCLUDED.total_count,
				latest_decision_date = EXCLUDED.latest_decision_date,
				source_url = EXCLUDED.source_url,
				imported_at = now()
		`,
			row.EmployerName,
			normalized,
			row.Program,
			row.FiscalYear,
			row.CertifiedCount,
			row.DeniedCount,
			row.WithdrawnCount,
			row.OtherCount,
			row.TotalCount,
			row.LatestDecisionDate,
			batch.SourceRelease,
			batch.SourceURL,
		)
		if err != nil {
			return 0, fmt.Errorf("upsert immigration evidence for %q: %w", row.EmployerName, err)
		}

		// Auto-link only exact normalized matches. Legal/division aliases that
		// do not match exactly must be explicitly reviewed and inserted.
		_, err = tx.Exec(ctx, `
			INSERT INTO company_immigration_aliases (
				company_id,
				evidence_employer_normalized_name,
				match_method,
				confidence
			)
			SELECT id, $1, 'EXACT_NORMALIZED', 1.0
			FROM companies
			WHERE normalized_name = $1
			ON CONFLICT DO NOTHING
		`, normalized)
		if err != nil {
			return 0, fmt.Errorf("link immigration employer alias %q: %w", normalized, err)
		}
		imported++
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return imported, nil
}

func validateEvidenceRow(row EvidenceRow) error {
	if strings.TrimSpace(row.EmployerName) == "" {
		return errors.New("employer_name is required")
	}
	if row.Program != ProgramLCAH1B && row.Program != ProgramPERM {
		return fmt.Errorf("unsupported immigration evidence program %q", row.Program)
	}
	if row.FiscalYear < 2000 || row.FiscalYear > 2200 {
		return fmt.Errorf("invalid fiscal_year %d", row.FiscalYear)
	}
	for name, value := range map[string]int{
		"certified_count": row.CertifiedCount,
		"denied_count":    row.DeniedCount,
		"withdrawn_count": row.WithdrawnCount,
		"other_count":     row.OtherCount,
		"total_count":     row.TotalCount,
	} {
		if value < 0 {
			return fmt.Errorf("%s cannot be negative", name)
		}
	}
	return nil
}

func (r *Repository) GetForCompany(ctx context.Context, companyID uuid.UUID, companyName string) (CompanyEvidence, error) {
	currentFY := time.Now().UTC().Year()
	if time.Now().UTC().Month() >= time.October {
		currentFY++
	}
	minFY := currentFY - 2
	normalizedName := NormalizeEmployerName(companyName)

	var evidence CompanyEvidence
	var employers []string
	err := r.pool.QueryRow(ctx, `
		WITH employer_keys AS (
			SELECT normalized_name AS employer_key
			FROM companies
			WHERE id = $1

			UNION

			SELECT evidence_employer_normalized_name
			FROM company_immigration_aliases
			WHERE company_id = $1

			UNION

			SELECT $2::text
			WHERE $2::text <> ''
		),
		matched AS (
			SELECT e.*
			FROM company_immigration_evidence e
			WHERE e.fiscal_year >= $3
			  AND EXISTS (
			      SELECT 1
			      FROM employer_keys k
			      WHERE e.employer_normalized_name = k.employer_key
			         OR (
			             length(k.employer_key) >= 6
			             AND e.employer_normalized_name LIKE k.employer_key || ' %'
			         )
			         OR (
			             length(e.employer_normalized_name) >= 6
			             AND k.employer_key LIKE e.employer_normalized_name || ' %'
			         )
			  )
		)
		SELECT
			COALESCE(sum(certified_count) FILTER (WHERE program = 'LCA_H1B'), 0)::int,
			COALESCE(sum(total_count) FILTER (WHERE program = 'LCA_H1B'), 0)::int,
			COALESCE(sum(certified_count) FILTER (WHERE program = 'PERM'), 0)::int,
			COALESCE(sum(total_count) FILTER (WHERE program = 'PERM'), 0)::int,
			COALESCE(max(fiscal_year), 0)::int,
			COALESCE(array_agg(DISTINCT employer_name) FILTER (WHERE employer_name IS NOT NULL), ARRAY[]::text[])
		FROM matched
	`, companyID, normalizedName, minFY).Scan(
		&evidence.H1BCertified,
		&evidence.H1BTotal,
		&evidence.PERMCertified,
		&evidence.PERMTotal,
		&evidence.LatestFiscalYear,
		&employers,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CompanyEvidence{}, nil
		}
		return CompanyEvidence{}, err
	}
	evidence.MatchedEmployers = employers
	return evidence, nil
}

type SponsorWatchlistSummary struct {
	Total            int `json:"total"`
	Hot              int `json:"hot"`
	Warm             int `json:"warm"`
	Cool             int `json:"cool"`
	Cold             int `json:"cold"`
	ResolvedSources  int `json:"resolved_sources"`
	PartialSources   int `json:"partial_sources"`
	PendingSources   int `json:"pending_sources"`
	FailedSources    int `json:"failed_sources"`
}

func (r *Repository) RefreshSponsorWatchlist(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 50000 {
		return 0, fmt.Errorf("watchlist limit must be between 1 and 50000")
	}
	var count int
	if err := r.pool.QueryRow(ctx, "SELECT refresh_company_sponsor_watchlist($1)", limit).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) GetSponsorWatchlistSummary(ctx context.Context) (SponsorWatchlistSummary, error) {
	var out SponsorWatchlistSummary
	err := r.pool.QueryRow(ctx, `
		SELECT
			count(*)::int,
			count(*) FILTER (WHERE tier = 'HOT')::int,
			count(*) FILTER (WHERE tier = 'WARM')::int,
			count(*) FILTER (WHERE tier = 'COOL')::int,
			count(*) FILTER (WHERE tier = 'COLD')::int,
			count(*) FILTER (WHERE source_discovery_status = 'RESOLVED')::int,
			count(*) FILTER (WHERE source_discovery_status = 'PARTIAL')::int,
			count(*) FILTER (WHERE source_discovery_status = 'PENDING')::int,
			count(*) FILTER (WHERE source_discovery_status = 'FAILED')::int
		FROM company_sponsor_watchlist
	`).Scan(
		&out.Total,
		&out.Hot,
		&out.Warm,
		&out.Cool,
		&out.Cold,
		&out.ResolvedSources,
		&out.PartialSources,
		&out.PendingSources,
		&out.FailedSources,
	)
	return out, err
}
