package agent

import (
	"context"
	"testing"

	"github.com/blueberrycongee/wuu/internal/providers"
)

func TestSystemPromptAssemblerEmpty(t *testing.T) {
	a := NewSystemPromptAssembler()
	prompt, sections := a.Assemble("")
	if prompt != "" {
		t.Errorf("expected empty prompt, got %q", prompt)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections, got %d", len(sections))
	}
}

func TestSystemPromptAssemblerBaseOnly(t *testing.T) {
	a := NewSystemPromptAssembler()
	prompt, sections := a.Assemble("You are a helpful assistant.")
	if prompt != "You are a helpful assistant." {
		t.Errorf("unexpected prompt: %q", prompt)
	}
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0].Key != "host.base" {
		t.Errorf("expected key host.base, got %s", sections[0].Key)
	}
	if !sections[0].Static {
		t.Error("expected static base section")
	}
}

func TestSystemPromptAssemblerWithProviders(t *testing.T) {
	a := NewSystemPromptAssembler()
	a.Add(NewStaticPromptSection("plugin-a.rules", "Rule: always format code.", 10))
	a.Add(NewStaticPromptSection("plugin-b.context", "Context: this is a Go project.", 5))

	_, sections := a.Assemble("You are a helpful assistant.")
	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}

	// Sections should be: host.base, plugin-a.rules (pri 10), plugin-b.context (pri 5)
	if sections[0].Key != "host.base" {
		t.Errorf("expected host.base first, got %s", sections[0].Key)
	}
	if sections[1].Key != "plugin-a.rules" {
		t.Errorf("expected plugin-a.rules second, got %s", sections[1].Key)
	}
	if sections[2].Key != "plugin-b.context" {
		t.Errorf("expected plugin-b.context third, got %s", sections[2].Key)
	}

	if !sections[1].Static {
		t.Error("plugin section should be static")
	}
}

func TestSystemPromptAssemblerPriorityOrdering(t *testing.T) {
	a := NewSystemPromptAssembler()
	a.Add(NewStaticPromptSection("low", "low priority", 1))
	a.Add(NewStaticPromptSection("high", "high priority", 100))
	a.Add(NewStaticPromptSection("mid", "mid priority", 50))

	_, sections := a.Assemble("")
	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}
	if sections[0].Key != "high" {
		t.Errorf("expected high first, got %s", sections[0].Key)
	}
	if sections[1].Key != "mid" {
		t.Errorf("expected mid second, got %s", sections[1].Key)
	}
	if sections[2].Key != "low" {
		t.Errorf("expected low third, got %s", sections[2].Key)
	}
}

func TestSystemPromptAssemblerRemove(t *testing.T) {
	a := NewSystemPromptAssembler()
	a.Add(NewStaticPromptSection("keep", "keep me", 1))
	a.Add(NewStaticPromptSection("remove", "remove me", 2))
	a.Remove("remove")

	_, sections := a.Assemble("")
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0].Key != "keep" {
		t.Errorf("expected keep, got %s", sections[0].Key)
	}
}

func TestSystemPromptAssemblerClear(t *testing.T) {
	a := NewSystemPromptAssembler()
	a.Add(NewStaticPromptSection("s1", "section 1", 1))
	a.Clear()

	if a.ProviderCount() != 0 {
		t.Errorf("expected 0 providers after clear, got %d", a.ProviderCount())
	}
}

func TestSystemPromptAssemblerDuplicateKey(t *testing.T) {
	a := NewSystemPromptAssembler()
	a.Add(NewStaticPromptSection("key", "first", 1))
	a.Add(NewStaticPromptSection("key", "second", 2))

	if a.ProviderCount() != 1 {
		t.Errorf("expected 1 provider after duplicate key, got %d", a.ProviderCount())
	}

	_, sections := a.Assemble("")
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0].Bytes != len("second") {
		t.Errorf("expected second section (last registered), got %q", sections[0].Key)
	}
}

func TestSystemPromptAssemblerEmptySectionSkipped(t *testing.T) {
	a := NewSystemPromptAssembler()
	a.Add(NewStaticPromptSection("empty", "", 1))
	a.Add(NewStaticPromptSection("valid", "valid content", 1))

	_, sections := a.Assemble("")
	if len(sections) != 1 {
		t.Fatalf("expected 1 section (empty skipped), got %d", len(sections))
	}
	if sections[0].Key != "valid" {
		t.Errorf("expected valid, got %s", sections[0].Key)
	}
}

func TestRequestTransformChainEmpty(t *testing.T) {
	c := NewRequestTransformChain()
	req := &providers.ChatRequest{Model: "test-model"}
	err := c.Apply(context.Background(), req, nil)
	if err != nil {
		t.Errorf("empty chain should not error: %v", err)
	}
}

