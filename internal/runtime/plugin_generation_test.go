package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/config"
	"github.com/blueberrycongee/wuu/internal/hooks"
	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/session"
)

type generationClient struct {
	id           string
	closed       bool
	closeOrder   *[]string
	capabilities []pluginhost.CapabilityDescriptor
}

func (c *generationClient) ID() string               { return c.id }
func (c *generationClient) Hooks() []pluginhost.Hook { return nil }
func (c *generationClient) Invoke(context.Context, pluginhost.InvokeParams) (pluginhost.InvokeResult, error) {
	return pluginhost.InvokeResult{}, nil
}
func (c *generationClient) Status() pluginhost.Status {
	return pluginhost.Status{ID: c.id, State: pluginhost.StateActive}
}
func (c *generationClient) Close(context.Context) error {
	c.closed = true
	if c.closeOrder != nil {
		*c.closeOrder = append(*c.closeOrder, c.id)
	}
	return nil
}

func (c *generationClient) ProtocolVersion() int { return pluginhost.CapabilityProtocolVersion }
func (c *generationClient) Capabilities() []pluginhost.CapabilityDescriptor {
	return append([]pluginhost.CapabilityDescriptor(nil), c.capabilities...)
}
func (c *generationClient) InvokeCapability(context.Context, pluginhost.CapabilityInvokeParams) (pluginhost.CapabilityInvokeResult, error) {
	return pluginhost.CapabilityInvokeResult{}, nil
}

func TestFailedCandidateClosesStartedProcessesAndPreservesOldGeneration(t *testing.T) {
	oldClient := &generationClient{id: "old"}
	old := testPluginGeneration("old", oldClient)
	session := testGenerationSession(old)

	var closeOrder []string
	first := &generationClient{id: "first", closeOrder: &closeOrder}
	second := &generationClient{id: "second", closeOrder: &closeOrder}
	plugins := []pluginpkg.Plugin{
		testRuntimePlugin("first"),
		testRuntimePlugin("second"),
		testRuntimePlugin("target"),
	}
	_, err := session.buildPluginGeneration(config.Config{}, plugins, map[string]bool{"target": true}, nil, func(_ context.Context, cfg pluginhost.ProcessConfig) (pluginhost.Client, error) {
		switch cfg.ID {
		case "first":
			return first, nil
		case "second":
			return second, nil
		default:
			return nil, errors.New("candidate initialization failed")
		}
	})
	if err == nil {
		t.Fatal("candidate generation unexpectedly succeeded")
	}
	if oldClient.closed || session.PluginHost != old.host || session.ActivePlugins[0].ID != "old" {
		t.Fatalf("old generation changed after failed candidate: closed=%v active=%+v", oldClient.closed, session.ActivePlugins)
	}
	if !first.closed || !second.closed {
		t.Fatalf("candidate processes were not closed: first=%v second=%v", first.closed, second.closed)
	}
	if len(closeOrder) != 2 || closeOrder[0] != "second" || closeOrder[1] != "first" {
		t.Fatalf("candidate close order = %v, want [second first]", closeOrder)
	}
}

func TestCapabilityConflictClosesCandidateAndPreservesOldGeneration(t *testing.T) {
	oldClient := &generationClient{id: "old"}
	old := testPluginGeneration("old", oldClient)
	session := testGenerationSession(old)
	one := &generationClient{id: "one", capabilities: []pluginhost.CapabilityDescriptor{{
		ID: "agent.capability.one", Kind: "transform", Version: 1, Conflicts: []string{"agent.capability.two"},
	}}}
	two := &generationClient{id: "two", capabilities: []pluginhost.CapabilityDescriptor{{
		ID: "agent.capability.two", Kind: "observe", Version: 1,
	}}}
	_, err := session.buildPluginGeneration(config.Config{}, []pluginpkg.Plugin{
		testRuntimePlugin("one"), testRuntimePlugin("two"),
	}, nil, nil, func(_ context.Context, cfg pluginhost.ProcessConfig) (pluginhost.Client, error) {
		if cfg.ID == "one" {
			return one, nil
		}
		return two, nil
	})
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("error = %v", err)
	}
	if !one.closed || !two.closed {
		t.Fatalf("candidate not closed: one=%v two=%v", one.closed, two.closed)
	}
	if oldClient.closed || session.PluginHost != old.host || session.ActivePlugins[0].ID != "old" {
		t.Fatal("old generation was polluted by rejected capabilities")
	}
}

func TestSuccessfulCandidateSwapsThenClosesOldGeneration(t *testing.T) {
	oldClient := &generationClient{id: "old"}
	old := testPluginGeneration("old", oldClient)
	session := testGenerationSession(old)
	candidateClient := &generationClient{id: "candidate"}
	candidate := testPluginGeneration("candidate", candidateClient)

	if err := session.ActivatePluginGeneration(candidate, nil); err != nil {
		t.Fatal(err)
	}
	if session.PluginHost != candidate.host || len(session.ActivePlugins) != 1 || session.ActivePlugins[0].ID != "candidate" {
		t.Fatalf("candidate was not installed: host=%p active=%+v", session.PluginHost, session.ActivePlugins)
	}
	if !oldClient.closed {
		t.Fatal("old generation was not closed after the swap")
	}
	if candidateClient.closed {
		t.Fatal("active candidate was closed")
	}
}

