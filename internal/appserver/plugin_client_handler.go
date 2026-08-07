package appserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
)

const maxPluginClientPayloadBytes = 1 << 20

// PluginClientRequestParams is a generation-bound opaque request from a Wuu
// client to one plugin runtime. Method and input are plugin-owned contracts.
type PluginClientRequestParams struct {
	ID          string          `json:"id"`
	Fingerprint string          `json:"fingerprint"`
	Method      string          `json:"method"`
	Input       json.RawMessage `json:"input,omitempty"`
}

type PluginClientRequestResult struct {
	Result json.RawMessage `json:"result,omitempty"`
}

func (s *Server) handlePluginClientRequest(ctx context.Context, req Request) error {
	var params PluginClientRequestParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	plugin, err := s.requireActiveDesktopPlugin(params.ID, params.Fingerprint)
	if err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	method := strings.TrimSpace(params.Method)
	if method == "" {
		return s.writeResponse(req.ID, nil, errors.New("plugin client method is required"))
	}
	if len(method) > 128 {
		return s.writeResponse(req.ID, nil, errors.New("plugin client method exceeds 128 bytes"))
	}
	if len(params.Input) > maxPluginClientPayloadBytes {
		return s.writeResponse(req.ID, nil, errors.New("plugin client input exceeds 1048576 bytes"))
	}
	if len(params.Input) != 0 && !json.Valid(params.Input) {
		return s.writeResponse(req.ID, nil, errors.New("plugin client input must be valid JSON"))
	}
	if s.rt == nil || s.rt.PluginHost == nil {
		return s.writeResponse(req.ID, nil, errors.New("plugin runtime is unavailable"))
	}
	capability, ok := s.rt.PluginHost.Capability(plugin.ID, pluginhost.CapabilityPluginClientRequest)
	if !ok {
		return s.writeResponse(req.ID, nil, errors.New("plugin does not accept client requests"))
	}
	output := pluginhost.PluginClientRequestOutput{}
	if err := s.rt.PluginHost.InvokeCapability(ctx, capability, pluginhost.PluginClientRequestInput{
		Method: method,
		Input:  append(json.RawMessage(nil), params.Input...),
	}, &output); err != nil {
		if policyErr := s.rt.PluginHost.HandleCapabilityError(capability, err); policyErr != nil {
			return s.writeResponse(req.ID, nil, policyErr)
		}
	}
	if len(output.Result) > maxPluginClientPayloadBytes {
		return s.writeResponse(req.ID, nil, errors.New("plugin client result exceeds 1048576 bytes"))
	}
	if len(output.Result) != 0 && !json.Valid(output.Result) {
		return s.writeResponse(req.ID, nil, errors.New("plugin client result must be valid JSON"))
	}
	return s.writeResponse(req.ID, PluginClientRequestResult{Result: output.Result}, nil)
}
