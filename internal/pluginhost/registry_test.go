package pluginhost

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeServiceProcess struct {
	id          string
	state       State
	provided    []ServiceDescriptor
	required    []ServiceRequirement
	invoke      func(ctx context.Context, params ServiceInvokeParams) (json.RawMessage, error)
	notified    []ServiceChangedParams
	invokeCalls []ServiceInvokeParams
}

func (c *fakeServiceProcess) ID() string     { return c.id }
func (c *fakeServiceProcess) Status() Status { return Status{ID: c.id, State: c.state} }
func (c *fakeServiceProcess) Close(context.Context) error {
	if c.state == StateActive {
		c.state = StateStopped
	}
	return nil
}
func (c *fakeServiceProcess) ProvidedServices() []ServiceDescriptor  { return c.provided }
func (c *fakeServiceProcess) RequiredServices() []ServiceRequirement { return c.required }
func (c *fakeServiceProcess) InvokeService(ctx context.Context, params ServiceInvokeParams) (json.RawMessage, error) {
	c.invokeCalls = append(c.invokeCalls, params)
	if c.invoke != nil {
		return c.invoke(ctx, params)
	}
	return json.RawMessage(`{"ok":true}`), nil
}
func (c *fakeServiceProcess) NotifyServiceChanged(_ context.Context, params ServiceChangedParams) error {
	c.notified = append(c.notified, params)
	return nil
}

func searchService(version string) ServiceDescriptor {
	return ServiceDescriptor{
		Name:    "search.provider",
		Version: version,
		Methods: []ServiceMethodDescriptor{
			{Name: "query", InputSchema: "search.query.request.v1", OutputSchema: "search.query.response.v1"},
		},
	}
}

func TestBuildServiceRegistry(t *testing.T) {
	providerV1 := &fakeServiceProcess{id: "search-a", state: StatePrepared, provided: []ServiceDescriptor{searchService("1.0.0")}}
	providerDup := &fakeServiceProcess{id: "search-b", state: StatePrepared, provided: []ServiceDescriptor{searchService("1.1.0")}}
	providerV2 := &fakeServiceProcess{id: "search-c", state: StatePrepared, provided: []ServiceDescriptor{searchService("2.0.0")}}
	consumer := &fakeServiceProcess{id: "notes", state: StatePrepared, required: []ServiceRequirement{{Name: "search.provider", MajorVersion: 2, Required: true}}}

	registry, conflicts := BuildServiceRegistry(providerV1, providerDup, providerV2, consumer)
	if len(conflicts) != 1 || conflicts[0].PluginID != "search-b" {
		t.Fatalf("conflicts = %+v, want one conflict for search-b", conflicts)
	}
	if !registry.HasProvider("search.provider", 1) || !registry.HasProvider("search.provider", 2) {
		t.Fatal("expected providers for majors 1 and 2")
	}
	if err := registry.CheckSatisfaction(consumer.RequiredServices()); err != nil {
		t.Fatalf("CheckSatisfaction() = %v, want nil", err)
	}
}

func TestServiceRegistryCheckSatisfaction(t *testing.T) {
	registry, _ := BuildServiceRegistry(&fakeServiceProcess{id: "search", state: StatePrepared, provided: []ServiceDescriptor{searchService("1.4.0")}})
	if err := registry.CheckSatisfaction([]ServiceRequirement{{Name: "search.provider", MajorVersion: 2, Required: true}}); err == nil || !strings.Contains(err.Error(), "provided majors") {
		t.Fatalf("version mismatch error = %v", err)
	}
	if err := registry.CheckSatisfaction([]ServiceRequirement{{Name: "memory.index", MajorVersion: 1, Required: true}}); err == nil || !strings.Contains(err.Error(), "has no provider") {
		t.Fatalf("missing service error = %v", err)
	}
	if err := registry.CheckSatisfaction([]ServiceRequirement{{Name: "memory.index", MajorVersion: 1}}); err != nil {
		t.Fatalf("optional requirement must not block: %v", err)
	}
}

