package pluginhost

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	ProtocolName    = "wuu-plugin-v1"
	ProtocolVersion = 1

	// maxResponseLineSize bounds the JSON-lines wire format so a single
	// plugin response cannot grow unbounded in memory. The limit applies to
	// one line including its trailing newline.
	maxResponseLineSize = 4 << 20 // 4 MiB
)

type ProcessConfig struct {
	ID                string
	Command           string
	Args              []string
	Env               map[string]string
	PluginRoot        string
	ProjectRoot       string
	WuuHome           string
	WorkspaceStateDir string
	Timeout           time.Duration
	// HostServiceHandler is the live dispatcher for Plugin -> Host calls.
	HostServiceHandler HostServiceHandler
	// SupportedHostServices is an optional assertion about HostServiceHandler's
	// declaration. When set it must match exactly; names alone never enable a
	// service. The handler declaration is what is advertised on the wire.
	SupportedHostServices []HostServiceMethod
	// PrepareOnly leaves a lifecycle-aware runtime initialized but inactive.
	// The owning generation activates it after its durable commit succeeds.
	PrepareOnly bool
}

type rpcRequest struct {
	ID     string `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

type rpcResponse struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

type remoteCallError struct {
	code    string
	message string
}

func (e *remoteCallError) Error() string { return e.message }

type ProcessClient struct {
	config             ProcessConfig
	cmd                *exec.Cmd
	stdin              io.WriteCloser
	scanner            *bufio.Scanner
	seq                atomic.Uint64
	writeMu            sync.Mutex
	pendingMu          sync.Mutex
	pending            map[string]chan rpcResponse
	readerDone         chan struct{}
	readerErrMu        sync.Mutex
	readerErr          error
	processCtx         context.Context
	processCancel      context.CancelFunc
	mu                 sync.RWMutex
	status             Status
	tools              []ToolRegistration
	protocol           int
	capabilities       []CapabilityDescriptor
	negotiated         map[HostServiceMethod]struct{}
	activationServices map[HostServiceMethod]struct{}
	lifecycleVersion   int
	stderr             lockedBuffer
	stopMu             sync.Mutex
	stopped            bool
	stopErr            error
	serviceClose       sync.Once
}

func Start(ctx context.Context, config ProcessConfig) (*ProcessClient, error) {
	config.ID = strings.TrimSpace(config.ID)
	config.Command = strings.TrimSpace(config.Command)
	if config.ID == "" || config.Command == "" {
		return nil, errors.New("plugin process requires id and command")
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	supported, err := configuredHostServices(config)
	if err != nil {
		return nil, fmt.Errorf("plugin %q host services: %w", config.ID, err)
	}
	config.SupportedHostServices = supported

	cmd := exec.Command(config.Command, config.Args...)
	cmd.Dir = config.PluginRoot
	cmd.Env = buildEnv(config.Env)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("plugin %q stdout: %w", config.ID, err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("plugin %q stdin: %w", config.ID, err)
	}
	client := &ProcessClient{
		config:     config,
		cmd:        cmd,
		stdin:      stdin,
		status:     Status{ID: config.ID, State: StateStarting},
		pending:    make(map[string]chan rpcResponse),
		readerDone: make(chan struct{}),
	}
	client.processCtx, client.processCancel = context.WithCancel(context.Background())
	// Initialization is a prepare phase. Only read-only services are available
	// until the generation and its durable policy commit.
	client.negotiated = preflightHostServices(config.SupportedHostServices)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 4096), maxResponseLineSize)
	client.scanner = scanner
	cmd.Stderr = &client.stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start plugin %q: %w", config.ID, err)
	}
	go client.readLoop()

	initCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()
	var initialized CapabilityInitializeResult
	if err := client.call(initCtx, "initialize", CapabilityInitializeParams{
		InitializeParams: InitializeParams{
			ProtocolVersion:   ProtocolVersion,
			PluginID:          config.ID,
			PluginRoot:        config.PluginRoot,
			ProjectRoot:       config.ProjectRoot,
			WuuHome:           config.WuuHome,
			WorkspaceStateDir: config.WorkspaceStateDir,
		},
		CapabilityProtocolVersion: CapabilityProtocolVersion,
		SupportedHostServices:     append([]HostServiceMethod(nil), config.SupportedHostServices...),
		LifecycleVersion:          RuntimeLifecycleVersion,
	}, &initialized); err != nil {
		_ = client.stopProcess()
		client.setFailure(err)
		return nil, fmt.Errorf("initialize plugin %q: %w", config.ID, err)
	}
	seen := make(map[Hook]struct{}, len(initialized.Hooks))
	for _, hook := range initialized.Hooks {
		if !IsValidHook(hook) {
			_ = client.stopProcess()
			return nil, fmt.Errorf("initialize plugin %q: unknown hook %q", config.ID, hook)
		}
		if _, ok := seen[hook]; ok {
			_ = client.stopProcess()
			return nil, fmt.Errorf("initialize plugin %q: duplicate hook %q", config.ID, hook)
		}
		seen[hook] = struct{}{}
	}
	if err := validateToolRegistrations(initialized.Tools); err != nil {
		_ = client.stopProcess()
		client.setFailure(err)
		return nil, fmt.Errorf("initialize plugin %q: invalid tool registrations: %w", config.ID, err)
	}
	if err := ValidateCapabilityNegotiation(initialized, config.SupportedHostServices); err != nil {
		_ = client.stopProcess()
		negotiationErr := &CapabilityNegotiationError{Err: err}
		client.setFailure(negotiationErr)
		return nil, fmt.Errorf("initialize plugin %q: invalid capability negotiation: %w", config.ID, negotiationErr)
	}
	if config.PrepareOnly && initialized.LifecycleVersion != RuntimeLifecycleVersion {
		_ = client.stopProcess()
		err := fmt.Errorf("runtime lifecycle version %d is required for generation preflight", RuntimeLifecycleVersion)
		client.setFailure(err)
		return nil, fmt.Errorf("initialize plugin %q: %w", config.ID, err)
	}
	client.mu.Lock()
	client.status.State = StatePrepared
	client.status.Hooks = append([]Hook(nil), initialized.Hooks...)
	client.tools = make([]ToolRegistration, len(initialized.Tools))
	for index, registration := range initialized.Tools {
		client.tools[index] = cloneToolRegistration(registration)
	}
	client.protocol = initialized.ProtocolVersion
	if client.protocol == 0 {
		client.protocol = ProtocolVersion
	}
	client.capabilities = cloneCapabilityDescriptors(initialized.Capabilities)
	client.activationServices = negotiatedHostServices(initialized.RequiredHostServices, config.SupportedHostServices)
	client.negotiated = preflightNegotiatedHostServices(client.activationServices)
	client.lifecycleVersion = initialized.LifecycleVersion
	client.mu.Unlock()
	if !config.PrepareOnly {
		if err := client.Activate(ctx); err != nil {
			return nil, fmt.Errorf("activate plugin %q: %w", config.ID, err)
		}
	}
	return client, nil
}

func (c *ProcessClient) ID() string { return c.config.ID }

func (c *ProcessClient) Hooks() []Hook {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]Hook(nil), c.status.Hooks...)
}

func (c *ProcessClient) Tools() []ToolRegistration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	tools := make([]ToolRegistration, len(c.tools))
	for index, registration := range c.tools {
		tools[index] = cloneToolRegistration(registration)
	}
	return tools
}

func (c *ProcessClient) ProtocolVersion() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.protocol
}

func (c *ProcessClient) Capabilities() []CapabilityDescriptor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return cloneCapabilityDescriptors(c.capabilities)
}

func (c *ProcessClient) InvokeCapability(ctx context.Context, params CapabilityInvokeParams) (CapabilityInvokeResult, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	var result CapabilityInvokeResult
	if err := c.call(callCtx, "capability.invoke", params, &result); err != nil {
		c.failFatalCall(err)
		return CapabilityInvokeResult{}, err
	}
	if len(result.Output) == 0 {
		protocolErr := errors.New("capability returned empty output")
		c.fail(protocolErr)
		return CapabilityInvokeResult{}, protocolErr
	}
	result.Output = append(json.RawMessage(nil), result.Output...)
	return result, nil
}

func (c *ProcessClient) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	status := c.status
	status.Hooks = append([]Hook(nil), status.Hooks...)
	return status
}

// Activate opens the negotiated service set and starts lifecycle-aware
// background effects. Generation owners call this only after durable commit.
func (c *ProcessClient) Activate(ctx context.Context) error {
	if c == nil {
		return errors.New("plugin process is not initialized")
	}
	c.mu.Lock()
	if c.status.State == StateActive {
		c.mu.Unlock()
		return nil
	}
	if c.status.State != StatePrepared {
		state := c.status.State
		c.mu.Unlock()
		return fmt.Errorf("plugin process cannot activate from state %q", state)
	}
	c.negotiated = cloneHostServiceSet(c.activationServices)
	lifecycleVersion := c.lifecycleVersion
	c.mu.Unlock()
	if lifecycleVersion == RuntimeLifecycleVersion {
		activateCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
		defer cancel()
		if err := c.call(activateCtx, "activate", nil, nil); err != nil {
			c.fail(err)
			return err
		}
	}
	c.mu.Lock()
	c.status.State = StateActive
	c.status.StartedAt = time.Now().UTC()
	c.mu.Unlock()
	return nil
}

func (c *ProcessClient) Invoke(ctx context.Context, params InvokeParams) (InvokeResult, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	var result InvokeResult
	if err := c.call(callCtx, "hook.invoke", params, &result); err != nil {
		c.failFatalCall(err)
		return InvokeResult{}, err
	}
	return result, nil
}

func (c *ProcessClient) ExecuteTool(ctx context.Context, params ToolExecuteParams) (ToolExecuteResult, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	var result ToolExecuteResult
	if err := c.call(callCtx, "tool.execute", params, &result); err != nil {
		c.failFatalCall(err)
		return ToolExecuteResult{}, err
	}
	if err := result.Result.Validate(); err != nil {
		protocolErr := fmt.Errorf("validate tool result: %w", err)
		c.fail(protocolErr)
		return ToolExecuteResult{}, protocolErr
	}
	result.Result = result.Result.Clone()
	return result, nil
}

func (c *ProcessClient) Close(ctx context.Context) error {
	if c == nil {
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = c.call(shutdownCtx, "shutdown", nil, nil)
	err := c.stopProcess()
	c.mu.Lock()
	c.status.State = StateStopped
	c.tools = nil
	c.capabilities = nil
	c.mu.Unlock()
	return err
}

func (c *ProcessClient) call(ctx context.Context, method string, params, result any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.stopMu.Lock()
	stopped := c.stopped
	c.stopMu.Unlock()
	if stopped {
		return errors.New("plugin process is not running")
	}
	if c.cmd == nil || c.cmd.Process == nil {
		return errors.New("plugin process is not running")
	}
	id := fmt.Sprintf("%d", c.seq.Add(1))
	payload, err := json.Marshal(rpcRequest{ID: id, Method: method, Params: params})
	if err != nil {
		return err
	}
	responseCh := make(chan rpcResponse, 1)
	c.pendingMu.Lock()
	c.pending[id] = responseCh
	c.pendingMu.Unlock()
	defer c.removePending(id)
	if err := c.writeLine(payload); err != nil {
		return fmt.Errorf("write request: %w", err)
	}

	select {
	case response := <-responseCh:
		if response.Error != nil {
			message := strings.TrimSpace(response.Error.Message)
			if message == "" {
				message = "plugin call failed"
			}
			return &remoteCallError{code: strings.TrimSpace(response.Error.Code), message: message}
		}
		if result == nil {
			return nil
		}
		if len(response.Result) == 0 {
			return errors.New("response is missing result")
		}
		if err := json.Unmarshal(response.Result, result); err != nil {
			return fmt.Errorf("decode result: %w", err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.readerDone:
		return c.readFailure()
	}
}

func (c *ProcessClient) readLoop() {
	defer func() {
		c.stopMu.Lock()
		stopped := c.stopped
		c.stopMu.Unlock()
		if !stopped {
			err := c.readFailure()
			_ = c.stopProcess()
			c.setFailure(err)
		}
		close(c.readerDone)
	}()
	for c.scanner.Scan() {
		line := bytes.Clone(c.scanner.Bytes())
		kind, call, response, err := decodePluginMessage(line)
		if err != nil {
			if call.ID != "" {
				err = errors.Join(err, c.writeHostServiceResult(HostServiceResult{
					ID:    call.ID,
					Error: &HostServiceError{Code: "invalid_request", Message: err.Error()},
				}))
			}
			c.setReadFailure(err)
			return
		}
		if kind == pluginMessageHostCall {
			go func() {
				if err := c.dispatchHostService(c.processCtx, call); err != nil {
					c.setReadFailure(err)
					_ = c.stopProcess()
				}
			}()
			continue
		}
		c.pendingMu.Lock()
		responseCh := c.pending[response.ID]
		c.pendingMu.Unlock()
		if responseCh != nil {
			responseCh <- response
		}
	}
	err := c.scanner.Err()
	if err == nil {
		err = io.EOF
	}
	if errors.Is(err, bufio.ErrTooLong) {
		err = fmt.Errorf("plugin response line exceeds %d bytes", maxResponseLineSize)
	}
	c.setReadFailure(fmt.Errorf("read response: %w%s", err, c.stderrSuffix()))
}

func (c *ProcessClient) removePending(id string) {
	c.pendingMu.Lock()
	delete(c.pending, id)
	c.pendingMu.Unlock()
}

func (c *ProcessClient) setReadFailure(err error) {
	if err == nil {
		return
	}
	c.readerErrMu.Lock()
	if c.readerErr == nil {
		c.readerErr = err
	}
	c.readerErrMu.Unlock()
}

func (c *ProcessClient) readFailure() error {
	c.readerErrMu.Lock()
	err := c.readerErr
	c.readerErrMu.Unlock()
	if err == nil {
		return errors.New("plugin process transport closed")
	}
	return err
}

func (c *ProcessClient) writeLine(payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	payload = append(payload, '\n')
	if _, err := c.stdin.Write(payload); err != nil {
		return err
	}
	return nil
}

type pluginMessageKind int

const (
	pluginMessageResponse pluginMessageKind = iota
	pluginMessageHostCall
)

func configuredHostServices(config ProcessConfig) ([]HostServiceMethod, error) {
	if config.HostServiceHandler == nil {
		if len(config.SupportedHostServices) != 0 {
			return nil, errors.New("supported services require a live handler")
		}
		return nil, nil
	}
	declared := append([]HostServiceMethod(nil), config.HostServiceHandler.SupportedHostServices()...)
	if err := validateHostServiceSet(declared); err != nil {
		return nil, fmt.Errorf("handler declaration: %w", err)
	}
	if config.SupportedHostServices != nil {
		if err := validateHostServiceSet(config.SupportedHostServices); err != nil {
			return nil, fmt.Errorf("supported services assertion: %w", err)
		}
		if !sameHostServiceSet(declared, config.SupportedHostServices) {
			return nil, errors.New("supported services do not match handler declaration")
		}
	}
	return declared, nil
}

func validateHostServiceSet(services []HostServiceMethod) error {
	seen := make(map[HostServiceMethod]struct{}, len(services))
	for _, service := range services {
		if err := ValidateHostServiceMethod(service); err != nil {
			return err
		}
		if _, ok := seen[service]; ok {
			return fmt.Errorf("duplicate host service %q", service)
		}
		seen[service] = struct{}{}
	}
	return nil
}

func sameHostServiceSet(left, right []HostServiceMethod) bool {
	if len(left) != len(right) {
		return false
	}
	set := make(map[HostServiceMethod]struct{}, len(left))
	for _, service := range left {
		set[service] = struct{}{}
	}
	for _, service := range right {
		if _, ok := set[service]; !ok {
			return false
		}
	}
	return true
}

func negotiatedHostServices(required []HostServiceDescriptor, supported []HostServiceMethod) map[HostServiceMethod]struct{} {
	available := make(map[HostServiceMethod]struct{}, len(supported))
	for _, service := range supported {
		available[service] = struct{}{}
	}
	negotiated := make(map[HostServiceMethod]struct{}, len(required))
	for _, descriptor := range required {
		service := HostServiceMethod(strings.TrimSpace(descriptor.ID))
		if _, ok := available[service]; ok {
			negotiated[service] = struct{}{}
		}
	}
	return negotiated
}

func preflightHostServices(supported []HostServiceMethod) map[HostServiceMethod]struct{} {
	available := make(map[HostServiceMethod]struct{}, len(supported))
	for _, service := range supported {
		available[service] = struct{}{}
	}
	return preflightNegotiatedHostServices(available)
}

func preflightNegotiatedHostServices(negotiated map[HostServiceMethod]struct{}) map[HostServiceMethod]struct{} {
	readOnly := map[HostServiceMethod]struct{}{
		HostServiceStorageGet:   {},
		HostServiceStorageKeys:  {},
		HostServiceSettingsGet:  {},
		HostServiceSettingsList: {},
		HostServiceSessionList:  {},
	}
	out := make(map[HostServiceMethod]struct{})
	for service := range negotiated {
		if _, ok := readOnly[service]; ok {
			out[service] = struct{}{}
		}
	}
	return out
}

func cloneHostServiceSet(source map[HostServiceMethod]struct{}) map[HostServiceMethod]struct{} {
	out := make(map[HostServiceMethod]struct{}, len(source))
	for service := range source {
		out[service] = struct{}{}
	}
	return out
}

func decodePluginMessage(line []byte) (pluginMessageKind, HostServiceCall, rpcResponse, error) {
	var envelope struct {
		ID     json.RawMessage `json:"id"`
		Method json.RawMessage `json:"method"`
		Params json.RawMessage `json:"params"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(line), &envelope); err != nil {
		return 0, HostServiceCall{}, rpcResponse{}, fmt.Errorf("decode plugin message: %w", err)
	}
	var id string
	if len(envelope.ID) == 0 || json.Unmarshal(envelope.ID, &id) != nil || strings.TrimSpace(id) == "" {
		return 0, HostServiceCall{}, rpcResponse{}, errors.New("plugin message id must be a non-empty string")
	}
	if len(envelope.Method) != 0 {
		var method HostServiceMethod
		if json.Unmarshal(envelope.Method, &method) != nil || strings.TrimSpace(string(method)) == "" {
			return 0, HostServiceCall{ID: id}, rpcResponse{}, errors.New("host service method must be a non-empty string")
		}
		call := HostServiceCall{ID: id, Method: method}
		params := bytes.TrimSpace(envelope.Params)
		if len(params) == 0 || !json.Valid(params) || params[0] != '{' {
			return 0, call, rpcResponse{}, errors.New("host service params must be a JSON object")
		}
		if len(envelope.Result) != 0 || len(envelope.Error) != 0 {
			return 0, call, rpcResponse{}, errors.New("host service request cannot contain result or error")
		}
		call.Params = bytes.Clone(params)
		return pluginMessageHostCall, call, rpcResponse{}, nil
	}
	if len(envelope.Params) != 0 {
		return 0, HostServiceCall{}, rpcResponse{}, errors.New("plugin response cannot contain params")
	}
	if (len(envelope.Result) == 0) == (len(envelope.Error) == 0) {
		return 0, HostServiceCall{}, rpcResponse{}, errors.New("plugin response must contain exactly one of result or error")
	}
	var response rpcResponse
	if err := json.Unmarshal(line, &response); err != nil {
		return 0, HostServiceCall{}, rpcResponse{}, fmt.Errorf("decode response: %w", err)
	}
	return pluginMessageResponse, HostServiceCall{}, response, nil
}

