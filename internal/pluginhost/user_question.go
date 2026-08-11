package pluginhost

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	UserQuestionRequested = "requested"
	UserQuestionResolved  = "resolved"
)

type UserQuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type UserQuestion struct {
	ID          string               `json:"id"`
	Question    string               `json:"question"`
	Header      string               `json:"header,omitempty"`
	Detail      string               `json:"detail,omitempty"`
	Options     []UserQuestionOption `json:"options,omitempty"`
	MultiSelect bool                 `json:"multi_select,omitempty"`
	AllowCustom bool                 `json:"allow_custom,omitempty"`
}

type UserQuestionAskParams struct {
	Questions []UserQuestion `json:"questions"`
}

type UserQuestionOwner struct {
	PluginID    string
	ExecutionID string
	SessionID   string
	ThreadID    string
	TurnID      string
	ActorID     string
	CallID      string
}

type UserQuestionAnswerItem struct {
	ID       string   `json:"id"`
	Selected []string `json:"selected"`
	Custom   string   `json:"custom,omitempty"`
}

type UserQuestionAnswer struct {
	Answers []UserQuestionAnswerItem `json:"answers"`
}

type UserQuestionRequest struct {
	RequestID   string         `json:"request_id"`
	PluginID    string         `json:"plugin_id"`
	ExecutionID string         `json:"execution_id"`
	SessionID   string         `json:"session_id,omitempty"`
	ThreadID    string         `json:"thread_id"`
	TurnID      string         `json:"turn_id"`
	ActorID     string         `json:"actor_id,omitempty"`
	CallID      string         `json:"call_id,omitempty"`
	Questions   []UserQuestion `json:"questions"`
	CreatedAt   time.Time      `json:"created_at"`
}

type UserQuestionEvent struct {
	Type      string               `json:"type"`
	Request   *UserQuestionRequest `json:"request,omitempty"`
	RequestID string               `json:"request_id,omitempty"`
	ThreadID  string               `json:"thread_id,omitempty"`
	Outcome   string               `json:"outcome,omitempty"`
}

type UserQuestionError struct {
	Code    string
	Message string
}

func (e *UserQuestionError) Error() string { return e.Message }

type userQuestionResult struct {
	answer UserQuestionAnswer
	err    error
}

type pendingUserQuestion struct {
	request UserQuestionRequest
	result  chan userQuestionResult
}

// UserQuestionBroker owns live, execution-bound human questions. Requests are
// intentionally not durable: cancelling the execution or closing the runtime
// resolves every waiter instead of reviving an orphaned Tool after restart.
type UserQuestionBroker struct {
	mu          sync.Mutex
	closed      bool
	pending     map[string]*pendingUserQuestion
	subscribers map[uint64]chan UserQuestionEvent
	nextSubID   uint64
}

func NewUserQuestionBroker() *UserQuestionBroker {
	return &UserQuestionBroker{
		pending:     make(map[string]*pendingUserQuestion),
		subscribers: make(map[uint64]chan UserQuestionEvent),
	}
}

