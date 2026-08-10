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
	kernel := newKernelHostServices(nil)
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
	kernel := newKernelHostServices(func() uint64 { return 42 })
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
