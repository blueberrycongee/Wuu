package runtime

import (
	"context"
	"encoding/json"
	"testing"

	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
)

func TestKernelHostServicesRouteThroughRegistryAndPreserveStorageScope(t *testing.T) {
	home, workspace := t.TempDir(), t.TempDir()
	item := serviceTestPlugin("demo", "plugin:user:demo", "generation")
	handler := newPluginHostServices(item, workspace, home, nil)
	kernel := newKernelHostServices(nil, nil)
	kernel.add(item.ID, handler)
	registry, conflicts := pluginhost.BuildServiceRegistry(kernel)
	kernel.bindRegistry(registry)
	if len(conflicts) != 0 {
		t.Fatalf("conflicts = %+v", conflicts)
	}
	for _, descriptor := range pluginhost.KernelServiceDescriptors() {
		if !registry.HasProvider(descriptor.Name, 1) {
			t.Fatalf("kernel service %q was not registered", descriptor.Name)
		}
	}
	registry.AllowPreflight(item.ID, pluginhost.KernelPreflightRequirements())

	if _, serviceErr := registry.Call(context.Background(), item.ID, pluginhost.ServiceCallParams{
		Service: pluginhost.KernelStorageSetService, Method: pluginhost.KernelServiceMethod,
		Params: json.RawMessage(`{"scope":"workspace","key":"state","value":"prepared"}`),
	}); serviceErr == nil || serviceErr.Code != "service_not_authorized" {
		t.Fatalf("prepare write error = %#v", serviceErr)
	}

	registry.RegisterClients(&kernelConsumer{id: item.ID, requirements: pluginhost.KernelServiceRequirements(
		pluginhost.KernelStorageGetService, pluginhost.KernelStorageSetService,
	)})
	registry.Activate()
	if _, serviceErr := registry.Call(context.Background(), item.ID, pluginhost.ServiceCallParams{
		Service: pluginhost.KernelStorageSetService, Method: pluginhost.KernelServiceMethod,
		Params: json.RawMessage(`{"scope":"workspace","key":"state","value":"active"}`),
	}); serviceErr != nil {
		t.Fatal(serviceErr)
	}
	result, serviceErr := registry.Call(context.Background(), item.ID, pluginhost.ServiceCallParams{
		Service: pluginhost.KernelStorageGetService, Method: pluginhost.KernelServiceMethod,
		Params: json.RawMessage(`{"scope":"workspace","key":"state"}`),
	})
	if serviceErr != nil || string(result) != `{"value":"active"}` {
		t.Fatalf("get = %s, error = %#v", result, serviceErr)
	}

	registry.Close(context.Background())
	if _, serviceErr := registry.Call(context.Background(), item.ID, pluginhost.ServiceCallParams{
		Service: pluginhost.KernelStorageGetService, Method: pluginhost.KernelServiceMethod,
		Params: json.RawMessage(`{"scope":"workspace","key":"state"}`),
	}); serviceErr == nil || serviceErr.Code != "service_unavailable" {
		t.Fatalf("closed error = %#v", serviceErr)
	}
}

func TestKernelRegistryIntrospectRoutesThroughRegistry(t *testing.T) {
	home, workspace := t.TempDir(), t.TempDir()
	item := serviceTestPlugin("demo", "plugin:user:demo", "generation")
	handler := newPluginHostServices(item, workspace, home, nil)
	kernel := newKernelHostServices(func() uint64 { return 42 }, nil)
	kernel.add(item.ID, handler)
	registry, conflicts := pluginhost.BuildServiceRegistry(kernel)
	kernel.bindRegistry(registry)
	if len(conflicts) != 0 {
		t.Fatalf("conflicts = %+v", conflicts)
	}
	registry.RegisterClients(&kernelConsumer{id: item.ID, requirements: pluginhost.KernelServiceRequirements(
		pluginhost.KernelRegistryIntrospectService,
	)})
	registry.Activate()

	result, serviceErr := registry.Call(context.Background(), item.ID, pluginhost.ServiceCallParams{
		Service: pluginhost.KernelRegistryIntrospectService, Method: pluginhost.KernelServiceMethod,
	})
	if serviceErr != nil {
		t.Fatal(serviceErr)
	}
	var snapshot pluginhost.ServiceRegistrySnapshot
	if err := json.Unmarshal(result, &snapshot); err != nil {
		t.Fatalf("snapshot decode: %v", err)
	}
	if snapshot.Generation != 42 {
		t.Fatalf("generation = %d, want 42", snapshot.Generation)
	}
	foundStorage, foundIntrospect := false, false
	for _, entry := range snapshot.Services {
		if entry.Provider != "kernel" || !entry.Kernel {
			t.Fatalf("unexpected non-kernel entry: %+v", entry)
		}
		switch entry.Service {
		case pluginhost.KernelStorageGetService:
			foundStorage = true
		case pluginhost.KernelRegistryIntrospectService:
			foundIntrospect = true
		}
	}
	if !foundStorage || !foundIntrospect {
		t.Fatalf("services = %+v", snapshot.Services)
	}

	if _, serviceErr := registry.Call(context.Background(), "stranger", pluginhost.ServiceCallParams{
		Service: pluginhost.KernelRegistryIntrospectService, Method: pluginhost.KernelServiceMethod,
	}); serviceErr == nil || serviceErr.Code != "service_not_authorized" {
		t.Fatalf("undeclared caller error = %#v, want service_not_authorized", serviceErr)
	}
}

