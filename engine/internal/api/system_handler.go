package api

import (
	"context"
	"net/http"
)

// engineVersion is the build-time version stamped on health responses.
// Kept as a package-level var (not const) so a future ldflags injection
// can override it without touching every callsite.
var engineVersion = "0.1.0"

// componentStatus is the per-subcomponent shape inside the health
// response. Status values are stable: "ok" / "degraded" / "unavailable".
// Extra fields per component live in Details so the wire format is
// uniform and operators can rely on a known top-level key.
type componentStatus struct {
	Status  string         `json:"status"`
	Details map[string]any `json:"details,omitempty"`
}

// healthResponse is the wire shape of GET /api/health.
type healthResponse struct {
	OK         bool                       `json:"ok"`
	Version    string                     `json:"version"`
	Components map[string]componentStatus `json:"components"`
}

// GetHealth returns a sub-component breakdown of the engine's runtime
// dependencies. Status check semantics:
//
//   db      — the storage layer is reachable (we ran migrations at startup,
//             and we re-ping on each call).
//   docker  — the sandbox manager reports its Docker daemon state.
//   grader  — the gRPC client's circuit breaker state (closed/half-open
//             counts as ok; open counts as unavailable).
//   queue   — the orchestrator's job queue snapshot.
//
// A missing dependency (svc constructed without a store/orchestrator,
// e.g., in a unit test) shows up as "degraded" rather than crashing the
// handler, so /api/health stays scrapeable even in partial wiring.
func (s *Service) GetHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	components := map[string]componentStatus{
		"db":     s.dbHealth(ctx),
		"docker": s.dockerHealth(ctx),
		"grader": s.graderHealth(),
		"queue":  s.queueHealth(),
	}

	// ok is the top-level readiness signal — any non-ok component flips
	// it to false. Returning ok=true with a degraded component would let
	// readiness probes / monitoring tools treat a half-broken engine as
	// healthy.
	ok := true
	for _, c := range components {
		if c.Status != "ok" {
			ok = false
			break
		}
	}

	JSON(w, http.StatusOK, healthResponse{
		OK:         ok,
		Version:    engineVersion,
		Components: components,
	})
}

func (s *Service) dbHealth(ctx context.Context) componentStatus {
	if s == nil || s.store == nil || s.store.DB == nil {
		return componentStatus{Status: "degraded"}
	}
	if err := s.store.DB.PingContext(ctx); err != nil {
		return componentStatus{Status: "unavailable", Details: map[string]any{"err": err.Error()}}
	}
	return componentStatus{Status: "ok"}
}

func (s *Service) dockerHealth(ctx context.Context) componentStatus {
	if s == nil || s.orchestrator == nil {
		return componentStatus{Status: "degraded"}
	}
	state := s.orchestrator.SandboxHealth(ctx)
	if state == nil {
		return componentStatus{Status: "degraded"}
	}
	// SandboxHealth returns a map[string]any; coerce its known fields.
	status := "ok"
	if healthy, ok := state["healthy"].(bool); ok && !healthy {
		status = "unavailable"
	}
	return componentStatus{Status: status, Details: state}
}

func (s *Service) graderHealth() componentStatus {
	// Read the circuit breaker state via the expvar exported by the
	// experiment package. Keeps health and metrics in sync — they share
	// the single source of truth.
	state := graderBreakerStateValue()
	switch state {
	case "open":
		return componentStatus{Status: "unavailable", Details: map[string]any{"breaker": state}}
	case "half-open":
		return componentStatus{Status: "degraded", Details: map[string]any{"breaker": state}}
	default: // "closed" or unset
		return componentStatus{Status: "ok", Details: map[string]any{"breaker": state}}
	}
}

func (s *Service) queueHealth() componentStatus {
	if s == nil || s.orchestrator == nil {
		return componentStatus{Status: "degraded"}
	}
	snap := s.orchestrator.QueueSnapshot()
	return componentStatus{
		Status: "ok",
		Details: map[string]any{
			"depth":   snap.Depth,
			"active":  snap.ActiveWorkers,
			"max":     snap.MaxWorkers,
		},
	}
}

// GetDockerStatus is the legacy endpoint kept for the frontend's
// system-status strip; new code should prefer /api/health. nil-guarded
// the same way dockerHealth() is so a partial-wiring test or a future
// lazy-orchestrator design cannot panic the route.
func (s *Service) GetDockerStatus(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.orchestrator == nil {
		JSON(w, http.StatusOK, map[string]any{"healthy": false, "mode": "unavailable"})
		return
	}
	JSON(w, http.StatusOK, s.orchestrator.SandboxHealth(r.Context()))
}

// GetQueueStatus is the legacy endpoint kept for the frontend's
// system-status strip; new code should prefer /api/health.
func (s *Service) GetQueueStatus(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.orchestrator == nil {
		JSON(w, http.StatusOK, map[string]any{"depth": 0, "active_workers": 0, "max_workers": 0})
		return
	}
	JSON(w, http.StatusOK, s.orchestrator.QueueSnapshot())
}
