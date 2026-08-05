package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/blueberrycongee/wuu/internal/config"
	"github.com/blueberrycongee/wuu/internal/extensions"
	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
	"github.com/blueberrycongee/wuu/internal/statepath"
)

func TestPluginCLIInstallListInspectAndRemove(t *testing.T) {
	home := t.TempDir()
	t.Setenv("WUU_HOME", home)
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "plugin.json"), []byte(`{"id":"cli-demo","name":"CLI Demo","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	inspectOutput := captureStdout(t, func() {
		if err := run([]string{"plugin", "inspect", "--json", source}); err != nil {
			t.Fatalf("plugin inspect: %v", err)
		}
	})
	var inspected pluginPackageOutput
	if err := json.Unmarshal([]byte(inspectOutput), &inspected); err != nil {
		t.Fatalf("decode inspect output: %v\n%s", err, inspectOutput)
	}
	if inspected.ID != "cli-demo" || inspected.SourceKind != "directory" {
		t.Fatalf("inspect output = %+v", inspected)
	}

	installOutput := captureStdout(t, func() {
		if err := run([]string{"plugin", "install", "--json", source}); err != nil {
			t.Fatalf("plugin install: %v", err)
		}
	})
	var installed pluginPackageOutput
	if err := json.Unmarshal([]byte(installOutput), &installed); err != nil {
		t.Fatalf("decode install output: %v\n%s", err, installOutput)
	}
	if installed.ID != "cli-demo" || !installed.ApprovalRequired || installed.Destination != filepath.Join(home, "plugins", "cli-demo") {
		t.Fatalf("install output = %+v", installed)
	}

	listOutput := captureStdout(t, func() {
		if err := run([]string{"plugin", "list", "--json"}); err != nil {
			t.Fatalf("plugin list: %v", err)
		}
	})
	var listed []pluginPackageOutput
	if err := json.Unmarshal([]byte(listOutput), &listed); err != nil {
		t.Fatalf("decode list output: %v\n%s", err, listOutput)
	}
	if len(listed) != 1 || listed[0].ID != "cli-demo" || listed[0].Source != "user" {
		t.Fatalf("list output = %+v", listed)
	}

	removeOutput := captureStdout(t, func() {
		if err := run([]string{"plugin", "remove", "--json", "cli-demo"}); err != nil {
			t.Fatalf("plugin remove: %v", err)
		}
	})
	var removed pluginPackageOutput
	if err := json.Unmarshal([]byte(removeOutput), &removed); err != nil {
		t.Fatalf("decode remove output: %v\n%s", err, removeOutput)
	}
	if !removed.Removed || removed.ID != "cli-demo" {
		t.Fatalf("remove output = %+v", removed)
	}
}

func TestPluginCLIRejectsUnknownSubcommand(t *testing.T) {
	if err := run([]string{"plugin", "unknown"}); err == nil {
		t.Fatal("unknown plugin subcommand unexpectedly succeeded")
	}
}

func TestPluginCLIPolicyActionsUseExactFingerprint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("WUU_HOME", home)
	configPath, err := statepath.ConfigPath("")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "plugin.json"), []byte(`{"id":"policy-demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := pluginpkg.InstallPackage(home, source); err != nil {
		t.Fatal(err)
	}

	var approved pluginPackageOutput
	output := captureStdout(t, func() {
		if err := run([]string{"plugin", "approve", "--json", "policy-demo"}); err != nil {
			t.Fatalf("plugin approve: %v", err)
		}
	})
	if err := json.Unmarshal([]byte(output), &approved); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := config.LoadFrom("", "")
	if err != nil {
		t.Fatal(err)
	}
	grant, ok := loaded.Extensions.FindGrant(approved.SubjectID, approved.Fingerprint)
	if !ok || grant.Scope != extensions.GrantScopeUser {
		t.Fatalf("grant = %+v, %v", grant, ok)
	}

	_ = captureStdout(t, func() {
		if err := run([]string{"plugin", "disable", "policy-demo"}); err != nil {
			t.Fatalf("plugin disable: %v", err)
		}
	})
	loaded, _, err = config.LoadFrom("", "")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Extensions == nil || !loaded.Extensions.IsDisabled(approved.SubjectID) {
		t.Fatalf("plugin was not disabled: %+v", loaded.Extensions)
	}
}

