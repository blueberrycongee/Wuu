package appserver

// Kanban-OS orchestration: RPC handlers for the agent-neutral task model and
// the dispatch path that binds a task to a named-agent execution. Dispatch is
// "create run with target" (the store action) plus spawning the execution
// site; terminal outcomes flow back through the agent notification hook in
// agent_threads.go.

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blueberrycongee/wuu/internal/agentcontrol"
	"github.com/blueberrycongee/wuu/internal/agentthread"
	"github.com/blueberrycongee/wuu/internal/kanban"
	"github.com/blueberrycongee/wuu/internal/participant"
	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/session"
	"github.com/blueberrycongee/wuu/internal/subagent"
)

// ---- wire shapes ----

type kanbanTaskWire struct {
	ID             string `json:"id"`
	SessionID      string `json:"session_id"`
	ParentID       string `json:"parent_id,omitempty"`
	Title          string `json:"title"`
	Brief          string `json:"brief,omitempty"`
	Status         string `json:"status"`
	SourceThreadID string `json:"source_thread_id,omitempty"`
	CreatedBy      string `json:"created_by,omitempty"`
	SortIndex      int    `json:"sort_index"`
	LatestRunID    string `json:"latest_run_id,omitempty"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

type kanbanRunWire struct {
	ID           string `json:"id"`
	TaskID       string `json:"task_id"`
	SessionID    string `json:"session_id"`
	Kind         string `json:"kind"`
	TargetID     string `json:"target_id"`
	ThreadID     string `json:"thread_id,omitempty"`
	Status       string `json:"status"`
	Summary      string `json:"summary,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	CreatedBy    string `json:"created_by,omitempty"`
	CreatedAt    int64  `json:"created_at"`
	StartedAt    int64  `json:"started_at,omitempty"`
	FinishedAt   int64  `json:"finished_at,omitempty"`
}

type kanbanArtifactWire struct {
	ID          string `json:"id"`
	RunID       string `json:"run_id"`
	TaskID      string `json:"task_id"`
	Path        string `json:"path"`
	DisplayName string `json:"display_name,omitempty"`
	MediaType   string `json:"media_type,omitempty"`
	SizeBytes   int64  `json:"size_bytes"`
	CreatedAt   int64  `json:"created_at"`
}

type kanbanTaskWithLatestRunWire struct {
	kanbanTaskWire
	LatestRun *kanbanRunWire `json:"latest_run,omitempty"`
}

type KanbanUpdatedNotification struct {
	SessionID string `json:"session_id"`
	TaskID    string `json:"task_id,omitempty"`
}

func kanbanTaskToWire(t kanban.Task) kanbanTaskWire {
	return kanbanTaskWire{
		ID: t.ID, SessionID: t.SessionID, ParentID: t.ParentID, Title: t.Title,
		Brief: t.Brief, Status: t.Status, SourceThreadID: t.SourceThreadID,
		CreatedBy: t.CreatedBy, SortIndex: t.SortIndex, LatestRunID: t.LatestRunID,
		CreatedAt: t.CreatedAt.UnixMilli(), UpdatedAt: t.UpdatedAt.UnixMilli(),
	}
}

func kanbanRunToWire(r kanban.Run) kanbanRunWire {
	return kanbanRunWire{
		ID: r.ID, TaskID: r.TaskID, SessionID: r.SessionID, Kind: r.Kind,
		TargetID: r.TargetID, ThreadID: r.ThreadID, Status: r.Status,
		Summary: r.Summary, ErrorMessage: r.ErrorMessage, CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt.UnixMilli(), StartedAt: r.StartedAt.UnixMilli(),
		FinishedAt: r.FinishedAt.UnixMilli(),
	}
}

func kanbanArtifactToWire(a kanban.Artifact) kanbanArtifactWire {
	return kanbanArtifactWire{
		ID: a.ID, RunID: a.RunID, TaskID: a.TaskID, Path: a.Path,
		DisplayName: a.DisplayName, MediaType: a.MediaType, SizeBytes: a.SizeBytes,
		CreatedAt: a.CreatedAt.UnixMilli(),
	}
}

func (s *Server) notifyKanbanUpdated(sessionID, taskID string) {
	_ = s.writeNotification(NotificationKanbanUpdated, KanbanUpdatedNotification{
		SessionID: sessionID, TaskID: taskID,
	})
}

// ---- task CRUD ----

type KanbanCreateTaskParams struct {
	SessionID      string `json:"session_id"`
	ParentID       string `json:"parent_id,omitempty"`
	Title          string `json:"title"`
	Brief          string `json:"brief,omitempty"`
	SourceThreadID string `json:"source_thread_id,omitempty"`
	Status         string `json:"status,omitempty"`
	CreatedBy      string `json:"created_by,omitempty"`
	SortIndex      int    `json:"sort_index,omitempty"`
}

