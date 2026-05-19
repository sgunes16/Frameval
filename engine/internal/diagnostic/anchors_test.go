package diagnostic

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
)

// tu builds a tool_use ParsedTurn with the fields anchorKey reads.
func tu(turnIdx, parentIdx int, toolName string, files []string, content string) executor.ParsedTurn {
	return executor.ParsedTurn{
		BlockKind:       "tool_use",
		ToolName:        toolName,
		FilesTouched:    files,
		Content:         content,
		TurnIndex:       turnIdx,
		ParentTurnIndex: parentIdx,
	}
}

func TestBuildAnchors_EmptyInputProducesEmptyBundle(t *testing.T) {
	got := BuildAnchors(nil)
	if len(got.Runs) != 0 {
		t.Fatalf("expected empty bundle, got %d runs", len(got.Runs))
	}
}

func TestBuildAnchors_OnlyToolUseBlocksProduceAnchors(t *testing.T) {
	turns := []executor.ParsedTurn{
		{BlockKind: "thinking", Content: "reasoning"},
		tu(1, 0, "Edit", []string{"src/main.go"}, ""),
		{BlockKind: "tool_result", Content: "ok", FilesTouched: []string{"src/main.go"}, ToolName: "Edit"},
		{BlockKind: "text", Content: "done"},
	}
	got := BuildAnchors(map[string][]executor.ParsedTurn{"r1": turns})
	if len(got.Runs) != 1 || len(got.Runs[0].Anchors) != 1 {
		t.Fatalf("expected 1 run with 1 anchor, got %+v", got)
	}
	if got.Runs[0].Anchors[0].Key != "Edit|src/main.go" {
		t.Fatalf("unexpected key: %q", got.Runs[0].Anchors[0].Key)
	}
}

func TestBuildAnchors_MultiFileKeyIsSortedCanonical(t *testing.T) {
	// The key must be stable regardless of files_touched ordering —
	// two runs that touched the same set in different order share
	// one anchor.
	turns1 := []executor.ParsedTurn{tu(0, 0, "Edit", []string{"b.go", "a.go"}, "")}
	turns2 := []executor.ParsedTurn{tu(0, 0, "Edit", []string{"a.go", "b.go"}, "")}
	got := BuildAnchors(map[string][]executor.ParsedTurn{"r1": turns1, "r2": turns2})
	k1 := got.Runs[0].Anchors[0].Key
	k2 := got.Runs[1].Anchors[0].Key
	if k1 != k2 {
		t.Fatalf("keys should match after sort, got %q vs %q", k1, k2)
	}
	if k1 != "Edit|a.go,b.go" {
		t.Fatalf("unexpected canonical key: %q", k1)
	}
}

func TestBuildAnchors_BashUsesContentHash(t *testing.T) {
	turns := []executor.ParsedTurn{
		tu(0, 0, "Bash", nil, "ls -la /tmp"),
		tu(1, 1, "Bash", nil, "ls -la /tmp"),
		tu(2, 2, "Bash", nil, "rm -rf /tmp"),
	}
	got := BuildAnchors(map[string][]executor.ParsedTurn{"r1": turns})
	anchors := got.Runs[0].Anchors
	if len(anchors) != 3 {
		t.Fatalf("expected 3 anchors, got %d", len(anchors))
	}
	// Same command -> same key; different command -> different key.
	if anchors[0].Key != anchors[1].Key {
		t.Fatalf("identical commands should produce identical keys: %q vs %q", anchors[0].Key, anchors[1].Key)
	}
	if anchors[0].Key == anchors[2].Key {
		t.Fatalf("different commands should produce different keys: %q", anchors[0].Key)
	}
	if !strings.HasPrefix(string(anchors[0].Key), "Bash|") {
		t.Fatalf("expected Bash| prefix, got %q", anchors[0].Key)
	}
	// 8 hex chars after the pipe.
	parts := strings.SplitN(string(anchors[0].Key), "|", 2)
	if len(parts) != 2 || len(parts[1]) != 8 {
		t.Fatalf("expected 8-char hash, got %q", anchors[0].Key)
	}
}

func TestBuildAnchors_DropsBlocksWithoutContentAndWithoutFiles(t *testing.T) {
	// A Bash invocation with no files_touched AND empty content has
	// nothing to hash on — drop it rather than produce a noisy
	// "Bash|" key that would collide with every other empty-content
	// Bash in the experiment.
	turns := []executor.ParsedTurn{tu(0, 0, "Bash", nil, "")}
	got := BuildAnchors(map[string][]executor.ParsedTurn{"r1": turns})
	if len(got.Runs[0].Anchors) != 0 {
		t.Fatalf("expected 0 anchors for empty-content no-files Bash, got %+v", got.Runs[0].Anchors)
	}
}

func TestBuildAnchors_DropsToolUseWithoutToolName(t *testing.T) {
	turns := []executor.ParsedTurn{
		{BlockKind: "tool_use", FilesTouched: []string{"src/x"}}, // missing ToolName
		tu(1, 1, "Edit", []string{"src/x"}, ""),
	}
	got := BuildAnchors(map[string][]executor.ParsedTurn{"r1": turns})
	if len(got.Runs[0].Anchors) != 1 {
		t.Fatalf("expected the unnamed tool_use to be skipped, got %d anchors", len(got.Runs[0].Anchors))
	}
}

func TestBuildAnchors_RunsOrderedByID(t *testing.T) {
	// Determinism: BuildAnchors must produce the same output order
	// regardless of map iteration order so the cached JSON payload
	// doesn't churn on every rebuild.
	turns := []executor.ParsedTurn{tu(0, 0, "Edit", []string{"x"}, "")}
	got := BuildAnchors(map[string][]executor.ParsedTurn{
		"run-z": turns,
		"run-a": turns,
		"run-m": turns,
	})
	want := []string{"run-a", "run-m", "run-z"}
	gotIDs := []string{got.Runs[0].RunID, got.Runs[1].RunID, got.Runs[2].RunID}
	if !reflect.DeepEqual(gotIDs, want) {
		t.Fatalf("run order = %v, want %v", gotIDs, want)
	}
}

func TestBuildAnchors_AnchorsOrderedByTurnIndex(t *testing.T) {
	// Out-of-order input must produce in-order output so the Tape UI
	// can walk it linearly.
	turns := []executor.ParsedTurn{
		tu(5, 5, "Edit", []string{"c"}, ""),
		tu(1, 1, "Edit", []string{"a"}, ""),
		tu(3, 3, "Edit", []string{"b"}, ""),
	}
	got := BuildAnchors(map[string][]executor.ParsedTurn{"r1": turns})
	anchors := got.Runs[0].Anchors
	if len(anchors) != 3 || anchors[0].TurnIndex != 1 || anchors[1].TurnIndex != 3 || anchors[2].TurnIndex != 5 {
		t.Fatalf("expected anchors sorted by turn_index, got %+v", anchors)
	}
}
