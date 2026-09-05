// Package resumeversion generates versioned, downloadable PDF/DOCX resumes
// by merging approved tailoring suggestions onto a base master resume (see
// MASTER_REQUIREMENTS.md §36-§38).
package resumeversion

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// ErrNotFound is returned when a resume version doesn't exist (or isn't owned by the caller).
var ErrNotFound = errors.New("resume version not found")

// Version is the domain representation of a generated resume version.
type Version struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	BaseResumeID   uuid.UUID
	JobID          *uuid.UUID
	TailoringRunID *uuid.UUID
	VersionNumber  int32
	Content        aiclient.ResumeProfile
	MatchScore     *int32
	AlignmentScore *int32
	TailoringMode  *string
	PDFStorageKey  *string
	DocxStorageKey *string
	CreatedAt      time.Time
}

func fromRow(row db.ResumeVersion) Version {
	var content aiclient.ResumeProfile
	_ = json.Unmarshal(row.ContentJson, &content)
	return Version{
		ID:             database.PGToUUID(row.ID),
		UserID:         database.PGToUUID(row.UserID),
		BaseResumeID:   database.PGToUUID(row.BaseResumeID),
		JobID:          database.UUIDOrNil(row.JobID),
		TailoringRunID: database.UUIDOrNil(row.TailoringRunID),
		VersionNumber:  row.VersionNumber,
		Content:        content,
		MatchScore:     database.Int4OrNil(row.MatchScore),
		AlignmentScore: database.Int4OrNil(row.AlignmentScore),
		TailoringMode:  database.TextOrNil(row.TailoringMode),
		PDFStorageKey:  database.TextOrNil(row.PdfStorageKey),
		DocxStorageKey: database.TextOrNil(row.DocxStorageKey),
		CreatedAt:      row.CreatedAt.Time,
	}
}

// Repository provides access to resume_versions records.
type Repository struct {
	q *db.Queries
}

// NewRepository builds a Repository from a database pool.
func NewRepository(pool *database.Pool) *Repository {
	return &Repository{q: pool.Queries()}
}

// NewRepositoryFromQueries builds a Repository directly from generated
// Queries — used by tests that run against a transaction-scoped connection.
func NewRepositoryFromQueries(q *db.Queries) *Repository {
	return &Repository{q: q}
}

// NextVersionNumber returns the next version number for a base resume.
func (r *Repository) NextVersionNumber(ctx context.Context, baseResumeID uuid.UUID) (int32, error) {
	return r.q.GetNextResumeVersionNumber(ctx, database.UUIDToPG(baseResumeID))
}

// Create persists a new resume version row (documents not yet generated).
func (r *Repository) Create(
	ctx context.Context,
	userID, baseResumeID uuid.UUID,
	jobID, tailoringRunID *uuid.UUID,
	versionNumber int32,
	content aiclient.ResumeProfile,
	matchScore, alignmentScore *int32,
	tailoringMode *string,
) (Version, error) {
	contentJSON, err := json.Marshal(content)
	if err != nil {
		return Version{}, err
	}
	row, err := r.q.CreateResumeVersion(ctx, db.CreateResumeVersionParams{
		UserID:         database.UUIDToPG(userID),
		BaseResumeID:   database.UUIDToPG(baseResumeID),
		JobID:          database.PGUUID(jobID),
		TailoringRunID: database.PGUUID(tailoringRunID),
		VersionNumber:  versionNumber,
		ContentJson:    contentJSON,
		MatchScore:     database.PGInt4(matchScore),
		AlignmentScore: database.PGInt4(alignmentScore),
		TailoringMode:  database.PGText(tailoringMode),
	})
	if err != nil {
		return Version{}, err
	}
	return fromRow(row), nil
}

// SetDocuments records the storage keys of the generated PDF/DOCX files.
func (r *Repository) SetDocuments(ctx context.Context, id uuid.UUID, pdfKey, docxKey string) (Version, error) {
	row, err := r.q.SetResumeVersionDocuments(ctx, db.SetResumeVersionDocumentsParams{
		ID:             database.UUIDToPG(id),
		PdfStorageKey:  database.PGText(&pdfKey),
		DocxStorageKey: database.PGText(&docxKey),
	})
	if err != nil {
		return Version{}, err
	}
	return fromRow(row), nil
}

// GetForUser returns a resume version owned by the given user.
func (r *Repository) GetForUser(ctx context.Context, id, userID uuid.UUID) (Version, error) {
	row, err := r.q.GetResumeVersionForUser(ctx, db.GetResumeVersionForUserParams{
		ID:     database.UUIDToPG(id),
		UserID: database.UUIDToPG(userID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Version{}, ErrNotFound
		}
		return Version{}, err
	}
	return fromRow(row), nil
}

// ListForResume returns all versions for a base resume, newest first.
func (r *Repository) ListForResume(ctx context.Context, baseResumeID uuid.UUID) ([]Version, error) {
	rows, err := r.q.ListResumeVersionsForResume(ctx, database.UUIDToPG(baseResumeID))
	if err != nil {
		return nil, err
	}
	out := make([]Version, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromRow(row))
	}
	return out, nil
}
