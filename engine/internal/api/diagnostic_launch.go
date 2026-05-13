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
		JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.TaskID) == "" {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "task_id is required"})
		return
	}
	if strings.TrimSpace(req.ExecutorID) == "" {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "executor_id is required"})
		return
	}
	if len(req.HarnessIDs) == 0 {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "harness_ids must contain at least one harness"})
		return
	}
	for _, hid := range req.HarnessIDs {
		if _, err := s.harnesses.Get(hid); err != nil {
			JSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("unknown harness %q", hid)})
			return
		}
	}
	if _, err := s.executors.Get(req.ExecutorID); err != nil {
		JSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("unknown executor %q", req.ExecutorID)})
		return
	}

	task, err := s.store.GetTask(r.Context(), req.TaskID)
	if err != nil {
		JSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("task %q not found", req.TaskID)})
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
	})
	if err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := s.orchestrator.StartExperiment(r.Context(), experiment.ID); err != nil {
		// The experiment + variants + (possibly) some run rows already
		// exist. Mark it failed so the ghost row doesn't sit at status="" or
		// "draft" forever — the user can find it in /experiments and delete it.
		_ = s.store.UpdateExperimentStatus(r.Context(), experiment.ID, "failed")
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	JSON(w, http.StatusAccepted, LaunchDiagnosticResponse{ExperimentID: experiment.ID})
}
