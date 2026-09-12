package analysis

import (
	"bytes"
	"context"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
)

// OutputLimitEnv names the environment variable bounding what one external process may
// write: one reply or one protocol line on standard output, and the whole of standard
// error, for tools and engines alike. Bytes, or bytes with a `K`, `M` or `G` suffix.
const OutputLimitEnv = "OPENSYSML_TOOL_MAX_OUTPUT"

// DefaultOutputLimit is the bound when OutputLimitEnv is unset or unusable.
const DefaultOutputLimit = 64 << 20

// outputLimitFromEnv reads OutputLimitEnv, falling back to DefaultOutputLimit for an unset,
// unparsable or non-positive value.
func outputLimitFromEnv() int {
	text := strings.TrimSpace(os.Getenv(OutputLimitEnv))
	if text == "" {
		return DefaultOutputLimit
	}
	n, ok := parseByteSize(text)
	if !ok || n <= 0 {
		return DefaultOutputLimit
	}
	return n
}

// parseByteSize reads a size: a plain byte count, or one with a K, M or G suffix (binary).
// A size that does not fit an int is not a size.
func parseByteSize(text string) (int, bool) {
	shift := 0
	switch {
	case strings.HasSuffix(text, "G"):
		shift, text = 30, strings.TrimSuffix(text, "G")
	case strings.HasSuffix(text, "M"):
		shift, text = 20, strings.TrimSuffix(text, "M")
	case strings.HasSuffix(text, "K"):
		shift, text = 10, strings.TrimSuffix(text, "K")
	}
	n, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	if err != nil || n < 0 || n > math.MaxInt64>>shift || n<<shift > math.MaxInt {
		return 0, false
	}
	return int(n << shift), true
}

// boundedBuffer keeps the first limit bytes written to it and stops the process at the
// first byte beyond. It is a plain Writer so every byte passes through Write; mu lets a
// process's two streams and the reader that inspects them share one.
type boundedBuffer struct {
	mu    sync.Mutex
	kept  bytes.Buffer
	limit int
	over  bool
	stop  context.CancelFunc
}

// newBoundedBuffer is a buffer of limit bytes that calls stop when the limit is passed.
func newBoundedBuffer(limit int, stop context.CancelFunc) *boundedBuffer {
	return &boundedBuffer{limit: max(limit, 0), stop: stop}
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if room := b.limit - b.kept.Len(); len(p) > room {
		b.kept.Write(p[:room])
		b.over = true
		if b.stop != nil {
			b.stop()
		}
		return len(p), nil
	}
	return b.kept.Write(p)
}

// Bytes is a copy of what was kept.
func (b *boundedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return bytes.Clone(b.kept.Bytes())
}

// Over reports whether the limit was passed.
func (b *boundedBuffer) Over() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.over
}
