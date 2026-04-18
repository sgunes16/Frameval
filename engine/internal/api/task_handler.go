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
	JSON(w, http.StatusCreated, created)
}

func (s *Service) GetTask(w http.ResponseWriter, r *http.Request) {
	task, err := s.store.GetTask(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		JSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, task)
}
