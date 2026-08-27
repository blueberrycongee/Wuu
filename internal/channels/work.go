package channels

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func insertWorkTx(ctx context.Context, tx *sql.Tx, task Message, params TaskCreateParams) error {
	sourceMessageID := strings.TrimSpace(params.SourceMessageID)
	if sourceMessageID == "" {
		sourceMessageID = strings.TrimSpace(params.ThreadID)
	}
	if sourceMessageID == "" {
		// A manually created root task has no preceding user message. Keeping the
		// task projection as its own debt anchor is explicit and recoverable.
		sourceMessageID = task.ID
	} else {
		source, err := loadMessageTx(ctx, tx, sourceMessageID)
		if err != nil {
			return fmt.Errorf("load work source message: %w", err)
		}
		if source.RoomID != task.RoomID {
			return errors.New("work source message belongs to another room")
		}
	}
	verification := WorkVerificationNotRequired
	if task.TaskVerificationRequired {
		verification = WorkVerificationPending
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO works(
			id, room_id, source_message_id, owner_named_agent_id, lead_named_agent_id,
			title, brief, goal_revision, candidate_revision, state,
			verification_state, verification_required, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'open', ?, ?, ?, ?)`,
		task.ID, task.RoomID, sourceMessageID, task.TaskOwner, nullableString(params.LeadNamedAgentID),
		task.TaskTitle, task.Body, task.TaskGoalRevision, task.TaskCandidateRevision,
		verification, boolInt(task.TaskVerificationRequired), toMillis(task.CreatedAt), toMillis(task.CreatedAt))
	if err != nil {
		return fmt.Errorf("insert durable work: %w", err)
	}
	if err := insertWorkEventTx(ctx, tx, WorkEvent{
		WorkID: task.ID, Kind: "state", State: string(WorkOpen), Summary: "Work created",
		GoalRevision: task.TaskGoalRevision, CandidateRevision: task.TaskCandidateRevision, CreatedAt: task.CreatedAt,
	}); err != nil {
		return err
	}
	return nil
}

func syncWorkFromTaskTx(ctx context.Context, tx *sql.Tx, task Message, updatedAt time.Time) error {
	state := workStateFromTask(TaskState(task.TaskState))
	var previousState WorkState
	_ = tx.QueryRowContext(ctx, `SELECT state FROM works WHERE id = ?`, task.ID).Scan(&previousState)
	verification := WorkVerificationNotRequired
	if task.TaskVerificationRequired {
		verification = WorkVerificationPending
		var decision VerificationDecision
		var goalRevision, candidateRevision int
		if err := tx.QueryRowContext(ctx, `
			SELECT decision, goal_revision, candidate_revision FROM task_verifications WHERE task_id = ?`, task.ID,
		).Scan(&decision, &goalRevision, &candidateRevision); err == nil &&
			goalRevision == task.TaskGoalRevision && candidateRevision == task.TaskCandidateRevision {
			verification = WorkVerificationState(decision)
		}
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE works SET owner_named_agent_id = ?, brief = ?, goal_revision = ?, candidate_revision = ?,
			state = ?, verification_state = ?, updated_at = ? WHERE id = ?`,
		task.TaskOwner, task.Body, task.TaskGoalRevision, task.TaskCandidateRevision,
		state, verification, toMillis(updatedAt), task.ID)
	if err != nil {
		return fmt.Errorf("sync durable work projection: %w", err)
	}
	if state == WorkCompleted || state == WorkFailed || state == WorkCancelled {
		if _, err := tx.ExecContext(ctx, `
			UPDATE collaboration_messages SET invalidated_at = ?
			WHERE work_id = ? AND pulled_at IS NULL AND invalidated_at IS NULL`,
			toMillis(updatedAt), task.ID); err != nil {
			return fmt.Errorf("invalidate terminal work deliveries: %w", err)
		}
	}
	if previousState != state {
		if err := insertWorkEventTx(ctx, tx, WorkEvent{
			WorkID: task.ID, Kind: "state", State: string(state), Summary: "Work state changed",
			GoalRevision: task.TaskGoalRevision, CandidateRevision: task.TaskCandidateRevision, CreatedAt: updatedAt,
		}); err != nil {
			return err
		}
	}
	return nil
}

