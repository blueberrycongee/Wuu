package memdir

import (
	"fmt"
	"strings"
)

// Teaching text builders. Wording follows Claude Code's buildMemoryLines
// (thirdparty/claude-code-sourcemap/src/memdir/memdir.ts), condensed per the
// memory-redesign contract §5: four memory types, the two-step save, the
// What-NOT-to-save gate, and the directory-already-exists guidance.

// dirExistsGuidance is shipped because models otherwise burn turns on
// `ls`/`mkdir -p` before writing; the harness guarantees the directory
// exists via EnsureDir.
const dirExistsGuidance = "This directory already exists — write to it directly with the file tools (do not run mkdir or check for its existence)."

var typesSection = []string{
	"## Types of memory",
	"Each memory file declares one `type` in its frontmatter:",
	"- `user` — who the user is: role, goals, preferences, knowledge. Written to make future collaboration better, never as a judgement of the user.",
	"- `feedback` — guidance the user gave you about how to work: corrections AND confirmations. Include *why* so you can judge edge cases later.",
	"- `reference` — pointers to where information lives in external systems (issue trackers, dashboards, channels).",
	"- `lesson` — a lesson learned from your own work that will matter again. Include *why* and how to apply it.",
}

var howToSaveSection = []string{
	"## How to save a memory",
	"Saving a memory is a two-step process:",
	"**Step 1** — write the memory to its own topic file (e.g. `user_role.md`, `feedback_testing.md`) with this frontmatter:",
	"```markdown",
	"---",
	"name: <kebab-case identifier>",
	"description: <one line, specific — used to decide relevance in future conversations>",
	"type: user | feedback | reference | lesson",
	"---",
	"",
	"<memory content — for feedback/lesson types: the rule or fact, then **Why:** and **How to apply:** lines>",
	"```",
	fmt.Sprintf("**Step 2** — add a pointer line to `%s`: `- [Title](file.md) — one-line hook` (one line, under ~150 characters).", EntrypointName),
	fmt.Sprintf("`%s` is an index, not a memory — never write memory content directly into it. Only the index is loaded into your context (truncated past %d lines / %s). Organize memory by topic, not chronologically; update or remove memories that turn out wrong or stale; check for an existing file to update before writing a new one.", EntrypointName, MaxIndexLines, formatByteSize(MaxIndexBytes)),
}

var whatNotToSaveSection = []string{
	"## What NOT to save",
	"- Anything derivable from the current repo or workspace: code patterns, architecture, file paths, git history.",
	"- Task progress, PR numbers, commit SHAs, temporary TODOs, or facts likely to go stale within a week.",
	"- Raw transcripts or activity logs; anything already covered by AGENTS.md files.",
	"These exclusions apply even when the user explicitly asks you to save. If they ask you to save a task log or activity summary, ask what was *surprising* or *non-obvious* about it — that is the part worth keeping.",
}

// IdentityTeaching returns the notebook teaching block for a collaboration
// named agent, which reads and writes its durable identity notebook at dir.
func IdentityTeaching(dir string) string {
	lines := []string{
		"# Identity notebook",
		fmt.Sprintf("You have a persistent, file-based memory directory at `%s` — your identity across days and tasks. %s", dir, dirExistsGuidance),
		"Build it up over time so future sessions know who the user is, how they like to collaborate, what to avoid or repeat, and the context behind their work. If the user explicitly asks you to remember something, save it immediately as whichever type fits best; if they ask you to forget something, find and remove the relevant topic file AND its index line.",
	}
	lines = append(lines, typesSection...)
	lines = append(lines, howToSaveSection...)
	lines = append(lines, whatNotToSaveSection...)
	return strings.Join(lines, "\n")
}
