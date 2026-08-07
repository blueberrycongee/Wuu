package pluginhost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/toolresult"
)

// Client is one initialized plugin runtime. Implementations may host plugins
// in subprocesses or in-process tests, but share the same typed wire contract.
type Client interface {
	ID() string
	Hooks() []Hook
	Invoke(context.Context, InvokeParams) (InvokeResult, error)
	Status() Status
	Close(context.Context) error
}

// ToolClient extends a plugin client with its initialized tool registrations
// and the typed execution method for those tools.
type ToolClient interface {
	Client
	Tools() []ToolRegistration
	ExecuteTool(context.Context, ToolExecuteParams) (ToolExecuteResult, error)
}

// CapabilityClient is a protocol-v2 client with negotiated capability calls.
type CapabilityClient interface {
	Client
	ProtocolVersion() int
	Capabilities() []CapabilityDescriptor
	InvokeCapability(context.Context, CapabilityInvokeParams) (CapabilityInvokeResult, error)
}

// RegisteredCapability is the host-owned view of one negotiated capability.
type RegisteredCapability struct {
	PluginID   string
	Descriptor CapabilityDescriptor
	client     CapabilityClient
}

// RegisteredTool is the host-owned public view of a plugin tool registration.
type RegisteredTool struct {
	PublicName   string
	PluginID     string
	Registration ToolRegistration
	client       ToolClient
}

// Host invokes plugins in discovery order. A hook receives the output from the
// previous plugin, preserving one deterministic Wuu transform chain.
type Host struct {
	mu           sync.RWMutex
	clients      []Client
	tools        map[string]RegisteredTool
	toolOrder    []string
	capabilities []RegisteredCapability
}

func New(clients ...Client) *Host {
	host := &Host{tools: make(map[string]RegisteredTool)}
	for _, client := range clients {
		host.addLocked(client)
	}
	return host
}

// Failed preserves a plugin load failure in the runtime inventory without
// registering any interception points.
func Failed(id string, err error) Client {
	message := "plugin failed to start"
	if err != nil {
		message = err.Error()
	}
	return &failedClient{status: Status{ID: id, State: StateFailed, Error: message}}
}

func (h *Host) Add(client Client) {
	if h == nil || client == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.addLocked(client)
}

func (h *Host) addLocked(client Client) {
	if client == nil {
		return
	}
	h.clients = append(h.clients, client)
	if capabilityClient, ok := client.(CapabilityClient); ok && capabilityClient.ProtocolVersion() >= CapabilityProtocolVersion {
		for _, descriptor := range capabilityClient.Capabilities() {
			h.capabilities = append(h.capabilities, RegisteredCapability{
				PluginID: client.ID(), Descriptor: descriptor, client: capabilityClient,
			})
		}
	}
	toolClient, ok := client.(ToolClient)
	if !ok {
		return
	}
	registrations := toolClient.Tools()
	if validateToolRegistrations(registrations) != nil {
		// Process clients reject invalid declarations before activation. Keeping
		// this guard makes in-process clients fail closed without affecting other
		// initialized plugins.
		return
	}
	if h.tools == nil {
		h.tools = make(map[string]RegisteredTool)
	}
	for _, registration := range registrations {
		publicName := h.availablePublicToolName(client.ID(), registration.ID)
		h.tools[publicName] = RegisteredTool{
			PublicName:   publicName,
			PluginID:     client.ID(),
			Registration: cloneToolRegistration(registration),
			client:       toolClient,
		}
		h.toolOrder = append(h.toolOrder, publicName)
	}
}

// ValidateCapabilities verifies generation-wide dependencies and conflicts.
func (h *Host) ValidateCapabilities() error {
	if h == nil {
		return nil
	}
	h.mu.RLock()
	registered := append([]RegisteredCapability(nil), h.capabilities...)
	h.mu.RUnlock()
	present := make(map[string]struct{}, len(registered))
	for _, capability := range registered {
		present[capability.Descriptor.ID] = struct{}{}
	}
	for _, capability := range registered {
		for _, dependency := range capability.Descriptor.DependsOn {
			if _, ok := present[dependency]; !ok {
				return fmt.Errorf("plugin %q capability %q requires missing capability %q", capability.PluginID, capability.Descriptor.ID, dependency)
			}
		}
		for _, conflict := range capability.Descriptor.Conflicts {
			if _, ok := present[conflict]; ok {
				return fmt.Errorf("plugin %q capability %q conflicts with capability %q", capability.PluginID, capability.Descriptor.ID, conflict)
			}
		}
	}
	return nil
}

