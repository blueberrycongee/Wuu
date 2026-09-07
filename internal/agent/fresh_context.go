package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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
	ErrFreshContextTooLarge    = errors.New("fresh context cannot fit the target budget")
	ErrFreshContextNotSmaller  = errors.New("fresh context would not shrink active history")
	ErrFreshContextCoverageGap = errors.New("uncovered history cannot fit beside the continuation note")
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

// FreshContextCommitFunc atomically installs a checkpoint and note anchor. The
// returned history includes assigned addresses and any concurrently saved tail.
type FreshContextCommitFunc func(context.Context, []providers.ChatMessage, int, string, CompactionNote) ([]providers.ChatMessage, int, error)

// FreshContextBuilder constructs a bounded replacement, using traditional model
// compaction when no usable note is available. fixedTokens accounts for current
// world state and tool schemas.
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
	// Coverage is an active-history prefix, not a physical Seq count. Preserve
	// original addresses across checkpoints; never derive one from a slice index.
	var uncoveredStart, uncoveredEnd, taskSeq int
	for index, message := range messages {
		if strings.EqualFold(message.Role, "user") && !message.Hidden && message.Seq > 0 {
			taskSeq = message.Seq
		}
		if index >= covered && message.Seq > 0 {
			if uncoveredStart == 0 || message.Seq < uncoveredStart {
				uncoveredStart = message.Seq
			}
			uncoveredEnd = max(uncoveredEnd, message.Seq)
		}
	}
	recovery := fmt.Sprintf(`

## Durable history recovery

The original conversation and tool facts remain available in this session through Seq %d. This note is working memory, not the final authority for past facts. Read a known address with history_read. For missing or uncertain details, use history_search and then history_read. If the recent workset is incomplete, begin with history_read at Seq %d.`, historyHeadSeq, recoveryStart)
	if uncoveredStart > 0 {
		recovery += fmt.Sprintf("\nThe note does not cover active-history records in Seq %d–%d. Only a bounded selection may be included below; recover missing progress before repeating actions.", uncoveredStart, uncoveredEnd)
	}
	if taskSeq > 0 {
		recovery += fmt.Sprintf("\nThe latest user instruction is at Seq %d; read it if it is not present below.", taskSeq)
	}
	summary := providers.ChatMessage{
		Role:    "system",
		Content: compact.BuildSummaryContent(noteBody + recovery),
		Hidden:  true,
		Origin:  "internal",
		Cause:   "fresh_context",
	}
	base := append(providers.CloneChatMessages(systemPrefix), summary)
	messageBudget := targetTokens - fixedTokens - min(freshContextTransformReserveTokens, targetTokens/10)
	// A requested reset must release space even when the entire old workset
	// fits the target. Otherwise adding recovery guidance makes every retry a
	// no-op. Leave at least a quarter of the old active input behind.
	messageBudget = min(messageBudget, estimateFreshContextMessages(messages)*3/4)
	if messageBudget <= 0 || estimateFreshContextMessages(base) > messageBudget {
		if noteOK {
			// A note produced under a larger model budget must not prevent a
			// model switch. Fall back to addresses and bounded original work.
			return buildFreshContext(messages, CompactionNote{}, false, historyHeadSeq, fixedTokens, targetTokens)
		}
		return nil, CompactionNote{}, fmt.Errorf("%w: fixed context and continuation note require about %d tokens for a %d-token target", ErrFreshContextTooLarge, fixedTokens+estimateFreshContextMessages(base), targetTokens)
	}
	var tail []providers.ChatMessage
	if noteOK {
		// Only the covered prefix may be replaced by the note. Dropping an
		// uncovered middle makes reusing a reanchored note erase recovered work
		// over and over. Let the caller summarize the current history instead.
		tail = providers.CloneChatMessages(messages[covered:])
		if estimateFreshContextMessages(append(providers.CloneChatMessages(base), tail...)) > messageBudget {
			return nil, CompactionNote{}, ErrFreshContextCoverageGap
		}
	} else {
		tail = freshContextRecentTail(messages, covered, base, messageBudget)
	}
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
	if len(selected) > 0 {
		return selected
	}
	// A checkpoint can cover the user instruction while tools keep running for
	// many steps. Retain the instruction when it fits, then a protocol-complete
	// suffix instead of dropping the whole in-progress turn.
	anchor := -1
	for index := len(messages) - 1; index >= 0; index-- {
		if strings.EqualFold(messages[index].Role, "user") && !messages[index].Hidden {
			anchor = index
			break
		}
	}
	if anchor >= 0 {
		candidate := append(providers.CloneChatMessages(base), messages[anchor])
		if estimateFreshContextMessages(candidate) <= budget {
			selected = providers.CloneChatMessages(messages[anchor : anchor+1])
		}
	}
	anchored := append(providers.CloneChatMessages(base), selected...)
	for start := len(messages) - 1; start >= max(covered, anchor+1); start-- {
		candidate := messages[start:]
		combined := append(providers.CloneChatMessages(anchored), candidate...)
		if estimateFreshContextMessages(combined) > budget {
			break
		}
		if providers.ValidateToolCallHistory(combined) == nil {
			selected = append(providers.CloneChatMessages(anchored[len(base):]), providers.CloneChatMessages(candidate)...)
		}
	}
	return selected
}

