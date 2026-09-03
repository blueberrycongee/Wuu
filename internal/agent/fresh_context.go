package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/blueberrycongee/wuu/internal/compact"
	"github.com/blueberrycongee/wuu/internal/providers"
)

const (
	// FreshContextTargetTokens is the maximum estimated complete model input
	// installed by a note-backed context-window transition.
	FreshContextTargetTokens = 50_000
	// Keep room for provider/plugin request transforms that are not represented
	// in the provider-neutral world-state estimate used by the builder.
	freshContextTransformReserveTokens = 4_000
	freshContextReminderTokens         = 16_000
	newContextToolName                 = "new_context"
)

var (
	ErrFreshContextTooLarge   = errors.New("fresh context cannot fit the target budget")
	ErrFreshContextNotSmaller = errors.New("fresh context would not shrink active history")
)

// HistoryArchive records the physical addresses assigned to one append. Seqs is
// aligned with the input messages; zero means that message was not persisted.
type HistoryArchive struct {
	Seqs    []int
	HeadSeq int
}

// HistoryArchiveFunc durably appends original messages before their active
// context is released.
type HistoryArchiveFunc func(ctx context.Context, messages []providers.ChatMessage) (HistoryArchive, error)

// FreshContextBuilder constructs a bounded replacement without making a model
// request. fixedTokens accounts for current world state and tool schemas.
type FreshContextBuilder func(ctx context.Context, messages []providers.ChatMessage, historyHeadSeq, fixedTokens, targetTokens int) ([]providers.ChatMessage, error)

func buildFreshContext(
	messages []providers.ChatMessage,
	note CompactionNote,
	noteOK bool,
	historyHeadSeq int,
	fixedTokens int,
	targetTokens int,
) ([]providers.ChatMessage, CompactionNote, error) {
	if targetTokens <= 0 {
		targetTokens = FreshContextTargetTokens
	}
	if fixedTokens < 0 {
		fixedTokens = 0
	}
	systemPrefix := freshContextSystemPrefix(messages)
	covered := len(systemPrefix)
	noteBody := "No completed continuation note was available. Recover the current task from the bounded recent workset below and the durable history tools before continuing."
	if noteOK && validCompactionNote(note, messages) {
		covered = note.CoveredMessages
		noteBody = strings.TrimSpace(note.Markdown)
	} else {
		noteOK = false
	}
	recoveryStart := max(1, historyHeadSeq-24)
	recovery := fmt.Sprintf(`

## Durable history recovery

The original conversation and tool facts remain available in this session through Seq %d. This note is working memory, not the final authority for past facts. Read a known address with history_read. For missing or uncertain details, use history_search and then history_read. If the recent workset is incomplete, begin with history_read at Seq %d.`, historyHeadSeq, recoveryStart)
	summary := providers.ChatMessage{
		Role:    "system",
		Content: compact.BuildSummaryContent(noteBody + recovery),
		Hidden:  true,
		Origin:  "internal",
		Cause:   "fresh_context",
	}
	base := append(providers.CloneChatMessages(systemPrefix), summary)
	messageBudget := targetTokens - fixedTokens - freshContextTransformReserveTokens
	if messageBudget <= 0 || estimateFreshContextMessages(base) > messageBudget {
		return nil, CompactionNote{}, fmt.Errorf("%w: fixed context and continuation note require about %d tokens for a %d-token target", ErrFreshContextTooLarge, fixedTokens+estimateFreshContextMessages(base), targetTokens)
	}
	tail := freshContextRecentTail(messages, covered, base, messageBudget)
	replacement := append(base, tail...)
	if err := providers.ValidateToolCallHistory(replacement); err != nil {
		return nil, CompactionNote{}, fmt.Errorf("fresh context has invalid tool-call history: %w", err)
	}
	afterTokens := fixedTokens + estimateFreshContextMessages(replacement)
	beforeTokens := fixedTokens + estimateFreshContextMessages(messages)
	if afterTokens > targetTokens {
		return nil, CompactionNote{}, fmt.Errorf("%w: replacement estimates %d tokens for a %d-token target", ErrFreshContextTooLarge, afterTokens, targetTokens)
	}
	if afterTokens >= beforeTokens {
		return nil, CompactionNote{}, fmt.Errorf("%w: before=%d after=%d", ErrFreshContextNotSmaller, beforeTokens, afterTokens)
	}
	if !noteOK {
		return replacement, CompactionNote{}, nil
	}
	reanchored := note
	reanchored.CoveredMessages = len(systemPrefix) + 1
	reanchored.CoveredHash = CompactionHistoryHash(replacement[:reanchored.CoveredMessages])
	return replacement, reanchored, nil
}

