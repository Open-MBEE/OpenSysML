package wasm

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// requireEnv is set where the run gate must run rather than skip, so CI cannot pass
// green without executing a WebAssembly binary.
const requireEnv = "OPENSYSML_REQUIRE_WASM"

// runTimeout bounds every run: a build that hangs — what a listener nothing could
// connect to would do — must fail the test rather than wedge it.
const runTimeout = 2 * time.Minute

// stdinMode is how a case hands a process its input.
//
// Node's WASI cannot block on a pipe: its poll_oneoff does not wait for file
// descriptor readiness, so a read with nothing ready comes back EAGAIN and the
// process fails on it. A regular file always answers, so wasip1 processes are
// driven with files. Node's js runtime reads pipes the way any client would, so js
// processes are driven with them — which is also what lets that target hold a pipe
// open across a request and its answer.
type stdinMode int

const (
	stdinFile stdinMode = iota
	stdinPipe
)

// skipOrFail skips when the gate's prerequisite is missing and fails when the
// require variable is set, so a green run cannot be a skipped one.
func skipOrFail(t *testing.T, reason, hint string) {
	t.Helper()
	if value := os.Getenv(requireEnv); value != "" {
		t.Fatalf("%s=%s but %s: %s", requireEnv, value, reason, hint)
	}
	fmt.Fprintf(os.Stderr, "\n!!! GATE NOT RUN: %s SKIPPED - %s.\n"+
		"!!! CI sets %s=1, where a missing runtime fails instead of skipping.\n"+
		"!!!   go test -count=1 ./tests/wasm\n\n", t.Name(), reason, requireEnv)
	t.Skip(hint)
}

// wasmExecNode is the Go toolchain's own runner for a js/wasm binary under Node.
func wasmExecNode() string {
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return ""
	}
	return filepath.Join(strings.TrimSpace(string(out)), "lib", "wasm", "wasm_exec_node.js")
}

// runner starts a built binary the way its target's runtime runs it.
type runner struct {
	target wasmTarget
	prefix []string // node and the runtime script, ahead of the binary
	env    []string // the environment the runtime gets
	stdin  stdinMode
}

// newRunner is the Node-backed runner for the target: wasip1 through the WASI
// preview 1 host this package keeps beside it, js through the toolchain's
// wasm_exec_node.js.
func newRunner(t *testing.T, target wasmTarget) runner {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		skipOrFail(t, "node is not on PATH", "install Node 20 or later: it hosts the WASI target and runs wasm_exec")
		return runner{}
	}
	switch target.goos {
	case "wasip1":
		return runner{
			target: target,
			prefix: []string{node, "--no-warnings", fixture(t, "wasi.mjs")},
			env:    os.Environ(),
			stdin:  stdinFile,
		}
	case "js":
		script := wasmExecNode()
		if script == "" {
			skipOrFail(t, "go env GOROOT failed", "the Go toolchain ships wasm_exec_node.js at $GOROOT/lib/wasm")
			return runner{}
		}
		if _, err := os.Stat(script); err != nil {
			skipOrFail(t, script+" is not readable", "the Go toolchain ships wasm_exec_node.js at $GOROOT/lib/wasm")
			return runner{}
		}
		return runner{
			target: target,
			prefix: []string{node, "--stack-size=8192", script},
			// wasm_exec.js writes argv and the environment into linear memory at a
			// fixed offset and stops at wasmMinDataAddr, leaving about 8 KiB for the
			// two together: a full CI environment overflows it and the runtime dies
			// before main. Three short variables are all these cases need.
			env: []string{
				"PATH=" + os.Getenv("PATH"),
				"HOME=" + os.Getenv("HOME"),
				"TMPDIR=" + os.TempDir(),
			},
			stdin: stdinPipe,
		}
	}
	t.Fatalf("unknown WebAssembly target %q", target.goos)
	return runner{}
}

