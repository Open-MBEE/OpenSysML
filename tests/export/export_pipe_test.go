//go:build !wasm

package export_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// A pipe or a device is a stream, not a file with contents to protect, so it is
// written as it stands rather than replaced by a rename.
//
// The test lives in a file no WebAssembly build compiles: a named pipe cannot be
// made on one, and GOOS=<target> go vet — which tests/wasm runs over both
// WebAssembly targets — compiles every test binary it vets.
func TestWriteFileWritesThroughAPipe(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "pipe.sysml")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	read := make(chan string, 1)
	go func() {
		data, err := os.ReadFile(fifo)
		if err != nil {
			t.Error(err)
		}
		read <- string(data)
	}()
	replaced, err := export.WriteFile(fifo, []byte("package Q;\n"))
	if err != nil {
		t.Fatal(err)
	}
	if replaced {
		t.Error("a pipe is not an existing file that was replaced")
	}
	if got := <-read; got != "package Q;\n" {
		t.Errorf("read %q from the pipe", got)
	}
	if info, err := os.Stat(fifo); err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		t.Errorf("the pipe was replaced by a regular file (%v)", err)
	}
}
