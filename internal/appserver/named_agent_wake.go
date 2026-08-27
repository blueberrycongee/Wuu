package appserver

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/blueberrycongee/wuu/internal/agentengine"
	"github.com/blueberrycongee/wuu/internal/channels"
	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/runtime"
	"github.com/blueberrycongee/wuu/internal/session"
)

const (
	namedAgentSessionSource = "named-agent:"
	namedAgentWakePrompt    = "你有新消息，用 chat_check 查收"
)

func (s *Server) Deliver(agentID string) {
	if s == nil || s.closed.Load() || strings.TrimSpace(agentID) == "" {
		return
	}
	s.startBackground(func() {
		if err := s.deliverNamedAgentWake(context.Background(), agentID); err != nil {
			providers.DebugLogf("deliver named agent wake %q: %v", agentID, err)
		}
	})
}

func (s *Server) Interrupt(agentID string) {
	s.interruptAgentSessions(agentID, "", true)
}

func (s *Server) InterruptSession(agentID, sessionRef string) {
	s.interruptAgentSessions(agentID, sessionRef, false)
}

func (s *Server) InterruptRunSession(sessionRef string) {
	if s == nil || s.closed.Load() {
		return
	}
	sessionRef = strings.TrimSpace(sessionRef)
	if sessionRef == "" {
		return
	}
	if th := s.thread(sessionRef); th != nil {
		th.mu.Lock()
		namedAgentID := strings.TrimSpace(th.NamedAgentID)
		th.mu.Unlock()
		if namedAgentID != "" {
			providers.DebugLogf("refuse to interrupt named session %q through hidden work run", sessionRef)
			return
		}
		if _, err := s.interruptThreadExecution(sessionRef, "", ""); err != nil {
			providers.DebugLogf("interrupt hidden work session %q: %v", sessionRef, err)
		}
	} else if s.rt != nil {
		stored, found, err := session.Find(s.rt.SessionDir, sessionRef)
		if err != nil {
			providers.DebugLogf("inspect hidden work session %q: %v", sessionRef, err)
			return
		}
		if found && strings.HasPrefix(stored.Source, namedAgentSessionSource) {
			providers.DebugLogf("refuse to reset named session %q through hidden work run", sessionRef)
			return
		}
		_, _ = session.RequestThreadExecutionReset(s.rt.SessionDir, sessionRef)
	}
}

func (s *Server) interruptAgentSessions(agentID, sessionRef string, all bool) {
	if s == nil || s.closed.Load() || strings.TrimSpace(agentID) == "" {
		return
	}
	sessionRef = strings.TrimSpace(sessionRef)
	if !all && sessionRef == "" {
		return
	}
	if s.channelService == nil {
		return
	}
	agent, err := s.channelService.GetAgentRuntime(context.Background(), agentID)
	if err != nil {
		providers.DebugLogf("interrupt agent runtime %q: %v", agentID, err)
		return
	}
	refs, bindings, listErr := s.namedAgentSessionRefs(context.Background(), agent)
	if listErr != nil {
		providers.DebugLogf("list agent runtime sessions %q: %v", agentID, listErr)
		return
	}
	if !all {
		owned := false
		for _, ref := range refs {
			if ref == sessionRef {
				owned = true
				break
			}
		}
		if !owned {
			providers.DebugLogf("refuse to interrupt session %q through agent %q: session is not owned by agent", sessionRef, agentID)
			return
		}
		refs = []string{sessionRef}
	}
	client, _ := s.channelService.BindAgent(context.Background(), agentID)
	for _, ref := range refs {
		if th := s.thread(ref); th != nil {
			if _, err := s.interruptThreadExecution(ref, "", ""); err != nil {
				providers.DebugLogf("interrupt agent runtime session %q: %v", ref, err)
			}
		} else if s.rt != nil {
			_, _ = session.RequestThreadExecutionReset(s.rt.SessionDir, ref)
		}
		if binding, ok := bindings[ref]; ok && client != nil && binding.State == channels.CollaborationSessionRunning {
			_, _ = client.UpdateCollaborationSessionState(context.Background(), channels.CollaborationSessionStateParams{
				SessionRef: ref, State: channels.CollaborationSessionInterrupted,
			})
		}
	}
}

