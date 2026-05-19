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

	// Only the local Ollama path needs us to drop a custom opencode.json
	// (so opencode learns the http://host.docker.internal:11434 base URL).
	// opencode's built-in cloud models (opencode/*) ship pre-configured;
	// writing an Ollama-only config there would force the run onto a
	// provider the model id doesn't belong to. Same for any future first-
	// class provider we surface — only ollama/* needs the override.
	if strings.HasPrefix(model, "ollama/") {
		ollamaBase := fallbackOllamaBase(cfg.Environment)
		if err := writeOpenCodeConfig(cfg.WorkspacePath, ollamaBase, model); err != nil {
			raw := "opencode setup failed: " + err.Error()
			turns, _ := e.ParseTranscript([]byte(raw))
			return &RunResult{RawOutput: raw, ParsedTurns: turns}, nil
		}
	}

	command := envOrOSGetenv(cfg.Environment, "FRAMEVAL_OPENCODE_COMMAND")
	if command == "" {
		// --format json gives us the structured event stream.
		// --model ollama/<id> uses the provider we defined above.
		// --dangerously-skip-permissions auto-approves "ask" permission
		//   prompts (e.g. `doom_loop`, `external_directory`). Without
		//   it the build agent silently aborts tool calls in our
		//   non-interactive `sh -c` sandbox and falls back to plain
		//   text output, so the model never actually edits files.
		// --thinking opts opencode into emitting `reasoning` events
		//   on top of the usual text/tool_use stream. Our parser maps
		//   these to BlockKindThinking and the Inspector renders them
		//   as italic muted prose — without the flag the model's
		//   internal monologue is dropped at the CLI boundary and the
		//   timeline only shows actions.
		// `stdbuf -oL` forces opencode's stdout into line-buffered
		//   mode. Without it Node/Bun block-buffers when stdout is a
		//   pipe (i.e. docker logs), holding events in 4-8 KiB chunks
		//   and only flushing at process exit — that's what made the
		//   Inspector "wait until the run finishes, then dump
		//   everything at once" instead of streaming turn-by-turn.
		command = `stdbuf -oL opencode run --format json --dangerously-skip-permissions --thinking --model "$OPENCODE_MODEL" "$FRAMEVAL_PROMPT"`
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
		turn.ToolUseID = extractPartMessageID(event.Part)
		turns = append(turns, *turn)
	}
	return assignOpenCodeGrouping(turns), nil
}