func TestServiceRegistryCall(t *testing.T) {
	provider := &fakeServiceProcess{id: "search", state: StateActive, provided: []ServiceDescriptor{searchService("1.0.0")}}
	consumer := &fakeServiceProcess{id: "notes", state: StateActive, required: []ServiceRequirement{
		{Name: "search.provider", MajorVersion: 1, Required: true},
		{Name: "memory.index", MajorVersion: 1},
	}}
	stranger := &fakeServiceProcess{id: "stranger", state: StateActive}

	registry, _ := BuildServiceRegistry(provider, consumer, stranger)
	call := ServiceCallParams{Service: "search.provider", Method: "query", Params: json.RawMessage(`{"q":"x"}`)}

	if _, err := registry.Call(context.Background(), "notes", call); err == nil || err.Code != "service_unavailable" {
		t.Fatalf("inactive registry call = %v, want service_unavailable", err)
	}
	registry.Activate()

	if _, err := registry.Call(context.Background(), "stranger", call); err == nil || err.Code != "service_not_authorized" {
		t.Fatalf("undeclared consumer = %v, want service_not_authorized", err)
	}
	// A declared-but-unprovided service resolves to not_found; an undeclared
	// one stays not_authorized so consumers cannot probe what exists.
	if _, err := registry.Call(context.Background(), "notes", ServiceCallParams{Service: "memory.index", Method: "query"}); err == nil || err.Code != "service_not_found" {
		t.Fatalf("declared unprovided service = %v, want service_not_found", err)
	}
	if _, err := registry.Call(context.Background(), "notes", ServiceCallParams{Service: "unknown.service", Method: "query"}); err == nil || err.Code != "service_not_authorized" {
		t.Fatalf("undeclared service = %v, want service_not_authorized", err)
	}
	majorMismatch := &fakeServiceProcess{id: "notes2", state: StateActive, required: []ServiceRequirement{{Name: "search.provider", MajorVersion: 2, Required: true}}}
	registry2, _ := BuildServiceRegistry(provider, majorMismatch)
	registry2.Activate()
	if _, err := registry2.Call(context.Background(), "notes2", call); err == nil || err.Code != "service_version_mismatch" {
		t.Fatalf("major mismatch = %v, want service_version_mismatch", err)
	}
	if _, err := registry.Call(context.Background(), "notes", ServiceCallParams{Service: "search.provider", Method: "delete"}); err == nil || err.Code != "method_not_found" {
		t.Fatalf("undeclared method = %v, want method_not_found", err)
	}
	if _, err := registry.Call(context.Background(), "notes", ServiceCallParams{Service: " ", Method: "query"}); err == nil || err.Code != "invalid_request" {
		t.Fatalf("blank service = %v, want invalid_request", err)
	}

	result, err := registry.Call(context.Background(), "notes", call)
	if err != nil {
		t.Fatalf("routed call failed: %v", err)
	}
	if string(result) != `{"ok":true}` {
		t.Fatalf("result = %s", result)
	}
	if len(provider.invokeCalls) != 1 || provider.invokeCalls[0].Caller != "notes" || provider.invokeCalls[0].Method != "query" {
		t.Fatalf("invoke params = %+v", provider.invokeCalls)
	}
}

func TestServiceRegistryCallProviderFailures(t *testing.T) {
	provider := &fakeServiceProcess{id: "search", state: StateActive, provided: []ServiceDescriptor{searchService("1.0.0")}}
	consumer := &fakeServiceProcess{id: "notes", state: StateActive, required: []ServiceRequirement{{Name: "search.provider", MajorVersion: 1, Required: true}}}
	registry, _ := BuildServiceRegistry(provider, consumer)
	registry.Activate()
	call := ServiceCallParams{Service: "search.provider", Method: "query"}

	provider.state = StateFailed
	if _, err := registry.Call(context.Background(), "notes", call); err == nil || err.Code != "service_unavailable" {
		t.Fatalf("dead provider = %v, want service_unavailable", err)
	}
	provider.state = StateActive

	provider.invoke = func(context.Context, ServiceInvokeParams) (json.RawMessage, error) {
		return nil, &remoteCallError{code: "quota_exceeded", message: "quota exceeded"}
	}
	if _, err := registry.Call(context.Background(), "notes", call); err == nil || err.Code != "quota_exceeded" {
		t.Fatalf("provider error = %v, want code passthrough", err)
	}

	provider.invoke = func(context.Context, ServiceInvokeParams) (json.RawMessage, error) {
		return nil, errors.New("transport broken")
	}
	if _, err := registry.Call(context.Background(), "notes", call); err == nil || err.Code != "service_unavailable" {
		t.Fatalf("transport error = %v, want service_unavailable", err)
	}
}

func TestServiceRegistryCloseNotifiesConsumers(t *testing.T) {
	provider := &fakeServiceProcess{id: "search", state: StateActive, provided: []ServiceDescriptor{searchService("1.0.0")}}
	consumer := &fakeServiceProcess{id: "notes", state: StateActive, required: []ServiceRequirement{{Name: "search.provider", MajorVersion: 1, Required: true}}}
	unrelated := &fakeServiceProcess{id: "other", state: StateActive, required: []ServiceRequirement{{Name: "memory.index", MajorVersion: 1}}}
	registry, _ := BuildServiceRegistry(provider, consumer, unrelated)
	registry.Activate()
	registry.Close(context.Background())

	if len(consumer.notified) != 1 || consumer.notified[0].Service != "search.provider" || consumer.notified[0].Reason != "provider_closed" {
		t.Fatalf("consumer notifications = %+v", consumer.notified)
	}
	if len(unrelated.notified) != 0 {
		t.Fatalf("unrelated consumer must not be notified: %+v", unrelated.notified)
	}
	if _, err := registry.Call(context.Background(), "notes", ServiceCallParams{Service: "search.provider", Method: "query"}); err == nil || err.Code != "service_unavailable" {
		t.Fatalf("call after close = %v, want service_unavailable", err)
	}
}

