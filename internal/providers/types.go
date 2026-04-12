package providers

import "context"

// ToolDefinition describes a callable tool exposed to the model.
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// ToolCall is a model requested tool execution.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// InputImage carries one user-provided image in base64 form.
type InputImage struct {
	MediaType string
	Data      string
}

// ChatMessage is a generic multi-provider chat message.
type ChatMessage struct {
	Role       string
	Name       string
	Content    string
	// ReasoningContent stores provider-emitted hidden reasoning that
	// must be replayed in follow-up assistant tool-call messages for
	// providers like Kimi when thinking mode is enabled.
	ReasoningContent string
	Images     []InputImage
	ToolCallID string
	ToolCalls  []ToolCall
}

// ChatRequest is the normalized request payload for providers.
type ChatRequest struct {
	Model       string
	Messages    []ChatMessage
	Tools       []ToolDefinition
	Temperature float64
}

// ChatResponse is the normalized response from providers.
type ChatResponse struct {
	Content          string
	ReasoningContent string
	ToolCalls        []ToolCall
	Usage            *TokenUsage // optional; nil when the provider didn't return usage
	// StopReason is the raw provider stop signal, normalized to lowercase.
	// Common values: "stop" / "end_turn" (natural finish), "length" /
	// "max_tokens" (output truncation), "tool_calls" / "tool_use".
	StopReason string
	// Truncated is true when the model hit its output token cap mid-response.
	// Callers (e.g. agent.Runner) can use this to issue a "continue" prompt.
	Truncated bool
}

// Client sends chat requests to an LLM provider.
type Client interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

// StreamEventType classifies events emitted during streaming.
type StreamEventType string

const (
	EventContentDelta  StreamEventType = "content_delta"
	EventThinkingDelta StreamEventType = "thinking_delta"
	EventThinkingDone  StreamEventType = "thinking_done"
	EventToolUseStart  StreamEventType = "tool_use_start"
	EventToolUseDelta  StreamEventType = "tool_use_delta"
	EventToolUseEnd    StreamEventType = "tool_use_end"
	EventReconnect     StreamEventType = "reconnect"
	EventCompact       StreamEventType = "compact"
	EventDone          StreamEventType = "done"
	EventError         StreamEventType = "error"
)

// TokenUsage reports token consumption for a single API call. Cache
// fields are populated when the provider supports prompt caching:
// Anthropic returns them on every messages response; OpenAI returns
// `cached_tokens` under prompt_tokens_details on supporting models.
//
// IMPORTANT: cached tokens still occupy the model's context window —
// they are read out of cache and packed into the prompt, the only
// difference is the per-token price. Auto-compact's fill-rate
// calculation must include them, otherwise providers using prompt
// caching look like they're using almost no context and the trigger
// fires far too late.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
	// CacheCreationTokens are the tokens that were written into the
	// provider's prompt cache as a side effect of this call (Anthropic
	// only; OpenAI doesn't expose this separately).
	CacheCreationTokens int
	// CacheReadTokens are the tokens served out of the prompt cache
	// for this call. Reported by both Anthropic
	// (cache_read_input_tokens) and OpenAI
	// (prompt_tokens_details.cached_tokens) on models that support
	// prompt caching.
	CacheReadTokens int
}

// TotalContextTokens returns the number of tokens this call actually
// consumed against the model's context window. Equals InputTokens +
// CacheReadTokens + OutputTokens. CacheCreationTokens are NOT
// included because the cache_creation count reported by Anthropic is
// already a subset of InputTokens — adding it would double-count.
func (u TokenUsage) TotalContextTokens() int {
	if u == (TokenUsage{}) {
		return 0
	}
	return u.InputTokens + u.CacheReadTokens + u.OutputTokens
}

// StreamEvent is a single event from a streaming chat response.
type StreamEvent struct {
	Type       StreamEventType
	Content    string
	ToolCall   *ToolCall
	ToolResult string
	Error      error
	Usage      *TokenUsage
	// StopReason / Truncated are populated on the terminal EventDone
	// when the provider reports them. They mirror the same fields on
	// ChatResponse so streaming callers can drive truncation-recovery
	// the same way the non-stream Runner does.
	StopReason string
	Truncated  bool
}

// StreamClient extends Client with streaming support.
type StreamClient interface {
	Client
	StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
}
