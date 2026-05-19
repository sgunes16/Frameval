package api

import (
	"expvar"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	builtinharness "github.com/mustafaselman/frameval/engine/internal/builtin/harness"
	"github.com/mustafaselman/frameval/engine/internal/executor"
	"github.com/mustafaselman/frameval/engine/internal/experiment"
	"github.com/mustafaselman/frameval/engine/internal/storage"
)

type Service struct {
	store        *storage.Store
	orchestrator *experiment.Orchestrator
	harnesses    *builtinharness.Registry
	executors    *executor.Registry
	hub          *Hub
}

func NewService(store *storage.Store, orchestrator *experiment.Orchestrator, harnesses *builtinharness.Registry, executors *executor.Registry, hub *Hub) *Service {
	return &Service{store: store, orchestrator: orchestrator, harnesses: harnesses, executors: executors, hub: hub}
}

// defaultBodyCap is the largest request body the engine accepts. 1 MiB is
// far more than any current endpoint needs (the largest payload is the
// experiment-create form, well under 100 KiB) and gives a generous margin
// for future growth. Crank it via the router-construction path if a real
// use case justifies more.
const defaultBodyCap = 1 << 20 // 1 MiB

func NewRouter(service *Service, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()
	// trace_id middleware must run first so subsequent middleware
	// (including requestLogger) sees it on the context.
	r.Use(WithTraceID)
	r.Use(requestLogger(logger))
	r.Use(WithBodyCap(defaultBodyCap))
	r.Use(corsMiddleware())
	r.Get("/ws", service.HandleWS)
	// /debug/vars exposes every expvar registered across the engine —
	// frameval_ws_dropped_total, frameval_grader_breaker_state, etc.
	// Mount the standard handler explicitly because chi does not share
	// http.DefaultServeMux. The output is JSON in the expvar format.
	r.Get("/debug/vars", func(w http.ResponseWriter, r *http.Request) {
		expvar.Handler().ServeHTTP(w, r)
	})
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
			r.Get("/{id}/export/{format}", service.ExportExperiment)
			r.Get("/{id}/runs", service.ListExperimentRuns)
			r.Get("/{id}/turns", service.GetExperimentTurns)
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
		r.Get("/runs/{id}/turns", service.GetRunTurns)
		r.Get("/runs/{id}/grade", service.GetRunGrade)
		r.Get("/runs/{id}/diagnostic", service.GetRunDiagnostic)
		r.Post("/runs/{id}/retry", service.RetryRun)
		r.Post("/runs/{id}/regrade", service.RegradeRun)
		r.Get("/tasks", service.ListTasks)
		r.Post("/tasks", service.CreateTask)
		r.Get("/tasks/{id}", service.GetTask)
		r.Get("/config/models", service.ListModels)
		r.Get("/config/agents", service.ListAgents)
		r.Get("/config/harnesses", service.ListHarnesses)
		r.Get("/config/executors", service.ListExecutors)
		r.Post("/diagnostic/launch", service.LaunchDiagnostic)
		r.Get("/config/api-keys", service.ListAPIKeys)
		r.Post("/config/api-keys", service.UpsertAPIKey)
		r.Delete("/config/api-keys/{provider}", service.DeleteAPIKey)
	})
	return r
}
