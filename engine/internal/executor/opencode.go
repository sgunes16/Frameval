package executor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mustafaselman/frameval/engine/internal/sandbox"
)

// OpenCodeExecutor runs sst/opencode (https://opencode.ai) against a
// local Ollama endpoint. Unlike aider — whose chat-style stdout has
// no structural markers — opencode emits a stream of typed JSON
// events on stdout when invoked with --format json. Each event maps
// 1:1 onto the Inspector V2 BlockKind taxonomy, so this executor
// produces fully-stamped ParsedTurns directly, no heuristic parser
// needed downstream.
//
// Event mapping:
//
//	{type:"reasoning", part:…}     → BlockKindThinking
//	{type:"tool_use",  part:{tool, state:{input}}} → BlockKindToolUse + ToolName
//	{type:"text",      part:…}     → BlockKindText
//	{type:"step_start" | "step_finish"} → BlockKindSystem (informational)
//	{type:"error",     error:…}    → BlockKindSystem with the error string
//
// The executor materialises a per-run opencode.json in the
// workspace so opencode knows how to reach the configured Ollama
// endpoint. opencode picks the file up automatically from CWD.
type OpenCodeExecutor struct {
	sandbox *sandbox.Manager
}

func NewOpenCodeExecutor(manager *sandbox.Manager) *OpenCodeExecutor {
	return &OpenCodeExecutor{sandbox: manager}
}

func (e *OpenCodeExecutor) Name() string { return "opencode" }

func (e *OpenCodeExecutor) SupportedModes() []ExecutionMode {
	return []ExecutionMode{ExecutionModeCLI}
}

func (e *OpenCodeExecutor) Execute(ctx context.Context, cfg RunConfig) (*RunResult, error) {
	prompt := promptWithDefaultCLILanguage(cfg.Prompt)
	model := fallbackOpenCodeModel(cfg.Model, cfg.Environment)
	ollamaBase := fallbackOllamaBase(cfg.Environment)

	if err := writeOpenCodeConfig(cfg.WorkspacePath, ollamaBase); err != nil {
		raw := "opencode setup failed: " + err.Error()
		turns, _ := e.ParseTranscript([]byte(raw))
		return &RunResult{RawOutput: raw, ParsedTurns: turns}, nil
	}

	command := envOrOSGetenv(cfg.Environment, "FRAMEVAL_OPENCODE_COMMAND")
	if command == "" {
		// --format json gives us the structured event stream.
		// --model ollama/<id> uses the provider we defined above.
		// The prompt is positional and goes last via a here-doc so
		// shell quoting doesn't mangle multiline content.
		command = `opencode run --format json --model "$OPENCODE_MODEL" "$FRAMEVAL_PROMPT"`
	}
	env := mergeEnv(map[string]string{
		"FRAMEVAL_PROMPT": prompt,
		"OPENCODE_MODEL":  model,
		"OPENAI_API_KEY":  fallbackOllamaKey(cfg.Environment),
	}, cfg.Environment)

	output, err := e.sandbox.RunShellWithOutput(ctx, cfg.WorkspacePath, env, command, cfg.OnOutput)
	turns, _ := e.ParseTranscript([]byte(output))
	return &RunResult{RawOutput: output, ParsedTurns: turns, StreamedOutput: cfg.OnOutput != nil}, err
}

// ParseTranscript reads opencode's --format json line-by-line. Each
// line is one event; we map it to a ParsedTurn with the appropriate
// BlockKind already stamped. Unknown event types are skipped (NOT
// rendered as system blocks) so future event additions in newer
// opencode versions degrade silently rather than spamming the
// Inspector with mystery rows.
func (e *OpenCodeExecutor) ParseTranscript(raw []byte) ([]ParsedTurn, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return nil, nil
	}
	turns := make([]ParsedTurn, 0)
	scanner := bufio.NewScanner(strings.NewReader(text))
	// opencode emits one JSON object per line; some objects can
	// reasonably grow beyond bufio's default 64 KiB (large tool
	// outputs). Bump to 4 MiB.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var event opencodeEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Not a JSON event line — keep going.
			continue
		}
		turn := opencodeEventToTurn(event)
		if turn == nil {
			continue
		}
		turn.Timestamp = formatTimestamp(event.Timestamp)
		turns = append(turns, *turn)
	}
	return AssignTurnGrouping(turns), nil
}

// opencodeEvent is the minimal shape we read from opencode's
// --format json stream. We deliberately don't model every field —
// only what maps onto ParsedTurn. Extra fields are ignored.
type opencodeEvent struct {
	Type      string          `json:"type"`
	Timestamp int64           `json:"timestamp"`
	SessionID string          `json:"sessionID"`
	Part      json.RawMessage `json:"part,omitempty"`
	Error     string          `json:"error,omitempty"`
}

