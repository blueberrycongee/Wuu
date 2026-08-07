package appserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/blueberrycongee/wuu/internal/agent"
	wuucontext "github.com/blueberrycongee/wuu/internal/context"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
)

const (
	maxPluginContinuationBlocks    = 16
	maxPluginContinuationBlockSize = 1 << 20
)

// pluginContinuation asks active plugins, in negotiated priority order, for
// one continuation turn. Queue admission and execution ownership stay in the
// app-server; plugins only decide whether they have work and prepare model
// context for it.
func (s *Server) pluginContinuation(ctx context.Context, threadID, phase string) (bool, []agent.ContextSegment, *pluginhost.AgentContinuationDisplay, error) {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return false, nil, nil, errors.New("thread_id is required")
	}
	if phase != pluginhost.ContinuationPhaseProbe && phase != pluginhost.ContinuationPhasePrepare {
		return false, nil, nil, fmt.Errorf("invalid continuation phase %q", phase)
	}
	if s == nil || s.rt == nil || s.rt.PluginHost == nil {
		return false, nil, nil, nil
	}
	for _, capability := range s.rt.PluginHost.Capabilities(pluginhost.CapabilityAgentContinuation) {
		var output pluginhost.AgentContinuationOutput
		if err := s.rt.PluginHost.InvokeCapability(ctx, capability, pluginhost.AgentContinuationInput{
			ThreadID: threadID,
			Phase:    phase,
		}, &output); err != nil {
			if policyErr := s.rt.PluginHost.HandleCapabilityError(capability, err); policyErr != nil {
				return false, nil, nil, fmt.Errorf("plugin %q continuation %s: %w", capability.PluginID, phase, policyErr)
			}
			continue
		}
		if !output.Continue {
			continue
		}
		if phase == pluginhost.ContinuationPhaseProbe {
			return true, nil, nil, nil
		}
		blocks, err := continuationContextSegments(output.Blocks)
		if err != nil {
			return false, nil, nil, fmt.Errorf("plugin %q continuation context: %w", capability.PluginID, err)
		}
		return true, blocks, output.Display, nil
	}
	return false, nil, nil, nil
}

func continuationContextSegments(input []pluginhost.AgentContinuationBlock) ([]agent.ContextSegment, error) {
	if len(input) == 0 {
		return nil, errors.New("prepare response requested a turn without context blocks")
	}
	if len(input) > maxPluginContinuationBlocks {
		return nil, fmt.Errorf("too many context blocks: %d", len(input))
	}
	blocks := make([]wuucontext.Block, 0, len(input))
	for index, block := range input {
		kind := strings.TrimSpace(block.Kind)
		content := strings.TrimSpace(block.Content)
		if kind == "" || content == "" {
			return nil, fmt.Errorf("context block %d requires kind and content", index)
		}
		if len(content) > maxPluginContinuationBlockSize {
			return nil, fmt.Errorf("context block %d exceeds %d bytes", index, maxPluginContinuationBlockSize)
		}
		blocks = append(blocks, wuucontext.Block{
			Kind:    wuucontext.BlockKind(kind),
			Title:   strings.TrimSpace(block.Title),
			Source:  strings.TrimSpace(block.Source),
			Content: content,
		})
	}
	return agent.RequestOnlyContextBlocks(blocks), nil
}
