# Conversation History Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make wuu send full conversation history (including tool calls/results) to the model on every API call, with auto-compaction when context limits are approached.

**Architecture:** Upgrade `memoryEntry` to store tool call data in JSONL. Add `chatHistory []ChatMessage` to TUI Model. Change `StreamRunner.RunWithCallback` to accept history and return new messages. Integrate existing compaction logic.

**Tech Stack:** Go, bubbletea TUI, OpenAI-compatible API

---

### Task 1: Upgrade memoryEntry and persistence (memory.go)

**Files:**
- Modify: `internal/tui/memory.go:13-17` (memoryEntry struct)
- Modify: `internal/tui/memory.go:19-66` (loadMemoryEntries)
- Modify: `internal/tui/memory.go:68-91` (appendMemoryEntry)
- Test: `internal/tui/memory_test.go`

**Step 1: Write failing test for tool call persistence**

Add to `internal/tui/memory_test.go`:

```go
func TestAppendAndLoadMemoryEntries_WithToolCalls(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	// Persist an assistant message with tool calls.
	if err := appendChatMessage(path, providers.ChatMessage{
		Role:    "assistant",
		Content: "Let me check that file.",
		ToolCalls: []providers.ToolCall{
			{ID: "call_1", Name: "read_file", Arguments: `{"path":"main.go"}`},
		},
	}); err != nil {
		t.Fatalf("append assistant with tool calls: %v", err)
	}

	// Persist a tool result.
	if err := appendChatMessage(path, providers.ChatMessage{
		Role:       "tool",
		Name:       "read_file",
		ToolCallID: "call_1",
		Content:    "package main\n",
	}); err != nil {
		t.Fatalf("append tool result: %v", err)
	}

	msgs, err := loadChatHistory(path)
	if err != nil {
		t.Fatalf("load chat history: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if len(msgs[0].ToolCalls) != 1 || msgs[0].ToolCalls[0].ID != "call_1" {
		t.Fatalf("tool call not preserved: %+v", msgs[0])
	}
	if msgs[1].ToolCallID != "call_1" || msgs[1].Name != "read_file" {
		t.Fatalf("tool result not preserved: %+v", msgs[1])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run TestAppendAndLoadMemoryEntries_WithToolCalls -v`
Expected: FAIL — `appendChatMessage` and `loadChatHistory` undefined.

**Step 3: Implement upgraded memoryEntry and new functions**

In `internal/tui/memory.go`:

1. Add `toolCallEntry` struct:
```go
type toolCallEntry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
```

2. Add fields to `memoryEntry`:
```go
type memoryEntry struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	At         time.Time        `json:"at"`
	ToolCalls  []toolCallEntry  `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	Name       string           `json:"name,omitempty"`
}
```

3. Add `appendChatMessage(path string, msg providers.ChatMessage) error` — converts `ChatMessage` to `memoryEntry` and appends to JSONL. Convert `msg.ToolCalls` ([]providers.ToolCall) to `[]toolCallEntry`.

4. Add `loadChatHistory(path string) ([]providers.ChatMessage, error)` — reads JSONL, converts each `memoryEntry` back to `providers.ChatMessage`. Convert `[]toolCallEntry` back to `[]providers.ToolCall`.

5. Keep existing `loadMemoryEntries` and `appendMemoryEntry` working (they are still used for UI transcript entries).

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/ -run TestAppendAndLoadMemoryEntries_WithToolCalls -v`
Expected: PASS

**Step 5: Write backward compatibility test**

```go
func TestLoadChatHistory_BackwardCompatible(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	// Write old-format entries (no tool fields).
	if err := appendMemoryEntry(path, transcriptEntry{Role: "USER", Content: "hello"}); err != nil {
		t.Fatalf("append old format: %v", err)
	}
	if err := appendMemoryEntry(path, transcriptEntry{Role: "ASSISTANT", Content: "hi there"}); err != nil {
		t.Fatalf("append old format: %v", err)
	}

	msgs, err := loadChatHistory(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hello" {
		t.Fatalf("unexpected msg[0]: %+v", msgs[0])
	}
}
```

**Step 6: Run all memory tests**

Run: `go test ./internal/tui/ -run TestAppendAndLoad -v`
Expected: all PASS

**Step 7: Commit**

```bash
git add internal/tui/memory.go internal/tui/memory_test.go
git commit -m "feat: upgrade memoryEntry to persist tool calls in JSONL"
```

