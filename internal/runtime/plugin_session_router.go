package runtime

import (
	"context"
	"errors"
	"sync"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
)

// PluginSessionCreateHandler is installed by the live app-server. The plugin ID
// is supplied by the generation-bound host-service dispatcher, never by the
// plugin request payload.
type PluginSessionCreateHandler func(context.Context, string, pluginhost.SessionCreateParams) (pluginhost.SessionCreateResult, error)
type PluginSessionSendHandler func(context.Context, string, pluginhost.SessionSendParams) (pluginhost.SessionSendResult, error)

// PluginSessionRouter lets plugin processes outlive individual app-server
// bindings without coupling runtime construction to an app-server package.
type PluginSessionRouter struct {
	mu      sync.RWMutex
	create  PluginSessionCreateHandler
	send    PluginSessionSendHandler
	binding uint64
}

func NewPluginSessionRouter() *PluginSessionRouter { return &PluginSessionRouter{} }

// Bind replaces the live handler and returns an idempotent unbind closure.
// The closure only clears the handler it installed, so closing an older server
// cannot disconnect a newer binding.
func (r *PluginSessionRouter) Bind(create PluginSessionCreateHandler, send PluginSessionSendHandler) func() {
	if r == nil {
		return func() {}
	}
	r.mu.Lock()
	r.binding++
	binding := r.binding
	r.create = create
	r.send = send
	r.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			r.mu.Lock()
			if r.binding == binding {
				r.create = nil
				r.send = nil
			}
			r.mu.Unlock()
		})
	}
}

func (r *PluginSessionRouter) Create(ctx context.Context, pluginID string, params pluginhost.SessionCreateParams) (pluginhost.SessionCreateResult, error) {
	if r == nil {
		return pluginhost.SessionCreateResult{}, errors.New("session service is unavailable")
	}
	r.mu.RLock()
	handler := r.create
	r.mu.RUnlock()
	if handler == nil {
		return pluginhost.SessionCreateResult{}, errors.New("session service is unavailable")
	}
	return handler(ctx, pluginID, params)
}

func (r *PluginSessionRouter) Send(ctx context.Context, pluginID string, params pluginhost.SessionSendParams) (pluginhost.SessionSendResult, error) {
	if r == nil {
		return pluginhost.SessionSendResult{}, errors.New("session service is unavailable")
	}
	r.mu.RLock()
	handler := r.send
	r.mu.RUnlock()
	if handler == nil {
		return pluginhost.SessionSendResult{}, errors.New("session service is unavailable")
	}
	return handler(ctx, pluginID, params)
}
