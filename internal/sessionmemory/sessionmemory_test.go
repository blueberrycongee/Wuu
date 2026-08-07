package sessionmemory

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	wuucontext "github.com/blueberrycongee/wuu/internal/context"
)

func TestAppendReplaceReadAndStatus(t *testing.T) {
	workspaceState := filepath.Join(t.TempDir(), "workspace-state")
	sessionArtifact := filepath.Join(workspaceState, "sessions", "session-1")
	now := time.Date(2026, 6, 12, 9, 30, 0, 0, time.UTC)

	path, n, err := AppendTarget(workspaceState, sessionArtifact, TargetProjectMemory, "Project uses make install for local CLI refresh.", "dream", now)
	if err != nil {
		t.Fatalf("AppendTarget: %v", err)
	}
	if n == 0 || path != filepath.Join(workspaceState, "memory", "MEMORY.md") {
		t.Fatalf("unexpected append result path=%q n=%d", path, n)
	}

	readPath, content, exists, err := ReadTarget(workspaceState, sessionArtifact, TargetProjectMemory)
	if err != nil {
		t.Fatalf("ReadTarget: %v", err)
	}
	if !exists || readPath != path {
		t.Fatalf("read exists/path mismatch exists=%t path=%q", exists, readPath)
	}
	for _, want := range []string{"# Project Memory", "2026-06-12T09:30:00Z (dream)", "Project uses make install"} {
		if !strings.Contains(content, want) {
			t.Fatalf("project memory missing %q:\n%s", want, content)
		}
	}

	replacePath, _, err := ReplaceTarget(workspaceState, sessionArtifact, TargetCheckpoint, "# Session Checkpoint\n\n## Active Intent\n\nShip memory support.")
	if err != nil {
		t.Fatalf("ReplaceTarget: %v", err)
	}
	if replacePath != filepath.Join(sessionArtifact, "memory", "checkpoint.md") {
		t.Fatalf("checkpoint path = %q", replacePath)
	}
	summaryPath, _, err := ReplaceTarget(workspaceState, sessionArtifact, TargetSummary, "# Session Summary\n\n## Active Intent\n\nShip memory support.")
	if err != nil {
		t.Fatalf("ReplaceTarget summary: %v", err)
	}
	if summaryPath != filepath.Join(sessionArtifact, "session-memory", "summary.md") {
		t.Fatalf("summary path = %q", summaryPath)
	}

	status, err := Status(workspaceState, sessionArtifact)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	byTarget := map[string]FileStatus{}
	for _, item := range status {
		byTarget[item.Target] = item
	}
	if !byTarget[TargetProjectMemory].Exists || byTarget[TargetProjectMemory].Bytes == 0 {
		t.Fatalf("project memory status missing: %+v", status)
	}
	if !byTarget[TargetCheckpoint].Exists || byTarget[TargetCheckpoint].Bytes == 0 {
		t.Fatalf("checkpoint status missing: %+v", status)
	}
	if !byTarget[TargetSummary].Exists || byTarget[TargetSummary].Bytes == 0 {
		t.Fatalf("summary status missing: %+v", status)
	}
	if byTarget[TargetNotes].Exists {
		t.Fatalf("notes should not exist yet: %+v", status)
	}
}

func TestAppendTargetSerializesAcrossProcesses(t *testing.T) {
	workspaceState := filepath.Join(t.TempDir(), "workspace-state")
	const processCount = 12

	type processResult struct {
		marker string
		output []byte
		err    error
	}
	start := make(chan struct{})
	results := make(chan processResult, processCount)
	markers := make([]string, processCount)
	for i := 0; i < processCount; i++ {
		marker := fmt.Sprintf("[session-memory-marker-%02d]", i)
		markers[i] = marker
		go func() {
			<-start
			command := exec.Command(os.Args[0], "-test.run=^TestAppendTargetProcessHelper$")
			command.Env = append(
				os.Environ(),
				"WUU_SESSION_MEMORY_APPEND_HELPER=1",
				"WUU_SESSION_MEMORY_WORKSPACE="+workspaceState,
				"WUU_SESSION_MEMORY_MARKER="+marker,
			)
			output, err := command.CombinedOutput()
			results <- processResult{marker: marker, output: output, err: err}
		}()
	}
	close(start)
	for range processCount {
		result := <-results
		if result.err != nil {
			t.Errorf("append %s: %v\n%s", result.marker, result.err, result.output)
		}
	}
	if t.Failed() {
		return
	}

	path, content, exists, err := ReadTarget(workspaceState, "", TargetProjectMemory)
	if err != nil {
		t.Fatalf("ReadTarget: %v", err)
	}
	if !exists {
		t.Fatal("project memory was not written")
	}
	for _, marker := range markers {
		if count := strings.Count(content, marker); count != 1 {
			t.Fatalf("marker %s appears %d times, want exactly once\n%s", marker, count, content)
		}
	}

	lockPath := path + ".lock"
	before, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("stat target lock: %v", err)
	}
	if _, _, err := ReplaceTarget(workspaceState, "", TargetProjectMemory, "# Project Memory\n\nReplacement"); err != nil {
		t.Fatalf("ReplaceTarget: %v", err)
	}
	after, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("stat target lock after replace: %v", err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("target lock sidecar inode changed across writes")
	}
}

