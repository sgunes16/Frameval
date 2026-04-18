package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Service) ListModels(w http.ResponseWriter, r *http.Request) {
	models, err := s.store.ListModelConfigs(r.Context())
	if err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, models)
}

func (s *Service) ListAgents(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, s.store.ListAgents())
}

func (s *Service) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.store.ListAPIKeys(r.Context())
	if err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, keys)
}

func (s *Service) UpsertAPIKey(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Provider string `json:"provider"`
		APIKey   string `json:"api_key"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.UpsertAPIKey(r.Context(), payload.Provider, payload.APIKey); err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Service) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteAPIKey(r.Context(), chi.URLParam(r, "provider")); err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
