// Package wasm holds the gate over the WebAssembly builds: that every package of this
// module compiles and vets for both Go wasm targets, that each command links carrying
// the version stamps a release build passes, and — where Node runs them — that the
// binaries do real work rather than only link. Every build embeds the library snapshot
// the gate generates for its run (see snapshotOverlay), so the artifacts it measures and
// runs are the ones the Makefile builds, whatever the tree holds.
package wasm

import (
	"bytes"
	"encoding/json"
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

// repoRoot is the repository root, two levels above this package: ./... there is the
// whole module, where from this package's directory it would be only this subtree.
func repoRoot() (string, error) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return "", fmt.Errorf("resolving the repository root: %w", err)
	}
	return root, nil
}

// goFor runs a go subcommand for the target over the snapshot overlay, at the
// repository root. Later environment entries win, so GOOS and GOARCH override whatever
// the test process carries.
func goFor(target wasmTarget, subcommand string, args ...string) ([]byte, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("go", append([]string{subcommand, "-overlay=" + snapshotOverlay}, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS="+target.goos, "GOARCH=wasm")
	return cmd.CombinedOutput()
}

// libsDir is package libs, relative to the repository root: where `make stdlib-snapshot` writes the
// library snapshot and the Go file embedding it, and where snapshotOverlay supplies them whether or not it ran.
const libsDir = "internal/workspace/libs"

// snapshotOverlay is the go build -overlay every go command of the gate runs with: the library snapshot
// generated for this run and the Go file embedding it, laid over package libs. The gate so measures and runs
// the embedding artifact every Makefile build target ships, not whichever of the two builds (1.6 MB apart gzipped) the tree holds.
var snapshotOverlay string

// generateSnapshot runs the generator behind `make stdlib-snapshot` into dir and returns the overlay file naming
// its output. It runs over an overlay deleting snapshot_embedded.go, which a tree holding it without stdlib.snapshot cannot compile.
func generateSnapshot(dir string) (string, error) {
	root, err := repoRoot()
	if err != nil {
		return "", err
	}
	libs := filepath.Join(root, filepath.FromSlash(libsDir))
	out := filepath.Join(dir, "libs")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return "", err
	}
	without, err := writeOverlay(filepath.Join(dir, "generate-overlay.json"), map[string]string{
		filepath.Join(libs, "snapshot_embedded.go"): "",
	})
	if err != nil {
		return "", err
	}
	cmd := exec.Command("go", "run", "-C", filepath.Join(root, "tools"), "-overlay="+without, "./gen/snapshot", "-dir", out)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS=", "GOARCH=")
	started := time.Now()
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go run -C tools ./gen/snapshot -dir %s: %w\n%s", out, err, output)
	}
	info, err := os.Stat(filepath.Join(out, "stdlib.snapshot"))
	if err != nil {
		return "", err
	}
	fmt.Fprintf(os.Stderr, "wasm gate: every build embeds the library snapshot generated for this run (%d bytes, %s)\n",
		info.Size(), time.Since(started).Round(time.Millisecond))
	return writeOverlay(filepath.Join(dir, "overlay.json"), map[string]string{
		filepath.Join(libs, "stdlib.snapshot"):      filepath.Join(out, "stdlib.snapshot"),
		filepath.Join(libs, "snapshot_embedded.go"): filepath.Join(out, "snapshot_embedded.go"),
	})
}

// writeOverlay writes a go build -overlay file replacing each key with its value; an
// empty value deletes the file from the build.
func writeOverlay(path string, replace map[string]string) (string, error) {
	data, err := json.Marshal(struct{ Replace map[string]string }{replace})
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
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

// artifacts are the linked commands of this run: the first test to ask links one and
// every later test shares the binary, so no command is linked twice.
var artifacts = struct {
	mu   sync.Mutex
	dir  string
	done map[artifactKey]*artifact
}{done: map[artifactKey]*artifact{}}

// TestMain generates the library snapshot every build of the run embeds, holds the
// shared binaries for the run and removes them all after it.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "wasm-gate-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating the directory for the linked commands: %v\n", err)
		os.Exit(1)
	}
	artifacts.dir = dir
	if snapshotOverlay, err = generateSnapshot(dir); err != nil {
		fmt.Fprintf(os.Stderr, "generating the library snapshot the gate embeds: %v\n", err)
		_ = os.RemoveAll(dir)
		os.Exit(1)
	}
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
		args := []string{"-trimpath"}
		if tags != "" {
			args = append(args, "-tags", tags)
		}
		args = append(args, "-ldflags", ldflags, "-o", a.path, "./cmd/"+name)
		started := time.Now()
		a.output, a.err = goFor(target, "build", args...)
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