func TestServiceRegistrySnapshot(t *testing.T) {
	providerZeta := &fakeServiceProcess{id: "zeta", state: StateActive, provided: []ServiceDescriptor{searchService("1.2.0")}}
	providerAlpha := &fakeServiceProcess{id: "alpha", state: StateActive, provided: []ServiceDescriptor{{
		Name: "agent.factory", Version: "2.0.0",
		Methods: []ServiceMethodDescriptor{
			{Name: "spawn", InputSchema: "agent.spawn.request.v1", OutputSchema: "agent.spawn.response.v1"},
			{Name: "list", InputSchema: "agent.list.request.v1", OutputSchema: "agent.list.response.v1"},
		},
	}}}
	registry, conflicts := BuildServiceRegistry(providerZeta, providerAlpha)
	if len(conflicts) != 0 {
		t.Fatalf("conflicts = %+v", conflicts)
	}

	snapshot := registry.Snapshot(7)
	if snapshot.Generation != 7 {
		t.Fatalf("generation = %d, want 7", snapshot.Generation)
	}
	if len(snapshot.Services) != 2 {
		t.Fatalf("services = %+v", snapshot.Services)
	}
	first, second := snapshot.Services[0], snapshot.Services[1]
	if first.Service != "agent.factory" || first.Provider != "alpha" || first.Version != "2.0.0" || first.Kernel {
		t.Fatalf("first entry = %+v", first)
	}
	if strings.Join(first.Methods, ",") != "list,spawn" {
		t.Fatalf("methods not sorted: %+v", first.Methods)
	}
	if second.Service != "search.provider" || second.Provider != "zeta" || second.Version != "1.2.0" {
		t.Fatalf("second entry = %+v", second)
	}
	if again := registry.Snapshot(7); !reflect.DeepEqual(snapshot, again) {
		t.Fatalf("snapshot not deterministic: %+v vs %+v", snapshot, again)
	}
}

func TestServiceRegistryCallPreservesTypedProviderErrors(t *testing.T) {
	provider := &fakeServiceProcess{
		id: "slow.tool", state: StateActive, provided: []ServiceDescriptor{searchService("1.0.0")},
		invoke: func(_ context.Context, params ServiceInvokeParams) (json.RawMessage, error) {
			return nil, &HostServiceError{Code: "execution_not_found", Message: "execution exec-1 is not live"}
		},
	}
	consumer := &fakeServiceProcess{id: "notes", state: StateActive, required: []ServiceRequirement{{Name: "search.provider", MajorVersion: 1, Required: true}}}
	registry, conflicts := BuildServiceRegistry(provider, consumer)
	if len(conflicts) != 0 {
		t.Fatalf("conflicts = %+v", conflicts)
	}
	registry.Activate()
	_, err := registry.Call(context.Background(), "notes", ServiceCallParams{Service: "search.provider", Method: "query"})
	if err == nil || err.Code != "execution_not_found" || err.Message != "execution exec-1 is not live" {
		t.Fatalf("typed provider error = %v, want execution_not_found", err)
	}
}

func TestServiceRegistryCallProviderFromKernel(t *testing.T) {
	provider := &fakeServiceProcess{id: "driver", state: StateActive, provided: []ServiceDescriptor{{
		Name:    "driver.singlepass",
		Version: "1.0.0",
		Methods: []ServiceMethodDescriptor{{Name: "run", InputSchema: "driver.run.request.v1", OutputSchema: "driver.run.response.v1"}},
	}}}
	registry, _ := BuildServiceRegistry(provider)

	if _, err := registry.CallProvider(context.Background(), "driver.singlepass", 1, "run", nil, "exec-1"); err == nil || err.Code != "service_unavailable" {
		t.Fatalf("inactive registry kernel call = %v, want service_unavailable", err)
	}
	registry.Activate()

	if _, err := registry.CallProvider(context.Background(), "driver.singlepass", 2, "run", nil, "exec-1"); err == nil || err.Code != "service_not_found" {
		t.Fatalf("missing major = %v, want service_not_found", err)
	}
	if _, err := registry.CallProvider(context.Background(), "driver.singlepass", 1, "create", nil, "exec-1"); err == nil || err.Code != "method_not_found" {
		t.Fatalf("undeclared method = %v, want method_not_found", err)
	}
	if _, err := registry.CallProvider(context.Background(), " ", 1, "run", nil, "exec-1"); err == nil || err.Code != "invalid_request" {
		t.Fatalf("blank service = %v, want invalid_request", err)
	}

	// The kernel needs no consumer declaration: the call routes with the
	// kernel caller identity and the execution id intact.
	result, err := registry.CallProvider(context.Background(), "driver.singlepass", 1, "run", json.RawMessage(`{"instance_id":"i-1"}`), "exec-1")
	if err != nil {
		t.Fatalf("kernel call failed: %v", err)
	}
	if string(result) != `{"ok":true}` {
		t.Fatalf("result = %s", result)
	}
	if len(provider.invokeCalls) != 1 {
		t.Fatalf("invoke calls = %+v", provider.invokeCalls)
	}
	got := provider.invokeCalls[0]
	if got.Caller != "kernel" || got.Method != "run" || got.ExecutionID != "exec-1" {
		t.Fatalf("invoke params = %+v", got)
	}
}
