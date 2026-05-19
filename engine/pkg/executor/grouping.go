package executor

// AssignTurnGrouping walks the input transcript and stamps TurnIndex and
// ParentTurnIndex on each block according to the Inspector-V2 grouping
// rules:
//
//   - TurnIndex: 0-based monotonic counter across the whole slice.
//   - tool_use + matching tool_result (by ToolUseID): share a ParentTurnIndex.
//   - thinking block immediately before a tool_use: joins that tool group.
//   - text block immediately after a tool_result: joins the preceding group.
//   - Any other transition starts a new ParentTurnIndex.
//
// Returns a fresh slice — the input is not mutated. Pass turns in
// chronological order; the function is a single forward pass with O(n)
// work and one helper-map for tool_use_id → parent lookups.
//
// Note on orphans: a tool_result with a ToolUseID that didn't appear in
// any prior tool_use gets its own ParentTurnIndex rather than silently
// joining whatever came before. The Inspector then renders it with a
// warning glyph.
func AssignTurnGrouping(in []ParsedTurn) []ParsedTurn {
	if len(in) == 0 {
		return nil
	}

	out := make([]ParsedTurn, len(in))
	copy(out, in)

	// toolUseParent maps a tool_use_id to the ParentTurnIndex it was
	// stamped with. The matching tool_result inherits that parent.
	toolUseParent := make(map[string]int, len(in)/2)

	currentParent := -1 // -1 sentinel: "no parent yet"
	for i := range out {
		out[i].TurnIndex = i

		switch out[i].BlockKind {
		case BlockKindThinking:
			// A bare thinking block opens a new decision. If a tool_use
			// follows, it will join us via the lookahead branch below.
			currentParent = i
			out[i].ParentTurnIndex = currentParent

		case BlockKindToolUse:
			// If the previous block was a thinking that started a fresh
			// group, reuse its parent. Otherwise, this tool_use opens a
			// new decision.
			if i > 0 && out[i-1].BlockKind == BlockKindThinking && out[i-1].ParentTurnIndex == currentParent {
				out[i].ParentTurnIndex = currentParent
			} else {
				currentParent = i
				out[i].ParentTurnIndex = currentParent
			}
			if out[i].ToolUseID != "" {
				toolUseParent[out[i].ToolUseID] = out[i].ParentTurnIndex
			}

		case BlockKindToolResult:
			// Match by ToolUseID. An orphan (no matching tool_use) gets
			// its own parent rather than absorbing whatever came before.
			if parent, ok := toolUseParent[out[i].ToolUseID]; ok && out[i].ToolUseID != "" {
				out[i].ParentTurnIndex = parent
				currentParent = parent
			} else {
				currentParent = i
				out[i].ParentTurnIndex = currentParent
			}

		case BlockKindText:
			// Trailing prose attached to a tool decision: keep the
			// current parent. Standalone text opens a new group.
			if i > 0 && out[i-1].BlockKind == BlockKindToolResult && out[i-1].ParentTurnIndex == currentParent {
				out[i].ParentTurnIndex = currentParent
			} else {
				currentParent = i
				out[i].ParentTurnIndex = currentParent
			}

		default:
			// system / unclassified / future block kinds: every block
			// is its own group.
			currentParent = i
			out[i].ParentTurnIndex = currentParent
		}
	}

	return out
}
