package migrations

import "embed"

// FS contains the versioned goose SQL migrations for the release migration
// binary. Embedding them makes production migrations independent of local
// filesystem layout and CLI installation.
var (
	//go:embed *.sql
	FS embed.FS
)
