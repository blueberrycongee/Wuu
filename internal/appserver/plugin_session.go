package appserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blueberrycongee/wuu/internal/agent"
	wuucontext "github.com/blueberrycongee/wuu/internal/context"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/runtime"
	"github.com/blueberrycongee/wuu/internal/session"
)

type pluginTurnReference struct {
	PluginID  string
	RequestID string
	QueueID   string
}

func clonePluginTurnReference(reference *pluginTurnReference) *pluginTurnReference {
	if reference == nil {
		return nil
	}
	cloned := *reference
	return &cloned
}

func (s *Server) notifyPluginTurnDiscarded(threadID string, entry queuedTurn, reason string) {
	reference := entry.snapshot.PluginTurn
	if reference == nil {
		return
	}
	s.notifyPluginTurnLifecycleAsync(reference.PluginID, pluginhost.AgentTurnLifecycleInput{
		RequestID: reference.RequestID, State: pluginhost.TurnLifecycleDiscarded,
		ThreadID: strings.TrimSpace(threadID), QueueID: reference.QueueID,
		Error: strings.TrimSpace(reason),
	})
}

func (s *Server) createPluginSession(_ context.Context, pluginID string, params pluginhost.SessionCreateParams) (pluginhost.SessionCreateResult, error) {
	if s == nil || s.closed.Load() {
		return pluginhost.SessionCreateResult{}, errServerClosed
	}
	pluginID = strings.TrimSpace(pluginID)
	params.RequestID = strings.TrimSpace(params.RequestID)
	params.Name = strings.TrimSpace(params.Name)
	params.Visibility = strings.TrimSpace(params.Visibility)
	params.ParentSessionID = strings.TrimSpace(params.ParentSessionID)
	params.ContextSource = strings.TrimSpace(params.ContextSource)
	params.Workspace = strings.TrimSpace(params.Workspace)
	params.ModelAlias = strings.TrimSpace(params.ModelAlias)
	if pluginID == "" {
		return pluginhost.SessionCreateResult{}, errors.New("plugin owner is required")
	}
	if params.RequestID == "" {
		return pluginhost.SessionCreateResult{}, errors.New("request_id is required")
	}
	if len([]byte(params.RequestID)) > pluginhost.MaxSessionSendRequestIDBytes {
		return pluginhost.SessionCreateResult{}, fmt.Errorf("request_id exceeds %d bytes", pluginhost.MaxSessionSendRequestIDBytes)
	}
	if params.Visibility != pluginhost.SessionVisibilityUser && params.Visibility != pluginhost.SessionVisibilityPlugin {
		return pluginhost.SessionCreateResult{}, errors.New("visibility must be user or plugin")
	}
	if params.ContextSource != pluginhost.SessionContextFresh && params.ContextSource != pluginhost.SessionContextFork {
		return pluginhost.SessionCreateResult{}, errors.New("context_source must be fresh or fork")
	}
	if params.ContextSource == pluginhost.SessionContextFork && params.ParentSessionID == "" {
		return pluginhost.SessionCreateResult{}, errors.New("parent_session_id is required for fork context")
	}
	if params.Workspace == "" {
		params.Workspace = "shared"
	}
	if params.Workspace != "shared" && params.Workspace != "worktree" {
		return pluginhost.SessionCreateResult{}, errors.New("workspace must be shared or worktree")
	}
	if params.Workspace == "worktree" && params.ContextSource != pluginhost.SessionContextFork {
		return pluginhost.SessionCreateResult{}, errors.New("worktree workspace requires fork context")
	}
	owner := "plugin:" + pluginID
	if existing, ok, err := session.FindManagedByRequest(s.rt.SessionDir, owner, params.RequestID); err != nil {
		return pluginhost.SessionCreateResult{}, err
	} else if ok {
		return pluginhost.SessionCreateResult{SessionID: existing.ID, Created: false}, nil
	}
	th, err := s.createPluginSessionThread(owner, params)
	if err != nil {
		if existing, ok, findErr := session.FindManagedByRequest(s.rt.SessionDir, owner, params.RequestID); findErr == nil && ok {
			return pluginhost.SessionCreateResult{SessionID: existing.ID, Created: false}, nil
		}
		return pluginhost.SessionCreateResult{}, err
	}
	return pluginhost.SessionCreateResult{SessionID: th.ID, Created: true}, nil
}

