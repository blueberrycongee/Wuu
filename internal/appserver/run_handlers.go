package appserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blueberrycongee/wuu/internal/execution"
	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/runtime"
	"github.com/blueberrycongee/wuu/internal/version"
)

type runTracker struct {
	id           string
	threadID     string
	outputSchema json.RawMessage
}

func (s *Server) handleRunStart(ctx context.Context, req Request) error {
	var params RunStartParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeRunError(req.ID, "invalid_params", err)
	}
	if s.runStore == nil || s.rt == nil || s.rt.InferenceJournalRuntime == nil {
		return s.writeRunError(req.ID, "internal_error", errors.New("execution run store is unavailable"))
	}
	params.ThreadID = strings.TrimSpace(params.ThreadID)
	params.Prompt = strings.TrimSpace(params.Prompt)
	if params.Request.Mode == "" {
		params.Request.Mode = execution.ModeStart
	}
	if params.ThreadID == "" {
		return s.writeRunError(req.ID, "invalid_params", errors.New("thread_id is required"))
	}
	images, err := normalizeTurnStartImages(params.Images)
	if err != nil {
		return s.writeRunError(req.ID, "invalid_params", err)
	}
	files, err := normalizeTurnStartFiles(params.Files)
	if err != nil {
		return s.writeRunError(req.ID, "invalid_params", err)
	}
	if params.Prompt == "" && len(images) == 0 && len(files) == 0 {
		return s.writeRunError(req.ID, "invalid_params", errors.New("prompt or attachment is required"))
	}
	if isManualCompactPrompt(params.Prompt) {
		return s.writeRunError(req.ID, "invalid_params", errors.New("execution runs do not accept compact commands"))
	}
	th, err := s.ensureThreadLoaded(params.ThreadID)
	if err != nil {
		return s.writeRunError(req.ID, "thread_not_found", err)
	}
	permissions, err := s.resolveThreadTurnPermissions(th, params.PermissionMode)
	if err != nil {
		return s.writeRunError(req.ID, "invalid_params", err)
	}
	userMsg, err := userMessageFromPrompt(params.Prompt, images, files, s.rt.ExperimentalHelpMe)
	if err != nil {
		return s.writeRunError(req.ID, "invalid_params", err)
	}

	params.Request.HasPrompt = params.Prompt != ""
	params.Request.ImageCount = len(images)
	params.Request.FileCount = len(files)
	params.Request.StructuredOutput = len(params.OutputSchema) > 0
	workspace, initialRuntime, ephemeral := s.executionFactsForThread(th, nil, turnRuntimeSnapshot{}.withPermissions(permissions))
	run, err := s.runStore.Create(ctx, execution.CreateParams{
		RuntimeID: s.rt.InferenceJournalRuntime.RuntimeID(),
		Request:   params.Request,
		Runtime:   initialRuntime,
		Workspace: workspace,
		ThreadID:  params.ThreadID,
		Ephemeral: ephemeral,
	})
	if err != nil {
		code := "internal_error"
		if errors.Is(err, execution.ErrConflict) {
			code = "thread_busy"
		}
		return s.writeRunError(req.ID, code, err)
	}

	snapshot := turnRuntimeSnapshot{}.withPermissions(permissions)
	snapshot.PermissionExplicit = params.PermissionMode != nil
	snapshot.Ultra = s.rt.UltraMode()
	snapshot.ExecutionRunID = run.ID
	var threadRuntime *runtime.ThreadRuntime
	started, ok, admissionErr := s.startThreadUserTurnWithAdmission(
		ctx, th, userMsg, snapshot, true, turnReadOnlyIgnore,
		turnAdmissionHooks{afterLease: func(admitted *threadState, _ *providers.ChatMessage) error {
			var runtimeErr error
			threadRuntime, runtimeErr = s.ensureThreadRuntimeAfterAdmission(admitted)
			if runtimeErr == nil {
				s.foldFrozenWorkerTree(admitted, threadRuntime)
			}
			return runtimeErr
		}},
	)
	if admissionErr != nil || !ok {
		if admissionErr == nil {
			admissionErr = fmt.Errorf("thread %q already has a running turn", params.ThreadID)
		}
		_, _ = s.runStore.Fail(ctx, run.ID, execution.StatusCancelled, execution.Result{}, execution.Error{Code: "admission_failed", Category: "cancelled", Message: admissionErr.Error()}, time.Now().UTC())
		return s.writeRunError(req.ID, "thread_busy", admissionErr)
	}

	workspace, resolvedRuntime, _ := s.executionFactsForThread(th, threadRuntime, started.runtime)
	run, err = s.runStore.Resolve(ctx, run.ID, resolvedRuntime, workspace, time.Now().UTC())
	if err == nil {
		run, err = s.runStore.AttachTurn(ctx, run.ID, params.ThreadID, started.turnID, started.admittedAt)
	}
	if err != nil {
		persistErr := s.abortStartedThreadTurnDurably(th, started, err)
		_, _ = s.runStore.Fail(ctx, run.ID, execution.StatusFailed, execution.Result{}, execution.Error{Code: "manifest_update_failed", Category: "internal", Message: err.Error()}, time.Now().UTC())
		return s.writeRunError(req.ID, "internal_error", errors.Join(err, persistErr))
	}
	s.registerExecutionRun(run, params.OutputSchema)

	launch, accepted := s.reserveBackground(func() {
		s.runTurn(started.ctx, th, threadRuntime, started.turnID, started.runtime, started.history)
	})
	if !accepted {
		persistErr := s.abortStartedThreadTurnDurably(th, started, errServerClosed)
		s.failAndDetachExecutionRun(run.ID, execution.StatusInterrupted, "server_closed", "cancelled", errServerClosed)
		return s.writeRunError(req.ID, "server_closed", errors.Join(errServerClosed, persistErr))
	}
	defer launch.Cancel()

	if err := s.writeResponse(req.ID, RunStartResult{Run: run}, nil); err != nil {
		persistErr := s.abortStartedThreadTurnDurably(th, started, err)
		s.failAndDetachExecutionRun(run.ID, execution.StatusInterrupted, "response_failed", "protocol", err)
		return errors.Join(err, persistErr)
	}
	if err := s.writeNotification(NotificationRunStarted, RunStartedNotification{Run: run}); err != nil {
		persistErr := s.abortStartedThreadTurnDurably(th, started, err)
		s.failAndDetachExecutionRun(run.ID, execution.StatusInterrupted, "notification_failed", "protocol", err)
		return errors.Join(err, persistErr)
	}
	if err := s.writeNotification(NotificationTurnStarted, TurnStartedNotification{ThreadID: params.ThreadID, Turn: started.turn}); err != nil {
		persistErr := s.abortStartedThreadTurnDurably(th, started, err)
		s.failAndDetachExecutionRun(run.ID, execution.StatusInterrupted, "notification_failed", "protocol", err)
		return errors.Join(err, persistErr)
	}
	launch.Commit()
	return nil
}