func (c *ProcessClient) dispatchHostService(ctx context.Context, call HostServiceCall) error {
	if ValidateHostServiceMethod(call.Method) != nil {
		return c.rejectHostService(call.ID, "method_not_found", fmt.Sprintf("unknown host service %q", call.Method))
	}
	c.mu.RLock()
	_, negotiated := c.negotiated[call.Method]
	c.mu.RUnlock()
	if !negotiated {
		return c.rejectHostService(call.ID, "service_not_negotiated", fmt.Sprintf("host service %q was not negotiated", call.Method))
	}
	result, err := c.config.HostServiceHandler.HandleHostService(ctx, call.Method, bytes.Clone(call.Params))
	response := HostServiceResult{ID: call.ID}
	if err != nil {
		var serviceErr *HostServiceError
		if errors.As(err, &serviceErr) && serviceErr != nil {
			response.Error = &HostServiceError{Code: strings.TrimSpace(serviceErr.Code), Message: strings.TrimSpace(serviceErr.Message)}
		} else {
			response.Error = &HostServiceError{Code: "handler_error", Message: strings.TrimSpace(err.Error())}
		}
		if response.Error.Code == "" {
			response.Error.Code = "handler_error"
		}
		if response.Error.Message == "" {
			response.Error.Message = "host service handler failed"
		}
	} else {
		if len(result) == 0 || !json.Valid(result) {
			response.Error = &HostServiceError{Code: "invalid_handler_result", Message: "host service handler returned invalid JSON"}
		} else {
			response.Result = bytes.Clone(result)
		}
	}
	return c.writeHostServiceResult(response)
}

