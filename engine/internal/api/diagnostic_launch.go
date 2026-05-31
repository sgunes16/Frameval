package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mustafaselman/frameval/engine/internal/models"
)

// LaunchDiagnosticRequest is the shorthand body the frontend launcher posts.
// It maps 1:1 to creating an experiment with one variant per harness, then
// starting the experiment in the same call.
//
// Defaults match the AgentDx demo profile: 5 runs per variant (matches the
// minimum enforced in storage), 600s timeout, 1 max concurrent.
type LaunchDiagnosticRequest struct {
	TaskID         string   `json:"task_id"`
	ExecutorID     string   `json:"executor_id"`
	HarnessIDs     []string `json:"harness_ids"`
	Model          string   `json:"model"`
	RunsPerVariant int      `json:"runs_per_variant"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	Name           string   `json:"name"`
	BatchID        string   `json:"batch_id"`
	BatchLabel     string   `json:"batch_label"`
}

// LaunchDiagnosticResponse returns the IDs the frontend needs to navigate
// straight to the Compare view.
type LaunchDiagnosticResponse struct {
	ExperimentID string `json:"experiment_id"`
}

// LaunchDiagnostic creates an experiment with one variant per requested
// harness, then enqueues runs immediately. Returns 202 with the experiment ID
// so the caller can redirect to `/diagnostic/compare?experiment=<id>`.
func (s *Service) LaunchDiagnostic(w http.ResponseWriter, r *http.Request) {
	var req LaunchDiagnosticRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "invalid request", err)
		return
	}
	if strings.TrimSpace(req.TaskID) == "" {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "task_id is required", nil)
		return
	}
	if strings.TrimSpace(req.ExecutorID) == "" {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "executor_id is required", nil)
		return
	}
	if len(req.HarnessIDs) == 0 {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "harness_ids must contain at least one harness", nil)
		return
	}
	for _, hid := range req.HarnessIDs {
		if _, err := s.harnesses.Get(hid); err != nil {
			renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, fmt.Sprintf("unknown harness %q", hid), err)
			return
		}
	}
	if _, err := s.executors.Get(req.ExecutorID); err != nil {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, fmt.Sprintf("unknown executor %q", req.ExecutorID), err)
		return
	}

	task, err := s.store.GetTask(r.Context(), req.TaskID)
	if err != nil {
		renderError(w, r.Context(), http.StatusNotFound, ErrCodeNotFound, fmt.Sprintf("task %q not found", req.TaskID), err)
		return
	}

	runsPerVariant := req.RunsPerVariant
	if runsPerVariant <= 0 {
		runsPerVariant = 5
	}
	timeout := req.TimeoutSeconds
	if timeout <= 0 {
		timeout = 600
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = fmt.Sprintf("Diagnostic %s · %s", task.Name, time.Now().UTC().Format("2006-01-02 15:04"))
	}

	variants := make([]models.VariantRequest, 0, len(req.HarnessIDs))
	for idx, hid := range req.HarnessIDs {
		variants = append(variants, models.VariantRequest{
			Name:        hid,
			Description: fmt.Sprintf("Harness: %s", hid),
			IsControl:   idx == 0,
			Ordering:    idx,
			HarnessID:   hid,
		})
	}

	experiment, err := s.store.CreateExperiment(r.Context(), models.ExperimentRequest{
		Name:           name,
		Description:    fmt.Sprintf("Diagnostic launcher: %d harness(es), executor=%s", len(req.HarnessIDs), req.ExecutorID),
		TaskID:         req.TaskID,
		Model:          req.Model,
		AgentCLI:       req.ExecutorID,
		ExecutionMode:  "cli",
		RunsPerVariant: runsPerVariant,
		TimeoutSeconds: timeout,
		MaxConcurrent:  1,
		Variants:       variants,
		BatchID:        req.BatchID,
		BatchLabel:     req.BatchLabel,
	})
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}

	if err := s.orchestrator.StartExperiment(r.Context(), experiment.ID); err != nil {
		// The experiment + variants + (possibly) some run rows already
		// exist. Mark it failed so the ghost row doesn't sit at status="" or
		// "draft" forever — the user can find it in /experiments and delete it.
		_ = s.store.UpdateExperimentStatus(r.Context(), experiment.ID, "failed")
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}

	JSON(w, http.StatusAccepted, LaunchDiagnosticResponse{ExperimentID: experiment.ID})
}
