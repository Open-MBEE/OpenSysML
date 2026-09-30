//go:build !wasm

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestConvertModelOutputIsAnInputRefusedBeforeReading -o naming one of a
// model's files is refused before any file is read: an earlier input that is
// a pipe with no writer yet, which a read would wait on, does not delay it.
//
// The test lives in a file no WebAssembly build compiles: a named pipe cannot
// be made on one, and GOOS=<target> go vet compiles every test binary it vets.
func TestConvertModelOutputIsAnInputRefusedBeforeReading(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	pipe := filepath.Join(dir, "a.sysml")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	b := filepath.Join(dir, "b.sysml")
	if err := os.WriteFile(b, []byte("package B;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	res := runCommand(t, exec.CommandContext(ctx, binary, pipe, b, "-convert", "ttl", "-o", b))
	if ctx.Err() != nil {
		t.Fatalf("the command waited on %s instead of refusing -o %s", pipe, b)
	}
	if res.status == 0 || !strings.Contains(res.stderr, "would replace") {
		t.Errorf("-o at a later input's path: status %d, stderr:\n%s", res.status, res.stderr)
	}
	if got, err := os.ReadFile(b); err != nil || string(got) != "package B;\n" {
		t.Errorf("%s was replaced (%v):\n%s", b, err, got)
	}
}
