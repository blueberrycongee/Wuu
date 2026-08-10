package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/pluginsettings"
)

// pluginHostServices is bound to exactly one installed plugin generation. It
// never accepts a caller-supplied plugin ID or filesystem path: ownership and
// roots are captured from the validated plugin inventory at activation time.
type pluginHostServices struct {
	mu          sync.RWMutex
	active      bool
	open        bool
	pluginID    string
	subjectID   string
	fingerprint string
	projectRoot string
	wuuHome     string
	settings    map[string]pluginpkg.SettingDefinition
	turnRouter  *PluginSessionRouter
}

func newPluginHostServices(item pluginpkg.Plugin, projectRoot, wuuHome string, turnRouter *PluginSessionRouter) *pluginHostServices {
	settings := make(map[string]pluginpkg.SettingDefinition, len(item.Settings))
	for key, definition := range item.Settings {
		settings[key] = definition
	}
	services := &pluginHostServices{
		open: true, pluginID: item.ID, subjectID: strings.TrimSpace(item.SubjectID),
		fingerprint: item.Fingerprint, projectRoot: projectRoot, wuuHome: wuuHome,
		settings: settings, turnRouter: turnRouter,
	}
	return services
}

func (s *pluginHostServices) close() {
	s.mu.Lock()
	s.open = false
	s.active = false
	s.mu.Unlock()
}

func (s *pluginHostServices) activate() {
	s.mu.Lock()
	if s.open {
		s.active = true
	}
	s.mu.Unlock()
}

func (s *pluginHostServices) invoke(ctx context.Context, method pluginhost.HostServiceMethod, raw json.RawMessage) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.open {
		return nil, serviceError("generation_closed", "plugin generation is no longer active")
	}
	if !s.active && !kernelServiceReadOnly(method) {
		return nil, serviceError("service_unavailable", "host service is unavailable during generation prepare")
	}
	if strings.TrimSpace(s.subjectID) == "" || strings.TrimSpace(s.fingerprint) == "" {
		return nil, serviceError("generation_invalid", "plugin generation has no stable package identity")
	}

	switch method {
	case pluginhost.HostServiceSessionCreate:
		if s.turnRouter == nil {
			return nil, serviceError("service_unavailable", "session service is unavailable")
		}
		var params pluginhost.SessionCreateParams
		if err := decodeServiceParams(raw, &params); err != nil {
			return nil, err
		}
		result, err := s.turnRouter.Create(ctx, s.pluginID, params)
		if err != nil {
			return nil, err
		}
		return marshalServiceResult(result)
	case pluginhost.HostServiceSessionSend:
		if s.turnRouter == nil {
			return nil, serviceError("service_unavailable", "session service is unavailable")
		}
		var params pluginhost.SessionSendParams
		if err := decodeServiceParams(raw, &params); err != nil {
			return nil, err
		}
		result, err := s.turnRouter.Send(ctx, s.pluginID, params)
		if err != nil {
			return nil, err
		}
		return marshalServiceResult(result)
	case pluginhost.HostServiceSessionList:
		if s.turnRouter == nil {
			return nil, serviceError("service_unavailable", "session service is unavailable")
		}
		var params pluginhost.SessionListParams
		if err := decodeServiceParams(raw, &params); err != nil {
			return nil, err
		}
		result, err := s.turnRouter.List(ctx, s.pluginID, params)
		if err != nil {
			return nil, err
		}
		return marshalServiceResult(result)
	case pluginhost.HostServiceSessionCancel:
		if s.turnRouter == nil {
			return nil, serviceError("service_unavailable", "session service is unavailable")
		}
		var params pluginhost.SessionCancelParams
		if err := decodeServiceParams(raw, &params); err != nil {
			return nil, err
		}
		result, err := s.turnRouter.Cancel(ctx, s.pluginID, params)
		if err != nil {
			return nil, err
		}
		return marshalServiceResult(result)
	case pluginhost.HostServiceSettingsGet:
		return s.settingsGet(raw)
	case pluginhost.HostServiceSettingsList:
		if err := decodeServiceParams(raw, &struct{}{}); err != nil {
			return nil, err
		}
		return s.settingsList()
	case pluginhost.HostServiceStorageGet:
		var params pluginhost.StorageGetParams
		if err := decodeServiceParams(raw, &params); err != nil {
			return nil, err
		}
		return s.storageGet(params)
	case pluginhost.HostServiceStorageSet:
		var params pluginhost.StorageSetParams
		if err := decodeServiceParams(raw, &params); err != nil {
			return nil, err
		}
		return s.storageSet(params)
	case pluginhost.HostServiceStorageDelete:
		var params pluginhost.StorageDeleteParams
		if err := decodeServiceParams(raw, &params); err != nil {
			return nil, err
		}
		return s.storageDelete(params)
	case pluginhost.HostServiceStorageKeys:
		var params pluginhost.StorageKeysParams
		if err := decodeServiceParams(raw, &params); err != nil {
			return nil, err
		}
		return s.storageKeys(params)
	case pluginhost.HostServiceStorageCompareExchange:
		var params pluginhost.StorageCompareExchangeParams
		if err := decodeServiceParams(raw, &params); err != nil {
			return nil, err
		}
		return s.storageCompareExchange(params)
	default:
		return nil, serviceError("method_not_found", fmt.Sprintf("host service %q is not implemented", method))
	}
}

