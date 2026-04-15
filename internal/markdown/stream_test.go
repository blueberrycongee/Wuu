package markdown

import (
	"strings"
	"testing"
)

func TestStreamCollector_RawDuringStreaming(t *testing.T) {
	c := NewStreamCollector(80, DefaultStyles())

	// Push partial line — Commit returns raw text immediately.
	c.Push("Hello ")
	if !c.Dirty() {
		t.Fatal("expected dirty after Push")
	}
	out := c.Commit()
	if out != "Hello " {
		t.Fatalf("expected raw 'Hello ', got %q", out)
	}
	if c.Dirty() {
		t.Fatal("expected clean after Commit")
	}

	// Push more — full accumulated text returned.
	c.Push("world")
	out = c.Commit()
	if out != "Hello world" {
		t.Fatalf("expected 'Hello world', got %q", out)
	}
}

func TestStreamCollector_DirtyTracking(t *testing.T) {
	c := NewStreamCollector(80, DefaultStyles())
	if c.Dirty() {
		t.Fatal("new collector should not be dirty")
	}

	c.Push("x")
	if !c.Dirty() {
		t.Fatal("expected dirty after Push")
	}

	c.Commit()
	if c.Dirty() {
		t.Fatal("expected clean after Commit")
	}

	// No push → not dirty.
	c.Push("")
	// Empty push still sets dirty (by design — caller decides).
}

func TestStreamCollector_Finalize_RendersMarkdown(t *testing.T) {
	c := NewStreamCollector(80, DefaultStyles())
	c.Push("```go\npackage main\n```\n")

	// Commit returns raw (no markdown parsing).
	raw := c.Commit()
	if strings.Contains(raw, "│") {
		t.Fatalf("Commit should return raw text, got %q", raw)
	}

	// Finalize renders through goldmark.
	c.Push("") // re-seed since buffer was already read via Commit
	final := c.Finalize()
	if final == "" {
		t.Fatal("expected finalize output")
	}
	if !strings.Contains(final, "package") {
		t.Fatalf("expected code content in finalize, got %q", final)
	}
}

func TestStreamCollector_Finalize_NoTrailingNewline(t *testing.T) {
	c := NewStreamCollector(80, DefaultStyles())
	c.Push("No trailing newline")
	out := c.Finalize()
	if out == "" {
		t.Fatal("expected finalize output")
	}
	if !strings.Contains(out, "No trailing newline") {
		t.Fatalf("expected content in finalize, got %q", out)
	}
}

func TestStreamCollector_Finalize_Empty(t *testing.T) {
	c := NewStreamCollector(80, DefaultStyles())
	out := c.Finalize()
	if out != "" {
		t.Fatalf("expected empty for empty buffer, got %q", out)
	}
}

func TestStreamCollector_Commit_Empty(t *testing.T) {
	c := NewStreamCollector(80, DefaultStyles())
	out := c.Commit()
	if out != "" {
		t.Fatalf("expected empty for empty buffer, got %q", out)
	}
}

func TestStreamCollector_MultilineThenFinalize(t *testing.T) {
	c := NewStreamCollector(80, DefaultStyles())

	// Simulate streaming: multiple pushes.
	c.Push("Line one\n")
	c.Push("Line two\n")
	c.Push("Line three")

	// Raw output includes all text.
	raw := c.Commit()
	if !strings.Contains(raw, "Line one") || !strings.Contains(raw, "Line three") {
		t.Fatalf("expected all lines in raw, got %q", raw)
	}

	// Finalize renders markdown.
	final := c.Finalize()
	if !strings.Contains(final, "Line one") || !strings.Contains(final, "Line three") {
		t.Fatalf("expected all lines in final, got %q", final)
	}
}
