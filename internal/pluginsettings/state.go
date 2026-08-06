package pluginsettings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blueberrycongee/wuu/internal/securefs"
	"github.com/blueberrycongee/wuu/internal/statepath"
	"github.com/blueberrycongee/wuu/internal/storelock"
)

const (
	stateDirectory     = "plugin-storage"
	maxStateEntries    = 256
	MaxStateKeyBytes   = 128
	MaxStateValueBytes = 1 << 20
)

type StateDocument struct {
	SchemaVersion int               `json:"schema_version"`
	PluginID      string            `json:"plugin_id"`
	Values        map[string]string `json:"values"`
}

func ValidateStateKey(key string) error {
	if len(key) > MaxStateKeyBytes || !safeNamePattern.MatchString(key) {
		return fmt.Errorf("invalid plugin storage key %q", key)
	}
	return nil
}

func ReadState(wuuHome, workspaceRoot, pluginID string, scope Scope) (StateDocument, error) {
	path, _, err := stateDocumentPath(wuuHome, workspaceRoot, pluginID, scope)
	if err != nil {
		return StateDocument{}, err
	}
	return readStateDocument(path, pluginID)
}

func UpdateState(wuuHome, workspaceRoot, pluginID string, scope Scope, update func(map[string]string) error) (StateDocument, error) {
	if update == nil {
		return StateDocument{}, errors.New("plugin storage update callback is required")
	}
	path, storeDir, err := stateDocumentPath(wuuHome, workspaceRoot, pluginID, scope)
	if err != nil {
		return StateDocument{}, err
	}
	lock, err := storelock.Acquire(storeDir)
	if err != nil {
		return StateDocument{}, fmt.Errorf("lock plugin storage: %w", err)
	}
	defer lock.Release() //nolint:errcheck -- the write result is authoritative
	document, err := readStateDocument(path, pluginID)
	if err != nil {
		return StateDocument{}, err
	}
	values := make(map[string]string, len(document.Values))
	for key, value := range document.Values {
		values[key] = value
	}
	if err := update(values); err != nil {
		return StateDocument{}, err
	}
	document = StateDocument{SchemaVersion: SchemaVersion, PluginID: pluginID, Values: values}
	if err := validateStateDocument(document, pluginID); err != nil {
		return StateDocument{}, err
	}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return StateDocument{}, fmt.Errorf("encode plugin storage: %w", err)
	}
	data = append(data, '\n')
	if len(data) > maxDocumentBytes+MaxStateValueBytes {
		return StateDocument{}, errors.New("plugin storage document exceeds the supported limit")
	}
	if err := securefs.WriteFileAtomic(path, data); err != nil {
		return StateDocument{}, fmt.Errorf("write plugin storage: %w", err)
	}
	return document, nil
}

func stateDocumentPath(wuuHome, workspaceRoot, pluginID string, scope Scope) (string, string, error) {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" || len(pluginID) > 256 {
		return "", "", fmt.Errorf("invalid plugin id %q", pluginID)
	}
	if strings.TrimSpace(wuuHome) == "" {
		return "", "", errors.New("Wuu home is required")
	}
	var storeDir string
	switch scope {
	case ScopeUser:
		storeDir = filepath.Join(wuuHome, stateDirectory, string(scope))
	case ScopeWorkspace:
		if strings.TrimSpace(workspaceRoot) == "" {
			return "", "", errors.New("workspace root is required for workspace plugin storage")
		}
		workspaceDir, err := statepath.WorkspaceDir(wuuHome, workspaceRoot)
		if err != nil {
			return "", "", fmt.Errorf("resolve plugin storage workspace: %w", err)
		}
		storeDir = filepath.Join(workspaceDir, stateDirectory)
	default:
		return "", "", fmt.Errorf("unsupported plugin storage scope %q", scope)
	}
	return filepath.Join(storeDir, pluginDocumentName(pluginID)), storeDir, nil
}

func readStateDocument(path, pluginID string) (StateDocument, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return StateDocument{SchemaVersion: SchemaVersion, PluginID: pluginID, Values: map[string]string{}}, nil
	}
	if err != nil {
		return StateDocument{}, fmt.Errorf("read plugin storage: %w", err)
	}
	var document StateDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return StateDocument{}, fmt.Errorf("decode plugin storage: %w", err)
	}
	if err := validateStateDocument(document, pluginID); err != nil {
		return StateDocument{}, err
	}
	return document, nil
}

func validateStateDocument(document StateDocument, pluginID string) error {
	if document.SchemaVersion != SchemaVersion || document.PluginID != pluginID {
		return errors.New("plugin storage ownership or schema mismatch")
	}
	if len(document.Values) > maxStateEntries {
		return fmt.Errorf("plugin storage contains more than %d values", maxStateEntries)
	}
	for key, value := range document.Values {
		if err := ValidateStateKey(key); err != nil {
			return err
		}
		if len([]byte(value)) > MaxStateValueBytes {
			return fmt.Errorf("plugin storage value %q exceeds %d bytes", key, MaxStateValueBytes)
		}
	}
	return nil
}