func (s *Server) deliverNamedAgentWake(ctx context.Context, agentID string) error {
	s.namedAgentMu.Lock()
	defer s.namedAgentMu.Unlock()
	if s.channelService == nil {
		return errors.New("channels service is unavailable")
	}
	if s.rt == nil {
		return errors.New("runtime session is unavailable")
	}
	agent, err := s.channelService.GetAgentRuntime(ctx, agentID)
	if err != nil {
		return err
	}
	if !agent.IsRoomRuntime() {
		return s.dispatchNamedAgentWakeLocked(ctx, agent, false)
	}
	threadID := agentRuntimeSessionID(agent)
	th := s.thread(threadID)
	if th == nil && !agent.Autostart {
		_, found, err := session.Find(s.rt.SessionDir, threadID)
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
	}
	th, err = s.ensureAgentRuntimeThreadLocked(agent)
	if err != nil {
		return err
	}
	if threadIsRunning(th) {
		if err := s.channelService.MarkWakePending(ctx, agent.ID); err != nil {
			return err
		}
		return s.holdNamedAgentWake(th.ID, agent.ID)
	}
	_, _, _, _ = s.removeHeldUserTurn(th.ID, namedAgentWakeID(agent.ID))
	return s.startAgentRuntimeWakeLocked(agent, th)
}

func (s *Server) ensureNamedAgentThreadLocked(agent channels.NamedAgent) (*threadState, error) {
	return s.ensureAgentRuntimeThreadLocked(agentRuntimeFromNamed(agent))
}

func (s *Server) ensureAgentRuntimeThreadLocked(agent channels.AgentRuntime) (*threadState, error) {
	return s.ensureAgentRuntimeThreadWithSessionLocked(agent, agentRuntimeSessionID(agent), "")
}

func (s *Server) ensureAgentRuntimeSessionThreadLocked(agent channels.AgentRuntime, threadID string) (*threadState, error) {
	return s.ensureAgentRuntimeThreadWithSessionLocked(agent, threadID, threadID)
}

