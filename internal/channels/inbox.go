package channels

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	checkLimit        = 50
	checkPreviewRunes = 80
)

func (s *Service) Check(ctx context.Context, agentID, token string) (CheckResult, error) {
	if _, err := s.AuthenticatePrincipal(ctx, agentID, token); err != nil {
		return CheckResult{}, err
	}
	return s.checkAgent(ctx, agentID)
}

func (s *Service) checkAgent(ctx context.Context, agentID string) (CheckResult, error) {
	agentID = strings.TrimSpace(agentID)
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CheckResult{}, fmt.Errorf("begin chat check: %w", err)
	}
	defer tx.Rollback()
	checkedAt := fromMillis(toMillis(s.now()))
	if _, err := tx.ExecContext(ctx, `
		UPDATE drafts SET state = 'expired', updated_at = ?
		WHERE state = 'held' AND updated_at < ?`,
		toMillis(checkedAt), toMillis(checkedAt.Add(-DraftExpiry))); err != nil {
		return CheckResult{}, fmt.Errorf("expire held drafts during chat check: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT inbox.id, COALESCE(inbox.room_id, ''), COALESCE(inbox.message_id, ''),
			COALESCE(inbox.reminder_id, ''), inbox.kind, inbox.created_at,
			COALESCE(message.thread_id, ''), COALESCE(message.author_type, ''),
			COALESCE(message.author_id, ''), COALESCE(message.body, ''), COALESCE(message.seq, 0),
			COALESCE(reminder.agent_id, ''), COALESCE(reminder.fire_at, 0),
			COALESCE(reminder.note, ''), COALESCE(reminder.room_id, ''),
			COALESCE(reminder.thread_id, ''), COALESCE(reminder.created_at, 0)
		FROM inbox_items inbox
		LEFT JOIN room_messages message ON message.id = inbox.message_id
		LEFT JOIN reminders reminder ON reminder.id = inbox.reminder_id
		WHERE inbox.member_type = 'agent' AND inbox.member_id = ? AND inbox.pulled_at IS NULL
		ORDER BY inbox.created_at, inbox.rowid LIMIT ?`, agentID, checkLimit+1)
	if err != nil {
		return CheckResult{}, fmt.Errorf("query chat check inbox: %w", err)
	}
	items := make([]CheckItem, 0, checkLimit)
	reminders := make([]Reminder, 0, checkLimit)
	inboxIDs := make([]string, 0, checkLimit)
	hasMore := false
	rowCount := 0
	for rows.Next() {
		var (
			inboxID, roomID, messageID, reminderID, kind   string
			threadID, authorType, authorID, body           string
			reminderAgentID, reminderNote                  string
			reminderRoomID, reminderThreadID               string
			inboxCreatedAt, seq, fireAt, reminderCreatedAt int64
		)
		if err := rows.Scan(
			&inboxID, &roomID, &messageID, &reminderID, &kind, &inboxCreatedAt,
			&threadID, &authorType, &authorID, &body, &seq,
			&reminderAgentID, &fireAt, &reminderNote, &reminderRoomID,
			&reminderThreadID, &reminderCreatedAt,
		); err != nil {
			rows.Close()
			return CheckResult{}, fmt.Errorf("scan chat check inbox: %w", err)
		}
		rowCount++
		if rowCount > checkLimit {
			hasMore = true
			continue
		}
		inboxIDs = append(inboxIDs, inboxID)
		if reminderID != "" {
			reminders = append(reminders, Reminder{
				ID: reminderID, AgentID: reminderAgentID, FireAt: fromMillis(fireAt),
				Note: reminderNote, RoomID: reminderRoomID, ThreadID: reminderThreadID,
				State: ReminderFired, CreatedAt: fromMillis(reminderCreatedAt),
			})
			continue
		}
		items = append(items, CheckItem{
			ID: inboxID, RoomID: roomID, MessageID: messageID, ThreadID: threadID,
			AuthorType: MemberType(authorType), AuthorID: authorID, Kind: InboxKind(kind),
			Preview: preview(body, checkPreviewRunes), Seq: seq, CreatedAt: fromMillis(inboxCreatedAt),
		})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return CheckResult{}, fmt.Errorf("iterate chat check inbox: %w", err)
	}
	if err := rows.Close(); err != nil {
		return CheckResult{}, fmt.Errorf("close chat check inbox: %w", err)
	}
	for _, id := range inboxIDs {
		if _, err := tx.ExecContext(ctx, `
			UPDATE inbox_items SET pulled_at = ? WHERE id = ? AND member_type = 'agent' AND member_id = ?`,
			toMillis(checkedAt), id, agentID); err != nil {
			return CheckResult{}, fmt.Errorf("mark chat check inbox pulled: %w", err)
		}
	}
	collaboration := make([]CollaborationMessage, 0, checkLimit)
	collaborationIDs := make([]string, 0, checkLimit)
	rows, err = tx.QueryContext(ctx, `
		SELECT delivery.id, delivery.room_id, delivery.from_type, delivery.from_id, delivery.to_agent_id,
			CASE WHEN principal.kind = 'named_agent' THEN delivery.to_agent_id ELSE '' END,
			delivery.kind, delivery.body, COALESCE(delivery.work_id, ''),
			COALESCE(delivery.source_message_id, ''), delivery.goal_revision, delivery.candidate_revision,
			delivery.artifact_refs_json, COALESCE(delivery.reply_to, ''), delivery.created_at
		FROM collaboration_messages delivery
		JOIN collaboration_principals principal ON principal.id = delivery.to_agent_id
		WHERE delivery.to_agent_id = ? AND delivery.pulled_at IS NULL AND delivery.invalidated_at IS NULL
		ORDER BY delivery.created_at, delivery.rowid LIMIT ?`, agentID, checkLimit+1)
	if err != nil {
		return CheckResult{}, fmt.Errorf("query collaboration inbox: %w", err)
	}
	rowCount = 0
	for rows.Next() {
		var message CollaborationMessage
		var createdAt int64
		var artifactRefsJSON string
		if err := rows.Scan(
			&message.ID, &message.RoomID, &message.FromType, &message.FromID, &message.ToAgentID,
			&message.RecipientNamedAgentID, &message.Kind, &message.Body, &message.WorkID,
			&message.SourceMessageID, &message.GoalRevision, &message.CandidateRevision,
			&artifactRefsJSON, &message.ReplyTo, &createdAt,
		); err != nil {
			rows.Close()
			return CheckResult{}, fmt.Errorf("scan collaboration inbox: %w", err)
		}
		rowCount++
		if rowCount > checkLimit {
			hasMore = true
			continue
		}
		message.CreatedAt = fromMillis(createdAt)
		if strings.TrimSpace(message.FromID) == "" {
			// Host-generated internal envelopes have no hidden author identity.
			message.FromType = ""
		}
		if err := json.Unmarshal([]byte(artifactRefsJSON), &message.ArtifactRefs); err != nil {
			rows.Close()
			return CheckResult{}, fmt.Errorf("decode collaboration artifacts: %w", err)
		}
		collaboration = append(collaboration, message)
		collaborationIDs = append(collaborationIDs, message.ID)
	}
	if err := rows.Close(); err != nil {
		return CheckResult{}, fmt.Errorf("close collaboration inbox: %w", err)
	}
	if err := rows.Err(); err != nil {
		return CheckResult{}, fmt.Errorf("iterate collaboration inbox: %w", err)
	}
	for _, id := range collaborationIDs {
		if _, err := tx.ExecContext(ctx, `UPDATE collaboration_messages SET pulled_at = ?, consumed_at = ? WHERE id = ? AND to_agent_id = ?`, toMillis(checkedAt), toMillis(checkedAt), id, agentID); err != nil {
			return CheckResult{}, fmt.Errorf("mark collaboration message pulled: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE room_cursors
		SET last_read_seq = COALESCE((SELECT MAX(message.seq) FROM room_messages message WHERE message.room_id = room_cursors.room_id), 0)
		WHERE member_type = 'agent' AND member_id = ?`, agentID); err != nil {
		return CheckResult{}, fmt.Errorf("advance chat check cursors: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE agent_wake_state SET outstanding = 0, pending = 0, updated_at = ? WHERE agent_id = ?`,
		toMillis(checkedAt), agentID); err != nil {
		return CheckResult{}, fmt.Errorf("clear chat check wake: %w", err)
	}
	scopes, err := checkScopesTx(ctx, tx, agentID, items)
	if err != nil {
		return CheckResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return CheckResult{}, fmt.Errorf("commit chat check: %w", err)
	}
	return CheckResult{Items: items, Collaboration: collaboration, Reminders: reminders, Scopes: scopes, HasMore: hasMore, CheckedAt: checkedAt}, nil
}

func (s *Service) ReadInboxMessages(ctx context.Context, agentID, token string, itemIDs []string) ([]Message, error) {
	if _, err := s.AuthenticatePrincipal(ctx, agentID, token); err != nil {
		return nil, err
	}
	return s.readInboxMessages(ctx, agentID, itemIDs)
}

func (s *Service) readInboxMessages(ctx context.Context, agentID string, itemIDs []string) ([]Message, error) {
	if len(itemIDs) == 0 || len(itemIDs) > checkLimit {
		return nil, fmt.Errorf("chat read requires 1-%d inbox item ids", checkLimit)
	}
	messages := make([]Message, 0, len(itemIDs))
	seen := make(map[string]struct{}, len(itemIDs))
	for _, itemID := range itemIDs {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" {
			return nil, errors.New("chat read inbox item id is required")
		}
		if _, ok := seen[itemID]; ok {
			continue
		}
		seen[itemID] = struct{}{}
		message, err := scanMessage(s.db.QueryRowContext(ctx, `
			SELECT message.id, message.room_id, message.seq, COALESCE(message.thread_id, ''),
				message.author_type, message.author_id, message.kind, message.body, message.images_json, message.files_json, message.mentions_json,
				COALESCE(message.reply_to, ''), COALESCE(message.task_title, ''), COALESCE(message.task_state, ''),
				COALESCE(message.task_owner, ''), message.task_verification_required,
				message.task_goal_revision, message.task_candidate_revision, message.created_at
			FROM inbox_items inbox
			JOIN room_messages message ON message.id = inbox.message_id
			WHERE inbox.id = ? AND inbox.member_type = 'agent' AND inbox.member_id = ?`, itemID, strings.TrimSpace(agentID)))
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("%w: inbox item %q", ErrNotFound, itemID)
		}
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func (s *Service) ReadAgentMessages(ctx context.Context, agentID, token, roomID string, afterSeq int64, limit int) ([]Message, error) {
	if _, err := s.AuthenticatePrincipal(ctx, agentID, token); err != nil {
		return nil, err
	}
	return s.readAgentMessages(ctx, agentID, roomID, afterSeq, limit)
}

func (s *Service) readAgentMessages(ctx context.Context, agentID, roomID string, afterSeq int64, limit int) ([]Message, error) {
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return nil, errors.New("chat read room is required")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin chat read access check: %w", err)
	}
	defer tx.Rollback()
	if err := requireRoomPrincipalAccessTx(ctx, tx, roomID, strings.TrimSpace(agentID)); err != nil {
		return nil, err
	}
	if err := tx.Rollback(); err != nil {
		return nil, fmt.Errorf("finish chat read access check: %w", err)
	}
	return s.ListMessages(ctx, roomID, afterSeq, limit)
}

