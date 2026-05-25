package api

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/mustafaselman/frameval/engine/internal/storage"
)

var rubricKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,40}$`)

type rubricPayload struct {
	Key         string `json:"key"`
	DisplayName string `json:"display_name"`
	Prompt      string `json:"prompt"`
	SortOrder   int    `json:"sort_order,omitempty"`
	IsBuiltin   bool   `json:"is_builtin,omitempty"`
}

func (s *Service) ListRubrics(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.ListRubrics(r.Context())
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, rows)
}

func (s *Service) GetRubric(w http.ResponseWriter, r *http.Request) {
	got, err := s.store.GetRubric(r.Context(), chi.URLParam(r, "key"))
	if errors.Is(err, sql.ErrNoRows) {
		renderError(w, r.Context(), http.StatusNotFound, ErrCodeNotFound, "rubric not found", err)
		return
	}
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, got)
}

func (s *Service) CreateRubric(w http.ResponseWriter, r *http.Request) {
	var p rubricPayload
	if err := decodeJSON(r, &p); err != nil {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "invalid request", err)
		return
	}
	if !rubricKeyPattern.MatchString(p.Key) {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "invalid key (lowercase snake_case, 2-41 chars)", nil)
		return
	}
	if len(p.DisplayName) < 1 || len(p.DisplayName) > 80 {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "display_name must be 1-80 chars", nil)
		return
	}
	if len(p.Prompt) < 50 || len(p.Prompt) > 20000 {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "prompt must be 50-20000 chars", nil)
		return
	}
	if _, err := s.store.GetRubric(r.Context(), p.Key); err == nil {
		renderError(w, r.Context(), http.StatusConflict, ErrCodeConflict, "rubric with this key already exists", nil)
		return
	}
	row := storage.Rubric{
		Key: p.Key, DisplayName: p.DisplayName, Prompt: p.Prompt,
		SortOrder: p.SortOrder, IsBuiltin: false, // user-created rubrics never builtin
	}
	if err := s.store.UpsertRubric(r.Context(), row); err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusCreated, row)
}

func (s *Service) UpdateRubric(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	existing, err := s.store.GetRubric(r.Context(), key)
	if errors.Is(err, sql.ErrNoRows) {
		renderError(w, r.Context(), http.StatusNotFound, ErrCodeNotFound, "rubric not found", err)
		return
	}
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	var p rubricPayload
	if err := decodeJSON(r, &p); err != nil {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "invalid request", err)
		return
	}
	if len(p.DisplayName) >= 1 && len(p.DisplayName) <= 80 {
		existing.DisplayName = p.DisplayName
	}
	if len(p.Prompt) >= 50 && len(p.Prompt) <= 20000 {
		existing.Prompt = p.Prompt
	}
	if p.SortOrder > 0 {
		existing.SortOrder = p.SortOrder
	}
	if err := s.store.UpsertRubric(r.Context(), existing); err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, existing)
}

func (s *Service) DeleteRubricHandler(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	existing, err := s.store.GetRubric(r.Context(), key)
	if errors.Is(err, sql.ErrNoRows) {
		w.WriteHeader(http.StatusNoContent) // idempotent delete
		return
	}
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	if existing.IsBuiltin {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "builtin rubric cannot be deleted", nil)
		return
	}
	if err := s.store.DeleteRubric(r.Context(), key); err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
