package channels

import (
	"context"
	"errors"
	"strings"
	"time"
)

type AgentClient struct {
	service       *Service
	agentID       string
	principalKind PrincipalKind
	token         string
}

func (s *Service) BindAgent(ctx context.Context, agentID string) (*AgentClient, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, errors.New("named agent id is required")
	}
	token, err := s.loadAgentToken(ctx, agentID)
	if err != nil {
		return nil, err
	}
	_, err = s.GetNamedAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	return &AgentClient{service: s, agentID: agentID, principalKind: PrincipalNamedAgent, token: token}, nil
}

func (s *Service) BindRuntime(ctx context.Context, runtimeID string) (*AgentClient, error) {
	runtimeID = strings.TrimSpace(runtimeID)
	if runtimeID == "" {
		return nil, errors.New("room runtime id is required")
	}
	runtime, err := s.GetRoomRuntime(ctx, runtimeID)
	if err != nil {
		return nil, err
	}
	token, err := s.loadPrincipalToken(ctx, runtimeID)
	if err != nil {
		return nil, err
	}
	return &AgentClient{service: s, agentID: runtimeID, principalKind: runtime.Kind, token: token}, nil
}

func (c *AgentClient) AgentID() string {
	if c == nil {
		return ""
	}
	return c.agentID
}

func (c *AgentClient) IsRoomRuntime() bool {
	if c == nil {
		return false
	}
	return c.principalKind == PrincipalRoomRuntime
}

func (c *AgentClient) Check(ctx context.Context) (CheckResult, error) {
	if c == nil || c.service == nil {
		return CheckResult{}, errors.New("chat agent is not bound")
	}
	return c.service.Check(ctx, c.agentID, c.token)
}

func (c *AgentClient) ReadInbox(ctx context.Context, itemIDs []string) ([]Message, error) {
	if c == nil || c.service == nil {
		return nil, errors.New("chat agent is not bound")
	}
	return c.service.ReadInboxMessages(ctx, c.agentID, c.token, itemIDs)
}

func (c *AgentClient) ReadRoom(ctx context.Context, roomID string, afterSeq int64, limit int) ([]Message, error) {
	if c == nil || c.service == nil {
		return nil, errors.New("chat agent is not bound")
	}
	return c.service.ReadAgentMessages(ctx, c.agentID, c.token, roomID, afterSeq, limit)
}

func (c *AgentClient) Send(ctx context.Context, params AgentSendParams) (SendResult, error) {
	if c == nil || c.service == nil {
		return SendResult{}, errors.New("chat agent is not bound")
	}
	if params.BasisSeq < 0 {
		return SendResult{}, errors.New("message basis sequence cannot be negative")
	}
	params.AgentID = c.agentID
	params.Token = c.token
	return c.service.SendAgent(ctx, params)
}

func (c *AgentClient) SendCollaboration(ctx context.Context, params CollaborationSendParams) (CollaborationMessage, error) {
	if c == nil || c.service == nil {
		return CollaborationMessage{}, errors.New("chat agent is not bound")
	}
	params.AgentID = c.agentID
	params.Token = c.token
	return c.service.SendCollaboration(ctx, params)
}

func (c *AgentClient) ListDrafts(ctx context.Context) ([]Draft, error) {
	if c == nil || c.service == nil {
		return nil, errors.New("chat agent is not bound")
	}
	return c.service.ListDrafts(ctx, c.agentID, c.token)
}

func (c *AgentClient) CreateTask(ctx context.Context, params TaskCreateParams) (Message, error) {
	if c == nil || c.service == nil {
		return Message{}, errors.New("chat agent is not bound")
	}
	params.AgentID = c.agentID
	params.Token = c.token
	return c.service.CreateTask(ctx, params)
}

func (c *AgentClient) UpdateTask(ctx context.Context, params TaskUpdateParams) (Message, error) {
	if c == nil || c.service == nil {
		return Message{}, errors.New("chat agent is not bound")
	}
	params.AgentID = c.agentID
	params.Token = c.token
	return c.service.UpdateTask(ctx, params)
}

func (c *AgentClient) ListTasks(ctx context.Context, roomID string) ([]Message, error) {
	if c == nil || c.service == nil {
		return nil, errors.New("chat agent is not bound")
	}
	return c.service.ListTasks(ctx, TaskListParams{RoomID: roomID, AgentID: c.agentID, Token: c.token})
}

func (c *AgentClient) SubmitTaskVerification(ctx context.Context, params TaskVerificationSubmitParams) (TaskVerificationSubmitResult, error) {
	if c == nil || c.service == nil {
		return TaskVerificationSubmitResult{}, errors.New("chat agent is not bound")
	}
	params.AgentID = c.agentID
	params.Token = c.token
	return c.service.SubmitTaskVerification(ctx, params)
}

