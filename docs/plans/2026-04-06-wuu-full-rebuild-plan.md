# wuu Full Rebuild Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rebuild wuu into a production-quality Coding Agent CLI with real SSE streaming, polished TUI, and extensible skill/hook system.

**Architecture:** Streaming-first agent loop with Go channels, Bubbletea TUI with responsive layout, provider abstraction supporting OpenAI and Anthropic SSE formats. Reference: Claude Code (UX), Crush (Go patterns), Codex (streaming).

**Tech Stack:** Go 1.26, Bubbletea v1, Glamour, Lipgloss, net/http (SSE), tiktoken-go (token counting)

---

## Task 1: Extend Provider Types with Streaming Support

**Files:**
- Modify: `internal/providers/types.go`

**Step 1: Add streaming types to providers/types.go**

Add after the existing `ChatResponse` struct (line 40):

```go
// StreamEventType identifies the kind of streaming event.
type StreamEventType string

const (
	EventContentDelta  StreamEventType = "content_delta"
	EventToolUseStart  StreamEventType = "tool_use_start"
	EventToolUseDelta  StreamEventType = "tool_use_delta"
	EventToolUseEnd    StreamEventType = "tool_use_end"
	EventDone          StreamEventType = "done"
	EventError         StreamEventType = "error"
)

// TokenUsage tracks token consumption for a request.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
}

// StreamEvent is one event from a streaming response.
type StreamEvent struct {
	Type     StreamEventType
	Content  string
	ToolCall *ToolCall
	Error    error
	Usage    *TokenUsage
}

// StreamClient extends Client with streaming support.
type StreamClient interface {
	Client
	StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
}
```

**Step 2: Verify it compiles**

Run: `cd /Users/blueberrycongee/wuu && go build ./internal/providers/...`
Expected: success, no errors

**Step 3: Commit**

```bash
git add internal/providers/types.go
git commit -m "feat: add streaming types to provider interface"
```

---

## Task 2: Implement OpenAI SSE Streaming

**Files:**
- Modify: `internal/providers/openai/client.go`

**Step 1: Add SSE stream types**

Add after the existing `chatResponseMessage` struct (line 264):

```go
// SSE streaming types
type chatCompletionsChunk struct {
	Choices []chatChunkChoice `json:"choices"`
	Usage   *chunkUsage       `json:"usage,omitempty"`
}

type chatChunkChoice struct {
	Delta        chatChunkDelta `json:"delta"`
	FinishReason *string        `json:"finish_reason"`
}

type chatChunkDelta struct {
	Content   string          `json:"content,omitempty"`
	ToolCalls []toolCallDelta `json:"tool_calls,omitempty"`
}

type toolCallDelta struct {
	Index    int              `json:"index"`
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function toolFunctionDelta `json:"function,omitempty"`
}

type toolFunctionDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type chunkUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}
```

**Step 2: Implement StreamChat method**

Add the `StreamChat` method to the `Client` struct. This method:
1. Builds the same request as `Chat` but adds `"stream": true`
2. Sends HTTP request, reads SSE lines from response body
3. Parses `data: {...}` lines, handles `[DONE]`
4. Accumulates tool call deltas by index
5. Emits `StreamEvent` on a channel

```go
func (c *Client) StreamChat(ctx context.Context, req providers.ChatRequest) (<-chan providers.StreamEvent, error) {
	// Validate, build payload (same as Chat), set Stream: true
	// Create HTTP request, send it
	// Spawn goroutine to read SSE lines:
	//   - Skip empty lines and "event:" lines
	//   - Parse "data: " prefix, handle "[DONE]"
	//   - JSON decode chatCompletionsChunk
	//   - For content deltas: emit EventContentDelta
	//   - For tool call deltas: accumulate by index, emit start/delta/end
	//   - On finish: emit EventDone with usage
	// Return channel
}
```

Reference: `thirdparty/codex/codex-rs/codex-api/src/sse/responses.rs` for SSE parsing patterns.
Reference: `thirdparty/claude-code-sourcemap/src/cli/transports/SSETransport.ts:parseSSEFrames` for frame parsing.

**Step 3: Verify it compiles**

Run: `cd /Users/blueberrycongee/wuu && go build ./internal/providers/openai/...`

**Step 4: Commit**

```bash
git add internal/providers/openai/client.go
git commit -m "feat: implement OpenAI SSE streaming"
```

---

## Task 3: Implement Anthropic SSE Streaming

