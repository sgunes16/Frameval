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
// When the raw output has no role markers at all (the real-world case for
// `aider --no-stream` output piped from the CLI), the parser falls into a
// structural mode that splits by paragraph and stamps BlockKind via Aider-
// specific heuristics (SEARCH/REPLACE blocks → tool_use, header lines →
// system, file mentions → tool_result). Without this fallback the Inspector
// V2 view shows a single 'Turn 0 · Assistant' blob with the entire run
// crammed in — which is what the user sees when nothing else stamps blocks.
func (e *AiderExecutor) ParseTranscript(raw []byte) ([]ParsedTurn, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return nil, nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	// Real Aider CLI output has no role prefixes at all. Detect that case
	// up front and route to the structural parser; the old role-based path
	// only fires when the transcript came from a wrapper that did stamp
	// `user:` / `assistant:` markers (tests + a few legacy integrations).
	if !hasAiderRoleMarkers(text) {
		return AssignTurnGrouping(parseAiderStructural(text, now)), nil
	}
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

// hasAiderRoleMarkers detects whether the transcript carries the
// `user:` / `assistant:` / `tool:` / `system:` line prefixes the old
// role-based parser expects. Real `aider --no-stream` output has none
// of these — the wrapper-based test fixtures do.
func hasAiderRoleMarkers(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		if aiderRolePrefix.MatchString(strings.ToLower(strings.TrimSpace(line))) {
			return true
		}
	}
	return false
}

// aiderHeaderLine matches the well-known Aider banner lines emitted at
// the top of every CLI run AND the misc system lines (token accounting,
// git errors) that appear in the middle/end of a run.
var aiderHeaderLine = regexp.MustCompile(
	`(?i)^(Aider v|Model:|Git repo:|Repo-map:|Warning(?: for [^:]+)?:|Update git \w+ with:|You can skip this check|Tokens: |Cost: |Added [^ ]+ to the chat|Cmd\('git'\) failed|usage: git|error: unknown option|Input is not a terminal)`,
)

// aiderFilePath matches a bare file path on its own — Aider prints
// these when listing files in/added to the chat. The list is
// conservative; only extensions that show up in real Aider output
// are recognised so we don't false-positive on prose.
var aiderFilePath = regexp.MustCompile(
	`^[A-Za-z0-9._/\-]+\.(go|py|ts|tsx|js|jsx|md|json|yaml|yml|toml|sql|sh|rs|java|cpp|c|h|hpp|rb|html|css|cfg|env|txt)$`,
)

// aiderSearchReplaceStart marks the opening of an Aider edit block —
// the agent's actual code change. The matching block_kind is tool_use
// with tool_name=Edit. Aider emits edits in two formats:
//   - whole edit format: `<<<<<<< SEARCH … ======= … >>>>>>> REPLACE`
//   - diff edit format:  fenced markdown `` ```diff `` block
// Either opener is enough to classify the paragraph as an edit.
var aiderSearchReplaceStart = regexp.MustCompile(`(?m)^<{5,}\s*SEARCH\b`)

// aiderDiffFenceStart matches a markdown-fenced diff block.
// Aider's `--edit-format diff` writes patches as ` ```diff ` fences,
// which we treat as tool_use just like the SEARCH/REPLACE form.
var aiderDiffFenceStart = regexp.MustCompile("(?m)^```diff\\b")

// fileMentionInLine extracts any file paths referenced inside a
// paragraph (e.g. "I'll edit src/main.go and tests/foo.py"). Used to
// populate FilesTouched on tool_use blocks.
var fileMentionInLine = regexp.MustCompile(
	`[A-Za-z0-9_./-]+\.(?:go|py|ts|tsx|js|jsx|md|json|yaml|yml|toml|sql|sh|rs|java|cpp|c|h|hpp|rb|html|css)`,
)

// parseAiderStructural splits prefix-less Aider output into turns by
// paragraph (`\n\n+` boundaries) and stamps each turn with a best-
// guess block_kind, tool_name and files_touched. Heuristic — not a
// full Aider grammar — but enough that the Inspector V2 filters
// (Thinking / Tool use / Tool result / Errors) become meaningful and
// the Tool Histogram shows non-zero counts.
func parseAiderStructural(text, now string) []ParsedTurn {
	paragraphs := splitParagraphs(text)
	turns := make([]ParsedTurn, 0, len(paragraphs))
	// Tracks the most-recently-named file across paragraphs so edit
	// blocks (SEARCH/REPLACE, ```diff fences) inherit the path from
	// the bare file-mention line aider prints before them. Reset on
	// any system-class turn so we don't carry a file from one logical
	// section to the next.
	var lastMentionedFiles []string
	for _, p := range paragraphs {
		if strings.TrimSpace(p) == "" {
			continue
		}
		turn := ParsedTurn{
			Role:      "assistant",
			Content:   p,
			Timestamp: now,
			BlockKind: classifyAiderParagraph(p),
		}
		switch turn.BlockKind {
		case BlockKindToolUse:
			turn.ToolName = "Edit"
			files := extractFileMentions(p)
			if len(files) == 0 && len(lastMentionedFiles) > 0 {
				files = append([]string(nil), lastMentionedFiles...)
			}
			turn.FilesTouched = files
		case BlockKindToolResult:
			turn.ToolName = "Read"
			files := extractFileMentions(p)
			turn.FilesTouched = files
			if len(files) > 0 {
				lastMentionedFiles = files
			}
		case BlockKindSystem:
			lastMentionedFiles = nil
		}
		turns = append(turns, turn)
	}
	return turns
}

func splitParagraphs(text string) []string {
	// Collapse runs of 2+ newlines into the split delimiter. Aider
	// prints blank lines between logical sections.
	out := []string{}
	current := strings.Builder{}
	prevBlank := false
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			if prevBlank {
				continue
			}
			prevBlank = true
			if current.Len() > 0 {
				out = append(out, strings.TrimRight(current.String(), "\n"))
				current.Reset()
			}
			continue
		}
		prevBlank = false
		current.WriteString(line)
		current.WriteByte('\n')
	}
	if current.Len() > 0 {
		out = append(out, strings.TrimRight(current.String(), "\n"))
	}
	return out
}

func classifyAiderParagraph(p string) string {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" {
		return BlockKindText
	}
	// SEARCH/REPLACE blocks AND ```diff fences are unambiguous edits.
	if aiderSearchReplaceStart.MatchString(trimmed) || aiderDiffFenceStart.MatchString(trimmed) {
		return BlockKindToolUse
	}
	firstLine := trimmed
	if idx := strings.IndexByte(trimmed, '\n'); idx >= 0 {
		firstLine = trimmed[:idx]
	}
	if aiderHeaderLine.MatchString(firstLine) {
		return BlockKindSystem
	}
	if aiderFilePath.MatchString(firstLine) {
		return BlockKindToolResult
	}
	return BlockKindText
}

func extractFileMentions(p string) []string {
	matches := fileMentionInLine.FindAllString(p, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, m := range matches {
		if seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}

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

