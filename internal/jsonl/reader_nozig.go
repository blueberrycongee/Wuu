//go:build !zig

package jsonl

import "io"

// forEachLineFast is the pure-Go dispatch target when the Zig SIMD scanner is
// not compiled in. Keeps the zero-cgo default build fully portable.
func forEachLineFast(r io.Reader, fn func(line []byte) error) error {
	return ForEachLineNative(r, fn)
}
