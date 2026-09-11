package jobs

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
)

// ResolveCompanyByDiscoveredSources uses already-verified source ownership as
// company identity evidence. This is stronger than trying to fuzzy-match a
// provider's display name (for example "Deloitte") to a DOL legal entity name
// (for example "Deloitte Consulting LLP").
//
// A result is returned only when all matching verified source identities point
// to exactly one company. Any ambiguity deliberately falls back to normal
// company-name handling instead of attaching immigration history incorrectly.
func (r *Repository) ResolveCompanyByDiscoveredSources(
	ctx context.Context,
	discoveries []DiscoveredCompanySource,
) (uuid.UUID, string, bool, error) {
	if r.q == nil || len(discoveries) == 0 {
		return uuid.Nil, "", false, nil
	}

	type candidate struct {
		id   uuid.UUID
		name string
	}
	candidates := make(map[uuid.UUID]candidate)

	for _, discovery := range discoveries {
		if discovery.SourceType == "" || discovery.BoardToken == "" {
			continue
		}
		rows, err := r.q.ResolveCompanyBySourceIdentity(ctx, discovery.SourceType, discovery.BoardToken)
		if err != nil {
			return uuid.Nil, "", false, fmt.Errorf(
				"resolve company from %s source identity %q: %w",
				discovery.SourceType,
				discovery.BoardToken,
				err,
			)
		}
		for _, row := range rows {
			id := database.PGToUUID(row.ID)
			if id == uuid.Nil {
				continue
			}
			candidates[id] = candidate{id: id, name: row.Name}
		}
	}

	if len(candidates) != 1 {
		return uuid.Nil, "", false, nil
	}
	for _, resolved := range candidates {
		return resolved.id, resolved.name, true, nil
	}
	return uuid.Nil, "", false, nil
}
