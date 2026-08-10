package pluginhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// The service registry is the only addressable layer for plugin-provided
// services. Plugins cannot discover, connect to, or address one another; a
// consumer emits one host-mediated frame (service.call) and the registry
// resolves the provider by (service name, required major version), enforces
// the consumer's explicit declaration, and routes the call. The registry is
// built from one generation's initialize results and only accepts calls while
// that generation is active; closing it revokes every address and notifies
// consumers so stale resolutions cannot leak across generations.

// ServiceInvoker is the provider-side surface the registry routes calls to.
// ProcessClient implements it over the service.invoke host -> plugin frame.
type ServiceInvoker interface {
	ID() string
	Status() Status
	InvokeService(ctx context.Context, params ServiceInvokeParams) (json.RawMessage, error)
}

// ServiceChangedNotifier receives best-effort service.changed notifications.
type ServiceChangedNotifier interface {
	NotifyServiceChanged(ctx context.Context, params ServiceChangedParams) error
}

// ServiceClient exposes the provide/consume declarations captured during
// initialize. ProcessClient implements it; failed clients do not.
type ServiceClient interface {
	Client
	ProvidedServices() []ServiceDescriptor
	RequiredServices() []ServiceRequirement
}

// ServiceConflict describes a rejected provider registration. The first
// provider of a (name, major) wins; later duplicates surface as diagnostics.
type ServiceConflict struct {
	PluginID string
	Service  string
	Major    int
	Message  string
}

type serviceKey struct {
	name  string
	major int
}

type serviceProvider struct {
	descriptor ServiceDescriptor
	pluginID   string
	invoker    ServiceInvoker
}

// ServiceRegistry resolves and routes service calls for one generation.
type ServiceRegistry struct {
	mu        sync.RWMutex
	active    bool
	providers map[serviceKey]serviceProvider
	consumers map[string]map[serviceKey]struct{}
	notifiers map[string]ServiceChangedNotifier
}

// BuildServiceRegistry collects provide/consume declarations from one
// generation's clients. Providers that cannot receive service.invoke are
// rejected as conflicts.
func BuildServiceRegistry(clients ...Client) (*ServiceRegistry, []ServiceConflict) {
	registry := &ServiceRegistry{
		providers: make(map[serviceKey]serviceProvider),
		consumers: make(map[string]map[serviceKey]struct{}),
		notifiers: make(map[string]ServiceChangedNotifier),
	}
	var conflicts []ServiceConflict
	for _, client := range clients {
		declared, ok := client.(ServiceClient)
		if !ok {
			continue
		}
		if notifier, ok := client.(ServiceChangedNotifier); ok {
			registry.notifiers[declared.ID()] = notifier
		}
		for _, descriptor := range declared.ProvidedServices() {
			major, ok := ServiceVersionMajor(descriptor.Version)
			if !ok {
				continue
			}
			key := serviceKey{name: strings.TrimSpace(descriptor.Name), major: major}
			if existing, duplicate := registry.providers[key]; duplicate {
				conflicts = append(conflicts, ServiceConflict{
					PluginID: declared.ID(),
					Service:  key.name,
					Major:    key.major,
					Message:  fmt.Sprintf("service %s@%d is already provided by plugin %q; keeping the first registration", key.name, key.major, existing.pluginID),
				})
				continue
			}
			invoker, ok := client.(ServiceInvoker)
			if !ok {
				conflicts = append(conflicts, ServiceConflict{
					PluginID: declared.ID(),
					Service:  key.name,
					Major:    key.major,
					Message:  fmt.Sprintf("service %s@%d cannot receive host-routed calls", key.name, key.major),
				})
				continue
			}
			registry.providers[key] = serviceProvider{descriptor: descriptor, pluginID: declared.ID(), invoker: invoker}
		}
		for _, requirement := range declared.RequiredServices() {
			key := serviceKey{name: strings.TrimSpace(requirement.Name), major: requirement.MajorVersion}
			if registry.consumers[declared.ID()] == nil {
				registry.consumers[declared.ID()] = make(map[serviceKey]struct{})
			}
			registry.consumers[declared.ID()][key] = struct{}{}
		}
	}
	return registry, conflicts
}

// Activate opens the registry for calls. The runtime activates the registry
// together with its generation so prepare-phase candidates stay unroutable.
func (r *ServiceRegistry) Activate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active = true
}

// Close revokes every address and best-effort notifies consumers that their
// services went away.
func (r *ServiceRegistry) Close(ctx context.Context) {
	r.mu.Lock()
	if !r.active {
		r.mu.Unlock()
		return
	}
	r.active = false
	keys := make([]serviceKey, 0, len(r.providers))
	for key := range r.providers {
		keys = append(keys, key)
	}
	r.mu.Unlock()
	for _, key := range keys {
		r.notifyConsumers(ctx, key, "provider_closed")
	}
}

// HasProvider reports whether a provider exists for (name, major).
func (r *ServiceRegistry) HasProvider(name string, major int) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.providers[serviceKey{name: strings.TrimSpace(name), major: major}]
	return ok
}