// command builds the process for the binary and its arguments, under the runner's
// runtime and environment.
func (r runner) command(ctx context.Context, binary string, args ...string) *exec.Cmd {
	full := append(append([]string{}, r.prefix[1:]...), binary)
	full = append(full, args...)
	cmd := exec.CommandContext(ctx, r.prefix[0], full...)
	cmd.Env = r.env
	return cmd
}

// result is a run that finished: its output and its exit status.
type result struct {
	output string
	code   int
}

// check is the invariant every run must hold: a WebAssembly build answers with an
// error a reader can act on, never a Go panic.
func (r runner) check(t *testing.T, res result) {
	t.Helper()
	if strings.Contains(res.output, "panic:") {
		t.Errorf("%s panicked:\n%s", r.target.name, res.output)
	}
}

// exitCode reads the status of a finished run, failing the test when the deadline
// is what ended it: a hang is a broken build, not a slow one.
func exitCode(t *testing.T, ctx context.Context, err error, output string) int {
	t.Helper()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("no answer within %s — a build that hangs here is a failure:\n%s", runTimeout, output)
	}
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	t.Fatalf("running the process: %v\n%s", err, output)
	return -1
}

// run completes a process that reads no input, returning its output and status.
func (r runner) run(t *testing.T, binary string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	cmd := r.command(ctx, binary, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if r.stdin == stdinFile {
		// An empty regular file, not a closed pipe: Node's WASI answers a pipe with
		// nothing ready by EAGAIN, where a file answers end of input.
		cmd.Stdin = emptyFile(t)
	}
	err := cmd.Run()
	res := result{output: out.String(), code: exitCode(t, ctx, err, out.String())}
	r.check(t, res)
	return res
}

// runWithInput completes a process fed lines — a file of them on a file-driven
// target, the lines themselves on a pipe-driven one — and returns what it wrote.
func (r runner) runWithInput(t *testing.T, binary string, lines string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	cmd := r.command(ctx, binary, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	var err error
	switch r.stdin {
	case stdinFile:
		path := filepath.Join(t.TempDir(), "stdin")
		if writeErr := os.WriteFile(path, []byte(lines), 0o600); writeErr != nil {
			t.Fatalf("writing the input: %v", writeErr)
		}
		file, openErr := os.Open(path)
		if openErr != nil {
			t.Fatalf("opening the input: %v", openErr)
		}
		defer func() { _ = file.Close() }()
		cmd.Stdin = file
		err = cmd.Run()
	case stdinPipe:
		var stdin io.WriteCloser
		if stdin, err = cmd.StdinPipe(); err != nil {
			t.Fatalf("piping standard input: %v", err)
		}
		if err = cmd.Start(); err != nil {
			t.Fatalf("starting the process: %v", err)
		}
		if _, err = io.WriteString(stdin, lines); err != nil {
			t.Fatalf("writing the input: %v", err)
		}
		if err = stdin.Close(); err != nil {
			t.Fatalf("ending the input: %v", err)
		}
		err = cmd.Wait()
	}
	res := result{output: out.String(), code: exitCode(t, ctx, err, out.String())}
	r.check(t, res)
	return res
}

// emptyFile is an empty regular file to stand in for no input at all.
func emptyFile(t *testing.T) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "empty-input")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("writing the empty input: %v", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening the empty input: %v", err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

// session is a running process driven as a client drives it: frames in on standard
// input, frames out on standard output, and logs kept on standard error, where
// mixing them would corrupt the frames.
type session struct {
	t      *testing.T
	cmd    *exec.Cmd
	cancel context.CancelFunc
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr *bytes.Buffer
	waited chan error
}

// start begins a session for the binary and its arguments.
func (r runner) start(t *testing.T, binary string, args ...string) *session {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	cmd := r.command(ctx, binary, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		t.Fatalf("piping standard input: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatalf("piping standard output: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("starting the process: %v", err)
	}
	s := &session{
		t:      t,
		cmd:    cmd,
		cancel: cancel,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
		stderr: &stderr,
		waited: make(chan error, 1),
	}
	go func() { s.waited <- cmd.Wait() }()
	t.Cleanup(func() {
		select {
		case <-s.waited:
		default:
			_ = cmd.Process.Kill()
		}
		cancel()
	})
	return s
}

// writeFrame sends one Content-Length-delimited frame: the framing sysml-lsp and
// sysml-grpc -transport stdio both speak.
func (s *session) writeFrame(body string) {
	s.t.Helper()
	if _, err := fmt.Fprintf(s.stdin, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
		s.t.Fatalf("writing a frame: %v\nstderr:\n%s", err, s.stderr.String())
	}
}

// readFrame reads one Content-Length-delimited frame, bounded by the deadline: an
// answer that never comes fails the test rather than leaving it blocked.
func (s *session) readFrame() string {
	s.t.Helper()
	type answer struct {
		body []byte
		err  error
	}
	read := make(chan answer, 1)
	go func() {
		body, err := frame(s.stdout)
		read <- answer{body: body, err: err}
	}()
	select {
	case got := <-read:
		if got.err != nil {
			s.t.Fatalf("reading a frame: %v\nstderr:\n%s", got.err, s.stderr.String())
		}
		return string(got.body)
	case <-time.After(runTimeout):
		s.t.Fatalf("no frame within %s\nstderr:\n%s", runTimeout, s.stderr.String())
	}
	return ""
}

// frame reads one frame off r: headers each a line, a blank line, then the body the
// Content-Length header counts.
func frame(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if value, ok := strings.CutPrefix(line, "Content-Length:"); ok {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("unreadable Content-Length %q", value)
			}
			length = n
		}
	}
	if length < 0 {
		return nil, errors.New("frame carries no Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

// wait ends the session's input and waits for the process to exit cleanly, failing
// the test on any other end: an error or a status that is not zero.
func (s *session) wait() {
	s.t.Helper()
	if err := s.stdin.Close(); err != nil {
		s.t.Fatalf("ending the input: %v", err)
	}
	select {
	case err := <-s.waited:
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 0 {
				s.t.Fatalf("the process ended: %v\nstderr:\n%s", err, s.stderr.String())
			}
		}
	case <-time.After(runTimeout):
		s.t.Fatalf("no exit within %s\nstderr:\n%s", runTimeout, s.stderr.String())
	}
}

// TestWasmRuns executes what TestWasmBuilds links: the work a WebAssembly host can
// do — evaluating expressions and a loaded model, running the prompt, listing
// engines — and the work it must refuse by name rather than fail obscurely — a
// solver, a compiler, a converter, a listener.
//
// It needs Node, which CI installs; without it the gate skips loudly, and with the
// require variable set a skip fails instead.
func TestWasmRuns(t *testing.T) {
	requireNode(t)
	for _, target := range wasmTargets {
		t.Run(target.name, func(t *testing.T) {
			r := newRunner(t, target)
			bins := make(map[string]string, len(commands))
			for _, name := range commands {
				bins[name] = link(t, target, name, t.TempDir())
			}

			t.Run("version", func(t *testing.T) {
				for _, name := range commands {
					got := r.run(t, bins[name], "-version")
					if got.code != 0 {
						t.Errorf("%s -version exited %d:\n%s", name, got.code, got.output)
					}
					if !strings.Contains(got.output, versionStamp) {
						t.Errorf("%s -version =\n%s\nwant it to report the stamped version %q", name, got.output, versionStamp)
					}
				}
			})

			t.Run("evaluates an expression", func(t *testing.T) {
				got := r.run(t, bins["sysml"], "-e", "2 + 3")
				if got.code != 0 || !strings.Contains(got.output, "= 5") {
					t.Errorf("sysml -e '2 + 3' exited %d:\n%s", got.code, got.output)
				}
			})

			t.Run("evaluates a loaded model", func(t *testing.T) {
				got := r.run(t, bins["sysml"], fixture(t, "model.sysml"), "-e", "total")
				if got.code != 0 || !strings.Contains(got.output, "= 6.0") {
					t.Errorf("sysml model.sysml -e total exited %d:\n%s", got.code, got.output)
				}
			})

			t.Run("runs the prompt", func(t *testing.T) {
				got := r.runWithInput(t, bins["sysml"], readFile(t, "repl.txt"))
				if got.code != 0 {
					t.Errorf("the prompt exited %d:\n%s", got.code, got.output)
				}
				for _, want := range []string{"sysml> ", "= 5", "show this help"} {
					if !strings.Contains(got.output, want) {
						t.Errorf("the prompt output is missing %q:\n%s", want, got.output)
					}
				}
			})

			t.Run("names the processes it cannot start", func(t *testing.T) {
				// Both manifests are read so a tool and an external engine are listed
				// beside the solver, and each status must name the host as the reason:
				// the check runs before the lookup, so the answer is about the host
				// rather than about whether the executable happens to be there. They are
				// copied out of the working directory first, because sysml refuses a
				// manifest under the workspace it reads models from: a manifest comes
				// from the environment, never from a workspace or a model.
				ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
				defer cancel()
				cmd := r.command(ctx, bins["sysml"], "-engines")
				cmd.Env = append(append([]string{}, r.env...),
					"OPENSYSML_TOOLS="+manifestDir(t, "tools"),
					"OPENSYSML_ENGINES="+manifestDir(t, "engines"))
				if r.stdin == stdinFile {
					cmd.Stdin = emptyFile(t)
				}
				var out bytes.Buffer
				cmd.Stdout = &out
				cmd.Stderr = &out
				err := cmd.Run()
				got := result{output: out.String(), code: exitCode(t, ctx, err, out.String())}
				r.check(t, got)
				if got.code != 0 {
					t.Errorf("sysml -engines exited %d:\n%s", got.code, got.output)
				}
				for _, want := range []string{
					"an SMT solver cannot run: a WebAssembly build cannot start external processes",
					"echo cannot run: a WebAssembly build cannot start external processes",
					"/bin/echo cannot run: a WebAssembly build cannot start external processes",
				} {
					if !strings.Contains(got.output, want) {
						t.Errorf("sysml -engines is missing %q:\n%s", want, got.output)
					}
				}
			})

			t.Run("names the compiler it cannot start", func(t *testing.T) {
				out := filepath.Join(t.TempDir(), "square")
				got := r.run(t, bins["sysml"], fixture(t, "calc.sysml"),
					"-compile", "calcdemo::Square", "-o", out)
				if got.code == 0 {
					t.Errorf("-compile exited 0, want a refusal:\n%s", got.output)
				}
				if !strings.Contains(got.output, "codegen:") || !strings.Contains(got.output, "cannot run") {
					t.Errorf("-compile did not name the compiler it cannot start:\n%s", got.output)
				}
			})

			t.Run("names the converter it cannot start", func(t *testing.T) {
				out := filepath.Join(t.TempDir(), "document.pdf")
				got := r.run(t, bins["sysml"], fixture(t, "document.sysml"),
					"-render-document", "Hello::HelloReport", "-doc-form", "pdf", "-o", out)
				if got.code == 0 {
					t.Errorf("-doc-form pdf exited 0, want a refusal:\n%s", got.output)
				}
				if !strings.Contains(got.output, "weasyprint cannot run: a WebAssembly build cannot start external processes") {
					t.Errorf("the PDF render did not name the converter it cannot start:\n%s", got.output)
				}
			})

			t.Run("refuses a transport that binds an address", func(t *testing.T) {
				for _, transport := range []string{"connect", "grpc"} {
					got := r.run(t, bins["sysml-grpc"], "-transport", transport)
					if got.code != 2 {
						t.Errorf("-transport %s exited %d, want 2:\n%s", transport, got.code, got.output)
					}
					for _, want := range []string{"binds an address", "use -transport stdio"} {
						if !strings.Contains(got.output, want) {
							t.Errorf("-transport %s is missing %q:\n%s", transport, want, got.output)
						}
					}
				}
			})

			if r.stdin == stdinFile {
				// A session needs a pipe a client holds open, which Node's WASI cannot
				// block on; these targets end each server at end of input instead, which
				// still proves it starts and shuts down cleanly. The js target below runs
				// the sessions themselves.
				t.Run("serves stdio to end of input", func(t *testing.T) {
					got := r.run(t, bins["sysml-grpc"], "-transport", "stdio")
					if got.code != 0 {
						t.Errorf("-transport stdio exited %d:\n%s", got.code, got.output)
					}
					if !strings.Contains(got.output, "Serving over stdin/stdout") {
						t.Errorf("-transport stdio did not serve:\n%s", got.output)
					}
				})
				t.Run("ends the language server at end of input", func(t *testing.T) {
					got := r.run(t, bins["sysml-lsp"], "-stdio")
					if got.code != 0 {
						t.Errorf("sysml-lsp -stdio exited %d:\n%s", got.code, got.output)
					}
				})
			} else {
				t.Run("answers a service session over stdio", func(t *testing.T) {
					s := r.start(t, bins["sysml-grpc"], "-transport", "stdio")
					s.writeFrame(`{"jsonrpc":"2.0","id":1,"method":"GetServerInfo","params":{}}`)
					answer := s.readFrame()
					if !strings.Contains(answer, versionStamp) {
						t.Errorf("GetServerInfo answered %s, want the stamped version %q", answer, versionStamp)
					}
					s.wait()
				})
				t.Run("answers a language server session", func(t *testing.T) {
					s := r.start(t, bins["sysml-lsp"], "-stdio")
					s.writeFrame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":1,"rootUri":"file:///","capabilities":{}}}`)
					answer := s.readFrame()
					if !strings.Contains(answer, "capabilities") {
						t.Errorf("initialize answered %s, want capabilities", answer)
					}
					s.writeFrame(`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`)
					s.readFrame()
					s.writeFrame(`{"jsonrpc":"2.0","method":"exit"}`)
					s.wait()
				})
			}
		})
	}
}

// readFile returns a fixture's contents, failing the test when it is absent: a gate
// whose fixture went missing must not pass on a skipped case.
func readFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	return string(data)
}

// fixture names a testdata file by an absolute path: the WebAssembly builds resolve
// paths against the host's preopened root or the runtime's working directory, neither
// of which is this package's, so every path a process is given is absolute.
func fixture(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("resolving the fixture: %v", err)
	}
	return path
}

// manifestDir copies a testdata manifest directory outside the working directory and
// returns where it landed. sysml refuses to read a manifest under the workspace its
// models are read from — a manifest comes from the environment, never from a
// workspace or a model — and a temporary directory is where an operator's copy would
// be: outside the project altogether.
func manifestDir(t *testing.T, name string) string {
	t.Helper()
	src := fixture(t, name)
	dst := filepath.Join(t.TempDir(), name)
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatalf("making the manifest directory: %v", err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("reading the manifest directory: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, entry.Name()))
		if err != nil {
			t.Fatalf("reading the manifest entry: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dst, entry.Name()), data, 0o644); err != nil {
			t.Fatalf("copying the manifest entry: %v", err)
		}
	}
	return dst
}

// requireNode skips the run gate when Node cannot host it, and fails when the
// require variable says it must run.
func requireNode(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		skipOrFail(t, "node is not on PATH", "install Node 20 or later: it hosts the WASI target and runs wasm_exec")
	}
}
