package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/blueberrycongee/wuu/internal/providers"
)

type ModelProviderFactory interface {
	ProviderKey() string
	SupportsModel(model string) bool
	CreateClient(ctx context.Context, model string, opts ModelProviderOptions) (providers.Client, error)
	Priority() int
}

type ModelProviderOptions struct {
	APIKey  string
	BaseURL string
	Extra   map[string]any
}

// ModelProviderRegistry manages registered model provider factories
// with plugin ownership tracking for generation-scoped cleanup.
type ModelProviderRegistry struct {
	mu        sync.RWMutex
	factories map[string]ModelProviderFactory
	owners    map[string]string // key → pluginID
	order     []string
}

func NewModelProviderRegistry() *ModelProviderRegistry {
	return &ModelProviderRegistry{
		factories: make(map[string]ModelProviderFactory),
		owners:    make(map[string]string),
	}
}

func (r *ModelProviderRegistry) Register(f ModelProviderFactory) {
	r.RegisterWithOwner(f, "")
}

func (r *ModelProviderRegistry) RegisterWithOwner(f ModelProviderFactory, pluginID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := f.ProviderKey()
	if _, exists := r.factories[key]; !exists {
		r.order = append(r.order, key)
	}
	r.factories[key] = f
	if pluginID != "" {
		r.owners[key] = pluginID
	}
}

func (r *ModelProviderRegistry) Unregister(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.factories, key)
	delete(r.owners, key)
	for i, k := range r.order {
		if k == key {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
}

func (r *ModelProviderRegistry) RemoveByPlugin(pluginID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var toRemove []string
	for key, owner := range r.owners {
		if owner == pluginID {
			toRemove = append(toRemove, key)
		}
	}
	for _, key := range toRemove {
		delete(r.factories, key)
		delete(r.owners, key)
	}
	filtered := make([]string, 0, len(r.order))
	for _, key := range r.order {
		if _, ok := r.factories[key]; ok {
			filtered = append(filtered, key)
		}
	}
	r.order = filtered
}

func (r *ModelProviderRegistry) Resolve(model string) ModelProviderFactory {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var best ModelProviderFactory
	bestPriority := -1

	for _, key := range r.order {
		f, ok := r.factories[key]
		if !ok {
			continue
		}
		if f.SupportsModel(model) {
			pri := f.Priority()
			if pri > bestPriority {
				best = f
				bestPriority = pri
			}
		}
	}
	return best
}

func (r *ModelProviderRegistry) CreateClient(ctx context.Context, model string, opts ModelProviderOptions) (providers.Client, error) {
	factory := r.Resolve(model)
	if factory == nil {
		return nil, fmt.Errorf("no provider factory supports model %q", model)
	}
	return factory.CreateClient(ctx, model, opts)
}

func (r *ModelProviderRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.factories)
}
