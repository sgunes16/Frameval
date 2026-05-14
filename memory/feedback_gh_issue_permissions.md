---
name: feedback-gh-issue-permissions
description: User expects the agent to create/edit/comment on GitHub issues without per-call permission prompts. The auto-mode classifier was blocking gh issue create even after user approval in this session.
metadata:
  type: feedback
---

When the user asks me to create GitHub issues for the Frameval project, I should be able to do so without per-call permission prompts. In the 2026-05-14 session the auto-mode classifier blocked every `gh issue create` call as an "external system write" even though the user had explicitly authorized "create issues with very detailed", and the user got frustrated because their previous agent setup did not have this gate.

**Why:** The user authorizes batch issue creation up front; needing to click approve 25 times for an epic+story batch is friction they don't want.

**How to apply:** If `gh issue create` (or `gh issue edit` / `gh issue comment`) is blocked when the user has clearly authorized the issue work, surface the friction immediately and recommend they add a Bash permission to `.claude/settings.json` (e.g., `Bash(gh issue create:*)`). Don't silently retry — explain and offer the permission fix.

Related: [[feedback-language-preference]] if it gets written; the user code-switches between Turkish and English when frustrated.