// assignOpenCodeGrouping stamps ParentTurnIndex by the opencode
// `messageID` carried on every event's part. Each unique messageID
// is one "agent decision" (a single model invocation + all the
// tool calls that decision spawned), so the Inspector renders them
// as a single TurnGroupCard — matching how the opencode TUI and
// Claude Code render an assistant step.
//
// Falls back to the per-event index when messageID is missing
// (junk lines, custom event types) so legacy callers keep working.
func assignOpenCodeGrouping(turns []ParsedTurn) []ParsedTurn {
	if len(turns) == 0 {
		return turns
	}
	groupFor := map[string]int{}
	nextGroup := 0
	for i := range turns {
		turns[i].TurnIndex = i
		msgID := turns[i].ToolUseID
		if msgID == "" {
			turns[i].ParentTurnIndex = nextGroup
			nextGroup++
			continue
		}
		if g, ok := groupFor[msgID]; ok {
			turns[i].ParentTurnIndex = g
		} else {
			groupFor[msgID] = nextGroup
			turns[i].ParentTurnIndex = nextGroup
			nextGroup++
		}
	}
	return turns
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
		// Ollama's OpenAI-compatible /v1/chat/completions endpoint
		// often returns a model's tool-call as plain text content
		// instead of populating message.tool_calls. opencode then
		// forwards that as a `text` event and the UI loses the
		// structure. Recover here: if the text body is itself a
		// `{"name": ..., "arguments": {...}}` envelope, promote it
		// back to a tool_use turn so the Inspector renders it under
		// "Tool use" and the right pane can display the target file.
		if toolName, files := detectTextToolCall(body); toolName != "" {
			return &ParsedTurn{
				Role:         "assistant",
				Content:      body,
				BlockKind:    BlockKindToolUse,
				ToolName:     toolName,
				FilesTouched: files,
			}
		}
		return &ParsedTurn{Role: "assistant", Content: body, BlockKind: BlockKindText}

	case "tool_use":
		toolName, input, files, output := extractToolUse(event.Part)
		if toolName == "" && input == "" {
			return nil
		}
		// opencode reports a model-side mistake (calling a tool that
		// doesn't exist, e.g. `globe` for `glob`) by emitting a
		// tool_use whose tool name is the literal "invalid" — the
		// `state.error` field then carries the human-readable reason.
		// Stamp Stage="error" so the Inspector's "Errors only" filter
		// chip surfaces it instead of treating it as a routine call.
		stage := ""
		if strings.EqualFold(toolName, "invalid") {
			stage = "error"
		}
		return &ParsedTurn{
			Role:         "assistant",
			Content:      input,
			BlockKind:    BlockKindToolUse,
			ToolName:     toolName,
			FilesTouched: files,
			ToolOutput:   output,
			Stage:        stage,
		}

	case "step_start", "step_finish":
		// step_start/finish are opencode's agent-loop iteration
		// boundaries (one "step" = one model call + its tools).
		// They have no human-readable payload — surfacing them as
		// turns just pollutes the Inspector with "System: step_start"
		// rows between every real action. Drop them here; the
		// underlying token/timing info lives on the surrounding
		// tool_use events that the user actually wants to see.
		return nil

	case "error":
		if event.Error == "" {
			return nil
		}
		return &ParsedTurn{Role: "system", Content: "error: " + event.Error, BlockKind: BlockKindSystem, Stage: "error"}
	}
	return nil
}