**Files:**
- Modify: `internal/providers/anthropic/client.go`

**Step 1: Add SSE event types**

Add Anthropic-specific SSE event structs:

```go
type sseEvent struct {
	Event string // from "event:" line
	Data  string // from "data:" line
}

type messageStartData struct {
	Message struct {
		Usage struct {
			InputTokens int `json:"input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

type contentBlockStartData struct {
	Index        int `json:"index"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	} `json:"content_block"`
}

type contentBlockDeltaData struct {
	Index int `json:"index"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text,omitempty"`
		PartialJSON string `json:"partial_json,omitempty"`
	} `json:"delta"`
}

type messageDeltaData struct {
	Usage struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}
```

**Step 2: Implement StreamChat method**

Anthropic SSE format uses `event:` + `data:` lines:
- `event: message_start` → extract input token count
- `event: content_block_start` → if type=tool_use, emit EventToolUseStart
- `event: content_block_delta` → if type=text_delta, emit EventContentDelta; if type=input_json_delta, emit EventToolUseDelta
- `event: content_block_stop` → if was tool_use block, emit EventToolUseEnd
- `event: message_delta` → extract output token count
- `event: message_stop` → emit EventDone

Track active content blocks by index to know which are text vs tool_use.

Reference: `thirdparty/crush/internal/server/proto.go:189-236` for Go SSE handler patterns.

**Step 3: Verify it compiles**

Run: `cd /Users/blueberrycongee/wuu && go build ./internal/providers/anthropic/...`

**Step 4: Commit**

```bash
git add internal/providers/anthropic/client.go
git commit -m "feat: implement Anthropic SSE streaming"
```

---

## Task 4: Update Provider Factory for StreamClient

**Files:**
- Modify: `internal/providerfactory/factory.go`

**Step 1: Add BuildStreamClient function**

```go
// BuildStreamClient constructs a streaming-capable provider client.
func BuildStreamClient(provider config.ProviderConfig) (providers.StreamClient, error) {
	typeName := normalizeType(provider.Type)
	apiKey, err := resolveAPIKey(provider)
	if err != nil {
		return nil, err
	}

	switch typeName {
	case "openai", "openai-compatible", "codex":
		return openai.New(openai.ClientConfig{
			BaseURL: provider.BaseURL,
			APIKey:  apiKey,
			Headers: provider.Headers,
		})
	case "anthropic", "claude", "anthropic-official":
		return anthropic.New(anthropic.ClientConfig{
			BaseURL: provider.BaseURL,
			APIKey:  apiKey,
			Headers: provider.Headers,
		})
	default:
		return nil, fmt.Errorf("unsupported provider type %q", provider.Type)
	}
}
```

Both `openai.Client` and `anthropic.Client` now implement `StreamClient` (they have both `Chat` and `StreamChat`).

**Step 2: Verify it compiles**

Run: `cd /Users/blueberrycongee/wuu && go build ./internal/providerfactory/...`

**Step 3: Commit**

```bash
git add internal/providerfactory/factory.go
git commit -m "feat: add BuildStreamClient to provider factory"
```

---

## Task 5: Implement Streaming Agent Runner

**Files:**
- Create: `internal/agent/stream_runner.go`

**Step 1: Create StreamRunner**

New file that implements the streaming-first agent loop:

```go
package agent

// StreamCallback receives streaming events for TUI rendering.
type StreamCallback func(event providers.StreamEvent)

// StreamRunner manages one multi-step coding turn with streaming.
type StreamRunner struct {
	Client       providers.StreamClient
	Tools        ToolExecutor
	Model        string
	SystemPrompt string
	MaxSteps     int
	Temperature  float64
	OnEvent      StreamCallback
}

// Run executes one prompt with streaming tool-use loop.
func (r *StreamRunner) Run(ctx context.Context, prompt string) (string, error) {
	// Build messages array (system + user)
	// Loop up to MaxSteps:
	//   1. Call StreamChat, get event channel
	//   2. Consume events:
	//      - content_delta → accumulate text, call OnEvent
	//      - tool_use_start → prepare tool call accumulator
	//      - tool_use_delta → accumulate tool arguments
	//      - tool_use_end → execute tool immediately, call OnEvent
	//      - done → break inner loop
	//      - error → return error
	//   3. If no tool calls, return accumulated text
	//   4. If tool calls, append results to messages, continue loop
}
```

Key difference from existing `Runner`: events flow to TUI in real-time via `OnEvent`, and tools execute as they arrive in the stream (not after full response).

**Step 2: Verify it compiles**

Run: `cd /Users/blueberrycongee/wuu && go build ./internal/agent/...`

**Step 3: Commit**

```bash
git add internal/agent/stream_runner.go
git commit -m "feat: add streaming agent runner"
```

---

## Task 6: Wire Streaming into TUI Model

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/app.go` (if exists, or the Run function)
- Modify: `cmd/wuu/main.go`