func (s *Server) handleKanbanCreateTask(req Request) error {
	var params KanbanCreateTaskParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	task, err := session.CreateKanbanTask(s.rt.SessionDir, kanban.Task{
		SessionID:      params.SessionID,
		ParentID:       strings.TrimSpace(params.ParentID),
		Title:          params.Title,
		Brief:          params.Brief,
		SourceThreadID: strings.TrimSpace(params.SourceThreadID),
		Status:         strings.TrimSpace(params.Status),
		CreatedBy:      strings.TrimSpace(params.CreatedBy),
		SortIndex:      params.SortIndex,
	})
	if err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	s.notifyKanbanUpdated(task.SessionID, task.ID)
	return s.writeResponse(req.ID, kanbanTaskToWire(task), nil)
}

type KanbanListTasksParams struct {
	SessionID string `json:"session_id"`
	ParentID  string `json:"parent_id,omitempty"`
}

func (s *Server) handleKanbanListTasks(req Request) error {
	var params KanbanListTasksParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	if strings.TrimSpace(params.SessionID) == "" {
		return s.writeResponse(req.ID, nil, errors.New("session_id is required"))
	}
	tasks, err := session.ListKanbanTasks(s.rt.SessionDir, params.SessionID, strings.TrimSpace(params.ParentID))
	if err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	out := make([]kanbanTaskWithLatestRunWire, 0, len(tasks))
	for _, t := range tasks {
		w := kanbanTaskWithLatestRunWire{kanbanTaskWire: kanbanTaskToWire(t)}
		if t.LatestRunID != "" {
			if run, err := session.GetKanbanRun(s.rt.SessionDir, t.LatestRunID); err == nil {
				rw := kanbanRunToWire(run)
				w.LatestRun = &rw
			}
		}
		out = append(out, w)
	}
	return s.writeResponse(req.ID, out, nil)
}

type KanbanTransitionTaskParams struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

func (s *Server) handleKanbanTransitionTask(req Request) error {
	var params KanbanTransitionTaskParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	task, err := session.TransitionKanbanTaskStatus(s.rt.SessionDir, strings.TrimSpace(params.TaskID), strings.TrimSpace(params.Status))
	if err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	s.notifyKanbanUpdated(task.SessionID, task.ID)
	return s.writeResponse(req.ID, kanbanTaskToWire(task), nil)
}

type KanbanListRunsParams struct {
	TaskID string `json:"task_id"`
}

func (s *Server) handleKanbanListRuns(req Request) error {
	var params KanbanListRunsParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	runs, err := session.ListKanbanRuns(s.rt.SessionDir, strings.TrimSpace(params.TaskID))
	if err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	out := make([]kanbanRunWire, 0, len(runs))
	for _, r := range runs {
		out = append(out, kanbanRunToWire(r))
	}
	return s.writeResponse(req.ID, out, nil)
}

type KanbanListArtifactsParams struct {
	TaskID string `json:"task_id"`
}

func (s *Server) handleKanbanListArtifacts(req Request) error {
	var params KanbanListArtifactsParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	artifacts, err := session.ListKanbanArtifacts(s.rt.SessionDir, strings.TrimSpace(params.TaskID))
	if err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	out := make([]kanbanArtifactWire, 0, len(artifacts))
	for _, a := range artifacts {
		out = append(out, kanbanArtifactToWire(a))
	}
	return s.writeResponse(req.ID, out, nil)
}

// ---- dispatch ----

type KanbanDispatchRunParams struct {
	ThreadID  string `json:"thread_id"` // host conversation thread that owns the spawned execution
	TaskID    string `json:"task_id"`
	TargetID  string `json:"target_id"`
	Kind      string `json:"kind,omitempty"`
	CreatedBy string `json:"created_by,omitempty"`
}

func (s *Server) handleKanbanDispatchRun(ctx context.Context, req Request) error {
	var params KanbanDispatchRunParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	threadID := strings.TrimSpace(params.ThreadID)
	if threadID == "" {
		return s.writeResponse(req.ID, nil, errors.New("thread_id is required"))
	}
	run, err := s.dispatchKanbanRun(ctx, threadID, strings.TrimSpace(params.TaskID),
		strings.TrimSpace(params.TargetID), strings.TrimSpace(params.Kind), strings.TrimSpace(params.CreatedBy))
	if err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	return s.writeResponse(req.ID, kanbanRunToWire(run), nil)
}

