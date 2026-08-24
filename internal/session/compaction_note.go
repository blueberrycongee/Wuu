package session

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// CompactionNote is one plugin-owned, session-scoped Markdown continuation
// document. CoveredMessages and CoveredHash anchor the document to the exact
// provider-history prefix it summarizes without exposing it as a chat message.
type CompactionNote struct {
	SessionID       string
	ProviderKey     string
	Markdown        string
	CoveredMessages int
	CoveredHash     string
	UpdatedAt       time.Time
}

// LoadCompactionNote returns the latest document for one compaction provider.
func LoadCompactionNote(sessDir, sessionID, providerKey string) (CompactionNote, bool, error) {
	sessionID = strings.TrimSpace(sessionID)
	providerKey = strings.TrimSpace(providerKey)
	if sessionID == "" || providerKey == "" {
		return CompactionNote{}, false, errors.New("session id and compaction provider key are required")
	}
	db, err := openStore(sessDir)
	if err != nil {
		return CompactionNote{}, false, err
	}
	defer db.Close()
	if ok, err := sessionExistsDB(db, sessionID); err != nil {
		return CompactionNote{}, false, err
	} else if !ok {
		return CompactionNote{}, false, fmt.Errorf("%w: %q", ErrSessionNotFound, sessionID)
	}
	var note CompactionNote
	var updated string
	err = db.QueryRow(`
		SELECT session_id, provider_key, markdown, covered_messages, covered_hash, updated_at
		FROM session_compaction_notes
		WHERE session_id = ? AND provider_key = ?`, sessionID, providerKey).Scan(
		&note.SessionID, &note.ProviderKey, &note.Markdown, &note.CoveredMessages, &note.CoveredHash, &updated,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return CompactionNote{}, false, nil
	}
	if err != nil {
		return CompactionNote{}, false, fmt.Errorf("load compaction note: %w", err)
	}
	note.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return note, true, nil
}

// StoreCompactionNote atomically replaces the active document for one
// provider. Old revisions are intentionally not accumulated.
func StoreCompactionNote(sessDir string, note CompactionNote) error {
	note.SessionID = strings.TrimSpace(note.SessionID)
	note.ProviderKey = strings.TrimSpace(note.ProviderKey)
	note.Markdown = strings.TrimSpace(note.Markdown)
	note.CoveredHash = strings.TrimSpace(note.CoveredHash)
	if note.SessionID == "" || note.ProviderKey == "" || note.Markdown == "" || note.CoveredMessages < 0 || note.CoveredHash == "" {
		return errors.New("compaction note requires session, provider, markdown, and a valid history anchor")
	}
	if note.UpdatedAt.IsZero() {
		note.UpdatedAt = time.Now().UTC()
	}
	db, err := openStore(sessDir)
	if err != nil {
		return err
	}
	defer db.Close()
	storeWriteMu.Lock()
	defer storeWriteMu.Unlock()
	_, err = db.Exec(`
		INSERT INTO session_compaction_notes (
			session_id, provider_key, markdown, covered_messages, covered_hash, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id, provider_key) DO UPDATE SET
			markdown = excluded.markdown,
			covered_messages = excluded.covered_messages,
			covered_hash = excluded.covered_hash,
			updated_at = excluded.updated_at`,
		note.SessionID, note.ProviderKey, note.Markdown, note.CoveredMessages, note.CoveredHash, note.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("store compaction note: %w", err)
	}
	return nil
}

// CopyCompactionNotesForFork copies only documents whose exact covered prefix
// is present in the fork. The caller supplies the fork's provider-history hash
// because session storage deliberately does not understand provider messages.
func CopyCompactionNotesForFork(sessDir, sourceID, forkID string, coveredHash func(int) (string, bool)) error {
	sourceID = strings.TrimSpace(sourceID)
	forkID = strings.TrimSpace(forkID)
	if sourceID == "" || forkID == "" || coveredHash == nil {
		return nil
	}
	db, err := openStore(sessDir)
	if err != nil {
		return err
	}
	rows, err := db.Query(`
		SELECT provider_key, markdown, covered_messages, covered_hash, updated_at
		FROM session_compaction_notes WHERE session_id = ?`, sourceID)
	if err != nil {
		db.Close()
		return fmt.Errorf("load source compaction notes: %w", err)
	}
	var notes []CompactionNote
	for rows.Next() {
		var note CompactionNote
		var updated string
		if err := rows.Scan(&note.ProviderKey, &note.Markdown, &note.CoveredMessages, &note.CoveredHash, &updated); err != nil {
			rows.Close()
			db.Close()
			return fmt.Errorf("scan source compaction note: %w", err)
		}
		if hash, ok := coveredHash(note.CoveredMessages); !ok || hash != note.CoveredHash {
			continue
		}
		note.SessionID = forkID
		note.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		notes = append(notes, note)
	}
	rowsErr := rows.Err()
	rows.Close()
	db.Close()
	if rowsErr != nil {
		return fmt.Errorf("iterate source compaction notes: %w", rowsErr)
	}
	for _, note := range notes {
		if err := StoreCompactionNote(sessDir, note); err != nil {
			return err
		}
	}
	return nil
}
