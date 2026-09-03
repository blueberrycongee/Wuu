package session

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// HistoryPage is a bounded, stable view of the physical append-only transcript.
// Records never come from a provider-history checkpoint: Seq always addresses
// session_messages directly.
type HistoryPage struct {
	Records []HistoryRecord
	HeadSeq int
	HasMore bool
}

// ReadHistoryPage reads non-meta transcript records at or after startSeq in
// ascending order. limit bounds database and caller memory use.
func ReadHistoryPage(ctx context.Context, sessDir, id string, startSeq, limit int) (HistoryPage, error) {
	if startSeq < 1 {
		return HistoryPage{}, errors.New("history start sequence must be positive")
	}
	if limit < 1 {
		return HistoryPage{}, errors.New("history page limit must be positive")
	}
	db, err := openStore(sessDir)
	if err != nil {
		return HistoryPage{}, err
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return HistoryPage{}, fmt.Errorf("begin history read: %w", err)
	}
	defer tx.Rollback()
	if ok, err := sessionExistsTx(tx, strings.TrimSpace(id)); err != nil {
		return HistoryPage{}, err
	} else if !ok {
		return HistoryPage{}, fmt.Errorf("%w: %q", ErrSessionNotFound, strings.TrimSpace(id))
	}
	page, err := readHistoryPageTx(tx, strings.TrimSpace(id), startSeq, limit)
	if err != nil {
		return HistoryPage{}, err
	}
	if err := tx.Commit(); err != nil {
		return HistoryPage{}, fmt.Errorf("commit history read: %w", err)
	}
	return page, nil
}

// SearchHistoryPage searches model-visible text and structured tool payloads in
// the current session's physical transcript. Results are newest-first. beforeSeq
// is an exclusive cursor; zero starts from the current head.
func SearchHistoryPage(ctx context.Context, sessDir, id, query string, beforeSeq, limit int) (HistoryPage, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return HistoryPage{}, errors.New("history search query is required")
	}
	if beforeSeq < 0 {
		return HistoryPage{}, errors.New("history search cursor must be non-negative")
	}
	if limit < 1 {
		return HistoryPage{}, errors.New("history search limit must be positive")
	}
	db, err := openStore(sessDir)
	if err != nil {
		return HistoryPage{}, err
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return HistoryPage{}, fmt.Errorf("begin history search: %w", err)
	}
	defer tx.Rollback()
	id = strings.TrimSpace(id)
	if ok, err := sessionExistsTx(tx, id); err != nil {
		return HistoryPage{}, err
	} else if !ok {
		return HistoryPage{}, fmt.Errorf("%w: %q", ErrSessionNotFound, id)
	}

	var headSeq int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) FROM session_messages WHERE session_id = ?`, id).Scan(&headSeq); err != nil {
		return HistoryPage{}, fmt.Errorf("load history search head: %w", err)
	}
	upper := beforeSeq
	if upper == 0 || upper > headSeq+1 {
		upper = headSeq + 1
	}
	searchable := `lower(
		coalesce(content, '') || char(10) || coalesce(display_content, '') || char(10) ||
		coalesce(reasoning_content, '') || char(10) || coalesce(tool_calls_json, '') || char(10) ||
		coalesce(tool_result_json, '') || char(10) || coalesce(name, '')
	)`
	rows, err := tx.QueryContext(ctx, historyRecordsSelect+`
		WHERE session_id = ? AND lower(role) <> 'meta' AND seq < ?
		  AND instr(`+searchable+`, lower(?)) > 0
		ORDER BY seq DESC LIMIT ?`, id, upper, query, limit+1)
	if err != nil {
		return HistoryPage{}, fmt.Errorf("search session history: %w", err)
	}
	records, err := scanHistoryRecords(rows)
	if err != nil {
		return HistoryPage{}, err
	}
	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}
	if err := tx.Commit(); err != nil {
		return HistoryPage{}, fmt.Errorf("commit history search: %w", err)
	}
	return HistoryPage{Records: records, HeadSeq: headSeq, HasMore: hasMore}, nil
}

func readHistoryPageTx(tx *sql.Tx, id string, startSeq, limit int) (HistoryPage, error) {
	var headSeq int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) FROM session_messages WHERE session_id = ?`, id).Scan(&headSeq); err != nil {
		return HistoryPage{}, fmt.Errorf("load history read head: %w", err)
	}
	rows, err := tx.Query(historyRecordsSelect+`
		WHERE session_id = ? AND lower(role) <> 'meta' AND seq >= ?
		ORDER BY seq ASC LIMIT ?`, id, startSeq, limit+1)
	if err != nil {
		return HistoryPage{}, fmt.Errorf("read session history: %w", err)
	}
	records, err := scanHistoryRecords(rows)
	if err != nil {
		return HistoryPage{}, err
	}
	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}
	return HistoryPage{Records: records, HeadSeq: headSeq, HasMore: hasMore}, nil
}
