package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

// TailoringSuggestionMetadata is supplemental model-controlled metadata kept
// separate from the sqlc-generated core suggestion row to avoid widening the
// stable query contract for existing callers.
type TailoringSuggestionMetadata struct {
	ID              pgtype.UUID
	SkillCategories []byte
	Operation       string
	TargetCompany   pgtype.Text
	TargetTitle     pgtype.Text
}

func (q *Queries) UpdateTailoringSuggestionMetadata(
	ctx context.Context,
	id pgtype.UUID,
	skillCategories string,
	operation string,
	targetCompany pgtype.Text,
	targetTitle pgtype.Text,
) error {
	_, err := q.db.Exec(
		ctx,
		`UPDATE tailoring_suggestions
         SET skill_categories = $2::jsonb,
             operation = $3,
             target_company = $4,
             target_title = $5,
             updated_at = now()
         WHERE id = $1`,
		id,
		skillCategories,
		operation,
		targetCompany,
		targetTitle,
	)
	return err
}

func (q *Queries) GetTailoringSuggestionMetadata(
	ctx context.Context,
	id pgtype.UUID,
) (TailoringSuggestionMetadata, error) {
	row := q.db.QueryRow(
		ctx,
		`SELECT id, skill_categories, operation, target_company, target_title
         FROM tailoring_suggestions
         WHERE id = $1`,
		id,
	)
	var out TailoringSuggestionMetadata
	err := row.Scan(
		&out.ID,
		&out.SkillCategories,
		&out.Operation,
		&out.TargetCompany,
		&out.TargetTitle,
	)
	return out, err
}

func (q *Queries) ListTailoringSuggestionMetadata(
	ctx context.Context,
	runID pgtype.UUID,
) ([]TailoringSuggestionMetadata, error) {
	rows, err := q.db.Query(
		ctx,
		`SELECT id, skill_categories, operation, target_company, target_title
         FROM tailoring_suggestions
         WHERE tailoring_run_id = $1`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []TailoringSuggestionMetadata{}
	for rows.Next() {
		var item TailoringSuggestionMetadata
		if err := rows.Scan(
			&item.ID,
			&item.SkillCategories,
			&item.Operation,
			&item.TargetCompany,
			&item.TargetTitle,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