func estimateFreshContextMessages(messages []providers.ChatMessage) int {
	return estimateOutboundRequestTokens(providers.ChatRequest{Messages: messages})
}

func acceptedNewContextRequest(results []providers.ChatMessage) bool {
	// Consume completed results, never the model's intent or a truncated
	// transcript. Rejected and failed tools have no control effect.
	for _, message := range results {
		if message.Role != "tool" || message.Name != newContextToolName || message.ToolResult == nil || message.ToolResult.IsError {
			continue
		}
		var signal struct {
			Requested bool `json:"requested"`
		}
		if json.Unmarshal([]byte(message.ToolResult.TextProjection()), &signal) == nil && signal.Requested {
			return true
		}
	}
	return false
}

func deferRepeatedContextTransition(messages []providers.ChatMessage, currentTokens, targetTokens int) bool {
	if targetTokens <= 0 {
		targetTokens = FreshContextTargetTokens
	}
	// Persisted summaries make admission independent of runs and restarts. A
	// voluntary reset is useful again once the window outgrows its target.
	// Forced capacity recovery is handled independently by the loop.
	return currentTokens <= targetTokens && compactSummaryFromMessages(messages) != ""
}

func withContextWindowGuidance(base func() []ContextSegment) func() []ContextSegment {
	return func() []ContextSegment {
		var segments []ContextSegment
		if base != nil {
			segments = append(segments, base()...)
		}
		segments = append(segments, RequestOnlyContextMessages([]providers.ChatMessage{
			contextWindowReminder(`Wuu maintains a continuation note in a background model fork and keeps exact prior conversation and tool facts in this session's durable history. Do not write or maintain a note yourself. When a low-budget reminder arrives, or when the active context has grown large and you reach a safe semantic breakpoint, call new_context. The host switches only after the current tool batch and does not reset environment state. After a switch, continue from the note and use history_read or history_search only when exact prior details are needed.`),
		})...)
		return segments
	}
}

func contextWindowReminder(content string) providers.ChatMessage {
	return providers.ChatMessage{
		Role:    "user",
		Name:    "wuu_context_window",
		Content: "<system-reminder>\n" + strings.TrimSpace(content) + "\n</system-reminder>",
		Hidden:  true,
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

// A rejected request is negative capacity evidence even when model metadata is
// missing. Never repeat the same fresh-window target after an overflow.
func reactiveFreshContextTarget(target, lastSuccessful, failed int) int {
	if target <= 0 {
		target = FreshContextTargetTokens
	}
	target = reactiveCompactTarget(target, lastSuccessful)
	probe := min(target, failed)
	margin := min(max(probe/observedCompactSafetyDivisor, observedCompactMinMargin), probe/2)
	return max(1, probe-margin)
}

func (r *StreamRunner) contextWindowStatus(ctx context.Context, providerKey string, messages []providers.ChatMessage) string {
	note, ok, err := loadValidCompactionNote(ctx, r.CompactionNoteStore, providerKey, messages)
	state := "No usable completed continuation note is available."
	if err != nil {
		state = "Continuation note availability could not be verified."
	} else if ok {
		state = "A completed continuation note is available."
		if note.CoveredMessages < len(messages) {
			state += " It does not yet cover all recent work."
		}
		// A reanchored prefix contains synthetic records with newer Seq values;
		// their maximum is not an original-history coverage watermark.
		state += fmt.Sprintf(" It covers %d of %d active-history messages, not necessarily every original record.", note.CoveredMessages, len(messages))
	}
	r.noteMu.Lock()
	inFlight := r.noteInFlight
	backingOff := time.Now().Before(r.noteRetryAfter)
	r.noteMu.Unlock()
	if inFlight {
		state += " A background refresh is scheduled or running."
	} else if backingOff {
		state += " Only the background note refresh failed; this is not a context-window transition or durable-history recovery failure. Continue the task using available history; the host will retry the note at a later safe boundary."
	}
	return state + " Do not wait, poll, or write a note yourself. new_context requests a transition; only a host completion confirms it. If no usable note is available, the host will attempt traditional compaction before installing a new context."
}
