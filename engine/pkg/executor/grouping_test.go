package executor_test

import (
	"testing"

	"github.com/mustafaselman/frameval/engine/pkg/executor"
)

// AssignTurnGrouping walks the parsed turns and stamps TurnIndex /
// ParentTurnIndex on each block according to the rules in the
// Inspector V2 spec:
//
//   - TurnIndex is a monotonic 0-based counter across the whole transcript.
//   - A `tool_use` block plus its matching `tool_result` (by ToolUseID) share a
//     ParentTurnIndex.
//   - A `thinking` block immediately preceding a `tool_use` joins that group.
//   - A `text` block following a `tool_result` joins the preceding group.
//   - Anything else starts a new ParentTurnIndex.
//
// Tests below pass shaped slices and assert the stamps come out right.

func TestAssignTurnGrouping_MonotonicTurnIndex(t *testing.T) {
	turns := []executor.ParsedTurn{
		{BlockKind: "text", Role: "assistant"},
		{BlockKind: "thinking", Role: "assistant"},
		{BlockKind: "tool_use", Role: "assistant", ToolUseID: "t1"},
		{BlockKind: "tool_result", Role: "tool", ToolUseID: "t1"},
	}

	out := executor.AssignTurnGrouping(turns)

	for i, turn := range out {
		if turn.TurnIndex != i {
			t.Errorf("turn %d: want TurnIndex=%d, got %d", i, i, turn.TurnIndex)
		}
	}
}

func TestAssignTurnGrouping_ToolUseAndResultShareParent(t *testing.T) {
	turns := []executor.ParsedTurn{
		{BlockKind: "tool_use", ToolUseID: "edit-1"},
		{BlockKind: "tool_result", ToolUseID: "edit-1"},
	}
	out := executor.AssignTurnGrouping(turns)

	if out[0].ParentTurnIndex != out[1].ParentTurnIndex {
		t.Errorf("tool_use + matching tool_result must share parent; got %d vs %d",
			out[0].ParentTurnIndex, out[1].ParentTurnIndex)
	}
}

func TestAssignTurnGrouping_ThinkingBeforeToolJoinsGroup(t *testing.T) {
	turns := []executor.ParsedTurn{
		{BlockKind: "thinking", Role: "assistant"},
		{BlockKind: "tool_use", ToolUseID: "edit-1"},
		{BlockKind: "tool_result", ToolUseID: "edit-1"},
	}
	out := executor.AssignTurnGrouping(turns)

	if !(out[0].ParentTurnIndex == out[1].ParentTurnIndex && out[1].ParentTurnIndex == out[2].ParentTurnIndex) {
		t.Errorf("thinking before tool_use should join the tool group; parents=[%d %d %d]",
			out[0].ParentTurnIndex, out[1].ParentTurnIndex, out[2].ParentTurnIndex)
	}
}

func TestAssignTurnGrouping_TextAfterToolResultJoinsPrevious(t *testing.T) {
	turns := []executor.ParsedTurn{
		{BlockKind: "tool_use", ToolUseID: "edit-1"},
		{BlockKind: "tool_result", ToolUseID: "edit-1"},
		{BlockKind: "text", Role: "assistant"},
	}
	out := executor.AssignTurnGrouping(turns)

	if !(out[0].ParentTurnIndex == out[1].ParentTurnIndex && out[1].ParentTurnIndex == out[2].ParentTurnIndex) {
		t.Errorf("text after tool_result should join the preceding group; parents=[%d %d %d]",
			out[0].ParentTurnIndex, out[1].ParentTurnIndex, out[2].ParentTurnIndex)
	}
}

func TestAssignTurnGrouping_NewGroupStartsOnFreshThinking(t *testing.T) {
	turns := []executor.ParsedTurn{
		{BlockKind: "tool_use", ToolUseID: "a"},
		{BlockKind: "tool_result", ToolUseID: "a"},
		{BlockKind: "thinking"},
		{BlockKind: "tool_use", ToolUseID: "b"},
		{BlockKind: "tool_result", ToolUseID: "b"},
	}
	out := executor.AssignTurnGrouping(turns)

	if out[0].ParentTurnIndex == out[3].ParentTurnIndex {
		t.Errorf("two distinct tool decisions must have distinct parents; got %d == %d",
			out[0].ParentTurnIndex, out[3].ParentTurnIndex)
	}
	// Within each decision, the three / two members share a parent.
	if out[0].ParentTurnIndex != out[1].ParentTurnIndex {
		t.Errorf("first tool_use + tool_result must share parent")
	}
	if !(out[2].ParentTurnIndex == out[3].ParentTurnIndex && out[3].ParentTurnIndex == out[4].ParentTurnIndex) {
		t.Errorf("second decision: thinking+tool_use+tool_result must share parent; parents=[%d %d %d]",
			out[2].ParentTurnIndex, out[3].ParentTurnIndex, out[4].ParentTurnIndex)
	}
}

func TestAssignTurnGrouping_OrphanedToolResultGetsOwnGroup(t *testing.T) {
	// tool_result without a matching tool_use should not be silently
	// merged into whatever came before — it gets its own parent.
	turns := []executor.ParsedTurn{
		{BlockKind: "text"},
		{BlockKind: "tool_result", ToolUseID: "no-match"},
	}
	out := executor.AssignTurnGrouping(turns)

	if out[0].ParentTurnIndex == out[1].ParentTurnIndex {
		t.Errorf("orphan tool_result must not share parent with preceding text; got %d == %d",
			out[0].ParentTurnIndex, out[1].ParentTurnIndex)
	}
}

func TestAssignTurnGrouping_EmptyInputDoesNotPanic(t *testing.T) {
	out := executor.AssignTurnGrouping(nil)
	if len(out) != 0 {
		t.Errorf("nil input should yield empty output; got %d turns", len(out))
	}
}

func TestParsedTurn_BackwardsCompatibleZeroValues(t *testing.T) {
	// Old transcripts serialized before the schema extension only have
	// Role/Content/Timestamp set. The new fields must be zero values
	// (empty strings, 0 ints, nil slices) so re-marshaling produces an
	// equivalent record.
	turn := executor.ParsedTurn{Role: "assistant", Content: "hello"}

	if turn.BlockKind != "" {
		t.Errorf("BlockKind should default empty for legacy data; got %q", turn.BlockKind)
	}
	if turn.TurnIndex != 0 {
		t.Errorf("TurnIndex should default 0; got %d", turn.TurnIndex)
	}
	if turn.FilesTouched != nil {
		t.Errorf("FilesTouched should default nil; got %v", turn.FilesTouched)
	}
}
