//go:build windows

package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// startPTYProcess: TTY-mode managed processes are not supported on
// Windows. Callers fall back to pipe mode, which covers every managed
// process the agent starts itself.
func startPTYProcess(cmd *exec.Cmd) (*os.File, error) {
	return nil, errors.New("tty processes are not supported on windows: start the process without tty to use pipe mode")
}

// configureProcessGroup keeps the child addressable and invisible: a new
// process group (so console control events cannot propagate back into the
// daemon) and no flashing console window. Tree termination itself is
// taskkill /T, which walks parent links rather than groups.
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}

// lookupProcessGroup: Windows has no POSIX process groups; records store
// the pid so every group operation resolves back to the tree root.
func lookupProcessGroup(pid int) int {
	return pid
}

// terminateProcessGroup asks the tree rooted at pid to exit. Console
// processes rarely honor the graceful form; the caller escalates to
// killProcessGroup after its grace window, so failure here only matters
// when the tree also still exists.
func terminateProcessGroup(pid int) error {
	return taskkillTree(pid, false)
}

// killProcessGroup force-kills the tree rooted at pid.
func killProcessGroup(pid int) error {
	return taskkillTree(pid, true)
}

func taskkillTree(pid int, force bool) error {
	if pid <= 1 {
		return nil
	}
	args := []string{"/pid", strconv.Itoa(pid), "/t"}
	if force {
		args = append(args, "/f")
	}
	cmd := exec.Command("taskkill", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	// The tree being gone already is success. taskkill reports that as
	// exit code 128, but the code is not contractual — re-checking the
	// root is what actually decides.
	if !processExists(pid) {
		return nil
	}
	if !force {
		// Graceful termination is best-effort on Windows: console
		// processes have no window to deliver WM_CLOSE to, and the
		// caller escalates to /f next.
		return nil
	}
	return fmt.Errorf("taskkill pid %d: %v: %s", pid, err, strings.TrimSpace(string(out)))
}

// verifyProcessGroup: no POSIX groups on Windows, so group identity
// carries no anti-reuse signal; the process start-time identity check in
// processMatchesRecord is the reuse guard.
func verifyProcessGroup(pid, pgid int) (bool, error) {
	return true, nil
}

func processExists(pid int) bool {
	if pid <= 1 {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// Access denied still proves existence, matching the unix
		// EPERM contract.
		return errors.Is(err, windows.ERROR_ACCESS_DENIED)
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return true
	}
	return exitCode == uint32(windows.STATUS_PENDING)
}