**Step 1: Add streaming event message type**

In `model.go`, add:

```go
type streamEventMsg struct {
	event providers.StreamEvent
}
```

**Step 2: Replace responseMsg flow with streaming flow**

Change `submit()` to use `StreamRunner` instead of the blocking `runPrompt`. The TUI receives `streamEventMsg` via Bubbletea commands, updating the display per-token.

**Step 3: Update Config and main.go**

Change `tui.Config.RunPrompt` to accept a `StreamCallback` or change to pass the `StreamRunner` directly. Update `cmd/wuu/main.go:runTUI` to use `BuildStreamClient` and `StreamRunner`.

**Step 4: Verify it compiles and runs**

Run: `cd /Users/blueberrycongee/wuu && go build ./cmd/wuu/...`

**Step 5: Commit**

```bash
git add internal/tui/model.go internal/tui/app.go cmd/wuu/main.go
git commit -m "feat: wire streaming into TUI for real-time token rendering"
```

---

## Task 7: TUI Layout Rewrite

**Files:**
- Create: `internal/tui/layout.go`
- Modify: `internal/tui/model.go`

**Step 1: Create layout.go with responsive rectangle system**

Reference: `thirdparty/crush/internal/ui/model/ui.go:2462-2601` for rectangle-based layout.

```go
package tui

type layoutRect struct {
	X, Y, Width, Height int
}

type layout struct {
	header  layoutRect
	chat    layoutRect
	input   layoutRect
	footer  layoutRect
	compact bool
}

func computeLayout(width, height int, inputLines int) layout {
	// Header: 1 line at top
	// Footer: 1 line at bottom
	// Input: inputLines + 2 (border) at bottom above footer
	// Chat: everything remaining
	// Compact mode: width < 80, no borders
}
```

**Step 2: Refactor Model.relayout() to use computeLayout**

Replace the existing `relayout()` method with one that uses the new layout system.

**Step 3: Implement dynamic input height**

Track actual line count in textarea, clamp to [3, 15]:

```go
func (m *Model) inputLineCount() int {
	lines := strings.Count(m.input.Value(), "\n") + 1
	if lines < 3 { return 3 }
	if lines > 15 { return 15 }
	return lines
}
```

**Step 4: Verify it compiles**

Run: `cd /Users/blueberrycongee/wuu && go build ./internal/tui/...`

**Step 5: Commit**

```bash
git add internal/tui/layout.go internal/tui/model.go
git commit -m "feat: responsive TUI layout with dynamic input height"
```

---

## Task 8: Polymorphic Message Items

**Files:**
- Create: `internal/tui/chat.go` (rewrite from scratch)
- Modify: `internal/tui/model.go`

**Step 1: Define message item types**

```go
package tui

type messageItemType int

const (
	itemUser messageItemType = iota
	itemAssistant
	itemTool
	itemSystem
)

type messageItem struct {
	Type      messageItemType
	Content   string
	ToolName  string // for tool items
	ToolID    string
	Collapsed bool  // for tool items, collapsible
	Streaming bool  // currently receiving content
}
```

**Step 2: Implement per-type rendering**

Each type renders differently:
- User: `> ` prefix, dimmed style
- Assistant: Glamour markdown rendering
- Tool: collapsible block with tool name header, output body
- System: italic, dimmed

Reference: `thirdparty/crush/internal/ui/chat/messages.go` for polymorphic message items.

**Step 3: Replace transcriptEntry with messageItem in Model**

Update all references from `entries []transcriptEntry` to `items []messageItem`.

**Step 4: Verify it compiles**

Run: `cd /Users/blueberrycongee/wuu && go build ./internal/tui/...`

**Step 5: Commit**

```bash
git add internal/tui/chat.go internal/tui/model.go
git commit -m "feat: polymorphic message items with per-type rendering"
```

---

## Task 9: Enhanced Markdown Rendering with Streaming

**Files:**
- Create: `internal/tui/markdown.go` (rewrite)
- Modify: `internal/tui/model.go`

**Step 1: Create markdown renderer with caching**

