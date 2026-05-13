// Package diagnostic defines the data shapes produced by the AgentDx pipeline.
//
// AgentDx maps each agent run to a four-part Diagnostic Profile:
//
//   - Fingerprint:           10-dim behavioral vector (deterministic)
//   - Symptoms:               compact symptom packet (deterministic)
//   - RecoveryProfile:        error-event timeline + recovery metrics (deterministic)
//   - FailureClassification:  multi-label categorical failure mode (LLM via Anthropic Haiku)
//
// The deterministic stages live under engine/pkg/diagnostic and are computed
// in-process by the Go engine. The classifier runs in the Python grader sidecar
// via gRPC and is the only stage that calls an LLM.
//
// Full design rationale is in docs/superpowers/specs/2026-05-12-agentdx-design.md
// (§4.7 pipeline, Appendix A taxonomy, Appendix B fingerprint formulas).
package diagnostic
