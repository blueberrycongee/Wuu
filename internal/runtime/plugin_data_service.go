package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/sessiontrace"
	"github.com/blueberrycongee/wuu/internal/statepath"
)

// dataQueryInvoker answers host.data.query from the workspace's persisted
// session trace. It is read-only and resolved from the thread_id validated in
// the request; the caller cannot address arbitrary filesystem paths.
type dataQueryInvoker struct {
	parent *kernelHostServices
}

func (k *dataQueryInvoker) ID() string                { return k.parent.ID() }
func (k *dataQueryInvoker) Status() pluginhost.Status { return k.parent.Status() }

func (k *dataQueryInvoker) InvokeService(ctx context.Context, params pluginhost.ServiceInvokeParams) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if params.Method != pluginhost.KernelServiceMethod {
		return nil, serviceError("method_not_found", "kernel service method is unavailable")
	}
	var query pluginhost.DataQueryParams
	if err := decodeServiceParams(params.Params, &query); err != nil {
		return nil, err
	}
	if err := pluginhost.ValidateDataQueryParams(query); err != nil {
		return nil, err
	}

	k.parent.mu.RLock()
	stateDir := k.parent.workspaceStateDir
	k.parent.mu.RUnlock()
	if strings.TrimSpace(stateDir) == "" {
		return nil, serviceError("service_unavailable", "host data service is unavailable")
	}

	tracePath := sessiontrace.Path(statepath.SessionArtifactDir(stateDir, query.ThreadID))
	rawEvents, err := sessiontrace.ReadEvents(tracePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return marshalServiceResult(pluginhost.DataQueryResult{
				Version:  pluginhost.DataQueryServiceVersion,
				ThreadID: query.ThreadID,
				Events:   []pluginhost.DataEvent{},
			})
		}
		return nil, serviceError("data_unavailable", "session data is unavailable")
	}

	events := make([]pluginhost.DataEvent, 0, len(rawEvents))
	for _, event := range rawEvents {
		events = append(events, pluginhost.DataEvent{
			Type:      event.Type,
			ThreadID:  event.ThreadID,
			TurnID:    event.TurnID,
			CreatedAt: event.CreatedAt,
			Data:      event.Data,
		})
	}
	events = pluginhost.FilterDataEvents(events, query)

	result := pluginhost.DataQueryResult{
		Version:  pluginhost.DataQueryServiceVersion,
		ThreadID: query.ThreadID,
		Events:   make([]pluginhost.DataEvent, 0, len(events)),
	}
	for _, event := range events {
		result.Events = append(result.Events, pluginhost.DataEvent{
			Type:      event.Type,
			ThreadID:  event.ThreadID,
			TurnID:    event.TurnID,
			CreatedAt: event.CreatedAt,
			Data:      event.Data,
		})
	}
	return marshalServiceResult(result)
}
