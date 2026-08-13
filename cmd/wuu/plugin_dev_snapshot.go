package main

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// pluginSourceFile is the identity of one watched source file. ModTime alone
// is not enough: some editors preserve timestamps, so size is included as a
// cheap second signal.
type pluginSourceFile struct {
	ModTime time.Time
	Size    int64
}

// pluginSourceSnapshot maps slash-normalized package-relative paths to their
// current file identity. Ignored directories are excluded so build output and
// dependencies never show up as source changes.
type pluginSourceSnapshot map[string]pluginSourceFile

func snapshotPluginSource(root string) pluginSourceSnapshot {
	out := make(pluginSourceSnapshot)
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			if path != root && (devWatchIgnoredDirs[entry.Name()] || strings.HasPrefix(entry.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		out[filepath.ToSlash(rel)] = pluginSourceFile{ModTime: info.ModTime(), Size: info.Size()}
		return nil
	})
	return out
}

// changedPluginSourcePaths returns the sorted package-relative paths that were
// added, removed, or changed between two snapshots.
func changedPluginSourcePaths(before, after pluginSourceSnapshot) []string {
	var out []string
	for rel, current := range after {
		previous, ok := before[rel]
		if !ok || !previous.ModTime.Equal(current.ModTime) || previous.Size != current.Size {
			out = append(out, rel)
		}
	}
	for rel := range before {
		if _, ok := after[rel]; !ok {
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	return out
}
