// Package http contains note HTTP handlers.
package http

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"prod-go/internal/modules/notes"
	"prod-go/internal/platform/httpx"
)

// NoteService lists the use cases required by the notes transport layer.
type NoteService interface {
	Create(context.Context, string, string, string) (*notes.Note, error)
	Get(context.Context, string, string) (*notes.Note, error)
	List(context.Context, string, int, string) ([]notes.Note, error)
	Update(context.Context, *notes.Note) (*notes.Note, error)
	Delete(context.Context, string, string, int) error
}

// Register mounts authenticated note CRUD routes on the provided mux.
func Register(mux *http.ServeMux, service NoteService, validateToken func(string) (string, error)) {
	h := &handler{service: service}
	register := func(method string, target http.HandlerFunc) {
		mux.Handle(method+" /api/v1/notes", httpx.Authenticate(validateToken, target))
	}
	register(http.MethodPost, h.create)
	register(http.MethodGet, h.list)
	mux.Handle("GET /api/v1/notes/{id}", httpx.Authenticate(validateToken, http.HandlerFunc(h.get)))
	mux.Handle("PUT /api/v1/notes/{id}", httpx.Authenticate(validateToken, http.HandlerFunc(h.replace)))
	mux.Handle("PATCH /api/v1/notes/{id}", httpx.Authenticate(validateToken, http.HandlerFunc(h.patch)))
	mux.Handle("DELETE /api/v1/notes/{id}", httpx.Authenticate(validateToken, http.HandlerFunc(h.delete)))
}

type handler struct {
	service NoteService
}

type noteInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Version int    `json:"version,omitempty"`
}

type noteResponse struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type listResponse struct {
	Items      []noteResponse `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	var input noteInput
	if !decodeJSON(w, r, &input) {
		return
	}
	note, err := h.service.Create(r.Context(), httpx.AuthenticatedUserID(r.Context()), input.Title, input.Content)
	if err != nil {
		httpx.WriteProblemFor(w, r, http.StatusUnprocessableEntity, "Unprocessable Content", err.Error())
		return
	}
	w.Header().Set("Location", "/api/v1/notes/"+note.ID)
	w.Header().Set("ETag", versionETag(note.Version))
	_ = httpx.WriteJSON(w, http.StatusCreated, toResponse(note))
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			httpx.WriteProblemFor(w, r, http.StatusBadRequest, "Bad Request", "limit must be between 1 and 100.")
			return
		}
		limit = parsed
	}
	items, err := h.service.List(r.Context(), httpx.AuthenticatedUserID(r.Context()), limit+1, r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.WriteProblemFor(w, r, http.StatusBadRequest, "Bad Request", "Cursor or pagination parameters are invalid.")
		return
	}
	result := listResponse{Items: make([]noteResponse, 0, min(len(items), limit))}
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		result.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(last.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + last.ID))
	}
	for _, item := range items {
		result.Items = append(result.Items, toResponse(&item))
	}
	_ = httpx.WriteJSON(w, http.StatusOK, result)
}

func (h *handler) get(w http.ResponseWriter, r *http.Request) {
	note, err := h.service.Get(r.Context(), httpx.AuthenticatedUserID(r.Context()), r.PathValue("id"))
	if err != nil {
		writeNoteError(w, r, err)
		return
	}
	w.Header().Set("ETag", versionETag(note.Version))
	_ = httpx.WriteJSON(w, http.StatusOK, toResponse(note))
}

func (h *handler) replace(w http.ResponseWriter, r *http.Request) {
	var input noteInput
	if !decodeJSON(w, r, &input) {
		return
	}
	version, ok := requestVersion(r, input.Version)
	if !ok {
		httpx.WriteProblemFor(w, r, http.StatusBadRequest, "Bad Request", "A valid If-Match header or version is required.")
		return
	}
	note, err := h.service.Update(r.Context(), &notes.Note{
		ID: r.PathValue("id"), OwnerID: httpx.AuthenticatedUserID(r.Context()),
		Title: input.Title, Content: input.Content, Version: version,
	})
	if err != nil {
		writeNoteError(w, r, err)
		return
	}
	w.Header().Set("ETag", versionETag(note.Version))
	_ = httpx.WriteJSON(w, http.StatusOK, toResponse(note))
}

func (h *handler) patch(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title   *string `json:"title"`
		Content *string `json:"content"`
		Version int     `json:"version,omitempty"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	version, ok := requestVersion(r, input.Version)
	if !ok {
		httpx.WriteProblemFor(w, r, http.StatusBadRequest, "Bad Request", "A valid If-Match header or version is required.")
		return
	}
	ownerID := httpx.AuthenticatedUserID(r.Context())
	note, err := h.service.Get(r.Context(), ownerID, r.PathValue("id"))
	if err != nil {
		writeNoteError(w, r, err)
		return
	}
	if input.Title != nil {
		note.Title = *input.Title
	}
	if input.Content != nil {
		note.Content = *input.Content
	}
	note.Version = version
	updated, err := h.service.Update(r.Context(), note)
	if err != nil {
		writeNoteError(w, r, err)
		return
	}
	w.Header().Set("ETag", versionETag(updated.Version))
	_ = httpx.WriteJSON(w, http.StatusOK, toResponse(updated))
}

func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	version, ok := requestVersion(r, 0)
	if !ok {
		httpx.WriteProblemFor(w, r, http.StatusBadRequest, "Bad Request", "A valid If-Match header is required.")
		return
	}
	if err := h.service.Delete(r.Context(), httpx.AuthenticatedUserID(r.Context()), r.PathValue("id"), version); err != nil {
		writeNoteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		httpx.WriteProblemFor(w, r, http.StatusUnsupportedMediaType, "Unsupported Media Type", "Content-Type must be application/json.")
		return false
	}
	if err := httpx.DecodeJSON(r, target); err != nil {
		var maxBytes *http.MaxBytesError
		if errors.As(err, &maxBytes) {
			httpx.WriteProblemFor(w, r, http.StatusRequestEntityTooLarge, "Content Too Large", "Request body exceeds the allowed size.")
			return false
		}
		httpx.WriteProblemFor(w, r, http.StatusBadRequest, "Bad Request", "Request body must be valid JSON with no unknown fields.")
		return false
	}
	return true
}

func writeNoteError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, notes.ErrNotFound):
		httpx.WriteProblemFor(w, r, http.StatusNotFound, "Not Found", "Note not found.")
	case errors.Is(err, notes.ErrConflict):
		httpx.WriteProblemFor(w, r, http.StatusConflict, "Conflict", "The note changed. Fetch it and retry your update.")
	default:
		httpx.WriteProblemFor(w, r, http.StatusInternalServerError, "Internal Server Error", "Unable to process note.")
	}
}

func requestVersion(r *http.Request, bodyVersion int) (int, bool) {
	if header := r.Header.Get("If-Match"); header != "" {
		value := strings.Trim(header, `"`)
		value = strings.TrimPrefix(value, "v")
		version, err := strconv.Atoi(value)
		return version, err == nil && version > 0
	}
	return bodyVersion, bodyVersion > 0
}

func versionETag(version int) string { return fmt.Sprintf(`"%d"`, version) }

func toResponse(note *notes.Note) noteResponse {
	return noteResponse{
		ID: note.ID, Title: note.Title, Content: note.Content,
		Version: note.Version, CreatedAt: note.CreatedAt.UTC(), UpdatedAt: note.UpdatedAt.UTC(),
	}
}
