package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscover_EmptyDirs(t *testing.T) {
	files := Discover("", "")
	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
}

func TestDiscover_UserOnly(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "CLAUDE.md"), []byte("user prefs"), 0o644); err != nil {
		t.Fatal(err)
	}

	files := Discover("", home)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Source != "user" {
		t.Errorf("expected source=user, got %q", files[0].Source)
	}
	if files[0].Content != "user prefs" {
		t.Errorf("unexpected content: %q", files[0].Content)
	}
}

func TestDiscover_ProjectHierarchy(t *testing.T) {
	// Build: tmp/a/b/c with CLAUDE.md at a and at c.
	root := t.TempDir()
	a := filepath.Join(root, "a")
	b := filepath.Join(a, "b")
	c := filepath.Join(b, "c")
	if err := os.MkdirAll(c, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a, "CLAUDE.md"), []byte("a-rules"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c, "AGENTS.md"), []byte("c-agents"), 0o644); err != nil {
		t.Fatal(err)
	}

	files := Discover(c, "")
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d: %+v", len(files), files)
	}
	// Highest ancestor first → "a-rules" should appear before "c-agents".
	if files[0].Content != "a-rules" {
		t.Errorf("expected first file = a-rules, got %q", files[0].Content)
	}
	if files[1].Content != "c-agents" {
		t.Errorf("expected second file = c-agents, got %q", files[1].Content)
	}
}

func TestDiscover_StopsAtHome(t *testing.T) {
	home := t.TempDir()
	proj := filepath.Join(home, "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	// Place a CLAUDE.md in $HOME itself — it should NOT be picked up
	// by the project walk (would be a user-style file).
	if err := os.WriteFile(filepath.Join(home, "CLAUDE.md"), []byte("home-root"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "CLAUDE.md"), []byte("proj-rules"), 0o644); err != nil {
		t.Fatal(err)
	}

	files := Discover(proj, home)
	// Only proj/CLAUDE.md should appear; the home-level walk is bounded.
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d: %+v", len(files), files)
	}
	if files[0].Content != "proj-rules" {
		t.Errorf("got %q", files[0].Content)
	}
}

func TestDiscover_BothFilesAtSameLevel(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("c-content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("a-content"), 0o644); err != nil {
		t.Fatal(err)
	}

	files := Discover(root, "")
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	// CLAUDE.md is read first (per memoryFileNames order).
	if files[0].Name != "CLAUDE.md" || files[1].Name != "AGENTS.md" {
		t.Errorf("unexpected order: %s, %s", files[0].Name, files[1].Name)
	}
}

func TestDiscover_Deduplication(t *testing.T) {
	// If rootDir == ~/.claude (unusual), the user file should not be
	// duplicated.
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("only-once"), 0o644); err != nil {
		t.Fatal(err)
	}

	files := Discover(claudeDir, home)
	count := 0
	for _, f := range files {
		if strings.Contains(f.Path, ".claude") && f.Name == "CLAUDE.md" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 .claude/CLAUDE.md, got %d (files: %+v)", count, files)
	}
}