// Capabilities returns active negotiated capabilities in deterministic
// priority order. Ties preserve plugin discovery order.
func (h *Host) Capabilities(id string) []RegisteredCapability {
	if h == nil {
		return nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []RegisteredCapability
	for _, capability := range h.capabilities {
		if capability.Descriptor.ID != id || capability.client.Status().State != StateActive {
			continue
		}
		copy := capability
		copy.Descriptor = cloneCapabilityDescriptors([]CapabilityDescriptor{capability.Descriptor})[0]
		out = append(out, copy)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Descriptor.Priority > out[j].Descriptor.Priority
	})
	return out
}

// Capability returns one active capability owned by the exact plugin.
func (h *Host) Capability(pluginID, id string) (RegisteredCapability, bool) {
	if h == nil {
		return RegisteredCapability{}, false
	}
	pluginID, id = strings.TrimSpace(pluginID), strings.TrimSpace(id)
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, capability := range h.capabilities {
		if capability.PluginID == pluginID && capability.Descriptor.ID == id && capability.client.Status().State == StateActive {
			return capability, true
		}
	}
	return RegisteredCapability{}, false
}

// InvokeCapability invokes one exact plugin registration.
func (h *Host) InvokeCapability(ctx context.Context, capability RegisteredCapability, input, output any) error {
	if capability.client == nil || capability.client.Status().State != StateActive {
		return fmt.Errorf("plugin %q capability %q is not active", capability.PluginID, capability.Descriptor.ID)
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("marshal capability %q input: %w", capability.Descriptor.ID, err)
	}
	outputJSON, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("marshal capability %q output: %w", capability.Descriptor.ID, err)
	}
	result, err := capability.client.InvokeCapability(ctx, CapabilityInvokeParams{
		Capability: capability.Descriptor.ID,
		Input:      inputJSON,
		Output:     outputJSON,
	})
	if err != nil {
		return fmt.Errorf("plugin %q capability %q: %w", capability.PluginID, capability.Descriptor.ID, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(result.Output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return fmt.Errorf("plugin %q capability %q returned invalid output: %w", capability.PluginID, capability.Descriptor.ID, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return fmt.Errorf("plugin %q capability %q returned invalid output: %w", capability.PluginID, capability.Descriptor.ID, err)
	}
	return nil
}

// HandleCapabilityError applies the negotiated descriptor policy to one
// capability failure. A nil return means the caller must skip this
// contribution and continue dispatch with its previous value.
func (h *Host) HandleCapabilityError(capability RegisteredCapability, err error) error {
	if err == nil {
		return nil
	}
	return handlePluginError(capability.PluginID, capability.Descriptor.ID, EffectiveErrorPolicy(capability.Descriptor), err)
}

func handlePluginError(pluginID, contribution string, policy ErrorPolicy, err error) error {
	switch policy {
	case ErrorPolicyIsolate:
		providers.DebugLogf("plugin %q contribution %q isolated after failure: %v", pluginID, contribution, err)
		return nil
	case ErrorPolicyIgnore:
		return nil
	default:
		return err
	}
}

// ToolDefinitions returns plugin tools in deterministic client and declaration
// order, ready to append to the executor definition surface sent to providers.
func (h *Host) ToolDefinitions() []providers.ToolDefinition {
	if h == nil {
		return nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	definitions := make([]providers.ToolDefinition, 0, len(h.toolOrder))
	for _, name := range h.toolOrder {
		tool := h.tools[name]
		if tool.client.Status().State != StateActive {
			continue
		}
		definitions = append(definitions, providers.ToolDefinition{
			Name:        name,
			Description: tool.Registration.Description,
			InputSchema: cloneSchema(tool.Registration.InputSchema),
		})
	}
	return definitions
}

// Tool returns a copy of the registration behind a public tool name.
func (h *Host) Tool(name string) (RegisteredTool, bool) {
	if h == nil {
		return RegisteredTool{}, false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	tool, ok := h.tools[name]
	if !ok || tool.client.Status().State != StateActive {
		return RegisteredTool{}, false
	}
	tool.Registration = cloneToolRegistration(tool.Registration)
	return tool, true
}

func (h *Host) SupportsTool(name string) bool {
	_, ok := h.Tool(name)
	return ok
}

// ExecuteTool routes a public tool call to its owning plugin and validates the
// structured result before returning it to the runtime.
func (h *Host) ExecuteTool(ctx context.Context, name string, input ToolExecuteInput) (toolresult.Result, error) {
	tool, ok := h.Tool(name)
	if !ok {
		return toolresult.Result{}, fmt.Errorf("plugin tool %q is not registered", name)
	}
	input.Tool = name
	response, err := tool.client.ExecuteTool(ctx, ToolExecuteParams{
		ToolExecuteInput: input,
		ToolID:           tool.Registration.ID,
	})
	if err != nil {
		return toolresult.Result{}, fmt.Errorf("plugin %q tool %q: %w", tool.PluginID, name, err)
	}
	if err := response.Result.Validate(); err != nil {
		return toolresult.Result{}, fmt.Errorf("plugin %q tool %q returned invalid result: %w", tool.PluginID, name, err)
	}
	return response.Result.Clone(), nil
}

// Run applies every plugin registered for hook to output. Output must be a
// non-nil pointer so each successful transform can become the next input.
func (h *Host) Run(ctx context.Context, hook Hook, input, output any) error {
	if h == nil {
		return nil
	}
	if !IsValidHook(hook) {
		return fmt.Errorf("unknown plugin hook %q", hook)
	}
	if output == nil {
		return fmt.Errorf("plugin hook %q requires output", hook)
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("marshal plugin hook %q input: %w", hook, err)
	}
	outputJSON, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("marshal plugin hook %q output: %w", hook, err)
	}

	h.mu.RLock()
	clients := append([]Client(nil), h.clients...)
	h.mu.RUnlock()
	for _, client := range clients {
		if !hasHook(client.Hooks(), hook) {
			continue
		}
		result, invokeErr := client.Invoke(ctx, InvokeParams{
			Hook:   hook,
			Input:  inputJSON,
			Output: outputJSON,
		})
		if invokeErr != nil {
			return fmt.Errorf("plugin %q hook %q: %w", client.ID(), hook, invokeErr)
		}
		if len(result.Output) == 0 {
			return fmt.Errorf("plugin %q hook %q returned empty output", client.ID(), hook)
		}
		if err := json.Unmarshal(result.Output, output); err != nil {
			return fmt.Errorf("plugin %q hook %q returned invalid output: %w", client.ID(), hook, err)
		}
		outputJSON = append(outputJSON[:0], result.Output...)
	}
	return nil
}

func (h *Host) Statuses() []Status {
	if h == nil {
		return nil
	}
	h.mu.RLock()
	clients := append([]Client(nil), h.clients...)
	h.mu.RUnlock()
	out := make([]Status, 0, len(clients))
	for _, client := range clients {
		out = append(out, client.Status())
	}
	return out
}

func (h *Host) HasHook(target Hook) bool {
	if h == nil {
		return false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, client := range h.clients {
		if hasHook(client.Hooks(), target) {
			return true
		}
	}
	return false
}

func (h *Host) Close(ctx context.Context) error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	clients := append([]Client(nil), h.clients...)
	h.clients = nil
	h.tools = nil
	h.toolOrder = nil
	h.capabilities = nil
	h.mu.Unlock()

	var firstErr error
	for i := len(clients) - 1; i >= 0; i-- {
		if err := clients[i].Close(ctx); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close plugin %q: %w", clients[i].ID(), err)
		}
	}
	return firstErr
}

func (h *Host) availablePublicToolName(pluginID, localID string) string {
	for attempt := 0; ; attempt++ {
		name := publicToolName(pluginID, localID, attempt)
		if _, exists := h.tools[name]; !exists {
			return name
		}
	}
}

func publicToolName(pluginID, localID string, attempt int) string {
	pluginPart := toolNamePart(pluginID)
	localPart := toolNamePart(localID)
	seed := fmt.Sprintf("%s\x00%s\x00%d", pluginID, localID, attempt)
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(seed)))[:16]
	return "plugin_" + pluginPart + "_" + localPart + "_" + digest
}