func (s *Server) listPluginSessions(_ context.Context, pluginID string, params pluginhost.SessionListParams) (pluginhost.SessionListResult, error) {
	pluginID = strings.TrimSpace(pluginID)
	parentID := strings.TrimSpace(params.ParentSessionID)
	if pluginID == "" {
		return pluginhost.SessionListResult{}, errors.New("plugin owner is required")
	}
	items, err := session.List(s.rt.SessionDir, 0)
	if err != nil {
		return pluginhost.SessionListResult{}, err
	}
	owner := "plugin:" + pluginID
	result := pluginhost.SessionListResult{Sessions: make([]pluginhost.SessionSummary, 0)}
	for _, item := range items {
		if item.Owner != owner || (parentID != "" && item.ParentID != parentID) {
			continue
		}
		state := pluginhost.TurnLifecycleCompleted
		if th := s.thread(item.ID); th != nil {
			th.mu.Lock()
			if th.running {
				state = pluginhost.TurnLifecycleRunning
			}
			th.mu.Unlock()
		}
		s.queuedTurnMu.Lock()
		if len(s.pendingQueuedTurns[item.ID]) != 0 {
			state = pluginhost.TurnLifecycleQueued
		}
		s.queuedTurnMu.Unlock()
		result.Sessions = append(result.Sessions, pluginhost.SessionSummary{SessionID: item.ID, Name: item.Title, ParentSessionID: item.ParentID, Visibility: item.Visibility, State: state, CreatedAt: item.CreatedAt.Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.Format(time.RFC3339Nano)})
	}
	return result, nil
}

func (s *Server) cancelPluginSession(_ context.Context, pluginID string, params pluginhost.SessionCancelParams) (pluginhost.SessionCancelResult, error) {
	pluginID = strings.TrimSpace(pluginID)
	sessionID := strings.TrimSpace(params.SessionID)
	turnID := strings.TrimSpace(params.TurnID)
	queueID := strings.TrimSpace(params.QueueID)
	if pluginID == "" || sessionID == "" {
		return pluginhost.SessionCancelResult{}, errors.New("plugin owner and session_id are required")
	}
	if turnID != "" && queueID != "" {
		return pluginhost.SessionCancelResult{}, errors.New("turn_id and queue_id are mutually exclusive")
	}
	metadata, ok, err := session.Find(s.rt.SessionDir, sessionID)
	if err != nil {
		return pluginhost.SessionCancelResult{}, err
	}
	if !ok {
		return pluginhost.SessionCancelResult{}, session.ErrSessionNotFound
	}
	if metadata.Owner != "plugin:"+pluginID {
		return pluginhost.SessionCancelResult{}, errors.New("plugin does not own the session")
	}
	if queueID != "" {
		entry, ok := s.removePluginQueuedTurn(sessionID, pluginID, queueID)
		if !ok {
			return pluginhost.SessionCancelResult{SessionID: sessionID, QueueID: queueID, Cancelled: false}, nil
		}
		s.notifyPluginTurnDiscarded(sessionID, entry, "queued plugin turn was cancelled")
		s.notifyQueuedTurnsDequeued(sessionID, []string{queueID})
		return pluginhost.SessionCancelResult{SessionID: sessionID, QueueID: queueID, Cancelled: true}, nil
	}
	if _, err := s.ensureThreadLoaded(sessionID); err != nil {
		return pluginhost.SessionCancelResult{}, err
	}
	cancelled, err := s.interruptThreadExecution(sessionID, "", turnID)
	if err != nil {
		return pluginhost.SessionCancelResult{}, err
	}
	return pluginhost.SessionCancelResult{SessionID: sessionID, TurnID: turnID, Cancelled: cancelled}, nil
}

