package hooks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/blueberrycongee/wuu/internal/config"
)

// ErrBlocked is returned when a hook exits with code 2, meaning the operation should be blocked.
var ErrBlocked = errors.New("operation blocked by hook")

// Run executes matching hook entries for the given tool name.
// Environment variables are passed to the hook command.
// Exit code 0 = success, 2 = block operation, other = error.
func Run(ctx context.Context, entries []config.HookEntry, toolName string, env map[string]string) error {
	for _, entry := range entries {
		if !matchesTool(entry.Tool, toolName) {
			continue
		}
		if err := runOne(ctx, entry.Command, env); err != nil {
			return err
		}
	}
	return nil
}

func matchesTool(pattern, toolName string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	return strings.EqualFold(pattern, toolName)
}

func runOne(ctx context.Context, command string, env map[string]string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 2 {
				return ErrBlocked
			}
			return fmt.Errorf("hook %q failed (exit %d): %s", command, exitErr.ExitCode(), strings.TrimSpace(string(output)))
		}
		return fmt.Errorf("hook %q failed: %w", command, err)
	}
	return nil
}
