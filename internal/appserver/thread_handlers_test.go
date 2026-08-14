package appserver

import (
	"testing"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/runtime"
	"github.com/blueberrycongee/wuu/internal/session"
)

func TestLoadPersistedThreadSnapshotReadsPluginGeneration(t *testing.T) {
	sessDir := t.TempDir()
	if _, err := session.CreateWithMetadata(sessDir, "t1", "/work"); err != nil {
		t.Fatalf("create thread: %v", err)
	}
	snapshot := session.PluginGenerationSnapshot{Plugins: []session.PluginGenerationBinding{
		{ID: "demo", Fingerprint: "fingerprint-a"},
	}}
	if err := session.WritePluginGenerationSnapshot(sessDir, "t1", snapshot); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}

	rt := &runtime.Session{
		SessionDir:   sessDir,
		StreamRunner: &agent.StreamRunner{SystemPrompt: "system"},
	}
	server := New(rt, nil)

	loaded, err := server.loadPersistedThreadSnapshot("t1")
	if err != nil {
		t.Fatalf("load thread: %v", err)
	}
	if len(loaded.pluginGeneration.Plugins) != 1 || loaded.pluginGeneration.Plugins[0].Fingerprint != "fingerprint-a" {
		t.Fatalf("plugin generation = %+v", loaded.pluginGeneration)
	}
}
