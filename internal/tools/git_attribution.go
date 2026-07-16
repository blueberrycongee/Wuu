package tools

import (
	"os"

	"github.com/blueberrycongee/wuu/internal/gitattribution"
)

func (e *Env) gitAttributionEnabled() bool {
	return e == nil || !e.GitAttributionDisabled
}

func (e *Env) gitAttributionShellPrefix() (string, error) {
	executable := ""
	stateDir := ""
	if e != nil {
		executable = e.GitWrapperExecutable
		stateDir = e.SessionDir
	}
	if executable == "" {
		resolved, err := os.Executable()
		if err != nil {
			return "", err
		}
		executable = resolved
	}
	binDir, err := gitattribution.EnsureWrapper(executable, stateDir)
	if err != nil {
		return "", err
	}
	return gitattribution.ShellPrefix(binDir), nil
}
