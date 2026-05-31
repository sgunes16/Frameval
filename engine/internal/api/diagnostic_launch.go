package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

// LaunchDiagnosticRequest is the shorthand body the frontend launcher posts.
// It maps 1:1 to creating an experiment with one variant per harness, then
// starting the experiment in the same call.
//
// Defaults match the AgentDx demo profile: 5 runs per variant (matches the
// minimum enforced in storage), 600s timeout, 1 max concurrent.
type LaunchDiagnosticRequest struct {
	TaskID          string         `json:"task_id"`
	ExecutorID      string         `json:"executor_id"`
	HarnessIDs      []string       `json:"harness_ids"`
	Model           string         `json:"model"`
	RunsPerVariant  int            `json:"runs_per_variant"`
	TimeoutSeconds  int            `json:"timeout_seconds"`
	Name            string         `json:"name"`
	BatchID         string         `json:"batch_id"`
	BatchLabel      string         `json:"batch_label"`
	HarnessConfigs  map[string]any `json:"harness_configs,omitempty"`
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
			Name:          hid,
			Description:   fmt.Sprintf("Harness: %s", hid),
			IsControl:     idx == 0,
			Ordering:      idx,
			HarnessID:     hid,
			HarnessConfig: req.HarnessConfigs,
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

// LaunchDiagnosticSuiteRequest launches N experiments in one POST,
// all sharing a server-minted batch_id. Display-only batch_label
// helps the Experiments list show a readable group title.
type LaunchDiagnosticSuiteRequest struct {
	TaskIDs         []string       `json:"task_ids"`
	ExecutorID      string         `json:"executor_id"`
	HarnessIDs      []string       `json:"harness_ids"`
	Model           string         `json:"model"`
	RunsPerVariant  int            `json:"runs_per_variant"`
	TimeoutSeconds  int            `json:"timeout_seconds"`
	BatchLabel      string         `json:"batch_label"`
	HarnessConfigs  map[string]any `json:"harness_configs,omitempty"`
}

// LaunchDiagnosticSuiteResponse returns the new batch identity and
// per-task outcomes. Partial failure is reported in Failures and
// does not abort the rest of the batch.
type LaunchDiagnosticSuiteResponse struct {
	BatchID       string               `json:"batch_id"`
	ExperimentIDs []string             `json:"experiment_ids"`
	Failures      []SuiteLaunchFailure `json:"failures,omitempty"`
}

// SuiteLaunchFailure records a per-task failure in a suite launch.
type SuiteLaunchFailure struct {
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
}

// LaunchDiagnosticSuite is the multi-task launch endpoint. It iterates
// over TaskIDs and creates one experiment per task with a shared
// batch_id. One bad task ID doesn't fail the rest — failures land in
// the response's Failures array.
func (s *Service) LaunchDiagnosticSuite(w http.ResponseWriter, r *http.Request) {
	var req LaunchDiagnosticSuiteRequest
	if err := decodeJSON(r, &req); err != nil {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "invalid request", err)
		return
	}
	if len(req.TaskIDs) == 0 {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "task_ids must contain at least one task", nil)
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

	runsPerVariant := req.RunsPerVariant
	if runsPerVariant <= 0 {
		runsPerVariant = 5
	}
	timeout := req.TimeoutSeconds
	if timeout <= 0 {
		timeout = 600
	}
	batchID := uuid.NewString()
	label := strings.TrimSpace(req.BatchLabel)
	if label == "" {
		label = fmt.Sprintf("Diagnostic suite · %s", time.Now().UTC().Format("2006-01-02 15:04"))
	}

	resp := LaunchDiagnosticSuiteResponse{BatchID: batchID}

	for _, tid := range req.TaskIDs {
		task, err := s.store.GetTask(r.Context(), tid)
		if err != nil {
			resp.Failures = append(resp.Failures, SuiteLaunchFailure{TaskID: tid, Message: fmt.Sprintf("task not found: %v", err)})
			continue
		}

		variants := make([]models.VariantRequest, 0, len(req.HarnessIDs))
		for idx, hid := range req.HarnessIDs {
			variants = append(variants, models.VariantRequest{
				Name:          hid,
				Description:   fmt.Sprintf("Harness: %s", hid),
				IsControl:     idx == 0,
				Ordering:      idx,
				HarnessID:     hid,
				HarnessConfig: req.HarnessConfigs,
			})
		}

		experiment, err := s.store.CreateExperiment(r.Context(), models.ExperimentRequest{
			Name:           fmt.Sprintf("%s · %s", label, task.Name),
			Description:    fmt.Sprintf("Diagnostic suite (%d task(s)), executor=%s", len(req.TaskIDs), req.ExecutorID),
			TaskID:         tid,
			Model:          req.Model,
			AgentCLI:       req.ExecutorID,
			ExecutionMode:  "cli",
			RunsPerVariant: runsPerVariant,
			TimeoutSeconds: timeout,
			MaxConcurrent:  1,
			Variants:       variants,
			BatchID:        batchID,
			BatchLabel:     label,
		})
		if err != nil {
			resp.Failures = append(resp.Failures, SuiteLaunchFailure{TaskID: tid, Message: fmt.Sprintf("create experiment: %v", err)})
			continue
		}
		if err := s.orchestrator.StartExperiment(r.Context(), experiment.ID); err != nil {
			_ = s.store.UpdateExperimentStatus(r.Context(), experiment.ID, "failed")
			resp.Failures = append(resp.Failures, SuiteLaunchFailure{TaskID: tid, Message: fmt.Sprintf("start experiment: %v", err)})
			continue
		}
		resp.ExperimentIDs = append(resp.ExperimentIDs, experiment.ID)
	}

	JSON(w, http.StatusAccepted, resp)
}
