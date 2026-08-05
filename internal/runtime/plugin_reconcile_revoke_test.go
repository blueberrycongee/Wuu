package runtime

import (
	"testing"

	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
)

// Revoking a grant removes the plugin from the active set; its runtime must be
// closed and unregistered so no stale hooks keep intercepting traffic
// (threat model control #10).
func TestReconcilePluginHostClosesRevokedPluginClient(t *testing.T) {
	revoked := &closeTrackingPluginClient{id: "revoked"}
	kept := &closeTrackingPluginClient{id: "kept"}
	host := pluginhost.New(revoked, kept)

	previous := []pluginpkg.Plugin{
		{Manifest: pluginpkg.Manifest{ID: "revoked", Runtime: &pluginpkg.RuntimeSpec{Command: "x"}}, Fingerprint: "fp-a"},
		{Manifest: pluginpkg.Manifest{ID: "kept", Runtime: &pluginpkg.RuntimeSpec{Command: "x"}}, Fingerprint: "fp-b"},
	}
	next := []pluginpkg.Plugin{
		{Manifest: pluginpkg.Manifest{ID: "kept", Runtime: &pluginpkg.RuntimeSpec{Command: "x"}}, Fingerprint: "fp-b"},
	}

	if err := reconcilePluginHost(host, previous, next, t.TempDir(), t.TempDir()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !revoked.closed {
		t.Fatal("revoked plugin client was not closed")
	}
	if kept.closed {
		t.Fatal("kept plugin client must not be closed")
	}
	for _, client := range host.Clients() {
		if client.ID() == "revoked" {
			t.Fatalf("revoked plugin still registered: %+v", host.Clients())
		}
	}
}

// A fingerprint change invalidates the grant: the old runtime must be closed
// and replaced by a freshly started client rather than reused.
func TestReconcilePluginHostRestartsClientWhenFingerprintChanges(t *testing.T) {
	stale := &closeTrackingPluginClient{id: "changed"}
	host := pluginhost.New(stale)

	previous := []pluginpkg.Plugin{
		{Manifest: pluginpkg.Manifest{ID: "changed", Runtime: &pluginpkg.RuntimeSpec{Command: "x"}}, Fingerprint: "fp-old"},
	}
	next := []pluginpkg.Plugin{
		{Manifest: pluginpkg.Manifest{ID: "changed", Runtime: &pluginpkg.RuntimeSpec{Command: "x"}}, Fingerprint: "fp-new"},
	}

	replacement := &closeTrackingPluginClient{id: "changed"}
	var startedID string
	err := reconcilePluginHostWithStarter(host, previous, next, "", "", func(item pluginpkg.Plugin, _, _ string) (pluginhost.Client, error) {
		startedID = item.ID
		return replacement, nil
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if startedID != "changed" {
		t.Fatalf("starter not invoked for fingerprint-changed plugin, got %q", startedID)
	}
	if !stale.closed {
		t.Fatal("stale fingerprint client was not closed")
	}
	clients := host.Clients()
	if len(clients) != 1 || clients[0] != pluginhost.Client(replacement) {
		t.Fatalf("host did not swap in replacement client: %+v", clients)
	}
}
