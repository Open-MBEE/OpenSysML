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
	"sync"
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

// childEnv is the environment every WebAssembly child runs with, the same for both
// targets so a variable a developer happens to have exported reaches neither half of
// the gate: these commands read PATH, HOME and TMPDIR, and nothing else a result
// here depends on. (The one place a manifest is needed sets OPENSYSML_TOOLS and
// OPENSYSML_ENGINES itself, which wins over anything inherited.)
//
// PWD is the working directory. A wasip1 program's os.Getwd is PWD and nothing else,
// so without it every relative path — and the workspace the manifest rules measure
// the manifest directory against — resolves against "/". The set is small because
// wasm_exec.js gives argv and the environment about 8 KiB between them.
func childEnv(t *testing.T) []string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolving the working directory: %v", err)
	}
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"TMPDIR=" + os.TempDir(),
		"PWD=" + wd,
	}
}

// runner starts a built binary the way its target's runtime runs it.
type runner struct {
	target  wasmTarget
	prefix  []string // node and the runtime script, ahead of the binary
	env     []string // the environment the runtime gets
	stdin   stdinMode
	timeout time.Duration // how long a run may take before it is a hang
}

// newRunner is the Node-backed runner for the target: wasip1 through the WASI
// preview 1 host this package keeps beside it, js through the toolchain's
// wasm_exec_node.js.
func newRunner(t *testing.T, target wasmTarget) runner {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		skipOrFail(t, "node is not on PATH", "install Node 24 or later: it hosts the WASI target and runs wasm_exec")
		return runner{}
	}
	switch target.goos {
	case "wasip1":
		return runner{
			target:  target,
			prefix:  []string{node, "--no-warnings", fixture(t, "wasi.mjs")},
			env:     childEnv(t),
			stdin:   stdinFile,
			timeout: runTimeout,
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
			// Node exits by joining V8's worker threads, and a worker still compiling
			// (Sparkplug, Maglev) can wait for a GC the exiting main thread never
			// runs: the exit deadlocks. --single-threaded keeps every compile and GC
			// task on the main thread, so there is no worker for the exit to wait on.
			prefix: []string{node, "--stack-size=8192", "--single-threaded", script},
			// childEnv is deliberately small: wasm_exec.js writes argv and the
			// environment into linear memory at a fixed offset and stops at
			// wasmMinDataAddr, leaving about 8 KiB for the two together, so a full CI
			// environment overflows it and the runtime dies before main.
			env:     childEnv(t),
			stdin:   stdinPipe,
			timeout: runTimeout,
		}
	}
	t.Fatalf("unknown WebAssembly target %q", target.goos)
	return runner{}
}

// withEnv is the runner with variables added to its environment; a variable set
// here wins over the same one inherited.
func (r runner) withEnv(vars ...string) runner {
	r.env = append(append([]string{}, r.env...), vars...)
	return r
}

// command builds the process for the binary and its arguments, under the runner's
// runtime and environment, and arms its deadline.
func (r runner) command(binary string, args ...string) (*exec.Cmd, *watch, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	full := append(append([]string{}, r.prefix[1:]...), binary)
	full = append(full, args...)
	cmd := exec.CommandContext(ctx, r.prefix[0], full...)
	cmd.Env = r.env
	return cmd, arm(cmd, r.timeout), cancel
}

// watch is a command's deadline, armed to explain a hang: when the deadline passes
// it reads the process's state from /proc before killing it, so the failure says
// how long the process ran, what it was, what its threads were doing and what it
// wrote — the state a hang cannot be explained without.
type watch struct {
	cmd     *exec.Cmd
	timeout time.Duration
	started time.Time

	mu    sync.Mutex
	ended time.Time // when the deadline killed the process; zero while it runs
	state string    // the /proc snapshot the deadline took
}