---

### Task 2: Change StreamRunner to accept history and return new messages

**Files:**
- Modify: `internal/agent/stream_runner.go:27-33` (Run and RunWithCallback signatures)
- Test: `internal/agent/stream_runner_test.go`

**Step 1: Write failing test for history-based RunWithCallback**

Add to `internal/agent/stream_runner_test.go`:

```go
func TestStreamRunner_AcceptsHistory(t *testing.T) {
	client := &mockStreamClient{
		events: []providers.StreamEvent{
			{Type: providers.EventContentDelta, Content: "I remember"},
			{Type: providers.EventDone},
		},
	}

	runner := StreamRunner{
		Client: client,
		Model:  "test-model",
	}

	history := []providers.ChatMessage{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "My name is Alice."},
		{Role: "assistant", Content: "Hello Alice!"},
		{Role: "user", Content: "What is my name?"},
	}

	result, newMsgs, err := runner.RunWithCallback(context.Background(), history, nil)
	if err != nil {
		t.Fatalf("RunWithCallback: %v", err)
	}
	if result != "I remember" {
		t.Fatalf("unexpected result: %q", result)
	}

	// Should return the assistant message as new messages.
	if len(newMsgs) != 1 {
		t.Fatalf("expected 1 new message, got %d", len(newMsgs))
	}
	if newMsgs[0].Role != "assistant" || newMsgs[0].Content != "I remember" {
		t.Fatalf("unexpected new message: %+v", newMsgs[0])
	}

	// Verify the full history was sent to the API.
	if len(client.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(client.requests))
	}
	if len(client.requests[0].Messages) != 4 {
		t.Fatalf("expected 4 messages in request, got %d", len(client.requests[0].Messages))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run TestStreamRunner_AcceptsHistory -v`
Expected: FAIL — signature mismatch (RunWithCallback still takes string, returns 2 values).

**Step 3: Change RunWithCallback signature and implementation**

In `internal/agent/stream_runner.go`:

1. Change `RunWithCallback` signature:
```go
func (r *StreamRunner) RunWithCallback(ctx context.Context, history []providers.ChatMessage, onEvent StreamCallback) (string, []providers.ChatMessage, error)
```

2. Replace the message-building logic (lines 44-48) with:
```go
messages := make([]providers.ChatMessage, len(history))
copy(messages, history)
```

3. Remove the empty-prompt validation (the caller is responsible for history content).

4. Track `startLen := len(messages)` before the loop. After the loop, compute `newMsgs := messages[startLen:]` and return it.

5. Update `Run` to build the initial history itself (system + user) and call `RunWithCallback`:
```go
func (r *StreamRunner) Run(ctx context.Context, prompt string) (string, error) {
	var history []providers.ChatMessage
	if strings.TrimSpace(r.SystemPrompt) != "" {
		history = append(history, providers.ChatMessage{Role: "system", Content: r.SystemPrompt})
	}
	history = append(history, providers.ChatMessage{Role: "user", Content: prompt})
	result, _, err := r.RunWithCallback(ctx, history, r.OnEvent)
	return result, err
}
```

**Step 4: Fix existing tests**

Update existing tests that call `RunWithCallback` directly — they now need to pass `[]ChatMessage` and handle 3 return values. The `Run` method signature is unchanged, so tests using `Run` should still compile.

Specifically update `TestStreamRunner_ValidationErrors` — remove the blank-prompt test case (no longer validated in RunWithCallback), and adjust any direct `RunWithCallback` calls.

**Step 5: Run all stream runner tests**

Run: `go test ./internal/agent/ -v`
Expected: all PASS

**Step 6: Commit**

```bash
git add internal/agent/stream_runner.go internal/agent/stream_runner_test.go
git commit -m "feat: StreamRunner accepts history and returns new messages"
```

---

### Task 3: Add chatHistory to TUI Model and wire up sendMessage

**Files:**
- Modify: `internal/tui/model.go:57-117` (Model struct, NewModel)
- Modify: `internal/tui/model.go:176-204` (loadMemory)
- Modify: `internal/tui/model.go:610-661` (sendMessage)
- Modify: `internal/tui/model.go:236-244` (streamFinishedMsg handler)
- Modify: `internal/tui/model.go:746-763` (appendEntry)