func (c *AgentClient) SetReminder(ctx context.Context, params ReminderSetParams) (Reminder, error) {
	if c == nil || c.service == nil {
		return Reminder{}, errors.New("chat agent is not bound")
	}
	params.AgentID = c.agentID
	params.Token = c.token
	return c.service.SetReminder(ctx, params)
}

func (c *AgentClient) SetReminderAfter(ctx context.Context, delay time.Duration, params ReminderSetParams) (Reminder, error) {
	if c == nil || c.service == nil {
		return Reminder{}, errors.New("chat agent is not bound")
	}
	params.AgentID = c.agentID
	params.Token = c.token
	return c.service.SetReminderAfter(ctx, params, delay)
}

func (c *AgentClient) ListReminders(ctx context.Context, state ReminderState) ([]Reminder, error) {
	if c == nil || c.service == nil {
		return nil, errors.New("chat agent is not bound")
	}
	return c.service.ListReminders(ctx, ReminderListParams{AgentID: c.agentID, Token: c.token, State: state})
}

func (c *AgentClient) CancelReminder(ctx context.Context, reminderID string) (Reminder, error) {
	if c == nil || c.service == nil {
		return Reminder{}, errors.New("chat agent is not bound")
	}
	return c.service.CancelReminder(ctx, ReminderCancelParams{AgentID: c.agentID, Token: c.token, ReminderID: reminderID})
}

func (c *AgentClient) ResolveDraft(ctx context.Context, params ResolveDraftParams) (DraftResult, error) {
	if c == nil || c.service == nil {
		return DraftResult{}, errors.New("chat agent is not bound")
	}
	params.AgentID = c.agentID
	params.Token = c.token
	return c.service.ResolveDraft(ctx, params)
}

func (c *AgentClient) GetWork(ctx context.Context, workID string) (Work, error) {
	if c == nil || c.service == nil {
		return Work{}, errors.New("chat agent is not bound")
	}
	work, err := c.service.GetWork(ctx, workID)
	if err != nil {
		return Work{}, err
	}
	if work.OwnerNamedAgentID != c.agentID && work.LeadNamedAgentID != c.agentID {
		if !c.IsRoomRuntime() {
			return Work{}, ErrUnauthorized
		}
		runtime, err := c.service.GetRoomRuntime(ctx, c.agentID)
		if err != nil || runtime.RoomID != work.RoomID {
			return Work{}, ErrUnauthorized
		}
	}
	return work, nil
}

func (c *AgentClient) ListWorks(ctx context.Context, roomID string) ([]Work, error) {
	if c == nil || c.service == nil {
		return nil, errors.New("chat agent is not bound")
	}
	return c.service.ListWorks(ctx, roomID, c.agentID, c.token)
}

func (c *AgentClient) StartWorkRun(ctx context.Context, params WorkRunStartParams) (WorkRun, error) {
	if c == nil || c.service == nil {
		return WorkRun{}, errors.New("chat agent is not bound")
	}
	params.AgentID, params.Token = c.agentID, c.token
	return c.service.StartWorkRun(ctx, params)
}

func (c *AgentClient) FinishWorkRun(ctx context.Context, params WorkRunFinishParams) (WorkRun, error) {
	if c == nil || c.service == nil {
		return WorkRun{}, errors.New("chat agent is not bound")
	}
	params.AgentID, params.Token = c.agentID, c.token
	return c.service.FinishWorkRun(ctx, params)
}

func (c *AgentClient) AddWorkArtifact(ctx context.Context, params WorkArtifactAddParams) (WorkArtifact, error) {
	if c == nil || c.service == nil {
		return WorkArtifact{}, errors.New("chat agent is not bound")
	}
	params.AgentID, params.Token = c.agentID, c.token
	return c.service.AddWorkArtifact(ctx, params)
}

func (c *AgentClient) CancelWork(ctx context.Context, workID, reason string) (Work, error) {
	if c == nil || c.service == nil {
		return Work{}, errors.New("chat agent is not bound")
	}
	return c.service.CancelWork(ctx, workID, reason, c.agentID, c.token)
}

func (c *AgentClient) UpdateWorkPolicy(ctx context.Context, params WorkPolicyUpdateParams) (Work, error) {
	if c == nil || c.service == nil {
		return Work{}, errors.New("chat agent is not bound")
	}
	params.AgentID, params.Token = c.agentID, c.token
	return c.service.UpdateWorkPolicy(ctx, params)
}

func (c *AgentClient) UpdateWorkEvidence(ctx context.Context, params WorkEvidenceUpdateParams) (Work, error) {
	if c == nil || c.service == nil {
		return Work{}, errors.New("chat agent is not bound")
	}
	params.AgentID, params.Token = c.agentID, c.token
	return c.service.UpdateWorkEvidence(ctx, params)
}