// arm wires the watch into the command: os/exec calls Cancel from its own goroutine
// when the deadline passes, so the snapshot is taken under a lock the report takes
// too. A process that leaves a pipe open past its own death — a child it started —
// is given WaitDelay to drain, then the pipe is closed rather than waited on forever.
func arm(cmd *exec.Cmd, timeout time.Duration) *watch {
	w := &watch{cmd: cmd, timeout: timeout, started: time.Now()}
	cmd.Cancel = func() error {
		w.mu.Lock()
		w.ended = time.Now()
		w.state = processState(cmd.Process.Pid)
		w.mu.Unlock()
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 10 * time.Second
	return w
}

// hangError is a run the deadline ended: a hang is a broken build, not a slow one.
// Its message is the whole report.
type hangError struct {
	report string
}

func (e *hangError) Error() string { return e.report }

// hang is the error for a run the deadline ended, or nil when the process ended
// itself. output is what the process wrote before the deadline.
func (w *watch) hang(output string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ended.IsZero() {
		return nil
	}
	return &hangError{report: fmt.Sprintf("no answer within %s — a build that hangs here is a failure\n"+
		"elapsed: %s\ncommand: %s\nprocess at the deadline:\n%soutput so far:\n%s",
		w.timeout, w.ended.Sub(w.started).Round(time.Millisecond), strings.Join(w.cmd.Args, " "), w.state, output)}
}

// processState is what /proc says of a live process: its scheduling state, resident
// and peak memory, and every thread's name, state and the kernel function it is
// waiting in. A deadlocked exit shows as a main thread and a worker both waiting.
func processState(pid int) string {
	dir := filepath.Join("/proc", strconv.Itoa(pid))
	status, err := os.ReadFile(filepath.Join(dir, "status"))
	if err != nil {
		return fmt.Sprintf("  (unavailable: %v)\n", err)
	}
	var b strings.Builder
	for _, line := range strings.Split(string(status), "\n") {
		switch key, _, _ := strings.Cut(line, ":"); key {
		case "State", "VmRSS", "VmHWM", "VmSwap", "Threads":
			fmt.Fprintf(&b, "  %s\n", strings.Join(strings.Fields(line), " "))
		}
	}
	tasks, _ := filepath.Glob(filepath.Join(dir, "task", "*"))
	for _, task := range tasks {
		fmt.Fprintf(&b, "  thread %s %q %s %s\n", filepath.Base(task),
			procFile(task, "comm"), threadState(procFile(task, "stat")), procFile(task, "wchan"))
	}
	return b.String()
}

// procFile is a /proc file's contents, or "?" for one that vanished with its thread.
func procFile(dir, name string) string {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(data))
}

// threadState is the state letter of a /proc/<pid>/task/<tid>/stat line, the field
// after the parenthesized command name.
func threadState(stat string) string {
	_, rest, ok := strings.Cut(stat, ") ")
	if !ok {
		return "?"
	}
	state, _, _ := strings.Cut(rest, " ")
	return state
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

// complete runs the binary on stdin to its end and returns what it wrote and its
// status. The error is a *hangError when the deadline ended the run, else what kept
// the process from running at all; a status other than zero is a result, not an error.
func (r runner) complete(stdin io.Reader, binary string, args ...string) (result, error) {
	cmd, w, cancel := r.command(binary, args...)
	defer cancel()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Stdin = stdin
	err := cmd.Run()
	res := result{output: out.String()}
	if hang := w.hang(res.output); hang != nil {
		return res, hang
	}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		res.code = exit.ExitCode()
	default:
		return res, fmt.Errorf("running the process: %v\n%s", err, res.output)
	}
	return res, nil
}

// run completes a process that reads no input, returning its output and status.
func (r runner) run(t *testing.T, binary string, args ...string) result {
	t.Helper()
	var stdin io.Reader
	if r.stdin == stdinFile {
		// An empty regular file, not a closed pipe: Node's WASI answers a pipe with
		// nothing ready by EAGAIN, where a file answers end of input.
		stdin = emptyFile(t)
	}
	res, err := r.complete(stdin, binary, args...)
	return r.finish(t, res, err)
}

// runWithInput completes a process fed lines — a file of them on a file-driven
// target, the lines themselves on a pipe-driven one — and returns what it wrote.
func (r runner) runWithInput(t *testing.T, binary string, lines string, args ...string) result {
	t.Helper()
	var stdin io.Reader = strings.NewReader(lines)
	if r.stdin == stdinFile {
		path := filepath.Join(t.TempDir(), "stdin")
		if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
			t.Fatalf("writing the input: %v", err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatalf("opening the input: %v", err)
		}
		defer func() { _ = file.Close() }()
		stdin = file
	}
	res, err := r.complete(stdin, binary, args...)
	return r.finish(t, res, err)
}

// finish fails the test on a run that did not end itself and applies the invariant
// every run must hold to one that did.
func (r runner) finish(t *testing.T, res result, err error) result {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
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

// syncBuffer collects what a process writes while the test may read it. os/exec
// copies into an io.Writer from a goroutine it starts with the process and joins it
// only in Wait, so an unsynchronized buffer would race with the failure paths that
// print it — and this suite runs under -race, where the report would land on top of
// the failure it is meant to explain.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write appends what the process wrote, under the lock a reader takes too.
func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// String is everything written so far, including from the copying goroutine.
func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// session is a running process driven as a client drives it: frames in on standard
// input, frames out on standard output, and logs kept on standard error, where
// mixing them would corrupt the frames.
type session struct {
	t      *testing.T
	target wasmTarget
	cmd    *exec.Cmd
	watch  *watch
	cancel context.CancelFunc
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr *syncBuffer
	waited chan error
}

// start begins a session for the binary and its arguments. A session needs a pipe
// driven target — Node's WASI cannot block on one — so only the js target has one
// here, and every case that starts a session is that target's.
func (r runner) start(t *testing.T, binary string, args ...string) *session {
	t.Helper()
	if r.stdin != stdinPipe {
		t.Fatalf("%s is driven with files, and a session needs a pipe it can block on", r.target.name)
	}
	cmd, w, cancel := r.command(binary, args...)
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
	var stderr syncBuffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("starting the process: %v", err)
	}
	s := &session{
		t:      t,
		target: r.target,
		cmd:    cmd,
		watch:  w,
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
			s.fail(fmt.Sprintf("reading a frame: %v", got.err))
		}
		return string(got.body)
	case <-time.After(s.watch.timeout):
		s.fail(fmt.Sprintf("no frame within %s", s.watch.timeout))
	}
	return ""
}

