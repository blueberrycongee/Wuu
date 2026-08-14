package runtime

import (
	"testing"
)

func TestSessionPluginGenerationSnapshotReflectsActiveGeneration(t *testing.T) {
	oldGeneration := testPluginGeneration("demo", &runtimeCapabilityClient{id: "old"})
	oldGeneration.active[0].Fingerprint = "fingerprint-a"
	oldSession := testGenerationSession(oldGeneration)

	nextGeneration := testPluginGeneration("demo", &runtimeCapabilityClient{id: "next"})
	nextGeneration.active[0].Fingerprint = "fingerprint-b"
	nextSession := testGenerationSession(nextGeneration)

	oldSnapshot := oldSession.PluginGenerationSnapshot()
	nextSnapshot := nextSession.PluginGenerationSnapshot()

	if len(oldSnapshot.Plugins) != 1 || oldSnapshot.Plugins[0].Fingerprint != "fingerprint-a" {
		t.Fatalf("old snapshot = %+v, want fingerprint-a", oldSnapshot)
	}
	if len(nextSnapshot.Plugins) != 1 || nextSnapshot.Plugins[0].Fingerprint != "fingerprint-b" {
		t.Fatalf("next snapshot = %+v, want fingerprint-b", nextSnapshot)
	}
	if oldSnapshot.Plugins[0].Fingerprint == nextSnapshot.Plugins[0].Fingerprint {
		t.Fatalf("old and next snapshots unexpectedly agree: %+v", oldSnapshot)
	}
}