// dispatchKanbanRun is the full dispatch path: record the run (the dispatch
// action itself), then spawn the target's execution site. Any failure after
// the run is recorded completes the run as failed so the task never parks in
// running with no execution behind it.
func (s *Server) dispatchKanbanRun(ctx context.Context, hostThreadID, taskID, targetID, kind, createdBy string) (kanban.Run, error) {
	task, err := session.GetKanbanTask(s.rt.SessionDir, taskID)
	if err != nil {
		return kanban.Run{}, err
	}
	run, err := session.CreateKanbanRun(s.rt.SessionDir, kanban.Run{
		TaskID: taskID, TargetID: targetID, Kind: kind, CreatedBy: createdBy,
	})
	if err != nil {
		return kanban.Run{}, err
	}
	fail := func(cause error) (kanban.Run, error) {
		if _, cerr := session.CompleteKanbanRun(s.rt.SessionDir, run.ID, kanban.RunStatusFailed, "", cause.Error()); cerr != nil {
			providers.DebugLogf("complete dispatch-failed kanban run %s: %v", run.ID, cerr)
		}
		s.notifyKanbanUpdated(task.SessionID, task.ID)
		return kanban.Run{}, cause
	}

	p, err := session.GetParticipant(s.rt.SessionDir, targetID)
	if err != nil {
		return fail(err)
	}
	if p.Kind != participant.KindNamed {
		return fail(fmt.Errorf("participant %q is not a named agent", targetID))
	}
	if s.residentDraining(targetID) || s.participantIsBusy(targetID) {
		return fail(fmt.Errorf("participant %q is busy; wait for it to finish", firstNonEmpty(p.Name, targetID)))
	}

	workspace, err := s.resolvedParticipantWorkspace(p)
	if err != nil {
		return fail(err)
	}
	outputDir := filepath.Join(workspace, "kanban-runs", run.ID)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fail(fmt.Errorf("create run output dir: %w", err))
	}
	memory, err := s.readParticipantMemory(p)
	if err != nil {
		return fail(err)
	}
	priorArtifacts, err := session.ListKanbanArtifacts(s.rt.SessionDir, task.ID)
	if err != nil {
		return fail(err)
	}
	prompt := namedParticipantPrompt(p, memory, kanbanRunRequestPrompt(task, run, priorArtifacts, outputDir), s.registeredWorkspaces())

	th, err := s.ensureResidentThread(hostThreadID)
	if err != nil {
		return fail(err)
	}
	threadRuntime, err := s.ensureThreadRuntimeAfterAdmission(th)
	if err != nil {
		return fail(err)
	}
	if threadRuntime == nil || threadRuntime.AgentControl == nil {
		return fail(errors.New("kanban dispatch requires agent control"))
	}
	if !s.tryAcquireParticipantBusy(targetID, "task") {
		return fail(fmt.Errorf("participant %q is busy running another task", firstNonEmpty(p.Name, targetID)))
	}
	spawned, err := threadRuntime.AgentControl.Spawn(ctx, agentcontrol.SpawnRequest{
		Type:             resolveParticipantSubagentType("", p),
		TaskName:         p.Name,
		ParticipantID:    targetID,
		AgentProfile:     p.Name,
		Description:      firstNonEmpty(p.Tagline, task.Title),
		Prompt:           prompt,
		ParentID:         hostThreadID,
		ParentPath:       agentthread.RootPath,
		SpeechCapability: false,
		Synchronous:      false,
	})
	if err != nil {
		s.releaseParticipantBusy(targetID, "")
		return fail(err)
	}
	s.bindParticipantBusyAgent(targetID, spawned.AgentID)
	threadRuntime.AgentControl.SetAgentParticipantID(spawned.AgentID, targetID)
	_ = session.UpsertParticipantRun(s.rt.SessionDir, session.ParticipantRun{
		ID:            spawned.AgentID,
		ParticipantID: targetID,
		AgentID:       spawned.AgentID,
		TaskID:        task.Title,
		SessionID:     hostThreadID,
		Summary:       task.Title,
		Outcome:       "running",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	})
	run, err = session.StartKanbanRun(s.rt.SessionDir, run.ID, spawned.AgentID)
	if err != nil {
		providers.DebugLogf("bind kanban run %s to agent %s: %v", run.ID, spawned.AgentID, err)
	}
	s.notifyKanbanUpdated(task.SessionID, task.ID)
	return run, nil
}

