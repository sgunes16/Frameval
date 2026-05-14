package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

func (s *Service) ListVariants(w http.ResponseWriter, r *http.Request) {
	variants, err := s.store.ListVariantsByExperiment(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, variants)
}

func (s *Service) CreateVariant(w http.ResponseWriter, r *http.Request) {
	var variant models.Variant
	if err := decodeJSON(r, &variant); err != nil {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "invalid request", err)
		return
	}
	variant.ExperimentID = chi.URLParam(r, "id")
	created, err := s.store.CreateVariant(r.Context(), variant)
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusCreated, created)
}

func (s *Service) UpdateVariant(w http.ResponseWriter, r *http.Request) {
	var variant models.Variant
	if err := decodeJSON(r, &variant); err != nil {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "invalid request", err)
		return
	}
	updated, err := s.store.UpdateVariant(r.Context(), chi.URLParam(r, "id"), variant)
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, updated)
}

func (s *Service) DeleteVariant(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteVariant(r.Context(), chi.URLParam(r, "id")); err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
