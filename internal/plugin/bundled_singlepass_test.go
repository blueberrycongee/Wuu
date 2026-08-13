package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundledSinglepassResolvesRuntime(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "wuu-singlepass-plugin")
	if err := os.WriteFile(helper, []byte("helper"), 0o755); err != nil {
		t.Fatal(err)
	}
	plugins := discoverBundled(t.TempDir(), DiscoverOptions{GOOS: "darwin", LookupEnv: func(key string) (string, bool) {
		if key == "WUU_SINGLEPASS_PLUGIN_HELPER" {
			return helper, true
		}
		if key == EnableSinglepassEnv {
			return "1", true
		}
		return "", false
	}})
	for _, item := range plugins {
		if item.ID == "singlepass" {
			if item.Runtime == nil || item.Runtime.Command != helper {
				t.Fatalf("singlepass=%+v", item)
			}
			return
		}
	}
	t.Fatal("bundled singlepass plugin was not discovered")
}