func freshContextSystemPrefix(messages []providers.ChatMessage) []providers.ChatMessage {
	end := 0
	for end < len(messages) && strings.EqualFold(strings.TrimSpace(messages[end].Role), "system") {
		if !compact.IsConversationSummaryContent(messages[end].Content) {
			end++
			continue
		}
		break
	}
	return providers.CloneChatMessages(messages[:end])
}

func freshContextRecentTail(messages []providers.ChatMessage, covered int, base []providers.ChatMessage, budget int) []providers.ChatMessage {
	if covered < 0 {
		covered = 0
	}
	if covered > len(messages) {
		covered = len(messages)
	}
	suffix := messages[covered:]
	userStarts := make([]int, 0)
	for index, message := range suffix {
		if strings.EqualFold(strings.TrimSpace(message.Role), "user") && !message.Hidden {
			userStarts = append(userStarts, index)
		}
	}
	var selected []providers.ChatMessage
	for index := len(userStarts) - 1; index >= 0; index-- {
		candidate := providers.CloneChatMessages(suffix[userStarts[index]:])
		combined := append(providers.CloneChatMessages(base), candidate...)
		if estimateFreshContextMessages(combined) > budget {
			continue
		}
		if providers.ValidateToolCallHistory(combined) != nil {
			continue
		}
		selected = candidate
	}
	return selected
}

func estimateFreshContextMessages(messages []providers.ChatMessage) int {
	return estimateOutboundRequestTokens(providers.ChatRequest{Messages: messages})
}

func hasNewContextToolCall(calls []providers.ToolCall) bool {
	for _, call := range calls {
		if strings.EqualFold(strings.TrimSpace(call.Name), newContextToolName) {
			return true
		}
	}
	return false
}

func withContextWindowGuidance(base func() []ContextSegment) func() []ContextSegment {
	return func() []ContextSegment {
		var segments []ContextSegment
		if base != nil {
			segments = append(segments, base()...)
		}
		segments = append(segments, RequestOnlyContextMessages([]providers.ChatMessage{{
			Role: "system", Name: "wuu_context_window",
			Content: `Wuu maintains a continuation note in a background model fork and keeps exact prior conversation and tool facts in this session's durable history. Do not write or maintain a note yourself. When a low-budget reminder arrives, or when the active context has grown large and you reach a safe semantic breakpoint, call new_context. The host switches only after the current tool batch and does not reset environment state. After a switch, continue from the note and use history_read or history_search only when exact prior details are needed.`,
		}})...)
		return segments
	}
}

func (r *StreamRunner) freshContextTargetTokens(compactThresholdTokens int) int {
	target := FreshContextTargetTokens
	if compactThresholdTokens > 0 && compactThresholdTokens < target {
		target = compactThresholdTokens
	}
	inputLimit := r.MaxInputTokens
	if inputLimit <= 0 && r.ContextWindowOverride > 0 {
		inputLimit = r.ContextWindowOverride - max(0, r.OutputReserveTokens)
	}
	if inputLimit > 0 && inputLimit < target {
		target = inputLimit
	}
	return max(1, target)
}
