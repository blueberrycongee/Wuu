package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestDevWatchPathIgnored(t *testing.T) {
	t.Parallel()
	ignored := []string{
		"dist/index.js",
		"node_modules/dep/index.js",
		".git/HEAD",
		"src/nested/node_modules/x.js",
	}
	for _, rel := range ignored {
		if !devWatchPathIgnored(rel) {
			t.Errorf("devWatchPathIgnored(%q) = false, want true", rel)
		}
	}
	watched := []string{
		"index.ts",
		"plugin.json",
		"src/index.ts",
		"src/theme/tokens.json",
	}
	for _, rel := range watched {
		if devWatchPathIgnored(rel) {
			t.Errorf("devWatchPathIgnored(%q) = true, want false", rel)
		}
	}
}

func TestDevWatchIgnoresEvent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	inside := filepath.Join(root, "dist", "index.js")
	if !devWatchIgnoresEvent(root, fsnotify.Event{Name: inside, Op: fsnotify.Write}) {
		t.Errorf("event under dist should be ignored")
	}
	source := filepath.Join(root, "src", "index.ts")
	if devWatchIgnoresEvent(root, fsnotify.Event{Name: source, Op: fsnotify.Write}) {
		t.Errorf("source event should not be ignored")
	}
	// An ignored-looking component above the watched root must not suppress events.
	aboveRoot := filepath.Join(filepath.Dir(root), "dist")
	if err := os.MkdirAll(aboveRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	nestedRoot := filepath.Join(aboveRoot, "plugin")
	if err := os.MkdirAll(nestedRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	event := filepath.Join(nestedRoot, "index.ts")
	if devWatchIgnoresEvent(nestedRoot, fsnotify.Event{Name: event, Op: fsnotify.Write}) {
		t.Errorf("event should not be ignored because of an ancestor directory name")
	}
	// Events outside the root are not ours to filter.
	if devWatchIgnoresEvent(root, fsnotify.Event{Name: filepath.Join(t.TempDir(), "dist", "x.js"), Op: fsnotify.Write}) {
		t.Errorf("unrelated event outside root should not be ignored")
	}
}

func TestAddDevWatchDirsRecursesAndSkipsIgnored(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, dir := range []string{"src", "src/deep", "dist", "node_modules/dep"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Skipf("fsnotify unavailable: %v", err)
	}
	defer watcher.Close()

	if err := addDevWatchDirs(watcher, root); err != nil {
		t.Fatalf("addDevWatchDirs: %v", err)
	}

	// A write in src/deep must produce an event; a write in dist must not
	// (dist is never registered, so no event can arrive from it).
	writeFile := func(rel string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, rel), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	waitEvent := func(d time.Duration) (fsnotify.Event, bool) {
		t.Helper()
		select {
		case event := <-watcher.Events:
			return event, true
		case <-time.After(d):
			return fsnotify.Event{}, false
		}
	}

	writeFile("src/deep/index.ts")
	event, ok := waitEvent(3 * time.Second)
	if !ok {
		t.Fatalf("expected fsnotify event for src/deep write")
	}
	if devWatchIgnoresEvent(root, event) {
		t.Fatalf("source event %s was filtered", event.Name)
	}

	// Drain any stragglers, then confirm dist writes stay silent.
	for {
		if _, ok := waitEvent(50 * time.Millisecond); !ok {
			break
		}
	}
	writeFile("dist/index.js")
	if event, ok := waitEvent(300 * time.Millisecond); ok {
		t.Fatalf("unexpected event from ignored dist tree: %s", event.Name)
	}
}
