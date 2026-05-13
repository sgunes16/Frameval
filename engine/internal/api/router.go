package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mustafaselman/frameval/engine/internal/experiment"
	"github.com/mustafaselman/frameval/engine/internal/storage"
)

type Service struct {
	store        *storage.Store
	orchestrator *experiment.Orchestrator
	hub          *Hub
}

func NewService(store *storage.Store, orchestrator *experiment.Orchestrator, hub *Hub) *Service {
	return &Service{store: store, orchestrator: orchestrator, hub: hub}
}

func NewRouter(service *Service) http.Handler {
	r := chi.NewRouter()
	r.Use(logger)
	r.Use(corsMiddleware())
	r.Get("/ws", service.HandleWS)
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", service.GetHealth)
		r.Get("/system/docker", service.GetDockerStatus)
		r.Get("/system/queue", service.GetQueueStatus)
		r.Route("/experiments", func(r chi.Router) {
			r.Post("/", service.CreateExperiment)
			r.Get("/", service.ListExperiments)
			r.Get("/{id}", service.GetExperiment)
			r.Put("/{id}", service.UpdateExperiment)
			r.Delete("/{id}", service.DeleteExperiment)
			r.Post("/{id}/estimate", service.EstimateExperiment)
			r.Post("/{id}/start", service.StartExperiment)
			r.Post("/{id}/cancel", service.CancelExperiment)
			r.Get("/{id}/stats", service.GetExperimentStats)
			r.Get("/{id}/export/{format}", service.ExportExperiment)
			r.Get("/{id}/runs", service.ListExperimentRuns)
			r.Route("/{id}/variants", func(r chi.Router) {
				r.Get("/", service.ListVariants)
				r.Post("/", service.CreateVariant)
			})
		})
		r.Put("/variants/{id}", service.UpdateVariant)
		r.Delete("/variants/{id}", service.DeleteVariant)
		r.Post("/variants/{id}/artifacts", service.CreateArtifact)
		r.Get("/variants/{id}/artifacts", service.ListArtifacts)
		r.Get("/artifacts/{id}", service.GetArtifact)
		r.Get("/artifacts/{id}/versions", service.ListArtifactVersions)
		r.Get("/artifacts/diff", service.DiffArtifacts)
		r.Get("/runs/{id}", service.GetRun)
		r.Get("/runs/{id}/transcript", service.GetTranscript)
		r.Get("/runs/{id}/grade", service.GetRunGrade)
		r.Get("/runs/{id}/diagnostic", service.GetRunDiagnostic)
		r.Post("/runs/{id}/retry", service.RetryRun)
		r.Post("/runs/{id}/regrade", service.RegradeRun)
		r.Get("/tasks", service.ListTasks)
		r.Post("/tasks", service.CreateTask)
		r.Get("/tasks/{id}", service.GetTask)
		r.Get("/config/models", service.ListModels)
		r.Get("/config/agents", service.ListAgents)
		r.Get("/config/api-keys", service.ListAPIKeys)
		r.Post("/config/api-keys", service.UpsertAPIKey)
		r.Delete("/config/api-keys/{provider}", service.DeleteAPIKey)
	})
	return r
}
