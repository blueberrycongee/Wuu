package appserver

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/runtime"
)

func TestConfigWatchUsesEventsInsteadOfContinuousReloads(t *testing.T) {
	root := t.TempDir()
	wuuHome := t.TempDir()
	refreshes := make(chan struct{}, 8)
	srv := &Server{
		rt: &runtime.Session{
			RootDir:    root,
			WuuHome:    wuuHome,
			ConfigPath: filepath.Join(root, ".wuu.json"),
		},
		out:     &lockedBuffer{},
		threads: map[string]*threadState{},
		refreshConfigForTest: func() error {
			refreshes <- struct{}{}
			return nil
		},
	}
	srv.startConfigWatch()
	t.Cleanup(func() {
		srv.closed.Store(true)
		srv.backgroundWG.Wait()
	})

	select {
	case <-refreshes:
	case <-time.After(3 * time.Second):
		t.Fatal("config watcher did not establish its initial baseline")
	}

	select {
	case <-refreshes:
		t.Fatal("config watcher reloaded an unchanged config")
	case <-time.After(700 * time.Millisecond):
	}
	if err := os.WriteFile(filepath.Join(wuuHome, "runtime-state.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-refreshes:
		t.Fatal("config watcher reloaded after an unrelated runtime-state change")
	case <-time.After(300 * time.Millisecond):
	}

	if err := os.WriteFile(filepath.Join(root, ".wuu.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-refreshes:
	case <-time.After(3 * time.Second):
		t.Fatal("config watcher did not refresh after a filesystem event")
	}
}
