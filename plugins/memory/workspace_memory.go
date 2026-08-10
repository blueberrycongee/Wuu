package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
)

const sessionMemoryReadMaxBytes = 64 * 1024

const sessionMemoryWriteMaxBytes = 256 * 1024

var sessionMemoryTargets = []string{"project_memory", "summary", "checkpoint", "notes"}

type sessionMemoryArgs struct {
	Action  string `json:"action"`
	Target  string `json:"target,omitempty"`
	Content string `json:"content,omitempty"`
	Source  string `json:"source,omitempty"`
}

func sessionMemorySchema() map[string]any {
	return objectSchema(map[string]any{
		"action":  map[string]any{"type": "string", "enum": []string{"status", "read", "append", "replace"}},
		"target":  map[string]any{"type": "string", "enum": sessionMemoryTargets},
		"content": stringField("Markdown content for append or replace."),
		"source":  stringField("Optional source label such as agent, dream, workflow, or user."),
	}, "action")
}

func (c *controller) executeSessionMemory(call pluginapi.ToolCall) (pluginapi.ToolResult, error) {
	var args sessionMemoryArgs
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return pluginapi.ToolResult{}, err
	}
	threadID := strings.TrimSpace(call.SessionID)
	if threadID == "" {
		threadID = strings.TrimSpace(call.ThreadID)
	}
	result, err := c.runSessionMemory(args, threadID)
	if err != nil {
		return pluginapi.ToolResult{}, err
	}
	return jsonResult(result, nil)
}

// runSessionMemory is the shared backend behind the session_memory tool and
// the memory.session service. An empty threadID restricts the operation to
// the workspace-scoped project_memory target.
func (c *controller) runSessionMemory(args sessionMemoryArgs, threadID string) (map[string]any, error) {
	var paths map[string]string
	if strings.TrimSpace(threadID) == "" {
		projectPath, err := c.projectMemoryPath()
		if err != nil {
			return nil, err
		}
		paths = map[string]string{"project_memory": projectPath}
	} else {
		var err error
		paths, err = c.sessionMemoryPaths(threadID)
		if err != nil {
			return nil, err
		}
	}
	switch strings.TrimSpace(args.Action) {
	case "status":
		files := make([]map[string]any, 0, len(sessionMemoryTargets))
		for _, target := range sessionMemoryTargets {
			path := paths[target]
			entry := map[string]any{"target": target, "path": path, "exists": false, "bytes": 0}
			if info, statErr := os.Stat(path); statErr == nil {
				entry["exists"], entry["bytes"] = true, info.Size()
			} else if !os.IsNotExist(statErr) {
				return nil, statErr
			}
			files = append(files, entry)
		}
		return map[string]any{"action": "status", "files": files}, nil
	case "read":
		target, path, err := checkedSessionMemoryTarget(args.Target, paths)
		if err != nil {
			return nil, err
		}
		content, readErr := os.ReadFile(path)
		exists := readErr == nil
		if readErr != nil && !os.IsNotExist(readErr) {
			return nil, readErr
		}
		truncated := false
		if len(content) > sessionMemoryReadMaxBytes {
			content = []byte(headTailText(string(content), sessionMemoryReadMaxBytes/2, sessionMemoryReadMaxBytes/2, "\n\n[trimmed session memory]\n\n"))
			truncated = true
		}
		return map[string]any{"action": "read", "target": target, "path": path, "exists": exists, "content": string(content), "truncated": truncated}, nil
	case "append", "replace":
		target, path, err := checkedSessionMemoryTarget(args.Target, paths)
		if err != nil {
			return nil, err
		}
		content := strings.TrimSpace(args.Content)
		if content == "" {
			return nil, errors.New("session_memory content is required")
		}
		if unsafeMemoryContent(content) {
			return nil, errors.New("session_memory content failed the prompt-injection safety scan")
		}
		if len([]byte(content)) > sessionMemoryWriteMaxBytes {
			return nil, fmt.Errorf("session_memory content exceeds %d bytes", sessionMemoryWriteMaxBytes)
		}
		release, err := acquireSessionMemoryLock(path)
		if err != nil {
			return nil, err
		}
		defer release()
		var next string
		if args.Action == "append" {
			previous, readErr := os.ReadFile(path)
			if readErr != nil && !os.IsNotExist(readErr) {
				return nil, readErr
			}
			existing := strings.TrimSpace(string(previous))
			if existing == "" {
				existing = sessionMemoryTemplate(target)
			}
			source := strings.TrimSpace(args.Source)
			if source == "" {
				source = "agent"
			}
			header := time.Now().UTC().Format(time.RFC3339) + " (" + source + ")"
			next = strings.TrimRight(existing, "\n") + "\n\n## " + header + "\n\n" + content + "\n"
		} else {
			next = content + "\n"
		}
		if err := writeAtomicFile(path, []byte(strings.TrimSpace(next)+"\n")); err != nil {
			return nil, err
		}
		return map[string]any{"action": args.Action, "target": target, "path": path, "written": true, "length": len(next)}, nil
	default:
		return nil, errors.New("session_memory action must be status, read, append, or replace")
	}
}

