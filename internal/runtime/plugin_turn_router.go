package runtime

import (
	"context"
	"errors"
	"sync"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
)

// PluginTurnSubmitHandler is installed by the live app-server. The plugin ID
// is supplied by the generation-bound host-service dispatcher, never by the
// plugin request payload.
type PluginTurnSubmitHandler func(context.Context, string, pluginhost.TurnSubmitParams) (pluginhost.TurnSubmitResult, error)

// PluginTurnRouter lets plugin processes outlive individual app-server
// bindings without coupling runtime construction to an app-server package.
type PluginTurnRouter struct {
	mu      sync.RWMutex
	handler PluginTurnSubmitHandler
	binding uint64
}

func NewPluginTurnRouter() *PluginTurnRouter { return &PluginTurnRouter{} }

// Bind replaces the live handler and returns an idempotent unbind closure.
// The closure only clears the handler it installed, so closing an older server
// cannot disconnect a newer binding.
func (r *PluginTurnRouter) Bind(handler PluginTurnSubmitHandler) func() {
	if r == nil {
		return func() {}
	}
	r.mu.Lock()
	r.binding++
	binding := r.binding
	r.handler = handler
	r.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			r.mu.Lock()
			if r.binding == binding {
				r.handler = nil
			}
			r.mu.Unlock()
		})
	}
}

func (r *PluginTurnRouter) Submit(ctx context.Context, pluginID string, params pluginhost.TurnSubmitParams) (pluginhost.TurnSubmitResult, error) {
	if r == nil {
		return pluginhost.TurnSubmitResult{}, errors.New("turn submission service is unavailable")
	}
	r.mu.RLock()
	handler := r.handler
	r.mu.RUnlock()
	if handler == nil {
		return pluginhost.TurnSubmitResult{}, errors.New("turn submission service is unavailable")
	}
	return handler(ctx, pluginID, params)
}
