package channels

import "time"

type CollaborationSessionPurpose string

const (
	CollaborationSessionConversation CollaborationSessionPurpose = "conversation"
	CollaborationSessionCoordination CollaborationSessionPurpose = "coordination"
	CollaborationSessionWork         CollaborationSessionPurpose = "work"
	CollaborationSessionVerification CollaborationSessionPurpose = "verification"
)

type CollaborationSessionState string

const (
	CollaborationSessionIdle        CollaborationSessionState = "idle"
	CollaborationSessionStarting    CollaborationSessionState = "starting"
	CollaborationSessionRunning     CollaborationSessionState = "running"
	CollaborationSessionInterrupted CollaborationSessionState = "interrupted"
	CollaborationSessionMissing     CollaborationSessionState = "missing"
)

// CollaborationSessionBinding is the durable execution identity behind a
// collaboration principal. Named Agents may own any number of bindings.
type CollaborationSessionBinding struct {
	SessionRef   string                      `json:"session_ref"`
	PrincipalID  string                      `json:"principal_id"`
	NamedAgentID string                      `json:"named_agent_id,omitempty"`
	RoomID       string                      `json:"room_id,omitempty"`
	WorkID       string                      `json:"work_id,omitempty"`
	RunID        string                      `json:"run_id,omitempty"`
	Purpose      CollaborationSessionPurpose `json:"purpose"`
	State        CollaborationSessionState   `json:"state"`
	CreatedAt    time.Time                   `json:"created_at"`
	UpdatedAt    time.Time                   `json:"updated_at"`
}

type CollaborationSessionBindParams struct {
	SessionRef  string
	PrincipalID string
	RoomID      string
	WorkID      string
	RunID       string
	Purpose     CollaborationSessionPurpose
	State       CollaborationSessionState
	AgentID     string
	Token       string
}

type CollaborationSessionListParams struct {
	PrincipalID string
	RoomID      string
	AgentID     string
	Token       string
}

type CollaborationSessionStateParams struct {
	SessionRef string
	State      CollaborationSessionState
	AgentID    string
	Token      string
}
