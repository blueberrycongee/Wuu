package tools

import (
	"strings"
	"unicode"
)

const (
	wuuGitCoauthorEmail   = "305930189+wuu-agent[bot]@users.noreply.github.com"
	wuuGitCoauthorTrailer = "Co-authored-by: WUU Agent <" + wuuGitCoauthorEmail + ">"
)

func (e *Env) gitAttributionEnabled() bool {
	return e == nil || !e.GitAttributionDisabled
}

// appendWuuGitCommitTrailer adds Git's native --trailer option to each
// top-level git commit segment. It preserves the original author, committer,
// message, and any existing co-author trailers; Git de-duplicates an identical
// trailer when it is already present.
func appendWuuGitCommitTrailer(command string) string {
	var out strings.Builder
	segmentStart := 0
	inSingle := false
	inDouble := false
	escaped := false

	for index, char := range command {
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && !inSingle {
			escaped = true
			continue
		}
		if char == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if char == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if !inSingle && !inDouble && strings.ContainsRune("\n;&|()`", char) {
			out.WriteString(appendWuuTrailerToShellSegment(command[segmentStart:index]))
			out.WriteRune(char)
			segmentStart = index + len(string(char))
		}
	}
	out.WriteString(appendWuuTrailerToShellSegment(command[segmentStart:]))
	return out.String()
}

func appendWuuTrailerToShellSegment(segment string) string {
	fields, ok := splitShellFields(segment)
	if !ok {
		return segment
	}
	normalized := normalizeShellCommandFields(fields)
	if len(normalized) < 2 || shellCommandBaseName(normalized[0]) != "git" || normalized[1] != "commit" {
		return segment
	}

	trimmed := strings.TrimRightFunc(segment, unicode.IsSpace)
	trailing := segment[len(trimmed):]
	return trimmed + " --trailer " + shellSingleQuote(wuuGitCoauthorTrailer) + trailing
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
