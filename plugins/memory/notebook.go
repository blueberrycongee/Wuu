package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	indexFileName     = "MEMORY.md"
	maxIndexLines     = 200
	maxIndexBytes     = 25_000
	removedLineNotice = "[memory line removed: security]"
)

var indexThreatPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore\s+(previous|all|above|prior)\s+instructions`),
	regexp.MustCompile(`(?i)you\s+are\s+now\s+`),
	regexp.MustCompile(`(?i)do\s+not\s+tell\s+the\s+user`),
	regexp.MustCompile(`(?i)system\s+prompt\s+override`),
	regexp.MustCompile(`(?i)disregard\s+(your|all|any)\s+(instructions|rules|guidelines)`),
	regexp.MustCompile(`(?i)act\s+as\s+(if|though)\s+you\s+(have\s+no|don'?t\s+have)\s+(restrictions|limits|rules)`),
	regexp.MustCompile(`(?i)(curl|wget)\s+[^\n]*\$\{?\w*(KEY|TOKEN|SECRET|PASSWORD|CREDENTIAL|API)`),
	regexp.MustCompile(`(?i)cat\s+[^\n]*(\.env|credentials|\.netrc|\.pgpass|\.npmrc|\.pypirc)`),
	regexp.MustCompile(`(?i)authorized_keys|\$HOME/\.ssh|~/\.ssh|\$HOME/\.wuu/\.env|~/\.wuu/\.env`),
}

var indexInvisibleChars = []rune{
	'\u200b', '\u200c', '\u200d', '\u2060', '\ufeff',
	'\u202a', '\u202b', '\u202c', '\u202d', '\u202e',
}

type indexSnapshot struct {
	Content string
}

func userNotebook(wuuHome string) string {
	home := strings.TrimSpace(wuuHome)
	if home == "" {
		return ""
	}
	return filepath.Join(home, "memory")
}

func ensureNotebook(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return errorsNewNotebookPath()
	}
	return os.MkdirAll(dir, 0o755)
}

func errorsNewNotebookPath() error {
	return fmt.Errorf("memory: notebook path is required")
}

func readSafeIndex(dir string) (indexSnapshot, error) {
	raw, err := os.ReadFile(filepath.Join(dir, indexFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return indexSnapshot{}, nil
		}
		return indexSnapshot{}, fmt.Errorf("memory: read index: %w", err)
	}
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		if unsafeIndexLine(line) {
			lines[i] = removedLineNotice
		}
	}
	trimmed := strings.TrimSpace(strings.Join(lines, "\n"))
	if trimmed == "" {
		return indexSnapshot{}, nil
	}
	allLines := strings.Split(trimmed, "\n")
	lineTruncated := len(allLines) > maxIndexLines
	byteTruncated := len(trimmed) > maxIndexBytes
	if lineTruncated {
		trimmed = strings.Join(allLines[:maxIndexLines], "\n")
	}
	if len(trimmed) > maxIndexBytes {
		if cut := strings.LastIndexByte(trimmed[:maxIndexBytes], '\n'); cut > 0 {
			trimmed = trimmed[:cut]
		} else {
			cut := maxIndexBytes
			for cut > 0 && !utf8.RuneStart(trimmed[cut]) {
				cut--
			}
			trimmed = trimmed[:cut]
		}
	}
	if lineTruncated || byteTruncated {
		trimmed += "\n\n> WARNING: MEMORY.md was truncated before prompt injection. Keep index entries short and move detail into topic files."
	}
	return indexSnapshot{Content: trimmed}, nil
}

func unsafeIndexLine(line string) bool {
	for _, char := range indexInvisibleChars {
		if strings.ContainsRune(line, char) {
			return true
		}
	}
	for _, pattern := range indexThreatPatterns {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}
