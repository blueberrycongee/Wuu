package session

import (
	"testing"
)

func TestPluginGenerationSnapshotPersistsPerSession(t *testing.T) {
	dir := t.TempDir()
	if _, err := CreateWithMetadata(dir, "s1", "/work"); err != nil {
		t.Fatalf("create s1: %v", err)
	}
	if _, err := CreateWithMetadata(dir, "s2", "/work"); err != nil {
		t.Fatalf("create s2: %v", err)
	}

	old := PluginGenerationSnapshot{Plugins: []PluginGenerationBinding{
		{ID: "demo", Fingerprint: "fingerprint-a"},
		{ID: "theme", Fingerprint: "theme-a"},
	}}
	next := PluginGenerationSnapshot{Plugins: []PluginGenerationBinding{
		{ID: "demo", Fingerprint: "fingerprint-b"},
	}}

	if err := WritePluginGenerationSnapshot(dir, "s1", old); err != nil {
		t.Fatalf("write s1: %v", err)
	}
	if err := WritePluginGenerationSnapshot(dir, "s2", next); err != nil {
		t.Fatalf("write s2: %v", err)
	}

	// Each read reopens the store, simulating an app-server restart.
	gotOld, ok, err := ReadPluginGenerationSnapshot(dir, "s1")
	if err != nil || !ok {
		t.Fatalf("read s1: ok=%v err=%v", ok, err)
	}
	if len(gotOld.Plugins) != 2 || gotOld.Plugins[0].ID != "demo" || gotOld.Plugins[0].Fingerprint != "fingerprint-a" {
		t.Fatalf("s1 snapshot = %+v", gotOld)
	}

	gotNext, ok, err := ReadPluginGenerationSnapshot(dir, "s2")
	if err != nil || !ok {
		t.Fatalf("read s2: ok=%v err=%v", ok, err)
	}
	if len(gotNext.Plugins) != 1 || gotNext.Plugins[0].ID != "demo" || gotNext.Plugins[0].Fingerprint != "fingerprint-b" {
		t.Fatalf("s2 snapshot = %+v", gotNext)
	}

	if _, ok, err := ReadPluginGenerationSnapshot(dir, "missing"); err != nil || ok {
		t.Fatalf("missing snapshot: ok=%v err=%v, want not found", ok, err)
	}
}

func TestPluginGenerationSnapshotNormalize(t *testing.T) {
	got := PluginGenerationSnapshot{Plugins: []PluginGenerationBinding{
		{ID: "b", Fingerprint: "f2"},
		{ID: "a", Fingerprint: "f1"},
		{ID: "b", Fingerprint: "ignored"},
		{ID: " ", Fingerprint: "ignored"},
	}}.Normalize()

	if len(got.Plugins) != 2 || got.Plugins[0].ID != "a" || got.Plugins[1].ID != "b" || got.Plugins[1].Fingerprint != "f2" {
		t.Fatalf("normalized = %+v", got)
	}
}
