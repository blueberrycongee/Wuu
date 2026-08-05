package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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
