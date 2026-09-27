// Package wasm holds the gate over the WebAssembly builds: that every package of this
// module compiles and vets for both Go wasm targets, that each command links carrying
// the version stamps a release build passes, and — where Node runs them — that the
// binaries do real work rather than only link.
package wasm

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// wasmTarget is one GOOS of GOARCH wasm.
type wasmTarget struct {
	name string // the label the tests report it under
	goos string // the GOOS to build it for
}

// wasmTargets are both WebAssembly targets: wasip1 runs under a WASI preview 1
// runtime, js under Node or a browser through the toolchain's wasm_exec.js.
var wasmTargets = []wasmTarget{
	{name: "wasip1", goos: "wasip1"},
	{name: "js", goos: "js"},
}

// commands are the commands every target builds.
var commands = []string{"sysml", "sysml-lsp", "sysml-grpc"}

// versionStamp is what -version reports after the cross-link: the value the ldflags
// below set, so a link that dropped the stamps is caught by a run, not by a release.
const versionStamp = "v0.0.0-wasm-gate"

// ldflags are the Makefile's stamps under other values, so a rename of any of the
// -X targets — which would leave a released binary reporting its own defaults —
// fails here first.
const ldflags = "-s -w -X main.Version=" + versionStamp +
	" -X main.Commit=wasm-gate -X main.BuildTime=2026-01-01_00:00:00" +
	" -X main.GoVersion=go-wasm-gate"

// goFor runs a go command for the target. Later environment entries win, so GOOS and
// GOARCH override whatever the test process carries. It runs at the repository root:
// ./... there is the whole module, where from this package's directory it would be
// only this subtree.
func goFor(t *testing.T, target wasmTarget, args ...string) ([]byte, error) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS="+target.goos, "GOARCH=wasm")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, err
	}
	return out, nil
}

// link builds cmd/<name> for the target, stamped, and checks the artifact is a
// WebAssembly module: the four bytes every runtime dispatches on. The binary is
// returned for the run gate to execute.
//
// It is a plain `go build`, not tests/testutil/gobuild: that helper adds -cover so a
// process it starts credits the coverage profile, and a WebAssembly process has
// nowhere to write Go's counters to — instrumenting these would buy nothing and make
// the link slower.
func link(t *testing.T, target wasmTarget, name, dir string) string {
	t.Helper()
	out := filepath.Join(dir, name+".wasm")
	if output, err := goFor(t, target, "build", "-ldflags", ldflags, "-o", out, "./cmd/"+name); err != nil {
		t.Fatalf("building cmd/%s for GOOS=%s: %v\n%s", name, target.goos, err, output)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatalf("opening %s: %v", out, err)
	}
	defer func() { _ = f.Close() }()
	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		t.Fatalf("reading the header of %s: %v", out, err)
	}
	if !bytes.Equal(magic, []byte{0x00, 'a', 's', 'm'}) {
		t.Fatalf("%s starts with %v, want the WebAssembly magic 00 61 73 6d", out, magic)
	}
	return out
}

// TestWasmBuilds is the half of the gate no runtime is needed for: the whole tree
// compiles and vets for both targets — vet type-checks the test binaries too, which
// is where a unix-only helper in a test file shows up — and each command links to a
// stamped WebAssembly module. It never skips: it needs the Go toolchain, which built
// this test.
func TestWasmBuilds(t *testing.T) {
	for _, target := range wasmTargets {
		t.Run(target.name, func(t *testing.T) {
			t.Run("tree compiles", func(t *testing.T) {
				if out, err := goFor(t, target, "build", "./..."); err != nil {
					t.Fatalf("GOOS=%s go build ./...: %v\n%s", target.goos, err, out)
				}
			})
			t.Run("tree vets", func(t *testing.T) {
				if out, err := goFor(t, target, "vet", "./..."); err != nil {
					t.Fatalf("GOOS=%s go vet ./...: %v\n%s", target.goos, err, out)
				}
			})
			for _, name := range commands {
				name := name
				t.Run("links "+name, func(t *testing.T) {
					link(t, target, name, t.TempDir())
				})
			}
		})
	}
}
