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

func (s *Server) deliverNamedAgentWake(ctx context.Context, agentID string) error {
	s.namedAgentMu.Lock()
	defer s.namedAgentMu.Unlock()
	if s.channelService == nil {
		return errors.New("channels service is unavailable")
	}
	if s.rt == nil {
		return errors.New("runtime session is unavailable")
	}
	agent, err := s.channelService.GetNamedAgent(ctx, agentID)
	if err != nil {
		return err
	}
	threadID := namedAgentSessionID(agent)
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
	th, err = s.ensureNamedAgentThreadLocked(agent)
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
	return s.startNamedAgentWakeLocked(agent, th)
}

func (s *Server) ensureNamedAgentThreadLocked(agent channels.NamedAgent) (*threadState, error) {
	if s.rt == nil {
		return nil, errors.New("runtime session is unavailable")
	}
	threadID := namedAgentSessionID(agent)
	if th := s.thread(threadID); th != nil {
		th.mu.Lock()
		defer th.mu.Unlock()
		if th.NamedAgentID != "" && th.NamedAgentID != agent.ID {
			return nil, fmt.Errorf("session %q is owned by named agent %q", threadID, th.NamedAgentID)
		}
		if th.Source != "" && th.Source != namedAgentSessionSource+agent.ID {
			return nil, fmt.Errorf("session %q is not owned by named agent %q", threadID, agent.ID)
		}
		needsNamedAgentRuntime := th.execRuntime == nil ||
			th.NamedAgentID != agent.ID ||
			th.execRuntime.StreamRunner == nil ||
			th.execRuntime.Toolkit == nil ||
			!th.execRuntime.Toolkit.SupportsTool("chat_check")
		th.NamedAgentID = agent.ID
		th.Source = namedAgentSessionSource + agent.ID
		th.CWD = filepath.Dir(agent.MemoryDir)
		th.EngineID = string(agentengine.NormalizeEngineID(agent.EngineOverride))
		if needsNamedAgentRuntime {
			selection := s.currentSessionRuntimeSelection()
			selection.Provider, selection.Model, selection.Effort = namedAgentModelSelection(
				selection.Provider, selection.Model, selection.Effort, agent,
			)
			threadRuntime, err := s.newNamedAgentRuntime(threadID, agent, runtime.ThreadModelSelection{
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
	selection := s.currentSessionRuntimeSelection()
	selection.Provider, selection.Model, selection.Effort = namedAgentModelSelection(
		selection.Provider, selection.Model, selection.Effort, agent,
	)
	threadRuntime, err := s.newNamedAgentRuntime(threadID, agent, runtime.ThreadModelSelection{
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
	if override := strings.TrimSpace(agent.ModelOverride); override != "" {
		provider = firstNonEmpty(strings.TrimSpace(agent.ProviderOverride), strings.TrimSpace(agent.EngineOverride))
		model = override
	}
	if override := strings.TrimSpace(agent.EffortOverride); override != "" {
		effort = override
	}
	return provider, model, effort
}

func (s *Server) newNamedAgentRuntime(threadID string, agent channels.NamedAgent, selection runtime.ThreadModelSelection) (*runtime.ThreadRuntime, error) {
	agentHome := filepath.Dir(agent.MemoryDir)
	threadRuntime, err := s.rt.NewNamedAgentThreadRuntime(
		threadID, agentHome, agent.MemoryDir, namedAgentOrientation(agent), selection,
	)
	if err != nil {
		return nil, err
	}
	chatAgent, err := s.channelService.BindAgent(context.Background(), agent.ID)
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
		namedAgentOrientation(agent),
	); err != nil {
		releaseDetachedThreadRuntime(detachedThreadRuntime{runtime: threadRuntime})
		return nil, err
	}
	s.attachNamedAgentRoomContext(threadRuntime, agent.ID)
	return threadRuntime, nil
}

func (s *Server) startNamedAgentWakeLocked(agent channels.NamedAgent, th *threadState) error {
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
	permissions, err := s.resolveThreadTurnPermissions(th, nil)
	if err != nil {
		return err
	}
	message := providers.ChatMessage{
		Role: "user", Content: namedAgentWakePrompt,
		ClientID: namedAgentWakeTurnID(agent.ID), Hidden: true, Phase: "channel_wake",
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
	setNamedAgentActivityRoomIDs(th, roomIDs)
	launch, accepted := s.reserveBackground(func() {
		s.runTurn(started.ctx, th, threadRuntime, started.turnID, started.runtime, started.history)
		s.completeNamedAgentTurn(agent.ID)
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

func (s *Server) completeNamedAgentTurn(agentID string) {
	s.namedAgentMu.Lock()
	defer s.namedAgentMu.Unlock()
	if s.closed.Load() || s.channelService == nil {
		return
	}
	followup, err := s.channelService.FinishWakeAttempt(context.Background(), agentID)
	if err != nil {
		providers.DebugLogf("finish named agent wake %q: %v", agentID, err)
		return
	}
	threadID := ""
	if agent, getErr := s.channelService.GetNamedAgent(context.Background(), agentID); getErr == nil {
		threadID = namedAgentSessionID(agent)
		_, _, _, _ = s.removeHeldUserTurn(threadID, namedAgentWakeID(agentID))
		if followup {
			th, ensureErr := s.ensureNamedAgentThreadLocked(agent)
			if ensureErr == nil {
				ensureErr = s.startNamedAgentWakeLocked(agent, th)
			}
			if ensureErr != nil {
				providers.DebugLogf("inject pending named agent wake %q: %v", agentID, ensureErr)
			}
		}
		return
	}
	if threadID != "" {
		_, _, _, _ = s.removeHeldUserTurn(threadID, namedAgentWakeID(agentID))
	}
}

func (s *Server) holdNamedAgentWake(threadID, agentID string) error {
	id := namedAgentWakeID(agentID)
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
	agents, err := s.channelService.ListAgentRuntimes(context.Background())
	if err != nil {
		providers.DebugLogf("restore named agent wakes: %v", err)
		return
	}
	for _, agent := range agents {
		state, err := s.channelService.WakeState(context.Background(), agent.ID)
		if err == nil && state.Outstanding {
			if err := s.deliverNamedAgentWake(context.Background(), agent.ID); err != nil {
				providers.DebugLogf("restore named agent wake %q: %v", agent.ID, err)
			}
		}
	}
}

func namedAgentSessionID(agent channels.NamedAgent) string {
	sum := sha256.Sum256([]byte(agent.ID))
	return agent.CreatedAt.UTC().Format("20060102-150405") + fmt.Sprintf("-%x", sum[:8])
}

func namedAgentWakeID(agentID string) string {
	return "channel-wake:" + strings.TrimSpace(agentID)
}

func namedAgentWakeTurnID(agentID string) string {
	return namedAgentWakeID(agentID) + ":" + session.NewID()
}

func namedAgentOrientation(agent channels.NamedAgent) string {
	agentHome := filepath.Dir(agent.MemoryDir)
	if agent.Kind == "room" {
		return fmt.Sprintf(`# Room agent

You are the hidden runtime for your mapped Wuu room. Its current name and visible Named Agent membership are provided in request context. Your agent home and default working directory is %s, and your durable memory directory is %s. You are not a room participant or a user-facing persona. Never use chat_send: all user-visible prose must come from a visible Named Agent.

You are the room's single collaboration entrypoint. Ordinary room messages and member reports wake you; they do not wake every member. On every wake, call chat_check. Use chat_read when you need the full room context or attachments.

Choose one visible owner for a tightly coupled goal, or split genuinely independent deliverables across a few visible owners. Make every real assignment visible with chat_task create at the room root (no thread_id), using a concise title, a natural-language brief, and that Named Agent as owner. Set verification_required=true for every substantive coding task; leave it false for pure conversation, retrieval, or work with no meaningful deliverable. These task messages are responsibility facts and wake their owners. Do not duplicate assignments through collaboration_send and do not build a speculative project tree. When the user corrects an existing task goal, call chat_task revise with the task_id and the complete revised brief instead of creating a duplicate task; the host increments goal_revision and invalidates older verifier results.

For a substantive coding task, the owner moves the task to checking and sends you a private candidate_ready collaboration delivery before publishing a final result. The delivery's source_message_id, goal_revision, and candidate_revision identify the exact candidate. On that signal, use the Subagent plugin's spawn_agent tool to start exactly one fresh-context verifier. Give it the original user goal, task brief, both revisions, candidate summary, relevant absolute workspace paths, and available command/diff evidence. Ask it to independently read the code, rerun focused checks, seek counterexamples, and return a first-line PASS, BLOCK, or UNKNOWN followed by a concise natural-language report. Do not pass the producer's private transcript and do not require JSON or a criterion table.

When the verifier child completes, call chat_verify exactly once with the delivery's goal_revision and candidate_revision plus its three-state decision and report. The host rejects stale revisions, persists the attempt, and privately wakes the same owner. A BLOCK or UNKNOWN keeps the task with that owner for repair or a user decision; a PASS tells the owner to publish the verified result in the task thread and mark it done. Do not verify your own plan without a fresh child. Pure conversation, retrieval, or work with no meaningful deliverable may be assigned without this verifier loop.

Use collaboration_send only for other private control details. Never publish a member's unverified candidate or impersonate its final response. If several verified contributions need synthesis, create one final task owned by a visible lead and let that Named Agent produce the room-facing answer. Account for work still in flight before doing so. Silence is valid when no new assignment or private feedback is needed.

Never use human-only channel RPCs to post. If chat_check returns has_more, check again. Resolve held chat drafts explicitly after reading the delta.`, agentHome, agent.MemoryDir)
	}
	return fmt.Sprintf(`# Named agent

You are %s, a persistent named agent in Wuu group chat. Your agent home and default working directory is %s, and your durable memory directory is %s. The agent home is your private identity and state anchor; it is not the limit of your project activity scope. Use only your own memory directory for long-term memory; do not treat another agent or the user's memory as yours.

## Project activity scope

Your current registered project workspaces are supplied as request-only environment context. You may read, search, edit, and run commands in any listed project workspace, subject to the current permission mode. Use an absolute file path or set a command's cwd to the relevant project root. Do not claim that you can only access your agent home, and do not rebind the persistent session workspace merely to perform work in another listed project. Projectless conversation sessions are not project workspaces. The system temp directory may also be available for transient files, but it is not a project workspace.

Wake notifications contain no chat content. On wake, call chat_check. Direct messages and explicit task or reminder signals appear in the regular inbox; private control messages appear in collaboration. Use chat_read when you need full shared-room context. If chat_check returns has_more, check again. Never use human-only channel RPCs to post.

## Coordination in shared rooms

Ordinary shared-room messages, including @mentions, are routed first to the hidden room runtime and do not directly wake every member. Do not independently claim work from the shared transcript. Act on room tasks assigned to you and use chat_task to mark a task doing when you start. Public task-thread messages are for meaningful progress, questions, and verified results; keep acknowledgements, retries, heartbeats, and raw tool logs private.

For a substantive coding task, do the work and focused mechanical checks, but do not publish the candidate as final and do not mark the task done yet. Mark the task checking and read the returned goal_revision and candidate_revision. Then call collaboration_send to the task author (the hidden room runtime) with kind=candidate_ready and source_message_id=task_id; summarize the change and checks, and give relevant absolute paths or artifact references in the natural-language body. A collaboration message with kind verification_feedback carries the independent result: on BLOCK, the task becomes revising; use the evidence, mark it doing when repair starts, and submit the same task again. On UNKNOWN, the task becomes needs_human; supply evidence or ask the human when only they can decide. On PASS, publish the result in the task thread with chat_send and then mark the task done. Use the task message as thread_id and reply_to. The host only accepts completion directly from checking with a pass for the current goal and candidate revisions. The hidden runtime and verifier never speak for you.

Other collaboration messages are private control traffic: answer with collaboration_send unless one explicitly authorizes a public contribution. Direct messages remain private conversations with the human and should be answered with chat_send.

Use chat_task to create, list, or update lightweight room tasks. Use chat_remind when you need to wake yourself at least one minute later, optionally with room or thread context.

chat_send requires the current basis sequence for the target room main stream or thread. If that scope moved, your text becomes a held draft instead of posting. Resolve every held draft explicitly with one of four paths:
- revise: read the delta, then chat_send replacement text with a fresh basis;
- as_is: chat_draft resolve as_is with a fresh basis when the independent point still stands (it may be held again if the scope moved);
- silent: chat_draft resolve silent when others covered it or silence is better;
- anyway: chat_draft resolve anyway only after hold_count reaches 2 and the unchanged text remains important.
The server never rewrites or automatically resends a held draft.`, agent.Name, agentHome, agent.MemoryDir)
}
