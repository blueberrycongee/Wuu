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
	kernel := newKernelHostServices()
	kernel.add(item.ID, handler)
	registry, conflicts := pluginhost.BuildServiceRegistry(kernel)
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
