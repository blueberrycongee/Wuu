package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundledGoalResolvesIndependentRuntimeHelper(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "wuu-goal-plugin")
	if err := os.WriteFile(helper, []byte("helper"), 0o755); err != nil {
		t.Fatal(err)
	}
	plugins := discoverBundled(t.TempDir(), DiscoverOptions{
		GOOS: "darwin",
		LookupEnv: func(key string) (string, bool) {
			if key == "WUU_GOAL_PLUGIN_HELPER" {
				return helper, true
			}
			return "", false
		},
	})
	for _, item := range plugins {
		if item.ID != "goal" {
			continue
		}
		if item.Runtime == nil || item.Runtime.Command != helper || item.Runtime.Protocol != "wuu-plugin-v1" {
			t.Fatalf("goal runtime = %+v", item.Runtime)
		}
		return
	}
	t.Fatal("bundled goal plugin was not discovered")
}