func insertWorkEventTx(ctx context.Context, tx *sql.Tx, event WorkEvent) error {
	if event.ID == "" {
		id, err := randomID("workevent", 12)
		if err != nil {
			return err
		}
		event.ID = id
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO work_events(id, work_id, kind, state, summary, goal_revision, candidate_revision, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, event.ID, event.WorkID, event.Kind, event.State,
		event.Summary, event.GoalRevision, event.CandidateRevision, toMillis(event.CreatedAt))
	if err != nil {
		return fmt.Errorf("insert work event: %w", err)
	}
	return nil
}

func workStateFromTask(state TaskState) WorkState {
	switch state {
	case TaskStateDoing:
		return WorkWorking
	case TaskStateChecking:
		return WorkChecking
	case TaskStateRevising:
		return WorkRevising
	case TaskStateNeedsHuman:
		return WorkNeedsHuman
	case TaskStateDone:
		return WorkCompleted
	default:
		return WorkOpen
	}
}

func terminalWorkState(state WorkState) bool {
	return state == WorkCompleted || state == WorkFailed || state == WorkCancelled
}

func (s *Service) GetWork(ctx context.Context, workID string) (Work, error) {
	workID = strings.TrimSpace(workID)
	if workID == "" {
		return Work{}, errors.New("work id is required")
	}
	work, err := scanWork(s.db.QueryRowContext(ctx, workSelect+` WHERE work.id = ?`, workID))
	if err != nil {
		return Work{}, err
	}
	if err := s.loadWorkDetails(ctx, &work); err != nil {
		return Work{}, err
	}
	return work, nil
}

func (s *Service) ListWorks(ctx context.Context, roomID, agentID, token string) ([]Work, error) {
	if _, err := s.AuthenticatePrincipal(ctx, agentID, token); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	if err := requireRoomPrincipalAccessTx(ctx, tx, roomID, agentID); err != nil {
		tx.Rollback()
		return nil, err
	}
	_ = tx.Rollback()
	rows, err := s.db.QueryContext(ctx, workSelect+` WHERE work.room_id = ? ORDER BY work.created_at, work.id`, roomID)
	if err != nil {
		return nil, fmt.Errorf("list works: %w", err)
	}
	defer rows.Close()
	var works []Work
	for rows.Next() {
		work, err := scanWork(rows)
		if err != nil {
			return nil, err
		}
		works = append(works, work)
	}
	return works, rows.Err()
}

const workSelect = `
	SELECT work.id, work.room_id, work.source_message_id, work.owner_named_agent_id,
		COALESCE(work.lead_named_agent_id, ''), work.title, work.brief,
		work.goal_revision, work.candidate_revision, work.state,
		COALESCE(work.current_run_ref, ''), COALESCE(work.candidate_artifact_ref, ''),
		COALESCE(work.candidate_workspace_revision, ''), work.verification_state,
		work.verification_required, work.max_verifier_attempts, work.max_candidates,
		work.verifier_attempts_used, work.candidates_used, work.fanout_reason,
		work.checks_summary, work.changed_files_count, work.unresolved_items, work.failure_reason,
		work.cancelled_at, work.created_at, work.updated_at
	FROM works work`

func scanWork(row scanner) (Work, error) {
	var work Work
	var verificationRequired int
	var cancelledAt sql.NullInt64
	var createdAt, updatedAt int64
	if err := row.Scan(
		&work.ID, &work.RoomID, &work.SourceMessageID, &work.OwnerNamedAgentID,
		&work.LeadNamedAgentID, &work.Title, &work.Brief, &work.GoalRevision,
		&work.CandidateRevision, &work.State, &work.CurrentRunRef, &work.CandidateArtifactRef,
		&work.CandidateWorkspaceRevision, &work.VerificationState, &verificationRequired,
		&work.MaxVerifierAttempts, &work.MaxCandidates, &work.VerifierAttemptsUsed,
		&work.CandidatesUsed, &work.FanoutReason, &work.ChecksSummary, &work.ChangedFilesCount,
		&work.UnresolvedItems, &work.FailureReason, &cancelledAt, &createdAt, &updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Work{}, ErrNotFound
		}
		return Work{}, fmt.Errorf("scan work: %w", err)
	}
	work.VerificationRequired = verificationRequired != 0
	if cancelledAt.Valid {
		work.CancelledAt = fromMillis(cancelledAt.Int64)
	}
	work.CreatedAt, work.UpdatedAt = fromMillis(createdAt), fromMillis(updatedAt)
	return work, nil
}

