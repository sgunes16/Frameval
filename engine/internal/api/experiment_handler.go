package api

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

func (s *Service) CreateExperiment(w http.ResponseWriter, r *http.Request) {
	var req models.ExperimentRequest
	if err := decodeAndValidate(r, &req); err != nil {
		var verr *ValidationError
		if errors.As(err, &verr) {
			renderValidationError(w, r.Context(), verr.Fields)
			return
		}
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "request body is not valid JSON", err)
		return
	}
	experiment, err := s.store.CreateExperiment(r.Context(), req)
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "failed to create experiment", err)
		return
	}
	JSON(w, http.StatusCreated, experiment)
}

func (s *Service) ListExperiments(w http.ResponseWriter, r *http.Request) {
	experiments, err := s.store.ListExperiments(r.Context())
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, experiments)
}

func (s *Service) GetExperiment(w http.ResponseWriter, r *http.Request) {
	experiment, err := s.store.GetExperiment(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		renderError(w, r.Context(), http.StatusNotFound, ErrCodeNotFound, "not found", err)
		return
	}
	JSON(w, http.StatusOK, experiment)
}

func (s *Service) UpdateExperiment(w http.ResponseWriter, r *http.Request) {
	var req models.ExperimentRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "invalid request", err)
		return
	}
	experiment, err := s.store.UpdateExperiment(r.Context(), chi.URLParam(r, "id"), req)
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, experiment)
}

func (s *Service) DeleteExperiment(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteExperiment(r.Context(), chi.URLParam(r, "id")); err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Service) EstimateExperiment(w http.ResponseWriter, r *http.Request) {
	estimate, err := s.orchestrator.EstimateExperiment(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, map[string]float64{"estimated_cost_usd": estimate})
}

func (s *Service) StartExperiment(w http.ResponseWriter, r *http.Request) {
	if err := s.orchestrator.StartExperiment(r.Context(), chi.URLParam(r, "id")); err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusAccepted, map[string]bool{"started": true})
}

func (s *Service) CancelExperiment(w http.ResponseWriter, r *http.Request) {
	if err := s.orchestrator.CancelExperiment(r.Context(), chi.URLParam(r, "id")); err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, map[string]bool{"cancelled": true})
}

// GetExperimentAnchors serves the cached AnchorBundle for an
// experiment as raw JSON. Empty / never-computed experiments return
// the canonical empty-bundle shape (`{}`) with status 200 — the
// frontend renders that as "no anchors yet" without distinguishing
// missing-default from explicit-empty. A missing experiment row
// returns 404; any other store error returns 500. Data layer for
// Compare V2's Tape tab (story #66).
func (s *Service) GetExperimentAnchors(w http.ResponseWriter, r *http.Request) {
	raw, err := s.store.GetExperimentAnchors(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			renderError(w, r.Context(), http.StatusNotFound, ErrCodeNotFound, "experiment not found", err)
			return
		}
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(raw))
}

func (s *Service) ExportExperiment(w http.ResponseWriter, r *http.Request) {
	experimentID := chi.URLParam(r, "id")
	format := chi.URLParam(r, "format")
	experiment, err := s.store.GetExperiment(r.Context(), experimentID)
	if err != nil {
		renderError(w, r.Context(), http.StatusNotFound, ErrCodeNotFound, "not found", err)
		return
	}
	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		writer := csv.NewWriter(w)
		_ = writer.Write([]string{"experiment_id", "name", "status", "model", "agent_cli"})
		_ = writer.Write([]string{experiment.ID, experiment.Name, experiment.Status, experiment.Model, experiment.AgentCLI})
		writer.Flush()
		return
	}
	JSON(w, http.StatusOK, experiment)
}
