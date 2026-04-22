//go:build zig

package jsonl

// #cgo CFLAGS: -I${SRCDIR}/zig
// #cgo LDFLAGS: ${SRCDIR}/zig/zig-out/lib/libjsonl.a
// #include "jsonl.h"
import "C"
import (
	"errors"
	"io"
	"unsafe"
)

const (
	// readChunk is the per-Read chunk size. Large enough to amortize cgo
	// transitions and give the SIMD loop room to run, small enough that peak
	// memory stays bounded even on huge session files.
	readChunk = 64 * 1024

	// initialOffsetsCap covers typical JSONL where the smallest record is at
	// least ~8 bytes. If underestimated the scanner is re-run once with an
	// exact-sized buffer, so this value only affects the allocation ceiling
	// in the happy path.
	initialOffsetsCap = readChunk/8 + 64
)

// ForEachLineZig is the Zig SIMD-backed streaming implementation. Kept
// exported under -tags zig so tests and benchmarks can call it directly.
func ForEachLineZig(r io.Reader, fn func(line []byte) error) error {
	return forEachLineFast(r, fn)
}

// forEachLineFast streams data from r through a rolling buffer, delegating
// newline discovery to jsonl_scan_lines (Zig SIMD). Complete lines are
// dispatched to fn immediately; incomplete tails are shifted to the start of
// the buffer and joined with the next Read. This preserves bufio's early-exit
// and bounded-memory properties while still benefiting from SIMD scanning.
//
// Memory envelope: O(readChunk + longest line). cgo overhead: one call per
// ~readChunk of input, negligible relative to the scan itself.
func forEachLineFast(r io.Reader, fn func(line []byte) error) error {
	buf := make([]byte, 0, readChunk)
	offsets := make([]C.size_t, initialOffsetsCap)

	var (
		eof      bool
		idleHits int
	)
	for !eof {
		if cap(buf) == len(buf) {
			// Either the buffer is pristine and needs its initial capacity,
			// or it currently holds a single line longer than cap; either way
			// give Read somewhere to write.
			grown := make([]byte, len(buf), cap(buf)*2)
			copy(grown, buf)
			buf = grown
		}
		n, err := r.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]
		// A well-behaved Reader returning (0, nil) repeatedly would otherwise
		// trap us in an unbounded loop. Surface it as ErrNoProgress after
		// 100 consecutive ticks — identical to bufio.Reader's threshold, so
		// exotic readers (TLS handshake, slow gateways) that legitimately
		// idle for a few calls are not false-positived.
		if n == 0 && err == nil {
			idleHits++
			if idleHits >= 100 {
				return io.ErrNoProgress
			}
		} else {
			idleHits = 0
		}
		if errors.Is(err, io.EOF) {
			eof = true
		} else if err != nil {
			// NOTE: per io.Reader contract a non-nil, non-EOF error may be
			// returned alongside n>0. We already appended those bytes into
			// buf, but we return before dispatching — intentional: it keeps
			// the error surfaces aligned with ForEachLineNative (bufio
			// drops the tail on error too). Losing trailing bytes on an
			// error path is acceptable; consistency matters more.
			return err
		}

		if len(buf) == 0 {
			continue
		}

		actual := C.jsonl_scan_lines(
			(*C.uint8_t)(unsafe.Pointer(&buf[0])),
			C.size_t(len(buf)),
			&offsets[0],
			C.size_t(len(offsets)),
		)
		if int(actual) > len(offsets) {
			offsets = make([]C.size_t, int(actual)+64)
			actual = C.jsonl_scan_lines(
				(*C.uint8_t)(unsafe.Pointer(&buf[0])),
				C.size_t(len(buf)),
				&offsets[0],
				C.size_t(len(offsets)),
			)
		}

		// offsets[0..actual-1] are line-start positions. Pairs (i, i+1) are
		// complete lines; offsets[actual-1] is the start of a possibly
		// incomplete tail we keep for the next chunk (or emit as the final
		// line at EOF if non-empty).
		end := int(actual) - 1
		for i := 0; i < end; i++ {
			line := buf[int(offsets[i]):int(offsets[i+1])]
			if cbErr := fn(line); cbErr != nil {
				if errors.Is(cbErr, ErrStop) {
					return nil
				}
				return cbErr
			}
		}

		tailStart := int(offsets[actual-1])
		if !eof {
			if tailStart > 0 {
				copy(buf, buf[tailStart:])
				buf = buf[:len(buf)-tailStart]
			}
			continue
		}

		if tailStart < len(buf) {
			line := buf[tailStart:]
			if cbErr := fn(line); cbErr != nil {
				if errors.Is(cbErr, ErrStop) {
					return nil
				}
				return cbErr
			}
		}
	}
	return nil
}