// kanbanRunRequestPrompt renders the per-run request: the distilled brief is
// the authority, prior artifacts and the source conversation are lazy
// references, and the output contract points at the run's output dir.
func kanbanRunRequestPrompt(task kanban.Task, run kanban.Run, priorArtifacts []kanban.Artifact, outputDir string) string {
	var b strings.Builder
	if run.Kind == kanban.RunKindReview {
		fmt.Fprintf(&b, "# Verification task: %s\n\n", strings.TrimSpace(task.Title))
		b.WriteString("You are the second pair of eyes on this task. You did not do the\n")
		b.WriteString("work and share none of the author's assumptions — that is the point.\n")
		b.WriteString("Verify the deliverables against the brief below and report concrete\n")
		b.WriteString("problems (missing requirements, defects, inconsistencies). Do not\n")
		b.WriteString("rewrite the deliverables yourself.\n\n")
	} else {
		fmt.Fprintf(&b, "# Task: %s\n\n", strings.TrimSpace(task.Title))
	}
	if brief := strings.TrimSpace(task.Brief); brief != "" {
		b.WriteString("## Brief\n\n")
		b.WriteString(brief)
		b.WriteString("\n\n")
	}
	b.WriteString("## References\n\n")
	if len(priorArtifacts) == 0 {
		b.WriteString("No prior artifacts from earlier runs of this task.\n")
	} else {
		b.WriteString("Artifacts from earlier runs of this task (read them only if useful):\n")
		for _, a := range priorArtifacts {
			fmt.Fprintf(&b, "- %s\n", a.Path)
		}
	}
	if threadID := strings.TrimSpace(task.SourceThreadID); threadID != "" {
		fmt.Fprintf(&b, "\nThis task crystallized from conversation thread %s; the brief above is the\n", threadID)
		b.WriteString("settled conclusion and normally all you need.\n")
	}
	b.WriteString("\n## Output contract\n\n")
	fmt.Fprintf(&b, "Write every deliverable file into this directory (it already exists):\n`%s`\n\n", outputDir)
	b.WriteString("Your final message must be a short summary (a few sentences) of what you\n")
	b.WriteString("did and the key decisions; it becomes the run record the human reviews.\n")
	return b.String()
}

// ---- terminal hook ----

// completeKanbanRunForAgent folds a spawned execution's terminal outcome back
// into its kanban run, collects produced artifacts, and notifies the board.
// It no-ops for participant runs that are not kanban executions.
func (s *Server) completeKanbanRunForAgent(participantID, agentID string, status subagent.Status, summary string) {
	if s.rt == nil || strings.TrimSpace(s.rt.SessionDir) == "" {
		return
	}
	run, err := session.GetActiveKanbanRunByThreadID(s.rt.SessionDir, agentID)
	if errors.Is(err, kanban.ErrRunNotFound) {
		return
	}
	if err != nil {
		providers.DebugLogf("lookup kanban run for agent %s: %v", agentID, err)
		return
	}
	runStatus := kanban.RunStatusInterrupted
	errorMessage := ""
	switch status {
	case subagent.StatusCompleted:
		runStatus = kanban.RunStatusSucceeded
	case subagent.StatusFailed:
		runStatus = kanban.RunStatusFailed
		errorMessage = summary
	}
	s.collectKanbanRunArtifacts(participantID, run)
	if _, err := session.CompleteKanbanRun(s.rt.SessionDir, run.ID, runStatus, summary, errorMessage); err != nil {
		providers.DebugLogf("complete kanban run %s for agent %s: %v", run.ID, agentID, err)
		return
	}
	s.notifyKanbanUpdated(run.SessionID, run.TaskID)
}

// collectKanbanRunArtifacts attributes every file the run left in its output
// dir to the run and its task. Best-effort: scan failures only log.
func (s *Server) collectKanbanRunArtifacts(participantID string, run kanban.Run) {
	p, err := session.GetParticipant(s.rt.SessionDir, participantID)
	if err != nil {
		providers.DebugLogf("load participant %s for artifact scan: %v", participantID, err)
		return
	}
	workspace, err := s.resolvedParticipantWorkspace(p)
	if err != nil {
		providers.DebugLogf("resolve participant %s workspace for artifact scan: %v", participantID, err)
		return
	}
	outputDir := filepath.Join(workspace, "kanban-runs", run.ID)
	_ = filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if strings.HasPrefix(name, ".") {
			return nil
		}
		rel, err := filepath.Rel(outputDir, path)
		if err != nil {
			return nil
		}
		if _, err := session.AddKanbanArtifact(s.rt.SessionDir, kanban.Artifact{
			RunID: run.ID, TaskID: run.TaskID, SessionID: run.SessionID,
			Path:        filepath.ToSlash(rel),
			DisplayName: name,
			MediaType:   mime.TypeByExtension(filepath.Ext(name)),
			SizeBytes:   info.Size(),
		}); err != nil {
			providers.DebugLogf("record kanban artifact %s for run %s: %v", rel, run.ID, err)
		}
		return nil
	})
}
