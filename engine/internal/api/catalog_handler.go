package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mustafaselman/frameval/engine/internal/catalog"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

func (s *Service) ListCatalogExtensions(w http.ResponseWriter, r *http.Request) {
	catalogResponse, err := catalog.LoadSpecKitCatalog(r.Context())
	if err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, catalogResponse)
}

func (s *Service) ImportCatalogExtensions(w http.ResponseWriter, r *http.Request) {
	var req models.CatalogExtensionImportRequest
	if err := decodeJSON(r, &req); err != nil {
		JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if len(req.ExtensionIDs) == 0 {
		JSON(w, http.StatusBadRequest, map[string]string{"error": "extension_ids is required"})
		return
	}
	extensions, err := catalog.SelectExtensions(r.Context(), req.ExtensionIDs)
	if err != nil {
		JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	created := make([]models.ArtifactVersion, 0)
	variantID := chi.URLParam(r, "id")
	for _, extension := range extensions {
		files, err := catalog.DownloadExtensionFiles(r.Context(), extension)
		if err != nil {
			JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		for _, file := range files {
			artifact, err := s.store.CreateArtifactVersion(r.Context(), variantID, file, classifyDimensions(file.Content, file.ArtifactType))
			if err != nil {
				JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			created = append(created, *artifact)
		}
	}
	JSON(w, http.StatusCreated, created)
}
