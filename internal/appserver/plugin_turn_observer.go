package appserver

import (
	"context"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
)

// notifyPluginTurnCompleted delivers a settled-turn observation to every
// active registration. Observers cannot change host scheduling or turn state.
func (s *Server) notifyPluginTurnCompleted(ctx context.Context, input pluginhost.AgentTurnCompletedInput) error {
	if s == nil || s.rt == nil || s.rt.PluginHost == nil {
		return nil
	}
	for _, capability := range s.rt.PluginHost.Capabilities(pluginhost.CapabilityAgentTurnCompleted) {
		var output pluginhost.AgentTurnCompletedOutput
		if err := s.rt.PluginHost.InvokeCapability(ctx, capability, input, &output); err != nil {
			return err
		}
	}
	return nil
}