// ProviderMajors returns the provided majors for one service name, for
// diagnostics and version-mismatch errors.
func (r *ServiceRegistry) ProviderMajors(name string) []int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	name = strings.TrimSpace(name)
	var majors []int
	for key := range r.providers {
		if key.name == name {
			majors = append(majors, key.major)
		}
	}
	return majors
}

// CheckSatisfaction returns a deterministic error when a required service has
// no provider. There is no dependency solver: the consumer is refused
// activation with this diagnostic.
func (r *ServiceRegistry) CheckSatisfaction(requirements []ServiceRequirement) error {
	for _, requirement := range requirements {
		if !requirement.Required {
			continue
		}
		name := strings.TrimSpace(requirement.Name)
		if r.HasProvider(name, requirement.MajorVersion) {
			continue
		}
		if majors := r.ProviderMajors(name); len(majors) > 0 {
			return fmt.Errorf("required service %s major version %d has no provider (provided majors: %v)", name, requirement.MajorVersion, majors)
		}
		return fmt.Errorf("required service %s has no provider", name)
	}
	return nil
}

// Call validates and routes one service.call frame from a consumer plugin.
// Authority is exactly the consumer's explicit declaration captured during
// initialize; the not-authorized check runs before existence checks so an
// undeclared consumer cannot probe which services exist.
func (r *ServiceRegistry) Call(ctx context.Context, consumerPluginID string, params ServiceCallParams) (json.RawMessage, *HostServiceError) {
	service := strings.TrimSpace(params.Service)
	method := strings.TrimSpace(params.Method)
	if service == "" || method == "" {
		return nil, &HostServiceError{Code: "invalid_request", Message: "service.call requires non-empty service and method"}
	}
	key := serviceKey{name: service}
	r.mu.RLock()
	active := r.active
	var declaredMajor *int
	if requirements, ok := r.consumers[consumerPluginID]; ok {
		for consumerKey := range requirements {
			if consumerKey.name == service {
				major := consumerKey.major
				declaredMajor = &major
				break
			}
		}
	}
	r.mu.RUnlock()
	if !active {
		return nil, &HostServiceError{Code: "service_unavailable", Message: "service registry is not active"}
	}
	if declaredMajor == nil {
		return nil, &HostServiceError{Code: "service_not_authorized", Message: fmt.Sprintf("plugin %q did not declare service %s in required services", consumerPluginID, service)}
	}
	key.major = *declaredMajor
	r.mu.RLock()
	provider, found := r.providers[key]
	nameExists := false
	if !found {
		for providerKey := range r.providers {
			if providerKey.name == service {
				nameExists = true
				break
			}
		}
	}
	r.mu.RUnlock()
	if !found {
		if !nameExists {
			return nil, &HostServiceError{Code: "service_not_found", Message: fmt.Sprintf("no provider for service %s", service)}
		}
		return nil, &HostServiceError{Code: "service_version_mismatch", Message: fmt.Sprintf("no provider for service %s major version %d (provided majors: %v)", service, key.major, r.ProviderMajors(service))}
	}
	methodDeclared := false
	for _, declared := range provider.descriptor.Methods {
		if declared.Name == method {
			methodDeclared = true
			break
		}
	}
	if !methodDeclared {
		return nil, &HostServiceError{Code: "method_not_found", Message: fmt.Sprintf("service %s does not declare method %q", service, method)}
	}
	if state := provider.invoker.Status().State; state != StateActive {
		go r.notifyConsumers(context.Background(), key, "provider_unavailable")
		return nil, &HostServiceError{Code: "service_unavailable", Message: fmt.Sprintf("service %s provider %q is %s", service, provider.pluginID, state)}
	}
	result, err := provider.invoker.InvokeService(ctx, ServiceInvokeParams{
		Service: service,
		Method:  method,
		Caller:  consumerPluginID,
		Params:  params.Params,
	})
	if err != nil {
		var remoteErr *remoteCallError
		if errors.As(err, &remoteErr) && remoteErr.code != "" {
			return nil, &HostServiceError{Code: remoteErr.code, Message: remoteErr.message}
		}
		return nil, &HostServiceError{Code: "service_unavailable", Message: fmt.Sprintf("service %s call failed: %v", service, err)}
	}
	if len(result) == 0 || !json.Valid(result) {
		return nil, &HostServiceError{Code: "service_unavailable", Message: fmt.Sprintf("service %s provider returned an invalid result", service)}
	}
	return result, nil
}

// notifyConsumers delivers service.changed to every consumer of (name,
// major), best effort. Notifications never block the caller's result.
func (r *ServiceRegistry) notifyConsumers(ctx context.Context, key serviceKey, reason string) {
	r.mu.RLock()
	targets := make([]ServiceChangedNotifier, 0, len(r.consumers))
	for consumerID, requirements := range r.consumers {
		if _, ok := requirements[key]; !ok {
			continue
		}
		if notifier, ok := r.notifiers[consumerID]; ok {
			targets = append(targets, notifier)
		}
	}
	r.mu.RUnlock()
	for _, notifier := range targets {
		notifyCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		_ = notifier.NotifyServiceChanged(notifyCtx, ServiceChangedParams{Service: key.name, Reason: reason})
		cancel()
	}
}
