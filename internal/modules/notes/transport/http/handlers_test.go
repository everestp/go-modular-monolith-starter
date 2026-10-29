package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"prod-go/internal/modules/notes"
)

type noteServiceFake struct {
	ownerID string
	item    *notes.Note
}

func (s *noteServiceFake) Create(_ context.Context, ownerID, title, content string) (*notes.Note, error) {
	if ownerID != s.ownerID {
		return nil, notes.ErrNotFound
	}
	s.item = &notes.Note{ID: "note-1", OwnerID: ownerID, Title: title, Content: content, Version: 1}
	return s.item, nil
}

func (s *noteServiceFake) Get(_ context.Context, ownerID, id string) (*notes.Note, error) {
	if s.item == nil || ownerID != s.ownerID || id != s.item.ID {
		return nil, notes.ErrNotFound
	}
	return s.item, nil
}

func (s *noteServiceFake) List(context.Context, string, int, string) ([]notes.Note, error) {
	if s.item == nil {
		return nil, nil
	}
	return []notes.Note{*s.item}, nil
}

func (s *noteServiceFake) Update(_ context.Context, note *notes.Note) (*notes.Note, error) {
	if s.item == nil || note.OwnerID != s.ownerID || note.ID != s.item.ID {
		return nil, notes.ErrNotFound
	}
	if note.Version != s.item.Version {
		return nil, notes.ErrConflict
	}
	note.Version++
	s.item = note
	return note, nil
}

func (s *noteServiceFake) Delete(_ context.Context, ownerID, id string, version int) error {
	if s.item == nil || ownerID != s.ownerID || id != s.item.ID {
		return notes.ErrNotFound
	}
	if version != s.item.Version {
		return notes.ErrConflict
	}
	s.item = nil
	return nil
}

func TestCreateNoteReturnsLocationAndETag(t *testing.T) {
	service := &noteServiceFake{ownerID: "user-1"}
	mux := http.NewServeMux()
	Register(mux, service, func(token string) (string, error) {
		if token != "token" {
			return "", errors.New("invalid token")
		}
		return "user-1", nil
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notes", strings.NewReader(`{"title":"first","content":"body"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, req)
	if response.Code != http.StatusCreated || response.Header().Get("Location") != "/api/v1/notes/note-1" || response.Header().Get("ETag") != `"1"` {
		t.Fatalf("create response = %d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
}

func TestNotesRequireBearerToken(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, &noteServiceFake{ownerID: "user-1"}, func(string) (string, error) { return "", errors.New("invalid token") })
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/notes", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestAnotherUsersNoteIsNotFound(t *testing.T) {
	service := &noteServiceFake{ownerID: "user-1", item: &notes.Note{ID: "note-1", OwnerID: "user-1", Version: 1}}
	mux := http.NewServeMux()
	Register(mux, service, func(string) (string, error) { return "user-2", nil })
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notes/note-1", nil)
	req.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, req)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