func (s *Server) removePluginQueuedTurn(threadID, pluginID, queueID string) (queuedTurn, bool) {
	if s == nil {
		return queuedTurn{}, false
	}
	s.queuedTurnMu.Lock()
	defer s.queuedTurnMu.Unlock()
	pending := s.pendingQueuedTurns[threadID]
	for index, entry := range pending {
		if entry.id != queueID || entry.snapshot.PluginTurn == nil || entry.snapshot.PluginTurn.PluginID != pluginID {
			continue
		}
		remaining := append(append([]queuedTurn(nil), pending[:index]...), pending[index+1:]...)
		if len(remaining) == 0 {
			delete(s.pendingQueuedTurns, threadID)
		} else {
			s.pendingQueuedTurns[threadID] = remaining
		}
		return entry, true
	}
	return queuedTurn{}, false
}

func (s *Server) sendPluginSession(ctx context.Context, pluginID string, params pluginhost.SessionSendParams) (pluginhost.SessionSendResult, error) {
	if s == nil || s.closed.Load() {
		return pluginhost.SessionSendResult{}, errServerClosed
	}
	pluginID = strings.TrimSpace(pluginID)
	params.RequestID = strings.TrimSpace(params.RequestID)
	params.SessionID = strings.TrimSpace(params.SessionID)
	params.Input.Prompt = strings.TrimSpace(params.Input.Prompt)
	params.Cause = strings.TrimSpace(params.Cause)
	params.IfRunning = strings.TrimSpace(params.IfRunning)
	if pluginID == "" || params.RequestID == "" || params.SessionID == "" || params.Input.Prompt == "" {
		return pluginhost.SessionSendResult{}, errors.New("plugin owner, request_id, session_id, and input.prompt are required")
	}
	if params.IfRunning == "" {
		params.IfRunning = pluginhost.SessionIfRunningQueue
	}
	if params.IfRunning != pluginhost.SessionIfRunningQueue && params.IfRunning != pluginhost.SessionIfRunningSteer {
		return pluginhost.SessionSendResult{}, errors.New("if_running must be queue or steer")
	}
	if params.IfRunning == pluginhost.SessionIfRunningSteer && len(params.Input.ContextBlocks) > 0 {
		return pluginhost.SessionSendResult{}, errors.New("context_blocks cannot be used when if_running is steer")
	}
	if len([]byte(params.RequestID)) > pluginhost.MaxSessionSendRequestIDBytes {
		return pluginhost.SessionSendResult{}, fmt.Errorf("request_id exceeds %d bytes", pluginhost.MaxSessionSendRequestIDBytes)
	}
	metadata, ok, err := session.Find(s.rt.SessionDir, params.SessionID)
	if err != nil {
		return pluginhost.SessionSendResult{}, err
	}
	if !ok {
		return pluginhost.SessionSendResult{}, session.ErrSessionNotFound
	}
	owner := "plugin:" + pluginID
	if metadata.Visibility == pluginhost.SessionVisibilityPlugin && metadata.Owner != owner {
		return pluginhost.SessionSendResult{}, errors.New("plugin does not own the private session")
	}
	requestContext, err := pluginTurnRequestContext(params.Input.ContextBlocks)
	if err != nil {
		return pluginhost.SessionSendResult{}, err
	}
	th, err := s.ensureThreadLoaded(params.SessionID)
	if err != nil {
		return pluginhost.SessionSendResult{}, err
	}
	clientID := pluginSessionRequestClientID(pluginID, params.RequestID)
	if existing, ok := s.findPluginSessionRequest(th, clientID); ok {
		return existing, nil
	}
	msg, err := userMessageFromPrompt(params.Input.Prompt, nil, nil)
	if err != nil {
		return pluginhost.SessionSendResult{}, err
	}
	msg.ClientID = clientID
	msg.Origin = pluginhost.SessionInputPlugin
	msg.OriginID = pluginID
	msg.Cause = params.Cause
	msg.PresentationKind = pluginhost.SessionPresentationQueryBubble
	msg.ReadOnly = true
	msg.DisplayContent = "插件已唤醒 Agent"
	if params.Presentation != nil {
		if kind := strings.TrimSpace(params.Presentation.Kind); kind != "" && kind != pluginhost.SessionPresentationQueryBubble {
			return pluginhost.SessionSendResult{}, errors.New("presentation.kind must be query_bubble")
		}
		if text := strings.TrimSpace(params.Presentation.Text); text != "" {
			msg.DisplayContent = text
		}
		msg.Name = strings.TrimSpace(params.Presentation.Name)
	}
	if params.IfRunning == pluginhost.SessionIfRunningSteer {
		if turnID, steered := s.steerPluginSession(th, msg); steered {
			return pluginhost.SessionSendResult{
				State: pluginhost.TurnLifecycleRunning, SessionID: th.ID, TurnID: turnID, Steered: true,
			}, nil
		}
	}
	snapshot := turnRuntimeSnapshot{}.withPermissions(normalizeTurnPermissions(s.rt.Permissions))
	snapshot.RequestContext = requestContext
	snapshot.PluginTurn = &pluginTurnReference{PluginID: pluginID, RequestID: params.RequestID}

	started, ok, err := s.startPluginSubmittedTurn(ctx, th, msg, snapshot)
	if err != nil {
		return pluginhost.SessionSendResult{}, err
	}
	if ok {
		return pluginhost.SessionSendResult{State: pluginhost.TurnLifecycleRunning, SessionID: th.ID, TurnID: started.turnID}, nil
	}

	queueID := session.NewID()
	snapshot.PluginTurn.QueueID = queueID
	entry := queuedTurn{id: queueID, msg: msg, snapshot: snapshot}
	s.enqueueQueuedUserTurn(th.ID, entry)
	queued := queuedTurnSummary(th.ID, entry)
	_ = s.writeNotification(NotificationTurnQueued, TurnQueuedNotification{Queued: queued})
	s.kickQueuedTurnDrain(th.ID)
	return pluginhost.SessionSendResult{State: pluginhost.TurnLifecycleQueued, SessionID: th.ID, QueueID: queueID}, nil
}

