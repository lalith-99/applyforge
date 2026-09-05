// Package resume owns the master resume domain: upload metadata, storage
// key, extracted text, and the AI-parsed structured profile (see
// MASTER_REQUIREMENTS.md §11). The original document is never modified by
// downstream tailoring.
package resume

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// ErrNotFound is returned when a resume does not exist (or isn't owned by the caller).
var ErrNotFound = errors.New("resume not found")

// Status values (mirrors the resumes.status check constraint).
const (
	StatusUploaded = "UPLOADED"
	StatusParsing  = "PARSING"
	StatusParsed   = "PARSED"
	StatusFailed   = "FAILED"
)

// Resume is the domain representation of an uploaded master resume.
type Resume struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	OriginalFilename string
	MimeType         string
	SizeBytes        int64
	StorageKey       string
	Status           string
	ParseError       *string
	RawText          *string
	ParsedProfile    json.RawMessage
	ParsedAt         *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func fromRow(row db.Resume) Resume {
	return Resume{
		ID:               database.PGToUUID(row.ID),
		UserID:           database.PGToUUID(row.UserID),
		OriginalFilename: row.OriginalFilename,
		MimeType:         row.MimeType,
		SizeBytes:        row.SizeBytes,
		StorageKey:       row.StorageKey,
		Status:           row.Status,
		ParseError:       database.TextOrNil(row.ParseError),
		RawText:          database.TextOrNil(row.RawText),
		ParsedProfile:    row.ParsedProfile,
		ParsedAt:         database.TimeOrNil(row.ParsedAt),
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
	}
}

// Experience is a single structured work-history entry parsed from a resume.
type Experience struct {
	ID             uuid.UUID
	ResumeID       uuid.UUID
	DisplayOrder   int32
	Company        *string
	Title          *string
	StartDate      *string
	EndDate        *string
	Location       *string
	Bullets        []string
	DetectedSkills []string
	Technologies   []string
}

func experienceFromRow(row db.ResumeExperience) Experience {
	return Experience{
		ID:             database.PGToUUID(row.ID),
		ResumeID:       database.PGToUUID(row.ResumeID),
		DisplayOrder:   row.DisplayOrder,
		Company:        database.TextOrNil(row.Company),
		Title:          database.TextOrNil(row.Title),
		StartDate:      database.TextOrNil(row.StartDate),
		EndDate:        database.TextOrNil(row.EndDate),
		Location:       database.TextOrNil(row.Location),
		Bullets:        row.Bullets,
		DetectedSkills: row.DetectedSkills,
		Technologies:   row.Technologies,
	}
}

// Repository provides access to resume records.
type Repository struct {
	q *db.Queries
}

// NewRepository builds a Repository from a database pool.
func NewRepository(pool *database.Pool) *Repository {
	return newRepository(pool.Queries())
}

// NewRepositoryFromQueries builds a Repository directly from generated
// Queries — used by tests that run against a transaction-scoped connection.
func NewRepositoryFromQueries(q *db.Queries) *Repository {
	return newRepository(q)
}

func newRepository(q *db.Queries) *Repository {
	return &Repository{q: q}
}

// Create stores metadata for a newly uploaded resume.
func (r *Repository) Create(ctx context.Context, userID uuid.UUID, filename, mimeType string, size int64, storageKey string) (Resume, error) {
	row, err := r.q.CreateResume(ctx, db.CreateResumeParams{
		UserID:           database.UUIDToPG(userID),
		OriginalFilename: filename,
		MimeType:         mimeType,
		SizeBytes:        size,
		StorageKey:       storageKey,
	})
	if err != nil {
		return Resume{}, err
	}
	return fromRow(row), nil
}

// SetStorageKey records where the uploaded file was written in object storage.
func (r *Repository) SetStorageKey(ctx context.Context, id uuid.UUID, storageKey string) error {
	return r.q.SetResumeStorageKey(ctx, db.SetResumeStorageKeyParams{
		ID:         database.UUIDToPG(id),
		StorageKey: storageKey,
	})
}

