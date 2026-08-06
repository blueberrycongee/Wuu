package runtime

import (
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/config"
	"github.com/blueberrycongee/wuu/internal/extensions"
	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
)

func TestActivatedPluginsFailClosed(t *testing.T) {
	community := pluginpkg.Plugin{
		Manifest:             pluginpkg.Manifest{ID: "community"},
		SubjectID:            "project:community",
		Fingerprint:          "sha256:current",
		EffectivePermissions: []string{extensions.PermProcessSpawn},
	}
	official := community
	official.SubjectID = "bundled:official"
	official.ID = "official"
	official.Official = true

	if got := activatedPlugins(config.Config{}, []pluginpkg.Plugin{community}); len(got) != 0 {
		t.Fatalf("pending package activated: %+v", got)
	}
	if got := activatedPlugins(config.Config{}, []pluginpkg.Plugin{official}); len(got) != 1 {
		t.Fatalf("official package did not activate: %+v", got)
	}

	settings := &extensions.Settings{}
	if err := settings.RecordGrant(extensions.Grant{
		SubjectID: community.SubjectID, Fingerprint: community.Fingerprint,
		Scope: extensions.GrantScopeProject, Permissions: community.EffectivePermissions,
		ApprovedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if got := activatedPlugins(config.Config{Extensions: settings}, []pluginpkg.Plugin{community}); len(got) != 1 {
		t.Fatalf("exactly granted package did not activate: %+v", got)
	}

	changed := community
	changed.Fingerprint = "sha256:changed"
	if got := activatedPlugins(config.Config{Extensions: settings}, []pluginpkg.Plugin{changed}); len(got) != 0 {
		t.Fatalf("changed package activated using stale grant: %+v", got)
	}

	insufficient := &extensions.Settings{}
	if err := insufficient.RecordGrant(extensions.Grant{
		SubjectID: community.SubjectID, Fingerprint: community.Fingerprint,
		Scope: extensions.GrantScopeProject, ApprovedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if got := activatedPlugins(config.Config{Extensions: insufficient}, []pluginpkg.Plugin{community}); len(got) != 0 {
		t.Fatalf("package activated without all effective permissions: %+v", got)
	}

	if err := settings.RecordRejection(community.SubjectID, community.Fingerprint); err != nil {
		t.Fatal(err)
	}
	if got := activatedPlugins(config.Config{Extensions: settings}, []pluginpkg.Plugin{community}); len(got) != 0 {
		t.Fatalf("rejected package activated: %+v", got)
	}

	settings.Revoke(community.SubjectID)
	settings.SetDisabled(community.SubjectID, true)
	if got := activatedPlugins(config.Config{Extensions: settings}, []pluginpkg.Plugin{community}); len(got) != 0 {
		t.Fatalf("disabled package activated: %+v", got)
	}
	settings.SetDisabled(official.SubjectID, true)
	if got := activatedPlugins(config.Config{Extensions: settings}, []pluginpkg.Plugin{official}); len(got) != 0 {
		t.Fatalf("disabled official package activated: %+v", got)
	}
}

func TestActivatedPluginsEnforcesMinimumWuuVersion(t *testing.T) {
	original := currentWuuVersion
	t.Cleanup(func() { currentWuuVersion = original })
	currentWuuVersion = func() string { return "0.15.0" }

	community := pluginpkg.Plugin{
		Manifest:             pluginpkg.Manifest{ID: "community"},
		SubjectID:            "project:community",
		Fingerprint:          "sha256:current",
		EffectivePermissions: []string{extensions.PermProcessSpawn},
	}
	settings := &extensions.Settings{}
	if err := settings.RecordGrant(extensions.Grant{
		SubjectID: community.SubjectID, Fingerprint: community.Fingerprint,
		Scope: extensions.GrantScopeProject, Permissions: community.EffectivePermissions,
		ApprovedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Extensions: settings}

	compatible := community
	compatible.MinimumWuuVersion = "0.14.0"
	if got := activatedPlugins(cfg, []pluginpkg.Plugin{compatible}); len(got) != 1 {
		t.Fatalf("package below host minor version did not activate: %+v", got)
	}

	incompatible := community
	incompatible.MinimumWuuVersion = "0.16.0"
	if got := activatedPlugins(cfg, []pluginpkg.Plugin{incompatible}); len(got) != 0 {
		t.Fatalf("package above host version activated: %+v", got)
	}

	// The gate applies to every trust tier: an official package declaring a
	// future minimum must not activate either.
	official := incompatible
	official.SubjectID = "bundled:official"
	official.ID = "official"
	official.Official = true
	if got := activatedPlugins(config.Config{}, []pluginpkg.Plugin{official}); len(got) != 0 {
		t.Fatalf("incompatible official package activated: %+v", got)
	}

	// Dev builds carry a pre-release marker and satisfy constraints at base.
	currentWuuVersion = func() string { return "v0.15.0-dev" }
	devCompatible := community
	devCompatible.MinimumWuuVersion = "0.15.0"
	if got := activatedPlugins(cfg, []pluginpkg.Plugin{devCompatible}); len(got) != 1 {
		t.Fatalf("dev host did not satisfy base-version constraint: %+v", got)
	}
}
