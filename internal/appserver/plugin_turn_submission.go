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
	if err := s.notifyPluginTurnLifecycle(context.Background(), reference.PluginID, pluginhost.AgentTurnLifecycleInput{
		RequestID: reference.RequestID, State: pluginhost.TurnLifecycleDiscarded,
		ThreadID: strings.TrimSpace(threadID), QueueID: reference.QueueID,
		Error: strings.TrimSpace(reason),
	}); err != nil {
		providers.DebugLogf("notify plugin turn discarded for queue %q: %v", reference.QueueID, err)
	}
}

func (s *Server) submitPluginTurn(ctx context.Context, pluginID string, params pluginhost.TurnSubmitParams) (pluginhost.TurnSubmitResult, error) {
	if s == nil || s.closed.Load() {
		return pluginhost.TurnSubmitResult{}, errServerClosed
	}
	pluginID = strings.TrimSpace(pluginID)
	params.RequestID = strings.TrimSpace(params.RequestID)
	params.ThreadID = strings.TrimSpace(params.ThreadID)
	params.Prompt = strings.TrimSpace(params.Prompt)
	if pluginID == "" {
		return pluginhost.TurnSubmitResult{}, errors.New("plugin owner is required")
	}
	if params.RequestID == "" {
		return pluginhost.TurnSubmitResult{}, errors.New("request_id is required")
	}
	if len([]byte(params.RequestID)) > pluginhost.MaxTurnSubmitRequestIDBytes {
		return pluginhost.TurnSubmitResult{}, fmt.Errorf("request_id exceeds %d bytes", pluginhost.MaxTurnSubmitRequestIDBytes)
	}
	if params.Prompt == "" {
		return pluginhost.TurnSubmitResult{}, errors.New("prompt is required")
	}
	requestContext, err := pluginTurnRequestContext(params.ContextBlocks)
	if err != nil {
		return pluginhost.TurnSubmitResult{}, err
	}

	var th *threadState
	if params.ThreadID == "" {
		th, err = s.createPluginTurnThread(pluginID)
	} else {
		th, err = s.ensureThreadLoaded(params.ThreadID)
	}
	if err != nil {
		return pluginhost.TurnSubmitResult{}, err
	}
	msg, err := userMessageFromPrompt(params.Prompt, nil, nil)
	if err != nil {
		return pluginhost.TurnSubmitResult{}, err
	}
	snapshot := turnRuntimeSnapshot{}.withPermissions(normalizeTurnPermissions(s.rt.Permissions))
	snapshot.Ultra = s.rt.UltraMode()
	snapshot.RequestContext = requestContext
	snapshot.PluginTurn = &pluginTurnReference{PluginID: pluginID, RequestID: params.RequestID}

	started, ok, err := s.startPluginSubmittedTurn(ctx, th, msg, snapshot)
	if err != nil {
		return pluginhost.TurnSubmitResult{}, err
	}
	if ok {
		return pluginhost.TurnSubmitResult{State: pluginhost.TurnLifecycleRunning, ThreadID: th.ID, TurnID: started.turnID}, nil
	}

	queueID := session.NewID()
	msg.ClientID = queueID
	snapshot.PluginTurn.QueueID = queueID
	entry := queuedTurn{id: queueID, msg: msg, snapshot: snapshot}
	s.enqueueQueuedUserTurn(th.ID, entry)
	queued := queuedTurnSummary(th.ID, entry)
	_ = s.writeNotification(NotificationTurnQueued, TurnQueuedNotification{Queued: queued})
	s.kickQueuedTurnDrain(th.ID)
	return pluginhost.TurnSubmitResult{State: pluginhost.TurnLifecycleQueued, ThreadID: th.ID, QueueID: queueID}, nil
}

func (s *Server) createPluginTurnThread(pluginID string) (*threadState, error) {
	if s.rt == nil || s.rt.StreamRunner == nil {
		return nil, errors.New("runtime session is required")
	}
	id := session.NewID()
	threadCWD := s.rt.RootDir
	if _, err := session.CreateWithMetadata(s.rt.SessionDir, id, threadCWD); err != nil {
		return nil, err
	}
	if _, err := session.SetRuntimeSelection(s.rt.SessionDir, id, s.currentSessionRuntimeSelection()); err != nil {
		return nil, err
	}
	source := "plugin:" + pluginID
	if _, err := session.SetSource(s.rt.SessionDir, id, source); err != nil {
		return nil, err
	}
	workspaceID := strings.TrimSpace(s.rt.WorkspaceID)
	if workspaceID != "" {
		if _, err := session.SetWorkspaceID(s.rt.SessionDir, id, workspaceID); err != nil {
			return nil, err
		}
	}
	history := make([]providers.ChatMessage, 0, 1)
	if prompt := strings.TrimSpace(s.rt.StreamRunner.SystemPrompt); prompt != "" {
		history = append(history, providers.ChatMessage{Role: "system", Content: prompt})
	}
	th := newThreadState(id, history, s.rt.ProviderName, s.rt.Model, threadCWD, true, time.Now().UTC())
	applyThreadRuntimeSelection(th, s.currentSessionRuntimeSelection())
	th.Source = source
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
	if err := s.notifyThreadStarted(thread); err != nil {
		return nil, err
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

func pluginTurnRequestContext(input []pluginhost.AgentContinuationBlock) ([]agent.ContextSegment, error) {
	if len(input) > pluginhost.MaxTurnSubmitContextBlocks {
		return nil, fmt.Errorf("context_blocks exceeds %d entries", pluginhost.MaxTurnSubmitContextBlocks)
	}
	blocks := make([]wuucontext.Block, 0, len(input))
	total := 0
	for _, block := range input {
		content := strings.TrimSpace(block.Content)
		if content == "" {
			return nil, errors.New("context block content is required")
		}
		size := len([]byte(content))
		if size > pluginhost.MaxTurnSubmitContextBlockBytes {
			return nil, fmt.Errorf("context block exceeds %d bytes", pluginhost.MaxTurnSubmitContextBlockBytes)
		}
		total += size
		if total > pluginhost.MaxTurnSubmitContextTotalBytes {
			return nil, fmt.Errorf("context_blocks exceeds %d total bytes", pluginhost.MaxTurnSubmitContextTotalBytes)
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