func TestCandidateCommitFailureRollsBackAllLiveSurfaces(t *testing.T) {
	oldClient := &generationClient{id: "old"}
	old := testPluginGeneration("old", oldClient)
	old.hooks = hooks.NewDispatcher(hooks.NewRegistry(map[hooks.Event][]hooks.HookConfig{
		hooks.PreToolUse: {{Command: "old-hook"}},
	}))
	session := testGenerationSession(old)
	liveHooks := hooks.NewDispatcher(nil)
	liveHooks.Replace(old.hooks)
	session.HookDispatcher = liveHooks

	candidateClient := &generationClient{id: "candidate"}
	candidate := testPluginGeneration("candidate", candidateClient)
	candidate.hooks = hooks.NewDispatcher(hooks.NewRegistry(map[hooks.Event][]hooks.HookConfig{
		hooks.PostToolUse: {{Command: "candidate-hook"}},
	}))

	err := session.ActivatePluginGeneration(candidate, func() error { return errors.New("publish failed") })
	if err == nil {
		t.Fatal("activation unexpectedly succeeded")
	}
	if session.PluginHost != old.host || session.ActivePlugins[0].ID != "old" || oldClient.closed {
		t.Fatalf("old generation was not restored: closed=%v active=%+v", oldClient.closed, session.ActivePlugins)
	}
	if !session.HookDispatcher.HasHooks(hooks.PreToolUse) || session.HookDispatcher.HasHooks(hooks.PostToolUse) {
		t.Fatal("old hook registry was not restored after rollback")
	}
	if !candidateClient.closed {
		t.Fatal("rolled-back candidate was not closed")
	}
}

func TestClosedGenerationReleasesCapabilityRegistry(t *testing.T) {
	client := &runtimeCapabilityClient{id: "capability"}
	host := pluginhost.New(client)
	generation := &PluginGeneration{
		host:              host,
		requestTransforms: buildPluginRequestTransforms(host, "openai", "", "/workspace"),
	}
	if generation.requestTransforms.Count() != 1 {
		t.Fatalf("transform count = %d", generation.requestTransforms.Count())
	}
	if err := generation.close(); err != nil {
		t.Fatal(err)
	}
	if generation.host != nil || generation.requestTransforms != nil {
		t.Fatal("closed generation retained capability state")
	}
	if capabilities := host.Capabilities(pluginhost.CapabilityAgentRequestTransform); len(capabilities) != 0 {
		t.Fatalf("closed host capabilities = %+v", capabilities)
	}
}

func TestRequiredCandidateMCPServerMustBeReady(t *testing.T) {
	manager, err := startMCPManager(config.Config{}, []pluginpkg.Plugin{{
		Manifest: pluginpkg.Manifest{
			ID: "candidate",
			MCPServers: map[string]config.MCPServerConfig{
				"broken": {Command: "/definitely/not/a/wuu-mcp-server"},
			},
		},
	}}, map[string]bool{"candidate": true})
	if err == nil {
		if manager != nil {
			_ = manager.Close()
		}
		t.Fatal("required MCP startup unexpectedly succeeded")
	}
	if manager != nil {
		t.Fatal("failed required MCP startup returned a live manager")
	}
}

func TestNewSessionDoesNotDiscoverPluginsDuringMutation(t *testing.T) {
	homeDir := t.TempDir()
	wuuHome := filepath.Join(homeDir, ".wuu")
	mutation, acquired, err := session.TryAcquirePluginGenerationMutationLease(wuuHome)
	if err != nil || !acquired {
		t.Fatalf("acquire mutation generation: acquired=%v err=%v", acquired, err)
	}
	defer mutation.Release()

	_, err = NewSession(Options{RootDir: t.TempDir(), HomeDir: homeDir})
	if err == nil || !strings.Contains(err.Error(), "plugin packages are being changed") {
		t.Fatalf("NewSession error = %v, want active plugin mutation", err)
	}
}

func testGenerationSession(generation *PluginGeneration) *Session {
	return &Session{
		PluginHost:       generation.host,
		HookDispatcher:   hooks.NewDispatcher(nil),
		Plugins:          append([]pluginpkg.Plugin(nil), generation.plugins...),
		ActivePlugins:    append([]pluginpkg.Plugin(nil), generation.active...),
		pluginGeneration: generation,
	}
}

func testPluginGeneration(id string, client pluginhost.Client) *PluginGeneration {
	plugin := pluginpkg.Plugin{Manifest: pluginpkg.Manifest{ID: id}}
	return &PluginGeneration{
		plugins: []pluginpkg.Plugin{plugin},
		active:  []pluginpkg.Plugin{plugin},
		host:    pluginhost.New(client),
		hooks:   hooks.NewDispatcher(nil),
	}
}

func testRuntimePlugin(id string) pluginpkg.Plugin {
	return pluginpkg.Plugin{
		Manifest: pluginpkg.Manifest{
			ID: id,
			Runtime: &pluginpkg.RuntimeSpec{
				Protocol: pluginhost.ProtocolName,
				Command:  id,
			},
		},
		Official: true,
	}
}
