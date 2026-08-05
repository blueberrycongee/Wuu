package plugin

import (
	"testing"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	reg := NewRegistry[string]()
	gen := NewGeneration("gen-1", "p1", "1.0.0", "fp1")

	entry := RegistryEntry[string]{
		Value:      "hello",
		PluginID:   "p1",
		Generation: gen,
	}

	dispose, err := reg.Register("key1", entry)
	if err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}
	if dispose == nil {
		t.Fatal("expected non-nil disposer")
	}

	got, ok := reg.Get("key1")
	if !ok {
		t.Fatal("expected entry to exist")
	}
	if got.Value != "hello" {
		t.Errorf("expected value hello, got %v", got.Value)
	}
	if got.PluginID != "p1" {
		t.Errorf("expected plugin p1, got %s", got.PluginID)
	}
	if got.Generation != gen {
		t.Error("expected same generation instance")
	}

	if reg.Count() != 1 {
		t.Errorf("expected count 1, got %d", reg.Count())
	}
}

func TestRegistryDuplicateKeyRejected(t *testing.T) {
	reg := NewRegistry[string]()
	gen := NewGeneration("gen-1", "p1", "1.0.0", "fp1")

	reg.Register("key1", RegistryEntry[string]{Value: "first", PluginID: "p1", Generation: gen})
	_, err := reg.Register("key1", RegistryEntry[string]{Value: "second", PluginID: "p1", Generation: gen})
	if err == nil {
		t.Fatal("expected duplicate key error")
	}
}

