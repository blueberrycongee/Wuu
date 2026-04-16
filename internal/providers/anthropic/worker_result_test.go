package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/providers"
)

// TestWorkerResultMessageStructure verifies that the message sequence
// after worker completion produces valid role alternation with no
// mixed tool_result+text content blocks in any user message.
//
// This is the root cause test for the deterministic reconnect loop
// on proxies after worker completion.
func TestWorkerResultMessageStructure(t *testing.T) {
	// Simulate the EXACT message sequence after worker completion:
	//
	// 1. user: original prompt
	// 2. assistant: spawns a worker (tool_use)
	// 3. tool: spawn result (maps to user+tool_result)
	// 4. assistant: "" (empty — model stopped after tool result)
	// 5. user: <worker-result>... (injected on completion)
	// 6. user: env context (BeforeStep injection)
	history := []providers.ChatMessage{
		{Role: "system", Content: "You are a coding agent."},
		{Role: "user", Content: "Run git pull"},
		{Role: "assistant", Content: "I'll spawn a worker.", ToolCalls: []providers.ToolCall{
			{ID: "call_001", Name: "spawn_agent", Arguments: `{"description":"git pull","prompt":"run git pull"}`},
		}},
		{Role: "tool", ToolCallID: "call_001", Name: "spawn_agent", Content: `{"agent_id":"w-123","status":"running"}`},
		// Key: empty assistant persisted for alternation (the fix).
		{Role: "assistant", Content: ""},
		// Worker result injected after completion.
		{Role: "user", Content: "<worker-result agent-id=\"w-123\">\nDone: git pull succeeded\n</worker-result>"},
		// BeforeStep env context injection.
		{Role: "user", Content: "[Environment context]: cwd=/workspace"},
	}

	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode: %v", err)
		}
		// Return a minimal valid SSE response.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"m\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"test\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n"))
		w.Write([]byte("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"))
		w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"OK\"}}\n\n"))
		w.Write([]byte("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n"))
		w.Write([]byte("event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":1}}\n\n"))
		w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer server.Close()

	client, err := New(ClientConfig{
		BaseURL:  server.URL,
		APIKey:   "test-key",
		MaxTokens: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}

	ch, err := client.StreamChat(context.Background(), providers.ChatRequest{
		Model:    "claude-opus-4-6",
		Messages: history,
	})
	if err != nil {
		t.Fatalf("StreamChat: %v", err)
	}
	// Drain events.
	for range ch {
	}

	// Verify the captured request body.
	msgs, ok := capturedBody["messages"].([]any)
	if !ok {
		t.Fatalf("no messages in request body")
	}

	// Check role alternation.
	prevRole := ""
	for i, raw := range msgs {
		msg := raw.(map[string]any)
		role := msg["role"].(string)
		if role == prevRole {
			t.Fatalf("consecutive same role at index %d: %s (prev also %s)", i, role, prevRole)
		}
		prevRole = role

		// Check no mixed tool_result+text blocks in any user message.
		if role == "user" {
			content := msg["content"].([]any)
			hasToolResult := false
			hasText := false
			for _, block := range content {
				b := block.(map[string]any)
				switch b["type"].(string) {
				case "tool_result":
					hasToolResult = true
				case "text":
					hasText = true
				}
			}
			if hasToolResult && hasText {
				t.Fatalf("user message at index %d has mixed tool_result+text blocks: %+v", i, content)
			}
		}
	}

	t.Logf("✓ %d messages, strict alternation, no mixed blocks", len(msgs))

	// Also print role sequence for clarity.
	roles := ""
	for i, raw := range msgs {
		msg := raw.(map[string]any)
		if i > 0 {
			roles += " → "
		}
		role := msg["role"].(string)
		content := msg["content"].([]any)
		types := ""
		for j, block := range content {
			b := block.(map[string]any)
			if j > 0 {
				types += "+"
			}
			types += b["type"].(string)
		}
		roles += role + "(" + types + ")"
	}
	t.Logf("  sequence: %s", roles)
}

