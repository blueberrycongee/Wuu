package jsonl

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestForEachLineHandlesLargeRecord(t *testing.T) {
	large := strings.Repeat("x", 3*1024*1024)
	input := large + "\nsmall\n"

	var got []string
	err := ForEachLine(strings.NewReader(input), func(line []byte) error {
		got = append(got, string(bytes.TrimSpace(line)))
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachLine: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(got))
	}
	if got[0] != large {
		t.Fatalf("unexpected large line length: got %d want %d", len(got[0]), len(large))
	}
	if got[1] != "small" {
		t.Fatalf("unexpected second line: %q", got[1])
	}
}

func TestForEachLineStopsEarly(t *testing.T) {
	input := "first\nsecond\nthird\n"
	var got []string

	err := ForEachLine(strings.NewReader(input), func(line []byte) error {
		got = append(got, string(bytes.TrimSpace(line)))
		if len(got) == 2 {
			return ErrStop
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachLine: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 lines before stop, got %d", len(got))
	}
	if got[0] != "first" || got[1] != "second" {
		t.Fatalf("unexpected lines before stop: %#v", got)
	}
}

func TestForEachLineReturnsCallbackError(t *testing.T) {
	want := errors.New("boom")
	err := ForEachLine(strings.NewReader("line\n"), func(_ []byte) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected callback error, got %v", err)
	}
}

func TestForEachLineEmptyInput(t *testing.T) {
	var count int
	err := ForEachLine(strings.NewReader(""), func(_ []byte) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachLine: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 lines for empty input, got %d", count)
	}
}

func TestForEachLineNoTrailingNewline(t *testing.T) {
	input := "first\nsecond"
	var got []string
	err := ForEachLine(strings.NewReader(input), func(line []byte) error {
		got = append(got, string(line))
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachLine: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(got))
	}
	if got[0] != "first\n" {
		t.Fatalf("first line: got %q, want %q", got[0], "first\n")
	}
	if got[1] != "second" {
		t.Fatalf("second line: got %q, want %q", got[1], "second")
	}
}

// countingReader wraps r and records how many bytes were actually read.
// Used to assert that ErrStop short-circuits I/O (not just iteration).
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// TestForEachLineErrStopLimitsIO verifies that returning ErrStop stops the
// underlying reader from being drained. The input is far larger than the
// implementation's internal buffer, so if the fast path were buffering the
// whole file this would read the entire thing.
func TestForEachLineErrStopLimitsIO(t *testing.T) {
	// 4 MiB of filler, well above any reasonable read chunk, with the bail
	// trigger on the very first line.
	input := "stop-here\n" + strings.Repeat("x", 4*1024*1024) + "\n"
	cr := &countingReader{r: strings.NewReader(input)}

	var seen int
	err := ForEachLine(cr, func(line []byte) error {
		seen++
		return ErrStop
	})
	if err != nil {
		t.Fatalf("ForEachLine: %v", err)
	}
	if seen != 1 {
		t.Fatalf("expected exactly 1 line before ErrStop, got %d", seen)
	}
	// Allow the fast path to have prefetched up to ~256 KiB (covers the
	// streaming chunk plus a generous margin); the old non-streaming ReadAll
	// path would read the full 4+ MiB here.
	const budget = 256 * 1024
	if cr.n > budget {
		t.Fatalf("ErrStop read too much: got %d bytes, budget %d", cr.n, budget)
	}
}