**Step 1: Add chatHistory field to Model**

In `internal/tui/model.go`, add to the Model struct (after `entries`):

```go
chatHistory []providers.ChatMessage
```

**Step 2: Prepend system prompt to chatHistory in NewModel**

In `NewModel`, after the model is constructed but before `loadMemory()`, prepend the system prompt:

```go
if m.streamRunner != nil && strings.TrimSpace(m.streamRunner.SystemPrompt) != "" {
	m.chatHistory = append(m.chatHistory, providers.ChatMessage{
		Role:    "system",
		Content: m.streamRunner.SystemPrompt,
	})
}
```

**Step 3: Update loadMemory to also populate chatHistory**

In `loadMemory()`, after loading transcript entries, also load chat history:

```go
chatMsgs, err := loadChatHistory(m.memoryPath)
if err != nil {
	m.statusLine = fmt.Sprintf("chat history load failed: %v", err)
} else if len(chatMsgs) > 0 {
	// Prepend system prompt if not already present.
	if len(m.chatHistory) > 0 && m.chatHistory[0].Role == "system" {
		m.chatHistory = append(m.chatHistory[:1], chatMsgs...)
	} else {
		m.chatHistory = chatMsgs
	}
}
```

**Step 4: Update sendMessage to use chatHistory**

In `sendMessage()`:

1. After `m.appendEntry("user", raw)`, also append to chatHistory:
```go
m.chatHistory = append(m.chatHistory, providers.ChatMessage{Role: "user", Content: raw})
```

2. Change the goroutine to pass `m.chatHistory` instead of `raw`:
```go
history := make([]providers.ChatMessage, len(m.chatHistory))
copy(history, m.chatHistory)
go func() {
	defer close(ch)
	onEvent := func(event providers.StreamEvent) {
		select {
		case ch <- event:
		case <-ctx.Done():
		}
	}
	_, newMsgs, err := runner.RunWithCallback(ctx, history, onEvent)
	if err != nil && ctx.Err() == nil {
		select {
		case ch <- providers.StreamEvent{Type: providers.EventError, Error: err}:
		case <-ctx.Done():
		}
	}
	// Send new messages back via a custom event so TUI can update chatHistory.
	if len(newMsgs) > 0 {
		select {
		case ch <- providers.StreamEvent{Type: providers.EventDone, Content: "__history__"}:
		case <-ctx.Done():
		}
	}
}()
```

Wait — this approach is awkward because we need to get `newMsgs` back to the TUI. Better approach: store the new messages in a shared variable that the goroutine writes and the TUI reads on `streamFinishedMsg`.

Add a field to Model:
```go
pendingNewMsgs []providers.ChatMessage
```

In the goroutine:
```go
go func() {
	defer close(ch)
	onEvent := func(event providers.StreamEvent) {
		select {
		case ch <- event:
		case <-ctx.Done():
		}
	}
	result, newMsgs, err := runner.RunWithCallback(ctx, history, onEvent)
	_ = result
	// Store new messages for the TUI to pick up.
	// Safe: only written here, read after channel close.
	m.pendingNewMsgs = newMsgs
	if err != nil && ctx.Err() == nil {
		select {
		case ch <- providers.StreamEvent{Type: providers.EventError, Error: err}:
		case <-ctx.Done():
		}
	}
}()
```

Note: `m` is a value copy in bubbletea, but `pendingNewMsgs` is a slice header pointing to shared backing memory. This won't work cleanly with bubbletea's value semantics.

Better approach: use a channel or a pointer. Simplest: use a `*[]providers.ChatMessage` field, or pass via a dedicated message type.

**Revised approach — use a message type:**

Add a new message type:
```go
type historyUpdateMsg struct {
	newMessages []providers.ChatMessage
}
```

In the goroutine, after `RunWithCallback` returns, send the new messages through a separate channel or tea.Cmd. Since we already have the stream channel, we can encode it differently.

Actually, the cleanest approach: wrap the result in a `streamFinishedMsg` that carries the new messages.

```go
type streamFinishedMsg struct {
	newMessages []providers.ChatMessage
}
```

