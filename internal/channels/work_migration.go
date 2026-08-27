package channels

import (
	"fmt"
	"strings"
)

func (s *Service) migrateInternalDeliveries() error {
	var schema string
	if err := s.db.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'collaboration_messages'`).Scan(&schema); err != nil {
		return fmt.Errorf("inspect internal delivery schema: %w", err)
	}
	if strings.Contains(schema, "'assignment'") && strings.Contains(schema, "artifact_refs_json") {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin internal delivery migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DROP INDEX IF EXISTS idx_collaboration_inbox`); err != nil {
		return fmt.Errorf("drop internal delivery index: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE collaboration_messages RENAME TO collaboration_messages_envelope_legacy`); err != nil {
		return fmt.Errorf("rename internal deliveries: %w", err)
	}
	if _, err := tx.Exec(`
		CREATE TABLE collaboration_messages (
			id TEXT PRIMARY KEY,
			room_id TEXT NOT NULL,
			from_type TEXT NOT NULL CHECK (from_type IN ('human', 'agent')),
			from_id TEXT NOT NULL,
			from_session_ref TEXT,
			to_agent_id TEXT NOT NULL,
			target_session_ref TEXT,
			kind TEXT NOT NULL DEFAULT 'control' CHECK (kind IN ('control', 'assignment', 'peer_result', 'candidate_ready', 'verification_feedback', 'completion')),
			body TEXT NOT NULL,
			work_id TEXT,
			source_message_id TEXT,
			goal_revision INTEGER NOT NULL DEFAULT 0,
			candidate_revision INTEGER NOT NULL DEFAULT 0,
			artifact_refs_json TEXT NOT NULL DEFAULT '[]',
			reply_to TEXT,
			created_at INTEGER NOT NULL,
			pulled_at INTEGER,
			consumed_at INTEGER,
			invalidated_at INTEGER,
			FOREIGN KEY (room_id) REFERENCES rooms(id) ON DELETE CASCADE,
			FOREIGN KEY (to_agent_id) REFERENCES collaboration_principals(id) ON DELETE CASCADE,
			FOREIGN KEY (work_id) REFERENCES works(id) ON DELETE CASCADE,
			FOREIGN KEY (source_message_id) REFERENCES room_messages(id) ON DELETE CASCADE,
			FOREIGN KEY (reply_to) REFERENCES collaboration_messages(id)
		)`); err != nil {
		return fmt.Errorf("create internal delivery envelopes: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO collaboration_messages(
			id, room_id, from_type, from_id, from_session_ref, to_agent_id, target_session_ref, kind, body, work_id, source_message_id,
			goal_revision, candidate_revision, artifact_refs_json, reply_to, created_at, pulled_at,
			consumed_at, invalidated_at
		)
		SELECT id, room_id, from_type, from_id, from_session_ref, to_agent_id, target_session_ref, kind, body, work_id, source_message_id,
			goal_revision, candidate_revision, artifact_refs_json, reply_to, created_at, pulled_at,
			consumed_at, invalidated_at
		FROM collaboration_messages_envelope_legacy`); err != nil {
		return fmt.Errorf("copy internal delivery envelopes: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE collaboration_messages_envelope_legacy`); err != nil {
		return fmt.Errorf("drop legacy internal deliveries: %w", err)
	}
	if _, err := tx.Exec(`CREATE INDEX idx_collaboration_inbox ON collaboration_messages(to_agent_id, pulled_at, created_at)`); err != nil {
		return fmt.Errorf("create internal delivery index: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit internal delivery migration: %w", err)
	}
	return nil
}

func (s *Service) migrateWorks() error {
	now := toMillis(s.now())
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO works(
			id, room_id, source_message_id, owner_named_agent_id, title, brief,
			goal_revision, candidate_revision, state, verification_state,
			verification_required, verifier_attempts_used, candidates_used, created_at, updated_at
		)
		SELECT task.id, task.room_id, COALESCE(task.thread_id, task.id), task.task_owner,
			COALESCE(task.task_title, ''), task.body,
			CASE WHEN task.task_goal_revision > 0 THEN task.task_goal_revision ELSE 1 END,
			task.task_candidate_revision,
			CASE task.task_state
				WHEN 'doing' THEN 'working'
				WHEN 'checking' THEN 'checking'
				WHEN 'revising' THEN 'revising'
				WHEN 'needs_human' THEN 'needs_human'
				WHEN 'done' THEN 'completed'
				ELSE 'open'
			END,
			CASE
				WHEN task.task_verification_required = 0 THEN 'not_required'
				ELSE COALESCE(verification.decision, 'pending')
			END,
			task.task_verification_required,
			COALESCE(verification.attempt, 0),
			CASE WHEN task.task_candidate_revision > 0 THEN task.task_candidate_revision ELSE 0 END,
			task.created_at, ?
		FROM room_messages task
		LEFT JOIN task_verifications verification ON verification.task_id = task.id
		WHERE task.kind = 'task' AND task.task_owner IS NOT NULL`, now)
	if err != nil {
		return fmt.Errorf("backfill durable works: %w", err)
	}
	if _, err := s.db.Exec(`
		INSERT OR IGNORE INTO work_events(id, work_id, kind, state, summary, goal_revision, candidate_revision, created_at)
		SELECT 'event-initial-' || id, id, 'state', state, 'Recovered initial Work state', goal_revision, candidate_revision, created_at
		FROM works work
		WHERE NOT EXISTS (SELECT 1 FROM work_events event WHERE event.work_id = work.id)`); err != nil {
		return fmt.Errorf("backfill work state events: %w", err)
	}
	return nil
}
