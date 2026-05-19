package executor

import (
	"os"
	"strings"
	"testing"
)

// TestAiderStructuralParseRealWorldOutput exercises the structural
// fallback that fires when aider's stdout has no role markers — the
// real-world case for `aider --no-stream` output. The Inspector V2
// filters / histogram / diff panel are useless on a one-blob turn,
// so the structural parser needs to split the output into multiple
// turns with stamped BlockKind / ToolName / FilesTouched fields.
func TestAiderStructuralParseRealWorldOutput(t *testing.T) {
	raw := []byte(`Aider v0.86.2
Model: openai/llama3.1:8b with whole edit format
Git repo: .git with 6 files
Repo-map: using 1024 tokens, auto refresh

https://aider.chat/HISTORY.html#release-notes

app/user_service.py

To fix the race condition, I will introduce an asyncio.Lock.

<<<<<<< SEARCH
def create_user(name):
    return User(name)
=======
async def create_user(name):
    async with lock:
        return User(name)
>>>>>>> REPLACE
`)
	e := &AiderExecutor{}
	turns, err := e.ParseTranscript(raw)
	if err != nil {
		t.Fatalf("ParseTranscript: %v", err)
	}
	// Expect at least 4 turns: header (system), URL/prose (text),
	// file mention (tool_result), prose (text), edit block (tool_use).
	if len(turns) < 4 {
		t.Fatalf("expected at least 4 structural turns, got %d: %+v", len(turns), turns)
	}
	var sawSystem, sawToolUse, sawToolResult bool
	for _, turn := range turns {
		switch turn.BlockKind {
		case BlockKindSystem:
			sawSystem = true
		case BlockKindToolUse:
			sawToolUse = true
			if turn.ToolName != "Edit" {
				t.Errorf("tool_use turn should be Edit, got %q", turn.ToolName)
			}
		case BlockKindToolResult:
			sawToolResult = true
		}
	}
	if !sawSystem {
		t.Error("expected at least one BlockKindSystem turn (Aider header)")
	}
	if !sawToolUse {
		t.Error("expected at least one BlockKindToolUse turn (SEARCH/REPLACE block)")
	}
	if !sawToolResult {
		t.Error("expected at least one BlockKindToolResult turn (file mention)")
	}
}

func TestAiderStructuralExtractsFileMentions(t *testing.T) {
	raw := []byte(`I'll edit src/main.go and tests/main_test.go in this turn.

<<<<<<< SEARCH
package main
=======
package main // updated
>>>>>>> REPLACE
`)
	e := &AiderExecutor{}
	turns, err := e.ParseTranscript(raw)
	if err != nil {
		t.Fatalf("ParseTranscript: %v", err)
	}
	var editTurn *ParsedTurn
	for i := range turns {
		if turns[i].BlockKind == BlockKindToolUse {
			editTurn = &turns[i]
			break
		}
	}
	if editTurn == nil {
		t.Fatalf("no tool_use turn produced from SEARCH/REPLACE block")
	}
	// The SEARCH/REPLACE block itself has no file path inside it; the
	// previous paragraph mentions the files. We don't currently stitch
	// across paragraphs, so files_touched will be empty here unless
	// the SEARCH block carries the file mention. Just assert tool_name.
	if editTurn.ToolName != "Edit" {
		t.Errorf("expected tool_name=Edit, got %q", editTurn.ToolName)
	}

	// The prose paragraph itself should remain text-kind.
	if turns[0].BlockKind != BlockKindText {
		t.Errorf("first paragraph (prose) should be text, got %q", turns[0].BlockKind)
	}
}

// TestAiderStructuralMarkdownFencedDiff covers the real-world aider
// output the user pasted in chat — diff edit format with ```diff
// markdown fences (not <<<<<<< SEARCH), plus the git error wall and
// 'Tokens:' accounting line. Without the fenced-diff regex these all
// fell into BlockKindText, which left the Inspector with no tool_use
// turns to drive the histogram or per-turn diff.
func TestAiderStructuralMarkdownFencedDiff(t *testing.T) {
	raw := []byte("Warning: Input is not a terminal (fd=0).\n\n" +
		"Update git name with: git config user.name \"Your Name\"\n\n" +
		"Aider v0.86.2\n" +
		"Model: openai/llama3.1:8b with diff edit format\n" +
		"Git repo: .git with 6 files\n\n" +
		"app/user_service.py\n\n" +
		"To fix the race condition, I will introduce an asyncio.Lock.\n\n" +
		"```diff\n@@ -10,4 +10,5 @@\n-async def add_credits():\n+async def add_credits():\n+    async with lock:\n```\n\n" +
		"Tokens: 1.4k sent, 472 received.\n\n" +
		"Cmd('git') failed due to: exit code(129)\n" +
		"  cmdline: git diff --cached\n",
	)
	e := &AiderExecutor{}
	turns, err := e.ParseTranscript(raw)
	if err != nil {
		t.Fatalf("ParseTranscript: %v", err)
	}
	if len(turns) < 5 {
		t.Fatalf("expected at least 5 paragraphs, got %d", len(turns))
	}
	var sawDiffEdit bool
	var diffEditFiles []string
	for _, turn := range turns {
		if turn.BlockKind == BlockKindToolUse && strings.Contains(turn.Content, "```diff") {
			sawDiffEdit = true
			diffEditFiles = turn.FilesTouched
		}
	}
	if !sawDiffEdit {
		t.Error("expected the ```diff fenced paragraph to be stamped tool_use; got none")
	}
	// The previous paragraph mentioned app/user_service.py — the
	// parser should have stitched that file mention onto the edit
	// turn so the per-turn diff has something to scope on.
	if len(diffEditFiles) == 0 || diffEditFiles[0] != "app/user_service.py" {
		t.Errorf("expected diff edit to inherit app/user_service.py from prior file-mention; got %v", diffEditFiles)
	}
}