func pluginSessionRequestClientID(pluginID, requestID string) string {
	return "plugin:" + strings.TrimSpace(pluginID) + ":" + strings.TrimSpace(requestID)
}

func (s *Server) steerPluginSession(th *threadState, msg providers.ChatMessage) (string, bool) {
	if th == nil {
		return "", false
	}
	th.mu.Lock()
	defer th.mu.Unlock()
	if !th.running || th.currentTurn == "" || th.currentTurnKind == TurnKindCompact || th.interrupting {
		return "", false
	}
	for _, existing := range th.pendingSteers {
		if existing.ClientID == msg.ClientID {
			return th.currentTurn, true
		}
	}
	msg.Steered = true
	th.pendingSteers = append(th.pendingSteers, msg)
	th.signalSteerWakeLocked()
	return th.currentTurn, true
}

func (s *Server) findPluginSessionRequest(th *threadState, clientID string) (pluginhost.SessionSendResult, bool) {
	if th == nil || strings.TrimSpace(clientID) == "" {
		return pluginhost.SessionSendResult{}, false
	}
	th.mu.Lock()
	for _, pending := range th.pendingSteers {
		if pending.ClientID == clientID {
			result := pluginhost.SessionSendResult{
				State: pluginhost.TurnLifecycleRunning, SessionID: th.ID, TurnID: th.currentTurn, Steered: true,
			}
			th.mu.Unlock()
			return result, true
		}
	}
	steered := false
	for _, message := range th.History {
		if message.ClientID == clientID && message.Steered {
			steered = true
			break
		}
	}
	for _, turn := range th.Turns {
		for _, item := range turn.Items {
			if item.Type != ThreadItemUserMessage || item.SourceID != clientID {
				continue
			}
			state := pluginhost.TurnLifecycleCompleted
			if turn.Status == TurnStatusInProgress {
				state = pluginhost.TurnLifecycleRunning
			}
			result := pluginhost.SessionSendResult{State: state, SessionID: th.ID, TurnID: turn.ID, Steered: steered}
			th.mu.Unlock()
			return result, true
		}
	}
	th.mu.Unlock()

	s.queuedTurnMu.Lock()
	defer s.queuedTurnMu.Unlock()
	for _, entry := range s.pendingQueuedTurns[th.ID] {
		if entry.msg.ClientID == clientID {
			return pluginhost.SessionSendResult{State: pluginhost.TurnLifecycleQueued, SessionID: th.ID, QueueID: entry.id}, true
		}
	}
	return pluginhost.SessionSendResult{}, false
}

