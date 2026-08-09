package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundledSubagentResolvesIndependentRuntimeHelper(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "wuu-subagent-plugin")
	if err := os.WriteFile(helper, []byte("helper"), 0o755); err != nil {
		t.Fatal(err)
	}
	plugins := discoverBundled(t.TempDir(), DiscoverOptions{
		GOOS: "darwin",
		LookupEnv: func(key string) (string, bool) {
			if key == "WUU_SUBAGENT_PLUGIN_HELPER" {
				return helper, true
			}
			return "", false
		},
	})
	for _, item := range plugins {
		if item.ID != "subagent" {
			continue
		}
		if item.Runtime == nil || item.Runtime.Command != helper || item.Runtime.Protocol != "wuu-plugin-v1" {
			t.Fatalf("subagent runtime = %+v", item.Runtime)
		}
		if len(item.Slots) != 2 || item.Slots[0].ID != "subagent-status" || item.Slots[1].ID != "subagent-ultra" {
			t.Fatalf("subagent slots = %+v", item.Slots)
		}
		if len(item.SettingsPages) != 1 || item.SettingsPages[0].View != "subagent.settings" {
			t.Fatalf("subagent settings pages = %+v", item.SettingsPages)
		}
		return
	}
	t.Fatal("bundled subagent plugin was not discovered")
}