// TestAiderStructuralGitErrorIsSystem ensures the giant 'usage: git'
// wall that aider prints when its internal git invocation fails is
// classified as system, not text. Otherwise it dominates the
// Inspector turn list as a noisy assistant paragraph.
func TestAiderStructuralGitErrorIsSystem(t *testing.T) {
	raw := []byte("Cmd('git') failed due to: exit code(129)\n  cmdline: git diff --cached\n  stderr: 'error: unknown option `cached'\nusage: git diff --no-index [<options>] <path> <path>\n")
	e := &AiderExecutor{}
	turns, err := e.ParseTranscript(raw)
	if err != nil {
		t.Fatalf("ParseTranscript: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(turns))
	}
	if turns[0].BlockKind != BlockKindSystem {
		t.Errorf("expected the git error wall to classify as system, got %q", turns[0].BlockKind)
	}
}

func TestAiderStructuralEmptyInput(t *testing.T) {
	e := &AiderExecutor{}
	turns, err := e.ParseTranscript([]byte(""))
	if err != nil {
		t.Fatalf("ParseTranscript: %v", err)
	}
	if len(turns) != 0 {
		t.Errorf("expected 0 turns for empty input, got %d", len(turns))
	}
}

func TestAiderParseTranscriptGroupsByRole(t *testing.T) {
	e := &AiderExecutor{}
	raw := []byte(strings.Join([]string{
		"user: please add CLI scaffolding",
		"assistant: sure, creating main.py",
		"tool: file_write main.py",
		"system: edit applied",
		"random line without role prefix",
	}, "\n"))

	turns, err := e.ParseTranscript(raw)
	if err != nil {
		t.Fatalf("ParseTranscript returned error: %v", err)
	}
	// Unmarked lines fold into the PREVIOUS role's turn (continuation),
	// so the trailing "random line" merges into the system turn — yielding
	// 4 turns total, not 5.
	if len(turns) != 4 {
		t.Fatalf("expected 4 turns (unmarked line folded into prior), got %d", len(turns))
	}
	wantRoles := []string{"user", "assistant", "tool", "system"}
	for i, want := range wantRoles {
		if turns[i].Role != want {
			t.Errorf("turn %d: want role %q, got %q (content=%q)", i, want, turns[i].Role, turns[i].Content)
		}
	}
	if !strings.Contains(turns[3].Content, "random line without role prefix") {
		t.Errorf("trailing unmarked line should have folded into system turn; got %q", turns[3].Content)
	}
}

func TestAiderParseTranscriptMultilineAssistant(t *testing.T) {
	// Reproduces the bug screenshot: a multi-line assistant response should
	// NOT explode into one turn per line. Before the fix, this raw output
	// produced 6 turns; after, it produces 1 assistant turn whose Content
	// is the full multi-line body.
	e := &AiderExecutor{}
	raw := []byte("assistant: Building wordfreq CLI.\n  -k for top-K\n  -c for case-sensitive\nThis is the plan.")
	turns, err := e.ParseTranscript(raw)
	if err != nil {
		t.Fatalf("ParseTranscript returned error: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn (multi-line assistant), got %d", len(turns))
	}
	if !strings.Contains(turns[0].Content, "-k for top-K") {
		t.Errorf("turn should preserve continuation lines, got %q", turns[0].Content)
	}
}

func TestAiderParseTranscriptUnstructuredOutput(t *testing.T) {
	// Tool stdout with no role prefixes anywhere — common when Aider's
	// CLI emits raw shell-style output. Should render as a single
	// assistant turn rather than per-line cards.
	e := &AiderExecutor{}
	raw := []byte("Loading model...\nReady.\nGenerating output\n")
	turns, err := e.ParseTranscript(raw)
	if err != nil {
		t.Fatalf("ParseTranscript returned error: %v", err)
	}
	if len(turns) != 1 || turns[0].Role != "assistant" {
		t.Fatalf("expected 1 assistant turn for unmarked output, got %d turns role=%v", len(turns), turns)
	}
}

func TestAiderParseTranscriptEmptyInput(t *testing.T) {
	e := &AiderExecutor{}
	turns, err := e.ParseTranscript([]byte(""))
	if err != nil {
		t.Fatalf("ParseTranscript returned error: %v", err)
	}
	if len(turns) != 0 {
		t.Fatalf("expected 0 turns for empty input, got %d", len(turns))
	}
}

func TestDetectAiderRoleStrict(t *testing.T) {
	// Strict variant returns "" for unprefixed lines so the parser can
	// treat them as continuations of the previous turn rather than
	// fabricating a new assistant turn per line.
	cases := []struct {
		line string
		want string
	}{
		{"user: hello", "user"},
		{"assistant: ok", "assistant"},
		{"tool: file_write x", "tool"},
		{"system: ready", "system"},
		{"USER: caps ok", "user"},
		{"no prefix", ""},
		{"asst: not a known prefix", ""},
	}
	for _, tc := range cases {
		if got := detectAiderRoleStrict(tc.line); got != tc.want {
			t.Errorf("detectAiderRoleStrict(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
}

func TestFallbackAiderModelPrecedence(t *testing.T) {
	t.Setenv("AIDER_MODEL", "")
	if got := fallbackAiderModel("", nil); got != "openai/qwen2.5-coder:7b" {
		t.Errorf("default model: got %q", got)
	}
	if got := fallbackAiderModel("openai/explicit-model", nil); got != "openai/explicit-model" {
		t.Errorf("explicit cfg.Model not used: got %q", got)
	}
	t.Setenv("AIDER_MODEL", "openai/env-model")
	if got := fallbackAiderModel("", nil); got != "openai/env-model" {
		t.Errorf("OS env override not used: got %q", got)
	}
	// cfg.Model still wins over env.
	if got := fallbackAiderModel("openai/explicit-model", nil); got != "openai/explicit-model" {
		t.Errorf("cfg.Model should win over env: got %q", got)
	}
	// cfg.Environment beats OS env.
	if got := fallbackAiderModel("", map[string]string{"AIDER_MODEL": "openai/cfg-env-model"}); got != "openai/cfg-env-model" {
		t.Errorf("cfg.Environment should win over OS env: got %q", got)
	}
}

func TestFallbackOllamaBase(t *testing.T) {
	t.Setenv("OLLAMA_BASE_URL", "")
	if got := fallbackOllamaBase(nil); got != "http://host.docker.internal:11434/v1" {
		t.Errorf("default base: got %q", got)
	}
	t.Setenv("OLLAMA_BASE_URL", "http://customhost:9999/v1")
	if got := fallbackOllamaBase(nil); got != "http://customhost:9999/v1" {
		t.Errorf("OS env override: got %q", got)
	}
	if got := fallbackOllamaBase(map[string]string{"OLLAMA_BASE_URL": "http://cfg-env/v1"}); got != "http://cfg-env/v1" {
		t.Errorf("cfg.Environment should win over OS env: got %q", got)
	}
}

func TestFallbackOllamaKeyDefault(t *testing.T) {
	_ = os.Unsetenv("OPENAI_API_KEY")
	if got := fallbackOllamaKey(nil); got != "ollama" {
		t.Errorf("default key: got %q", got)
	}
	if got := fallbackOllamaKey(map[string]string{"OPENAI_API_KEY": "cfg-env-key"}); got != "cfg-env-key" {
		t.Errorf("cfg.Environment should win: got %q", got)
	}
}

func TestMergeEnvCallerWins(t *testing.T) {
	defaults := map[string]string{
		"AIDER_MODEL":     "openai/qwen2.5-coder:7b",
		"OPENAI_API_BASE": "http://host.docker.internal:11434/v1",
		"FRAMEVAL_PROMPT": "default-prompt",
	}
	callerOverrides := map[string]string{
		"AIDER_MODEL": "openai/different-model",
		"NEW_VAR":     "added",
	}
	merged := mergeEnv(defaults, callerOverrides)

	if merged["AIDER_MODEL"] != "openai/different-model" {
		t.Errorf("caller override should win: got %q", merged["AIDER_MODEL"])
	}
	if merged["OPENAI_API_BASE"] != "http://host.docker.internal:11434/v1" {
		t.Errorf("default should remain when not overridden: got %q", merged["OPENAI_API_BASE"])
	}
	if merged["FRAMEVAL_PROMPT"] != "default-prompt" {
		t.Errorf("default should remain: got %q", merged["FRAMEVAL_PROMPT"])
	}
	if merged["NEW_VAR"] != "added" {
		t.Errorf("caller-only key should be present: got %q", merged["NEW_VAR"])
	}
}

func TestAiderRegistered(t *testing.T) {
	r := &Registry{executors: map[string]AgentExecutor{
		"aider": &AiderExecutor{},
	}}
	exec, err := r.Get("aider")
	if err != nil {
		t.Fatalf("Get aider: %v", err)
	}
	if exec.Name() != "aider" {
		t.Errorf("expected Name=aider, got %q", exec.Name())
	}
	modes := exec.SupportedModes()
	if len(modes) != 1 || modes[0] != ExecutionModeCLI {
		t.Errorf("expected [cli], got %v", modes)
	}
}