func (s *Server) ensureAgentRuntimeThreadWithSessionLocked(agent channels.AgentRuntime, threadID, collaborationSessionRef string) (*threadState, error) {
	if s.rt == nil {
		return nil, errors.New("runtime session is unavailable")
	}
	threadID = strings.TrimSpace(threadID)
	collaborationSessionRef = strings.TrimSpace(collaborationSessionRef)
	if threadID == "" {
		return nil, errors.New("agent runtime session is required")
	}
	if th := s.thread(threadID); th != nil {
		th.mu.Lock()
		defer th.mu.Unlock()
		if th.NamedAgentID != "" && th.NamedAgentID != agent.ID {
			return nil, fmt.Errorf("session %q is owned by named agent %q", threadID, th.NamedAgentID)
		}
		if th.Source != "" && th.Source != namedAgentSessionSource+agent.ID {
			return nil, fmt.Errorf("session %q is not owned by named agent %q", threadID, agent.ID)
		}
		sessionBindingChanged := th.CollaborationSessionRef != collaborationSessionRef
		needsNamedAgentRuntime := th.execRuntime == nil ||
			th.NamedAgentID != agent.ID ||
			sessionBindingChanged ||
			th.execRuntime.StreamRunner == nil ||
			th.execRuntime.Toolkit == nil ||
			!th.execRuntime.Toolkit.SupportsTool("chat_check")
		th.NamedAgentID = agent.ID
		th.CollaborationSessionRef = collaborationSessionRef
		th.Source = namedAgentSessionSource + agent.ID
		th.CWD = filepath.Dir(agent.MemoryDir)
		th.EngineID = string(agentengine.NormalizeEngineID(agent.EngineOverride))
		if needsNamedAgentRuntime {
			selection := s.collaborationRuntimeSelection(s.currentSessionRuntimeSelection(), agent)
			selection.Provider, selection.Model, selection.Effort = agentRuntimeModelSelection(
				selection.Provider, selection.Model, selection.Effort, agent,
			)
			threadRuntime, err := s.newAgentExecutionRuntimeForSession(threadID, collaborationSessionRef, agent, runtime.ThreadModelSelection{
				Provider: selection.Provider, Model: selection.Model, Variant: selection.Variant,
				Effort: selection.Effort, PermissionMode: selection.PermissionMode,
			})
			if err != nil {
				return nil, err
			}
			if _, err := session.SetRuntimeSelection(s.rt.SessionDir, threadID, selection); err != nil {
				releaseDetachedThreadRuntime(detachedThreadRuntime{runtime: threadRuntime})
				return nil, err
			}
			staleRuntime := th.execRuntime
			th.execRuntime = threadRuntime
			applyThreadRuntimeSelection(th, selection)
			if len(th.History) > 0 && strings.EqualFold(strings.TrimSpace(th.History[0].Role), "system") {
				th.History[0].Content = threadRuntime.StreamRunner.SystemPrompt
			}
			if staleRuntime != nil {
				releaseDetachedThreadRuntime(detachedThreadRuntime{runtime: staleRuntime})
			}
		}
		return th, nil
	}
	agentHome := filepath.Dir(agent.MemoryDir)
	selection := s.collaborationRuntimeSelection(s.currentSessionRuntimeSelection(), agent)
	selection.Provider, selection.Model, selection.Effort = agentRuntimeModelSelection(
		selection.Provider, selection.Model, selection.Effort, agent,
	)
	threadRuntime, err := s.newAgentExecutionRuntimeForSession(threadID, collaborationSessionRef, agent, runtime.ThreadModelSelection{
		Provider: selection.Provider, Model: selection.Model, Variant: selection.Variant,
		Effort: selection.Effort, PermissionMode: selection.PermissionMode,
	})
	if err != nil {
		return nil, err
	}

	metadata, found, err := session.Find(s.rt.SessionDir, threadID)
	if err != nil {
		releaseDetachedThreadRuntime(detachedThreadRuntime{runtime: threadRuntime})
		return nil, err
	}
	var th *threadState
	if found {
		if metadata.Source != namedAgentSessionSource+agent.ID {
			releaseDetachedThreadRuntime(detachedThreadRuntime{runtime: threadRuntime})
			return nil, fmt.Errorf("session %q is not owned by named agent %q", threadID, agent.ID)
		}
		th, err = s.loadPersistedThreadState(threadID, time.Now().UTC())
		if err != nil {
			releaseDetachedThreadRuntime(detachedThreadRuntime{runtime: threadRuntime})
			return nil, err
		}
		if _, err := session.SetEngine(s.rt.SessionDir, threadID, string(agentengine.NormalizeEngineID(agent.EngineOverride))); err != nil {
			releaseDetachedThreadRuntime(detachedThreadRuntime{runtime: threadRuntime})
			return nil, err
		}
		if _, err := session.SetRuntimeSelection(s.rt.SessionDir, threadID, selection); err != nil {
			releaseDetachedThreadRuntime(detachedThreadRuntime{runtime: threadRuntime})
			return nil, err
		}
		applyThreadRuntimeSelection(th, selection)
		th.EngineID = string(agentengine.NormalizeEngineID(agent.EngineOverride))
	} else {
		if _, err := session.CreateWithMetadata(s.rt.SessionDir, threadID, agentHome); err != nil {
			releaseDetachedThreadRuntime(detachedThreadRuntime{runtime: threadRuntime})
			return nil, err
		}
		if _, err := session.SetSource(s.rt.SessionDir, threadID, namedAgentSessionSource+agent.ID); err != nil {
			releaseDetachedThreadRuntime(detachedThreadRuntime{runtime: threadRuntime})
			return nil, err
		}
		if _, err := session.SetEngine(s.rt.SessionDir, threadID, string(agentengine.NormalizeEngineID(agent.EngineOverride))); err != nil {
			releaseDetachedThreadRuntime(detachedThreadRuntime{runtime: threadRuntime})
			return nil, err
		}
		if _, err := session.UpdateTitle(s.rt.SessionDir, threadID, agent.Name); err != nil {
			releaseDetachedThreadRuntime(detachedThreadRuntime{runtime: threadRuntime})
			return nil, err
		}
		if _, err := session.SetRuntimeSelection(s.rt.SessionDir, threadID, selection); err != nil {
			releaseDetachedThreadRuntime(detachedThreadRuntime{runtime: threadRuntime})
			return nil, err
		}
		history := []providers.ChatMessage{{Role: "system", Content: threadRuntime.StreamRunner.SystemPrompt}}
		th = newThreadState(threadID, history, selection.Provider, selection.Model, agentHome, true, time.Now().UTC())
		th.Source = namedAgentSessionSource + agent.ID
		th.Title = agent.Name
		th.EngineID = string(agentengine.NormalizeEngineID(agent.EngineOverride))
		applyThreadRuntimeSelection(th, selection)
	}
	th.NamedAgentID = agent.ID
	th.CollaborationSessionRef = collaborationSessionRef
	if len(th.History) > 0 && strings.EqualFold(strings.TrimSpace(th.History[0].Role), "system") {
		th.History[0].Content = threadRuntime.StreamRunner.SystemPrompt
	}
	th.execRuntime = threadRuntime
	s.mu.Lock()
	existing := s.threads[threadID]
	if existing == nil {
		s.threads[threadID] = th
	}
	s.mu.Unlock()
	if existing != nil {
		releaseDetachedThreadRuntime(detachedThreadRuntime{runtime: threadRuntime})
		return existing, nil
	}
	return th, nil
}

func namedAgentModelSelection(provider, model, effort string, agent channels.NamedAgent) (string, string, string) {
	return agentRuntimeModelSelection(provider, model, effort, agentRuntimeFromNamed(agent))
}

func agentRuntimeModelSelection(provider, model, effort string, agent channels.AgentRuntime) (string, string, string) {
	if override := strings.TrimSpace(agent.ModelOverride); override != "" {
		provider = firstNonEmpty(strings.TrimSpace(agent.ProviderOverride), strings.TrimSpace(agent.EngineOverride))
		model = override
	}
	if override := strings.TrimSpace(agent.EffortOverride); override != "" {
		effort = override
	}
	return provider, model, effort
}

