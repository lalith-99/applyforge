package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

// ResolveCompanyBySourceIdentityRow is intentionally handwritten rather than
// generated from the general company queries: source identity belongs to the
// acquisition control plane and joins the source registry/job source tables.
type ResolveCompanyBySourceIdentityRow struct {
	ID   pgtype.UUID `json:"id"`
	Name string      `json:"name"`
}

const resolveCompanyBySourceIdentity = `
WITH source_companies AS (
    SELECT js.company_id
    FROM job_sources js
    WHERE js.source_type = $1
      AND js.board_token = $2
      AND js.enabled = true

    UNION

    SELECT csr.company_id
    FROM company_source_registry csr
    WHERE csr.source_type = $1
      AND csr.board_token = $2
      AND csr.monitorable = true
      AND csr.confidence >= 0.90
)
SELECT DISTINCT c.id, c.name
FROM source_companies sc
JOIN companies c ON c.id = sc.company_id
ORDER BY c.id
LIMIT 2
`

// ResolveCompanyBySourceIdentity returns up to two companies on purpose. The
// caller may resolve the source only when exactly one canonical company owns
// the verified source identity; two rows means the registry is ambiguous and
// must not be guessed through.
func (q *Queries) ResolveCompanyBySourceIdentity(
	ctx context.Context,
	sourceType string,
	boardToken string,
) ([]ResolveCompanyBySourceIdentityRow, error) {
	rows, err := q.db.Query(ctx, resolveCompanyBySourceIdentity, sourceType, boardToken)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ResolveCompanyBySourceIdentityRow, 0, 2)
	for rows.Next() {
		var item ResolveCompanyBySourceIdentityRow
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
