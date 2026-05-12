package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

func (s *Service) ListTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.store.ListTasks(r.Context())
	if err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	for idx := range tasks {
		tasks[idx] = sanitizeTaskForAPI(tasks[idx])
	}
	JSON(w, http.StatusOK, tasks)
}

func (s *Service) CreateTask(w http.ResponseWriter, r *http.Request) {
	var task models.Task
	if err := decodeJSON(r, &task); err != nil {
		JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	created, err := s.store.UpsertTask(r.Context(), task)
	if err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	sanitized := sanitizeTaskForAPI(*created)
	JSON(w, http.StatusCreated, sanitized)
}

func (s *Service) GetTask(w http.ResponseWriter, r *http.Request) {
	task, err := s.store.GetTask(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		JSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	sanitized := sanitizeTaskForAPI(*task)
	JSON(w, http.StatusOK, sanitized)
}

func sanitizeTaskForAPI(task models.Task) models.Task {
	sanitized := task
	if len(task.Metadata) > 0 {
		sanitized.Metadata = map[string]any{}
		for key, value := range task.Metadata {
			if key == "hidden_files" || key == "workspace_files" {
				continue
			}
			sanitized.Metadata[key] = value
		}
	}
	sanitized.TestCases = make([]models.TestCase, 0, len(task.TestCases))
	for _, testCase := range task.TestCases {
		if testCase.Visibility == "hidden" {
			testCase.TestCommand = ""
			testCase.ExpectedResult = ""
			testCase.SetupScript = ""
		}
		sanitized.TestCases = append(sanitized.TestCases, testCase)
	}
	return sanitized
}
