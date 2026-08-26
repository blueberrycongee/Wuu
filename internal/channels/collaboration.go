package channels

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

func (s *Service) SendCollaboration(ctx context.Context, params CollaborationSendParams) (CollaborationMessage, error) {
	if _, err := s.AuthenticatePrincipal(ctx, params.AgentID, params.Token); err != nil {
		return CollaborationMessage{}, err
	}
	params.RoomID = strings.TrimSpace(params.RoomID)
	params.ToAgentID = strings.TrimSpace(params.ToAgentID)
	params.Body = strings.TrimSpace(params.Body)
	params.SourceMessageID = strings.TrimSpace(params.SourceMessageID)
	params.ReplyTo = strings.TrimSpace(params.ReplyTo)
	for index := range params.ArtifactRefs {
		params.ArtifactRefs[index] = strings.TrimSpace(params.ArtifactRefs[index])
	}
	if params.RoomID == "" || params.Body == "" {
		return CollaborationMessage{}, errors.New("collaboration room and body are required")
	}
	if utf8.RuneCountInString(params.Body) > MaxMessageRunes {
		return CollaborationMessage{}, fmt.Errorf("collaboration message exceeds %d characters", MaxMessageRunes)
	}
	if params.Kind == "" {
		params.Kind = CollaborationControl
	}
	if params.Kind != CollaborationControl && params.Kind != CollaborationCandidateReady && params.Kind != CollaborationPeerResult {
		return CollaborationMessage{}, fmt.Errorf("invalid collaboration kind %q", params.Kind)
	}
	if params.Kind == CollaborationControl && params.ToAgentID == "" {
		return CollaborationMessage{}, errors.New("control delivery recipient is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CollaborationMessage{}, fmt.Errorf("begin collaboration send: %w", err)
	}
	defer tx.Rollback()
	implicitRoomRecipient := false
	if (params.Kind == CollaborationCandidateReady || params.Kind == CollaborationPeerResult) && params.ToAgentID == "" {
		if err := tx.QueryRowContext(ctx, `SELECT id FROM room_runtimes WHERE room_id = ?`, params.RoomID).Scan(&params.ToAgentID); err != nil {
			return CollaborationMessage{}, fmt.Errorf("resolve room runtime recipient: %w", err)
		}
		implicitRoomRecipient = true
	}
	if params.ToAgentID == params.AgentID {
		return CollaborationMessage{}, errors.New("collaboration recipient must be another principal")
	}
	for _, agentID := range []string{params.AgentID, params.ToAgentID} {
		if err := requireRoomPrincipalAccessTx(ctx, tx, params.RoomID, agentID); err != nil {
			return CollaborationMessage{}, err
		}
	}
	var goalRevision, candidateRevision int
	if params.Kind == CollaborationCandidateReady || params.Kind == CollaborationPeerResult {
		if params.SourceMessageID == "" {
			return CollaborationMessage{}, errors.New("candidate and verification deliveries require a source task")
		}
		task, err := loadMessageTx(ctx, tx, params.SourceMessageID)
		if err != nil {
			return CollaborationMessage{}, err
		}
		if task.RoomID != params.RoomID || task.Kind != MessageTask || !task.TaskVerificationRequired || task.TaskState != string(TaskStateChecking) {
			return CollaborationMessage{}, fmt.Errorf("%w: source must be a checking task that requires verification", ErrConflict)
		}
		if params.Kind == CollaborationCandidateReady && task.TaskOwner != params.AgentID {
			return CollaborationMessage{}, fmt.Errorf("%w: candidate-ready source must be the sender's task", ErrConflict)
		}
		if params.Kind == CollaborationPeerResult {
			if task.TaskOwner == params.AgentID {
				return CollaborationMessage{}, fmt.Errorf("%w: task owner cannot verify its own candidate", ErrConflict)
			}
			var verifierID string
			if err := tx.QueryRowContext(ctx, `
				SELECT run.profile
				FROM works work JOIN work_runs run ON run.id = work.current_run_ref
				WHERE work.id = ? AND run.kind = 'verifier' AND run.state = 'running'`, task.ID).Scan(&verifierID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return CollaborationMessage{}, fmt.Errorf("%w: no active named verifier run", ErrConflict)
				}
				return CollaborationMessage{}, fmt.Errorf("read active named verifier: %w", err)
			}
			if verifierID != params.AgentID {
				return CollaborationMessage{}, fmt.Errorf("%w: verification result came from an unassigned agent", ErrUnauthorized)
			}
		}
		var recipientRoomID string
		if err := tx.QueryRowContext(ctx, `SELECT room_id FROM room_runtimes WHERE id = ?`, params.ToAgentID).Scan(&recipientRoomID); err != nil {
			return CollaborationMessage{}, fmt.Errorf("validate candidate-ready recipient: %w", err)
		}
		if recipientRoomID != params.RoomID {
			return CollaborationMessage{}, fmt.Errorf("%w: recipient must be the hidden room runtime", ErrUnauthorized)
		}
		goalRevision = task.TaskGoalRevision
		candidateRevision = task.TaskCandidateRevision
		for _, artifactRef := range params.ArtifactRefs {
			if artifactRef == "" {
				continue
			}
			var artifactWorkID string
			if err := tx.QueryRowContext(ctx, `SELECT work_id FROM work_artifacts WHERE id = ?`, artifactRef).Scan(&artifactWorkID); err != nil || artifactWorkID != task.ID {
				return CollaborationMessage{}, fmt.Errorf("%w: candidate artifact %q does not belong to current work", ErrConflict, artifactRef)
			}
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
		ToAgentID: params.ToAgentID, Kind: params.Kind, Body: params.Body,
		WorkID: params.SourceMessageID, ArtifactRefs: params.ArtifactRefs,
		SourceMessageID: params.SourceMessageID, GoalRevision: goalRevision, CandidateRevision: candidateRevision,
		ReplyTo: params.ReplyTo, CreatedAt: now,
	})
	if err != nil {
		return CollaborationMessage{}, err
	}
	if params.Kind == CollaborationCandidateReady {
		candidateRef := message.ID
		workspaceRevision := ""
		if len(params.ArtifactRefs) > 0 && params.ArtifactRefs[0] != "" {
			candidateRef = params.ArtifactRefs[0]
			_ = tx.QueryRowContext(ctx, `SELECT workspace_revision FROM work_artifacts WHERE id = ?`, candidateRef).Scan(&workspaceRevision)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE works SET state = 'checking', candidate_revision = ?, candidate_artifact_ref = ?,
				candidate_workspace_revision = ?, verification_state = 'pending', updated_at = ?
			WHERE id = ? AND goal_revision = ?`, candidateRevision, candidateRef,
			workspaceRevision, toMillis(now), params.SourceMessageID, goalRevision); err != nil {
			return CollaborationMessage{}, fmt.Errorf("project candidate-ready work: %w", err)
		}
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
	if implicitRoomRecipient {
		message.ToAgentID = ""
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
	if message.Kind == "" {
		message.Kind = CollaborationControl
	}
	if message.FromType == "" {
		message.FromType = MemberAgent
	}
	artifactRefsJSON, err := json.Marshal(message.ArtifactRefs)
	if err != nil {
		return CollaborationMessage{}, fmt.Errorf("encode collaboration artifacts: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO collaboration_messages (
			id, room_id, from_type, from_id, to_agent_id, kind, body, work_id, source_message_id,
			goal_revision, candidate_revision, artifact_refs_json, reply_to, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		message.ID, message.RoomID, message.FromType, message.FromID, message.ToAgentID, message.Kind, message.Body,
		nullableString(message.WorkID), nullableString(message.SourceMessageID), message.GoalRevision, message.CandidateRevision, string(artifactRefsJSON),
		nullableString(message.ReplyTo), toMillis(message.CreatedAt))
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