// fail ends the test on what stopped the session: the deadline's report when the
// deadline killed the process — a frame that never comes ends in the deadline, and
// the report says what the process was doing — else the reason and standard error.
func (s *session) fail(reason string) {
	s.t.Helper()
	if hang := s.watch.hang(s.stderr.String()); hang != nil {
		s.t.Fatal(hang)
	}
	s.t.Fatalf("%s\nstderr:\n%s", reason, s.stderr.String())
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

// check applies the invariant every other run applies: a WebAssembly build answers
// with an error a reader can act on, never a Go panic. wait calls it once the
// process has ended, when its output is complete and reading it races nothing.
func (s *session) check() {
	s.t.Helper()
	if strings.Contains(s.stderr.String(), "panic:") {
		s.t.Errorf("%s panicked:\n%s", s.target.name, s.stderr.String())
	}
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
				s.fail(fmt.Sprintf("the process ended: %v", err))
			}
		}
	case <-time.After(s.watch.timeout):
		s.fail(fmt.Sprintf("no exit within %s", s.watch.timeout))
	}
	s.check()
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

			t.Run("validates a model", func(t *testing.T) {
				// The check tier end to end: parse, resolve, analyse, report. It is the
				// command a user runs first, so it is the one that has to work on the
				// target rather than merely link.
				got := r.run(t, bins["sysml"], fixture(t, "model.sysml"), "-validate")
				if got.code != 0 || !strings.Contains(got.output, "no errors") {
					t.Errorf("sysml -validate exited %d:\n%s", got.code, got.output)
				}
			})

			t.Run("answers an element query", func(t *testing.T) {
				got := r.run(t, bins["sysml"], fixture(t, "model.sysml"), "-query", `sysml:name="Widget"`)
				if got.code != 0 || !strings.Contains(got.output, "Widget") {
					t.Errorf("sysml -query exited %d:\n%s", got.code, got.output)
				}
			})

			t.Run("converts a model to turtle", func(t *testing.T) {
				out := filepath.Join(t.TempDir(), "model.ttl")
				got := r.run(t, bins["sysml"], fixture(t, "model.sysml"), "-convert", "ttl", "-o", out)
				if got.code != 0 {
					t.Errorf("sysml -convert ttl exited %d:\n%s", got.code, got.output)
				}
				if data, err := os.ReadFile(out); err != nil || !bytes.HasPrefix(data, []byte("@prefix")) {
					t.Errorf("the Turtle written is %v bytes (%v), want a graph", len(data), err)
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
				got := r.withEnv(
					"OPENSYSML_TOOLS="+manifestDir(t, "tools"),
					"OPENSYSML_ENGINES="+manifestDir(t, "engines"),
				).run(t, bins["sysml"], "-engines")
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

			t.Run("names the Go toolchain it cannot start", func(t *testing.T) {
				out := filepath.Join(t.TempDir(), "square")
				got := r.run(t, bins["sysml"], fixture(t, "calc.sysml"),
					"-compile", "calcdemo::Square", "-target", "go", "-o", out)
				if got.code == 0 {
					t.Errorf("-compile -target go exited 0, want a refusal:\n%s", got.output)
				}
				const want = "codegen: go cannot run: a WebAssembly build cannot start external processes"
				if !strings.Contains(got.output, want) {
					t.Errorf("-compile -target go output missing %q:\n%s", want, got.output)
				}
				if _, err := os.Stat(out + ".go"); !os.IsNotExist(err) {
					t.Errorf("generated source stat error = %v, want not-exist", err)
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

			// sysml-engine's half of the run gate: a stdio session doing real work
			// on both targets, and on js the globalThis host surface plus the size
			// budget that surface exists to keep.
			engineSubtests(t, target, r, bins)

			// sysml-syntax's half: the same session shape over the syntactic RPCs.
			syntaxSubtests(t, target, r, bins)

			// sysml-core's half: validation RPCs with symbol facts, stdio and JS host.
			coreSubtests(t, target, r, bins)
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
		skipOrFail(t, "node is not on PATH", "install Node 24 or later: it hosts the WASI target and runs wasm_exec")
	}
}