```go
package tui

type markdownRenderer struct {
	renderer *glamour.TermRenderer
	width    int
}

func newMarkdownRenderer(width int) (*markdownRenderer, error) {
	// Create Glamour renderer with AutoStyle and word wrap
}

func (r *markdownRenderer) Render(content string) (string, error) {
	// Render markdown, handle empty content
}

func (r *markdownRenderer) UpdateWidth(width int) error {
	// Recreate renderer if width changed
}
```

**Step 2: Implement streaming-aware rendering**

During streaming, render partial markdown. After stream completes, do final render.
Key: Glamour can handle incomplete markdown gracefully — unclosed code blocks just render as-is.

**Step 3: Verify it compiles**

Run: `cd /Users/blueberrycongee/wuu && go build ./internal/tui/...`

**Step 4: Commit**

```bash
git add internal/tui/markdown.go internal/tui/model.go
git commit -m "feat: cached markdown renderer with streaming support"
```

---

## Task 10: Mouse Support and Interaction Polish

**Files:**
- Modify: `internal/tui/model.go`

**Step 1: Enable mouse support in Bubbletea**

In the `Run` function (app.go), add `tea.WithMouseAllMotion()` option.

**Step 2: Handle mouse events in Update**

Expand the existing `tea.MouseMsg` handler:
- Click in input area → focus input
- Click in chat area → focus chat (for scrolling)
- Scroll wheel → scroll viewport
- Click on "Jump to bottom" → go to bottom
- Click on tool item header → toggle collapse

Reference: `thirdparty/crush/internal/ui/model/ui.go:674-797` for mouse event handling.

**Step 3: Add focus state management**

```go
type focusArea int
const (
	focusInput focusArea = iota
	focusChat
)
```

Route keyboard events based on focus state.

**Step 4: Verify it compiles**

Run: `cd /Users/blueberrycongee/wuu && go build ./internal/tui/...`

**Step 5: Commit**

```bash
git add internal/tui/model.go
git commit -m "feat: mouse support with focus management"
```

---

## Task 11: Slash Command System Rewrite

**Files:**
- Rewrite: `internal/tui/commands.go`

**Step 1: Define command registry**

```go
package tui

type commandType string
const (
	cmdLocal  commandType = "local"
	cmdPrompt commandType = "prompt"
)

type command struct {
	Name        string
	Aliases     []string
	Description string
	ArgHint     string
	InlineArgs  bool
	DuringTask  bool
	Hidden      bool
	Type        commandType
	Execute     func(args string, m *Model) string
}

func builtinCommands() []command {
	return []command{
		{Name: "help", Description: "Show available commands", Execute: cmdHelp},
		{Name: "clear", Description: "Clear screen", Execute: cmdClear},
		{Name: "status", Description: "Show session config and token usage", Execute: cmdStatus},
		{Name: "compact", Description: "Compress conversation context", Execute: cmdCompact},
		{Name: "model", Description: "Switch model/provider", InlineArgs: true, Execute: cmdModel},
		{Name: "resume", Description: "Resume previous session", Execute: cmdResume},
		{Name: "fork", Description: "Fork current session", Execute: cmdFork},
		{Name: "new", Description: "Start new conversation", Execute: cmdNew},
		{Name: "diff", Description: "Show git diff", Execute: cmdDiff},
		{Name: "copy", Description: "Copy last output", Execute: cmdCopy},
		{Name: "worktree", Description: "Create/switch git worktree", InlineArgs: true, Execute: cmdWorktree},
		{Name: "skills", Description: "List available skills", Execute: cmdSkills},
		{Name: "insight", Description: "Session stats and diagnostics", Execute: cmdInsight},
		{Name: "exit", Aliases: []string{"quit"}, Description: "Exit wuu", Execute: cmdExit},
	}
}
```

Reference: `thirdparty/codex/codex-rs/tui/src/slash_command.rs` for command metadata patterns.
Reference: `thirdparty/claude-code-sourcemap/src/commands.ts` for command type system.

**Step 2: Implement each command handler**

Implement `cmdHelp`, `cmdClear`, `cmdStatus`, `cmdCompact`, `cmdModel`, `cmdResume`, `cmdFork`, `cmdNew`, `cmdDiff`, `cmdCopy`, `cmdWorktree`, `cmdSkills`, `cmdInsight`, `cmdExit`.

For `/diff`: run `git diff` and `git diff --cached` in workspace.
For `/copy`: use `github.com/atotto/clipboard` (already in go.sum as indirect dep).
For `/compact`: trigger context compaction (Task 14).
For `/model`: parse inline arg, update model in config.