// projectMemoryPath locates the workspace-scoped project memory file without
// requiring a session id; used by thread-agnostic memory.session calls.
func (c *controller) projectMemoryPath() (string, error) {
	c.mu.Lock()
	stateDir := strings.TrimSpace(c.workspaceStateDir)
	c.mu.Unlock()
	if stateDir == "" {
		return "", errors.New("memory plugin requires workspace_state_dir")
	}
	return filepath.Join(stateDir, "memory", "MEMORY.md"), nil
}

func (c *controller) sessionMemoryPaths(threadID string) (map[string]string, error) {
	c.mu.Lock()
	stateDir := strings.TrimSpace(c.workspaceStateDir)
	c.mu.Unlock()
	if stateDir == "" {
		return nil, errors.New("memory plugin requires workspace_state_dir")
	}
	threadID = strings.TrimSpace(threadID)
	if threadID == "" || filepath.Base(threadID) != threadID || threadID == "." || threadID == ".." {
		return nil, errors.New("memory plugin requires a safe session id")
	}
	sessionRoot := filepath.Join(stateDir, "sessions", threadID)
	return map[string]string{
		"project_memory": filepath.Join(stateDir, "memory", "MEMORY.md"),
		"summary":        filepath.Join(sessionRoot, "session-memory", "summary.md"),
		"checkpoint":     filepath.Join(sessionRoot, "memory", "checkpoint.md"),
		"notes":          filepath.Join(sessionRoot, "memory", "notes.md"),
	}, nil
}

func checkedSessionMemoryTarget(target string, paths map[string]string) (string, string, error) {
	target = strings.TrimSpace(target)
	path, ok := paths[target]
	if !ok {
		return "", "", fmt.Errorf("unknown session_memory target %q", target)
	}
	return target, path, nil
}

func unsafeMemoryContent(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		if unsafeIndexLine(line) {
			return true
		}
	}
	return false
}

func writeAtomicFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".memory-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}

func sessionMemoryTemplate(target string) string {
	switch target {
	case "project_memory":
		return "# Project Memory\n\nDurable workspace facts that should survive across sessions.\n\n## Project Context\n\n## Rules\n\n## Architecture Decisions\n\n## Discovered Durable Knowledge\n"
	case "summary", "checkpoint":
		title := "Session Summary"
		if target == "checkpoint" {
			title = "Session Checkpoint"
		}
		return "# " + title + "\n\nCompact recoverable state for the active session.\n\n## Active Intent\n\n## Next Action\n\n## Current Work\n\n## Decisions\n\n## Open Questions\n"
	case "notes":
		return "# Session Notes\n\nScratch notes for this session. Promote durable facts to project memory or user memory.\n"
	default:
		return ""
	}
}

func headTailText(content string, headBytes, tailBytes int, marker string) string {
	if len(content) <= headBytes+tailBytes {
		return content
	}
	headEnd := headBytes
	for headEnd > 0 && !utf8.RuneStart(content[headEnd]) {
		headEnd--
	}
	tailStart := len(content) - tailBytes
	for tailStart < len(content) && !utf8.RuneStart(content[tailStart]) {
		tailStart++
	}
	return content[:headEnd] + marker + content[tailStart:]
}

func acquireSessionMemoryLock(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create memory directory: %w", err)
	}
	release, err := lockSessionMemoryFile(path + ".lock")
	if err != nil {
		return nil, fmt.Errorf("lock memory target: %w", err)
	}
	return release, nil
}