func TestKernelExecutionUpdateRoutesToExecutionTable(t *testing.T) {
	home, workspace := t.TempDir(), t.TempDir()
	item := serviceTestPlugin("demo", "plugin:user:demo", "generation")
	handler := newPluginHostServices(item, workspace, home, nil)
	recorder := &fakeExecutionRecorder{}
	kernel := newKernelHostServices(nil, recorder)
	kernel.add(item.ID, handler)
	registry, conflicts := pluginhost.BuildServiceRegistry(kernel)
	kernel.bindRegistry(registry)
	if len(conflicts) != 0 {
		t.Fatalf("conflicts = %+v", conflicts)
	}
	registry.RegisterClients(&kernelConsumer{id: item.ID, requirements: pluginhost.KernelServiceRequirements(
		pluginhost.KernelExecutionUpdateService,
	)})
	registry.Activate()

	result, serviceErr := registry.Call(context.Background(), item.ID, pluginhost.ServiceCallParams{
		Service: pluginhost.KernelExecutionUpdateService, Method: pluginhost.KernelServiceMethod,
		Params: json.RawMessage(`{"execution_id":"exec-7","message":"halfway","detail":{"pct":50}}`),
	})
	if serviceErr != nil {
		t.Fatal(serviceErr)
	}
	if string(result) != `{}` {
		t.Fatalf("update result = %s", result)
	}
	if recorder.caller != item.ID || recorder.params.ExecutionID != "exec-7" || recorder.params.Message != "halfway" {
		t.Fatalf("recorder = %q %+v", recorder.caller, recorder.params)
	}

	recorder.err = &pluginhost.HostServiceError{Code: "execution_not_found", Message: "execution exec-7 is not live"}
	if _, serviceErr := registry.Call(context.Background(), item.ID, pluginhost.ServiceCallParams{
		Service: pluginhost.KernelExecutionUpdateService, Method: pluginhost.KernelServiceMethod,
		Params: json.RawMessage(`{"execution_id":"exec-7","message":"late"}`),
	}); serviceErr == nil || serviceErr.Code != "execution_not_found" {
		t.Fatalf("typed tracker error = %#v, want execution_not_found", serviceErr)
	}

	if _, serviceErr := registry.Call(context.Background(), "stranger", pluginhost.ServiceCallParams{
		Service: pluginhost.KernelExecutionUpdateService, Method: pluginhost.KernelServiceMethod,
		Params: json.RawMessage(`{"execution_id":"exec-7"}`),
	}); serviceErr == nil || serviceErr.Code != "service_not_authorized" {
		t.Fatalf("undeclared caller error = %#v, want service_not_authorized", serviceErr)
	}
}

type fakeExecutionRecorder struct {
	caller string
	params pluginhost.ExecutionUpdateParams
	err    *pluginhost.HostServiceError
}

func (f *fakeExecutionRecorder) RecordExecutionUpdate(caller string, params pluginhost.ExecutionUpdateParams) *pluginhost.HostServiceError {
	f.caller, f.params = caller, params
	return f.err
}

type kernelConsumer struct {
	id           string
	requirements []pluginhost.ServiceRequirement
}

func (c *kernelConsumer) ID() string { return c.id }
func (c *kernelConsumer) Status() pluginhost.Status {
	return pluginhost.Status{ID: c.id, State: pluginhost.StatePrepared}
}
func (c *kernelConsumer) Close(context.Context) error                      { return nil }
func (c *kernelConsumer) ProvidedServices() []pluginhost.ServiceDescriptor { return nil }
func (c *kernelConsumer) RequiredServices() []pluginhost.ServiceRequirement {
	return c.requirements
}

func serviceTestPlugin(id, subject, fingerprint string) pluginpkg.Plugin {
	return pluginpkg.Plugin{Manifest: pluginpkg.Manifest{ID: id}, SubjectID: subject, Fingerprint: fingerprint}
}
