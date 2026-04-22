//go:build zig

package jsonl

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"testing"
)

// stubborn0ReaderZig never makes progress — returns (0, nil) forever. A bug
// in the streaming loop would turn this into an unbounded buffer grow + spin.
// We verify it's rejected with io.ErrNoProgress within a bounded wall time.
type stubborn0ReaderZig struct{ calls int }

func (s *stubborn0ReaderZig) Read(p []byte) (int, error) {
	s.calls++
	return 0, nil
}

func TestForEachLineZig_NoProgressTerminates(t *testing.T) {
	r := &stubborn0ReaderZig{}
	err := ForEachLineZig(r, func(_ []byte) error { return nil })
	if !errors.Is(err, io.ErrNoProgress) {
		t.Fatalf("expected io.ErrNoProgress, got %v", err)
	}
	// Threshold aligned with bufio.Reader (100). Allow a small slack.
	if r.calls < 100 || r.calls > 110 {
		t.Fatalf("guard tripped at unexpected read count: %d (want ~100)", r.calls)
	}
}

// TestForEachLineZig_ParityWithNative runs a batch of random inputs through
// both the streaming Zig path and the bufio path and asserts they yield the
// same sequence of line bytes. Guards against drift between the two
// implementations as either side evolves.
func TestForEachLineZig_ParityWithNative(t *testing.T) {
	rng := rand.New(rand.NewSource(0xC0FFEE))
	for trial := 0; trial < 50; trial++ {
		input := randomJSONLLike(rng)

		var native, zig [][]byte
		if err := ForEachLineNative(strings.NewReader(input), func(line []byte) error {
			native = append(native, append([]byte(nil), line...))
			return nil
		}); err != nil {
			t.Fatalf("trial %d native: %v", trial, err)
		}
		if err := ForEachLineZig(strings.NewReader(input), func(line []byte) error {
			zig = append(zig, append([]byte(nil), line...))
			return nil
		}); err != nil {
			t.Fatalf("trial %d zig: %v", trial, err)
		}

		if len(native) != len(zig) {
			t.Fatalf("trial %d: line count mismatch native=%d zig=%d", trial, len(native), len(zig))
		}
		for i := range native {
			if string(native[i]) != string(zig[i]) {
				t.Fatalf("trial %d line %d mismatch:\n native=%q\n zig=%q", trial, i, native[i], zig[i])
			}
		}
	}
}

// TestForEachLineZig_LineLongerThanChunk exercises the buffer-growth path:
// a single line whose length exceeds readChunk so we loop through multiple
// Reads without emitting anything, then finally dispatch the combined line.
func TestForEachLineZig_LineLongerThanChunk(t *testing.T) {
	long := strings.Repeat("a", readChunk*3+123)
	input := "short\n" + long + "\ntail\n"

	var got []string
	if err := ForEachLineZig(strings.NewReader(input), func(line []byte) error {
		got = append(got, string(line))
		return nil
	}); err != nil {
		t.Fatalf("ForEachLineZig: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 lines, got %d", len(got))
	}
	if got[0] != "short\n" || got[1] != long+"\n" || got[2] != "tail\n" {
		t.Fatalf("unexpected lines: %q / (len %d) / %q", got[0], len(got[1]), got[2])
	}
}

// TestForEachLineZig_NewlineAtChunkBoundary places a newline at exactly the
// chunk boundary to catch off-by-one errors in the tail-shift logic.
func TestForEachLineZig_NewlineAtChunkBoundary(t *testing.T) {
	// First line fills exactly readChunk bytes including the \n.
	first := strings.Repeat("a", readChunk-1) + "\n"
	input := first + "second\n"

	var got []string
	if err := ForEachLineZig(strings.NewReader(input), func(line []byte) error {
		got = append(got, string(line))
		return nil
	}); err != nil {
		t.Fatalf("ForEachLineZig: %v", err)
	}
	if len(got) != 2 || got[0] != first || got[1] != "second\n" {
		t.Fatalf("unexpected: %d lines, first=%d bytes, second=%q", len(got), len(got[0]), got[1])
	}
}

func randomJSONLLike(rng *rand.Rand) string {
	var sb strings.Builder
	nLines := rng.Intn(200)
	for i := 0; i < nLines; i++ {
		// Vary line length from 0 to ~2x readChunk so we routinely span
		// chunk boundaries and occasionally exercise the grow path.
		length := rng.Intn(readChunk*2 + 100)
		payload := fmt.Sprintf(`{"i":%d,"x":%q}`, i, strings.Repeat("y", length))
		sb.WriteString(payload)
		// 10% of the time, drop the trailing newline on the last line only.
		if i == nLines-1 && rng.Intn(10) == 0 {
			break
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}
