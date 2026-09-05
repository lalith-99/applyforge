// Package skills provides deterministic skill-name normalization backed by
// the skill_aliases table (see MASTER_REQUIREMENTS.md §18). AI may help
// identify new aliases in the future, but canonical mapping always lives in
// application data, not model output.
package skills

import (
	"context"
	"strings"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
)

// Normalizer resolves raw skill names to a canonical display name.
type Normalizer struct {
	aliases map[string]string // lowercased alias -> canonical name
}

// NewNormalizer loads the alias table from the database.
func NewNormalizer(ctx context.Context, pool *database.Pool) (*Normalizer, error) {
	q := pool.Queries()
	rows, err := q.GetSkillAliasesAsMap(ctx)
	if err != nil {
		return nil, err
	}

	aliases := make(map[string]string, len(rows))
	for _, row := range rows {
		aliases[strings.ToLower(row.Alias)] = row.CanonicalName
	}
	return &Normalizer{aliases: aliases}, nil
}

// Canonical returns the canonical display name for a raw skill string,
// falling back to a trimmed copy of the input if no alias is known.
func (n *Normalizer) Canonical(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if canonical, ok := n.aliases[strings.ToLower(trimmed)]; ok {
		return canonical
	}
	return trimmed
}

// NormalizedKey returns a lowercase key suitable for de-duplication/lookup.
func NormalizedKey(displayName string) string {
	return strings.ToLower(strings.TrimSpace(displayName))
}
