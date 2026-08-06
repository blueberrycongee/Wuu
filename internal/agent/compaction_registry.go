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
	CompactionKey() string
	Compact(ctx context.Context, model string, messages []providers.ChatMessage) ([]providers.ChatMessage, error)
	CompactionPriority() int
}

// CompactionRegistry manages registered compaction strategies with
// plugin ownership tracking for generation-scoped cleanup.
type CompactionRegistry struct {
	mu        sync.RWMutex
	providers map[string]CompactionProvider
	owners    map[string]string // key → pluginID
	order     []string
}

func NewCompactionRegistry() *CompactionRegistry {
	return &CompactionRegistry{
		providers: make(map[string]CompactionProvider),
		owners:    make(map[string]string),
	}
}

func (r *CompactionRegistry) Register(p CompactionProvider) {
	r.RegisterWithOwner(p, "")
}

func (r *CompactionRegistry) RegisterWithOwner(p CompactionProvider, pluginID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := p.CompactionKey()
	if _, exists := r.providers[key]; !exists {
		r.order = append(r.order, key)
	}
	r.providers[key] = p
	if pluginID != "" {
		r.owners[key] = pluginID
	}
}

func (r *CompactionRegistry) Unregister(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, key)
	delete(r.owners, key)
	for i, k := range r.order {
		if k == key {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
}

func (r *CompactionRegistry) RemoveByPlugin(pluginID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeByOwner(pluginID)
}

func (r *CompactionRegistry) RemoveByGeneration(generationID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeByOwner(generationID)
}

func (r *CompactionRegistry) removeByOwner(owner string) {
	var toRemove []string
	for key, entryOwner := range r.owners {
		if entryOwner == owner {
			toRemove = append(toRemove, key)
		}
	}
	for _, key := range toRemove {
		delete(r.providers, key)
		delete(r.owners, key)
	}
	filtered := make([]string, 0, len(r.order))
	for _, key := range r.order {
		if _, ok := r.providers[key]; ok {
			filtered = append(filtered, key)
		}
	}
	r.order = filtered
}

func (r *CompactionRegistry) Resolve(fallback CompactionProvider) CompactionProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var best CompactionProvider
	bestPriority := -1

	for _, key := range r.order {
		p, ok := r.providers[key]
		if !ok {
			continue
		}
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

func (r *CompactionRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.providers)
}
