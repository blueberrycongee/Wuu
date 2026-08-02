package process

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// writeLeftoverRecord plants a registry row as if a previous app-server had
// started it and then died without settling it.
func writeLeftoverRecord(t *testing.T, registryDir string, p Process) {
	t.Helper()
	if err := os.MkdirAll(registryDir, 0o755); err != nil {
		t.Fatalf("mkdir registry: %v", err)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	if err := os.WriteFile(filepath.Join(registryDir, p.ID+".json"), raw, 0o644); err != nil {
		t.Fatalf("write record: %v", err)
	}
}

func recordByID(t *testing.T, m *Manager, id string) Process {
	t.Helper()
	list, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, p := range list {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("record %q not found in %+v", id, list)
	return Process{}
}

// A session command left behind by a crashed app-server used to sit at
// "running" forever: unreachable, but never settled.
func TestStartupRetiresLeftoverSessionRecordAsLost(t *testing.T) {
	runtimeDir := filepath.Join(t.TempDir(), "runtime")
	registryDir := filepath.Join(runtimeDir, "processes")
	writeLeftoverRecord(t, registryDir, Process{
		ID:               "leftover-1",
		OwnerKind:        OwnerMainAgent,
		OwnerID:          "main",
		RootThreadID:     "thread-1",
		HostGenerationID: "host-from-a-previous-run",
		Lifecycle:        LifecycleSession,
		Status:           StatusRunning,
		PID:              -1,
		Command:          "sleep 600",
		CWD:              t.TempDir(),
		StartedAt:        time.Now().Add(-time.Hour),
	})

	m, err := NewManager(t.TempDir(), runtimeDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = m.CleanupSession() })

	got := recordByID(t, m, "leftover-1")
	if got.Status != StatusLost {
		t.Fatalf("Status = %q, want lost", got.Status)
	}
	// The command may have exited cleanly, been killed, or still be detached.
	// Claiming a terminal cause would assert an exit nobody observed.
	if got.TerminalCause != "" {
		t.Fatalf("a lost record must not claim a terminal cause, got %q", got.TerminalCause)
	}
	if got.LossReason == "" || got.RecoveryCleanup == "" {
		t.Fatalf("a lost record must explain itself: %+v", got)
	}
}

// The correction on the previous step: several managers share one host
// lifetime. A sibling manager's live record is still controllable by the host
// and must survive startup untouched.
func TestStartupLeavesSameHostGenerationRecordsAlone(t *testing.T) {
	runtimeDir := filepath.Join(t.TempDir(), "runtime")
	first, err := NewManager(t.TempDir(), runtimeDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = first.CleanupSession() })

	writeLeftoverRecord(t, filepath.Join(runtimeDir, "processes"), Process{
		ID:               "sibling-1",
		OwnerKind:        OwnerMainAgent,
		OwnerID:          "main",
		HostGenerationID: first.HostGenerationID(),
		Lifecycle:        LifecycleSession,
		Status:           StatusRunning,
		PID:              -1,
		Command:          "sleep 600",
		CWD:              t.TempDir(),
		StartedAt:        time.Now(),
	})

	sibling, err := NewManagerWithHostGeneration(t.TempDir(), first.HostGenerationID(), runtimeDir)
	if err != nil {
		t.Fatalf("NewManagerWithHostGeneration: %v", err)
	}
	t.Cleanup(func() { _ = sibling.CleanupSession() })

	if got := recordByID(t, sibling, "sibling-1"); got.Status != StatusRunning {
		t.Fatalf("Status = %q, want running: a sibling manager in the same host must not retire it", got.Status)
	}
}

// Managed re-adoption across restarts is still in force. Retiring it belongs to
// a later step, so this reconciliation must not touch those records.
func TestStartupDoesNotRetireLeftoverManagedRecords(t *testing.T) {
	runtimeDir := filepath.Join(t.TempDir(), "runtime")
	writeLeftoverRecord(t, filepath.Join(runtimeDir, "processes"), Process{
		ID:               "managed-1",
		OwnerKind:        OwnerMainAgent,
		OwnerID:          "main",
		HostGenerationID: "host-from-a-previous-run",
		Lifecycle:        LifecycleManaged,
		Status:           StatusRunning,
		PID:              -1,
		Command:          "sleep 600",
		CWD:              t.TempDir(),
		StartedAt:        time.Now().Add(-time.Hour),
	})

	m, err := NewManager(t.TempDir(), runtimeDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = m.CleanupSession() })

	if got := recordByID(t, m, "managed-1"); got.Status == StatusLost {
		t.Fatalf("managed records are still re-adopted at startup; got %+v", got)
	}
}

// Cleanup is best effort and its outcome is reported separately from the loss
// reason. A live leftover whose identity checks out gets its tree terminated.
func TestStartupTerminatesVerifiedLeftoverProcessTree(t *testing.T) {
	runtimeDir := filepath.Join(t.TempDir(), "runtime")
	cmd := exec.Command("sleep", "600")
	// Isolate the helper into its own process group. Without this the recorded
	// group is the test runner's own, and cleanup would kill the test process.
	PrepareCommand(cmd)
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start helper process: %v", err)
	}
	pid := cmd.Process.Pid
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	identity, _, _, err := readProcessIdentity(pid)
	if err != nil {
		t.Skipf("process identity unavailable on this platform: %v", err)
	}
	writeLeftoverRecord(t, filepath.Join(runtimeDir, "processes"), Process{
		ID:               "leftover-live",
		OwnerKind:        OwnerMainAgent,
		OwnerID:          "main",
		HostGenerationID: "host-from-a-previous-run",
		Lifecycle:        LifecycleSession,
		Status:           StatusRunning,
		PID:              pid,
		PGID:             ProcessTreeForPID(pid).ID(),
		ProcessStartTime: identity,
		Command:          "sleep 600",
		CWD:              t.TempDir(),
		StartedAt:        time.Now().Add(-time.Hour),
	})

	m, err := NewManager(t.TempDir(), runtimeDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = m.CleanupSession() })

	got := recordByID(t, m, "leftover-live")
	if got.Status != StatusLost {
		t.Fatalf("Status = %q, want lost", got.Status)
	}
	if got.RecoveryCleanup != RecoveryTerminated {
		t.Fatalf("RecoveryCleanup = %q, want terminated", got.RecoveryCleanup)
	}
	// Cleanup succeeding does not tell this host how the command originally
	// ended, so the record stays lost rather than becoming a normal stop.
	if got.LossReason != LossHostRestarted {
		t.Fatalf("LossReason = %q, want host_restarted", got.LossReason)
	}
}