func (s *Server) handleRunRead(ctx context.Context, req Request) error {
	var params RunReadParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeRunError(req.ID, "invalid_params", err)
	}
	view, err := s.readExecutionRun(ctx, strings.TrimSpace(params.RunID))
	if err != nil {
		code := "internal_error"
		if errors.Is(err, execution.ErrNotFound) {
			code = "run_not_found"
		}
		return s.writeRunError(req.ID, code, err)
	}
	return s.writeResponse(req.ID, RunReadResult(view), nil)
}

func (s *Server) handleRunList(ctx context.Context, req Request) error {
	var params RunListParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeRunError(req.ID, "invalid_params", err)
	}
	if s.runStore == nil {
		return s.writeRunError(req.ID, "internal_error", errors.New("execution run store is unavailable"))
	}
	runs, err := s.runStore.List(ctx, execution.ListOptions{
		WorkspaceID: strings.TrimSpace(params.WorkspaceID), WorkspaceRoot: strings.TrimSpace(params.WorkspaceRoot),
		ThreadID: strings.TrimSpace(params.ThreadID), Status: params.Status, Limit: params.Limit,
	})
	if err != nil {
		return s.writeRunError(req.ID, "internal_error", err)
	}
	return s.writeResponse(req.ID, RunListResult{Runs: runs}, nil)
}

