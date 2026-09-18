package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// client is the protocol-neutral interface used by conformance scenarios.
type client interface {
	protocol() string
	call(context.Context, string, protoreflect.Message) (protoreflect.Message, error)
	close()
}

type grpcClient struct{ conn *grpc.ClientConn }

func (c *grpcClient) protocol() string { return "grpc" }
func (c *grpcClient) close()           { /* the service owns the connection */ }
func (c *grpcClient) call(ctx context.Context, method string, request protoreflect.Message) (protoreflect.Message, error) {
	descriptor, err := methodByName(method)
	if err != nil {
		return nil, err
	}
	if request == nil {
		request = dynamicpb.NewMessage(descriptor.Input())
	}
	response := dynamicpb.NewMessage(descriptor.Output())
	path := fmt.Sprintf("/%s/%s", serviceName, descriptor.Name())
	if err := c.conn.Invoke(ctx, path, request.Interface(), response.Interface()); err != nil {
		return nil, err
	}
	return response, nil
}

type connectClient struct {
	httpClient *http.Client
	baseURL    string
	json       bool
}

func (c *connectClient) protocol() string {
	if c.json {
		return "connect-json"
	}
	return "connect"
}

func (c *connectClient) close() { /* the HTTP client holds nothing to release */ }

func (c *connectClient) call(ctx context.Context, method string, request protoreflect.Message) (protoreflect.Message, error) {
	descriptor, err := methodByName(method)
	if err != nil {
		return nil, err
	}
	if request == nil {
		request = dynamicpb.NewMessage(descriptor.Input())
	}
	var body []byte
	contentType := "application/proto"
	if c.json {
		body, err = protojson.Marshal(request.Interface())
		contentType = "application/json"
	} else {
		body, err = proto.Marshal(request.Interface())
	}
	if err != nil {
		return nil, fmt.Errorf("marshal %s request: %w", method, err)
	}
	url := fmt.Sprintf("%s/%s/%s", c.baseURL, serviceName, descriptor.Name())
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Content-Type", contentType)
	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var problem struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal(data, &problem) != nil {
			return nil, status.Error(codes.Unknown, response.Status)
		}
		return nil, status.Error(connectCode(problem.Code), problem.Message)
	}
	result := dynamicpb.NewMessage(descriptor.Output())
	if c.json {
		err = protojson.Unmarshal(data, result)
	} else {
		err = proto.Unmarshal(data, result)
	}
	if err != nil {
		return nil, fmt.Errorf("unmarshal %s response: %w", method, err)
	}
	return result, nil
}

func connectCode(name string) codes.Code {
	switch strings.ToLower(name) {
	case "canceled":
		return codes.Canceled
	case "unknown":
		return codes.Unknown
	case "invalid_argument":
		return codes.InvalidArgument
	case "deadline_exceeded":
		return codes.DeadlineExceeded
	case "not_found":
		return codes.NotFound
	case "already_exists":
		return codes.AlreadyExists
	case "permission_denied":
		return codes.PermissionDenied
	case "resource_exhausted":
		return codes.ResourceExhausted
	case "failed_precondition":
		return codes.FailedPrecondition
	case "aborted":
		return codes.Aborted
	case "out_of_range":
		return codes.OutOfRange
	case "unimplemented":
		return codes.Unimplemented
	case "internal":
		return codes.Internal
	case "unavailable":
		return codes.Unavailable
	case "data_loss":
		return codes.DataLoss
	case "unauthenticated":
		return codes.Unauthenticated
	default:
		return codes.Unknown
	}
}

// service is a sysml-grpc process the runner started.
type service struct {
	binary  string
	process *exec.Cmd
	conn    *grpc.ClientConn
	target  string
	log     *os.File
	// exited closes when the process has been reaped, so an early crash is seen
	// rather than waited out. waitErr is read only after it closes.
	exited  chan struct{}
	waitErr error
}

// buildService builds cmd/sysml-grpc, so a run tests the working tree rather
// than whatever binary happens to be installed.
func buildService(repoRoot, outDir string) (string, error) {
	binary := filepath.Join(outDir, "sysml-grpc")
	// #nosec G204 -- the command is fixed; only the output path varies.
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/sysml-grpc")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("building cmd/sysml-grpc: %w", err)
	}
	return binary, nil
}

// startService starts the service on a free port and waits until it answers.
func startService(ctx context.Context, binary, logPath string, cacheSize int) (*service, error) {
	return startServiceWithTransport(ctx, binary, logPath, cacheSize, transportConnect)
}

func startServiceWithTransport(ctx context.Context, binary, logPath string, cacheSize int, transport string) (*service, error) {
	return startServiceWithCapabilities(ctx, binary, logPath, cacheSize, transport, nil)
}