func TestAppendTargetProcessHelper(t *testing.T) {
	if os.Getenv("WUU_SESSION_MEMORY_APPEND_HELPER") != "1" {
		return
	}
	workspaceState := os.Getenv("WUU_SESSION_MEMORY_WORKSPACE")
	marker := os.Getenv("WUU_SESSION_MEMORY_MARKER")
	if _, _, err := AppendTarget(
		workspaceState,
		"",
		TargetProjectMemory,
		marker,
		"concurrency-test",
		time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC),
	); err != nil {
		t.Fatal(err)
	}
}

func TestRequestContextBlocksSkipsMissingAndInjectsExistingFiles(t *testing.T) {
	workspaceState := filepath.Join(t.TempDir(), "workspace-state")
	sessionArtifact := filepath.Join(workspaceState, "sessions", "session-1")

	if blocks := RequestContextBlocks(workspaceState, sessionArtifact); len(blocks) != 0 {
		t.Fatalf("missing memory files should not inject blocks: %+v", blocks)
	}

	if _, _, err := ReplaceTarget(workspaceState, sessionArtifact, TargetNotes, "# Session Notes\n\nRemember to verify workflow memory."); err != nil {
		t.Fatalf("ReplaceTarget notes: %v", err)
	}

	blocks := RequestContextBlocks(workspaceState, sessionArtifact)
	if len(blocks) != 1 {
		t.Fatalf("blocks len = %d, want 1: %+v", len(blocks), blocks)
	}
	if blocks[0].Kind != wuucontext.BlockTaskState {
		t.Fatalf("unexpected block kind: %+v", blocks)
	}
	if blocks[0].Source != "session.notes:notes" {
		t.Fatalf("unexpected block source: %+v", blocks)
	}
	if blocks[0].TokenBudget <= 0 {
		t.Fatalf("block missing token budget: %+v", blocks[0])
	}
}

func TestRequestContextBlocksExcludesProjectMemory(t *testing.T) {
	workspaceState := filepath.Join(t.TempDir(), "workspace-state")
	sessionArtifact := filepath.Join(workspaceState, "sessions", "session-1")

	if _, _, err := ReplaceTarget(workspaceState, sessionArtifact, TargetProjectMemory, "# Project Memory\n\nDurable project fact."); err != nil {
		t.Fatalf("ReplaceTarget project: %v", err)
	}
	if _, _, err := ReplaceTarget(workspaceState, sessionArtifact, TargetSummary, "# Session Summary\n\nActive task state."); err != nil {
		t.Fatalf("ReplaceTarget summary: %v", err)
	}
	if _, _, err := ReplaceTarget(workspaceState, sessionArtifact, TargetNotes, "# Session Notes\n\nScratch state."); err != nil {
		t.Fatalf("ReplaceTarget notes: %v", err)
	}

	blocks := RequestContextBlocks(workspaceState, sessionArtifact)
	if len(blocks) != 2 {
		t.Fatalf("request blocks len = %d, want 2: %+v", len(blocks), blocks)
	}
	sources := map[string]bool{}
	for _, block := range blocks {
		sources[block.Source] = true
		if block.Kind == wuucontext.BlockMemory || strings.Contains(block.Content, "Durable project fact") {
			t.Fatalf("request context should not include project memory: %+v", blocks)
		}
	}
	if !sources["session.summary:summary"] || !sources["session.notes:notes"] {
		t.Fatalf("request context missing session state sources: %+v", blocks)
	}
}

func TestRequestContextBlocksPrefersSummaryOverLegacyCheckpoint(t *testing.T) {
	workspaceState := filepath.Join(t.TempDir(), "workspace-state")
	sessionArtifact := filepath.Join(workspaceState, "sessions", "session-1")

	if _, _, err := ReplaceTarget(workspaceState, sessionArtifact, TargetCheckpoint, "# Session Checkpoint\n\nLegacy checkpoint."); err != nil {
		t.Fatalf("ReplaceTarget checkpoint: %v", err)
	}
	if _, _, err := ReplaceTarget(workspaceState, sessionArtifact, TargetSummary, "# Session Summary\n\nCurrent summary."); err != nil {
		t.Fatalf("ReplaceTarget summary: %v", err)
	}

	blocks := RequestContextBlocks(workspaceState, sessionArtifact)
	if len(blocks) != 1 {
		t.Fatalf("blocks len = %d, want 1: %+v", len(blocks), blocks)
	}
	if blocks[0].Source != "session.summary:summary" || !strings.Contains(blocks[0].Content, "Current summary") {
		t.Fatalf("summary block not injected: %+v", blocks[0])
	}
	if strings.Contains(blocks[0].Content, "Legacy checkpoint") {
		t.Fatalf("legacy checkpoint should not be injected when summary exists: %+v", blocks[0])
	}
}

func TestRejectsUnknownTargetAndEmptyContent(t *testing.T) {
	workspaceState := t.TempDir()
	sessionArtifact := filepath.Join(workspaceState, "sessions", "session-1")

	if _, _, err := AppendTarget(workspaceState, sessionArtifact, "unknown", "fact", "", time.Time{}); err == nil {
		t.Fatal("expected unknown target error")
	}
	if _, _, err := AppendTarget(workspaceState, sessionArtifact, TargetNotes, "   ", "", time.Time{}); err == nil {
		t.Fatal("expected empty content error")
	}

	paths := PathsFor(workspaceState, sessionArtifact)
	if _, err := os.Stat(paths.Notes); !os.IsNotExist(err) {
		t.Fatalf("empty append should not create notes file: %v", err)
	}
}