In the goroutine:
```go
go func() {
	defer func() {
		// Don't close ch here — let waitStreamEvent handle it.
	}()
	onEvent := func(event providers.StreamEvent) {
		select {
		case ch <- event:
		case <-ctx.Done():
		}
	}
	_, newMsgs, err := runner.RunWithCallback(ctx, history, onEvent)
	if err != nil && ctx.Err() == nil {
		select {
		case ch <- providers.StreamEvent{Type: providers.EventError, Error: err}:
		case <-ctx.Done():
		}
	}
	// Channel close triggers streamFinishedMsg in waitStreamEvent.
	// We need another way to pass newMsgs.
}()
```

Hmm, the channel close is what triggers `streamFinishedMsg`. We need to smuggle `newMsgs` through.

**Final approach — use a shared pointer:**

Add to Model:
```go
historyResult *historyResult // shared with goroutine
```

```go
type historyResult struct {
	mu       sync.Mutex
	messages []providers.ChatMessage
}
```

The goroutine writes to it, the `streamFinishedMsg` handler reads from it. This is safe because the goroutine finishes before `streamFinishedMsg` is processed (channel close happens first).

Actually, since the goroutine closes the channel AFTER writing, and `streamFinishedMsg` is only sent after the channel is closed, there's a happens-before relationship. So a simple shared `*[]providers.ChatMessage` without a mutex is safe.

Add to Model:
```go
pendingNewMsgs *[]providers.ChatMessage
```

In `sendMessage`:
```go
var newMsgsHolder []providers.ChatMessage
m.pendingNewMsgs = &newMsgsHolder

go func() {
	defer close(ch)
	onEvent := func(event providers.StreamEvent) {
		select {
		case ch <- event:
		case <-ctx.Done():
		}
	}
	_, newMsgs, err := runner.RunWithCallback(ctx, history, onEvent)
	newMsgsHolder = newMsgs  // safe: written before close(ch)
	if err != nil && ctx.Err() == nil {
		select {
		case ch <- providers.StreamEvent{Type: providers.EventError, Error: err}:
		case <-ctx.Done():
		}
	}
}()
```

Wait, `newMsgsHolder` is a local variable. Assigning to it in the goroutine doesn't update `*m.pendingNewMsgs`. We need:

```go
newMsgsHolder := &[]providers.ChatMessage{}
m.pendingNewMsgs = newMsgsHolder

go func() {
	defer close(ch)
	...
	_, newMsgs, err := runner.RunWithCallback(ctx, history, onEvent)
	*newMsgsHolder = newMsgs
	...
}()
```

This works. The pointer is shared, the goroutine writes through it before closing the channel, and the TUI reads it after `streamFinishedMsg`.

**Step 5: Handle streamFinishedMsg — merge new messages into chatHistory**

In the `streamFinishedMsg` handler:

```go
case streamFinishedMsg:
	m.streaming = false
	m.pendingRequest = false
	m.streamTarget = -1
	m.statusLine = "ready"
	m.cacheRenderedEntries()

	// Merge new messages from the completed turn into chatHistory.
	if m.pendingNewMsgs != nil && len(*m.pendingNewMsgs) > 0 {
		for _, msg := range *m.pendingNewMsgs {
			m.chatHistory = append(m.chatHistory, msg)
			// Persist each message to JSONL.
			_ = appendChatMessage(m.memoryPath, msg)
		}
		m.pendingNewMsgs = nil
	}

	m.refreshViewport(true)
	return m, func() tea.Msg { return queueDrainMsg{} }
```

**Step 6: Run existing tests to check nothing is broken**

