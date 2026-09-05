package database

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// UUIDToPG converts a uuid.UUID to the pgx representation used by generated queries.
func UUIDToPG(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// PGToUUID converts a pgx UUID into uuid.UUID. Returns uuid.Nil if not valid.
func PGToUUID(id pgtype.UUID) uuid.UUID {
	if !id.Valid {
		return uuid.Nil
	}
	return uuid.UUID(id.Bytes)
}

// PGUUID converts a nullable uuid.UUID into pgtype.UUID.
func PGUUID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

// UUIDOrNil converts pgtype.UUID into a nullable uuid.UUID.
func UUIDOrNil(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	v := uuid.UUID(id.Bytes)
	return &v
}

// PGText converts a nullable string into pgtype.Text.
func PGText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// TextOrNil converts pgtype.Text into a nullable string.
func TextOrNil(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

// PGInt4 converts a nullable int32 into pgtype.Int4.
func PGInt4(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}

// Int4OrNil converts pgtype.Int4 into a nullable int32.
func Int4OrNil(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	return &v.Int32
}

// PGTimestamptz converts a nullable time.Time into pgtype.Timestamptz.
func PGTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// TimeOrNil converts pgtype.Timestamptz into a nullable time.Time.
func TimeOrNil(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}