func (b *UserQuestionBroker) Ask(ctx context.Context, owner UserQuestionOwner, params UserQuestionAskParams) (UserQuestionAnswer, error) {
	if err := validateUserQuestionAsk(owner, params); err != nil {
		return UserQuestionAnswer{}, err
	}
	requestID, err := newUserQuestionRequestID()
	if err != nil {
		return UserQuestionAnswer{}, &UserQuestionError{Code: "service_unavailable", Message: "create user question id"}
	}
	entry := &pendingUserQuestion{
		request: UserQuestionRequest{
			RequestID: requestID, PluginID: strings.TrimSpace(owner.PluginID), ExecutionID: strings.TrimSpace(owner.ExecutionID),
			SessionID: strings.TrimSpace(owner.SessionID),
			ThreadID:  strings.TrimSpace(owner.ThreadID), TurnID: strings.TrimSpace(owner.TurnID),
			ActorID: strings.TrimSpace(owner.ActorID), CallID: strings.TrimSpace(owner.CallID),
			Questions: cloneUserQuestions(params.Questions), CreatedAt: time.Now().UTC(),
		},
		result: make(chan userQuestionResult, 1),
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return UserQuestionAnswer{}, &UserQuestionError{Code: "service_unavailable", Message: "user interaction is unavailable"}
	}
	b.pending[requestID] = entry
	subscribers := b.subscribersLocked()
	b.mu.Unlock()
	b.publish(subscribers, UserQuestionEvent{Type: UserQuestionRequested, Request: cloneUserQuestionRequest(&entry.request)})

	select {
	case result := <-entry.result:
		return result.answer, result.err
	case <-ctx.Done():
		cancelErr := userQuestionContextError(ctx)
		if b.resolve(requestID, userQuestionResult{err: cancelErr}, "cancelled") {
			return UserQuestionAnswer{}, cancelErr
		}
		result := <-entry.result
		return result.answer, result.err
	}
}

func (b *UserQuestionBroker) Respond(requestID string, answer UserQuestionAnswer) error {
	requestID = strings.TrimSpace(requestID)
	b.mu.Lock()
	entry := b.pending[requestID]
	if entry == nil {
		b.mu.Unlock()
		return &UserQuestionError{Code: "question_not_pending", Message: "user question is no longer pending"}
	}
	if err := validateUserQuestionAnswer(entry.request.Questions, answer); err != nil {
		b.mu.Unlock()
		return err
	}
	delete(b.pending, requestID)
	subscribers := b.subscribersLocked()
	b.mu.Unlock()
	entry.result <- userQuestionResult{answer: cloneUserQuestionAnswer(answer)}
	b.publish(subscribers, UserQuestionEvent{Type: UserQuestionResolved, RequestID: requestID, ThreadID: entry.request.ThreadID, Outcome: "answered"})
	return nil
}

func (b *UserQuestionBroker) Cancel(requestID string) error {
	if !b.resolve(strings.TrimSpace(requestID), userQuestionResult{err: &UserQuestionError{Code: "question_cancelled", Message: "user cancelled the question"}}, "cancelled") {
		return &UserQuestionError{Code: "question_not_pending", Message: "user question is no longer pending"}
	}
	return nil
}

func (b *UserQuestionBroker) List(threadID string) []UserQuestionRequest {
	threadID = strings.TrimSpace(threadID)
	b.mu.Lock()
	defer b.mu.Unlock()
	requests := make([]UserQuestionRequest, 0, len(b.pending))
	for _, entry := range b.pending {
		if threadID != "" && entry.request.ThreadID != threadID {
			continue
		}
		requests = append(requests, *cloneUserQuestionRequest(&entry.request))
	}
	return requests
}

func (b *UserQuestionBroker) Subscribe(buffer int) (<-chan UserQuestionEvent, func()) {
	if buffer < 1 {
		buffer = 1
	}
	b.mu.Lock()
	if b.closed {
		closed := make(chan UserQuestionEvent)
		close(closed)
		b.mu.Unlock()
		return closed, func() {}
	}
	b.nextSubID++
	id := b.nextSubID
	ch := make(chan UserQuestionEvent, buffer)
	b.subscribers[id] = ch
	b.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			if _, ok := b.subscribers[id]; ok {
				delete(b.subscribers, id)
			}
			b.mu.Unlock()
		})
	}
}

func (b *UserQuestionBroker) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	pending := b.pending
	b.pending = make(map[string]*pendingUserQuestion)
	for id := range b.subscribers {
		delete(b.subscribers, id)
	}
	b.mu.Unlock()
	for _, entry := range pending {
		entry.result <- userQuestionResult{err: &UserQuestionError{Code: "service_unavailable", Message: "user interaction is unavailable"}}
	}
}

