package api

import (
	"encoding/csv"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

func (s *Service) CreateExperiment(w http.ResponseWriter, r *http.Request) {
	var req models.ExperimentRequest
	if err := decodeJSON(r, &req); err != nil {
		JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	experiment, err := s.store.CreateExperiment(r.Context(), req)
	if err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusCreated, experiment)
}

func (s *Service) ListExperiments(w http.ResponseWriter, r *http.Request) {
	experiments, err := s.store.ListExperiments(r.Context())
	if err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, experiments)
}

func (s *Service) GetExperiment(w http.ResponseWriter, r *http.Request) {
	experiment, err := s.store.GetExperiment(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		JSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, experiment)
}

func (s *Service) UpdateExperiment(w http.ResponseWriter, r *http.Request) {
	var req models.ExperimentRequest
	if err := decodeJSON(r, &req); err != nil {
		JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	experiment, err := s.store.UpdateExperiment(r.Context(), chi.URLParam(r, "id"), req)
	if err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, experiment)
}

func (s *Service) DeleteExperiment(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteExperiment(r.Context(), chi.URLParam(r, "id")); err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Service) EstimateExperiment(w http.ResponseWriter, r *http.Request) {
	estimate, err := s.orchestrator.EstimateExperiment(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, map[string]float64{"estimated_cost_usd": estimate})
}

func (s *Service) StartExperiment(w http.ResponseWriter, r *http.Request) {
	if err := s.orchestrator.StartExperiment(r.Context(), chi.URLParam(r, "id")); err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusAccepted, map[string]bool{"started": true})
}

func (s *Service) CancelExperiment(w http.ResponseWriter, r *http.Request) {
	if err := s.orchestrator.CancelExperiment(r.Context(), chi.URLParam(r, "id")); err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, map[string]bool{"cancelled": true})
}

func (s *Service) ExportExperiment(w http.ResponseWriter, r *http.Request) {
	experimentID := chi.URLParam(r, "id")
	format := chi.URLParam(r, "format")
	experiment, err := s.store.GetExperiment(r.Context(), experimentID)
	if err != nil {
		JSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
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
