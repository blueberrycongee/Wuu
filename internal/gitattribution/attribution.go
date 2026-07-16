package gitattribution

import "strings"

const (
	Email   = "305930189+wuu-agent[bot]@users.noreply.github.com"
	Trailer = "Co-authored-by: WUU Agent <" + Email + ">"

	internalCommand = "__wuu_internal_git_wrapper"
)

var globalOptionsWithValue = map[string]bool{
	"-C":             true,
	"-c":             true,
	"--config-env":   true,
	"--git-dir":      true,
	"--namespace":    true,
	"--super-prefix": true,
	"--work-tree":    true,
}

// AddToCommitArgs injects WUU attribution at Git's argv boundary. It leaves
// non-commit commands and amend operations unchanged.
func AddToCommitArgs(args []string) ([]string, bool) {
	subcommandIndex := findSubcommand(args)
	if subcommandIndex < 0 || args[subcommandIndex] != "commit" {
		return args, false
	}

	pathspecIndex := len(args)
	for index := subcommandIndex + 1; index < len(args); index++ {
		arg := args[index]
		if isAmendFlag(arg) {
			return args, false
		}
		switch arg {
		case "--":
			pathspecIndex = index
			index = len(args)
		}
	}

	config := []string{
		"-c", "trailer.Co-authored-by.ifexists=addIfDifferent",
	}
	out := make([]string, 0, len(args)+len(config)+2)
	out = append(out, args[:subcommandIndex]...)
	out = append(out, config...)
	out = append(out, args[subcommandIndex:pathspecIndex]...)
	out = append(out, "--trailer", Trailer)
	out = append(out, args[pathspecIndex:]...)
	return out, true
}

func isAmendFlag(arg string) bool {
	return len(arg) >= len("--am") && strings.HasPrefix("--amend", arg) ||
		strings.HasPrefix(arg, "--amend=")
}

func findSubcommand(args []string) int {
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if globalOptionsWithValue[arg] {
			if index+1 >= len(args) {
				return -1
			}
			index++
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return index
	}
	return -1
}
