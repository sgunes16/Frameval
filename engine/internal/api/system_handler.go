package api

import "net/http"

func (s *Service) GetHealth(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, map[string]any{"healthy": true, "version": "0.1.0"})
}

func (s *Service) GetDockerStatus(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, s.orchestrator.SandboxHealth(r.Context()))
}

func (s *Service) GetQueueStatus(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, s.orchestrator.QueueSnapshot())
}