func (s *Server) collaborationRuntimeSelection(selection session.RuntimeSelection, agent channels.AgentRuntime) session.RuntimeSelection {
	if s == nil || s.rt == nil || !agent.IsRoomRuntime() {
		return selection
	}
	role := s.rt.ModelRoles.Coordination
	selection.Provider = role.Provider
	selection.Model = role.Model
	selection.Variant = role.Variant
	selection.Effort = role.LegacyEffort
	return selection
}

func (s *Server) newNamedAgentRuntime(threadID string, agent channels.NamedAgent, selection runtime.ThreadModelSelection) (*runtime.ThreadRuntime, error) {
	return s.newAgentExecutionRuntime(threadID, agentRuntimeFromNamed(agent), selection)
}

func (s *Server) newAgentExecutionRuntime(threadID string, agent channels.AgentRuntime, selection runtime.ThreadModelSelection) (*runtime.ThreadRuntime, error) {
	return s.newAgentExecutionRuntimeForSession(threadID, "", agent, selection)
}

func (s *Server) newAgentExecutionRuntimeForSession(threadID, collaborationSessionRef string, agent channels.AgentRuntime, selection runtime.ThreadModelSelection) (*runtime.ThreadRuntime, error) {
	agentHome := filepath.Dir(agent.MemoryDir)
	threadRuntime, err := s.rt.NewNamedAgentThreadRuntime(
		threadID, agentHome, agent.MemoryDir, agentRuntimeOrientation(agent), selection,
	)
	if err != nil {
		return nil, err
	}
	var chatAgent *channels.AgentClient
	if agent.IsRoomRuntime() {
		chatAgent, err = s.channelService.BindRuntime(context.Background(), agent.ID)
	} else if collaborationSessionRef = strings.TrimSpace(collaborationSessionRef); collaborationSessionRef != "" {
		chatAgent, err = s.channelService.BindAgentSession(context.Background(), agent.ID, collaborationSessionRef)
	} else {
		chatAgent, err = s.channelService.BindAgent(context.Background(), agent.ID)
	}
	if err != nil {
		releaseDetachedThreadRuntime(detachedThreadRuntime{runtime: threadRuntime})
		return nil, err
	}
	if threadRuntime.Toolkit == nil {
		releaseDetachedThreadRuntime(detachedThreadRuntime{runtime: threadRuntime})
		return nil, errors.New("named agent toolkit is unavailable")
	}
	threadRuntime.Toolkit.SetChatAgent(chatAgent)
	if err := s.rt.ConfigureNamedAgentThreadRuntime(
		threadRuntime, agentHome, agent.MemoryDir,
		agentRuntimeOrientation(agent),
	); err != nil {
		releaseDetachedThreadRuntime(detachedThreadRuntime{runtime: threadRuntime})
		return nil, err
	}
	s.attachNamedAgentRoomContext(threadRuntime, agent.ID)
	return threadRuntime, nil
}

func (s *Server) startNamedAgentWakeLocked(agent channels.NamedAgent, th *threadState) error {
	return s.startAgentRuntimeWakeLocked(agentRuntimeFromNamed(agent), th)
}

func (s *Server) startAgentRuntimeWakeLocked(agent channels.AgentRuntime, th *threadState) error {
	inbox, err := s.channelService.ListInbox(context.Background(), agent.ID, true)
	if err != nil {
		return err
	}
	roomIDs := distinctInboxRoomIDs(inbox)
	collaborationRoomIDs, err := s.channelService.PendingCollaborationRoomIDs(context.Background(), agent.ID)
	if err != nil {
		return err
	}
	roomIDs = appendDistinctStrings(roomIDs, collaborationRoomIDs...)
	return s.startAgentRuntimeSessionWakeLocked(agent, th, roomIDs, "", "")
}

