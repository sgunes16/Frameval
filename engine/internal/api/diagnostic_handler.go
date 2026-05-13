package api

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// GetRunDiagnostic returns the AgentDx Diagnostic Profile for a single run.
//
// Response shape mirrors `storage.DiagnosticRecord` serialized to JSON. When
// the run has no diagnostic row yet (orchestrator wiring is the deferred
// follow-up), this returns 404 so the frontend can render an empty state.
func (s *Service) GetRunDiagnostic(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "id")
	rec, err := s.store.GetDiagnosticByRun(r.Context(), runID)
	if errors.Is(err, sql.ErrNoRows) {
		JSON(w, http.StatusNotFound, map[string]string{"error": "diagnostic not yet available for run"})
		return
	}
	if err != nil {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	JSON(w, http.StatusOK, rec)
}