func opencodeEventToTurn(event opencodeEvent) *ParsedTurn {
	switch event.Type {
	case "reasoning":
		body, _ := extractPartText(event.Part)
		if body == "" {
			return nil
		}
		return &ParsedTurn{Role: "assistant", Content: body, BlockKind: BlockKindThinking}

	case "text":
		body, _ := extractPartText(event.Part)
		if body == "" {
			return nil
		}
		return &ParsedTurn{Role: "assistant", Content: body, BlockKind: BlockKindText}

	case "tool_use":
		toolName, input, files := extractToolUse(event.Part)
		if toolName == "" && input == "" {
			return nil
		}
		return &ParsedTurn{
			Role:         "assistant",
			Content:      input,
			BlockKind:    BlockKindToolUse,
			ToolName:     toolName,
			FilesTouched: files,
		}

	case "step_start", "step_finish":
		return &ParsedTurn{Role: "system", Content: event.Type, BlockKind: BlockKindSystem}

	case "error":
		if event.Error == "" {
			return nil
		}
		return &ParsedTurn{Role: "system", Content: "error: " + event.Error, BlockKind: BlockKindSystem, Stage: "error"}
	}
	return nil
}

// extractPartText pulls the user-visible body out of a `part`
// payload. opencode's `part` for text/reasoning carries the
// content under one of a few well-known keys; we try them in order
// and fall back to a JSON-stringified view if none match (rare;
// covers the case where opencode adds a new field shape).
func extractPartText(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return "", err
	}
	for _, key := range []string{"text", "content", "value", "body"} {
		if v, ok := generic[key].(string); ok && v != "" {
			return v, nil
		}
	}
	// Fallback: dump the whole part as JSON. Better than dropping
	// the event silently.
	out, _ := json.Marshal(generic)
	return string(out), nil
}

// extractToolUse reads the tool name + the tool input + any file
// paths referenced in the input. opencode's tool_use part shape:
//
//	{ "tool": "Edit", "state": { "input": { "path": "src/main.go", ... } } }
//
// We accept several legal-looking shapes since this is best-effort.
func extractToolUse(raw json.RawMessage) (toolName, input string, files []string) {
	if len(raw) == 0 {
		return "", "", nil
	}
	var part struct {
		Tool  string          `json:"tool"`
		State json.RawMessage `json:"state"`
	}
	if err := json.Unmarshal(raw, &part); err != nil {
		return "", "", nil
	}
	toolName = part.Tool

	var state struct {
		Input map[string]any `json:"input"`
	}
	if err := json.Unmarshal(part.State, &state); err == nil && state.Input != nil {
		if buf, err := json.MarshalIndent(state.Input, "", "  "); err == nil {
			input = string(buf)
		}
		for _, key := range []string{"path", "file", "filePath", "file_path"} {
			if v, ok := state.Input[key].(string); ok && v != "" {
				files = append(files, v)
				break
			}
		}
	}
	return toolName, input, files
}

func formatTimestamp(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

// writeOpenCodeConfig drops a minimal opencode.json into the
// workspace so opencode knows about the local Ollama endpoint. The
// CLI picks up the file automatically from CWD.
//
// The provider block uses `@ai-sdk/openai-compatible` because
// Ollama exposes an OpenAI-shaped /v1 API. The models map is
// intentionally empty — opencode will accept any model id under
// the `ollama/` prefix at invocation time (matches whatever the
// user has pulled in their local Ollama).
func writeOpenCodeConfig(workspacePath, ollamaBase string) error {
	cfg := map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"provider": map[string]any{
			"ollama": map[string]any{
				"npm":  "@ai-sdk/openai-compatible",
				"name": "Ollama (local)",
				"options": map[string]any{
					"baseURL": ollamaBase,
				},
				"models": map[string]any{},
			},
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(workspacePath, "opencode.json")
	return os.WriteFile(path, data, 0o644)
}

// fallbackOpenCodeModel resolves the model identifier we hand to
// `opencode run --model …`. Caller's cfg.Model wins; then the
// OPENCODE_MODEL env override; then a sane Ollama default.
func fallbackOpenCodeModel(model string, env map[string]string) string {
	if strings.TrimSpace(model) != "" {
		if !strings.Contains(model, "/") {
			// Caller passed a bare model ID; assume Ollama.
			return "ollama/" + model
		}
		return model
	}
	if value := envOrOSGetenv(env, "OPENCODE_MODEL"); value != "" {
		return value
	}
	if value := envOrOSGetenv(env, "AIDER_MODEL"); value != "" {
		// Reuse the AIDER_MODEL env when migrating from the old
		// executor. aider format is `openai/qwen2.5-coder:7b`;
		// opencode wants `ollama/qwen2.5-coder:7b`. Swap the
		// provider segment and keep the model id.
		if idx := strings.Index(value, "/"); idx >= 0 {
			return "ollama/" + value[idx+1:]
		}
		return "ollama/" + value
	}
	return "ollama/qwen2.5-coder:7b"
}

// envOrOSGetenv is a tiny helper local to this package: prefer the
// caller-supplied env map, fall back to the process env. Mirrors
// the same helper used by aider.go to keep behaviour parallel.
// (Kept private here as a name-spaced duplicate to avoid cross-
// file coupling on a helper that may move.)
var _ = fmt.Sprintf