// TestWorkerResultMergesConsecutiveUserMessages verifies that consecutive
// user messages (tool result + worker-result text) are correctly merged
// into a single user message. Both the Anthropic API and proxies accept
// mixed tool_result+text blocks — see docs/proxy-compatibility.md.
func TestWorkerResultMergesConsecutiveUserMessages(t *testing.T) {
	history := []providers.ChatMessage{
		{Role: "system", Content: "You are a coding agent."},
		{Role: "user", Content: "Run git pull"},
		{Role: "assistant", Content: "I'll spawn a worker.", ToolCalls: []providers.ToolCall{
			{ID: "call_001", Name: "spawn_agent", Arguments: `{"description":"git pull","prompt":"run git pull"}`},
		}},
		{Role: "tool", ToolCallID: "call_001", Name: "spawn_agent", Content: `{"agent_id":"w-123","status":"running"}`},
		{Role: "user", Content: "<worker-result>Done</worker-result>"},
	}

	payload, err := buildAnthropicRequest(providers.ChatRequest{
		Model:    "claude-opus-4-6",
		Messages: history,
	}, 1024, false)
	if err != nil {
		t.Fatal(err)
	}

	// Verify strict role alternation (no consecutive same-role).
	prevRole := ""
	for i, msg := range payload.Messages {
		if msg.Role == prevRole {
			t.Fatalf("consecutive same role at index %d: %s", i, msg.Role)
		}
		prevRole = msg.Role
	}

	// Verify the tool_result and worker-result text were merged into
	// a single user message (this is accepted by the proxy).
	lastUser := payload.Messages[len(payload.Messages)-1]
	if lastUser.Role != "user" {
		t.Fatalf("expected last message to be user, got %s", lastUser.Role)
	}
	hasToolResult := false
	hasText := false
	for _, block := range lastUser.Content {
		if block.Type == "tool_result" {
			hasToolResult = true
		}
		if block.Type == "text" {
			hasText = true
		}
	}
	if !hasToolResult || !hasText {
		t.Fatalf("expected merged user message with tool_result+text, got blocks: %+v", lastUser.Content)
	}
	t.Log("✓ Consecutive user messages correctly merged with tool_result+text blocks")
}

// TestThinkingBlockSerialized verifies that assistant messages with
// ReasoningContent produce a {"type":"thinking"} block in the
// Anthropic API request, as required when adaptive/extended thinking
// is enabled. This was the root cause of the post-worker-completion
// reconnect loop — see docs/proxy-compatibility.md.
func TestThinkingBlockSerialized(t *testing.T) {
	history := []providers.ChatMessage{
		{Role: "user", Content: "analyze this code"},
		{
			Role:             "assistant",
			Content:          "Let me look at that.",
			ReasoningContent: "The user wants code analysis. I should read the file first.",
			ToolCalls: []providers.ToolCall{
				{ID: "call_001", Name: "read_file", Arguments: `{"path":"main.go"}`},
			},
		},
		{Role: "tool", ToolCallID: "call_001", Name: "read_file", Content: "package main\n..."},
	}

	payload, err := buildAnthropicRequest(providers.ChatRequest{
		Model:    "claude-opus-4-6",
		Messages: history,
	}, 1024, false)
	if err != nil {
		t.Fatal(err)
	}

	// Find the assistant message.
	var assistantMsg *anthropicMessage
	for i := range payload.Messages {
		if payload.Messages[i].Role == "assistant" {
			assistantMsg = &payload.Messages[i]
			break
		}
	}
	if assistantMsg == nil {
		t.Fatal("no assistant message in payload")
	}

	// Verify block order: thinking → text → tool_use.
	if len(assistantMsg.Content) != 3 {
		t.Fatalf("expected 3 blocks (thinking+text+tool_use), got %d: %+v",
			len(assistantMsg.Content), assistantMsg.Content)
	}
	if assistantMsg.Content[0].Type != "thinking" {
		t.Fatalf("expected first block to be thinking, got %s", assistantMsg.Content[0].Type)
	}
	if assistantMsg.Content[0].Thinking == "" {
		t.Fatal("thinking block has empty content")
	}
	if assistantMsg.Content[1].Type != "text" {
		t.Fatalf("expected second block to be text, got %s", assistantMsg.Content[1].Type)
	}
	if assistantMsg.Content[2].Type != "tool_use" {
		t.Fatalf("expected third block to be tool_use, got %s", assistantMsg.Content[2].Type)
	}

	t.Log("✓ Thinking block correctly serialized in assistant message")
}

// TestThinkingOnlyAssistant_NoSpaceTextBlock verifies that an
// assistant message with ONLY thinking + tool_use (no text content)
// does NOT get a space text block injected. The previous code always
// added {"type":"text","text":" "} which proxies rejected.
func TestThinkingOnlyAssistant_NoSpaceTextBlock(t *testing.T) {
	history := []providers.ChatMessage{
		{Role: "user", Content: "run git pull"},
		{
			Role:             "assistant",
			Content:          "", // no text — model went straight to tool call
			ReasoningContent: "I should run git pull in the worker.",
			ToolCalls: []providers.ToolCall{
				{ID: "call_001", Name: "spawn_agent", Arguments: `{"prompt":"git pull"}`},
			},
		},
		{Role: "tool", ToolCallID: "call_001", Name: "spawn_agent", Content: `{"agent_id":"w-1"}`},
	}

	payload, err := buildAnthropicRequest(providers.ChatRequest{
		Model:    "claude-opus-4-6",
		Messages: history,
	}, 1024, false)
	if err != nil {
		t.Fatal(err)
	}

	var assistantMsg *anthropicMessage
	for i := range payload.Messages {
		if payload.Messages[i].Role == "assistant" {
			assistantMsg = &payload.Messages[i]
			break
		}
	}
	if assistantMsg == nil {
		t.Fatal("no assistant message in payload")
	}

	// Should have exactly 2 blocks: thinking + tool_use. NO text block.
	if len(assistantMsg.Content) != 2 {
		t.Fatalf("expected 2 blocks (thinking+tool_use), got %d: %+v",
			len(assistantMsg.Content), assistantMsg.Content)
	}
	for _, block := range assistantMsg.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) == "" {
			t.Fatalf("found empty/space text block — proxy would reject this: %+v", block)
		}
	}
	if assistantMsg.Content[0].Type != "thinking" {
		t.Fatalf("expected first block to be thinking, got %s", assistantMsg.Content[0].Type)
	}
	if assistantMsg.Content[1].Type != "tool_use" {
		t.Fatalf("expected second block to be tool_use, got %s", assistantMsg.Content[1].Type)
	}

	t.Log("✓ No space text block for thinking-only assistant message")
}