func (s *Server) handleRunInterrupt(ctx context.Context, req Request) error {
	var params RunInterruptParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeRunError(req.ID, "invalid_params", err)
	}
	runID := strings.TrimSpace(params.RunID)
	if runID == "" {
		return s.writeRunError(req.ID, "invalid_params", errors.New("run_id is required"))
	}
	view, err := s.readExecutionRun(ctx, runID)
	if err != nil {
		if errors.Is(err, execution.ErrNotFound) {
			return s.writeRunError(req.ID, "run_not_found", err)
		}
		return s.writeRunError(req.ID, "internal_error", err)
	}
	if view.Run.Status.Terminal() {
		return s.writeResponse(req.ID, RunInterruptResult{Run: view.Run}, nil)
	}
	if !view.Attached {
		return s.writeRunError(req.ID, "run_not_attached", fmt.Errorf("run %q is not attached to this app-server", runID))
	}
	turnActive, err := s.interruptThreadExecution(view.Run.ThreadID)
	if err != nil {
		return s.writeRunError(req.ID, "internal_error", err)
	}
	if !turnActive {
		view.Run = s.failAndDetachExecutionRun(runID, execution.StatusInterrupted, "interrupted", "cancelled", errors.New("execution run interrupted"))
	}
	return s.writeResponse(req.ID, RunInterruptResult{Run: view.Run}, nil)
}

func (s *Server) readExecutionRun(ctx context.Context, runID string) (RunView, error) {
	if s == nil || s.runStore == nil {
		return RunView{}, errors.New("execution run store is unavailable")
	}
	run, err := s.runStore.Get(ctx, runID)
	if err != nil {
		return RunView{}, err
	}
	view := RunView{Run: run}
	if th := s.thread(run.ThreadID); th != nil {
		th.mu.Lock()
		thread := th.snapshotLocked()
		th.mu.Unlock()
		view.Thread = &thread
		view.Attached = s.executionRunAttached(run.ID)
		return view, nil
	}
	if !run.Ephemeral {
		if th, loadErr := s.loadPersistedThreadState(run.ThreadID, time.Now().UTC()); loadErr == nil {
			th.mu.Lock()
			thread := th.snapshotLocked()
			th.mu.Unlock()
			view.Thread = &thread
		}
	}
	return view, nil
}

func (s *Server) executionFactsForThread(th *threadState, threadRuntime *runtime.ThreadRuntime, snapshot turnRuntimeSnapshot) (execution.WorkspaceRef, execution.RuntimeManifest, bool) {
	workspace := execution.WorkspaceRef{Root: s.rt.RootDir}
	selection := execution.Selection{Provider: s.rt.ProviderName, Model: s.rt.Model, PermissionMode: snapshot.PermissionMode}
	ephemeral := false
	if th != nil {
		th.mu.Lock()
		workspace = execution.WorkspaceRef{ID: th.WorkspaceID, Root: firstNonEmpty(th.CWD, s.rt.RootDir)}
		selection.Provider = firstNonEmpty(th.ModelProvider, selection.Provider)
		selection.Model = firstNonEmpty(th.Model, selection.Model)
		selection.Variant = th.ModelVariant
		selection.Effort = th.ModelEffort
		selection.PermissionMode = firstNonEmpty(snapshot.PermissionMode, th.PermissionMode, s.rt.Permissions.Mode)
		ephemeral = th.Ephemeral
		th.mu.Unlock()
	}
	runner := s.rt.StreamRunner
	if threadRuntime != nil && threadRuntime.StreamRunner != nil {
		runner = threadRuntime.StreamRunner
	}
	if runner != nil {
		selection.Provider = firstNonEmpty(runner.ProviderName, selection.Provider)
		selection.Model = firstNonEmpty(runner.Model, selection.Model)
		selection.Variant = runner.Variant
		selection.Effort = runner.Effort
	}
	core := version.Info()
	return workspace, execution.RuntimeManifest{
		Resolved: selection, ProtocolVersion: ProtocolVersion, CoreVersion: core.Version, CoreCommit: core.Commit,
		Ultra: snapshot.Ultra, MaxParallel: s.rt.MaxParallel(),
	}, ephemeral
}

func (s *Server) registerExecutionRun(run execution.Run, outputSchema json.RawMessage) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	tracker := &runTracker{id: run.ID, threadID: run.ThreadID, outputSchema: append(json.RawMessage(nil), outputSchema...)}
	s.runs[run.ID] = tracker
	s.activeRunByThread[run.ThreadID] = run.ID
}