func (c *ProcessClient) rejectHostService(id, code, message string) error {
	if err := c.writeHostServiceResult(HostServiceResult{ID: id, Error: &HostServiceError{Code: code, Message: message}}); err != nil {
		return err
	}
	return errors.New(message)
}

func (c *ProcessClient) writeHostServiceResult(response HostServiceResult) error {
	payload, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("encode host service result: %w", err)
	}
	if len(payload)+1 > maxResponseLineSize {
		return fmt.Errorf("host service result line exceeds %d bytes", maxResponseLineSize)
	}
	if err := c.writeLine(payload); err != nil {
		return fmt.Errorf("write host service result: %w", err)
	}
	return nil
}

func (c *ProcessClient) stopProcess() error {
	if c == nil {
		return nil
	}
	c.serviceClose.Do(func() {
		if lifecycle, ok := c.config.HostServiceHandler.(HostServiceLifecycle); ok {
			lifecycle.CloseHostServices()
		}
	})
	c.stopMu.Lock()
	defer c.stopMu.Unlock()
	if c.stopped {
		return c.stopErr
	}
	c.stopped = true
	if c.processCancel != nil {
		c.processCancel()
	}
	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	_ = c.stdin.Close()
	_ = c.cmd.Process.Kill()
	err := c.cmd.Wait()
	if err != nil && !strings.Contains(err.Error(), "signal: killed") {
		c.stopErr = err
	}
	return c.stopErr
}

