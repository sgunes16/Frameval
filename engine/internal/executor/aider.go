package executor

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/mustafaselman/frameval/engine/internal/sandbox"
)

// AiderExecutor runs Aider against a local Ollama server (or any OpenAI-compatible endpoint).
//
// Defaults (overridable via env):
//
//	FRAMEVAL_AIDER_COMMAND   — full shell command (escape hatch; if set, used verbatim)
//	OLLAMA_BASE_URL          — OpenAI-compatible endpoint (default http://host.docker.internal:11434/v1)
//	AIDER_MODEL              — model identifier passed to Aider (default openai/qwen2.5-coder:7b)
//
// Aider's invocation:
//
//	aider --model "$AIDER_MODEL"
//	      --openai-api-base "$OLLAMA_BASE_URL"
//	      --openai-api-key "ollama"
//	      --no-stream --yes-always --no-pretty
//	      --message "$FRAMEVAL_PROMPT"
//
// Output capture:
//
//	stdout is streamed via cfg.OnOutput line-by-line (if set) and accumulated
//	for the final RunResult.RawOutput. Aider also writes .aider.chat.history.md
//	into the workspace; ParseTranscript prefers it when present, otherwise falls
//	back to stdout heuristics.
type AiderExecutor struct {
	sandbox *sandbox.Manager
}

// NewAiderExecutor constructs the Aider adapter. The sandbox manager owns the
// container lifecycle; this executor only issues shell commands inside it.
func NewAiderExecutor(manager *sandbox.Manager) *AiderExecutor {
	return &AiderExecutor{sandbox: manager}
}

func (e *AiderExecutor) Name() string { return "aider" }

func (e *AiderExecutor) SupportedModes() []ExecutionMode {
	return []ExecutionMode{ExecutionModeCLI}
}

// Execute invokes Aider in the sandbox workspace with the configured model and endpoint.
//
// The previous LookPath("aider") check ran on the HOST and false-negatived
// every run because aider lives inside the sandbox image, not on the
// developer's Mac. The sandbox shell-out below surfaces a real exit
// code if aider really is missing inside the container.
func (e *AiderExecutor) Execute(ctx context.Context, cfg RunConfig) (*RunResult, error) {
	prompt := promptWithDefaultCLILanguage(cfg.Prompt)
	command := envOrOSGetenv(cfg.Environment, "FRAMEVAL_AIDER_COMMAND")
	if command == "" {
		command = `aider --model "$AIDER_MODEL" --openai-api-base "$OPENAI_API_BASE" --openai-api-key "$OPENAI_API_KEY" --no-stream --yes-always --no-pretty --message "$FRAMEVAL_PROMPT"`
	}
	// Defaults first, caller's cfg.Environment overrides last (caller wins).
	env := mergeEnv(map[string]string{
		"FRAMEVAL_PROMPT": prompt,
		"AIDER_MODEL":     fallbackAiderModel(cfg.Model, cfg.Environment),
		"OPENAI_API_BASE": fallbackOllamaBase(cfg.Environment),
		"OPENAI_API_KEY":  fallbackOllamaKey(cfg.Environment),
	}, cfg.Environment)
	output, err := e.sandbox.RunShellWithOutput(ctx, cfg.WorkspacePath, env, command, cfg.OnOutput)
	turns, _ := e.ParseTranscript([]byte(output))
	return &RunResult{RawOutput: output, ParsedTurns: turns, StreamedOutput: cfg.OnOutput != nil}, err
}

// ParseTranscript turns Aider's stdout into structured turns.
//
// Aider's --no-stream output groups by speaker: a `user:`/`assistant:`/
// `tool:`/`system:` role marker introduces a turn, then every subsequent line
// (including blanks and continuations) belongs to that turn until the next
// role marker. The previous version emitted ONE TURN PER LINE which made the
// Agent Timeline render 50 tiny cards for a single assistant response.
//
// If the raw output has no role markers at all (e.g. raw stdout from a tool
// invocation), the whole blob is rendered as a single assistant turn.
func (e *AiderExecutor) ParseTranscript(raw []byte) ([]ParsedTurn, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return nil, nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	lines := strings.Split(text, "\n")

	turns := make([]ParsedTurn, 0)
	var current ParsedTurn
	var buf strings.Builder
	flush := func() {
		body := strings.TrimRight(buf.String(), "\n")
		if body == "" && current.Role == "" {
			return
		}
		current.Content = body
		if current.Role == "" {
			current.Role = "assistant"
		}
		if current.Timestamp == "" {
			current.Timestamp = now
		}
		// Inspector V2: map Aider's role-based output to the structured
		// BlockKind taxonomy. Aider's plain-text history exposes a `tool:`
		// prefix that is *already a combined tool_use+result emission* —
		// the agent has both invoked the tool and seen the result by the
		// time the line is flushed. Modeling it as tool_use would make
		// every Aider turn appear as an orphaned tool call to the Inspector
		// (because there's no matching tool_result), spamming the warning
		// glyph. Map it to text instead; downstream code that wants the
		// "this turn used a tool" signal reads Role == "tool".
		switch current.Role {
		case "system":
			current.BlockKind = BlockKindSystem
		default:
			current.BlockKind = BlockKindText
		}
		turns = append(turns, current)
		current = ParsedTurn{}
		buf.Reset()
	}
	for _, line := range lines {
		if role := detectAiderRoleStrict(line); role != "" {
			flush()
			current = ParsedTurn{Role: role, Timestamp: now}
			// Strip the role prefix off the first line; keep the rest.
			body := aiderRolePrefix.ReplaceAllString(line, "")
			if body != "" {
				buf.WriteString(body)
				buf.WriteString("\n")
			}
			continue
		}
		buf.WriteString(line)
		buf.WriteString("\n")
	}
	flush()
	return AssignTurnGrouping(turns), nil
}

var aiderRolePrefix = regexp.MustCompile(`(?i)^(user|assistant|tool|system)\s*:\s*`)

// detectAiderRoleStrict returns the role IFF the line opens with a role
// prefix; an empty string otherwise. Unlike the previous detectAiderRole,
// it does NOT default unmarked lines to "assistant" — that's the caller's
// job once it knows whether the line is a continuation or a new turn.
func detectAiderRoleStrict(line string) string {
	match := aiderRolePrefix.FindStringSubmatch(strings.ToLower(strings.TrimSpace(line)))
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func fallbackAiderModel(model string, env map[string]string) string {
	if strings.TrimSpace(model) != "" {
		return model
	}
	if value := envOrOSGetenv(env, "AIDER_MODEL"); value != "" {
		return value
	}
	return "openai/qwen2.5-coder:7b"
}

func fallbackOllamaBase(env map[string]string) string {
	if value := envOrOSGetenv(env, "OLLAMA_BASE_URL"); value != "" {
		return value
	}
	return "http://host.docker.internal:11434/v1"
}

func fallbackOllamaKey(env map[string]string) string {
	// Ollama ignores the key but Aider's OpenAI client requires one to be set.
	if value := envOrOSGetenv(env, "OPENAI_API_KEY"); value != "" {
		return value
	}
	return "ollama"
}

