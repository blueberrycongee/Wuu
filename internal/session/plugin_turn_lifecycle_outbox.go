package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// PluginTurnLifecycleOutboxEntry is one owner-scoped terminal turn event that
// remains pending until the target plugin acknowledges delivery.
type PluginTurnLifecycleOutboxEntry struct {
	PluginID  string
	RequestID string
	Payload   json.RawMessage
	UpdatedAt time.Time
}

// PutPluginTurnLifecycleOutbox stores the terminal event before delivery. The
// owner/request identity is stable, so retrying the same event is idempotent.
func PutPluginTurnLifecycleOutbox(sessDir, pluginID, requestID string, payload json.RawMessage) error {
	pluginID = strings.TrimSpace(pluginID)
	requestID = strings.TrimSpace(requestID)
	if pluginID == "" || requestID == "" {
		return errors.New("plugin lifecycle outbox requires plugin_id and request_id")
	}
	if !json.Valid(payload) {
		return errors.New("plugin lifecycle outbox payload is invalid JSON")
	}
	db, err := openStore(sessDir)
	if err != nil {
		return err
	}
	defer db.Close()

	storeWriteMu.Lock()
	defer storeWriteMu.Unlock()
	_, err = db.Exec(`
INSERT INTO plugin_turn_lifecycle_outbox (plugin_id, request_id, payload_json, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(plugin_id, request_id) DO UPDATE SET
    payload_json = excluded.payload_json,
    updated_at = excluded.updated_at`, pluginID, requestID, string(payload), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("store plugin lifecycle outbox event: %w", err)
	}
	return nil
}

// DeletePluginTurnLifecycleOutbox acknowledges one delivered terminal event.
func DeletePluginTurnLifecycleOutbox(sessDir, pluginID, requestID string) error {
	pluginID = strings.TrimSpace(pluginID)
	requestID = strings.TrimSpace(requestID)
	if pluginID == "" || requestID == "" {
		return errors.New("plugin lifecycle outbox requires plugin_id and request_id")
	}
	db, err := openStore(sessDir)
	if err != nil {
		return err
	}
	defer db.Close()

	storeWriteMu.Lock()
	defer storeWriteMu.Unlock()
	if _, err := db.Exec(`DELETE FROM plugin_turn_lifecycle_outbox WHERE plugin_id = ? AND request_id = ?`, pluginID, requestID); err != nil {
		return fmt.Errorf("acknowledge plugin lifecycle outbox event: %w", err)
	}
	return nil
}

// DeletePluginTurnLifecycleOutboxForPlugin removes dormant delivery state when
// its owning plugin is permanently uninstalled.
func DeletePluginTurnLifecycleOutboxForPlugin(sessDir, pluginID string) error {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return errors.New("plugin lifecycle outbox requires plugin_id")
	}
	db, err := openStore(sessDir)
	if err != nil {
		return err
	}
	defer db.Close()

	storeWriteMu.Lock()
	defer storeWriteMu.Unlock()
	if _, err := db.Exec(`DELETE FROM plugin_turn_lifecycle_outbox WHERE plugin_id = ?`, pluginID); err != nil {
		return fmt.Errorf("delete plugin lifecycle outbox events: %w", err)
	}
	return nil
}

// ListPluginTurnLifecycleOutbox returns pending events without creating a
// missing store. Startup recovery can therefore remain a read-only probe.
func ListPluginTurnLifecycleOutbox(sessDir string) ([]PluginTurnLifecycleOutboxEntry, error) {
	db, ok, err := openStoreForScan(sessDir)
	if err != nil || !ok {
		return nil, err
	}
	defer db.Close()
	exists, err := storeTableExists(db, "plugin_turn_lifecycle_outbox")
	if err != nil || !exists {
		return nil, err
	}
	rows, err := db.Query(`
SELECT plugin_id, request_id, payload_json, updated_at
FROM plugin_turn_lifecycle_outbox
ORDER BY updated_at, plugin_id, request_id`)
	if err != nil {
		return nil, fmt.Errorf("list plugin lifecycle outbox events: %w", err)
	}
	defer rows.Close()

	var entries []PluginTurnLifecycleOutboxEntry
	for rows.Next() {
		var entry PluginTurnLifecycleOutboxEntry
		var payload, updatedAt string
		if err := rows.Scan(&entry.PluginID, &entry.RequestID, &payload, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan plugin lifecycle outbox event: %w", err)
		}
		if !json.Valid([]byte(payload)) {
			return nil, fmt.Errorf("plugin lifecycle outbox event %q/%q contains invalid JSON", entry.PluginID, entry.RequestID)
		}
		entry.Payload = json.RawMessage(payload)
		entry.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse plugin lifecycle outbox timestamp: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plugin lifecycle outbox events: %w", err)
	}
	return entries, nil
}