func (c *ProcessClient) setFailure(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status.State = StateFailed
	c.status.Error = err.Error()
	c.status.Hooks = nil
	c.tools = nil
	c.capabilities = nil
}

func (c *ProcessClient) fail(err error) {
	_ = c.stopProcess()
	c.setFailure(err)
}

func (c *ProcessClient) failFatalCall(err error) {
	if err == nil {
		return
	}
	var domainErr *remoteCallError
	if errors.As(err, &domainErr) {
		return
	}
	c.fail(err)
}

func (c *ProcessClient) stderrSuffix() string {
	value := strings.TrimSpace(c.stderr.String())
	if value == "" {
		return ""
	}
	return ": " + value
}

// buildEnv returns a minimal, deterministic environment for a plugin process.
//
// Only a documented cross-platform baseline is inherited from the parent
// process (PATH, HOME, temp directories, locale variables, and Windows launch
// essentials). The full parent os.Environ is intentionally not inherited, so
// ambient API keys, tokens, and other secrets cannot leak into plugins. Values
// supplied in ProcessConfig.Env are overlaid on the baseline and always take
// precedence. The resulting slice is sorted by key for deterministic ordering.
func buildEnv(explicit map[string]string) []string {
	values := make(map[string]string, len(baselineEnvKeys)+len(explicit))
	for _, key := range baselineEnvKeys {
		if v := os.Getenv(key); v != "" {
			values[key] = v
		}
	}
	for key, value := range explicit {
		values[key] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+values[key])
	}
	return out
}

