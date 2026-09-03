package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/blueberrycongee/wuu/internal/providers"
)

// CompactionNote is the active hidden Markdown continuation document for one
// session and compaction provider. The history anchor makes stale documents
// fail closed after edits, rewrites, and forks.
type CompactionNote struct {
	Markdown        string
	CoveredMessages int
	CoveredHash     string
}

type CompactionNotePlan struct {
	Prompt         string
	IntervalTokens int
	MaxBytes       int
}

type CompactionNoteForkResult struct {
	Markdown string
	Usage    *providers.TokenUsage
}

type CompactionNoteReplacement struct {
	Messages        []providers.ChatMessage
	CoveredMessages int
}

// CompactionNoteStore persists only the active document for a provider. The
// session identity is bound by the runtime adapter, not supplied by plugins.
type CompactionNoteStore interface {
	LoadCompactionNote(ctx context.Context, providerKey string) (CompactionNote, bool, error)
	StoreCompactionNote(ctx context.Context, providerKey string, note CompactionNote) error
}

// ConditionalCompactionNoteStore prevents an older background fork from
// overwriting a checkpoint that was refreshed or re-anchored while it ran.
type ConditionalCompactionNoteStore interface {
	CompareAndSwapCompactionNote(ctx context.Context, providerKey string, expected CompactionNote, expectedExists bool, replacement CompactionNote) (bool, error)
}

// ForkingCompactionProvider extends the existing replacement contract with a
// cache-friendly model fork that authors the Markdown checkpoint out of band.
// It is generic: the host owns inference and persistence, while the provider
// owns the prompt, cadence, and replacement policy.
type ForkingCompactionProvider interface {
	CompactionProvider
	CompactionNotesEnabled() bool
	PlanCompactionNote(ctx context.Context, model string, messages, delta []providers.ChatMessage, previous CompactionNote) (CompactionNotePlan, error)
	CompactWithNote(ctx context.Context, model string, messages []providers.ChatMessage, note CompactionNote) (CompactionNoteReplacement, error)
}

type CompactionNoteFork func(context.Context, []providers.ChatMessage, CompactionNotePlan) (CompactionNoteForkResult, error)

// CompactionHistoryHash returns a stable semantic hash suitable for copying a
// note across a session fork. Thread-local sequence and tool-ledger addresses
// are deliberately excluded.
func CompactionHistoryHash(messages []providers.ChatMessage) string {
	cloned := providers.CloneChatMessages(messages)
	for index := range cloned {
		cloned[index].Seq = 0
		cloned[index].ToolInvocationID = ""
	}
	encoded, _ := json.Marshal(cloned)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func validCompactionNote(note CompactionNote, messages []providers.ChatMessage) bool {
	if strings.TrimSpace(note.Markdown) == "" || note.CoveredMessages < 0 || note.CoveredMessages > len(messages) {
		return false
	}
	return note.CoveredHash != "" && note.CoveredHash == CompactionHistoryHash(messages[:note.CoveredMessages])
}

func loadValidCompactionNote(ctx context.Context, store CompactionNoteStore, providerKey string, messages []providers.ChatMessage) (CompactionNote, bool, error) {
	if store == nil {
		return CompactionNote{}, false, nil
	}
	note, ok, err := store.LoadCompactionNote(ctx, providerKey)
	if err != nil || !ok {
		return CompactionNote{}, false, err
	}
	if !validCompactionNote(note, messages) {
		return CompactionNote{}, false, nil
	}
	return note, true, nil
}

func generateCompactionNote(
	ctx context.Context,
	provider ForkingCompactionProvider,
	store CompactionNoteStore,
	fork CompactionNoteFork,
	model string,
	messages []providers.ChatMessage,
	force bool,
) (CompactionNote, *providers.TokenUsage, error) {
	if provider == nil || fork == nil {
		return CompactionNote{}, nil, errors.New("compaction note fork is unavailable")
	}
	var storedPrevious CompactionNote
	var err error
	storedPreviousExists := false
	if store != nil {
		storedPrevious, storedPreviousExists, err = store.LoadCompactionNote(ctx, provider.CompactionKey())
		if err != nil {
			return CompactionNote{}, nil, err
		}
	}
	previous := storedPrevious
	previousExists := storedPreviousExists && validCompactionNote(storedPrevious, messages)
	if !previousExists {
		previous = CompactionNote{}
	}
	start := 0
	if previousExists {
		start = previous.CoveredMessages
	}
	delta := messages[start:]
	plan, err := provider.PlanCompactionNote(ctx, model, messages, delta, previous)
	if err != nil {
		return CompactionNote{}, nil, err
	}
	if strings.TrimSpace(plan.Prompt) == "" {
		return CompactionNote{}, nil, errors.New("compaction provider returned an empty note prompt")
	}
	if !force && plan.IntervalTokens > 0 && estimateCompactionMessagesTokens(delta) < plan.IntervalTokens {
		return previous, nil, ErrCompactionNoteNotDue
	}
	result, err := fork(ctx, messages, plan)
	if err != nil {
		return CompactionNote{}, result.Usage, err
	}
	markdown := strings.TrimSpace(result.Markdown)
	if markdown == "" {
		return CompactionNote{}, result.Usage, errors.New("compaction note fork returned empty Markdown")
	}
	if plan.MaxBytes > 0 && len([]byte(markdown)) > plan.MaxBytes {
		return CompactionNote{}, result.Usage, fmt.Errorf("compaction note exceeds %d bytes", plan.MaxBytes)
	}
	note := CompactionNote{
		Markdown:        markdown,
		CoveredMessages: len(messages),
		CoveredHash:     CompactionHistoryHash(messages),
	}
	if store != nil {
		if conditional, conditionalOK := store.(ConditionalCompactionNoteStore); conditionalOK {
			stored, err := conditional.CompareAndSwapCompactionNote(ctx, provider.CompactionKey(), storedPrevious, storedPreviousExists, note)
			if err != nil {
				return CompactionNote{}, result.Usage, err
			}
			if !stored {
				return CompactionNote{}, result.Usage, ErrCompactionNoteSuperseded
			}
		} else if err := store.StoreCompactionNote(ctx, provider.CompactionKey(), note); err != nil {
			return CompactionNote{}, result.Usage, err
		}
	}
	return note, result.Usage, nil
}

var ErrCompactionNoteNotDue = errors.New("compaction note update is not due")
var ErrCompactionNoteUnsupported = errors.New("compaction provider does not support note forks")
var ErrCompactionNoteSuperseded = errors.New("compaction note update was superseded")

func estimateCompactionMessagesTokens(messages []providers.ChatMessage) int {
	total := 0
	for _, message := range messages {
		total += len([]rune(message.Content))/4 + 8
		for _, call := range message.ToolCalls {
			total += len([]rune(call.Name+call.Arguments))/4 + 16
		}
		if message.ToolResult != nil || strings.EqualFold(message.Role, "tool") {
			total += 256
		}
	}
	return total
}

func annotateCompactionNoteHistorySeqs(messages []providers.ChatMessage) []providers.ChatMessage {
	out := providers.CloneChatMessages(messages)
	for index := range out {
		if out[index].Seq <= 0 {
			continue
		}
		out[index].Content = fmt.Sprintf("[History Seq %d]\n%s", out[index].Seq, out[index].Content)
	}
	return out
}
