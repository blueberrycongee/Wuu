package appserver

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/session"
)

const pluginObserverDeliveryTimeout = 5 * time.Second

func (s *Server) notifyPluginTurnLifecycle(ctx context.Context, pluginID string, input pluginhost.AgentTurnLifecycleInput) error {
	if s == nil || s.rt == nil || s.rt.PluginHost == nil {
		return nil
	}
	terminal, err := s.persistPluginTurnLifecycle(pluginID, input)
	if err != nil {
		return err
	}
	return s.deliverPluginTurnLifecycle(ctx, pluginID, input, terminal)
}

// notifyPluginTurnLifecycleAsync persists terminal observations before
// scheduling their best-effort delivery. Plugin helpers use a single ordered
// request worker, so a synchronous callback from a host service (for example,
// close_agent cancelling a queued child) can deadlock the host call that is
// waiting for the callback's response. Durable outbox state lets the next
// replay retry delivery when the helper becomes available again.
func (s *Server) notifyPluginTurnLifecycleAsync(pluginID string, input pluginhost.AgentTurnLifecycleInput) {
	if s == nil || s.rt == nil || s.rt.PluginHost == nil {
		return
	}
	terminal, err := s.persistPluginTurnLifecycle(pluginID, input)
	if err != nil {
		providers.DebugLogf("persist plugin turn lifecycle for %q: %v", input.RequestID, err)
		return
	}
	if !s.startBackground(func() {
		ctx, cancel := context.WithTimeout(context.Background(), pluginObserverDeliveryTimeout)
		defer cancel()
		if err := s.deliverPluginTurnLifecycle(ctx, pluginID, input, terminal); err != nil {
			providers.DebugLogf("deliver plugin turn lifecycle for %q: %v", input.RequestID, err)
		}
	}) {
		// Keep a terminal record in the outbox if shutdown races delivery.
		return
	}
}

func (s *Server) persistPluginTurnLifecycle(pluginID string, input pluginhost.AgentTurnLifecycleInput) (bool, error) {
	terminal := pluginTurnLifecycleTerminal(input.State)
	if !terminal {
		return false, nil
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return true, fmt.Errorf("encode plugin turn lifecycle: %w", err)
	}
	if err := session.PutPluginTurnLifecycleOutbox(s.rt.SessionDir, pluginID, input.RequestID, payload); err != nil {
		return true, err
	}
	return true, nil
}

func (s *Server) deliverPluginTurnLifecycle(ctx context.Context, pluginID string, input pluginhost.AgentTurnLifecycleInput, terminal bool) error {
	capability, ok := s.rt.PluginHost.Capability(pluginID, pluginhost.CapabilityAgentTurnLifecycle)
	if !ok {
		return nil
	}
	var output pluginhost.AgentTurnLifecycleOutput
	if err := s.rt.PluginHost.InvokeCapability(ctx, capability, input, &output); err != nil {
		if terminal {
			s.schedulePluginTurnLifecycleReplay()
		}
		return s.rt.PluginHost.HandleCapabilityError(capability, err)
	}
	if terminal {
		if err := session.DeletePluginTurnLifecycleOutbox(s.rt.SessionDir, pluginID, input.RequestID); err != nil {
			s.schedulePluginTurnLifecycleReplay()
			return err
		}
	}
	return nil
}

const pluginTurnLifecycleReplayDelay = time.Second

func (s *Server) schedulePluginTurnLifecycleReplay() {
	if s == nil || !s.pluginLifecycleReplayPending.CompareAndSwap(false, true) {
		return
	}
	started := s.startBackground(func() {
		timer := time.NewTimer(pluginTurnLifecycleReplayDelay)
		defer timer.Stop()
		<-timer.C
		s.pluginLifecycleReplayPending.Store(false)
		if !s.closed.Load() {
			s.replayPendingPluginTurnLifecycles()
		}
	})
	if !started {
		s.pluginLifecycleReplayPending.Store(false)
	}
}

func pluginTurnLifecycleTerminal(state string) bool {
	switch state {
	case pluginhost.TurnLifecycleCompleted, pluginhost.TurnLifecycleFailed,
		pluginhost.TurnLifecycleInterrupted, pluginhost.TurnLifecycleDiscarded:
		return true
	default:
		return false
	}
}

func (s *Server) replayPendingPluginTurnLifecycles() {
	if s == nil || s.rt == nil {
		return
	}
	entries, err := session.ListPluginTurnLifecycleOutbox(s.rt.SessionDir)
	if err != nil {
		providers.DebugLogf("list pending plugin turn lifecycle events: %v", err)
		return
	}
	for _, entry := range entries {
		var input pluginhost.AgentTurnLifecycleInput
		if err := json.Unmarshal(entry.Payload, &input); err != nil {
			providers.DebugLogf("decode pending plugin turn lifecycle event %q/%q: %v", entry.PluginID, entry.RequestID, err)
			continue
		}
		if err := s.notifyPluginTurnLifecycle(context.Background(), entry.PluginID, input); err != nil {
			providers.DebugLogf("replay pending plugin turn lifecycle event %q/%q: %v", entry.PluginID, entry.RequestID, err)
		}
	}
}

// notifyPluginTurnCompleted delivers a settled-turn observation to every
// active registration. Observers cannot change host scheduling or turn state.
func (s *Server) notifyPluginTurnCompleted(ctx context.Context, input pluginhost.AgentTurnCompletedInput) error {
	if s == nil || s.rt == nil || s.rt.PluginHost == nil {
		return nil
	}
	for _, capability := range s.rt.PluginHost.Capabilities(pluginhost.CapabilityAgentTurnCompleted) {
		var output pluginhost.AgentTurnCompletedOutput
		if err := s.rt.PluginHost.InvokeCapability(ctx, capability, input, &output); err != nil {
			if policyErr := s.rt.PluginHost.HandleCapabilityError(capability, err); policyErr != nil {
				return policyErr
			}
			continue
		}
	}
	return nil
}
