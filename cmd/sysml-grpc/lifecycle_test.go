// The binary is a released artifact, and the port it listens on, the exit status
// a supervisor reads and a leaked process are only observable outside it.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/testutil/gobuild"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

var (
	buildOnce   sync.Once
	builtServer string
	buildErr    error
)

// serviceBinary builds cmd/sysml-grpc once per test binary and returns its path.
func serviceBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "sysml-grpc-build")
		if err != nil {
			buildErr = err
			return
		}
		builtServer = filepath.Join(dir, "sysml-grpc")
		build := exec.Command("go", gobuild.Args(builtServer)...)
		if out, err := build.CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if buildErr != nil {
		t.Fatalf("building the service: %v", buildErr)
	}
	return builtServer
}

// service is a started binary, driven the way a client drives it.
type service struct {
	t   *testing.T
	cmd *exec.Cmd
	// ready carries the address from the listening line the server logs, so a
	// test waits on the server's own readiness rather than on a sleep.
	ready chan string
	logs  *safeBuilder
	// scanned is closed when the log reader reaches EOF, which is when the logs
	// are complete and the process may be waited on.
	scanned chan struct{}
	// exit carries the process's end, from the single goroutine that waits on it.
	exit chan error
}

// safeBuilder collects the server's stderr while a test reads it.
type safeBuilder struct {
	mu   sync.Mutex
	text strings.Builder
}

func (b *safeBuilder) add(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.text.WriteString(line)
	b.text.WriteString("\n")
}

func (b *safeBuilder) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.text.String()
}

// startService starts the built binary with args. It does not wait for
// readiness, so a failure to start is observable too.
func startService(t *testing.T, args ...string) *service {
	t.Helper()
	cmd := exec.Command(serviceBinary(t), args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	s := &service{
		t: t, cmd: cmd,
		ready:   make(chan string, 1),
		logs:    &safeBuilder{},
		scanned: make(chan struct{}),
		exit:    make(chan error, 1),
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the service: %v", err)
	}
	go s.scan(stderr)
	go func() {
		// The logs come from a pipe, so they have to be drained to EOF before the
		// process is waited on, or a log line is lost with the pipe.
		<-s.scanned
		s.exit <- cmd.Wait()
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-s.exit
	})
	return s
}

// scan reads the server's log lines, publishing the address of the first
// listening line it sees.
func (s *service) scan(r io.Reader) {
	defer close(s.scanned)
	announced := false
	lines := bufio.NewScanner(r)
	for lines.Scan() {
		line := lines.Text()
		s.logs.add(line)
		if announced || (!strings.Contains(line, "gRPC server listening") &&
			!strings.Contains(line, "Connect server listening")) {
			continue
		}
		if addr, ok := listeningAddr(line); ok {
			announced = true
			s.ready <- addr
		}
	}
}

// listeningAddr extracts the dialable address from a listening log line, whose
// addr= holds the listener's address (for example [::]:39485).
func listeningAddr(line string) (string, bool) {
	_, rest, ok := strings.Cut(line, "addr=")
	if !ok {
		return "", false
	}
	addr := strings.Fields(rest)[0]
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", false
	}
	return net.JoinHostPort("127.0.0.1", port), true
}

// address waits for the server to report the port it listens on. A server that
// never reports one within the timeout is a hang, and a failure.
func (s *service) address(timeout time.Duration) string {
	s.t.Helper()
	select {
	case addr := <-s.ready:
		return addr
	case <-time.After(timeout):
		s.t.Fatalf("the service did not report a listening address within %s\nlogs: %s", timeout, s.logs.String())
		return ""
	}
}

// waitStatus waits for the process to end and returns its exit status. A
// process still running when the timeout expires is a leak, and a failure.
func (s *service) waitStatus(timeout time.Duration) int {
	s.t.Helper()
	select {
	case err := <-s.exit:
		s.exit <- err // the cleanup drains it, so exactly one Wait ever runs
		var exit *exec.ExitError
		switch {
		case err == nil:
			return 0
		case errors.As(err, &exit):
			return exit.ExitCode()
		default:
			s.t.Fatalf("waiting for the service: %v", err)
			return -1
		}
	case <-time.After(timeout):
		s.t.Fatalf("the service was still running %s after shutdown\nlogs: %s", timeout, s.logs.String())
		return -1
	}
}

// terminate asks for the graceful shutdown a supervisor asks for.
func (s *service) terminate() {
	s.t.Helper()
	if err := s.cmd.Process.Signal(os.Interrupt); err != nil {
		s.t.Fatalf("signalling the service: %v", err)
	}
}