func (s *Server) startAgentRuntimeSessionWakeLocked(agent channels.AgentRuntime, th *threadState, roomIDs []string, workID, runID string) error {
	permissions, err := s.resolveThreadTurnPermissions(th, nil)
	if err != nil {
		return err
	}
	clientID := namedAgentWakeTurnID(agent.ID, th.ID)
	if strings.TrimSpace(runID) != "" {
		clientID = namedAgentWorkWakeTurnID(runID)
	}
	message := providers.ChatMessage{
		Role: "user", Content: namedAgentWakePrompt,
		ClientID: clientID, Hidden: true, Phase: "channel_wake",
	}
	var threadRuntime *runtime.ThreadRuntime
	started, ok, err := s.startThreadUserTurnWithAdmission(
		context.Background(), th, message, turnRuntimeSnapshot{}.withPermissions(permissions), false,
		turnReadOnlyFail, turnAdmissionHooks{afterLease: func(admitted *threadState, _ *providers.ChatMessage) error {
			threadRuntime, err = s.ensureThreadRuntimeAfterAdmission(admitted)
			return err
		}},
	)
	if err != nil {
		return err
	}
	if !ok {
		if err := s.channelService.MarkWakePending(context.Background(), agent.ID); err != nil {
			return err
		}
		return s.holdNamedAgentWake(th.ID, agent.ID)
	}
	if strings.TrimSpace(runID) != "" {
		client, bindErr := s.channelService.BindAgentSession(context.Background(), agent.ID, th.ID)
		if bindErr == nil {
			_, bindErr = client.AttachWorkRunTurn(context.Background(), channels.WorkRunTurnParams{
				WorkID: workID, RunID: runID, TurnID: started.turnID,
			})
		}
		if bindErr != nil {
			return errors.Join(bindErr, s.abortStartedThreadTurnDurably(th, started, bindErr))
		}
	}
	setNamedAgentActivityRoomIDs(th, roomIDs)
	launch, accepted := s.reserveBackground(func() {
		s.runTurn(started.ctx, th, threadRuntime, started.turnID, started.runtime, started.history)
		s.completeNamedAgentSessionTurn(agent.ID, th.ID, workID, runID, started.turnID)
	})
	if !accepted {
		return errors.Join(errServerClosed, s.abortStartedThreadTurnDurably(th, started, errServerClosed))
	}
	defer launch.Cancel()
	launch.Commit()
	return nil
}

func distinctInboxRoomIDs(items []channels.InboxItem) []string {
	seen := make(map[string]struct{}, len(items))
	roomIDs := make([]string, 0, len(items))
	for _, item := range items {
		roomID := strings.TrimSpace(item.RoomID)
		if roomID == "" {
			continue
		}
		if _, ok := seen[roomID]; ok {
			continue
		}
		seen[roomID] = struct{}{}
		roomIDs = append(roomIDs, roomID)
	}
	return roomIDs
}

