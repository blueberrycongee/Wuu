package agent

import (
	"context"
	"fmt"

	"github.com/blueberrycongee/wuu/internal/providers"
)

// ModelProviderFactory creates a provider client for a given model.
// Plugins register factories to add support for new model providers
// without modifying the Wuu source.
type ModelProviderFactory interface {
	// ProviderKey returns a unique identifier for this provider
	// (e.g. "openai", "anthropic", "custom-llm").
	ProviderKey() string

	// SupportsModel reports whether this factory can serve the given model.
	SupportsModel(model string) bool

	// CreateClient creates a new provider client for the given model.
	// The caller is responsible for closing the client.
	CreateClient(ctx context.Context, model string, opts ModelProviderOptions) (providers.Client, error)

	// Priority determines resolution order when multiple factories
	// support the same model. Higher values win.
	Priority() int
}

// ModelProviderOptions carries configuration for provider client creation.
type ModelProviderOptions struct {
	// APIKey is the authentication key, if required.
	APIKey string
	// BaseURL overrides the provider's default endpoint.
	BaseURL string
	// Extra is provider-specific configuration.
	Extra map[string]any
}

// ModelProviderRegistry manages registered model provider factories.
// It resolves which factory handles a given model and creates clients.
type ModelProviderRegistry struct {
	factories map[string]ModelProviderFactory
	order     []string
}

// NewModelProviderRegistry creates an empty provider registry.
func NewModelProviderRegistry() *ModelProviderRegistry {
	return &ModelProviderRegistry{
		factories: make(map[string]ModelProviderFactory),
	}
}

// Register adds a provider factory. If a factory with the same key
// exists, it is replaced.
func (r *ModelProviderRegistry) Register(f ModelProviderFactory) {
	key := f.ProviderKey()
	if _, exists := r.factories[key]; !exists {
		r.order = append(r.order, key)
	}
	r.factories[key] = f
}

// Unregister removes a provider factory by key.
func (r *ModelProviderRegistry) Unregister(key string) {
	delete(r.factories, key)
	for i, k := range r.order {
		if k == key {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
}

// Resolve finds the highest-priority factory that supports the given model.
// Returns nil if no factory supports the model.
func (r *ModelProviderRegistry) Resolve(model string) ModelProviderFactory {
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

// CreateClient resolves the appropriate factory and creates a client.
func (r *ModelProviderRegistry) CreateClient(ctx context.Context, model string, opts ModelProviderOptions) (providers.Client, error) {
	factory := r.Resolve(model)
	if factory == nil {
		return nil, fmt.Errorf("no provider factory supports model %q", model)
	}
	return factory.CreateClient(ctx, model, opts)
}

// Count returns the number of registered factories.
func (r *ModelProviderRegistry) Count() int {
	return len(r.factories)
}
