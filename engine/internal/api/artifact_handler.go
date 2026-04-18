package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

func classifyDimensions(content string, artifactType string) map[string]any {
	return map[string]any{
		"framing":            "instructional",
		"specificity":        "medium",
		"structure":          "hierarchical",
		"scope":              "task-focused",
		"tone":               "neutral",
		"constraint_density": 0.5,
		"example_presence":   "low",
		"priority_signaling": "medium",
		"tool_guidance":      artifactType,
		"error_handling":     "implicit",
	}
}

func (s *Service) CreateArtifact(w http.ResponseWriter, r *http.Request) {
	var req models.ArtifactRequest
	if err := decodeJSON(r, &req); err != nil {
		JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	artifact, err := s.store.CreateArtifactVersion(r.Context(), chi.URLParam(r, "id"), req, classifyDimensions(req.Content, req.ArtifactType))
	if err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusCreated, artifact)
}

func (s *Service) ListArtifacts(w http.ResponseWriter, r *http.Request) {
	artifacts, err := s.store.ListArtifactVersionsByVariant(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, artifacts)
}

func (s *Service) GetArtifact(w http.ResponseWriter, r *http.Request) {
	artifact, err := s.store.GetArtifactVersion(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		JSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, artifact)
}

func (s *Service) ListArtifactVersions(w http.ResponseWriter, r *http.Request) {
	artifact, err := s.store.GetArtifactVersion(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		JSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	versions, err := s.store.ListArtifactVersionsByVariant(r.Context(), artifact.VariantID)
	if err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, versions)
}

func (s *Service) DiffArtifacts(w http.ResponseWriter, r *http.Request) {
	fromID := r.URL.Query().Get("from")
	toID := r.URL.Query().Get("to")
	diff, err := s.store.DiffArtifactVersions(r.Context(), fromID, toID)
	if err != nil {
		JSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, diff)
}
