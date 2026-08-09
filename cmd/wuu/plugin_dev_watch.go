package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// devWatchDebounce collapses editor save bursts into one generation refresh.
const devWatchDebounce = 300 * time.Millisecond

// devWatchIgnoredDirs never trigger refreshes: build output would loop the
// watcher (refresh rebuilds into dist), and dependency/VCS churn is not source.
var devWatchIgnoredDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"dist":         true,
}

// watchDevDirFS watches the plugin source tree with fsnotify and refreshes the
// development generation after each settled save burst. It falls back to
// polling when the platform watcher cannot be created.
func watchDevDirFS(wuuHome, dir, packageManager string, pollInterval time.Duration, initialPending bool) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("fsnotify unavailable (%v); falling back to polling every %s\n", err, pollInterval)
		watchDevDir(wuuHome, dir, packageManager, pollInterval, initialPending)
		return
	}
	defer watcher.Close()

	if err := addDevWatchDirs(watcher, dir); err != nil {
		fmt.Printf("fsnotify setup failed (%v); falling back to polling every %s\n", err, pollInterval)
		watchDevDir(wuuHome, dir, packageManager, pollInterval, initialPending)
		return
	}

	fmt.Printf("Watching for changes (fsnotify, debounce %s)... (Ctrl+C to stop)\n", devWatchDebounce)

	var debounce *time.Timer
	var debounceC <-chan time.Time
	pending := false
	schedule := func(delay time.Duration) {
		if debounce == nil {
			debounce = time.NewTimer(delay)
		} else {
			if !debounce.Stop() && pending {
				select {
				case <-debounce.C:
				default:
				}
			}
			debounce.Reset(delay)
		}
		debounceC = debounce.C
		pending = true
	}
	stopDebounce := func() {
		if debounce != nil {
			if !debounce.Stop() && pending {
				select {
				case <-debounce.C:
				default:
				}
			}
		}
		debounceC = nil
		pending = false
	}
	defer stopDebounce()
	if initialPending {
		schedule(pollInterval)
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if devWatchIgnoresEvent(dir, event) {
				continue
			}
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() && !devWatchIgnoredDirs[filepath.Base(event.Name)] {
					// New source trees must be watched too; fsnotify is not recursive.
					_ = addDevWatchDirs(watcher, event.Name)
				}
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
				schedule(devWatchDebounce)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("watch error: %v\n", err)
		case <-debounceC:
			pending = false
			debounceC = nil
			diagnostic, err := refreshDevGeneration(wuuHome, dir, packageManager)
			printDevDiagnostic(diagnostic)
			if errors.Is(err, errDevGenerationBusy) {
				schedule(pollInterval)
			}
		}
	}
}

// addDevWatchDirs registers root and every non-ignored subdirectory.
func addDevWatchDirs(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root && devWatchIgnoredDirs[entry.Name()] {
			return filepath.SkipDir
		}
		return watcher.Add(path)
	})
}

// devWatchIgnoresEvent filters events under ignored directories, judged
// relative to the watched root so an ancestor directory that happens to be
// named dist/node_modules above the plugin cannot suppress its own events.
func devWatchIgnoresEvent(root string, event fsnotify.Event) bool {
	rel, err := filepath.Rel(root, event.Name)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return devWatchPathIgnored(rel)
}

// devWatchPathIgnored reports whether rel (a path relative to the watched
// root) sits under an ignored directory. Pure helper for tests.
func devWatchPathIgnored(rel string) bool {
	parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
	for _, part := range parts {
		if devWatchIgnoredDirs[part] {
			return true
		}
	}
	return false
}
