package appserver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blueberrycongee/wuu/internal/participant"
)

func TestReadParticipantMemoryUsesNotebookDirectoryOnly(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "MEMORY.md"), []byte("old flat memory\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := participant.Participant{Workspace: workspace}
	got, err := (&Server{}).readParticipantMemory(p)
	if err != nil {
		t.Fatalf("read participant memory: %v", err)
	}
	if got != "" {
		t.Fatalf("flat MEMORY.md must not be used, got %q", got)
	}

	notebook := filepath.Join(workspace, "memory")
	if err := os.MkdirAll(notebook, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notebook, "MEMORY.md"), []byte("- [Current](current.md) — current memory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = (&Server{}).readParticipantMemory(p)
	if err != nil {
		t.Fatalf("read participant notebook: %v", err)
	}
	if got != "- [Current](current.md) — current memory" {
		t.Fatalf("participant notebook = %q", got)
	}
}