// extractPartMessageID pulls opencode's per-step `messageID` off any
// event's `part` payload. Returns "" if the field is absent (junk
// lines, custom event types, or partial flush windows). Used to
// group every turn from the same agent decision under one
// TurnGroupCard in the Inspector.
func extractPartMessageID(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var part struct {
		MessageID string `json:"messageID"`
	}
	if err := json.Unmarshal(raw, &part); err != nil {
		return ""
	}
	return part.MessageID
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
// paths referenced in the input + the agent-visible output the
// sandbox returned. opencode's tool_use part shape:
//
//	{ "tool": "Edit", "state": {
//	    "input":  { "path": "src/main.go", ... },
//	    "output": "<rendered text or stdout>",
//	    "metadata": { "preview": "..." }
//	}}
//
// We accept several legal-looking shapes since this is best-effort.
// Output prefers state.output (raw); falls back to state.metadata.preview
// (which read/glob etc. use for a truncated rendition).
func extractToolUse(raw json.RawMessage) (toolName, input string, files []string, output string) {
	if len(raw) == 0 {
		return "", "", nil, ""
	}
	var part struct {
		Tool  string          `json:"tool"`
		State json.RawMessage `json:"state"`
	}
	if err := json.Unmarshal(raw, &part); err != nil {
		return "", "", nil, ""
	}
	toolName = part.Tool

	var state struct {
		Input    map[string]any `json:"input"`
		Output   string         `json:"output"`
		Metadata map[string]any `json:"metadata"`
	}
	if err := json.Unmarshal(part.State, &state); err == nil {
		if state.Input != nil {
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
		output = strings.TrimSpace(state.Output)
		// read/glob tools sometimes leave output blank and only stash
		// a human preview under metadata.preview. Use it as a fallback
		// so the Inspector doesn't look like the call produced nothing.
		if output == "" && state.Metadata != nil {
			if v, ok := state.Metadata["preview"].(string); ok {
				output = strings.TrimSpace(v)
			}
		}
	}
	return toolName, input, files, output
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
// Ollama exposes an OpenAI-shaped /v1 API. opencode (≥1.15) refuses
// any --model not enumerated in the provider's `models` map with
// "Model not found", so we register the exact model id the run will
// invoke.
func writeOpenCodeConfig(workspacePath, ollamaBase, model string) error {
	// model arrives as "ollama/<id>"; strip the provider segment to
	// get the bare id opencode expects as the models map key.
	modelID := model
	if idx := strings.Index(model, "/"); idx >= 0 {
		modelID = model[idx+1:]
	}
	cfg := map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"provider": map[string]any{
			"ollama": map[string]any{
				"npm":  "@ai-sdk/openai-compatible",
				"name": "Ollama (local)",
				"options": map[string]any{
					"baseURL": ollamaBase,
				},
				"models": map[string]any{
					modelID: map[string]any{},
				},
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
	if n := normalizeOpenCodeModel(model); n != "" {
		return n
	}
	if n := normalizeOpenCodeModel(envOrOSGetenv(env, "OPENCODE_MODEL")); n != "" {
		return n
	}
	if n := normalizeOpenCodeModel(envOrOSGetenv(env, "AIDER_MODEL")); n != "" {
		return n
	}
	return "ollama/qwen2.5-coder:7b"
}

// detectTextToolCall inspects a text part body for the
// `{"name": "...", "arguments": {...}}` shape that Ollama-hosted
// models commonly emit when the underlying chat API doesn't surface
// proper tool_calls. Returns the tool name and any file paths
// referenced in the arguments, or ("", nil) if the body is plain
// prose. Tolerant of leading/trailing whitespace and code-fence
// wrappers ("```json ... ```") since small models sometimes wrap
// their structured output that way.
func detectTextToolCall(body string) (toolName string, files []string) {
	trimmed := strings.TrimSpace(body)
	// Strip a leading ```json (or just ```) and trailing ``` fence.
	if strings.HasPrefix(trimmed, "```") {
		if idx := strings.Index(trimmed, "\n"); idx >= 0 {
			trimmed = trimmed[idx+1:]
		}
		trimmed = strings.TrimSuffix(strings.TrimSpace(trimmed), "```")
		trimmed = strings.TrimSpace(trimmed)
	}
	if !strings.HasPrefix(trimmed, "{") {
		return "", nil
	}
	var envelope struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(trimmed), &envelope); err != nil {
		return "", nil
	}
	if envelope.Name == "" || envelope.Arguments == nil {
		return "", nil
	}
	// Pull every plausible path key. Different opencode tools use
	// different field names (write → filePath, edit → path,
	// multi-edit → files[].path). We collect the union so the
	// Inspector's file-filter chips light up regardless of which
	// shape the model used.
	for _, key := range []string{"filePath", "file_path", "path"} {
		if v, ok := envelope.Arguments[key].(string); ok && v != "" {
			files = append(files, v)
		}
	}
	if raw, ok := envelope.Arguments["files"].([]any); ok {
		for _, item := range raw {
			if obj, ok := item.(map[string]any); ok {
				if p, ok := obj["path"].(string); ok && p != "" {
					files = append(files, p)
				}
			} else if s, ok := item.(string); ok && s != "" {
				files = append(files, s)
			}
		}
	}
	return envelope.Name, files
}

// normalizeOpenCodeModel rewrites bare or LiteLLM-style ids onto the
// `ollama/` provider (the one writeOpenCodeConfig registers locally).
// Built-in opencode providers (opencode/*) are passed through
// unchanged so they reach opencode's bundled cloud transport rather
// than being misrouted through the local Ollama config.
func normalizeOpenCodeModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
	}
	if strings.HasPrefix(model, "opencode/") {
		return model
	}
	if idx := strings.Index(model, "/"); idx >= 0 {
		return "ollama/" + model[idx+1:]
	}
	return "ollama/" + model
}

// envOrOSGetenv is a tiny helper local to this package: prefer the
// caller-supplied env map, fall back to the process env. Mirrors
// the same helper used by aider.go to keep behaviour parallel.
// (Kept private here as a name-spaced duplicate to avoid cross-
// file coupling on a helper that may move.)
var _ = fmt.Sprintf