**Step 3: Update handleSlash to use registry**

Replace the switch statement with registry lookup.

**Step 4: Verify it compiles**

Run: `cd /Users/blueberrycongee/wuu && go build ./internal/tui/...`

**Step 5: Commit**

```bash
git add internal/tui/commands.go
git commit -m "feat: slash command registry with 14 built-in commands"
```

---

## Task 12: Skills System

**Files:**
- Create: `internal/skills/skills.go`
- Create: `internal/skills/loader.go`

**Step 1: Define skill types**

```go
package skills

type Skill struct {
	Name        string
	Description string
	Content     string
	Source      string // "project" or "user"
	Path        string
}
```

**Step 2: Implement skill discovery**

```go
func Discover(projectDir, userDir string) ([]Skill, error) {
	// Walk projectDir (.wuu/skills/) and userDir (~/.config/wuu/skills/)
	// Parse YAML frontmatter from .md files
	// User-level overrides project-level same-name skills
	// Return deduplicated list
}
```

Reference: `thirdparty/crush/internal/skills/skills.go` for recursive walk + frontmatter parsing.
Reference: `thirdparty/claude-code-sourcemap/src/skills/loadSkillsDir.ts` for deduplication.

**Step 3: Implement YAML frontmatter parser**

Parse `---\n...\n---` header from markdown files. Extract `name` and `description` fields.

**Step 4: Verify it compiles**

Run: `cd /Users/blueberrycongee/wuu && go build ./internal/skills/...`

**Step 5: Commit**

```bash
git add internal/skills/
git commit -m "feat: skill discovery with YAML frontmatter parsing"
```

---

## Task 13: Hooks System

**Files:**
- Create: `internal/hooks/hooks.go`
- Modify: `internal/config/config.go`

**Step 1: Add hooks config to Config struct**

In `config.go`, add:

```go
type HookEntry struct {
	Tool    string `json:"tool,omitempty"`
	Command string `json:"command"`
}

type HooksConfig struct {
	PrePrompt  []HookEntry `json:"pre_prompt,omitempty"`
	PostPrompt []HookEntry `json:"post_prompt,omitempty"`
	PreTool    []HookEntry `json:"pre_tool,omitempty"`
	PostTool   []HookEntry `json:"post_tool,omitempty"`
}
```

Add `Hooks HooksConfig` field to `Config` struct.

**Step 2: Implement hook executor**

```go
package hooks

func Run(ctx context.Context, entries []config.HookEntry, toolName string, env map[string]string) error {
	// Filter entries by tool name (or "*" for all)
	// Execute each matching hook command
	// Exit code 0 = success
	// Exit code 2 = block operation (return special error)
	// Other = show error
}
```

Reference: Claude Code's hook system in `src/utils/hooks/`.

**Step 3: Verify it compiles**

Run: `cd /Users/blueberrycongee/wuu && go build ./internal/hooks/...`

**Step 4: Commit**

```bash
git add internal/hooks/ internal/config/config.go
git commit -m "feat: lifecycle hooks system with exit code semantics"
```

---

## Task 14: Context Management and Token Counting

**Files:**
- Create: `internal/compact/compact.go`
- Create: `internal/compact/tokens.go`

**Step 1: Implement token estimator**

```go
package compact

func EstimateTokens(text string) int {
	// Simple heuristic: ~4 chars per token for English, ~2 chars for CJK
	// Good enough for display; API returns precise counts
}
```

**Step 2: Implement auto-compact**

```go
func Compact(messages []providers.ChatMessage, client providers.StreamClient, model string, maxTokens int) ([]providers.ChatMessage, error) {
	// Find compaction boundary (oldest assistant message)
	// Build summary prompt from messages before boundary
	// Call client.Chat to generate summary
	// Replace old messages with single summary message
	// Preserve tool call trajectory integrity
}
```

Reference: `thirdparty/claude-code-sourcemap/src/services/compact/` for compaction strategies.

**Step 3: Wire into agent runner**

In `StreamRunner.Run`, check token count after each turn. If > 80% of context window, trigger auto-compact.

**Step 4: Wire /compact command**

In the slash command handler for `/compact`, trigger manual compaction.

**Step 5: Verify it compiles**

Run: `cd /Users/blueberrycongee/wuu && go build ./internal/compact/...`

**Step 6: Commit**