func (b *UserQuestionBroker) resolve(requestID string, result userQuestionResult, outcome string) bool {
	b.mu.Lock()
	entry := b.pending[requestID]
	if entry == nil {
		b.mu.Unlock()
		return false
	}
	delete(b.pending, requestID)
	subscribers := b.subscribersLocked()
	b.mu.Unlock()
	entry.result <- result
	b.publish(subscribers, UserQuestionEvent{Type: UserQuestionResolved, RequestID: requestID, ThreadID: entry.request.ThreadID, Outcome: outcome})
	return true
}

func (b *UserQuestionBroker) subscribersLocked() []chan UserQuestionEvent {
	result := make([]chan UserQuestionEvent, 0, len(b.subscribers))
	for _, ch := range b.subscribers {
		result = append(result, ch)
	}
	return result
}

func (b *UserQuestionBroker) publish(subscribers []chan UserQuestionEvent, event UserQuestionEvent) {
	for _, ch := range subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func validateUserQuestionAsk(owner UserQuestionOwner, params UserQuestionAskParams) error {
	if strings.TrimSpace(owner.PluginID) == "" || strings.TrimSpace(owner.ExecutionID) == "" {
		return &UserQuestionError{Code: "invalid_request", Message: "user questions require a live plugin execution"}
	}
	if strings.TrimSpace(owner.ThreadID) == "" || strings.TrimSpace(owner.TurnID) == "" || strings.TrimSpace(owner.CallID) == "" {
		return &UserQuestionError{Code: "invalid_request", Message: "user questions require a scoped tool execution"}
	}
	if len(params.Questions) == 0 || len(params.Questions) > 8 {
		return &UserQuestionError{Code: "invalid_request", Message: "user questions must contain between 1 and 8 items"}
	}
	ids := make(map[string]struct{}, len(params.Questions))
	for index, question := range params.Questions {
		id := strings.TrimSpace(question.ID)
		if id == "" || len(id) > 64 {
			return &UserQuestionError{Code: "invalid_request", Message: fmt.Sprintf("question %d requires an id of at most 64 bytes", index)}
		}
		if _, exists := ids[id]; exists {
			return &UserQuestionError{Code: "invalid_request", Message: fmt.Sprintf("duplicate question id %q", id)}
		}
		ids[id] = struct{}{}
		if strings.TrimSpace(question.Question) == "" || len(question.Question) > 4096 {
			return &UserQuestionError{Code: "invalid_request", Message: fmt.Sprintf("question %q requires text of at most 4096 bytes", id)}
		}
		if len(question.Header) > 256 || len(question.Detail) > 4096 {
			return &UserQuestionError{Code: "invalid_request", Message: fmt.Sprintf("question %q exceeds display text limits", id)}
		}
		if len(question.Options) > 20 {
			return &UserQuestionError{Code: "invalid_request", Message: fmt.Sprintf("question %q exceeds 20 options", id)}
		}
		labels := make(map[string]struct{}, len(question.Options))
		for _, option := range question.Options {
			label := strings.TrimSpace(option.Label)
			if label == "" || len(label) > 256 {
				return &UserQuestionError{Code: "invalid_request", Message: fmt.Sprintf("question %q has an invalid option label", id)}
			}
			if len(option.Description) > 1024 {
				return &UserQuestionError{Code: "invalid_request", Message: fmt.Sprintf("question %q has an option description over 1024 bytes", id)}
			}
			if _, exists := labels[label]; exists {
				return &UserQuestionError{Code: "invalid_request", Message: fmt.Sprintf("question %q has duplicate option %q", id, label)}
			}
			labels[label] = struct{}{}
		}
		if len(question.Options) == 0 && !question.AllowCustom {
			return &UserQuestionError{Code: "invalid_request", Message: fmt.Sprintf("question %q must offer options or custom input", id)}
		}
	}
	return nil
}

func userQuestionContextError(ctx context.Context) error {
	cause := context.Cause(ctx)
	var questionErr *UserQuestionError
	if errors.As(cause, &questionErr) {
		return questionErr
	}
	return &UserQuestionError{Code: "execution_cancelled", Message: "owning execution was cancelled"}
}

func validateUserQuestionAnswer(questions []UserQuestion, answer UserQuestionAnswer) error {
	if len(answer.Answers) != len(questions) {
		return &UserQuestionError{Code: "invalid_answer", Message: "answer must contain exactly one item per question"}
	}
	byID := make(map[string]UserQuestion, len(questions))
	for _, question := range questions {
		byID[question.ID] = question
	}
	seen := make(map[string]struct{}, len(answer.Answers))
	for _, item := range answer.Answers {
		question, ok := byID[item.ID]
		if !ok {
			return &UserQuestionError{Code: "invalid_answer", Message: fmt.Sprintf("answer references unknown question %q", item.ID)}
		}
		if _, exists := seen[item.ID]; exists {
			return &UserQuestionError{Code: "invalid_answer", Message: fmt.Sprintf("answer repeats question %q", item.ID)}
		}
		seen[item.ID] = struct{}{}
		if !question.MultiSelect && len(item.Selected) > 1 {
			return &UserQuestionError{Code: "invalid_answer", Message: fmt.Sprintf("question %q accepts only one selection", item.ID)}
		}
		allowed := make(map[string]struct{}, len(question.Options))
		for _, option := range question.Options {
			allowed[option.Label] = struct{}{}
		}
		selected := make(map[string]struct{}, len(item.Selected))
		for _, label := range item.Selected {
			if _, ok := allowed[label]; !ok {
				return &UserQuestionError{Code: "invalid_answer", Message: fmt.Sprintf("question %q answer selects unknown option %q", item.ID, label)}
			}
			if _, exists := selected[label]; exists {
				return &UserQuestionError{Code: "invalid_answer", Message: fmt.Sprintf("question %q answer repeats option %q", item.ID, label)}
			}
			selected[label] = struct{}{}
		}
		if strings.TrimSpace(item.Custom) != "" && !question.AllowCustom {
			return &UserQuestionError{Code: "invalid_answer", Message: fmt.Sprintf("question %q does not accept custom input", item.ID)}
		}
		if len(item.Custom) > 4096 {
			return &UserQuestionError{Code: "invalid_answer", Message: fmt.Sprintf("question %q custom input exceeds 4096 bytes", item.ID)}
		}
		if len(item.Selected) == 0 && strings.TrimSpace(item.Custom) == "" {
			return &UserQuestionError{Code: "invalid_answer", Message: fmt.Sprintf("question %q requires a selection or custom input", item.ID)}
		}
	}
	return nil
}

func newUserQuestionRequestID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "question-" + hex.EncodeToString(raw[:]), nil
}

func cloneUserQuestions(questions []UserQuestion) []UserQuestion {
	result := make([]UserQuestion, len(questions))
	for index, question := range questions {
		result[index] = question
		result[index].Options = append([]UserQuestionOption(nil), question.Options...)
	}
	return result
}

func cloneUserQuestionRequest(request *UserQuestionRequest) *UserQuestionRequest {
	if request == nil {
		return nil
	}
	clone := *request
	clone.Questions = cloneUserQuestions(request.Questions)
	return &clone
}

func cloneUserQuestionAnswer(answer UserQuestionAnswer) UserQuestionAnswer {
	clone := UserQuestionAnswer{Answers: make([]UserQuestionAnswerItem, len(answer.Answers))}
	for index, item := range answer.Answers {
		clone.Answers[index] = item
		clone.Answers[index].Selected = append([]string(nil), item.Selected...)
	}
	return clone
}

func IsUserQuestionErrorCode(err error, code string) bool {
	var target *UserQuestionError
	return errors.As(err, &target) && target.Code == code
}