func appendDistinctStrings(values []string, additions ...string) []string {
	seen := make(map[string]struct{}, len(values)+len(additions))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	for _, value := range additions {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

func setNamedAgentActivityRoomIDs(th *threadState, roomIDs []string) {
	if th == nil {
		return
	}
	th.mu.Lock()
	th.namedAgentRoomIDs = append(th.namedAgentRoomIDs[:0], roomIDs...)
	th.mu.Unlock()
}

func namedAgentActivityRoomIDs(th *threadState) []string {
	if th == nil {
		return nil
	}
	th.mu.Lock()
	defer th.mu.Unlock()
	return append([]string(nil), th.namedAgentRoomIDs...)
}

func (s *Server) completeNamedAgentSessionTurn(agentID, sessionRef, workID, runID, turnID string) {
	s.namedAgentMu.Lock()
	defer s.namedAgentMu.Unlock()
	if s.closed.Load() || s.channelService == nil {
		return
	}
	if workID != "" && runID != "" {
		s.finishNamedAgentWorkRun(context.Background(), agentID, sessionRef, workID, runID, turnID)
	} else if th := s.thread(sessionRef); th != nil {
		th.mu.Lock()
		boundSessionRef := th.CollaborationSessionRef
		nextState := channels.CollaborationSessionIdle
		for _, turn := range th.Turns {
			if turn.ID == turnID && turn.Status == TurnStatusInterrupted {
				nextState = channels.CollaborationSessionInterrupted
				break
			}
		}
		th.mu.Unlock()
		if boundSessionRef != "" {
			client, bindErr := s.channelService.BindAgentSession(context.Background(), agentID, boundSessionRef)
			if bindErr == nil {
				_, bindErr = client.UpdateCollaborationSessionState(context.Background(), channels.CollaborationSessionStateParams{
					SessionRef: boundSessionRef, State: nextState,
				})
			}
			if bindErr != nil {
				providers.DebugLogf("settle named agent session %q: %v", boundSessionRef, bindErr)
			}
		}
	}
	_, _, _, _ = s.removeHeldUserTurn(sessionRef, namedAgentWakeID(agentID, sessionRef))
	followup, err := s.channelService.FinishWakeAttempt(context.Background(), agentID)
	if err != nil {
		providers.DebugLogf("finish named agent wake %q: %v", agentID, err)
	}
	if agent, getErr := s.channelService.GetAgentRuntime(context.Background(), agentID); getErr == nil {
		var dispatchErr error
		if agent.IsRoomRuntime() && followup {
			state, stateErr := s.channelService.WakeState(context.Background(), agentID)
			if stateErr == nil && state.Outstanding {
				th, ensureErr := s.ensureAgentRuntimeThreadLocked(agent)
				if ensureErr == nil {
					ensureErr = s.startAgentRuntimeWakeLocked(agent, th)
				}
				dispatchErr = ensureErr
			}
		} else if !agent.IsRoomRuntime() && followup {
			dispatchErr = s.dispatchNamedAgentWakeLocked(context.Background(), agent, false)
		}
		if dispatchErr != nil {
			providers.DebugLogf("inject pending named agent wake %q: %v", agentID, dispatchErr)
		}
	}
}

func (s *Server) holdNamedAgentWake(threadID, agentID string) error {
	id := namedAgentWakeID(agentID, threadID)
	if _, found, err := s.findHeldUserTurn(threadID, id); err != nil {
		return err
	} else if found {
		return nil
	}
	_, err := s.appendHeldUserTurns(threadID, []queuedTurn{{
		id: id,
		msg: providers.ChatMessage{
			Role: "user", Content: namedAgentWakePrompt, ClientID: id, Hidden: true, Phase: "channel_wake",
		},
		origin: session.HeldUserWorkOriginQueue,
	}})
	return err
}

func (s *Server) restoreNamedAgentWakes() {
	if s == nil || s.channelService == nil || s.closed.Load() {
		return
	}
	if err := s.reconcileChannelWorkRuns(context.Background()); err != nil {
		providers.DebugLogf("reconcile channel work runs: %v", err)
	}
	agents, err := s.channelService.ListAgentRuntimes(context.Background())
	if err != nil {
		providers.DebugLogf("restore named agent wakes: %v", err)
		return
	}
	for _, agent := range agents {
		if !agent.IsRoomRuntime() {
			s.namedAgentMu.Lock()
			resumeErr := s.resumeNamedAgentBoundSessionsLocked(context.Background(), agent)
			s.namedAgentMu.Unlock()
			if resumeErr != nil {
				providers.DebugLogf("restore named agent sessions %q: %v", agent.ID, resumeErr)
			}
		}
		state, err := s.channelService.WakeState(context.Background(), agent.ID)
		if err == nil && state.Outstanding {
			if err := s.deliverNamedAgentWake(context.Background(), agent.ID); err != nil {
				providers.DebugLogf("restore named agent wake %q: %v", agent.ID, err)
			}
		}
	}
}

func namedAgentSessionID(agent channels.NamedAgent) string {
	return principalSessionID(agent.ID, agent.CreatedAt)
}

func agentRuntimeSessionID(agent channels.AgentRuntime) string {
	return principalSessionID(agent.ID, agent.CreatedAt)
}

func principalSessionID(id string, createdAt time.Time) string {
	sum := sha256.Sum256([]byte(id))
	return createdAt.UTC().Format("20060102-150405") + fmt.Sprintf("-%x", sum[:8])
}

func namedAgentWakeID(agentID string, sessionRefs ...string) string {
	id := "channel-wake:" + strings.TrimSpace(agentID)
	if len(sessionRefs) > 0 && strings.TrimSpace(sessionRefs[0]) != "" {
		id += ":" + strings.TrimSpace(sessionRefs[0])
	}
	return id
}

func namedAgentWakeTurnID(agentID string, sessionRefs ...string) string {
	return namedAgentWakeID(agentID, sessionRefs...) + ":" + session.NewID()
}

func namedAgentWorkWakeTurnID(runID string) string {
	return "channel-work-run:" + strings.TrimSpace(runID)
}

func namedAgentOrientation(agent channels.NamedAgent) string {
	return agentRuntimeOrientation(agentRuntimeFromNamed(agent))
}

func agentRuntimeFromNamed(agent channels.NamedAgent) channels.AgentRuntime {
	return channels.AgentRuntime{
		ID: agent.ID, Kind: channels.PrincipalNamedAgent, Name: agent.Name, Role: agent.Role, MemoryDir: agent.MemoryDir,
		EngineOverride: agent.EngineOverride, ProviderOverride: agent.ProviderOverride,
		ModelOverride: agent.ModelOverride, EffortOverride: agent.EffortOverride,
		Autostart: agent.Autostart, CreatedAt: agent.CreatedAt,
	}
}

func agentRuntimeOrientation(agent channels.AgentRuntime) string {
	agentHome := filepath.Dir(agent.MemoryDir)
	if agent.IsRoomRuntime() {
		return fmt.Sprintf(`# Room agent

You are the hidden runtime for your mapped Wuu room. Its current name and visible Named Agent membership are provided in request context. Your agent home and default working directory is %s, and your durable memory directory is %s. You are not a room participant or a user-facing persona. Never use chat_send: all user-visible prose must come from a visible Named Agent.

You are the room's single collaboration entrypoint. Ordinary room messages and member reports wake you; they do not wake every member. On every wake, call chat_check. Use chat_read when you need the full room context or attachments.

Delegate from evidence, not names alone. The request context gives every current member's durable role and the current membership revision. Use chat_roster list when you need model configuration, the current members' sanitized memory-index hooks and session summaries, or need to consider Named Agents outside the room. Treat memory_index as private routing context: never quote it publicly or copy it into another Agent's context, and do not inspect the owning Agent's memory topic files. Prefer a current capable member; invite an existing outside Agent when its durable role fits. Use chat_roster create only for a genuinely durable missing role and always give it a clear role; this creates a persistent proposal card, not an Agent, and you must wait for the user to choose a model and approve it before assigning that role. Never claim that a proposed Agent already exists or joined. All visible Named Agents have the normal project-work tool surface; their role, model configuration, durable experience, and current evidence determine suitability.

Classify the user's turn before creating work. For casual conversation, retrieval, explanation, open-ended discussion, or idea exploration, privately invite suitable members to answer publicly and do not create a task. When the user explicitly asks what everyone thinks, give every current visible member a real opportunity to answer from its own perspective. Use parallel private control messages when viewpoints are independent or latency matters. Use serial invitations when each answer should see what has already been said; after one public answer wakes you, invite the next. In a parallel round, each member composes against the current room sequence, and the held-draft mechanism makes later overlapping answers read the delta, revise, add a distinct point, or stay silent. Never force repetitive agreement merely to make every member speak. A direct @mention normally routes to that member unless safety or missing capability requires help.

For a real task, choose one visible owner for a tightly coupled goal, or split genuinely independent deliverables across a few visible owners. Make every real assignment visible with chat_task create at the room root (no thread_id), using a concise title, a natural-language brief, the triggering source_message_id, and that Named Agent as owner. Set verification_required=true whenever the task promises a concrete deliverable. Conversation and open-ended idea exploration stay on the public conversation path and never create a task; if a discussion later becomes a request for a final proposal, create a task for that deliverable. The host creates one durable Work debt and, when target_session_ref is omitted, a dedicated durable Work session under the chosen Named Agent. Specify target_session_ref only to promote a listed idle unscoped session that belongs to the chosen owner in this room and has no Work or run; otherwise omit it. Never route Work to the Agent's fixed conversation session by default. Do not duplicate assignments through collaboration_send and do not build a speculative project tree. When the user corrects an existing task goal, call chat_task revise with the task_id and the complete revised brief instead of creating a duplicate task; the host increments goal_revision, invalidates older deliveries, and interrupts stale run handles.

An incoming room delivery with work_id belongs to that existing Work. Route a correction back to the same owner with chat_task revise, use chat_work cancel for an explicit cancellation, and forward a needs-human answer to the same owner rather than creating another Work. Cancellation prevents new runs and integration but does not claim that already-applied side effects were rolled back.

When the owner moves a deliverable task to checking and sends candidate_ready, do not publish it. First read the complete Work and the current source message so the verification input comes from room facts rather than the owner's selection. Recovery may redeliver candidate_ready, so reuse a matching active verifier run instead of starting a second one. By default use the Subagent plugin's spawn_agent tool with model @verification to start exactly one fresh-context verifier, then start a chat_work verifier run with profile independent, the child session id, and the candidate workspace revision. Give the child the user's current goal, task brief, both revisions, the complete machine-listed artifacts and checks, relevant workspace paths, and the candidate itself. Label the owner's summary as an unverified claim. Do not pass the producer's private conversation or any other candidate's opinion. Ask the child to independently inspect the result, rerun focused checks, seek counterexamples, and return a first-line PASS, BLOCK, or UNKNOWN followed by concise natural-language evidence. Do not require JSON or a criterion table. The child may inspect but must not repair or publish the candidate.

Only when the user explicitly asks a particular Named Agent to check the result should verification use that member instead of the fresh hidden run. Start the verifier run with that member's id, send the private request, and accept its peer_result. Do not add a persistent room member merely to perform default verification.

When the hidden verifier completes, or a requested Named verifier returns peer_result, first settle the referenced chat_work run with the actual terminal state, checks rerun, findings, and outcome. Then call chat_verify exactly once with the run_ref, delivery revisions, three-state decision, report, and evidence_refs. The host rejects stale revisions, persists the attempt, and privately wakes the same owner. A BLOCK returns the same task to the same owner for repair and another fresh verification round. UNKNOWN or exhausted attempts asks the user for the missing decision instead of silently retrying or lowering the bar. PASS tells the owner to publish the verified result to the room's public timeline and mark it done. The owner may never verify its own candidate. Conversation and open-ended idea exploration do not use this loop because they never created Work.

The default policy is one candidate. Use chat_work policy to raise max_candidates only when the user asked for alternatives, two approaches are genuinely mutually exclusive and costly, repeated blocks justify escaping an anchor, or a high-risk policy requires redundancy; always record the concrete fanout_reason. Prefer a visible owner or lead to choose among qualified candidate artifacts. Do not create hidden workers or unnamed selectors.

Use collaboration_send only for other private control details. Never publish a member's unverified candidate or impersonate its final response. If several verified contributions need synthesis, create one final task owned by a visible lead and let that Named Agent produce the room-facing answer. Account for work still in flight before doing so. Silence is valid when no new assignment or private feedback is needed.

Never use human-only channel RPCs to post. If chat_check returns has_more, check again. Resolve held chat drafts explicitly after reading the delta.`, agentHome, agent.MemoryDir)
	}
	role := strings.TrimSpace(agent.Role)
	if role == "" {
		role = "No durable role has been set. Follow the concrete assignment and avoid inventing a specialty."
	} else {
		role = "Your durable team role is: " + role
	}
	return fmt.Sprintf(`# Named agent

You are %s, a persistent named agent in Wuu group chat. %s Your agent home and default working directory is %s, and your durable memory directory is %s. The agent home is your private identity and state anchor; it is not the limit of your project activity scope. Use only your own memory directory for long-term memory; do not treat another agent or the user's memory as yours.

## Project activity scope

Your current registered project workspaces are supplied as request-only environment context. You may read, search, edit, and run commands in any listed project workspace, subject to the current permission mode. Use an absolute file path or set a command's cwd to the relevant project root. Do not claim that you can only access your agent home, and do not rebind the persistent session workspace merely to perform work in another listed project. Projectless conversation sessions are not project workspaces. The system temp directory may also be available for transient files, but it is not a project workspace.

Wake notifications contain no chat content. On wake, call chat_check. Direct messages and explicit task or reminder signals appear in the regular inbox; private control messages appear in collaboration. Use chat_read when you need full shared-room context. If chat_check returns has_more, check again. Never use human-only channel RPCs to post.

## Coordination in shared rooms

Ordinary shared-room messages, including @mentions, are routed first to the hidden room runtime and do not directly wake every member. Do not independently claim work from the shared transcript. Act on room tasks assigned to you and use chat_task to mark a task doing when you start. Public task-thread messages are for meaningful progress, questions, and verified results; keep acknowledgements, retries, heartbeats, and raw tool logs private.

For a substantive coding task, do the work and focused mechanical checks, but do not publish the candidate as final and do not mark the task done yet. Use chat_work add_artifact for the diff or snapshot and important check logs, then chat_work evidence for the compact checks/files/unresolved summary. Mark the task checking and read the returned goal_revision and candidate_revision. Then call collaboration_send with kind=candidate_ready, source_message_id=task_id, and artifact_refs; omit to_agent_id because the host routes this handoff to the hidden room runtime without exposing its identity. Summarize the change and checks in the natural-language body. A collaboration message with kind verification_feedback carries the independent result: on BLOCK, the task becomes revising; use the evidence, mark it doing when repair starts, and submit the same task again. On UNKNOWN, the task becomes needs_human; supply evidence or ask the human when only they can decide. On PASS, publish the result to the room's public timeline with chat_send using a fresh room basis_seq, then mark the task done. The host only accepts completion directly from checking with a pass for the current goal and candidate revisions. The hidden runtime and verifier never speak for you.

Other collaboration messages are private control traffic: answer with collaboration_send unless one explicitly authorizes a public contribution. Direct messages remain private conversations with the human and should be answered with chat_send.

When a control message asks for a public conversational contribution, read the current room sequence, answer with chat_send only if you have a useful distinct point, and rely on held-draft resolution if another member spoke first. When a control message assigns independent verification for a task owned by someone else, perform the checks without asking for the owner's private conversation, then return collaboration_send kind=peer_result with source_message_id=task_id, no to_agent_id, and a first-line PASS, BLOCK, or UNKNOWN followed by the natural-language evidence. Never use peer_result for your own task.

Use chat_task to create, list, or update lightweight room tasks. Use chat_remind when you need to wake yourself at least one minute later, optionally with room or thread context.

chat_send requires the current basis sequence for the room's public timeline. If the room moved, your text becomes a held draft instead of posting. Resolve every held draft explicitly with one of four paths:
- revise: read the delta, then chat_send replacement text with a fresh basis;
- as_is: chat_draft resolve as_is with a fresh basis when the independent point still stands (it may be held again if the scope moved);
- silent: chat_draft resolve silent when others covered it or silence is better;
- anyway: chat_draft resolve anyway only after hold_count reaches 2 and the unchanged text remains important.
The server never rewrites or automatically resends a held draft.`, agent.Name, role, agentHome, agent.MemoryDir)
}