// TestWorkerCompletionWithThinking_FullScenario is the end-to-end
// regression test for the post-worker-completion reconnect loop.
// It simulates the exact message sequence: user → assistant(thinking+
// tool_use) → tool(result) → assistant(response) → user(worker-result),
// and verifies the Anthropic request is structurally valid.
func TestWorkerCompletionWithThinking_FullScenario(t *testing.T) {
	history := []providers.ChatMessage{
		{Role: "system", Content: "You are a coding agent."},
		{Role: "user", Content: "Run all tests"},
		{
			Role:             "assistant",
			Content:          "",
			ReasoningContent: "I should spawn a worker to run tests.",
			ToolCalls: []providers.ToolCall{
				{ID: "call_001", Name: "spawn_agent", Arguments: `{"prompt":"run tests"}`},
			},
		},
		{Role: "tool", ToolCallID: "call_001", Name: "spawn_agent", Content: `{"agent_id":"w-1","status":"running"}`},
		{
			Role:             "assistant",
			Content:          "I've spawned a worker to run the tests.",
			ReasoningContent: "Worker is running, I should tell the user.",
		},
		{Role: "user", Content: "<worker-result agent-id=\"w-1\">\nAll 42 tests passed.\n</worker-result>"},
	}

	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":1}}}\n\n"))
		w.Write([]byte("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"))
		w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"All tests passed!\"}}\n\n"))
		w.Write([]byte("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n"))
		w.Write([]byte("event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":3}}\n\n"))
		w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer server.Close()

	client, err := New(ClientConfig{BaseURL: server.URL, APIKey: "test-key", MaxTokens: 1024})
	if err != nil {
		t.Fatal(err)
	}

	ch, err := client.StreamChat(context.Background(), providers.ChatRequest{
		Model:    "claude-opus-4-6",
		Messages: history,
	})
	if err != nil {
		t.Fatalf("StreamChat: %v", err)
	}
	for range ch {
	}

	// Validate the captured request.
	msgs, ok := capturedBody["messages"].([]any)
	if !ok {
		t.Fatalf("no messages in request body")
	}

	// 1. Check role alternation.
	prevRole := ""
	for i, raw := range msgs {
		msg := raw.(map[string]any)
		role := msg["role"].(string)
		if role == prevRole {
			t.Fatalf("consecutive same role at index %d: %s", i, role)
		}
		prevRole = role
	}

	// 2. Check that thinking blocks are present in assistant messages.
	thinkingCount := 0
	for _, raw := range msgs {
		msg := raw.(map[string]any)
		if msg["role"] != "assistant" {
			continue
		}
		content := msg["content"].([]any)
		for _, block := range content {
			b := block.(map[string]any)
			if b["type"] == "thinking" {
				thinkingCount++
				if b["thinking"] == nil || b["thinking"] == "" {
					t.Fatal("thinking block has empty content")
				}
			}
		}
	}
	if thinkingCount != 2 {
		t.Fatalf("expected 2 thinking blocks (one per assistant), got %d", thinkingCount)
	}

	// 3. Check no empty/space-only text blocks in assistant messages.
	for i, raw := range msgs {
		msg := raw.(map[string]any)
		if msg["role"] != "assistant" {
			continue
		}
		content := msg["content"].([]any)
		for _, block := range content {
			b := block.(map[string]any)
			if b["type"] == "text" {
				text, _ := b["text"].(string)
				if strings.TrimSpace(text) == "" {
					t.Fatalf("assistant msg[%d] has empty/space text block — proxy would reject: %+v", i, content)
				}
			}
		}
	}

	t.Logf("✓ Full worker-completion scenario: %d messages, strict alternation, %d thinking blocks, no empty text blocks",
		len(msgs), thinkingCount)
}
