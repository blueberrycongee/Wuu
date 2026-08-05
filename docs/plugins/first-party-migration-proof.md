# First-Party Migration Proof — Phase B

This document demonstrates that the public capability contract established in
Phases A-D is sufficient for real first-party Wuu features. It shows how three
existing features would be implemented through the public seam instead of direct
`internal/agent` modification.

## Principle

> 如果一项功能只能通过修改 Agent loop 实现，应先判断缺少的是哪一个公共能力，而不是直接增加产品专用分支。

## Example 1: Custom Tool Permission Guard

**Current implementation**: Tool permission checks are embedded in the tool execution path within `internal/agent/tool_runtime.go`.

**Public seam migration**:

```go
// Register via the capability contract as a plugin would:
// seam: agent.tool.execute.before (guard)
// priority: 100 (executes before other guards)

func GuardFilePathAccess(ctx context.Context, input ToolExecuteInput) (ToolExecuteInput, error) {
    // Check if the tool is attempting to access paths outside the workspace.
    // Reject before execution reaches the tool runtime.
    if isOutsideWorkspace(input.Arguments) {
        return input, ErrPathAccessDenied
    }
    return input, nil
}
```

**Why this works**: The `agent.tool.execute.before` seam is a guard — it short-circuits on rejection. The permission policy plugin registers at high priority and blocks unauthorized access before any other guard runs.

## Example 2: Plan Mode as a System Prompt Section

**Current implementation**: Plan mode logic is embedded in prompt assembly within `internal/prompt/`.

**Public seam migration**:

```go
// seam: agent.system_prompt.section (transform)
// key: "wuu.plan-mode"
// priority: 50

type PlanModePromptProvider struct{}

func (p *PlanModePromptProvider) SystemPromptKey() string { return "wuu.plan-mode" }
func (p *PlanModePromptProvider) SystemPromptPriority() int { return 50 }
func (p *PlanModePromptProvider) SystemPromptSection() (string, bool) {
    return `## Planning mode
When plan mode is active, you MUST create a plan using update_plan before
making any file changes. Break work into discrete steps and mark each step's
status as pending, in_progress, or completed.`, true
}
```

**Why this works**: The `SystemPromptAssembler` collects sections from all providers. Plan mode becomes a section provider registered at priority 50, inserted between the host base prompt and lower-priority plugin sections.

## Example 3: Compaction Strategy

**Current implementation**: `compact.CompactWithBudgetAndOptions` is called directly from `Runner.RunWithUsage`.

**Public seam migration**:

```go
// seam: agent.compaction (decision)
// key: "wuu.summary-compaction"
// priority: 1 (default; custom strategies use higher priority)

type SummaryCompactionProvider struct{}

func (p *SummaryCompactionProvider) CompactionKey() string { return "wuu.summary-compaction" }
func (p *SummaryCompactionProvider) CompactionPriority() int { return 1 }
func (p *SummaryCompactionProvider) Compact(ctx context.Context, model string, messages []providers.ChatMessage) ([]providers.ChatMessage, error) {
    return compact.CompactWithBudgetAndOptions(ctx, messages, client, model, budget, opts)
}
```

**Why this works**: `CompactionRegistry.Resolve()` returns the highest-priority provider. A plugin can register a custom compaction strategy at priority 100 to override the default.

## Verification

All three examples use only:
- `internal/agent` public interfaces (SystemPromptProvider, CompactionProvider, RequestTransformProvider)
- `internal/plugin` scope/registry types (Generation, Registry, PluginScope)
- Standard seam names from `internal/plugin/seam.go`

They do NOT import:
- `internal/agent` unexported types
- `internal/prompt` internals
- Private React state or internal class names
