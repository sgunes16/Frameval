package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Service) ListExperimentRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := s.store.ListRunsByExperiment(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, runs)
}

func (s *Service) GetRun(w http.ResponseWriter, r *http.Request) {
	run, err := s.store.GetRun(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		JSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, run)
}

func (s *Service) GetTranscript(w http.ResponseWriter, r *http.Request) {
	transcript, err := s.store.GetTranscriptByRun(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		JSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, transcript)
}

func (s *Service) GetRunGrade(w http.ResponseWriter, r *http.Request) {
	grade, err := s.store.GetGradeByRun(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		JSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, grade)
}

func (s *Service) RetryRun(w http.ResponseWriter, r *http.Request) {
	if err := s.orchestrator.RetryRun(r.Context(), chi.URLParam(r, "id")); err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusAccepted, map[string]bool{"queued": true})
}

func (s *Service) RegradeRun(w http.ResponseWriter, r *http.Request) {
	if err := s.orchestrator.RegradeRun(r.Context(), chi.URLParam(r, "id")); err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusAccepted, map[string]bool{"regraded": true})
}