// Get returns a resume owned by the given user.
func (r *Repository) Get(ctx context.Context, id, userID uuid.UUID) (Resume, error) {
	row, err := r.q.GetResumeForUser(ctx, db.GetResumeForUserParams{
		ID:     database.UUIDToPG(id),
		UserID: database.UUIDToPG(userID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Resume{}, ErrNotFound
		}
		return Resume{}, err
	}
	return fromRow(row), nil
}

// GetByID returns a resume by id regardless of owner (used by background workers).
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (Resume, error) {
	row, err := r.q.GetResumeByID(ctx, database.UUIDToPG(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Resume{}, ErrNotFound
		}
		return Resume{}, err
	}
	return fromRow(row), nil
}

// ListForUser returns all resumes owned by a user, newest first.
func (r *Repository) ListForUser(ctx context.Context, userID uuid.UUID) ([]Resume, error) {
	rows, err := r.q.ListResumesForUser(ctx, database.UUIDToPG(userID))
	if err != nil {
		return nil, err
	}
	resumes := make([]Resume, 0, len(rows))
	for _, row := range rows {
		resumes = append(resumes, fromRow(row))
	}
	return resumes, nil
}

// Delete permanently removes a resume owned by the given user (via ON
// DELETE CASCADE this also removes its resume_experiences and
// resume_versions rows). The caller is responsible for deleting the
// underlying object storage files first — this only removes DB rows.
func (r *Repository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return r.q.DeleteResume(ctx, db.DeleteResumeParams{
		ID:     database.UUIDToPG(id),
		UserID: database.UUIDToPG(userID),
	})
}

// MarkParsing flips a resume's status to PARSING.
func (r *Repository) MarkParsing(ctx context.Context, id uuid.UUID) error {
	return r.q.MarkResumeParsing(ctx, database.UUIDToPG(id))
} // MarkParsed stores the extracted text and structured profile for a resume.
func (r *Repository) MarkParsed(ctx context.Context, id uuid.UUID, rawText string, parsedProfile json.RawMessage) error {
	return r.q.MarkResumeParsed(ctx, db.MarkResumeParsedParams{
		ID:            database.UUIDToPG(id),
		RawText:       database.PGText(&rawText),
		ParsedProfile: parsedProfile,
	})
}

// MarkFailed records a parsing failure reason for a resume.
func (r *Repository) MarkFailed(ctx context.Context, id uuid.UUID, reason string) error {
	return r.q.MarkResumeFailed(ctx, db.MarkResumeFailedParams{
		ID:         database.UUIDToPG(id),
		ParseError: database.PGText(&reason),
	})
}

// ReplaceExperiences deletes any existing parsed experiences for a resume and inserts new ones.
func (r *Repository) ReplaceExperiences(ctx context.Context, resumeID uuid.UUID, experiences []Experience) error {
	if err := r.q.DeleteResumeExperiences(ctx, database.UUIDToPG(resumeID)); err != nil {
		return err
	}
	for i, exp := range experiences {
		_, err := r.q.CreateResumeExperience(ctx, db.CreateResumeExperienceParams{
			ResumeID:       database.UUIDToPG(resumeID),
			DisplayOrder:   int32(i),
			Company:        database.PGText(exp.Company),
			Title:          database.PGText(exp.Title),
			StartDate:      database.PGText(exp.StartDate),
			EndDate:        database.PGText(exp.EndDate),
			Location:       database.PGText(exp.Location),
			Bullets:        orEmpty(exp.Bullets),
			DetectedSkills: orEmpty(exp.DetectedSkills),
			Technologies:   orEmpty(exp.Technologies),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// ListExperiences returns the parsed experiences for a resume, in display order.
func (r *Repository) ListExperiences(ctx context.Context, resumeID uuid.UUID) ([]Experience, error) {
	rows, err := r.q.ListResumeExperiences(ctx, database.UUIDToPG(resumeID))
	if err != nil {
		return nil, err
	}
	experiences := make([]Experience, 0, len(rows))
	for _, row := range rows {
		experiences = append(experiences, experienceFromRow(row))
	}
	return experiences, nil
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
