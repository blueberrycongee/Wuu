package channels

import "time"

type WorkState string

const (
	WorkOpen        WorkState = "open"
	WorkWorking     WorkState = "working"
	WorkChecking    WorkState = "checking"
	WorkRevising    WorkState = "revising"
	WorkIntegrating WorkState = "integrating"
	WorkNeedsHuman  WorkState = "needs_human"
	WorkCompleted   WorkState = "completed"
	WorkFailed      WorkState = "failed"
	WorkCancelled   WorkState = "cancelled"
	WorkInterrupted WorkState = "interrupted"
)

type WorkVerificationState string

const (
	WorkVerificationNotRequired WorkVerificationState = "not_required"
	WorkVerificationPending     WorkVerificationState = "pending"
	WorkVerificationPass        WorkVerificationState = "pass"
	WorkVerificationBlock       WorkVerificationState = "block"
	WorkVerificationUnknown     WorkVerificationState = "unknown"
)

type Work struct {
	ID                         string                `json:"id"`
	RoomID                     string                `json:"room_id"`
	SourceMessageID            string                `json:"source_message_id"`
	OwnerNamedAgentID          string                `json:"owner_named_agent_id"`
	LeadNamedAgentID           string                `json:"lead_named_agent_id,omitempty"`
	Title                      string                `json:"title"`
	Brief                      string                `json:"brief"`
	GoalRevision               int                   `json:"goal_revision"`
	CandidateRevision          int                   `json:"candidate_revision"`
	State                      WorkState             `json:"state"`
	CurrentRunRef              string                `json:"current_run_ref,omitempty"`
	CandidateArtifactRef       string                `json:"candidate_artifact_ref,omitempty"`
	CandidateWorkspaceRevision string                `json:"candidate_workspace_revision,omitempty"`
	VerificationState          WorkVerificationState `json:"verification_state"`
	VerificationRequired       bool                  `json:"verification_required"`
	PendingDeliveryRefs        []string              `json:"pending_delivery_refs,omitempty"`
	MaxVerifierAttempts        int                   `json:"max_verifier_attempts"`
	MaxCandidates              int                   `json:"max_candidates"`
	VerifierAttemptsUsed       int                   `json:"verifier_attempts_used"`
	CandidatesUsed             int                   `json:"candidates_used"`
	FanoutReason               string                `json:"fanout_reason,omitempty"`
	ChecksSummary              string                `json:"checks_summary,omitempty"`
	ChangedFilesCount          int                   `json:"changed_files_count,omitempty"`
	UnresolvedItems            string                `json:"unresolved_items,omitempty"`
	FailureReason              string                `json:"failure_reason,omitempty"`
	CancelledAt                time.Time             `json:"cancelled_at,omitempty"`
	CreatedAt                  time.Time             `json:"created_at"`
	UpdatedAt                  time.Time             `json:"updated_at"`
	Runs                       []WorkRun             `json:"runs,omitempty"`
	Artifacts                  []WorkArtifact        `json:"artifacts,omitempty"`
	Events                     []WorkEvent           `json:"events,omitempty"`
	Verification               *TaskVerification     `json:"verification,omitempty"`
}

type WorkEvent struct {
	ID                string    `json:"id"`
	WorkID            string    `json:"work_id"`
	Kind              string    `json:"kind"`
	State             string    `json:"state,omitempty"`
	Summary           string    `json:"summary,omitempty"`
	GoalRevision      int       `json:"goal_revision"`
	CandidateRevision int       `json:"candidate_revision"`
	CreatedAt         time.Time `json:"created_at"`
}

type WorkRunKind string

const (
	WorkRunProducer    WorkRunKind = "producer"
	WorkRunVerifier    WorkRunKind = "verifier"
	WorkRunSelector    WorkRunKind = "selector"
	WorkRunIntegration WorkRunKind = "integration"
)

type WorkRunState string

const (
	WorkRunQueued      WorkRunState = "queued"
	WorkRunRunning     WorkRunState = "running"
	WorkRunCompleted   WorkRunState = "completed"
	WorkRunFailed      WorkRunState = "failed"
	WorkRunCancelled   WorkRunState = "cancelled"
	WorkRunInterrupted WorkRunState = "interrupted"
)

type WorkRun struct {
	ID                string       `json:"id"`
	WorkID            string       `json:"work_id"`
	Kind              WorkRunKind  `json:"kind"`
	Profile           string       `json:"profile,omitempty"`
	SessionRef        string       `json:"session_ref,omitempty"`
	State             WorkRunState `json:"state"`
	GoalRevision      int          `json:"goal_revision"`
	CandidateRevision int          `json:"candidate_revision"`
	WorkspaceRevision string       `json:"workspace_revision,omitempty"`
	Provider          string       `json:"provider,omitempty"`
	Model             string       `json:"model,omitempty"`
	InputTokens       int64        `json:"input_tokens,omitempty"`
	OutputTokens      int64        `json:"output_tokens,omitempty"`
	ChecksRerun       int          `json:"checks_rerun,omitempty"`
	FindingsCount     int          `json:"findings_count,omitempty"`
	Outcome           string       `json:"outcome,omitempty"`
	RepairOutcome     string       `json:"repair_outcome,omitempty"`
	StartedAt         time.Time    `json:"started_at,omitempty"`
	EndedAt           time.Time    `json:"ended_at,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
}

type WorkArtifactKind string

const (
	WorkArtifactCandidate  WorkArtifactKind = "candidate"
	WorkArtifactDiff       WorkArtifactKind = "diff"
	WorkArtifactSnapshot   WorkArtifactKind = "snapshot"
	WorkArtifactCheckLog   WorkArtifactKind = "check_log"
	WorkArtifactScreenshot WorkArtifactKind = "screenshot"
	WorkArtifactReport     WorkArtifactKind = "report"
	WorkArtifactOther      WorkArtifactKind = "other"
)

type WorkArtifact struct {
	ID                string           `json:"id"`
	WorkID            string           `json:"work_id"`
	RunID             string           `json:"run_id,omitempty"`
	Kind              WorkArtifactKind `json:"kind"`
	URI               string           `json:"uri"`
	Label             string           `json:"label,omitempty"`
	Summary           string           `json:"summary,omitempty"`
	WorkspaceRevision string           `json:"workspace_revision,omitempty"`
	CreatedAt         time.Time        `json:"created_at"`
}

type WorkRunStartParams struct {
	WorkID            string
	Kind              WorkRunKind
	Profile           string
	SessionRef        string
	WorkspaceRevision string
	AgentID           string
	Token             string
}

type WorkRunFinishParams struct {
	WorkID        string
	RunID         string
	State         WorkRunState
	Outcome       string
	Provider      string
	Model         string
	InputTokens   int64
	OutputTokens  int64
	ChecksRerun   int
	FindingsCount int
	RepairOutcome string
	AgentID       string
	Token         string
}

type WorkArtifactAddParams struct {
	WorkID            string
	RunID             string
	Kind              WorkArtifactKind
	URI               string
	Label             string
	Summary           string
	WorkspaceRevision string
	AgentID           string
	Token             string
}

type WorkPolicyUpdateParams struct {
	WorkID              string
	LeadNamedAgentID    string
	MaxVerifierAttempts int
	MaxCandidates       int
	FanoutReason        string
	AgentID             string
	Token               string
}

type WorkEvidenceUpdateParams struct {
	WorkID            string
	ChecksSummary     string
	ChangedFilesCount int
	UnresolvedItems   string
	AgentID           string
	Token             string
}