Run: `go test ./internal/tui/ -v`
Expected: PASS (existing tests don't use streaming path directly)

Run: `go test ./... -count=1`
Expected: all PASS

**Step 7: Commit**

```bash
git add internal/tui/model.go
git commit -m "feat: wire chatHistory into TUI message flow"
```

---

### Task 4: Add maxContextTokens config and integrate compaction

**Files:**
- Modify: `internal/config/config.go:50-55` (AgentConfig)
- Modify: `internal/config/config.go:220-229` (applyDefaults)
- Modify: `internal/tui/model.go` (sendMessage — add compaction check)
- Modify: `internal/compact/compact.go:74-78` (include tool calls in summary)
- Test: `internal/config/config_test.go`
- Test: `internal/compact/compact_test.go`

**Step 1: Add MaxContextTokens to AgentConfig**

In `internal/config/config.go`:

```go
type AgentConfig struct {
	MaxSteps         int     `json:"max_steps"`
	MaxContextTokens int     `json:"max_context_tokens"`
	Temperature      float64 `json:"temperature"`
	SystemPrompt     string  `json:"system_prompt"`
}
```

**Step 2: Set default in applyDefaults**

```go
if cfg.Agent.MaxContextTokens == 0 {
	cfg.Agent.MaxContextTokens = 128000 // sensible default for most models
}
```

**Step 3: Write test for compaction with tool messages**

Add to `internal/compact/compact_test.go`:

```go
func TestCompact_IncludesToolCallsInSummary(t *testing.T) {
	messages := []providers.ChatMessage{
		{Role: "user", Content: "Read main.go"},
		{Role: "assistant", Content: "Sure.", ToolCalls: []providers.ToolCall{
			{ID: "c1", Name: "read_file", Arguments: `{"path":"main.go"}`},
		}},
		{Role: "tool", Name: "read_file", ToolCallID: "c1", Content: "package main"},
		{Role: "assistant", Content: "Here is main.go content."},
		{Role: "user", Content: "Now fix the bug."},
		{Role: "assistant", Content: "Fixed."},
		{Role: "user", Content: "Thanks."},
		{Role: "assistant", Content: "You're welcome."},
	}

	// The summary input should mention tool calls, not just skip them.
	// We test this indirectly: Compact should not error and should produce
	// a compacted list shorter than the original.
	client := &mockCompactClient{response: "User asked to read main.go, assistant used read_file tool, then fixed a bug."}
	result, err := Compact(context.Background(), messages, client, "test")
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if len(result) >= len(messages) {
		t.Fatalf("expected compacted result to be shorter, got %d vs %d", len(result), len(messages))
	}
	// First message should be the summary.
	if result[0].Role != "system" {
		t.Fatalf("expected system summary, got %s", result[0].Role)
	}
}
```

You'll need a `mockCompactClient` in the test file:
```go
type mockCompactClient struct {
	response string
}

func (m *mockCompactClient) Chat(_ context.Context, req providers.ChatRequest) (providers.ChatResponse, error) {
	return providers.ChatResponse{Content: m.response}, nil
}
```

**Step 4: Run test to verify it fails**

Run: `go test ./internal/compact/ -run TestCompact_IncludesToolCallsInSummary -v`
Expected: FAIL — mockCompactClient undefined (need to add it), and the summary builder doesn't include tool info.

**Step 5: Update Compact to include tool calls in summary**

In `internal/compact/compact.go`, update the summary-building loop (line 76-78):

```go
for _, msg := range toSummarize {
	summaryInput.WriteString(fmt.Sprintf("[%s]: %s\n", msg.Role, truncate(msg.Content, 500)))
	for _, tc := range msg.ToolCalls {
		summaryInput.WriteString(fmt.Sprintf("  -> tool_call: %s(%s)\n", tc.Name, truncate(tc.Arguments, 200)))
	}
	if msg.ToolCallID != "" {
		summaryInput.WriteString(fmt.Sprintf("  (result for tool call %s)\n", msg.ToolCallID))
	}
	summaryInput.WriteString("\n")
}
```

**Step 6: Run compact tests**

Run: `go test ./internal/compact/ -v`
Expected: all PASS

**Step 7: Wire compaction into sendMessage**

In `sendMessage()`, after appending the user message to `m.chatHistory` and before passing to the goroutine:

The TUI Model needs access to `maxContextTokens`. Add it as a field:
```go
maxContextTokens int
```

Set it in `NewModel` from the config (this requires passing it through `tui.Config`). Add to `tui.Config`:
```go
MaxContextTokens int
```

In `sendMessage`, before creating the goroutine:
```go
if compact.ShouldCompact(m.chatHistory, m.maxContextTokens) {
	// Need a providers.Client for compaction. Use streamRunner's client.
	if m.streamRunner != nil {
		compacted, err := compact.Compact(
			context.Background(),
			m.chatHistory,
			m.streamRunner.Client,
			m.streamRunner.Model,
		)
		if err == nil {
			m.chatHistory = compacted
			m.statusLine = "compacted history"
		}
	}
}
```

Note: `StreamRunner.Client` is `StreamClient` which embeds `Client`, so this works.

**Step 8: Run all tests**

Run: `go test ./... -count=1`
Expected: all PASS

**Step 9: Commit**

```bash
git add internal/config/config.go internal/compact/compact.go internal/compact/compact_test.go internal/tui/model.go internal/tui/app.go
git commit -m "feat: integrate compaction with maxContextTokens config"
```

---

### Task 5: Update appendEntry to also persist chat messages

**Files:**
- Modify: `internal/tui/model.go:746-763` (appendEntry)

**Step 1: Update appendEntry**

Currently `appendEntry` persists via `appendMemoryEntry` (old format). We need to also persist via `appendChatMessage` (new format) for the user message. But actually, the user message is already appended to chatHistory in `sendMessage`, and assistant/tool messages are persisted in the `streamFinishedMsg` handler.

The issue: `appendEntry` currently calls `appendMemoryEntry` which writes old-format JSONL. We should switch it to write new-format JSONL via `appendChatMessage` for user messages, and keep the old path for backward compat of the transcript entries.

Actually, the simplest approach: have `sendMessage` call `appendChatMessage` for the user message directly (instead of relying on `appendEntry`), and have the `streamFinishedMsg` handler call `appendChatMessage` for new messages. `appendEntry` continues to call `appendMemoryEntry` for the transcript — this means the JSONL will have both old-format and new-format entries, which is fine since `loadChatHistory` handles both.

Wait, that would create duplicate entries. Better: replace `appendMemoryEntry` calls in `appendEntry` with `appendChatMessage` calls. But `appendEntry` doesn't have tool call info.

**Revised approach:** 
- `appendEntry` stops calling `appendMemoryEntry` for user/assistant roles (those are now persisted via `appendChatMessage` at the appropriate points in the flow).
- For system/error entries (which are UI-only), `appendEntry` still calls `appendMemoryEntry`.
- In `sendMessage`: call `appendChatMessage(m.memoryPath, ChatMessage{Role: "user", Content: raw})` explicitly.
- In `streamFinishedMsg` handler: call `appendChatMessage` for each new message.

This avoids double-writing and ensures tool data is preserved.

Modify `appendEntry`:
```go
func (m *Model) appendEntry(role, content string) int {
	text := strings.TrimSpace(content)
	if text == "" {
		text = "(empty)"
	}
	entry := transcriptEntry{
		Role:    strings.ToUpper(role),
		Content: text,
	}
	m.entries = append(m.entries, entry)
	m.ensureSessionFile()

	// Only persist non-chat entries via old format (errors, system messages).
	// User/assistant/tool messages are persisted via appendChatMessage elsewhere.
	upperRole := strings.ToUpper(role)
	if upperRole != "USER" && upperRole != "ASSISTANT" && upperRole != "TOOL" {
		if err := appendMemoryEntry(m.memoryPath, entry); err != nil {
			m.statusLine = fmt.Sprintf("memory write failed: %v", err)
		}
	}
	return len(m.entries) - 1
}
```

**Step 2: Add explicit appendChatMessage calls**

In `sendMessage`, after appending to chatHistory:
```go
m.chatHistory = append(m.chatHistory, providers.ChatMessage{Role: "user", Content: raw})
_ = appendChatMessage(m.memoryPath, providers.ChatMessage{Role: "user", Content: raw})
```

In `streamFinishedMsg` handler (already added in Task 3):
```go
for _, msg := range *m.pendingNewMsgs {
	m.chatHistory = append(m.chatHistory, msg)
	_ = appendChatMessage(m.memoryPath, msg)
}
```

**Step 3: Run all tests**

Run: `go test ./... -count=1`
Expected: all PASS

**Step 4: Commit**

```bash
git add internal/tui/model.go
git commit -m "feat: persist chat messages with full tool call data"
```

---

### Task 6: End-to-end manual test and cleanup

**Step 1: Build and run**

Run: `go build -o wuu ./cmd/wuu && ./wuu`

**Step 2: Manual test sequence**

1. Send "Hello, my name is Alice"
2. Wait for response
3. Send "What is my name?" — model should remember "Alice"
4. Send a message that triggers a tool call (e.g., "Read the go.mod file")
5. After tool execution, send "What file did you just read?" — model should remember
6. Exit and resume the session — verify history is loaded

**Step 3: Check for any compilation issues**

Run: `go vet ./...`
Expected: no issues

**Step 4: Run full test suite**

Run: `go test ./... -count=1`
Expected: all PASS

**Step 5: Commit any final fixes**

```bash
git add -A
git commit -m "fix: address issues found in manual testing"
```