func (s *pluginHostServices) settingsGet(raw json.RawMessage) (json.RawMessage, error) {
	var params pluginhost.SettingsGetParams
	if err := decodeServiceParams(raw, &params); err != nil {
		return nil, err
	}
	localKey, definition, err := s.ownedSetting(params.Key)
	if err != nil {
		return nil, err
	}
	value, err := s.settingValue(localKey, definition)
	if err != nil {
		return nil, err
	}
	return marshalServiceResult(pluginhost.SettingsGetResult{Value: value})
}

func (s *pluginHostServices) settingsList() (json.RawMessage, error) {
	entries := make(map[string]json.RawMessage, len(s.settings))
	keys := make([]string, 0, len(s.settings))
	for qualified := range s.settings {
		prefix := s.pluginID + "."
		if strings.HasPrefix(qualified, prefix) {
			keys = append(keys, strings.TrimPrefix(qualified, prefix))
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		_, definition, err := s.ownedSetting(key)
		if err != nil {
			return nil, err
		}
		value, err := s.settingValue(key, definition)
		if err != nil {
			return nil, err
		}
		entries[key] = value
	}
	return marshalServiceResult(pluginhost.SettingsListResult{Entries: entries})
}

func (s *pluginHostServices) ownedSetting(key string) (string, pluginpkg.SettingDefinition, error) {
	key = strings.TrimSpace(key)
	if key == "" || strings.Contains(key, "/") || strings.Contains(key, "\\") || strings.Contains(key, "..") {
		return "", pluginpkg.SettingDefinition{}, serviceError("invalid_params", "setting key must be a plugin-local key")
	}
	definition, ok := s.settings[s.pluginID+"."+key]
	if !ok {
		return "", pluginpkg.SettingDefinition{}, serviceError("setting_not_owned", fmt.Sprintf("plugin does not own setting %q", key))
	}
	return key, definition, nil
}

func (s *pluginHostServices) settingValue(key string, definition pluginpkg.SettingDefinition) (json.RawMessage, error) {
	scope := pluginsettings.Scope(definition.Scope)
	// Settings are intentionally read-only on this channel. Persisted values
	// survive plugin upgrades, so a document fingerprint may differ from this
	// generation's fingerprint. Reading never rewrites it; Desktop's existing
	// fingerprint-gated direct API remains the sole settings writer.
	document, err := pluginsettings.Read(s.wuuHome, s.projectRoot, s.subjectID, scope)
	if err != nil {
		return nil, err
	}
	value, ok := document.Values[key]
	if !ok {
		value, err = json.Marshal(definition.Default)
		if err != nil {
			return nil, fmt.Errorf("encode setting default: %w", err)
		}
	}
	if err := validateRuntimeSettingValue(definition, value); err != nil {
		return nil, fmt.Errorf("persisted setting %q: %w", key, err)
	}
	return append(json.RawMessage(nil), value...), nil
}

// HandleHostService remains an internal semantic test seam; transport routing
// enters through the kernel registry above it.
func (s *pluginHostServices) HandleHostService(ctx context.Context, method pluginhost.HostServiceMethod, raw json.RawMessage) (json.RawMessage, error) {
	return s.invoke(ctx, method, raw)
}

func (s *pluginHostServices) CloseHostServices() { s.close() }

func kernelServiceReadOnly(method pluginhost.HostServiceMethod) bool {
	switch method {
	case pluginhost.HostServiceStorageGet, pluginhost.HostServiceStorageKeys,
		pluginhost.HostServiceSettingsGet, pluginhost.HostServiceSettingsList,
		pluginhost.HostServiceSessionList:
		return true
	default:
		return false
	}
}

type kernelHostServices struct {
	mu       sync.RWMutex
	active   bool
	services map[string]*pluginHostServices
}

func newKernelHostServices() *kernelHostServices {
	return &kernelHostServices{services: make(map[string]*pluginHostServices)}
}

func (k *kernelHostServices) add(pluginID string, services *pluginHostServices) {
	k.mu.Lock()
	k.services[pluginID] = services
	k.mu.Unlock()
}

func (k *kernelHostServices) ID() string { return "kernel" }

func (k *kernelHostServices) Status() pluginhost.Status {
	k.mu.RLock()
	defer k.mu.RUnlock()
	state := pluginhost.StatePrepared
	if k.active {
		state = pluginhost.StateActive
	}
	return pluginhost.Status{ID: k.ID(), State: state}
}

func (k *kernelHostServices) Close(context.Context) error { return nil }
func (k *kernelHostServices) ProvidedServices() []pluginhost.ServiceDescriptor {
	return pluginhost.KernelServiceDescriptors()
}
func (k *kernelHostServices) RequiredServices() []pluginhost.ServiceRequirement { return nil }
func (k *kernelHostServices) KernelServices()                                   {}

func (k *kernelHostServices) ActivateKernelServices() {
	k.mu.Lock()
	k.active = true
	services := make([]*pluginHostServices, 0, len(k.services))
	for _, service := range k.services {
		services = append(services, service)
	}
	k.mu.Unlock()
	for _, service := range services {
		service.activate()
	}
}

func (k *kernelHostServices) CloseKernelServices() {
	k.mu.Lock()
	k.active = false
	services := make([]*pluginHostServices, 0, len(k.services))
	for _, service := range k.services {
		services = append(services, service)
	}
	k.mu.Unlock()
	for _, service := range services {
		service.close()
	}
}

func (k *kernelHostServices) InvokeService(ctx context.Context, params pluginhost.ServiceInvokeParams) (json.RawMessage, error) {
	k.mu.RLock()
	services := k.services[params.Caller]
	k.mu.RUnlock()
	if services == nil {
		return nil, serviceError("service_unavailable", "kernel service caller is unavailable")
	}
	method, ok := kernelHostServiceMethod(params.Service)
	if !ok || params.Method != pluginhost.KernelServiceMethod {
		return nil, serviceError("method_not_found", "kernel service method is unavailable")
	}
	return services.invoke(ctx, method, params.Params)
}

func kernelHostServiceMethod(service string) (pluginhost.HostServiceMethod, bool) {
	methods := map[string]pluginhost.HostServiceMethod{
		pluginhost.KernelStorageGetService:             pluginhost.HostServiceStorageGet,
		pluginhost.KernelStorageSetService:             pluginhost.HostServiceStorageSet,
		pluginhost.KernelStorageDeleteService:          pluginhost.HostServiceStorageDelete,
		pluginhost.KernelStorageKeysService:            pluginhost.HostServiceStorageKeys,
		pluginhost.KernelStorageCompareExchangeService: pluginhost.HostServiceStorageCompareExchange,
		pluginhost.KernelSettingsGetService:            pluginhost.HostServiceSettingsGet,
		pluginhost.KernelSettingsListService:           pluginhost.HostServiceSettingsList,
		pluginhost.KernelSessionCreateService:          pluginhost.HostServiceSessionCreate,
		pluginhost.KernelSessionSendService:            pluginhost.HostServiceSessionSend,
		pluginhost.KernelSessionListService:            pluginhost.HostServiceSessionList,
		pluginhost.KernelSessionCancelService:          pluginhost.HostServiceSessionCancel,
	}
	method, ok := methods[service]
	return method, ok
}

func (s *pluginHostServices) storageGet(params pluginhost.StorageGetParams) (json.RawMessage, error) {
	scope, err := storageScope(params.Scope)
	if err != nil {
		return nil, err
	}
	if err := pluginsettings.ValidateStateKey(params.Key); err != nil {
		return nil, serviceError("invalid_params", err.Error())
	}
	document, err := pluginsettings.ReadState(s.wuuHome, s.projectRoot, s.subjectID, scope)
	if err != nil {
		return nil, err
	}
	value, ok := document.Values[params.Key]
	if !ok {
		return marshalServiceResult(pluginhost.StorageGetResult{})
	}
	return marshalServiceResult(pluginhost.StorageGetResult{Value: &value})
}

func (s *pluginHostServices) storageSet(params pluginhost.StorageSetParams) (json.RawMessage, error) {
	scope, err := storageScope(params.Scope)
	if err != nil {
		return nil, err
	}
	if err := pluginsettings.ValidateStateKey(params.Key); err != nil {
		return nil, serviceError("invalid_params", err.Error())
	}
	if len([]byte(params.Value)) > pluginsettings.MaxStateValueBytes {
		return nil, serviceError("limit_exceeded", fmt.Sprintf("plugin storage value exceeds %d bytes", pluginsettings.MaxStateValueBytes))
	}
	_, err = pluginsettings.UpdateState(s.wuuHome, s.projectRoot, s.subjectID, scope, func(values map[string]string) error {
		values[params.Key] = params.Value
		return nil
	})
	if err != nil {
		return nil, err
	}
	return marshalServiceResult(struct{}{})
}

func (s *pluginHostServices) storageDelete(params pluginhost.StorageDeleteParams) (json.RawMessage, error) {
	scope, err := storageScope(params.Scope)
	if err != nil {
		return nil, err
	}
	if err := pluginsettings.ValidateStateKey(params.Key); err != nil {
		return nil, serviceError("invalid_params", err.Error())
	}
	_, err = pluginsettings.UpdateState(s.wuuHome, s.projectRoot, s.subjectID, scope, func(values map[string]string) error {
		delete(values, params.Key)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return marshalServiceResult(struct{}{})
}

func (s *pluginHostServices) storageKeys(params pluginhost.StorageKeysParams) (json.RawMessage, error) {
	scope, err := storageScope(params.Scope)
	if err != nil {
		return nil, err
	}
	document, err := pluginsettings.ReadState(s.wuuHome, s.projectRoot, s.subjectID, scope)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(document.Values))
	for key := range document.Values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return marshalServiceResult(pluginhost.StorageKeysResult{Keys: keys})
}

func (s *pluginHostServices) storageCompareExchange(params pluginhost.StorageCompareExchangeParams) (json.RawMessage, error) {
	scope, err := storageScope(params.Scope)
	if err != nil {
		return nil, err
	}
	if err := pluginsettings.ValidateStateKey(params.Key); err != nil {
		return nil, serviceError("invalid_params", err.Error())
	}
	if params.Value != nil && len([]byte(*params.Value)) > pluginsettings.MaxStateValueBytes {
		return nil, serviceError("limit_exceeded", fmt.Sprintf("plugin storage value exceeds %d bytes", pluginsettings.MaxStateValueBytes))
	}
	result := pluginhost.StorageCompareExchangeResult{}
	_, err = pluginsettings.UpdateState(s.wuuHome, s.projectRoot, s.subjectID, scope, func(values map[string]string) error {
		current, exists := values[params.Key]
		matches := params.Expected != nil && exists && current == *params.Expected
		if params.Expected == nil {
			matches = !exists
		}
		if !matches {
			if exists {
				currentCopy := current
				result.Value = &currentCopy
			}
			return nil
		}
		result.Swapped = true
		if params.Value == nil {
			delete(values, params.Key)
			return nil
		}
		values[params.Key] = *params.Value
		valueCopy := *params.Value
		result.Value = &valueCopy
		return nil
	})
	if err != nil {
		return nil, err
	}
	return marshalServiceResult(result)
}

func storageScope(value string) (pluginsettings.Scope, error) {
	switch pluginsettings.Scope(strings.TrimSpace(value)) {
	case pluginsettings.ScopeUser:
		return pluginsettings.ScopeUser, nil
	case pluginsettings.ScopeWorkspace:
		return pluginsettings.ScopeWorkspace, nil
	default:
		return "", serviceError("invalid_params", "storage scope must be user or workspace")
	}
}

func decodeServiceParams(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return serviceError("invalid_params", "invalid host service parameters")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return serviceError("invalid_params", "invalid host service parameters")
	}
	return nil
}

func marshalServiceResult(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode host service result: %w", err)
	}
	return raw, nil
}

func serviceError(code, message string) error {
	return &pluginhost.HostServiceError{Code: code, Message: message}
}

func validateRuntimeSettingValue(definition pluginpkg.SettingDefinition, raw json.RawMessage) error {
	if len(raw) == 0 || len(raw) > 64<<10 || !json.Valid(raw) {
		return errors.New("value must be valid JSON within 65536 bytes")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return errors.New("value is invalid")
	}
	valid := false
	switch definition.Type {
	case pluginpkg.SettingTypeBoolean:
		_, valid = value.(bool)
	case pluginpkg.SettingTypeString:
		_, valid = value.(string)
	case pluginpkg.SettingTypeNumber:
		_, valid = value.(json.Number)
	case pluginpkg.SettingTypeEnum:
		text, ok := value.(string)
		for _, candidate := range definition.Enum {
			valid = valid || (ok && text == candidate)
		}
	}
	if !valid {
		return fmt.Errorf("value does not match declared type %q", definition.Type)
	}
	return nil
}