// dial connects to a started service. Both ports are ephemeral, so tests never
// contend for a fixed one.
func dial(t *testing.T, addr string) pb.SysMLServiceClient {
	t.Helper()
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dialling %s: %v", addr, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return pb.NewSysMLServiceClient(conn)
}

// callContext bounds every RPC, so a service that stops answering fails the
// test instead of hanging it.
func callContext(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// fixture is a gRPC conformance model, reused here rather than restated: the
// point of this test is the process, not the model.
func fixture(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "internal", "grpc", "testdata", "conformance", name))
	if err != nil {
		t.Fatalf("resolving fixture %s: %v", name, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return path
}

// The lifecycle a user hits: start on the given port, real RPCs against a model
// it parsed, then a signal ending the process with 0 and no leak.
func TestServiceServesRPCsAndShutsDownCleanly(t *testing.T) {
	s := startService(t, "-port", "0", "-health-port", "0")
	client := dial(t, s.address(30*time.Second))
	ctx := callContext(t)

	info, err := client.GetServerInfo(ctx, &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatalf("GetServerInfo: %v\nlogs: %s", err, s.logs.String())
	}
	if info.Version != "dev" {
		t.Errorf("version = %q, want %q (the default when the linker set none)", info.Version, "dev")
	}
	if len(info.Capabilities) == 0 {
		t.Error("the service reported no capabilities")
	}

	parsed, err := client.ParseFile(ctx, &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_FilePath{FilePath: fixture(t, "instantiate_part.sysml")},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v\nlogs: %s", err, s.logs.String())
	}
	if parsed.Error != "" {
		t.Fatalf("ParseFile reported %q", parsed.Error)
	}
	if parsed.ModelHash == "" {
		t.Fatal("ParseFile returned no model hash")
	}

	inst, err := client.Instantiate(ctx, &pb.InstantiateRequest{
		ModelHash: parsed.ModelHash,
		SymbolId:  "test::Vehicle",
	})
	if err != nil {
		t.Fatalf("Instantiate: %v\nlogs: %s", err, s.logs.String())
	}
	if inst.Error != "" {
		t.Fatalf("Instantiate reported %q", inst.Error)
	}
	mass := featureValueOf(t, inst, "mass")
	if mass.GetValue().GetIntValue() != 1500 {
		t.Errorf("mass = %v, want 1500", mass.GetValue())
	}

	s.terminate()
	if status := s.waitStatus(45 * time.Second); status != 0 {
		t.Errorf("exit status = %d, want 0\nlogs: %s", status, s.logs.String())
	}
}

// featureValueOf returns the named feature value of an instantiation response.
func featureValueOf(t *testing.T, res *pb.InstantiateResponse, name string) *pb.FeatureValue {
	t.Helper()
	for _, fv := range res.GetInstance().GetFeatureValues() {
		if fv.GetFeatureName() == name {
			return fv
		}
	}
	t.Fatalf("no feature value %q in %v", name, res.GetInstance())
	return nil
}

// The health endpoint is what a container orchestrator polls, so it answers on
// the port the flag names.
func TestHealthEndpointReportsTheBuild(t *testing.T) {
	healthPort := freePort(t)
	s := startService(t, "-port", "0", "-health-port", healthPort)
	s.address(30 * time.Second)

	url := "http://" + net.JoinHostPort("127.0.0.1", healthPort) + "/health"
	deadline := time.Now().Add(30 * time.Second)
	var body string
	for {
		res, err := http.Get(url) // #nosec G107 -- the test built this URL
		if err == nil {
			raw, readErr := io.ReadAll(res.Body)
			_ = res.Body.Close()
			if readErr != nil {
				t.Fatalf("reading /health: %v", readErr)
			}
			if res.StatusCode != http.StatusOK {
				t.Fatalf("/health status = %d, want 200: %s", res.StatusCode, raw)
			}
			body = string(raw)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("/health never answered: %v\nlogs: %s", err, s.logs.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, want := range []string{`"status":"ok"`, `"service":"sysml-grpc"`, `"version":"dev"`} {
		if !strings.Contains(body, want) {
			t.Errorf("/health body is missing %s:\n%s", want, body)
		}
	}

	s.terminate()
	if status := s.waitStatus(45 * time.Second); status != 0 {
		t.Errorf("exit status = %d, want 0\nlogs: %s", status, s.logs.String())
	}
}

func TestDefaultTransportServesConnectAndHealthOnMainPort(t *testing.T) {
	s := startService(t, "-port", "0", "-health-port", "0")
	addr := s.address(30 * time.Second)

	body := getHealth(t, addr)
	for _, want := range []string{`"status":"ok"`, `"service":"sysml-grpc"`, `"version":"dev"`} {
		if !strings.Contains(body, want) {
			t.Errorf("/health body is missing %s:\n%s", want, body)
		}
	}

	client := dial(t, addr)
	if _, err := client.GetServerInfo(callContext(t), &pb.ServerInfoRequest{}); err != nil {
		t.Fatalf("GetServerInfo over the main port: %v\nlogs: %s", err, s.logs.String())
	}

	s.terminate()
	if status := s.waitStatus(45 * time.Second); status != 0 {
		t.Errorf("exit status = %d, want 0\nlogs: %s", status, s.logs.String())
	}
}

func TestSecondaryHealthPortLogsDeprecation(t *testing.T) {
	healthPort := freePort(t)
	s := startService(t, "-port", "0", "-health-port", healthPort)
	mainAddr := s.address(30 * time.Second)

	getHealth(t, mainAddr)
	getHealth(t, net.JoinHostPort("127.0.0.1", healthPort))
	if logs := s.logs.String(); !strings.Contains(logs, "the separate health listener is deprecated") {
		t.Errorf("deprecation warning missing from logs:\n%s", logs)
	}

	s.terminate()
	if status := s.waitStatus(45 * time.Second); status != 0 {
		t.Errorf("exit status = %d, want 0\nlogs: %s", status, s.logs.String())
	}
}

func TestHealthPortZeroServesHealthOnMainPortOnly(t *testing.T) {
	s := startService(t, "-port", "0", "-health-port", "0")
	addr := s.address(30 * time.Second)

	getHealth(t, addr)
	logs := s.logs.String()
	if strings.Contains(logs, "Health check server listening") {
		t.Errorf("health listener unexpectedly started:\n%s", logs)
	}
	if strings.Contains(logs, "the separate health listener is deprecated") {
		t.Errorf("health listener deprecation warning unexpectedly logged:\n%s", logs)
	}

	s.terminate()
	if status := s.waitStatus(45 * time.Second); status != 0 {
		t.Errorf("exit status = %d, want 0\nlogs: %s", status, s.logs.String())
	}
}

func getHealth(t *testing.T, addr string) string {
	t.Helper()
	url := "http://" + addr + "/health"
	deadline := time.Now().Add(30 * time.Second)
	for {
		res, err := http.Get(url) // #nosec G107 -- the test built this URL
		if err == nil {
			raw, readErr := io.ReadAll(res.Body)
			_ = res.Body.Close()
			if readErr != nil {
				t.Fatalf("reading /health: %v", readErr)
			}
			if res.StatusCode != http.StatusOK {
				t.Fatalf("/health status = %d, want 200: %s", res.StatusCode, raw)
			}
			return string(raw)
		}
		if time.Now().After(deadline) {
			t.Fatalf("/health never answered at %s: %v", addr, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// freePort returns a port nothing is listening on, for the one flag whose
// resolved port the service does not log.
func freePort(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	_, port, err := net.SplitHostPort(lis.Addr().String())
	if err != nil {
		t.Fatalf("reading the reserved port: %v", err)
	}
	if err := lis.Close(); err != nil {
		t.Fatalf("releasing the reserved port: %v", err)
	}
	return port
}

// A flag the binary does not define is rejected before anything listens, with a
// nonzero status and a message naming the flag.
func TestUnknownFlagExitsNonzero(t *testing.T) {
	s := startService(t, "-listen", "0")
	if s.waitStatus(30*time.Second) == 0 {
		t.Errorf("exit status = 0, want nonzero\nlogs: %s", s.logs.String())
	}
	if logs := s.logs.String(); !strings.Contains(logs, "flag provided but not defined: -listen") {
		t.Errorf("the service did not report the bad flag:\n%s", logs)
	}
}

// A port already in use ends the process instead of retrying or waiting: the
// supervisor gets a nonzero status and a message naming the port.
func TestOccupiedPortExitsNonzero(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupying a port: %v", err)
	}
	defer func() { _ = lis.Close() }()
	_, port, err := net.SplitHostPort(lis.Addr().String())
	if err != nil {
		t.Fatalf("reading the occupied port: %v", err)
	}

	s := startService(t, "-port", port, "-health-port", "0")
	if status := s.waitStatus(30 * time.Second); status != 1 {
		t.Errorf("exit status = %d, want 1\nlogs: %s", status, s.logs.String())
	}
	logs := s.logs.String()
	if !strings.Contains(logs, "Failed to listen") {
		t.Errorf("the service did not report the failed listen:\n%s", logs)
	}
	if !strings.Contains(logs, "port="+port) {
		t.Errorf("the failed listen did not name port %s:\n%s", port, logs)
	}
}

// A cache size the service cannot honour is a configuration error, reported
// before it listens.
func TestInvalidCacheSizeExitsNonzero(t *testing.T) {
	s := startService(t, "-port", "0", "-health-port", "0", "-cache-size", "0")
	if status := s.waitStatus(30 * time.Second); status != 1 {
		t.Errorf("exit status = %d, want 1\nlogs: %s", status, s.logs.String())
	}
	if logs := s.logs.String(); !strings.Contains(logs, "Invalid service configuration") {
		t.Errorf("the service did not report the bad configuration:\n%s", logs)
	}
}

// A model it cannot read and a hash it never issued are typed errors on the call:
// the service answers the next request and still shuts down cleanly.
func TestMissingModelIsATypedError(t *testing.T) {
	s := startService(t, "-port", "0", "-health-port", "0")
	client := dial(t, s.address(30*time.Second))
	ctx := callContext(t)

	missing := filepath.Join(t.TempDir(), "absent.sysml")
	_, err := client.ParseFile(ctx, &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_FilePath{FilePath: missing},
	})
	if code := status.Code(err); code != codes.NotFound {
		t.Errorf("ParseFile of a missing file = %v (code %s), want NotFound", err, code)
	}

	_, err = client.GetSymbol(ctx, &pb.GetSymbolRequest{ModelHash: "nosuchhash", SymbolId: "test::Vehicle"})
	if code := status.Code(err); code != codes.NotFound {
		t.Errorf("GetSymbol against an unknown model = %v (code %s), want NotFound", err, code)
	}

	if _, err := client.GetServerInfo(ctx, &pb.ServerInfoRequest{}); err != nil {
		t.Fatalf("the service stopped answering after the failed calls: %v\nlogs: %s", err, s.logs.String())
	}

	s.terminate()
	if status := s.waitStatus(45 * time.Second); status != 0 {
		t.Errorf("exit status = %d, want 0\nlogs: %s", status, s.logs.String())
	}
}

// Shutdown with a client still connected completes, and a later call on that
// connection reports the service gone rather than hanging the caller.
func TestShutdownWithAnOpenConnection(t *testing.T) {
	s := startService(t, "-port", "0", "-health-port", "0")
	addr := s.address(30 * time.Second)
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dialling %s: %v", addr, err)
	}
	defer func() { _ = conn.Close() }()
	client := pb.NewSysMLServiceClient(conn)
	if _, err := client.GetServerInfo(callContext(t), &pb.ServerInfoRequest{}); err != nil {
		t.Fatalf("GetServerInfo: %v\nlogs: %s", err, s.logs.String())
	}

	s.terminate()
	if status := s.waitStatus(45 * time.Second); status != 0 {
		t.Errorf("exit status = %d, want 0\nlogs: %s", status, s.logs.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := client.GetServerInfo(ctx, &pb.ServerInfoRequest{}); err == nil {
		t.Error("a call on the open connection succeeded after shutdown")
	} else if status.Code(err) == codes.DeadlineExceeded {
		t.Errorf("a call after shutdown hung until the deadline: %v", err)
	}
}

// -transport grpc serves grpc-go alone: calls succeed, a refused call carries
// the same canonical code and message every transport answers, and a signal
// still ends the process cleanly.
func TestGRPCTransportServesGRPCClients(t *testing.T) {
	s := startService(t, "-transport", "grpc", "-port", "0", "-health-port", "0")
	client := dial(t, s.address(30*time.Second))
	ctx := callContext(t)

	if _, err := client.GetServerInfo(ctx, &pb.ServerInfoRequest{}); err != nil {
		t.Fatalf("GetServerInfo: %v\nlogs: %s", err, s.logs.String())
	}

	_, err := client.Query(ctx, &pb.QueryRequest{ModelHash: "no-such-model"})
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Query against an unknown model = %v, want a gRPC status", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("Query code = %s, want NotFound", st.Code())
	}
	if want := "model not found: no-such-model"; st.Message() != want {
		t.Errorf("Query message = %q, want %q", st.Message(), want)
	}

	s.terminate()
	if status := s.waitStatus(45 * time.Second); status != 0 {
		t.Errorf("exit status = %d, want 0\nlogs: %s", status, s.logs.String())
	}
}
