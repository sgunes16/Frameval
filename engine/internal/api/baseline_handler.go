package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Service) ListBaselines(w http.ResponseWriter, r *http.Request) {
	baselines, err := s.store.ListBaselines(r.Context())
	if err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, baselines)
}

func (s *Service) GetBaseline(w http.ResponseWriter, r *http.Request) {
	baseline, err := s.store.GetBaseline(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		JSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, baseline)
}