func checkScopesTx(ctx context.Context, tx *sql.Tx, agentID string, items []CheckItem) ([]ScopeSequence, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT cursor.room_id, COALESCE(MAX(message.seq), 0)
		FROM room_cursors cursor
		LEFT JOIN room_messages message ON message.room_id = cursor.room_id AND message.thread_id IS NULL
		WHERE cursor.member_type = 'agent' AND cursor.member_id = ?
		GROUP BY cursor.room_id ORDER BY cursor.room_id`, agentID)
	if err != nil {
		return nil, fmt.Errorf("query chat check room scopes: %w", err)
	}
	defer rows.Close()
	scopes := make([]ScopeSequence, 0)
	for rows.Next() {
		var scope ScopeSequence
		if err := rows.Scan(&scope.RoomID, &scope.Seq); err != nil {
			return nil, fmt.Errorf("scan chat check room scope: %w", err)
		}
		scopes = append(scopes, scope)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat check room scopes: %w", err)
	}
	threads := make(map[string]ScopeSequence)
	for _, item := range items {
		if item.ThreadID == "" {
			continue
		}
		key := item.RoomID + "\x00" + item.ThreadID
		threads[key] = ScopeSequence{RoomID: item.RoomID, ThreadID: item.ThreadID}
	}
	rows, err = tx.QueryContext(ctx, `
		SELECT DISTINCT message.room_id,
			CASE WHEN message.thread_id IS NULL THEN message.id ELSE message.thread_id END
		FROM room_messages message
		JOIN room_members member ON member.room_id = message.room_id
		WHERE member.member_type = 'agent' AND member.member_id = ?
			AND message.author_type = 'agent' AND message.author_id = ?
			AND (message.thread_id IS NOT NULL OR EXISTS (
				SELECT 1 FROM room_messages reply
				WHERE reply.room_id = message.room_id AND reply.thread_id = message.id
			))`, agentID, agentID)
	if err != nil {
		return nil, fmt.Errorf("query chat check active threads: %w", err)
	}
	for rows.Next() {
		var scope ScopeSequence
		if err := rows.Scan(&scope.RoomID, &scope.ThreadID); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan chat check active thread: %w", err)
		}
		threads[scope.RoomID+"\x00"+scope.ThreadID] = scope
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate chat check active threads: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close chat check active threads: %w", err)
	}
	keys := make([]string, 0, len(threads))
	for key := range threads {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		scope := threads[key]
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(seq), 0) FROM room_messages WHERE room_id = ? AND (id = ? OR thread_id = ?)`,
			scope.RoomID, scope.ThreadID, scope.ThreadID).Scan(&scope.Seq); err != nil {
			return nil, fmt.Errorf("query chat check thread scope: %w", err)
		}
		scopes = append(scopes, scope)
	}
	return scopes, nil
}

func preview(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:limit])) + "…"
}
