package agent

import (
	"context"
	"testing"

	"github.com/blueberrycongee/wuu/internal/providers"
)

func TestSystemPromptAssemblerRemoveByPlugin(t *testing.T) {
	a := NewSystemPromptAssembler()

	// Register sections owned by different plugins.
	a.AddWithOwner(NewStaticPromptSection("p1.section-a", "Section A", 1), "plugin-1")
	a.AddWithOwner(NewStaticPromptSection("p1.section-b", "Section B", 2), "plugin-1")
	a.AddWithOwner(NewStaticPromptSection("p2.section-c", "Section C", 3), "plugin-2")

	if a.ProviderCount() != 3 {
		t.Fatalf("expected 3 providers, got %d", a.ProviderCount())
	}

	// Remove plugin-1. Sections A and B should be gone.
	a.RemoveByPlugin("plugin-1")
	if a.ProviderCount() != 1 {
		t.Fatalf("expected 1 provider after removing plugin-1, got %d", a.ProviderCount())
	}

	_, sections := a.Assemble("base")
	if len(sections) != 2 { // base + section-c
		t.Fatalf("expected 2 sections (base + p2), got %d", len(sections))
	}
	if sections[1].Key != "p2.section-c" {
		t.Errorf("expected p2.section-c, got %s", sections[1].Key)
	}
}

func TestSystemPromptAssemblerRemoveByPluginNoOpForUnknown(t *testing.T) {
	a := NewSystemPromptAssembler()
	a.AddWithOwner(NewStaticPromptSection("s1", "text", 1), "plugin-1")

	a.RemoveByPlugin("nonexistent")
	if a.ProviderCount() != 1 {
		t.Errorf("expected 1 provider, got %d", a.ProviderCount())
	}
}

func TestRequestTransformRemoveByPlugin(t *testing.T) {
	c := NewRequestTransformChain()

	var called1, called2 bool
	c.AddWithOwner(NewRequestTransform("p1.t1", func(ctx context.Context, req *providers.ChatRequest) error { called1 = true; return nil }, 1), "plugin-1")
	c.AddWithOwner(NewRequestTransform("p2.t2", func(ctx context.Context, req *providers.ChatRequest) error { called2 = true; return nil }, 1), "plugin-2")

	c.RemoveByPlugin("plugin-1")

	req := &providers.ChatRequest{Model: "test"}
	_ = c.Apply(context.Background(), req, nil)

	if called1 {
		t.Error("plugin-1 transform should have been removed")
	}
	if !called2 {
		t.Error("plugin-2 transform should still execute")
	}
	if c.Count() != 1 {
		t.Errorf("expected 1 transform, got %d", c.Count())
	}
}

func TestCompactionRegistryRemoveByPlugin(t *testing.T) {
	r := NewCompactionRegistry()

	r.RegisterWithOwner(&mockCompactionProvider{key: "p1.compact", priority: 1}, "plugin-1")
	r.RegisterWithOwner(&mockCompactionProvider{key: "p2.compact", priority: 10}, "plugin-2")

	// Before removal, p2 should win due to higher priority.
	resolved := r.Resolve(nil)
	if resolved.CompactionKey() != "p2.compact" {
		t.Errorf("expected p2.compact, got %s", resolved.CompactionKey())
	}

	// Remove p2. Now p1 should win.
	r.RemoveByPlugin("plugin-2")
	resolved = r.Resolve(nil)
	if resolved.CompactionKey() != "p1.compact" {
		t.Errorf("expected p1.compact after removing p2, got %s", resolved.CompactionKey())
	}
	if r.Count() != 1 {
		t.Errorf("expected 1 provider, got %d", r.Count())
	}
}

func TestModelProviderRegistryRemoveByPlugin(t *testing.T) {
	r := NewModelProviderRegistry()

	r.RegisterWithOwner(&mockProviderFactory{key: "p1", models: []string{"m1"}, priority: 1}, "plugin-1")
	r.RegisterWithOwner(&mockProviderFactory{key: "p2", models: []string{"m1"}, priority: 10}, "plugin-2")

	// Before removal, p2 wins.
	resolved := r.Resolve("m1")
	if resolved.ProviderKey() != "p2" {
		t.Errorf("expected p2, got %s", resolved.ProviderKey())
	}

	r.RemoveByPlugin("plugin-2")
	resolved = r.Resolve("m1")
	if resolved.ProviderKey() != "p1" {
		t.Errorf("expected p1 after removing p2, got %s", resolved.ProviderKey())
	}
}