func TestRegistryRequiredDependency(t *testing.T) {
	reg := NewRegistry[string]()
	gen := NewGeneration("gen-1", "p1", "1.0.0", "fp1")

	// Register a dependency first.
	reg.Register("base", RegistryEntry[string]{Value: "base", PluginID: "p1", Generation: gen})

	// Register an entry that requires it.
	_, err := reg.Register("derived", RegistryEntry[string]{
		Value:    "derived",
		PluginID: "p1",
		Generation: gen,
		DependsOn: map[string]DependencyRule{"base": DepRequired},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryRequiredDependencyMissing(t *testing.T) {
	reg := NewRegistry[string]()
	gen := NewGeneration("gen-1", "p1", "1.0.0", "fp1")

	_, err := reg.Register("derived", RegistryEntry[string]{
		Value:    "derived",
		PluginID: "p1",
		Generation: gen,
		DependsOn: map[string]DependencyRule{"base": DepRequired},
	})
	if err == nil {
		t.Fatal("expected error for missing required dependency")
	}
}

func TestRegistryOptionalDependency(t *testing.T) {
	reg := NewRegistry[string]()
	gen := NewGeneration("gen-1", "p1", "1.0.0", "fp1")

	// Optional dependency can be missing.
	_, err := reg.Register("derived", RegistryEntry[string]{
		Value:    "derived",
		PluginID: "p1",
		Generation: gen,
		DependsOn: map[string]DependencyRule{"base": DepOptional},
	})
	if err != nil {
		t.Fatalf("unexpected error for optional missing: %v", err)
	}

	// And it can be present.
	reg.Register("base", RegistryEntry[string]{Value: "base", PluginID: "p1", Generation: gen})
	_, err = reg.Register("derived2", RegistryEntry[string]{
		Value:    "derived2",
		PluginID: "p1",
		Generation: gen,
		DependsOn: map[string]DependencyRule{"base": DepOptional},
	})
	if err != nil {
		t.Fatalf("unexpected error for optional present: %v", err)
	}
}

func TestRegistryConflictDependency(t *testing.T) {
	reg := NewRegistry[string]()
	gen := NewGeneration("gen-1", "p1", "1.0.0", "fp1")

	reg.Register("compactor-a", RegistryEntry[string]{Value: "a", PluginID: "p1", Generation: gen})

	_, err := reg.Register("compactor-b", RegistryEntry[string]{
		Value:    "b",
		PluginID: "p1",
		Generation: gen,
		DependsOn: map[string]DependencyRule{"compactor-a": DepConflicts},
	})
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestRegistryDisposeRemovesEntries(t *testing.T) {
	reg := NewRegistry[string]()
	gen := NewGeneration("gen-1", "p1", "1.0.0", "fp1")

	reg.Register("key1", RegistryEntry[string]{Value: "v1", PluginID: "p1", Generation: gen})
	reg.Register("key2", RegistryEntry[string]{Value: "v2", PluginID: "p1", Generation: gen})

	if reg.Count() != 2 {
		t.Fatalf("expected 2 entries, got %d", reg.Count())
	}

	gen.Dispose()

	if reg.Count() != 0 {
		t.Errorf("expected 0 entries after dispose, got %d", reg.Count())
	}
	_, ok := reg.Get("key1")
	if ok {
		t.Error("key1 should not exist after generation dispose")
	}
}

func TestRegistryList(t *testing.T) {
	reg := NewRegistry[string]()
	gen := NewGeneration("gen-1", "p1", "1.0.0", "fp1")

	reg.Register("key1", RegistryEntry[string]{Value: "v1", PluginID: "p1", Generation: gen})
	reg.Register("key2", RegistryEntry[string]{Value: "v2", PluginID: "p1", Generation: gen})

	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}
	if list[0].Value != "v1" {
		t.Errorf("expected v1 first, got %v", list[0].Value)
	}
	if list[1].Value != "v2" {
		t.Errorf("expected v2 second, got %v", list[1].Value)
	}
}

func TestRegistryListByPlugin(t *testing.T) {
	reg := NewRegistry[string]()
	gen1 := NewGeneration("gen-1", "p1", "1.0.0", "fp1")
	gen2 := NewGeneration("gen-2", "p2", "1.0.0", "fp2")

	reg.Register("key1", RegistryEntry[string]{Value: "v1", PluginID: "p1", Generation: gen1})
	reg.Register("key2", RegistryEntry[string]{Value: "v2", PluginID: "p2", Generation: gen2})

	p1Entries := reg.ListByPlugin("p1")
	if len(p1Entries) != 1 {
		t.Fatalf("expected 1 entry for p1, got %d", len(p1Entries))
	}
	if p1Entries[0].Value != "v1" {
		t.Errorf("expected v1, got %v", p1Entries[0].Value)
	}

	p2Entries := reg.ListByPlugin("p2")
	if len(p2Entries) != 1 {
		t.Fatalf("expected 1 entry for p2, got %d", len(p2Entries))
	}
	if p2Entries[0].Value != "v2" {
		t.Errorf("expected v2, got %v", p2Entries[0].Value)
	}
}

func TestRegistryKeys(t *testing.T) {
	reg := NewRegistry[string]()
	gen := NewGeneration("gen-1", "p1", "1.0.0", "fp1")

	reg.Register("b", RegistryEntry[string]{Value: "b", PluginID: "p1", Generation: gen})
	reg.Register("a", RegistryEntry[string]{Value: "a", PluginID: "p1", Generation: gen})

	keys := reg.Keys()
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if keys[0] != "b" || keys[1] != "a" {
		t.Errorf("expected keys in insertion order [b a], got %v", keys)
	}
}

func TestRegistryRegisterOnDisposedGeneration(t *testing.T) {
	reg := NewRegistry[string]()
	gen := NewGeneration("gen-1", "p1", "1.0.0", "fp1")
	gen.Dispose()

	_, err := reg.Register("key1", RegistryEntry[string]{Value: "v1", PluginID: "p1", Generation: gen})
	if err == nil {
		t.Fatal("expected error for disposed generation")
	}
}

func TestPluginRegistriesCreation(t *testing.T) {
	regs := NewPluginRegistries()

	if regs.Tools == nil {
		t.Error("Tools registry should not be nil")
	}
	if regs.SystemPrompts == nil {
		t.Error("SystemPrompts registry should not be nil")
	}
	if regs.Providers == nil {
		t.Error("Providers registry should not be nil")
	}
	if regs.Compactions == nil {
		t.Error("Compactions registry should not be nil")
	}
	if regs.Views == nil {
		t.Error("Views registry should not be nil")
	}
	if regs.Themes == nil {
		t.Error("Themes registry should not be nil")
	}
}

func TestPluginRegistriesRegistryFor(t *testing.T) {
	regs := NewPluginRegistries()

	tests := map[EffectKind]*Registry[any]{
		EffectTool:         regs.Tools,
		EffectSystemPrompt: regs.SystemPrompts,
		EffectContext:      regs.Contexts,
		EffectProvider:     regs.Providers,
		EffectCompaction:   regs.Compactions,
		EffectContinuation: regs.Continuations,
		EffectSubagent:     regs.Subagents,
		EffectPermission:   regs.Permissions,
		EffectCommand:      regs.Commands,
		EffectView:         regs.Views,
		EffectTheme:        regs.Themes,
		EffectSetting:      regs.Settings,
		EffectStorage:      regs.Storages,
		EffectLayout:       regs.Layouts,
		EffectRenderer:     regs.Renderers,
		EffectShell:        regs.Shells,
	}

	for kind, want := range tests {
		got := regs.RegistryFor(kind)
		if got != want {
			t.Errorf("RegistryFor(%s) returned wrong registry", kind)
		}
	}

	if regs.RegistryFor("nonexistent") != nil {
		t.Error("RegistryFor(nonexistent) should return nil")
	}
}

func TestValidateDependencies(t *testing.T) {
	reg := NewRegistry[string]()
	gen := NewGeneration("gen-1", "p1", "1.0.0", "fp1")

	_, err := reg.Register("base", RegistryEntry[string]{Value: "base", PluginID: "p1", Generation: gen})
	if err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}
	_, err = reg.Register("derived", RegistryEntry[string]{
		Value:    "derived",
		PluginID: "p1",
		Generation: gen,
		DependsOn: map[string]DependencyRule{"base": DepRequired},
	})
	if err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}

	if err := ValidateDependencies(reg); err != nil {
		t.Errorf("expected valid dependencies: %v", err)
	}
}

func TestValidateDependenciesMissingAfterDispose(t *testing.T) {
	reg := NewRegistry[string]()
	gen1 := NewGeneration("gen-1", "p1", "1.0.0", "fp1")
	gen2 := NewGeneration("gen-2", "p2", "1.0.0", "fp2")

	_, err := reg.Register("base", RegistryEntry[string]{Value: "base", PluginID: "p1", Generation: gen1})
	if err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}
	_, err = reg.Register("derived", RegistryEntry[string]{
		Value:    "derived",
		PluginID: "p2",
		Generation: gen2,
		DependsOn: map[string]DependencyRule{"base": DepRequired},
	})
	if err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}

	// Before dispose, everything is valid.
	if err := ValidateDependencies(reg); err != nil {
		t.Fatalf("expected valid before dispose: %v", err)
	}

	// Dispose gen1 — "derived" now has a disposed required dependency.
	gen1.Dispose()

	if err := ValidateDependencies(reg); err == nil {
		t.Error("expected validation error after dependency generation was disposed")
	}
}

func TestEmptyKeyRejected(t *testing.T) {
	reg := NewRegistry[string]()
	gen := NewGeneration("gen-1", "p1", "1.0.0", "fp1")

	_, err := reg.Register("", RegistryEntry[string]{Value: "v1", PluginID: "p1", Generation: gen})
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}
