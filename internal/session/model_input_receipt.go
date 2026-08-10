package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type ModelInputReceiptRecord struct {
	OperationID     string
	ContractVersion int
	DriverID        string
	DriverVersion   string
	Payload         json.RawMessage
	CreatedAt       time.Time
}

func SaveModelInputReceipt(sessDir, sessionID string, record ModelInputReceiptRecord) error {
	sessionID = strings.TrimSpace(sessionID)
	record.OperationID = strings.TrimSpace(record.OperationID)
	record.DriverID = strings.TrimSpace(record.DriverID)
	record.DriverVersion = strings.TrimSpace(record.DriverVersion)
	if sessionID == "" || record.OperationID == "" || record.DriverID == "" || record.DriverVersion == "" || record.ContractVersion < 1 {
		return errors.New("complete model input receipt identity is required")
	}
	if len(record.Payload) == 0 || !json.Valid(record.Payload) {
		return errors.New("model input receipt payload must be valid JSON")
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}

	db, err := openStore(sessDir)
	if err != nil {
		return err
	}
	defer db.Close()
	storeWriteMu.Lock()
	defer storeWriteMu.Unlock()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin model input receipt write: %w", err)
	}
	defer tx.Rollback()
	if ok, err := sessionExistsTx(tx, sessionID); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("%w: %q", ErrSessionNotFound, sessionID)
	}
	result, err := tx.Exec(`
INSERT OR IGNORE INTO session_model_input_receipts (
    session_id, operation_id, contract_version, driver_id, driver_version, receipt_json, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sessionID,
		record.OperationID,
		record.ContractVersion,
		record.DriverID,
		record.DriverVersion,
		string(record.Payload),
		timeText(record.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("write model input receipt: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect model input receipt write: %w", err)
	}
	if inserted == 0 {
		var contractVersion int
		var driverID, driverVersion, payload string
		if err := tx.QueryRow(`
SELECT contract_version, driver_id, driver_version, receipt_json
FROM session_model_input_receipts
WHERE session_id = ? AND operation_id = ?`, sessionID, record.OperationID).Scan(
			&contractVersion,
			&driverID,
			&driverVersion,
			&payload,
		); err != nil {
			return fmt.Errorf("load existing model input receipt: %w", err)
		}
		if contractVersion != record.ContractVersion || driverID != record.DriverID || driverVersion != record.DriverVersion || payload != string(record.Payload) {
			return fmt.Errorf("model input receipt %q changed after persistence", record.OperationID)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit model input receipt: %w", err)
	}
	return nil
}

func LoadModelInputReceipts(sessDir, sessionID string) ([]ModelInputReceiptRecord, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("model input receipt session id is required")
	}
	db, err := openStore(sessDir)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if ok, err := sessionExistsDB(db, sessionID); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("%w: %q", ErrSessionNotFound, sessionID)
	}
	rows, err := db.Query(`
SELECT operation_id, contract_version, driver_id, driver_version, receipt_json, created_at
FROM session_model_input_receipts
WHERE session_id = ?
ORDER BY created_at, operation_id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load model input receipts: %w", err)
	}
	defer rows.Close()
	var records []ModelInputReceiptRecord
	for rows.Next() {
		var record ModelInputReceiptRecord
		var payload, createdAt string
		if err := rows.Scan(
			&record.OperationID,
			&record.ContractVersion,
			&record.DriverID,
			&record.DriverVersion,
			&payload,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan model input receipt: %w", err)
		}
		record.Payload = json.RawMessage(payload)
		record.CreatedAt = parseTime(createdAt)
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load model input receipts: %w", err)
	}
	return records, nil
}
