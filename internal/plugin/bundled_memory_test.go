package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundledMemoryResolvesRuntimeAndSettingsView(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "wuu-memory-plugin")
	if err := os.WriteFile(helper, []byte("helper"), 0o755); err != nil {
		t.Fatal(err)
	}
	plugins := discoverBundled(t.TempDir(), DiscoverOptions{GOOS: "darwin", LookupEnv: func(key string) (string, bool) {
		if key == "WUU_MEMORY_PLUGIN_HELPER" {
			return helper, true
		}
		return "", false
	}})
	for _, item := range plugins {
		if item.ID != "memory" {
			continue
		}
		if item.Runtime == nil || item.Runtime.Command != helper {
			t.Fatalf("memory runtime = %+v", item.Runtime)
		}
		if item.Desktop == nil || item.Desktop.Entry != "desktop.js" {
			t.Fatalf("memory desktop = %+v", item.Desktop)
		}
		if len(item.SettingsPages) != 1 || item.SettingsPages[0].View != "memory.settings" {
			t.Fatalf("memory settings pages = %+v", item.SettingsPages)
		}
		return
	}
	t.Fatal("bundled memory plugin was not discovered")
}