func (s *Server) executionRunAttached(runID string) bool {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	_, ok := s.runs[runID]
	return ok
}

func (s *Server) activeExecutionRunID(threadID string) string {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	return s.activeRunByThread[strings.TrimSpace(threadID)]
}

func (s *Server) attachExecutionTurn(runID, threadID, turnID string, at time.Time) error {
	if strings.TrimSpace(runID) == "" {
		return nil
	}
	_, err := s.runStore.AttachTurn(context.Background(), runID, threadID, turnID, at)
	return err
}

func (s *Server) settleExecutionRunTurn(runID, turnID, tracePath string, turn Turn, structured *TurnError, turnErr error, awaitingContinuation bool, at time.Time) (execution.Run, bool, error) {
	if strings.TrimSpace(runID) == "" {
		return execution.Run{}, false, nil
	}
	run, err := s.runStore.FinishTurn(context.Background(), runID, turnID, execution.TurnTerminal{TracePath: tracePath, At: at})
	if err != nil {
		return execution.Run{}, false, err
	}
	result := execution.Result{FinalTurnID: turnID, TracePath: tracePath}
	if turnErr != nil {
		status := execution.StatusFailed
		if turn.Status == TurnStatusInterrupted || errors.Is(turnErr, context.Canceled) {
			status = execution.StatusInterrupted
		}
		result.ExitCode = executionExitCode(status)
		runError := execution.Error{Code: "turn_failed", Category: "unknown", Message: turnErr.Error()}
		if structured != nil {
			runError = execution.Error{
				Code: structured.Code, Category: string(structured.Category), Message: structured.Message,
				Provider: structured.Provider, StatusCode: structured.StatusCode,
			}
		}
		run, err = s.runStore.Fail(context.Background(), runID, status, result, runError, at)
	} else if !awaitingContinuation {
		run, err = s.runStore.Complete(context.Background(), runID, result, at)
	}
	if err != nil {
		return execution.Run{}, false, err
	}
	terminal := run.Status.Terminal()
	if terminal {
		s.detachExecutionRun(runID)
	}
	return run, terminal, nil
}

func (s *Server) executionRunAwaitsContinuation(threadID string, threadRuntime *runtime.ThreadRuntime) bool {
	if threadRuntimeAwaitsAutoContinuation(threadID, threadRuntime) {
		return true
	}
	goal, ok, err := s.currentRuntimeGoal(threadID)
	if err != nil {
		providers.DebugLogf("inspect execution run goal continuation for thread %q: %v", threadID, err)
		return true
	}
	return ok && goal.CanAutoContinue()
}

func (s *Server) detachExecutionRun(runID string) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	tracker := s.runs[runID]
	delete(s.runs, runID)
	if tracker != nil && s.activeRunByThread[tracker.threadID] == runID {
		delete(s.activeRunByThread, tracker.threadID)
	}
}

func (s *Server) failAndDetachExecutionRun(runID string, status execution.Status, code, category string, cause error) execution.Run {
	message := "execution run failed"
	if cause != nil {
		message = cause.Error()
	}
	run, err := s.runStore.Fail(context.Background(), runID, status, execution.Result{ExitCode: executionExitCode(status)}, execution.Error{Code: code, Category: category, Message: message}, time.Now().UTC())
	if err != nil {
		providers.DebugLogf("settle execution run %q: %v", runID, err)
	}
	s.detachExecutionRun(runID)
	s.kickQueuedTurnDrain(run.ThreadID)
	return run
}

func executionExitCode(status execution.Status) int {
	switch status {
	case execution.StatusCompleted:
		return 0
	case execution.StatusInterrupted, execution.StatusCancelled:
		return 5
	case execution.StatusTimedOut:
		return 4
	default:
		return 1
	}
}

func (s *Server) interruptAttachedRunsOnClose() {
	s.runMu.Lock()
	ids := make([]string, 0, len(s.runs))
	for id := range s.runs {
		ids = append(ids, id)
	}
	s.runMu.Unlock()
	for _, id := range ids {
		s.failAndDetachExecutionRun(id, execution.StatusInterrupted, "server_closed", "cancelled", errServerClosed)
	}
}

func (s *Server) writeRunError(id json.RawMessage, code string, err error) error {
	resp := Response{ID: id, Error: &ResponseError{Code: code, Message: err.Error()}}
	return s.writeJSON(resp)
}
