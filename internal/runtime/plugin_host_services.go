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

var productionPluginHostServices = pluginhost.AllHostServices()

// pluginHostServices is bound to exactly one installed plugin generation. It
// never accepts a caller-supplied plugin ID or filesystem path: ownership and
// roots are captured from the validated plugin inventory at activation time.
type pluginHostServices struct {
	mu              sync.RWMutex
	active          bool
	pluginID        string
	subjectID       string
	fingerprint     string
	projectRoot     string
	wuuHome         string
	settings        map[string]pluginpkg.SettingDefinition
	turnRouter      *PluginSessionRouter
	serviceRegistry *pluginhost.ServiceRegistry
}

func newPluginHostServices(item pluginpkg.Plugin, projectRoot, wuuHome string, turnRouter *PluginSessionRouter) *pluginHostServices {
	settings := make(map[string]pluginpkg.SettingDefinition, len(item.Settings))
	for key, definition := range item.Settings {
		settings[key] = definition
	}
	services := &pluginHostServices{
		active: true, pluginID: item.ID, subjectID: strings.TrimSpace(item.SubjectID),
		fingerprint: item.Fingerprint, projectRoot: projectRoot, wuuHome: wuuHome,
		settings: settings, turnRouter: turnRouter,
	}
	return services
}

func (s *pluginHostServices) SupportedHostServices() []pluginhost.HostServiceMethod {
	if s == nil || strings.TrimSpace(s.subjectID) == "" || strings.TrimSpace(s.fingerprint) == "" || strings.TrimSpace(s.wuuHome) == "" {
		return nil
	}
	return append([]pluginhost.HostServiceMethod(nil), productionPluginHostServices...)
}

func (s *pluginHostServices) CloseHostServices() {
	s.mu.Lock()
	s.active = false
	s.mu.Unlock()
}

// setServiceRegistry binds the generation's service registry after every
// plugin process of the generation has completed initialize. Calls only flow
// once the generation is active; the registry enforces that itself.
func (s *pluginHostServices) setServiceRegistry(registry *pluginhost.ServiceRegistry) {
	s.mu.Lock()
	s.serviceRegistry = registry
	s.mu.Unlock()
}

// RouteServiceCall implements pluginhost.ServiceRouter: the transport passes
// the caller identity it authenticated at process start, and every
// authorization decision belongs to the registry.
func (s *pluginHostServices) RouteServiceCall(ctx context.Context, pluginID string, params pluginhost.ServiceCallParams) (json.RawMessage, *pluginhost.HostServiceError) {
	if s == nil {
		return nil, &pluginhost.HostServiceError{Code: "service_unavailable", Message: "service routing is unavailable"}
	}
	s.mu.RLock()
	active := s.active
	registry := s.serviceRegistry
	s.mu.RUnlock()
	if !active || registry == nil {
		return nil, &pluginhost.HostServiceError{Code: "service_unavailable", Message: "service routing is unavailable for this plugin generation"}
	}
	return registry.Call(ctx, pluginID, params)
}

func (s *pluginHostServices) HandleHostService(ctx context.Context, method pluginhost.HostServiceMethod, raw json.RawMessage) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.active {
		return nil, serviceError("generation_closed", "plugin generation is no longer active")
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
