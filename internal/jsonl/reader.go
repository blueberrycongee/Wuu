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
// This implementation uses a Zig SIMD scanner via cgo for performance.
func ForEachLine(r io.Reader, fn func(line []byte) error) error {
	return ForEachLineZig(r, fn)
}

// ForEachLineNative is the original Go implementation, kept for benchmarks
// and as a fallback if the Zig build is unavailable.
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
