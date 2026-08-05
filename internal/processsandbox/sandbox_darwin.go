//go:build darwin

package processsandbox

import (
	"fmt"
	"os"
	"os/exec"
)

const seatbeltExecutable = "/usr/bin/sandbox-exec"

func platformSupported() bool { return true }

func applyPlatform(cmd *exec.Cmd, policy Policy) error {
	info, err := os.Stat(seatbeltExecutable)
	if err != nil {
		return fmt.Errorf("%w: inspect %s: %v", ErrUnavailable, seatbeltExecutable, err)
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		return fmt.Errorf("%w: %s is not executable", ErrUnavailable, seatbeltExecutable)
	}

	originalPath := cmd.Path
	originalArgs := append([]string(nil), cmd.Args...)
	if originalPath == "" || len(originalArgs) == 0 {
		return fmt.Errorf("filesystem process sandbox requires a complete command argv")
	}
	args := []string{seatbeltExecutable, "-p", seatbeltProfile(policy), "--", originalPath}
	args = append(args, originalArgs[1:]...)
	cmd.Path = seatbeltExecutable
	cmd.Args = args
	return nil
}
