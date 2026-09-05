package resume

import (
	"context"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
)

const maxUploadBytes = 10 << 20 // 10 MiB

var allowedMimeTypes = map[string]bool{
	"application/pdf": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
}

// Enqueuer schedules a parse_resume background job.
type Enqueuer interface {
	Enqueue(ctx context.Context, jobType string, payload any, maxAttempts int32) error
}

// Uploader stores the raw resume bytes.
type Uploader interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
}

// Handlers wires the resume Repository to HTTP routes.
type Handlers struct {
	repo    *Repository
	storage Uploader
	queue   Enqueuer
}

// NewHandlers builds resume Handlers.
func NewHandlers(repo *Repository, storageClient Uploader, queue Enqueuer) *Handlers {
	return &Handlers{repo: repo, storage: storageClient, queue: queue}
}

// Mount registers resume routes onto r. Callers must apply auth.RequireAuth
// before mounting, since every route here is user-scoped.
func (h *Handlers) Mount(r chi.Router) {
	r.Post("/resumes", h.handleUpload)
	r.Get("/resumes", h.handleList)
	r.Get("/resumes/{id}", h.handleGet)
}

func (h *Handlers) handleUpload(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "file too large or invalid upload")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if !allowedMimeTypes[mimeType] {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "only PDF and DOCX resumes are supported")
		return
	}

	resumeRecord, err := h.repo.Create(r.Context(), u.ID, header.Filename, mimeType, header.Size, "")
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not create resume record")
		return
	}

	storageKey := "resumes/" + u.ID.String() + "/" + resumeRecord.ID.String()
	if err := h.storage.Put(r.Context(), storageKey, file, header.Size, mimeType); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not store resume file")
		return
	}

	// storageKey is only known after the object is written; patch it in via a
	// second, cheap update rather than complicating Create's signature.
	if err := h.repo.SetStorageKey(r.Context(), resumeRecord.ID, storageKey); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not finalize resume upload")
		return
	}

	if err := h.queue.Enqueue(r.Context(), JobTypeParse, ParsePayload{ResumeID: resumeRecord.ID.String()}, 5); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not schedule resume parsing")
		return
	}

	resumeRecord.StorageKey = storageKey
	httpx.WriteJSON(w, http.StatusCreated, toSummary(resumeRecord))
}

func (h *Handlers) handleList(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	resumes, err := h.repo.ListForUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not list resumes")
		return
	}

	summaries := make([]map[string]any, 0, len(resumes))
	for _, res := range resumes {
		summaries = append(summaries, toSummary(res))
	}
	httpx.WriteJSON(w, http.StatusOK, summaries)
}

func (h *Handlers) handleGet(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid resume id")
		return
	}

	res, err := h.repo.Get(r.Context(), id, u.ID)
	if err != nil {
		if err == ErrNotFound {
			httpx.WriteError(w, http.StatusNotFound, "resume not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "could not load resume")
		return
	}

	experiences, err := h.repo.ListExperiences(r.Context(), res.ID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not load resume experiences")
		return
	}

	detail := toSummary(res)
	detail["parsed_profile"] = res.ParsedProfile
	detail["experiences"] = experiences
	httpx.WriteJSON(w, http.StatusOK, detail)
}

func toSummary(r Resume) map[string]any {
	return map[string]any{
		"id":                r.ID,
		"original_filename": r.OriginalFilename,
		"mime_type":         r.MimeType,
		"size_bytes":        r.SizeBytes,
		"status":            r.Status,
		"parse_error":       r.ParseError,
		"parsed_at":         r.ParsedAt,
		"created_at":        r.CreatedAt,
		"updated_at":        r.UpdatedAt,
	}
}
