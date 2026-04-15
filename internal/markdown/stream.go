package markdown

import (
	"strings"
)

// StreamCollector accumulates streaming text deltas and returns the
// raw text during streaming. Markdown is NOT parsed during streaming
// — this eliminates the O(n²) re-parse-on-every-token bottleneck.
// Markdown rendering happens once at Finalize() when the stream ends.
//
// Aligned with Claude Code's approach: streaming text is displayed as
// plain text; final message is parsed once and cached.
type StreamCollector struct {
	buffer strings.Builder
	width  int
	styles Styles
	// dirty tracks whether new content was pushed since last Commit.
	dirty bool
}

// NewStreamCollector creates a new collector for streaming markdown.
func NewStreamCollector(width int, styles Styles) *StreamCollector {
	return &StreamCollector{
		width:  width,
		styles: styles,
	}
}

// Push appends a delta to the buffer.
func (c *StreamCollector) Push(delta string) {
	c.buffer.WriteString(delta)
	c.dirty = true
}

// Dirty reports whether new content was pushed since the last Commit.
func (c *StreamCollector) Dirty() bool {
	return c.dirty
}

// Commit returns the full accumulated text as-is (no markdown parse)
// and clears the dirty flag. Returns "" only when the buffer is empty.
// The raw text is displayed directly during streaming — users see
// words appear immediately without any rendering overhead.
func (c *StreamCollector) Commit() string {
	c.dirty = false
	src := c.buffer.String()
	if src == "" {
		return ""
	}
	return src
}

// Finalize renders the complete buffer through the markdown pipeline
// and resets state. Called once when the stream ends (EventDone).
// This is the ONLY point where goldmark is invoked — converting the
// 200 per-token parses to exactly 1.
func (c *StreamCollector) Finalize() string {
	src := c.buffer.String()
	if src == "" {
		c.buffer.Reset()
		return ""
	}
	if !strings.HasSuffix(src, "\n") {
		src += "\n"
	}

	rendered := Render(src, c.width, c.styles)
	c.buffer.Reset()
	c.dirty = false
	return strings.TrimRight(rendered, "\n")
}