func (s *Server) createPluginSessionThread(owner string, params pluginhost.SessionCreateParams) (*threadState, error) {
	if s.rt == nil || s.rt.StreamRunner == nil {
		return nil, errors.New("runtime session is required")
	}
	id := session.NewID()
	threadCWD := s.rt.RootDir
	managed := session.ManagedMetadata{Owner: owner, Visibility: params.Visibility, ParentID: params.ParentSessionID, ContextSource: params.ContextSource, CreationRequestID: params.RequestID}
	var history []providers.ChatMessage
	var createdWorktreePath string
	cleanupWorktree := false
	fork := session.ForkMetadata{}
	if params.ContextSource == pluginhost.SessionContextFork {
		parent, loadErr := s.loadPersistedThreadSnapshot(params.ParentSessionID)
		if loadErr != nil {
			return nil, loadErr
		}
		threadCWD = firstNonEmpty(parent.metadata.CWD, threadCWD)
		history = cloneForkHistory(parent.history)
		fork = session.ForkMetadata{ForkedFromID: params.ParentSessionID}
	}
	selection := s.currentSessionRuntimeSelection()
	if params.ModelAlias != "" {
		resolved := s.resolveSubagentModelAlias(params.ModelAlias)
		if resolved.Err != nil {
			return nil, resolved.Err
		}
		if !resolved.Found {
			return nil, fmt.Errorf("unknown model alias %q (available: %s)", params.ModelAlias, strings.Join(resolved.ValidAliases, ", "))
		}
		selection.Provider = resolved.Runtime.Provider
		selection.Model = resolved.Runtime.Model
		selection.Variant = resolved.Runtime.Variant
		selection.Effort = resolved.Runtime.Effort
	}
	source := owner
	workspaceID := strings.TrimSpace(s.rt.WorkspaceID)
	if len(history) == 0 {
		history = make([]providers.ChatMessage, 0, 1)
	}
	if prompt := strings.TrimSpace(s.rt.StreamRunner.SystemPrompt); prompt != "" && len(history) == 0 {
		history = append(history, providers.ChatMessage{Role: "system", Content: prompt})
	}

	worktree := session.WorktreeInfo{}
	if params.Workspace == "worktree" {
		baseRepo := threadCWD
		manager, err := s.worktreeManager(baseRepo)
		if err != nil {
			return nil, err
		}
		createdWorktree, err := manager.Create(id, "plugin", "")
		if err != nil {
			return nil, err
		}
		createdWorktreePath = createdWorktree.Path
		cleanupWorktree = true
		threadCWD = createdWorktree.Path
		worktree = session.WorktreeInfo{Path: createdWorktree.Path, BaseHEAD: createdWorktree.HEAD, BaseRepo: baseRepo}
		defer func() {
			if cleanupWorktree {
				_ = manager.Cleanup(createdWorktree)
			}
		}()
	}
	initial := session.Session{
		ID: id, Title: params.Name, CWD: threadCWD,
		WorkspaceID: workspaceID, Source: source,
		Owner: managed.Owner, Visibility: managed.Visibility, ParentID: managed.ParentID,
		ContextSource: managed.ContextSource, CreationRequestID: managed.CreationRequestID,
		ForkedFromID: fork.ForkedFromID,
		WorktreePath: worktree.Path, WorktreeBaseHEAD: worktree.BaseHEAD, WorktreeBaseRepo: worktree.BaseRepo,
		Provider: selection.Provider, Model: selection.Model, Variant: selection.Variant,
		Effort: selection.Effort, PermissionMode: selection.PermissionMode,
	}
	var records []session.HistoryRecord
	if params.ContextSource == pluginhost.SessionContextFork {
		records = historyRecordsFromChatMessages(history)
	}
	created, err := session.CreateInitialized(s.rt.SessionDir, initial, records)
	if err != nil {
		return nil, err
	}
	cleanupWorktree = false
	th := newThreadState(id, history, s.rt.ProviderName, s.rt.Model, threadCWD, true, time.Now().UTC())
	applyThreadRuntimeSelection(th, selection)
	th.Source = source
	th.Title = params.Name
	th.Owner = owner
	th.Visibility = params.Visibility
	th.ParentID = params.ParentSessionID
	th.WorktreePath = createdWorktreePath
	if created != nil {
		th.WorktreeBaseHEAD = created.WorktreeBaseHEAD
		th.WorktreeBaseRepo = created.WorktreeBaseRepo
	}
	th.WorkspaceKind = workspaceKindForCWD(s.rt.WuuHome, threadCWD)
	if workspaceID != "" {
		th.WorkspaceKind = WorkspaceKindProject
	}
	s.mu.Lock()
	s.threads[id] = th
	s.mu.Unlock()
	th.mu.Lock()
	thread := th.snapshotLocked()
	th.mu.Unlock()
	if params.Visibility == pluginhost.SessionVisibilityUser {
		if err := s.notifyThreadStarted(thread); err != nil {
			providers.DebugLogf("notify plugin-created thread %q: %v", id, err)
		}
	}
	s.pruneCachedThreads(id)
	return th, nil
}

