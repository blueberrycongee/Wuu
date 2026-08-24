package channels

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

func (s *Service) SendCollaboration(ctx context.Context, params CollaborationSendParams) (CollaborationMessage, error) {
	if _, err := s.AuthenticateAgent(ctx, params.AgentID, params.Token); err != nil {
		return CollaborationMessage{}, err
	}
	params.RoomID = strings.TrimSpace(params.RoomID)
	params.ToAgentID = strings.TrimSpace(params.ToAgentID)
	params.Body = strings.TrimSpace(params.Body)
	params.ReplyTo = strings.TrimSpace(params.ReplyTo)
	if params.RoomID == "" || params.ToAgentID == "" || params.Body == "" {
		return CollaborationMessage{}, errors.New("collaboration room, recipient, and body are required")
	}
	if params.ToAgentID == params.AgentID {
		return CollaborationMessage{}, errors.New("collaboration recipient must be another agent")
	}
	if utf8.RuneCountInString(params.Body) > MaxMessageRunes {
		return CollaborationMessage{}, fmt.Errorf("collaboration message exceeds %d characters", MaxMessageRunes)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CollaborationMessage{}, fmt.Errorf("begin collaboration send: %w", err)
	}
	defer tx.Rollback()
	for _, agentID := range []string{params.AgentID, params.ToAgentID} {
		if err := requireMemberTx(ctx, tx, params.RoomID, MemberAgent, agentID); err != nil {
			return CollaborationMessage{}, err
		}
	}
	if params.ReplyTo != "" {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM collaboration_messages WHERE id = ? AND room_id = ?`, params.ReplyTo, params.RoomID).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return CollaborationMessage{}, fmt.Errorf("%w: collaboration message %q", ErrNotFound, params.ReplyTo)
			}
			return CollaborationMessage{}, fmt.Errorf("validate collaboration reply: %w", err)
		}
	}
	now := fromMillis(toMillis(s.now()))
	message, err := enqueueCollaborationTx(ctx, tx, CollaborationMessage{
		RoomID: params.RoomID, FromType: MemberAgent, FromID: params.AgentID,
		ToAgentID: params.ToAgentID, Body: params.Body, ReplyTo: params.ReplyTo, CreatedAt: now,
	})
	if err != nil {
		return CollaborationMessage{}, err
	}
	shouldDeliver, err := requestWakeTx(ctx, tx, params.ToAgentID, toMillis(now))
	if err != nil {
		return CollaborationMessage{}, err
	}
	if err := tx.Commit(); err != nil {
		return CollaborationMessage{}, fmt.Errorf("commit collaboration send: %w", err)
	}
	if shouldDeliver && s.wake != nil {
		s.wake.Deliver(params.ToAgentID)
	}
	return message, nil
}

func enqueueCollaborationTx(ctx context.Context, tx *sql.Tx, message CollaborationMessage) (CollaborationMessage, error) {
	if message.ID == "" {
		id, err := randomID("collab", 12)
		if err != nil {
			return CollaborationMessage{}, err
		}
		message.ID = id
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO collaboration_messages (
			id, room_id, from_type, from_id, to_agent_id, body, source_message_id, reply_to, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		message.ID, message.RoomID, message.FromType, message.FromID, message.ToAgentID, message.Body,
		nullableString(message.SourceMessageID), nullableString(message.ReplyTo), toMillis(message.CreatedAt))
	if err != nil {
		return CollaborationMessage{}, fmt.Errorf("enqueue collaboration message: %w", err)
	}
	return message, nil
}

func (s *Service) PendingCollaborationRoomIDs(ctx context.Context, agentID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT room_id FROM collaboration_messages
		WHERE to_agent_id = ? AND pulled_at IS NULL ORDER BY room_id`, strings.TrimSpace(agentID))
	if err != nil {
		return nil, fmt.Errorf("list pending collaboration rooms: %w", err)
	}
	defer rows.Close()
	roomIDs := make([]string, 0)
	for rows.Next() {
		var roomID string
		if err := rows.Scan(&roomID); err != nil {
			return nil, fmt.Errorf("scan pending collaboration room: %w", err)
		}
		roomIDs = append(roomIDs, roomID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending collaboration rooms: %w", err)
	}
	return roomIDs, nil
}
