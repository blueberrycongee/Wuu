// Package memory discovers and loads project / user memory files
// (CLAUDE.md and AGENTS.md) so they can be injected into the system
// prompt at session start. Behavior mirrors Claude Code:
//
//   - User-level: ~/.claude/CLAUDE.md and ~/.claude/AGENTS.md
//   - Project hierarchy: walking up from the workspace root, every
//     ancestor directory contributes any CLAUDE.md or AGENTS.md it
//     contains, ordered from highest ancestor down to the workspace
//     itself (more specific files appear later, so they have the most
//     influence on the model).
package memory

import (
	"os"
	"path/filepath"
)

// File holds one loaded memory file.
type File struct {
	Path    string // absolute path on disk
	Content string // raw file contents
	Source  string // "user" or "project"
	Name    string // base name (CLAUDE.md or AGENTS.md)
}

// memoryFileNames is the list of recognized memory file names. Order is
// stable so output is deterministic.
var memoryFileNames = []string{"CLAUDE.md", "AGENTS.md"}

// Discover scans both the user directory and the project hierarchy for
// memory files and returns them in priority order:
//
//  1. ~/.claude/CLAUDE.md, ~/.claude/AGENTS.md
//  2. Project ancestors from highest down to rootDir
//  3. rootDir itself
//
// Files are deduplicated by absolute path so a memory file under $HOME
// is never read twice. Missing files are silently skipped.
func Discover(rootDir, homeDir string) []File {
	var out []File
	seen := make(map[string]struct{})

	add := func(path, source string) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return
		}
		if _, ok := seen[abs]; ok {
			return
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return
		}
		seen[abs] = struct{}{}
		out = append(out, File{
			Path:    abs,
			Content: string(data),
			Source:  source,
			Name:    filepath.Base(abs),
		})
	}

	// 1. User-level files under ~/.claude/
	if homeDir != "" {
		for _, name := range memoryFileNames {
			add(filepath.Join(homeDir, ".claude", name), "user")
		}
	}

	// 2. Project hierarchy: collect ancestors of rootDir up to (but not
	// including) $HOME or filesystem root, then walk highest → lowest.
	if rootDir != "" {
		absRoot, err := filepath.Abs(rootDir)
		if err == nil {
			var dirs []string
			cur := absRoot
			for {
				// Stop before stepping into $HOME — we don't want to
				// pick up CLAUDE.md sitting in $HOME itself as if it
				// were a project ancestor.
				if homeDir != "" && cur == homeDir {
					break
				}
				dirs = append(dirs, cur)
				parent := filepath.Dir(cur)
				if parent == cur {
					break // filesystem root
				}
				cur = parent
			}
			// Reverse so highest ancestor first.
			for i, j := 0, len(dirs)-1; i < j; i, j = i+1, j-1 {
				dirs[i], dirs[j] = dirs[j], dirs[i]
			}
			for _, dir := range dirs {
				for _, name := range memoryFileNames {
					add(filepath.Join(dir, name), "project")
				}
			}
		}
	}

	return out
}
