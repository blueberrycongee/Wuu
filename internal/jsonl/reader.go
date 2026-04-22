package jsonl

import (
	"bufio"
	"errors"
	"io"
)

// ErrStop lets callbacks end iteration without treating it as a read failure.
var ErrStop = errors.New("jsonl: stop iteration")

// ForEachLine reads one logical line at a time from r and passes the raw line
// bytes to fn. The trailing newline, if present, is included in the slice.
// When built with -tags zig, it streams chunks through a Zig SIMD newline
// scanner via cgo; otherwise it falls back to a pure-Go bufio implementation.
// In both paths the slice passed to fn is only valid during the callback —
// copy it if you need to retain the bytes.
func ForEachLine(r io.Reader, fn func(line []byte) error) error {
	return forEachLineFast(r, fn)
}

// ForEachLineNative is the pure-Go bufio implementation. Exported so callers
// can force the native path (e.g. when running in constrained environments)
// and for side-by-side benchmarks.
func ForEachLineNative(r io.Reader, fn func(line []byte) error) error {
	reader := bufio.NewReader(r)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if cbErr := fn(line); cbErr != nil {
				if errors.Is(cbErr, ErrStop) {
					return nil
				}
				return cbErr
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