func TestRegisterWithOwnerBackwardCompat(t *testing.T) {
	// Calling Add/Register without owner should work (backward compat).
	a := NewSystemPromptAssembler()
	a.Add(NewStaticPromptSection("s1", "text", 1))
	if a.ProviderCount() != 1 {
		t.Errorf("expected 1 provider, got %d", a.ProviderCount())
	}

	c := NewRequestTransformChain()
	c.Add(NewRequestTransform("t1", func(ctx context.Context, req *providers.ChatRequest) error { return nil }, 1))
	if c.Count() != 1 {
		t.Errorf("expected 1 transform, got %d", c.Count())
	}

	r := NewCompactionRegistry()
	r.Register(&mockCompactionProvider{key: "c1", priority: 1})
	if r.Count() != 1 {
		t.Errorf("expected 1 compaction, got %d", r.Count())
	}

	pr := NewModelProviderRegistry()
	pr.Register(&mockProviderFactory{key: "f1", models: []string{"m"}, priority: 1})
	if pr.Count() != 1 {
		t.Errorf("expected 1 factory, got %d", pr.Count())
	}
}

// TestRemoveByGenerationIsolation verifies that RemoveByGeneration only
// removes entries from the specified generation, leaving other generations
// (including newer ones from the same plugin) intact.
func TestRemoveByGenerationIsolation(t *testing.T) {
	// Simulate: old gen "p1-gen-old" and new gen "p1-gen-new" from same plugin "p1".
	a := NewSystemPromptAssembler()
	a.AddWithOwner(NewStaticPromptSection("p1.section-a", "Section A", 1), "p1-gen-old")
	a.AddWithOwner(NewStaticPromptSection("p1.section-b", "Section B", 1), "p1-gen-new")

	if a.ProviderCount() != 2 {
		t.Fatalf("expected 2 providers, got %d", a.ProviderCount())
	}

	// Remove old generation. Should only remove section-a.
	a.RemoveByGeneration("p1-gen-old")

	if a.ProviderCount() != 1 {
		t.Fatalf("expected 1 provider after removing old gen, got %d", a.ProviderCount())
	}

	_, sections := a.Assemble("")
	if len(sections) != 1 {
		t.Fatalf("expected 1 section (p1.section-b), got %d", len(sections))
	}
	if sections[0].Key != "p1.section-b" {
		t.Errorf("new generation section should survive: got %s", sections[0].Key)
	}
}

// TestRemoveByGenerationVsPlugin verifies generation-scoped cleanup
// across all 4 registries.
func TestRemoveByGenerationAllRegistries(t *testing.T) {
	genOld := "p1-gen-old-abc"
	genNew := "p1-gen-new-def"

	// SystemPromptAssembler
	spa := NewSystemPromptAssembler()
	spa.AddWithOwner(NewStaticPromptSection("old-sec", "old", 1), genOld)
	spa.AddWithOwner(NewStaticPromptSection("new-sec", "new", 1), genNew)
	spa.RemoveByGeneration(genOld)
	if spa.ProviderCount() != 1 {
		t.Errorf("SPA: expected 1, got %d", spa.ProviderCount())
	}

	// RequestTransformChain
	rtc := NewRequestTransformChain()
	rtc.AddWithOwner(NewRequestTransform("old-t", func(ctx context.Context, req *providers.ChatRequest) error { return nil }, 1), genOld)
	rtc.AddWithOwner(NewRequestTransform("new-t", func(ctx context.Context, req *providers.ChatRequest) error { return nil }, 1), genNew)
	rtc.RemoveByGeneration(genOld)
	if rtc.Count() != 1 {
		t.Errorf("RTC: expected 1, got %d", rtc.Count())
	}

	// CompactionRegistry
	cr := NewCompactionRegistry()
	cr.RegisterWithOwner(&mockCompactionProvider{key: "old-comp", priority: 10}, genOld)
	cr.RegisterWithOwner(&mockCompactionProvider{key: "new-comp", priority: 1}, genNew)
	cr.RemoveByGeneration(genOld)
	resolved := cr.Resolve(nil)
	if resolved.CompactionKey() != "new-comp" {
		t.Errorf("CR: expected new-comp after old gen removed, got %s", resolved.CompactionKey())
	}

	// ModelProviderRegistry
	mpr := NewModelProviderRegistry()
	mpr.RegisterWithOwner(&mockProviderFactory{key: "old-prov", models: []string{"m"}, priority: 10}, genOld)
	mpr.RegisterWithOwner(&mockProviderFactory{key: "new-prov", models: []string{"m"}, priority: 1}, genNew)
	mpr.RemoveByGeneration(genOld)
	resolved2 := mpr.Resolve("m")
	if resolved2.ProviderKey() != "new-prov" {
		t.Errorf("MPR: expected new-prov after old gen removed, got %s", resolved2.ProviderKey())
	}
}
