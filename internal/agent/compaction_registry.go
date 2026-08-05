package agent

import (
	"context"
	"sync"

	"github.com/blueberrycongee/wuu/internal/providers"
)

// CompactionProvider contributes a named compaction strategy. Plugins
// register compaction providers to replace or augment the default
// conversation summarization behavior.
//
// Only one compaction provider is active for a given run; the highest
// priority provider wins. This matches the decision seam semantics
// from the capability contract.
type CompactionProvider interface {
	// CompactionKey returns a unique identifier for this strategy.
	CompactionKey() string

	// Compact summarizes the conversation history. It receives the
	// current model and full message history and returns the compacted
	// history. Returning unchanged history signals "no-op".
	Compact(ctx context.Context, model string, messages []providers.ChatMessage) ([]providers.ChatMessage, error)

	// CompactionPriority determines which strategy wins when multiple
	// are registered. Higher values take precedence.
	CompactionPriority() int
}

// CompactionRegistry manages registered compaction strategies and
// resolves the active strategy for a run.
type CompactionRegistry struct {
	mu        sync.RWMutex
	providers map[string]CompactionProvider
	order     []string
}

// NewCompactionRegistry creates an empty compaction registry.
func NewCompactionRegistry() *CompactionRegistry {
	return &CompactionRegistry{
		providers: make(map[string]CompactionProvider),
	}
}

// Register adds a compaction provider. If a provider with the same key
// exists, it is replaced.
func (r *CompactionRegistry) Register(p CompactionProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := p.CompactionKey()
	if _, exists := r.providers[key]; !exists {
		r.order = append(r.order, key)
	}
	r.providers[key] = p
}

// Unregister removes a compaction provider by key.
func (r *CompactionRegistry) Unregister(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.providers, key)
	for i, k := range r.order {
		if k == key {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
}

// Resolve returns the highest-priority compaction provider, or nil if
// none are registered. When fallback is non-nil and no provider is
// registered, fallback is returned.
func (r *CompactionRegistry) Resolve(fallback CompactionProvider) CompactionProvider {
	r.mu.RLock()
	providers := make([]CompactionProvider, 0, len(r.order))
	for _, key := range r.order {
		if provider, ok := r.providers[key]; ok {
			providers = append(providers, provider)
		}
	}
	r.mu.RUnlock()

	var best CompactionProvider
	bestPriority := -1

	for _, p := range providers {
		pri := p.CompactionPriority()
		if pri > bestPriority {
			best = p
			bestPriority = pri
		}
	}

	if best != nil {
		return best
	}
	return fallback
}

// Count returns the number of registered strategies.
func (r *CompactionRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.providers)
}
