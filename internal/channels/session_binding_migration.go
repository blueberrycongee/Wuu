package channels

import "fmt"

func (s *Service) migrateCollaborationSessions() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin collaboration session migration: %w", err)
	}
	defer tx.Rollback()
	for _, statement := range []string{
		`DROP INDEX IF EXISTS idx_work_runs_session`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_work_runs_active_session
			ON work_runs(session_ref)
			WHERE session_ref IS NOT NULL AND state IN ('queued', 'running')`,
		`CREATE INDEX IF NOT EXISTS idx_collaboration_inbox_session
			ON collaboration_messages(to_agent_id, target_session_ref, pulled_at, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_drafts_agent_session_state
			ON drafts(agent_id, session_ref, state, updated_at)`,
	} {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("migrate collaboration session indexes: %w", err)
		}
	}
	if _, err := tx.Exec(`
		UPDATE work_runs AS run
		SET named_agent_id = CASE
			WHEN EXISTS (SELECT 1 FROM named_agents agent WHERE agent.id = run.profile) THEN run.profile
			WHEN run.kind = 'producer' THEN (
				SELECT work.owner_named_agent_id FROM works work WHERE work.id = run.work_id
			)
			ELSE NULL
		END
		WHERE run.named_agent_id IS NULL`); err != nil {
		return fmt.Errorf("backfill work run named agents: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT OR IGNORE INTO collaboration_session_bindings(
			session_ref, principal_id, named_agent_id, room_id, work_id, run_id,
			purpose, state, created_at, updated_at
		)
		SELECT run.session_ref, run.named_agent_id, run.named_agent_id, work.room_id,
			run.work_id, run.id,
			CASE WHEN run.kind = 'verifier' THEN 'verification' ELSE 'work' END,
			CASE run.state
				WHEN 'queued' THEN 'running'
				WHEN 'running' THEN 'running'
				WHEN 'interrupted' THEN 'interrupted'
				ELSE 'idle'
			END,
			run.created_at, run.updated_at
		FROM work_runs run
		JOIN works work ON work.id = run.work_id
		WHERE run.session_ref IS NOT NULL AND run.named_agent_id IS NOT NULL`); err != nil {
		return fmt.Errorf("backfill collaboration session bindings: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit collaboration session migration: %w", err)
	}
	return nil
}