```bash
git add internal/compact/
git commit -m "feat: context compaction with token counting"
```

---

## Task 15: Error Recovery and Retry Logic

**Files:**
- Create: `internal/providers/retry.go`

**Step 1: Implement retry wrapper**

```go
package providers

type RetryConfig struct {
	MaxRetries    int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
}

func WithRetry(ctx context.Context, cfg RetryConfig, fn func() error) error {
	// Exponential backoff with jitter
	// Parse Retry-After header from 429 responses
	// Classify errors:
	//   429, 529 → retryable with backoff
	//   500, 502, 503 → retryable up to MaxRetries
	//   401 → not retryable (auth error)
	//   context window exceeded → not retryable (caller handles compact)
	//   network error → retryable
}
```

Reference: `thirdparty/claude-code-sourcemap/src/services/api/withRetry.ts` for retry patterns.
Reference: `thirdparty/codex/codex-rs/codex-api/src/sse/responses.rs` for error classification.

**Step 2: Integrate retry into StreamChat calls**

Wrap `StreamChat` calls in the agent runner with retry logic.

**Step 3: Verify it compiles**

Run: `cd /Users/blueberrycongee/wuu && go build ./internal/providers/...`

**Step 4: Commit**

```bash
git add internal/providers/retry.go
git commit -m "feat: exponential backoff retry with error classification"
```

---

## Task 16: Header and Footer Polish

**Files:**
- Modify: `internal/tui/model.go` (View method)

**Step 1: Redesign header**

```
wuu · anthropic/claude-3-5-sonnet · 1.2k/200k tokens
```

Show: product name, provider/model, token usage (from StreamEvent.Usage).

**Step 2: Redesign footer**

```
● streaming · 1.2s · 15:42:30
```

States: `ready`, `streaming`, `executing tool: <name>`, `compacting...`

Show elapsed time during streaming. Show clock always.

**Step 3: Verify it compiles**

Run: `cd /Users/blueberrycongee/wuu && go build ./internal/tui/...`

**Step 4: Commit**

```bash
git add internal/tui/model.go
git commit -m "feat: polished header with token usage and footer with status"
```

---

## Task 17: Integration Testing

**Files:**
- Create: `internal/providers/openai/client_test.go` (streaming test)
- Create: `internal/providers/anthropic/client_test.go` (streaming test)
- Create: `internal/agent/stream_runner_test.go`

**Step 1: Write SSE mock server for OpenAI format**

```go
func TestStreamChat_OpenAI(t *testing.T) {
	// Start httptest.Server that returns SSE stream
	// Verify events arrive in correct order on channel
	// Verify tool calls are accumulated correctly
}
```

**Step 2: Write SSE mock server for Anthropic format**

```go
func TestStreamChat_Anthropic(t *testing.T) {
	// Start httptest.Server that returns Anthropic SSE stream
	// Verify content_block_delta events map correctly
	// Verify tool_use blocks emit start/delta/end
}
```

**Step 3: Write StreamRunner integration test**

```go
func TestStreamRunner_ToolLoop(t *testing.T) {
	// Mock StreamClient that returns tool calls then final answer
	// Mock ToolExecutor
	// Verify tools execute during stream, not after
	// Verify OnEvent callback receives all events
}
```

**Step 4: Run tests**

Run: `cd /Users/blueberrycongee/wuu && go test ./internal/... -v`

**Step 5: Commit**

```bash
git add internal/providers/openai/client_test.go internal/providers/anthropic/client_test.go internal/agent/stream_runner_test.go
git commit -m "test: SSE streaming and agent runner integration tests"
```

---

## Task 18: Final Polish and Performance

**Files:**
- Various files across the codebase

**Step 1: Startup optimization**

Ensure heavy operations (skill discovery, memory loading) happen lazily or in parallel.

**Step 2: Render performance**

- Cache rendered markdown (invalidate on width change)
- Throttle viewport updates during fast streaming (max 30fps)

**Step 3: Visual alignment**

- Consistent spacing and borders
- Proper handling of wide characters (CJK)
- Clean exit (restore terminal state)

**Step 4: Run full test suite**

Run: `cd /Users/blueberrycongee/wuu && go test ./... -v`

**Step 5: Manual smoke test**

Run: `cd /Users/blueberrycongee/wuu && go run ./cmd/wuu tui`
Verify: real-time streaming, tool execution, slash commands, mouse support.

**Step 6: Commit**

```bash
git add -A
git commit -m "feat: final polish and performance optimization"
```
```
