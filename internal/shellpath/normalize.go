package shellpath

import (
	"regexp"
	"runtime"
)

// cmdNullRedirectRe matches cmd.exe-style redirection to the NUL device
// (2>nul, >nul, 1>>nul, 2> nul), keeping the redirection operator and the
// character that terminates the device name.
var cmdNullRedirectRe = regexp.MustCompile(`(?i)([0-9]?>>?[ \t]*)nul([^.\w]|$)`)

// NormalizeBashCommand rewrites cmd.exe-isms that Git Bash would
// misinterpret. Models emit "2>nul" from Windows muscle memory; under
// bash that creates a literal file named "nul" in the repository instead
// of discarding output. No-op outside Windows.
func NormalizeBashCommand(command string) string {
	if runtime.GOOS != "windows" {
		return command
	}
	return rewriteCmdNullRedirect(command)
}

func rewriteCmdNullRedirect(command string) string {
	return cmdNullRedirectRe.ReplaceAllString(command, "${1}/dev/null${2}")
}
