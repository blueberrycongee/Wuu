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
	params.FromSessionRef = strings.TrimSpace(params.FromSessionRef)
	params.TargetSessionRef = strings.TrimSpace(params.TargetSessionRef)
	params.Body = strings.TrimSpace(params.Body)
	params.SourceMessageID = strings.TrimSpace(params.SourceMessageID)
	params.WorkID = strings.TrimSpace(params.WorkID)
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
	if params.ToAgentID == params.AgentID &&
		(params.FromSessionRef == "" || params.TargetSessionRef == "" || params.FromSessionRef == params.TargetSessionRef) {
		return CollaborationMessage{}, errors.New("same-principal collaboration requires two distinct sessions")
	}
	if params.Kind == CollaborationCandidateReady || params.Kind == CollaborationPeerResult {
		if params.WorkID != "" && params.WorkID != params.SourceMessageID {
			return CollaborationMessage{}, fmt.Errorf("%w: verification delivery work must match its source task", ErrConflict)
		}
		params.WorkID = params.SourceMessageID
	} else if params.WorkID == "" && params.SourceMessageID != "" {
		var sourceWorkRoomID string
		err := tx.QueryRowContext(ctx, `SELECT room_id FROM works WHERE id = ?`, params.SourceMessageID).Scan(&sourceWorkRoomID)
		if err == nil {
			if sourceWorkRoomID != params.RoomID {
				return CollaborationMessage{}, fmt.Errorf("%w: source work belongs to another room", ErrConflict)
			}
			params.WorkID = params.SourceMessageID
		} else if !errors.Is(err, sql.ErrNoRows) {
			return CollaborationMessage{}, fmt.Errorf("resolve collaboration source work: %w", err)
		}
	}
	if params.WorkID != "" {
		var workRoomID string
		if err := tx.QueryRowContext(ctx, `SELECT room_id FROM works WHERE id = ?`, params.WorkID).Scan(&workRoomID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return CollaborationMessage{}, fmt.Errorf("%w: work %q", ErrNotFound, params.WorkID)
			}
			return CollaborationMessage{}, fmt.Errorf("load collaboration work: %w", err)
		} else if workRoomID != params.RoomID {
			return CollaborationMessage{}, fmt.Errorf("%w: collaboration work belongs to another room", ErrConflict)
		}
	}
	for _, agentID := range []string{params.AgentID, params.ToAgentID} {
		if err := requireRoomPrincipalAccessTx(ctx, tx, params.RoomID, agentID); err != nil {
			return CollaborationMessage{}, err
		}
	}
	if err := validateCollaborationSessionWriteTx(ctx, tx, params.FromSessionRef, params.AgentID, params.RoomID, params.WorkID, 0); err != nil {
		return CollaborationMessage{}, err
	}
	if err := validateCollaborationSessionRouteTx(ctx, tx, params.TargetSessionRef, params.ToAgentID, params.RoomID, params.WorkID); err != nil {
		return CollaborationMessage{}, err
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
		FromSessionRef: params.FromSessionRef, ToAgentID: params.ToAgentID,
		TargetSessionRef: params.TargetSessionRef, Kind: params.Kind, Body: params.Body,
		WorkID: params.WorkID, ArtifactRefs: params.ArtifactRefs,
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
			id, room_id, from_type, from_id, from_session_ref, to_agent_id, target_session_ref, kind, body, work_id, source_message_id,
			goal_revision, candidate_revision, artifact_refs_json, reply_to, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		message.ID, message.RoomID, message.FromType, message.FromID, nullableString(message.FromSessionRef), message.ToAgentID, nullableString(message.TargetSessionRef), message.Kind, message.Body,
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

// PendingCollaborationDispatches returns only routing metadata. It does not
// claim messages or reveal their contents; the selected session must still use
// CheckSession to consume its own delivery.
func (s *Service) PendingCollaborationDispatches(ctx context.Context, agentID string) ([]CollaborationDispatch, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, errors.New("collaboration recipient is required")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, room_id, COALESCE(target_session_ref, ''), COALESCE(work_id, ''), kind
		FROM collaboration_messages
		WHERE to_agent_id = ? AND pulled_at IS NULL AND invalidated_at IS NULL
		ORDER BY created_at, rowid`, agentID)
	if err != nil {
		return nil, fmt.Errorf("list pending collaboration dispatches: %w", err)
	}
	defer rows.Close()
	dispatches := make([]CollaborationDispatch, 0)
	for rows.Next() {
		var dispatch CollaborationDispatch
		if err := rows.Scan(&dispatch.ID, &dispatch.RoomID, &dispatch.TargetSessionRef, &dispatch.WorkID, &dispatch.Kind); err != nil {
			return nil, fmt.Errorf("scan pending collaboration dispatch: %w", err)
		}
		dispatches = append(dispatches, dispatch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending collaboration dispatches: %w", err)
	}
	return dispatches, nil
}

// RoutePendingCollaborationToSession durably assigns every currently pending
// delivery for one Work to its selected session before any agent turn starts.
func (s *Service) RoutePendingCollaborationToSession(ctx context.Context, agentID, workID, sessionRef string) error {
	agentID = strings.TrimSpace(agentID)
	workID = strings.TrimSpace(workID)
	sessionRef = strings.TrimSpace(sessionRef)
	if agentID == "" || workID == "" || sessionRef == "" {
		return errors.New("collaboration recipient, work, and session are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin collaboration session route: %w", err)
	}
	defer tx.Rollback()
	binding, err := scanCollaborationSession(tx.QueryRowContext(ctx, collaborationSessionSelect+` WHERE binding.session_ref = ?`, sessionRef))
	if err != nil {
		return err
	}
	if binding.PrincipalID != agentID {
		return ErrUnauthorized
	}
	if binding.WorkID != workID || binding.RoomID == "" ||
		binding.State == CollaborationSessionInterrupted || binding.State == CollaborationSessionMissing {
		return fmt.Errorf("%w: collaboration session does not own the requested work", ErrConflict)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE collaboration_messages SET target_session_ref = ?
		WHERE to_agent_id = ? AND work_id = ? AND room_id = ?
			AND target_session_ref IS NULL AND pulled_at IS NULL AND invalidated_at IS NULL`,
		sessionRef, agentID, workID, binding.RoomID); err != nil {
		return fmt.Errorf("route pending collaboration deliveries: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit collaboration session route: %w", err)
	}
	return nil
}