// baselineEnvKeys lists the small set of parent variables that plugins need to
// start command-line binaries and resolve basic paths. It intentionally omits
// credential- and tool-specific variables.
var baselineEnvKeys = func() []string {
	keys := []string{
		"PATH",
		"HOME",
		"USER",
		"LOGNAME",
		"SHELL",
		"TMPDIR",
		"TMP",
		"TEMP",
		"LANG",
		"LC_ALL",
		"LC_CTYPE",
		"LC_MESSAGES",
		"LC_NUMERIC",
		"LC_TIME",
		"LC_COLLATE",
		"LC_MONETARY",
		"LC_PAPER",
		"LC_NAME",
		"LC_ADDRESS",
		"LC_TELEPHONE",
		"LC_MEASUREMENT",
		"LC_IDENTIFICATION",
	}
	if runtime.GOOS == "windows" {
		keys = append(keys,
			"SYSTEMROOT",
			"SYSTEMDRIVE",
			"USERPROFILE",
			"APPDATA",
			"LOCALAPPDATA",
			"ProgramFiles",
			"ProgramFiles(x86)",
			"ProgramData",
			"CommonProgramFiles",
			"CommonProgramFiles(x86)",
			"PROCESSOR_ARCHITECTURE",
			"PROCESSOR_IDENTIFIER",
			"COMPUTERNAME",
			"USERNAME",
		)
	}
	return keys
}()

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}
