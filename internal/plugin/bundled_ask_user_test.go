package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundledAskUserResolvesIndependentRuntimeHelper(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "wuu-ask-user-plugin")
	if err := os.WriteFile(helper, []byte("helper"), 0o755); err != nil {
		t.Fatal(err)
	}
	plugins := discoverBundled(t.TempDir(), DiscoverOptions{
		GOOS: "darwin",
		LookupEnv: func(key string) (string, bool) {
			if key == "WUU_ASK_USER_PLUGIN_HELPER" {
				return helper, true
			}
			return "", false
		},
	})
	for _, item := range plugins {
		if item.ID != "ask-user" {
			continue
		}
		if item.Runtime == nil || item.Runtime.Command != helper || item.Runtime.Protocol != "wuu-plugin-v1" || item.Runtime.Timeout != 15 {
			t.Fatalf("ask-user runtime = %+v", item.Runtime)
		}
		if item.Icon == nil || item.Icon.Path != "assets/icon.svg" {
			t.Fatalf("ask-user icon = %+v", item.Icon)
		}
		return
	}
	t.Fatal("bundled ask-user plugin was not discovered")
}
