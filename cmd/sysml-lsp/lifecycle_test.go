package main

import (
	"bufio"
	"encoding/json"
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

	"github.com/Open-MBEE/OpenSysML/internal/testutil/gobuild"
)

// The binary is what an editor starts, so the lifecycle is tested through it:
// the transport flag a language client passes and the exit status it waits for
// are only observable from outside the process.
var (
	buildOnce   sync.Once
	builtServer string
	buildErr    error
)

// serverBinary builds cmd/sysml-lsp once per test binary and returns its path.
func serverBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "sysml-lsp-build")
		if err != nil {
			buildErr = err
			return
		}
		builtServer = filepath.Join(dir, "sysml-lsp")
		build := exec.Command("go", gobuild.Args(builtServer)...)
		if out, err := build.CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if buildErr != nil {
		t.Fatalf("building the server: %v", buildErr)
	}
	return builtServer
}

// session drives a started server over its stdin/stdout pipes.
type session struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr *strings.Builder
	waited bool
	status int
}

// startServer starts the built binary with args, as a language client does.
func startServer(t *testing.T, args ...string) *session {
	t.Helper()
	cmd := exec.Command(serverBinary(t), args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the server: %v", err)
	}
	s := &session{t: t, cmd: cmd, stdin: stdin, stdout: bufio.NewReader(stdout), stderr: &stderr}
	t.Cleanup(func() {
		if !s.waited {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	return s
}

// send writes one framed JSON-RPC message.
func (s *session) send(msg any) {
	s.t.Helper()
	body, err := json.Marshal(msg)
	if err != nil {
		s.t.Fatalf("marshal %v: %v", msg, err)
	}
	if _, err := fmt.Fprintf(s.stdin, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
		s.t.Fatalf("write %v: %v", msg, err)
	}
}

// request sends a request with the given id.
func (s *session) request(id int, method string, params any) {
	s.send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
}

// notify sends a notification.
func (s *session) notify(method string, params any) {
	s.send(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

// response reads messages until the response to id arrives, so that the
// diagnostics the server publishes alongside do not confuse the assertions.
func (s *session) response(id int) map[string]any {
	s.t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		msg := s.read()
		got, ok := msg["id"].(float64)
		if ok && int(got) == id {
			return msg
		}
	}
	s.t.Fatalf("no response for request id %d\nstderr: %s", id, s.stderr.String())
	return nil
}

// read reads one framed message from the server.
func (s *session) read() map[string]any {
	s.t.Helper()
	length := -1
	for {
		line, err := s.stdout.ReadString('\n')
		if err != nil {
			s.t.Fatalf("read header: %v\nstderr: %s", err, s.stderr.String())
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			s.t.Fatalf("unframed server output: %q", line)
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				s.t.Fatalf("bad Content-Length %q: %v", value, err)
			}
		}
	}
	if length < 0 {
		s.t.Fatal("message without Content-Length header")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(s.stdout, body); err != nil {
		s.t.Fatalf("read body: %v", err)
	}
	var msg map[string]any
	if err := json.Unmarshal(body, &msg); err != nil {
		s.t.Fatalf("unmarshal %q: %v", body, err)
	}
	return msg
}

// waitStatus waits for the process to end and returns its exit status. A server
// still running when the timeout expires is a leaked server, and a failure.
func (s *session) waitStatus(timeout time.Duration) int {
	s.t.Helper()
	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()
	select {
	case err := <-done:
		s.waited = true
		var exit *exec.ExitError
		switch {
		case err == nil:
			s.status = 0
		case errors.As(err, &exit):
			s.status = exit.ExitCode()
		default:
			s.t.Fatalf("waiting for the server: %v", err)
		}
		return s.status
	case <-time.After(timeout):
		s.t.Fatalf("the server was still running %s after exit\nstderr: %s", timeout, s.stderr.String())
		return -1
	}
}

// initialize performs the handshake a client opens with.
func (s *session) initialize(id int) map[string]any {
	s.t.Helper()
	s.request(id, "initialize", map[string]any{"capabilities": map[string]any{}})
	res := s.response(id)
	if res["error"] != nil {
		s.t.Fatalf("initialize failed: %v", res["error"])
	}
	s.notify("initialized", map[string]any{})
	return res
}

// The transport the standard language client names must be one the server
// accepts: with --stdio it has to serve the session and exit 0, not reject the
// command line. This is the shape editors/vscode starts the server in.
func TestStdioTransportServesTheLifecycle(t *testing.T) {
	for _, flag := range []string{"--stdio", "-stdio"} {
		t.Run(flag, func(t *testing.T) {
			s := startServer(t, flag)
			s.initialize(1)
			s.request(2, "shutdown", nil)
			if res := s.response(2); res["error"] != nil {
				t.Fatalf("shutdown failed: %v", res["error"])
			}
			s.notify("exit", nil)
			if status := s.waitStatus(20 * time.Second); status != exitServed {
				t.Errorf("exit status = %d, want %d\nstderr: %s", status, exitServed, s.stderr.String())
			}
		})
	}
}

// A session with no transport flag behaves the same: exit after shutdown ends
// the process with 0, so an editor closing a window leaves no server behind.
func TestExitAfterShutdownEndsTheProcess(t *testing.T) {
	s := startServer(t)
	s.initialize(1)
	s.request(2, "shutdown", nil)
	s.response(2)
	s.notify("exit", nil)
	if status := s.waitStatus(20 * time.Second); status != exitServed {
		t.Errorf("exit status = %d, want %d\nstderr: %s", status, exitServed, s.stderr.String())
	}
}

// LSP 3.17: exit without a preceding shutdown ends the process with 1.
func TestExitWithoutShutdownIsNonzero(t *testing.T) {
	s := startServer(t, "--stdio")
	s.initialize(1)
	s.notify("exit", nil)
	status := s.waitStatus(20 * time.Second)
	if status == 0 {
		t.Errorf("exit status = 0, want nonzero\nstderr: %s", s.stderr.String())
	}
	if status != exitProtocol {
		t.Errorf("exit status = %d, want %d\nstderr: %s", status, exitProtocol, s.stderr.String())
	}
}

// After shutdown the session answers nothing but exit: a request gets the
// InvalidRequest error, and the server is still there to be exited.
func TestRequestAfterShutdownIsInvalidRequest(t *testing.T) {
	const invalidRequest = -32600

	s := startServer(t, "--stdio")
	s.initialize(1)
	s.request(2, "shutdown", nil)
	s.response(2)

	s.request(3, "textDocument/hover", map[string]any{
		"textDocument": map[string]any{"uri": "file:///tmp/after-shutdown.sysml"},
		"position":     map[string]any{"line": 0, "character": 0},
	})
	res := s.response(3)
	failure, ok := res["error"].(map[string]any)
	if !ok {
		t.Fatalf("hover after shutdown answered %v, want an error", res)
	}
	if code, _ := failure["code"].(float64); int(code) != invalidRequest {
		t.Errorf("error code = %v, want %d (InvalidRequest)", failure["code"], invalidRequest)
	}

	s.notify("exit", nil)
	if status := s.waitStatus(20 * time.Second); status != exitServed {
		t.Errorf("exit status = %d, want %d\nstderr: %s", status, exitServed, s.stderr.String())
	}
}

// A client that closes the stream instead of exiting has still been served: the
// process ends by itself, with 0.
func TestClosedStreamEndsTheProcess(t *testing.T) {
	s := startServer(t, "--stdio")
	s.initialize(1)
	if err := s.stdin.Close(); err != nil {
		t.Fatalf("closing stdin: %v", err)
	}
	if status := s.waitStatus(20 * time.Second); status != exitServed {
		t.Errorf("exit status = %d, want %d\nstderr: %s", status, exitServed, s.stderr.String())
	}
}
