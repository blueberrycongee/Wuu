package pluginhost

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestHostRetirePluginLeavesUnrelatedPluginActive(t *testing.T) {
	retired := &fakeCapabilityClient{
		fakeClient:   &fakeClient{id: "plugin-a", status: Status{ID: "plugin-a", State: StateActive}},
		capabilities: []CapabilityDescriptor{{ID: "capability.a", Kind: "observe", Version: 1}},
	}
	retained := &fakeCapabilityClient{
		fakeClient:   &fakeClient{id: "plugin-b", status: Status{ID: "plugin-b", State: StateActive}},
		capabilities: []CapabilityDescriptor{{ID: "capability.b", Kind: "observe", Version: 1}},
	}
	host := New(retired, retained)
	staleCapability := host.Capabilities("capability.a")[0]
	retiredExecutionID := host.executions.Begin(retired.id)
	retiredContext := host.executions.live[retiredExecutionID].ctx
	retainedExecutionID := host.executions.Begin(retained.id)
	retainedContext := host.executions.live[retainedExecutionID].ctx

	outcome, found := host.RetirePlugin(context.Background(), retired.id, errors.New("retired"))
	if !found || outcome.PluginID != retired.id || outcome.Err != nil {
		t.Fatalf("retirement = (%+v, %v)", outcome, found)
	}
	if !retired.closed || retained.closed {
		t.Fatalf("closed clients: retired=%v retained=%v", retired.closed, retained.closed)
	}
	if context.Cause(retiredContext) == nil {
		t.Fatal("retired plugin execution was not canceled")
	}
	if context.Cause(retainedContext) != nil {
		t.Fatalf("retained plugin execution canceled: %v", context.Cause(retainedContext))
	}
	if got := host.Statuses(); len(got) != 1 || got[0].ID != retained.id {
		t.Fatalf("statuses = %+v", got)
	}
	if got := host.Capabilities("capability.a"); len(got) != 0 {
		t.Fatalf("retired capabilities remain: %+v", got)
	}
	if got := host.Capabilities("capability.b"); len(got) != 1 || got[0].PluginID != retained.id {
		t.Fatalf("retained capabilities = %+v", got)
	}
	if err := host.InvokeCapability(context.Background(), staleCapability, struct{}{}, &struct{}{}); err == nil || !strings.Contains(err.Error(), "is not active") {
		t.Fatalf("stale capability invocation error = %v", err)
	}
}