func toolNamePart(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if r <= unicode.MaxASCII {
				b.WriteRune(r)
			} else {
				b.WriteByte('_')
			}
		} else {
			b.WriteByte('_')
		}
		if b.Len() >= 19 {
			break
		}
	}
	part := strings.Trim(b.String(), "_")
	if part == "" {
		return "unnamed"
	}
	return part
}

func cloneToolRegistration(registration ToolRegistration) ToolRegistration {
	clone := registration
	clone.InputSchema = cloneSchema(registration.InputSchema)
	clone.ExecutionScopes = append([]string(nil), registration.ExecutionScopes...)
	if registration.Activity != nil {
		activity := *registration.Activity
		clone.Activity = &activity
	}
	if registration.Display != nil {
		display := *registration.Display
		clone.Display = &display
	}
	return clone
}

func cloneSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil
	}
	var clone map[string]any
	if err := json.Unmarshal(raw, &clone); err != nil {
		return nil
	}
	return clone
}

func hasHook(hooks []Hook, target Hook) bool {
	for _, hook := range hooks {
		if hook == target {
			return true
		}
	}
	return false
}

type failedClient struct{ status Status }

func (c *failedClient) ID() string    { return c.status.ID }
func (c *failedClient) Hooks() []Hook { return nil }
func (c *failedClient) Invoke(context.Context, InvokeParams) (InvokeResult, error) {
	return InvokeResult{}, errors.New(c.status.Error)
}
func (c *failedClient) Status() Status              { return c.status }
func (c *failedClient) Close(context.Context) error { return nil }
