package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/mustafaselman/frameval/engine/internal/experiment"
)

func (s *Service) ListExperimentRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := s.store.ListRunsByExperiment(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, runs)
}

func (s *Service) GetRun(w http.ResponseWriter, r *http.Request) {
	run, err := s.store.GetRun(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		renderError(w, r.Context(), http.StatusNotFound, ErrCodeNotFound, "not found", err)
		return
	}
	JSON(w, http.StatusOK, run)
}

func (s *Service) GetTranscript(w http.ResponseWriter, r *http.Request) {
	transcript, err := s.store.GetTranscriptByRun(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		renderError(w, r.Context(), http.StatusNotFound, ErrCodeNotFound, "not found", err)
		return
	}
	JSON(w, http.StatusOK, transcript)
}

// GetRunTurns returns the structured ParsedTurns for a single run.
// Used by Inspector V2's data hook — cheaper than fetching the full
// transcript when only the turn list is needed.
//
// Returns 200 with an empty array (not 404) when the run has no
// transcript yet, so the Inspector can poll this during a live run.
func (s *Service) GetRunTurns(w http.ResponseWriter, r *http.Request) {
	turns, err := s.store.ListTurns(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, turns)
}

// GetExperimentTurns returns turns grouped by run_id for every run in
// an experiment. Used by Compare V2 to fan-out N runs in a single
// round-trip instead of N parallel GET /runs/:id/turns calls.
func (s *Service) GetExperimentTurns(w http.ResponseWriter, r *http.Request) {
	turns, err := s.store.ListTurnsByExperiment(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, turns)
}

func (s *Service) GetRunGrade(w http.ResponseWriter, r *http.Request) {
	grade, err := s.store.GetGradeByRun(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		renderError(w, r.Context(), http.StatusNotFound, ErrCodeNotFound, "not found", err)
		return
	}
	JSON(w, http.StatusOK, grade)
}

func (s *Service) RetryRun(w http.ResponseWriter, r *http.Request) {
	if err := s.orchestrator.RetryRun(r.Context(), chi.URLParam(r, "id")); err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusAccepted, map[string]bool{"queued": true})
}

func (s *Service) RegradeRun(w http.ResponseWriter, r *http.Request) {
	err := s.orchestrator.RegradeRun(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, experiment.ErrGraderUnavailable) {
		renderError(w, r.Context(), http.StatusServiceUnavailable, ErrCodeGraderDown, "grader is unavailable; original grade preserved", err)
		return
	}
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusAccepted, map[string]bool{"regraded": true})
}
