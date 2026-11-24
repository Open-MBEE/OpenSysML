// Package subprocess runs the short helper programs the tools call under a time and output bound.
package subprocess

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// ErrTimeout and ErrOutput are the two bounds a program can pass.
var (
	ErrTimeout = errors.New("subprocess exceeded its time limit")
	ErrOutput  = errors.New("subprocess exceeded its output limit")
)

// WaitDelay is how long a killed program's descendants may hold its pipes open.
const WaitDelay = 10 * time.Second

// Timeout is ctx bounded by timeout, where zero or less means no limit.
func Timeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}

// Output runs name and returns its streams, each held to limit bytes, within timeout.
func Output(timeout time.Duration, limit int, name string, args ...string) (stdout, stderr []byte, err error) {
	ctx, cancel := Timeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...) // #nosec G204 -- every caller names an operator-chosen or resolved program
	cmd.WaitDelay = WaitDelay
	out, errs := Stream(limit, cancel), Stream(limit, cancel)
	cmd.Stdout, cmd.Stderr = out, errs
	runErr := cmd.Run()
	stdout, stderr = out.Bytes(), errs.Bytes()
	switch {
	case out.Over() || errs.Over():
		return stdout, stderr, fmt.Errorf("%s: %w of %d bytes on one stream", name, ErrOutput, limit)
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return stdout, stderr, fmt.Errorf("%s: %w of %s", name, ErrTimeout, timeout)
	}
	return stdout, stderr, runErr
}

// Buffer keeps at most limit bytes of one stream and stops the program that writes more.
// It stands apart from the product's bounded buffer so the tools do not import the engines.
type Buffer struct {
	mu    sync.Mutex
	kept  bytes.Buffer
	limit int
	over  bool
	stop  context.CancelFunc
}

// Stream is a Buffer of limit bytes that calls stop when the program writes more.
func Stream(limit int, stop context.CancelFunc) *Buffer {
	return &Buffer{limit: max(limit, 0), stop: stop}
}

func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if room := b.limit - b.kept.Len(); len(p) > room {
		b.kept.Write(p[:room])
		b.over = true
		b.stop()
		return len(p), nil
	}
	return b.kept.Write(p)
}

// Bytes is a copy of what was kept.
func (b *Buffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return bytes.Clone(b.kept.Bytes())
}

// Over reports whether the program wrote past the limit.
func (b *Buffer) Over() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.over
}
