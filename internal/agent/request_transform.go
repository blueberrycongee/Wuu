package agent

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/blueberrycongee/wuu/internal/providers"
)

// RequestTransform is a function that inspects or modifies a provider
// request before it is sent. Transforms execute in a stable chain: each
// transform receives the output of the previous transform.
//
// Returning an error aborts the current model round. Transforms must not
// mutate the request after returning.
type RequestTransform func(ctx context.Context, req *providers.ChatRequest) error

// RequestTransformProvider contributes a named request transform with
// ordering metadata. It is the pluggable equivalent of LoopConfig.BeforeRequest.
type RequestTransformProvider interface {
	// TransformKey returns a unique key for this transform.
	TransformKey() string

	// Transform returns the transform function.
	Transform() RequestTransform

	// TransformPriority determines ordering. Higher values execute first.
	TransformPriority() int
}

// RequestTransformChain assembles and executes an ordered chain of
// request transforms. It is safe for concurrent use.
type RequestTransformChain struct {
	mu         sync.RWMutex
	transforms map[string]RequestTransformProvider
	order      []string
}

// NewRequestTransformChain creates an empty transform chain.
func NewRequestTransformChain() *RequestTransformChain {
	return &RequestTransformChain{
		transforms: make(map[string]RequestTransformProvider),
	}
}

// Add registers a transform provider. If a provider with the same key
// already exists, it is replaced.
func (c *RequestTransformChain) Add(p RequestTransformProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := p.TransformKey()
	if _, exists := c.transforms[key]; !exists {
		c.order = append(c.order, key)
	}
	c.transforms[key] = p
}

// Remove unregisters a transform provider by key.
func (c *RequestTransformChain) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.transforms, key)
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

// Apply runs all registered transforms in priority order against the
// given request. The first error stops the chain.
//
// When beforeFn is non-nil, it is executed before the chain (backward
// compatibility with LoopConfig.BeforeRequest).
func (c *RequestTransformChain) Apply(ctx context.Context, req *providers.ChatRequest, beforeFn RequestTransform) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Legacy BeforeRequest callback.
	if beforeFn != nil {
		if err := beforeFn(ctx, req); err != nil {
			return fmt.Errorf("request transform (legacy before): %w", err)
		}
	}

	// Collect and sort providers.
	providers := make([]RequestTransformProvider, 0, len(c.transforms))
	for _, key := range c.order {
		if p, ok := c.transforms[key]; ok {
			providers = append(providers, p)
		}
	}

	sort.Slice(providers, func(i, j int) bool {
		pi, pj := providers[i].TransformPriority(), providers[j].TransformPriority()
		if pi != pj {
			return pi > pj
		}
		return providers[i].TransformKey() < providers[j].TransformKey()
	})

	for _, p := range providers {
		if err := p.Transform()(ctx, req); err != nil {
			return fmt.Errorf("request transform %s: %w", p.TransformKey(), err)
		}
	}

	return nil
}

// Count returns the number of registered transforms.
func (c *RequestTransformChain) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.transforms)
}

// simpleRequestTransform is a basic transform provider.
type simpleRequestTransform struct {
	key      string
	fn       RequestTransform
	priority int
}

func (p *simpleRequestTransform) TransformKey() string      { return p.key }
func (p *simpleRequestTransform) Transform() RequestTransform { return p.fn }
func (p *simpleRequestTransform) TransformPriority() int     { return p.priority }

// NewRequestTransform creates a simple request transform provider.
func NewRequestTransform(key string, fn RequestTransform, priority int) RequestTransformProvider {
	return &simpleRequestTransform{key: key, fn: fn, priority: priority}
}
