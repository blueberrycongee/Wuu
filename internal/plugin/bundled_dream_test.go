package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundledDreamResolvesRuntimeAndSettingsView(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "wuu-dream-plugin")
	if err := os.WriteFile(helper, []byte("helper"), 0o755); err != nil {
		t.Fatal(err)
	}
	plugins := discoverBundled(t.TempDir(), DiscoverOptions{GOOS: "darwin", LookupEnv: func(key string) (string, bool) {
		if key == "WUU_DREAM_PLUGIN_HELPER" {
			return helper, true
		}
		return "", false
	}})
	for _, item := range plugins {
		if item.ID == "dream" {
			if item.Runtime == nil || item.Runtime.Command != helper || len(item.SettingsPages) != 1 || item.SettingsPages[0].View != "dream.settings" {
				t.Fatalf("dream=%+v", item)
			}
			return
		}
	}
	t.Fatal("bundled dream plugin was not discovered")
}
