package jsonl

import (
	"bytes"
	"errors"
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