func (s *Service) loadWorkDetails(ctx context.Context, work *Work) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM collaboration_messages
		WHERE work_id = ? AND pulled_at IS NULL AND invalidated_at IS NULL
		ORDER BY created_at, id`, work.ID)
	if err != nil {
		return fmt.Errorf("list pending work deliveries: %w", err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		work.PendingDeliveryRefs = append(work.PendingDeliveryRefs, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	work.Deliveries, err = s.listWorkDeliveries(ctx, work.ID)
	if err != nil {
		return err
	}
	work.Runs, err = s.listWorkRuns(ctx, work.ID)
	if err != nil {
		return err
	}
	work.Artifacts, err = s.listWorkArtifacts(ctx, work.ID)
	if err != nil {
		return err
	}
	work.Events, err = s.listWorkEvents(ctx, work.ID)
	if err != nil {
		return err
	}
	if verification, err := s.GetTaskVerification(ctx, work.ID); err == nil {
		work.Verification = &verification
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}
	return nil
}

func (s *Service) listWorkDeliveries(ctx context.Context, workID string) ([]CollaborationMessage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT delivery.id, delivery.room_id, delivery.from_type,
			CASE WHEN sender.kind = 'named_agent' THEN delivery.from_id ELSE '' END,
			COALESCE(delivery.from_session_ref, ''),
			delivery.to_agent_id,
			COALESCE(delivery.target_session_ref, ''),
			CASE WHEN principal.kind = 'named_agent' THEN delivery.to_agent_id ELSE '' END,
			delivery.kind, delivery.body, COALESCE(delivery.work_id, ''),
			COALESCE(delivery.source_message_id, ''), delivery.goal_revision, delivery.candidate_revision,
			delivery.artifact_refs_json, COALESCE(delivery.reply_to, ''), delivery.created_at,
			COALESCE(delivery.consumed_at, 0), COALESCE(delivery.invalidated_at, 0)
		FROM collaboration_messages delivery
		JOIN collaboration_principals principal ON principal.id = delivery.to_agent_id
		LEFT JOIN collaboration_principals sender ON sender.id = delivery.from_id
		WHERE delivery.work_id = ?
		ORDER BY delivery.created_at, delivery.rowid`, workID)
	if err != nil {
		return nil, fmt.Errorf("list work collaboration messages: %w", err)
	}
	defer rows.Close()
	var messages []CollaborationMessage
	for rows.Next() {
		var message CollaborationMessage
		var artifactRefsJSON string
		var createdAt, consumedAt, invalidatedAt int64
		if err := rows.Scan(
			&message.ID, &message.RoomID, &message.FromType, &message.FromID, &message.FromSessionRef, &message.ToAgentID,
			&message.TargetSessionRef,
			&message.RecipientNamedAgentID, &message.Kind, &message.Body, &message.WorkID,
			&message.SourceMessageID, &message.GoalRevision, &message.CandidateRevision,
			&artifactRefsJSON, &message.ReplyTo, &createdAt, &consumedAt, &invalidatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan work collaboration message: %w", err)
		}
		message.CreatedAt = fromMillis(createdAt)
		if message.RecipientNamedAgentID == "" {
			message.ToAgentID = ""
		}
		if consumedAt != 0 {
			message.ConsumedAt = fromMillis(consumedAt)
		}
		if invalidatedAt != 0 {
			message.InvalidatedAt = fromMillis(invalidatedAt)
		}
		if err := json.Unmarshal([]byte(artifactRefsJSON), &message.ArtifactRefs); err != nil {
			return nil, fmt.Errorf("decode work collaboration artifacts: %w", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate work collaboration messages: %w", err)
	}
	return messages, nil
}

func (s *Service) StartWorkRun(ctx context.Context, params WorkRunStartParams) (WorkRun, error) {
	actor, err := s.AuthenticatePrincipal(ctx, params.AgentID, params.Token)
	if err != nil {
		return WorkRun{}, err
	}
	params.WorkID, params.SessionRef = strings.TrimSpace(params.WorkID), strings.TrimSpace(params.SessionRef)
	params.NamedAgentID = strings.TrimSpace(params.NamedAgentID)
	params.Profile = strings.TrimSpace(params.Profile)
	if params.WorkID == "" || !validWorkRunKind(params.Kind) {
		return WorkRun{}, errors.New("work run requires a work and valid kind")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WorkRun{}, err
	}
	defer tx.Rollback()
	work, err := scanWork(tx.QueryRowContext(ctx, workSelect+` WHERE work.id = ?`, params.WorkID))
	if err != nil {
		return WorkRun{}, err
	}
	if err := authorizeWorkActorTx(ctx, tx, work, actor); err != nil {
		return WorkRun{}, err
	}
	namedAgentID := params.NamedAgentID
	if !actor.IsRoomRuntime() {
		if namedAgentID != "" && namedAgentID != actor.ID {
			return WorkRun{}, fmt.Errorf("%w: named agents may only start their own sessions", ErrUnauthorized)
		}
		namedAgentID = actor.ID
	} else if params.Kind == WorkRunVerifier && params.Profile != "" && params.Profile != WorkVerifierProfileIndependent {
		if namedAgentID != "" && namedAgentID != params.Profile {
			return WorkRun{}, fmt.Errorf("%w: verifier profile and named session owner must match", ErrConflict)
		}
		namedAgentID = params.Profile
	}
	if namedAgentID != "" {
		if err := s.requireRoomAgentMemberTx(ctx, tx, work.RoomID, namedAgentID); err != nil {
			return WorkRun{}, err
		}
	}
	freshNamedSession := false
	if params.SessionRef == "" && namedAgentID != "" {
		params.SessionRef, err = randomID("collab-session", 12)
		if err != nil {
			return WorkRun{}, err
		}
		freshNamedSession = true
	}
	allowFreshNamedSession := freshNamedSession ||
		(actor.IsRoomRuntime() && params.Kind == WorkRunVerifier && namedAgentID != "")
	bindRunSession := params.SessionRef != "" && namedAgentID != ""
	if bindRunSession && (actor.IsRoomRuntime() || params.Kind != WorkRunVerifier) {
		if err := validateCollaborationSessionRouteTx(ctx, tx, params.SessionRef, namedAgentID, work.RoomID, work.ID); err != nil {
			if allowFreshNamedSession && errors.Is(err, ErrNotFound) {
				// The binding is inserted atomically with the run below.
			} else {
				// A fresh unscoped conversation binding can be promoted to this run.
				binding, bindErr := scanCollaborationSession(tx.QueryRowContext(ctx, collaborationSessionSelect+` WHERE binding.session_ref = ?`, params.SessionRef))
				if bindErr != nil || binding.PrincipalID != namedAgentID || binding.RoomID != work.RoomID || binding.WorkID != "" || binding.RunID != "" || binding.State == CollaborationSessionRunning || binding.State == CollaborationSessionMissing {
					if bindErr != nil {
						return WorkRun{}, fmt.Errorf("validate work session: %w", bindErr)
					}
					return WorkRun{}, fmt.Errorf("%w: work session is already scoped or unavailable", ErrConflict)
				}
			}
		}
	}
	if work.State == WorkCancelled || work.State == WorkCompleted || work.State == WorkFailed {
		return WorkRun{}, fmt.Errorf("%w: work is %s", ErrConflict, work.State)
	}
	if params.Kind == WorkRunVerifier {
		if !work.VerificationRequired || work.VerifierAttemptsUsed >= work.MaxVerifierAttempts {
			return WorkRun{}, fmt.Errorf("%w: verifier budget is unavailable", ErrConflict)
		}
		if work.State != WorkChecking || work.CandidateRevision == 0 {
			return WorkRun{}, fmt.Errorf("%w: verifier run requires a checking candidate", ErrConflict)
		}
		if !actor.IsRoomRuntime() || actor.RoomID != work.RoomID {
			return WorkRun{}, fmt.Errorf("%w: only the room runtime may start verification", ErrUnauthorized)
		}
		verifierID := strings.TrimSpace(params.Profile)
		if verifierID == "" {
			verifierID = WorkVerifierProfileIndependent
			params.Profile = verifierID
		}
		if namedAgentID != "" && verifierID != namedAgentID {
			return WorkRun{}, fmt.Errorf("%w: verifier profile and named session owner must match", ErrConflict)
		}
		if verifierID == work.OwnerNamedAgentID {
			return WorkRun{}, fmt.Errorf("%w: verifier run requires a different named agent", ErrConflict)
		}
		if verifierID != WorkVerifierProfileIndependent {
			var verifierMember int
			if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM room_members member
			JOIN named_agents agent ON agent.id = member.member_id AND agent.kind = 'named'
			WHERE member.room_id = ? AND member.member_type = 'agent' AND member.member_id = ?`,
				work.RoomID, verifierID).Scan(&verifierMember); err != nil {
				return WorkRun{}, fmt.Errorf("validate named verifier: %w", err)
			}
			if verifierMember == 0 {
				return WorkRun{}, fmt.Errorf("%w: named verifier must be a current room member", ErrUnauthorized)
			}
		}
		if work.CurrentRunRef != "" {
			var state WorkRunState
			if err := tx.QueryRowContext(ctx, `SELECT state FROM work_runs WHERE id = ?`, work.CurrentRunRef).Scan(&state); err == nil && state == WorkRunRunning {
				return WorkRun{}, fmt.Errorf("%w: verifier run %q is already active", ErrConflict, work.CurrentRunRef)
			}
		}
	}
	if params.Kind == WorkRunSelector && (work.MaxCandidates < 2 || work.CandidatesUsed < 2 || strings.TrimSpace(work.FanoutReason) == "") {
		return WorkRun{}, fmt.Errorf("%w: selector requires an explicit multi-candidate policy", ErrConflict)
	}
	id, err := randomID("workrun", 12)
	if err != nil {
		return WorkRun{}, err
	}
	now := fromMillis(toMillis(s.now()))
	run := WorkRun{
		ID: id, WorkID: work.ID, NamedAgentID: namedAgentID, Kind: params.Kind, Profile: params.Profile,
		SessionRef: params.SessionRef, State: WorkRunRunning, GoalRevision: work.GoalRevision,
		CandidateRevision: work.CandidateRevision, WorkspaceRevision: strings.TrimSpace(params.WorkspaceRevision),
		StartedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_runs(id, work_id, named_agent_id, kind, profile, session_ref, state, goal_revision,
			candidate_revision, workspace_revision, started_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'running', ?, ?, ?, ?, ?, ?)`,
		run.ID, run.WorkID, nullableString(run.NamedAgentID), run.Kind, run.Profile, nullableString(run.SessionRef), run.GoalRevision,
		run.CandidateRevision, run.WorkspaceRevision, toMillis(now), toMillis(now), toMillis(now)); err != nil {
		return WorkRun{}, fmt.Errorf("insert work run: %w", err)
	}
	verifierIncrement := 0
	if run.Kind == WorkRunVerifier {
		verifierIncrement = 1
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE works SET current_run_ref = ?, verifier_attempts_used = verifier_attempts_used + ?, updated_at = ? WHERE id = ?`,
		run.ID, verifierIncrement, toMillis(now), work.ID); err != nil {
		return WorkRun{}, fmt.Errorf("activate work run: %w", err)
	}
	if bindRunSession {
		purpose := CollaborationSessionWork
		if run.Kind == WorkRunVerifier {
			purpose = CollaborationSessionVerification
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO collaboration_session_bindings(
				session_ref, principal_id, named_agent_id, room_id, work_id, run_id,
				purpose, state, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, 'running', ?, ?)
			ON CONFLICT(session_ref) DO UPDATE SET
				room_id = excluded.room_id, work_id = excluded.work_id, run_id = excluded.run_id,
				purpose = excluded.purpose, state = excluded.state, updated_at = excluded.updated_at`,
			run.SessionRef, namedAgentID, namedAgentID, work.RoomID, work.ID, run.ID,
			purpose, toMillis(now), toMillis(now)); err != nil {
			return WorkRun{}, fmt.Errorf("bind work run session: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return WorkRun{}, err
	}
	return run, nil
}

func (s *Service) FinishWorkRun(ctx context.Context, params WorkRunFinishParams) (WorkRun, error) {
	actor, err := s.AuthenticatePrincipal(ctx, params.AgentID, params.Token)
	if err != nil {
		return WorkRun{}, err
	}
	if !terminalWorkRunState(params.State) {
		return WorkRun{}, errors.New("work run finish requires a terminal state")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WorkRun{}, err
	}
	defer tx.Rollback()
	work, err := scanWork(tx.QueryRowContext(ctx, workSelect+` WHERE work.id = ?`, params.WorkID))
	if err != nil {
		return WorkRun{}, err
	}
	if err := authorizeWorkActorTx(ctx, tx, work, actor); err != nil {
		return WorkRun{}, err
	}
	run, err := scanWorkRun(tx.QueryRowContext(ctx, workRunSelect+` WHERE run.id = ? AND run.work_id = ?`, params.RunID, work.ID))
	if err != nil {
		return WorkRun{}, err
	}
	if run.State != WorkRunRunning && run.State != WorkRunQueued {
		return WorkRun{}, fmt.Errorf("%w: work run is already %s", ErrConflict, run.State)
	}
	if run.GoalRevision != work.GoalRevision || run.CandidateRevision != work.CandidateRevision {
		return WorkRun{}, fmt.Errorf("%w: work run revisions are stale", ErrConflict)
	}
	now := fromMillis(toMillis(s.now()))
	run.State, run.Outcome, run.Provider, run.Model = params.State, strings.TrimSpace(params.Outcome), strings.TrimSpace(params.Provider), strings.TrimSpace(params.Model)
	run.InputTokens, run.OutputTokens, run.ChecksRerun = params.InputTokens, params.OutputTokens, params.ChecksRerun
	run.FindingsCount, run.RepairOutcome, run.EndedAt, run.UpdatedAt = params.FindingsCount, strings.TrimSpace(params.RepairOutcome), now, now
	if _, err := tx.ExecContext(ctx, `
		UPDATE work_runs SET state = ?, outcome = ?, provider = ?, model = ?, input_tokens = ?,
			output_tokens = ?, checks_rerun = ?, findings_count = ?, repair_outcome = ?, ended_at = ?, updated_at = ?
		WHERE id = ?`, run.State, run.Outcome, run.Provider, run.Model, run.InputTokens, run.OutputTokens,
		run.ChecksRerun, run.FindingsCount, run.RepairOutcome, toMillis(now), toMillis(now), run.ID); err != nil {
		return WorkRun{}, fmt.Errorf("finish work run: %w", err)
	}
	if err := refreshWorkCurrentRunRefTx(ctx, tx, work.ID, now); err != nil {
		return WorkRun{}, fmt.Errorf("settle work run handle: %w", err)
	}
	if run.SessionRef != "" {
		nextState := CollaborationSessionIdle
		if run.State == WorkRunInterrupted {
			nextState = CollaborationSessionInterrupted
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE collaboration_session_bindings SET state = ?, run_id = NULL, updated_at = ?
			WHERE session_ref = ? AND run_id = ?`, nextState, toMillis(now), run.SessionRef, run.ID); err != nil {
			return WorkRun{}, fmt.Errorf("release work run session: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return WorkRun{}, err
	}
	return run, nil
}

func (s *Service) AttachWorkRunTurn(ctx context.Context, params WorkRunTurnParams) (WorkRun, error) {
	actor, err := s.AuthenticatePrincipal(ctx, params.AgentID, params.Token)
	if err != nil {
		return WorkRun{}, err
	}
	params.WorkID = strings.TrimSpace(params.WorkID)
	params.RunID = strings.TrimSpace(params.RunID)
	params.SessionRef = strings.TrimSpace(params.SessionRef)
	params.TurnID = strings.TrimSpace(params.TurnID)
	if params.WorkID == "" || params.RunID == "" || params.SessionRef == "" || params.TurnID == "" {
		return WorkRun{}, errors.New("work run turn requires work, run, session, and turn")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WorkRun{}, err
	}
	defer tx.Rollback()
	run, err := scanWorkRun(tx.QueryRowContext(ctx, workRunSelect+` WHERE run.id = ? AND run.work_id = ?`, params.RunID, params.WorkID))
	if err != nil {
		return WorkRun{}, err
	}
	if run.NamedAgentID == "" || run.NamedAgentID != actor.ID || run.SessionRef != params.SessionRef {
		return WorkRun{}, ErrUnauthorized
	}
	if run.State != WorkRunRunning && run.State != WorkRunQueued {
		return WorkRun{}, fmt.Errorf("%w: work run is already %s", ErrConflict, run.State)
	}
	binding, err := scanCollaborationSession(tx.QueryRowContext(ctx, collaborationSessionSelect+` WHERE binding.session_ref = ?`, params.SessionRef))
	if err != nil {
		return WorkRun{}, err
	}
	if binding.PrincipalID != actor.ID || binding.WorkID != params.WorkID || binding.RunID != params.RunID || binding.State != CollaborationSessionRunning {
		return WorkRun{}, fmt.Errorf("%w: work run session binding does not match", ErrConflict)
	}
	if run.TurnID != "" && run.TurnID != params.TurnID {
		return WorkRun{}, fmt.Errorf("%w: work run is already attached to turn %q", ErrConflict, run.TurnID)
	}
	if run.TurnID == "" {
		if _, err := tx.ExecContext(ctx, `UPDATE work_runs SET turn_id = ?, updated_at = ? WHERE id = ?`, params.TurnID, toMillis(s.now()), run.ID); err != nil {
			return WorkRun{}, fmt.Errorf("attach work run turn: %w", err)
		}
	}
	updated, err := scanWorkRun(tx.QueryRowContext(ctx, workRunSelect+` WHERE run.id = ?`, run.ID))
	if err != nil {
		return WorkRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return WorkRun{}, err
	}
	return updated, nil
}

func (s *Service) AddWorkArtifact(ctx context.Context, params WorkArtifactAddParams) (WorkArtifact, error) {
	actor, err := s.AuthenticatePrincipal(ctx, params.AgentID, params.Token)
	if err != nil {
		return WorkArtifact{}, err
	}
	params.WorkID, params.URI = strings.TrimSpace(params.WorkID), strings.TrimSpace(params.URI)
	if params.WorkID == "" || params.URI == "" || !validWorkArtifactKind(params.Kind) {
		return WorkArtifact{}, errors.New("work artifact requires work, uri, and valid kind")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WorkArtifact{}, err
	}
	defer tx.Rollback()
	work, err := scanWork(tx.QueryRowContext(ctx, workSelect+` WHERE work.id = ?`, params.WorkID))
	if err != nil {
		return WorkArtifact{}, err
	}
	if err := authorizeWorkActorTx(ctx, tx, work, actor); err != nil {
		return WorkArtifact{}, err
	}
	if runID := strings.TrimSpace(params.RunID); runID != "" {
		run, err := scanWorkRun(tx.QueryRowContext(ctx, workRunSelect+` WHERE run.id = ? AND run.work_id = ?`, runID, work.ID))
		if err != nil {
			return WorkArtifact{}, err
		}
		if run.GoalRevision != work.GoalRevision || run.CandidateRevision != work.CandidateRevision {
			return WorkArtifact{}, fmt.Errorf("%w: artifact run revisions are stale", ErrConflict)
		}
	}
	if params.Kind == WorkArtifactCandidate && work.CandidatesUsed >= work.MaxCandidates {
		return WorkArtifact{}, fmt.Errorf("%w: candidate budget exhausted", ErrConflict)
	}
	id, err := randomID("artifact", 12)
	if err != nil {
		return WorkArtifact{}, err
	}
	now := fromMillis(toMillis(s.now()))
	artifact := WorkArtifact{ID: id, WorkID: work.ID, RunID: strings.TrimSpace(params.RunID), Kind: params.Kind, URI: params.URI, Label: strings.TrimSpace(params.Label), Summary: strings.TrimSpace(params.Summary), WorkspaceRevision: strings.TrimSpace(params.WorkspaceRevision), CreatedAt: now}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_artifacts(id, work_id, run_id, kind, uri, label, summary, workspace_revision, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, artifact.ID, artifact.WorkID, nullableString(artifact.RunID), artifact.Kind,
		artifact.URI, artifact.Label, artifact.Summary, artifact.WorkspaceRevision, toMillis(now)); err != nil {
		return WorkArtifact{}, fmt.Errorf("insert work artifact: %w", err)
	}
	if artifact.Kind == WorkArtifactCandidate || artifact.Kind == WorkArtifactSnapshot || artifact.Kind == WorkArtifactDiff {
		if _, err := tx.ExecContext(ctx, `
			UPDATE works SET candidate_artifact_ref = ?, candidate_workspace_revision = ?,
				candidates_used = candidates_used + CASE WHEN ? = 'candidate' THEN 1 ELSE 0 END, updated_at = ?
			WHERE id = ?`, artifact.ID, artifact.WorkspaceRevision, artifact.Kind, toMillis(now), work.ID); err != nil {
			return WorkArtifact{}, fmt.Errorf("project candidate artifact: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return WorkArtifact{}, err
	}
	return artifact, nil
}

func (s *Service) CancelWork(ctx context.Context, workID, reason, agentID, token string) (Work, error) {
	actor, err := s.AuthenticatePrincipal(ctx, agentID, token)
	if err != nil {
		return Work{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Work{}, err
	}
	defer tx.Rollback()
	work, err := scanWork(tx.QueryRowContext(ctx, workSelect+` WHERE work.id = ?`, workID))
	if err != nil {
		return Work{}, err
	}
	if err := authorizeWorkActorTx(ctx, tx, work, actor); err != nil {
		return Work{}, err
	}
	if work.State == WorkCompleted {
		return Work{}, fmt.Errorf("%w: completed work cannot be cancelled", ErrConflict)
	}
	interruptTargets, err := activeWorkSessionInterruptTargetsTx(ctx, tx, work.ID)
	if err != nil {
		return Work{}, err
	}
	wakeRecipients, err := pendingWorkWakeRecipientsTx(ctx, tx, work.ID)
	if err != nil {
		return Work{}, err
	}
	now := fromMillis(toMillis(s.now()))
	if _, err := tx.ExecContext(ctx, `UPDATE works SET state = 'cancelled', current_run_ref = NULL, failure_reason = ?, cancelled_at = ?, updated_at = ? WHERE id = ?`, strings.TrimSpace(reason), toMillis(now), toMillis(now), work.ID); err != nil {
		return Work{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE room_messages SET task_state = 'open' WHERE id = ?`, work.ID); err != nil {
		return Work{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE collaboration_messages SET invalidated_at = ? WHERE work_id = ? AND pulled_at IS NULL`, toMillis(now), work.ID); err != nil {
		return Work{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE inbox_items SET pulled_at = ?
		WHERE member_type = 'agent' AND message_id = ? AND kind = 'task' AND pulled_at IS NULL`, toMillis(now), work.ID); err != nil {
		return Work{}, fmt.Errorf("retire cancelled work inbox: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE collaboration_session_bindings
		SET state = 'interrupted', run_id = NULL, updated_at = ?
		WHERE work_id = ? AND state != 'missing'`, toMillis(now), work.ID); err != nil {
		return Work{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE work_runs SET state = 'cancelled', ended_at = ?, updated_at = ? WHERE work_id = ? AND state IN ('queued', 'running')`, toMillis(now), toMillis(now), work.ID); err != nil {
		return Work{}, err
	}
	if err := insertWorkEventTx(ctx, tx, WorkEvent{WorkID: work.ID, Kind: "cancellation", State: string(WorkCancelled), Summary: strings.TrimSpace(reason), GoalRevision: work.GoalRevision, CandidateRevision: work.CandidateRevision, CreatedAt: now}); err != nil {
		return Work{}, err
	}
	for _, recipientID := range wakeRecipients {
		if err := recomputeAgentWakeTx(ctx, tx, recipientID, toMillis(now)); err != nil {
			return Work{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Work{}, err
	}
	s.interruptWorkSessions(interruptTargets)
	return s.GetWork(ctx, work.ID)
}

type workSessionInterruptTarget struct {
	agentID    string
	sessionRef string
}

func pendingWorkWakeRecipientsTx(ctx context.Context, tx *sql.Tx, workID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT member_id FROM inbox_items
		WHERE member_type = 'agent' AND message_id = ? AND kind = 'task' AND pulled_at IS NULL
		UNION
		SELECT to_agent_id FROM collaboration_messages
		WHERE work_id = ? AND pulled_at IS NULL AND invalidated_at IS NULL
		ORDER BY 1`, workID, workID)
	if err != nil {
		return nil, fmt.Errorf("list cancelled work wake recipients: %w", err)
	}
	defer rows.Close()
	recipients := make([]string, 0)
	for rows.Next() {
		var recipientID string
		if err := rows.Scan(&recipientID); err != nil {
			return nil, fmt.Errorf("scan cancelled work wake recipient: %w", err)
		}
		if recipientID != "" {
			recipients = append(recipients, recipientID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cancelled work wake recipients: %w", err)
	}
	return recipients, nil
}

func activeWorkSessionInterruptTargetsTx(ctx context.Context, tx *sql.Tx, workID string) ([]workSessionInterruptTarget, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT
			COALESCE(NULLIF(run.named_agent_id, ''), NULLIF(binding.named_agent_id, ''), ''),
			COALESCE(NULLIF(run.session_ref, ''), NULLIF(binding.session_ref, ''), '')
		FROM work_runs run
		LEFT JOIN collaboration_session_bindings binding ON binding.run_id = run.id
		WHERE run.work_id = ? AND run.state IN ('queued', 'running')
		ORDER BY 1, 2`, workID)
	if err != nil {
		return nil, fmt.Errorf("list active work sessions for cancellation: %w", err)
	}
	defer rows.Close()
	targets := make([]workSessionInterruptTarget, 0)
	for rows.Next() {
		var target workSessionInterruptTarget
		if err := rows.Scan(&target.agentID, &target.sessionRef); err != nil {
			return nil, fmt.Errorf("scan active work session for cancellation: %w", err)
		}
		if target.sessionRef != "" {
			targets = append(targets, target)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active work sessions for cancellation: %w", err)
	}
	return targets, nil
}

func (s *Service) interruptWorkSessions(targets []workSessionInterruptTarget) {
	if s == nil || s.wake == nil {
		return
	}
	namedInterrupt, hasNamedInterrupt := s.wake.(WakeSessionInterruptSink)
	runInterrupt, hasRunInterrupt := s.wake.(WakeRunSessionInterruptSink)
	fallback, hasFallback := s.wake.(WakeInterruptSink)
	interruptedAgents := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if target.agentID == "" {
			if hasRunInterrupt {
				runInterrupt.InterruptRunSession(target.sessionRef)
			}
			continue
		}
		if hasNamedInterrupt {
			namedInterrupt.InterruptSession(target.agentID, target.sessionRef)
			continue
		}
		if !hasFallback {
			continue
		}
		if _, exists := interruptedAgents[target.agentID]; exists {
			continue
		}
		interruptedAgents[target.agentID] = struct{}{}
		fallback.Interrupt(target.agentID)
	}
}

func (s *Service) UpdateWorkPolicy(ctx context.Context, params WorkPolicyUpdateParams) (Work, error) {
	actor, err := s.AuthenticatePrincipal(ctx, params.AgentID, params.Token)
	if err != nil {
		return Work{}, err
	}
	if params.MaxVerifierAttempts < 1 || params.MaxVerifierAttempts > 10 || params.MaxCandidates < 1 || params.MaxCandidates > 4 {
		return Work{}, errors.New("work policy budgets are out of range")
	}
	params.FanoutReason = strings.TrimSpace(params.FanoutReason)
	if params.MaxCandidates > 1 && params.FanoutReason == "" {
		return Work{}, errors.New("multi-candidate policy requires a concrete fan-out reason")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Work{}, err
	}
	defer tx.Rollback()
	work, err := scanWork(tx.QueryRowContext(ctx, workSelect+` WHERE work.id = ?`, params.WorkID))
	if err != nil {
		return Work{}, err
	}
	if err := authorizeWorkActorTx(ctx, tx, work, actor); err != nil {
		return Work{}, err
	}
	if params.LeadNamedAgentID != "" {
		if err := requireMemberTx(ctx, tx, work.RoomID, MemberAgent, params.LeadNamedAgentID); err != nil {
			return Work{}, err
		}
	}
	now := toMillis(s.now())
	if _, err := tx.ExecContext(ctx, `
		UPDATE works SET lead_named_agent_id = ?, max_verifier_attempts = ?, max_candidates = ?,
			fanout_reason = ?, updated_at = ? WHERE id = ?`, nullableString(params.LeadNamedAgentID),
		params.MaxVerifierAttempts, params.MaxCandidates, params.FanoutReason, now, work.ID); err != nil {
		return Work{}, fmt.Errorf("update work policy: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Work{}, err
	}
	return s.GetWork(ctx, work.ID)
}

func (s *Service) UpdateWorkEvidence(ctx context.Context, params WorkEvidenceUpdateParams) (Work, error) {
	actor, err := s.AuthenticatePrincipal(ctx, params.AgentID, params.Token)
	if err != nil {
		return Work{}, err
	}
	if params.ChangedFilesCount < 0 {
		return Work{}, errors.New("changed files count cannot be negative")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Work{}, err
	}
	defer tx.Rollback()
	work, err := scanWork(tx.QueryRowContext(ctx, workSelect+` WHERE work.id = ?`, params.WorkID))
	if err != nil {
		return Work{}, err
	}
	if err := authorizeWorkActorTx(ctx, tx, work, actor); err != nil {
		return Work{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE works SET checks_summary = ?, changed_files_count = ?, unresolved_items = ?, updated_at = ? WHERE id = ?`,
		strings.TrimSpace(params.ChecksSummary), params.ChangedFilesCount,
		strings.TrimSpace(params.UnresolvedItems), toMillis(s.now()), work.ID); err != nil {
		return Work{}, fmt.Errorf("update work evidence summary: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Work{}, err
	}
	return s.GetWork(ctx, work.ID)
}

func authorizeWorkActorTx(ctx context.Context, tx *sql.Tx, work Work, actor AgentRuntime) error {
	if actor.ID == work.OwnerNamedAgentID || actor.ID == work.LeadNamedAgentID {
		return nil
	}
	if actor.IsRoomRuntime() && actor.RoomID == work.RoomID {
		return nil
	}
	return ErrUnauthorized
}

func refreshWorkCurrentRunRefTx(ctx context.Context, tx *sql.Tx, workID string, updatedAt time.Time) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE works
		SET current_run_ref = (
			SELECT run.id FROM work_runs run
			WHERE run.work_id = works.id AND run.state IN ('queued', 'running')
			ORDER BY run.created_at DESC, run.id DESC LIMIT 1
		), updated_at = ?
		WHERE id = ?`, toMillis(updatedAt), workID)
	return err
}

func validWorkRunKind(kind WorkRunKind) bool {
	return kind == WorkRunProducer || kind == WorkRunVerifier || kind == WorkRunSelector || kind == WorkRunIntegration
}

func terminalWorkRunState(state WorkRunState) bool {
	return state == WorkRunCompleted || state == WorkRunFailed || state == WorkRunCancelled || state == WorkRunInterrupted
}

func validWorkArtifactKind(kind WorkArtifactKind) bool {
	switch kind {
	case WorkArtifactCandidate, WorkArtifactDiff, WorkArtifactSnapshot, WorkArtifactCheckLog, WorkArtifactScreenshot, WorkArtifactReport, WorkArtifactOther:
		return true
	default:
		return false
	}
}

const workRunSelect = `
	SELECT run.id, run.work_id, COALESCE(run.named_agent_id, ''), run.kind, run.profile, COALESCE(run.session_ref, ''), run.turn_id, run.state,
		run.goal_revision, run.candidate_revision, run.workspace_revision, run.provider, run.model,
		run.input_tokens, run.output_tokens, run.checks_rerun, run.findings_count, run.outcome,
		run.repair_outcome, run.started_at, run.ended_at, run.created_at, run.updated_at
	FROM work_runs run`

func scanWorkRun(row scanner) (WorkRun, error) {
	var run WorkRun
	var startedAt, endedAt sql.NullInt64
	var createdAt, updatedAt int64
	if err := row.Scan(&run.ID, &run.WorkID, &run.NamedAgentID, &run.Kind, &run.Profile, &run.SessionRef, &run.TurnID, &run.State,
		&run.GoalRevision, &run.CandidateRevision, &run.WorkspaceRevision, &run.Provider, &run.Model,
		&run.InputTokens, &run.OutputTokens, &run.ChecksRerun, &run.FindingsCount, &run.Outcome,
		&run.RepairOutcome, &startedAt, &endedAt, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WorkRun{}, ErrNotFound
		}
		return WorkRun{}, err
	}
	if startedAt.Valid {
		run.StartedAt = fromMillis(startedAt.Int64)
	}
	if endedAt.Valid {
		run.EndedAt = fromMillis(endedAt.Int64)
	}
	run.CreatedAt, run.UpdatedAt = fromMillis(createdAt), fromMillis(updatedAt)
	return run, nil
}

func (s *Service) listWorkRuns(ctx context.Context, workID string) ([]WorkRun, error) {
	rows, err := s.db.QueryContext(ctx, workRunSelect+` WHERE run.work_id = ? ORDER BY run.created_at, run.rowid`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []WorkRun
	for rows.Next() {
		run, err := scanWorkRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *Service) listWorkArtifacts(ctx context.Context, workID string) ([]WorkArtifact, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, work_id, COALESCE(run_id, ''), kind, uri, label, summary, workspace_revision, created_at
		FROM work_artifacts WHERE work_id = ? ORDER BY created_at, id`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var artifacts []WorkArtifact
	for rows.Next() {
		var artifact WorkArtifact
		var createdAt int64
		if err := rows.Scan(&artifact.ID, &artifact.WorkID, &artifact.RunID, &artifact.Kind, &artifact.URI, &artifact.Label, &artifact.Summary, &artifact.WorkspaceRevision, &createdAt); err != nil {
			return nil, err
		}
		artifact.CreatedAt = fromMillis(createdAt)
		artifacts = append(artifacts, artifact)
	}
	return artifacts, rows.Err()
}

func (s *Service) listWorkEvents(ctx context.Context, workID string) ([]WorkEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, work_id, kind, state, summary, goal_revision, candidate_revision, created_at
		FROM work_events WHERE work_id = ? ORDER BY created_at, id`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []WorkEvent
	for rows.Next() {
		var event WorkEvent
		var createdAt int64
		if err := rows.Scan(&event.ID, &event.WorkID, &event.Kind, &event.State, &event.Summary, &event.GoalRevision, &event.CandidateRevision, &createdAt); err != nil {
			return nil, err
		}
		event.CreatedAt = fromMillis(createdAt)
		events = append(events, event)
	}
	return events, rows.Err()
}