func TestRequestTransformChainOrdering(t *testing.T) {
	c := NewRequestTransformChain()
	var order []string

	c.Add(NewRequestTransform("mid", func(ctx context.Context, req *providers.ChatRequest) error {
		order = append(order, "mid")
		return nil
	}, 50))
	c.Add(NewRequestTransform("high", func(ctx context.Context, req *providers.ChatRequest) error {
		order = append(order, "high")
		return nil
	}, 100))
	c.Add(NewRequestTransform("low", func(ctx context.Context, req *providers.ChatRequest) error {
		order = append(order, "low")
		return nil
	}, 1))

	req := &providers.ChatRequest{Model: "test"}
	err := c.Apply(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(order) != 3 {
		t.Fatalf("expected 3 transforms, got %d", len(order))
	}
	if order[0] != "high" || order[1] != "mid" || order[2] != "low" {
		t.Errorf("expected [high mid low], got %v", order)
	}
}

func TestRequestTransformChainModifiesRequest(t *testing.T) {
	c := NewRequestTransformChain()
	c.Add(NewRequestTransform("set-temp", func(ctx context.Context, req *providers.ChatRequest) error {
		req.Temperature = 0.7
		return nil
	}, 10))

	req := &providers.ChatRequest{Model: "test", Temperature: 0.0}
	err := c.Apply(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7, got %f", req.Temperature)
	}
}

func TestRequestTransformChainLegacyBeforeFn(t *testing.T) {
	c := NewRequestTransformChain()
	var legacyCalled bool

	req := &providers.ChatRequest{Model: "test"}
	err := c.Apply(context.Background(), req, func(ctx context.Context, req *providers.ChatRequest) error {
		legacyCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !legacyCalled {
		t.Error("legacy BeforeRequest should be called")
	}
}

func TestRequestTransformChainErrorStops(t *testing.T) {
	c := NewRequestTransformChain()
	var secondCalled bool

	c.Add(NewRequestTransform("first", func(ctx context.Context, req *providers.ChatRequest) error {
		return context.Canceled
	}, 10))
	c.Add(NewRequestTransform("second", func(ctx context.Context, req *providers.ChatRequest) error {
		secondCalled = true
		return nil
	}, 5))

	req := &providers.ChatRequest{Model: "test"}
	err := c.Apply(context.Background(), req, nil)
	if err == nil {
		t.Fatal("expected error from first transform")
	}
	if secondCalled {
		t.Error("second transform should not be called after error")
	}
}

func TestCompactionRegistryResolve(t *testing.T) {
	r := NewCompactionRegistry()

	// Empty registry returns nil.
	if r.Resolve(nil) != nil {
		t.Error("expected nil from empty registry")
	}

	// Register providers.
	r.Register(&mockCompactionProvider{key: "default", priority: 1})
	r.Register(&mockCompactionProvider{key: "custom", priority: 10})

	resolved := r.Resolve(nil)
	if resolved == nil {
		t.Fatal("expected resolved provider")
	}
	if resolved.CompactionKey() != "custom" {
		t.Errorf("expected custom (higher priority), got %s", resolved.CompactionKey())
	}

	if r.Count() != 2 {
		t.Errorf("expected 2 providers, got %d", r.Count())
	}
}

func TestCompactionRegistryFallback(t *testing.T) {
	r := NewCompactionRegistry()
	fallback := &mockCompactionProvider{key: "fallback", priority: 0}

	resolved := r.Resolve(fallback)
	if resolved == nil {
		t.Fatal("expected fallback provider")
	}
	if resolved.CompactionKey() != "fallback" {
		t.Errorf("expected fallback, got %s", resolved.CompactionKey())
	}
}

func TestCompactionRegistryUnregister(t *testing.T) {
	r := NewCompactionRegistry()
	r.Register(&mockCompactionProvider{key: "a", priority: 1})
	r.Unregister("a")

	if r.Resolve(nil) != nil {
		t.Error("expected nil after unregister")
	}
}

func TestModelProviderRegistryResolve(t *testing.T) {
	r := NewModelProviderRegistry()
	r.Register(&mockProviderFactory{key: "openai", models: []string{"gpt-4", "gpt-3.5"}, priority: 1})
	r.Register(&mockProviderFactory{key: "custom", models: []string{"custom-model"}, priority: 10})

	factory := r.Resolve("custom-model")
	if factory == nil {
		t.Fatal("expected factory for custom-model")
	}
	if factory.ProviderKey() != "custom" {
		t.Errorf("expected custom factory, got %s", factory.ProviderKey())
	}

	factory = r.Resolve("gpt-4")
	if factory == nil {
		t.Fatal("expected factory for gpt-4")
	}
	if factory.ProviderKey() != "openai" {
		t.Errorf("expected openai factory, got %s", factory.ProviderKey())
	}

	if r.Resolve("unknown-model") != nil {
		t.Error("expected nil for unknown model")
	}
}

func TestModelProviderRegistryCount(t *testing.T) {
	r := NewModelProviderRegistry()
	if r.Count() != 0 {
		t.Errorf("expected 0, got %d", r.Count())
	}
	r.Register(&mockProviderFactory{key: "a", models: []string{"m1"}, priority: 1})
	if r.Count() != 1 {
		t.Errorf("expected 1, got %d", r.Count())
	}
}

// mock types for testing

type mockCompactionProvider struct {
	key      string
	priority int
}

func (m *mockCompactionProvider) CompactionKey() string   { return m.key }
func (m *mockCompactionProvider) CompactionPriority() int { return m.priority }
func (m *mockCompactionProvider) Compact(ctx context.Context, model string, messages []providers.ChatMessage) ([]providers.ChatMessage, error) {
	return messages, nil
}

type mockProviderFactory struct {
	key      string
	models   []string
	priority int
}

func (m *mockProviderFactory) ProviderKey() string { return m.key }
func (m *mockProviderFactory) Priority() int       { return m.priority }
func (m *mockProviderFactory) SupportsModel(model string) bool {
	for _, m := range m.models {
		if m == model {
			return true
		}
	}
	return false
}

func (m *mockProviderFactory) CreateClient(ctx context.Context, model string, opts ModelProviderOptions) (providers.Client, error) {
	return nil, nil
}
