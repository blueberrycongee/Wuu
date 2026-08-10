package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundledAutomationResolvesRuntimeAndDesktopView(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "wuu-automation-plugin")
	if err := os.WriteFile(helper, []byte("helper"), 0o755); err != nil {
		t.Fatal(err)
	}
	plugins := discoverBundled(t.TempDir(), DiscoverOptions{GOOS: "darwin", LookupEnv: func(key string) (string, bool) {
		if key == "WUU_AUTOMATION_PLUGIN_HELPER" {
			return helper, true
		}
		return "", false
	}})
	for _, item := range plugins {
		if item.ID != "automation" {
			continue
		}
		if item.Runtime == nil || item.Runtime.Command != helper {
			t.Fatalf("automation runtime = %+v", item.Runtime)
		}
		if item.Desktop == nil || item.Desktop.Entry != "desktop.js" {
			t.Fatalf("automation desktop = %+v", item.Desktop)
		}
		if len(item.Navigation) != 1 || item.Navigation[0].View != "automation.catalog" {
			t.Fatalf("automation navigation = %+v", item.Navigation)
		}
		return
	}
	t.Fatal("bundled automation plugin was not discovered")
}
