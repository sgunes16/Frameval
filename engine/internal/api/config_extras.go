package api

import (
	"net/http"
	"sort"
)

// HarnessInfo is the shape the frontend's launcher picker consumes.
type HarnessInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ExecutorInfo describes a runnable agent executor.
type ExecutorInfo struct {
	ID    string   `json:"id"`
	Modes []string `json:"modes"`
}

// ListHarnesses returns every harness adapter registered at startup. Stable
// sort by ID so the picker UI is deterministic.
func (s *Service) ListHarnesses(w http.ResponseWriter, _ *http.Request) {
	entries := s.harnesses.Entries()
	out := make([]HarnessInfo, 0, len(entries))
	for _, h := range entries {
		out = append(out, HarnessInfo{ID: h.Name(), Name: h.Name(), Description: h.Description()})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	JSON(w, http.StatusOK, out)
}

// ListExecutors returns every executor registered at startup.
func (s *Service) ListExecutors(w http.ResponseWriter, _ *http.Request) {
	entries := s.executors.Entries()
	out := make([]ExecutorInfo, 0, len(entries))
	for _, e := range entries {
		modes := make([]string, 0, len(e.SupportedModes()))
		for _, m := range e.SupportedModes() {
			modes = append(modes, string(m))
		}
		out = append(out, ExecutorInfo{ID: e.Name(), Modes: modes})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	JSON(w, http.StatusOK, out)
}
