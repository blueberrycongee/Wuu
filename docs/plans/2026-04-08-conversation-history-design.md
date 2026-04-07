# Conversation History Support

## Problem

`StreamRunner.RunWithCallback` builds a fresh `[system, user]` message array on every call. No prior conversation history is sent to the model. When a user says "continue", the model has zero context about what came before and hallucinates.

## Decision

Approach B — upgrade JSONL storage format to persist full `ChatMessage` data (including tool calls/results), maintain an in-memory `[]ChatMessage` history in the TUI model, and pass it to the API on every call. Use the existing compaction logic to auto-summarize when context limits are approached.

## Design

### 1. Data Structures

**Upgraded `memoryEntry` (memory.go):**

```go
type memoryEntry struct {
    Role       string           `json:"role"`
    Content    string           `json:"content"`
    At         time.Time        `json:"at"`
    ToolCalls  []toolCallEntry  `json:"tool_calls,omitempty"`
    ToolCallID string           `json:"tool_call_id,omitempty"`
    Name       string           `json:"name,omitempty"`
}

type toolCallEntry struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    Arguments string `json:"arguments"`
}
```

Backward compatible: old JSONL files without tool fields unmarshal cleanly (zero values).

**New field in TUI `Model`:**

```go
chatHistory []providers.ChatMessage
```

Separate from `entries []transcriptEntry` which remains for UI rendering only.

### 2. Message Flow

1. User input → append `ChatMessage{Role: "user", Content: raw}` to `m.chatHistory`
2. Check `compact.ShouldCompact(m.chatHistory, maxContextTokens)` — compact if needed
3. Pass `m.chatHistory` to `runner.RunWithCallback(ctx, m.chatHistory, onEvent)`
4. `RunWithCallback` uses the history directly as its messages array (no longer builds from scratch)
5. After the turn completes, `RunWithCallback` returns the new messages produced (assistant + tool messages)
6. TUI appends returned messages to `m.chatHistory` and persists to JSONL

### 3. StreamRunner Interface Change

```go
// Before
func (r *StreamRunner) RunWithCallback(ctx context.Context, prompt string, onEvent StreamCallback) (string, error)

// After
func (r *StreamRunner) RunWithCallback(ctx context.Context, history []ChatMessage, onEvent StreamCallback) (string, []ChatMessage, error)
```

- Accepts full history instead of single prompt
- Returns new messages produced during the turn (assistant replies + tool call/result messages)
- System prompt injection moves to the caller (TUI prepends it to history)

### 4. Compaction

Reuse existing `compact.go`:
- `ShouldCompact`: triggers at 80% of `maxContextTokens`
- `Compact`: summarizes old history, keeps last 2 exchanges
- Adjustment needed: serialize tool call/result messages as readable text when building the summary prompt
- `maxContextTokens` read from config (new field, with sensible default)

### 5. Session Resume

- Load JSONL → convert `memoryEntry` to `[]ChatMessage` → populate `m.chatHistory`
- Old-format entries load with empty tool fields (graceful degradation)
- UI entries (`m.entries`) populated as before for rendering

### 6. Unchanged

- `transcriptEntry` and UI rendering logic
- `index.jsonl` format
- `/diff`, `/worktree`, and other commands
- Session creation/listing

## Reference Implementations

- **crush**: SQLite storage with structured message parts, full history sent per API call, auto-summarization when context threshold exceeded
- **codex**: In-memory `Vec<ResponseItem>` history, cloned and sent per API call, compaction when token limit approached

Both follow the same core pattern: send full history, compress when needed.