func startServiceWithCapabilities(ctx context.Context, binary, logPath string, cacheSize int, transport string, unavailable []string) (*service, error) {
	port, healthPort, err := freePorts()
	if err != nil {
		return nil, err
	}
	log, err := os.Create(logPath) // #nosec G304 -- logPath is the runner's own temporary directory.
	if err != nil {
		return nil, err
	}
	// #nosec G204 -- binary is what the runner just built, or the one named on the command line.
	process := exec.CommandContext(ctx, binary,
		"-port", fmt.Sprint(port),
		"-health-port", fmt.Sprint(healthPort),
		"-cache-size", fmt.Sprint(cacheSize),
		"-log-level", "error",
		"-transport", transport,
	)
	process.Stdout = log
	process.Stderr = log
	process.Env = withoutEnvironment(os.Environ(), testWithholdCapabilitiesEnv)
	if len(unavailable) > 0 {
		process.Env = append(process.Env,
			testWithholdCapabilitiesEnv+"="+strings.Join(unavailable, ","))
	}
	if err := process.Start(); err != nil {
		_ = log.Close()
		return nil, fmt.Errorf("starting %s: %w", binary, err)
	}

	svc := &service{binary: binary, process: process, log: log, exited: make(chan struct{})}
	go func() {
		svc.waitErr = process.Wait()
		close(svc.exited)
	}()
	target := fmt.Sprintf("localhost:%d", port)
	conn, err := grpc.Dial(target, grpc.WithTransportCredentials(insecure.NewCredentials())) //nolint:staticcheck // grpc.NewClient postdates the pinned grpc version
	if err != nil {
		svc.stop()
		return nil, err
	}
	svc.conn = conn
	svc.target = target
	if err := svc.waitReady(ctx, target, logPath); err != nil {
		svc.stop()
		return nil, err
	}
	return svc, nil
}

func withoutEnvironment(environment []string, name string) []string {
	prefix := name + "="
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func (s *service) client(protocol string) (client, error) {
	switch protocol {
	case "grpc":
		return &grpcClient{conn: s.conn}, nil
	case "connect":
		return &connectClient{httpClient: http.DefaultClient, baseURL: "http://" + s.target}, nil
	case "connect-json":
		return &connectClient{httpClient: http.DefaultClient, baseURL: "http://" + s.target, json: true}, nil
	case "pkg":
		api, err := opensysml.New()
		if err != nil {
			return nil, err
		}
		return newPkgClient(protocol, api), nil
	case "pkg-connect":
		api, err := opensysml.Dial(s.target)
		if err != nil {
			return nil, err
		}
		return newPkgClient(protocol, api), nil
	default:
		return nil, fmt.Errorf("unknown protocol %q", protocol)
	}
}

// waitReady calls GetServerInfo until the service answers it.
func (s *service) waitReady(ctx context.Context, target, logPath string) error {
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		select {
		case <-s.exited:
			return fmt.Errorf("the service exited before answering (%v); its log is %s", s.waitErr, logPath)
		default:
		}
		call, cancel := context.WithTimeout(ctx, 2*time.Second)
		_, lastErr = s.call(call, "GetServerInfo", nil)
		cancel()
		if lastErr == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("the service on %s did not answer GetServerInfo (%v); its log is %s", target, lastErr, logPath)
}

// call invokes one method by name, with a request built for its input type. A
// nil request sends an empty message.
func (s *service) call(ctx context.Context, method string, request protoreflect.Message) (protoreflect.Message, error) {
	descriptor, err := methodByName(method)
	if err != nil {
		return nil, err
	}
	if request == nil {
		request = dynamicpb.NewMessage(descriptor.Input())
	}
	response := dynamicpb.NewMessage(descriptor.Output())
	path := fmt.Sprintf("/%s/%s", serviceName, descriptor.Name())
	if err := s.conn.Invoke(ctx, path, request.Interface(), response.Interface()); err != nil {
		return nil, err
	}
	return response, nil
}

// stop closes the connection and terminates the service.
func (s *service) stop() {
	if s.conn != nil {
		_ = s.conn.Close()
	}
	if s.process != nil && s.process.Process != nil {
		_ = s.process.Process.Kill()
		<-s.exited
	}
	if s.log != nil {
		_ = s.log.Close()
	}
}

// freePorts reserves two distinct ports. Both listeners stay open until both
// ports are chosen, so the second cannot be the port the first just released.
func freePorts() (int, int, error) {
	var listeners []net.Listener
	defer func() {
		for _, listener := range listeners {
			_ = listener.Close()
		}
	}()
	var ports []int
	for range 2 {
		listener, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			return 0, 0, err
		}
		listeners = append(listeners, listener)
		ports = append(ports, listener.Addr().(*net.TCPAddr).Port)
	}
	return ports[0], ports[1], nil
}
