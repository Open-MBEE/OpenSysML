// Package wasm holds the gate over the WebAssembly builds: that every package of this
// module compiles and vets for both Go wasm targets, that each command links carrying
// the version stamps a release build passes, and — where Node runs them — that the
// binaries do real work rather than only link.
package wasm

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
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
var commands = []string{"sysml", "sysml-lsp", "sysml-grpc", "sysml-engine", "sysml-syntax", "sysml-core", "sysml-wasm"}

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
func goFor(target wasmTarget, args ...string) ([]byte, error) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return nil, fmt.Errorf("resolving the repository root: %w", err)
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS="+target.goos, "GOARCH=wasm")
	return cmd.CombinedOutput()
}

// artifactKey names one linked command: a target, a command and its build tags.
type artifactKey struct {
	goos, name, tags string
}

// artifact is one linked command and the outcome of linking it.
type artifact struct {
	once   sync.Once
	path   string
	output []byte
	err    error
}

// artifacts are the linked commands of this run. The first test to ask for one links
// it; every later test shares the binary, so TestWasmBuilds, TestWasmRuns and the
// browser REPL walkthrough never link the same command twice.
var artifacts = struct {
	mu   sync.Mutex
	dir  string
	done map[artifactKey]*artifact
}{done: map[artifactKey]*artifact{}}

// TestMain holds the shared binaries for the run and removes them after it.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "wasm-gate-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating the directory for the linked commands: %v\n", err)
		os.Exit(1)
	}
	artifacts.dir = dir
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// link builds cmd/<name> for the target under the build tags (none when empty),
// stamped, once per run, and checks the artifact is a WebAssembly module: the four
// bytes every runtime dispatches on. The binary is returned for the run gate to
// execute; a test asking after a failed link fails with the same report.
//
// It is a plain `go build`, not tests/testutil/gobuild: that helper adds -cover so a
// process it starts credits the coverage profile, and a WebAssembly process has
// nowhere to write Go's counters to — instrumenting these would buy nothing and make
// the link slower.
func link(t *testing.T, target wasmTarget, name, tags string) string {
	t.Helper()
	key := artifactKey{goos: target.goos, name: name, tags: tags}
	artifacts.mu.Lock()
	a, ok := artifacts.done[key]
	if !ok {
		a = &artifact{}
		artifacts.done[key] = a
	}
	artifacts.mu.Unlock()
	what := "cmd/" + name
	file := name
	if tags != "" {
		what += " -tags " + tags
		file += "-" + strings.ReplaceAll(tags, ",", "-")
	}
	a.once.Do(func() {
		a.path = filepath.Join(artifacts.dir, target.goos, file+".wasm")
		if a.err = os.MkdirAll(filepath.Dir(a.path), 0o755); a.err != nil {
			return
		}
		args := []string{"build"}
		if tags != "" {
			args = append(args, "-tags", tags)
		}
		args = append(args, "-ldflags", ldflags, "-o", a.path, "./cmd/"+name)
		started := time.Now()
		a.output, a.err = goFor(target, args...)
		if a.err == nil {
			a.err = wasmModule(a.path)
		}
		t.Logf("linked %s for GOOS=%s in %s", what, target.goos, time.Since(started).Round(time.Millisecond))
	})
	if a.err != nil {
		t.Fatalf("building %s for GOOS=%s: %v\n%s", what, target.goos, a.err, a.output)
	}
	return a.path
}

// wasmModule reports whether the file starts with the WebAssembly magic.
func wasmModule(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return fmt.Errorf("reading the header of %s: %w", path, err)
	}
	if !bytes.Equal(magic, []byte{0x00, 'a', 's', 'm'}) {
		return fmt.Errorf("%s starts with %v, want the WebAssembly magic 00 61 73 6d", path, magic)
	}
	return nil
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
				if out, err := goFor(target, "build", "./..."); err != nil {
					t.Fatalf("GOOS=%s go build ./...: %v\n%s", target.goos, err, out)
				}
			})
			t.Run("tree vets", func(t *testing.T) {
				if out, err := goFor(target, "vet", "./..."); err != nil {
					t.Fatalf("GOOS=%s go vet ./...: %v\n%s", target.goos, err, out)
				}
			})
			for _, name := range commands {
				t.Run("links "+name, func(t *testing.T) {
					link(t, target, name, "")
				})
			}
			if target.goos == "js" {
				// The CLI page's in-browser REPL ships this build (make build-wasm-re).
				t.Run("links sysml -tags sysml_re", func(t *testing.T) {
					link(t, target, "sysml", "sysml_re")
				})
			}
		})
	}
}