func (s *Server) startPluginSubmittedTurn(ctx context.Context, th *threadState, msg providers.ChatMessage, snapshot turnRuntimeSnapshot) (startedThreadTurn, bool, error) {
	var threadRuntime *runtime.ThreadRuntime
	started, ok, err := s.startThreadUserTurnWithAdmission(
		ctx, th, msg, snapshot, false, turnReadOnlyFail,
		turnAdmissionHooks{afterLease: func(admitted *threadState, _ *providers.ChatMessage) error {
			var runtimeErr error
			threadRuntime, runtimeErr = s.ensureThreadRuntimeAfterAdmission(admitted)
			if runtimeErr == nil {
				s.foldFrozenWorkerTree(admitted, threadRuntime)
			}
			return runtimeErr
		}},
	)
	if err != nil {
		if errors.Is(err, errThreadExecutionBusy) {
			return startedThreadTurn{}, false, nil
		}
		return startedThreadTurn{}, false, err
	}
	if !ok {
		return startedThreadTurn{}, false, nil
	}
	launch, accepted := s.reserveBackground(func() {
		s.runTurn(started.ctx, th, threadRuntime, started.turnID, started.runtime, started.history)
	})
	if !accepted {
		return startedThreadTurn{}, false, errors.Join(errServerClosed, s.abortStartedThreadTurnDurably(th, started, errServerClosed))
	}
	defer launch.Cancel()
	if err := s.writeNotification(NotificationTurnStarted, TurnStartedNotification{ThreadID: th.ID, Turn: started.turn}); err != nil {
		return startedThreadTurn{}, false, errors.Join(err, s.abortStartedThreadTurnDurably(th, started, err))
	}
	launch.Commit()
	return started, true, nil
}

func pluginTurnRequestContext(input []pluginhost.SessionContextBlock) ([]agent.ContextSegment, error) {
	if len(input) > pluginhost.MaxSessionSendContextBlocks {
		return nil, fmt.Errorf("context_blocks exceeds %d entries", pluginhost.MaxSessionSendContextBlocks)
	}
	blocks := make([]wuucontext.Block, 0, len(input))
	total := 0
	for _, block := range input {
		content := strings.TrimSpace(block.Content)
		if content == "" {
			return nil, errors.New("context block content is required")
		}
		size := len([]byte(content))
		if size > pluginhost.MaxSessionSendContextBlockBytes {
			return nil, fmt.Errorf("context block exceeds %d bytes", pluginhost.MaxSessionSendContextBlockBytes)
		}
		total += size
		if total > pluginhost.MaxSessionSendContextTotalBytes {
			return nil, fmt.Errorf("context_blocks exceeds %d total bytes", pluginhost.MaxSessionSendContextTotalBytes)
		}
		kind := strings.TrimSpace(block.Kind)
		if kind == "" {
			kind = string(wuucontext.BlockAdditionalContext)
		}
		blocks = append(blocks, wuucontext.Block{
			Kind: wuucontext.BlockKind(kind), Title: strings.TrimSpace(block.Title),
			Source: strings.TrimSpace(block.Source), Content: content,
		})
	}
	return agent.RequestOnlyContextBlocks(blocks), nil
}