func TestPluginCLIStagesAndDecidesUpdatesWithoutReplacingEarly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("WUU_HOME", home)
	configPath, err := statepath.ConfigPath("")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	manifestPath := filepath.Join(source, "plugin.json")
	if err := os.WriteFile(manifestPath, []byte(`{"id":"update-cli","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"plugin", "install", source}); err != nil {
		t.Fatalf("install v1: %v", err)
	}
	if err := run([]string{"plugin", "approve", "update-cli"}); err != nil {
		t.Fatalf("approve v1: %v", err)
	}
	activeBefore := pluginpkg.Discover("", home)[0]

	if err := os.WriteFile(manifestPath, []byte(`{"id":"update-cli","version":"2.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var staged pluginPackageOutput
	output := captureStdout(t, func() {
		if err := run([]string{"plugin", "update", "--json", "update-cli", source}); err != nil {
			t.Fatalf("stage v2: %v", err)
		}
	})
	if err := json.Unmarshal([]byte(output), &staged); err != nil {
		t.Fatal(err)
	}
	if !staged.Pending || !staged.ApprovalRequired || staged.ActiveFingerprint != activeBefore.Fingerprint {
		t.Fatalf("staged update = %+v", staged)
	}
	activeWhilePending := pluginpkg.Discover("", home)[0]
	if activeWhilePending.Version != "1.0.0" || activeWhilePending.Fingerprint != activeBefore.Fingerprint {
		t.Fatalf("active plugin changed while update was pending: %+v", activeWhilePending)
	}

	listOutput := captureStdout(t, func() {
		if err := run([]string{"plugin", "list", "--json"}); err != nil {
			t.Fatalf("list pending: %v", err)
		}
	})
	var listed []pluginPackageOutput
	if err := json.Unmarshal([]byte(listOutput), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || !listed[0].Pending {
		t.Fatalf("pending list = %+v", listed)
	}

	if err := run([]string{"plugin", "reject", "update-cli"}); err != nil {
		t.Fatalf("reject v2: %v", err)
	}
	if _, err := pluginpkg.ReadPendingUpdate(home, "update-cli"); !errors.Is(err, pluginpkg.ErrPendingUpdateNotFound) {
		t.Fatalf("pending update after rejection = %v", err)
	}
	if pluginpkg.Discover("", home)[0].Version != "1.0.0" {
		t.Fatal("rejected update replaced the installed generation")
	}

	if err := run([]string{"plugin", "update", "update-cli", source}); err != nil {
		t.Fatalf("restage v2: %v", err)
	}
	if err := run([]string{"plugin", "approve", "update-cli"}); err != nil {
		t.Fatalf("approve v2: %v", err)
	}
	promoted := pluginpkg.Discover("", home)[0]
	if promoted.Version != "2.0.0" || promoted.Fingerprint != staged.Fingerprint {
		t.Fatalf("promoted plugin = %+v, staged = %+v", promoted, staged)
	}
	loaded, _, err := config.LoadFrom("", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded.Extensions.FindGrant(promoted.SubjectID, promoted.Fingerprint); !ok {
		t.Fatalf("promoted fingerprint was not approved: %+v", loaded.Extensions)
	}

	if err := run([]string{"plugin", "remove", "update-cli"}); err != nil {
		t.Fatalf("remove promoted plugin: %v", err)
	}
	loaded, _, err = config.LoadFrom("", "")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Extensions != nil {
		if _, ok := loaded.Extensions.Grants[promoted.SubjectID]; ok {
			t.Fatalf("removed plugin grant remains: %+v", loaded.Extensions.Grants)
		}
	}
}
