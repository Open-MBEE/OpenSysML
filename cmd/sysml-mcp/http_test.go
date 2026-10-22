package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
)

// httpServer is the binary serving over Streamable HTTP for one test.
type httpServer struct {
	cmd *exec.Cmd
	// url is the endpoint clients connect to.
	url string
	// stderr collects what the server logged, for assertions and for failures.
	stderr *syncBuffer
	// exited is closed when the process has exited.
	exited chan struct{}
}

// syncBuffer is a bytes.Buffer safe to write from the stderr reader and read
// from the test.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// startHTTPServer starts the binary with -http on a free loopback port over a
// one-document workspace and reads the bound address from its log.
func startHTTPServer(t *testing.T, scheme string, args ...string) *httpServer {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "m.sysml"), []byte("package M { part def P; }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(serverBinary(t), append([]string{"-http", "127.0.0.1:0", "-root", root, "-log-level", "info"}, args...)...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the server: %v", err)
	}
	s := &httpServer{cmd: cmd, stderr: &syncBuffer{}, exited: make(chan struct{})}
	t.Cleanup(func() {
		select {
		case <-s.exited:
		default:
			_ = cmd.Process.Kill()
			<-s.exited
		}
	})
	addr := make(chan string, 1)
	go func() {
		defer close(s.exited)
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			s.stderr.Write([]byte(line + "\n"))
			if _, rest, ok := strings.Cut(line, " addr="); ok && strings.Contains(line, "serving the Model Context Protocol") {
				addr <- strings.Fields(rest)[0]
			}
		}
		_ = cmd.Wait()
	}()
	select {
	case a := <-addr:
		s.url = scheme + "://" + a + "/mcp"
	case <-s.exited:
		t.Fatalf("the server exited before serving:\n%s", s.stderr)
	case <-time.After(2 * time.Minute):
		t.Fatalf("the server did not report its address:\n%s", s.stderr)
	}
	return s
}

// stop sends SIGTERM and returns the exit status, failing when the process
// outlives the shutdown's bound.
func (s *httpServer) stop(t *testing.T) int {
	t.Helper()
	if err := s.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("SIGTERM: %v", err)
	}
	select {
	case <-s.exited:
	case <-time.After(45 * time.Second):
		t.Fatalf("the server did not exit on SIGTERM:\n%s", s.stderr)
	}
	return s.cmd.ProcessState.ExitCode()
}

// connectHTTP opens a client session over the Streamable HTTP transport.
func connectHTTP(ctx context.Context, t *testing.T, url string, client *http.Client) *sdk.ClientSession {
	t.Helper()
	c := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := c.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: url, HTTPClient: client}, nil)
	if err != nil {
		t.Fatalf("Connect %s: %v", url, err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// callHTTP calls a tool and decodes its structured content into out, failing
// on a transport error or a tool error.
func callHTTP(ctx context.Context, t *testing.T, cs *sdk.ClientSession, name string, args any, out any) {
	t.Helper()
	res, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s failed: %+v", name, res.Content)
	}
	if out == nil {
		return
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("%s: structured content %s: %v", name, raw, err)
	}
}

func TestTheBinaryServesTheProtocolOverHTTP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s := startHTTPServer(t, "http")
	if log := s.stderr.String(); !strings.Contains(log, "path=/mcp") || !strings.Contains(log, "security=cleartext") || strings.Contains(log, "no authentication") {
		t.Errorf("a loopback cleartext server should log its path and security and no warning:\n%s", log)
	}

	cs := connectHTTP(ctx, t, s.url, nil)
	if info := cs.InitializeResult().ServerInfo; info.Name != "sysml-mcp" {
		t.Errorf("server info = %+v", info)
	}
	if cs.InitializeResult().Instructions == "" {
		t.Error("the server gave the client no instructions")
	}
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var names []string
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	if want := catalog.Names(catalog.Groups()...); !slices.Equal(names, want) {
		t.Errorf("tools = %v, want the whole catalog %v", names, want)
	}

	// One call in each group: the workspace (lsp), the runtime and a session.
	var info struct {
		Documents []string `json:"documents"`
	}
	callHTTP(ctx, t, cs, "workspace_info", map[string]any{}, &info)
	if len(info.Documents) != 1 || filepath.Base(info.Documents[0]) != "m.sysml" {
		t.Errorf("documents = %v", info.Documents)
	}
	var server struct {
		Capabilities []string `json:"capabilities"`
	}
	callHTTP(ctx, t, cs, "server_info", map[string]any{}, &server)
	if !slices.Contains(server.Capabilities, "parse_sources") {
		t.Errorf("capabilities = %v", server.Capabilities)
	}
	var opened struct {
		Session string `json:"session"`
	}
	callHTTP(ctx, t, cs, "session_open", map[string]any{"session": "shared"}, &opened)
	if opened.Session != "shared" {
		t.Errorf("opened %q", opened.Session)
	}
	callHTTP(ctx, t, cs, "session_submit", map[string]any{"session": "shared", "text": "attribute answer = 42;"}, nil)

	// A second client, connected at the same time, sees the session the first
	// opened under the name it gave it: the manager is one per process.
	other := connectHTTP(ctx, t, s.url, nil)
	var list struct {
		Sessions []string `json:"sessions"`
	}
	callHTTP(ctx, t, other, "session_list", map[string]any{}, &list)
	if !slices.Equal(list.Sessions, []string{"shared"}) {
		t.Errorf("the second client lists sessions %v, want [shared]", list.Sessions)
	}
	var value struct {
		Text string `json:"text"`
	}
	callHTTP(ctx, t, other, "session_eval", map[string]any{"session": "shared", "expression": "answer"}, &value)
	if !strings.Contains(value.Text, "42") {
		t.Errorf("the second client evaluates answer as %q", value.Text)
	}
	// Both clients calling into the one session at once are served in turn.
	var wg sync.WaitGroup
	for _, client := range []*sdk.ClientSession{cs, other} {
		for i := range 5 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var v struct {
					Text string `json:"text"`
				}
				res, err := client.CallTool(ctx, &sdk.CallToolParams{Name: "session_eval", Arguments: map[string]any{"session": "shared", "expression": fmt.Sprintf("answer + %d", i)}})
				if err != nil || res.IsError {
					t.Errorf("session_eval answer + %d: %v %+v", i, err, res)
					return
				}
				raw, _ := json.Marshal(res.StructuredContent)
				_ = json.Unmarshal(raw, &v)
				if !strings.Contains(v.Text, fmt.Sprint(42+i)) {
					t.Errorf("answer + %d = %q", i, v.Text)
				}
			}()
		}
	}
	wg.Wait()
	if res, err := other.CallTool(ctx, &sdk.CallToolParams{Name: "session_open", Arguments: map[string]any{"session": "shared"}}); err != nil || !res.IsError {
		t.Errorf("opening the shared session again from the second client should fail: %v %+v", err, res)
	}

	// SIGTERM stops the server with the clients' sessions live: the sessions
	// are closed, the connections drain and the process exits served.
	if code := s.stop(t); code != exitServed {
		t.Errorf("exit status = %d, want %d:\n%s", code, exitServed, s.stderr)
	}
	if log := s.stderr.String(); !strings.Contains(log, "stopped serving over Streamable HTTP") {
		t.Errorf("the server did not log its stop:\n%s", log)
	}
	if _, err := cs.ListTools(ctx, nil); err == nil {
		t.Error("a call after the server stopped succeeded")
	}
}

// originTransport sets an Origin header on every request, as a browser does.
type originTransport struct {
	origin string
}

func (o originTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Origin", o.origin)
	return http.DefaultTransport.RoundTrip(r)
}

func TestTheHTTPTransportRefusesUnknownOrigins(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s := startHTTPServer(t, "http", "-allow-origin", "https://agents.example.org")

	// A loopback origin and the allowed one are served; any other is refused
	// before the protocol is reached.
	for _, origin := range []string{"http://localhost:3000", "http://127.0.0.1", "https://agents.example.org", "https://AGENTS.example.org"} {
		cs := connectHTTP(ctx, t, s.url, &http.Client{Transport: originTransport{origin}})
		if _, err := cs.ListTools(ctx, nil); err != nil {
			t.Errorf("origin %s: ListTools: %v", origin, err)
		}
	}
	for _, origin := range []string{"https://evil.example.org", "http://agents.example.org", "https://agents.example.org:8443", "null"} {
		c := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
		cs, err := c.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: s.url, HTTPClient: &http.Client{Transport: originTransport{origin}}}, nil)
		if err == nil {
			_ = cs.Close()
			t.Errorf("origin %s was served", origin)
			continue
		}
		if !strings.Contains(err.Error(), "Forbidden") {
			t.Errorf("origin %s: refused with %v, want Forbidden", origin, err)
		}
	}

	// The refusal is the status a browser's preflight or request sees.
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.url, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Origin", "https://evil.example.org")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden || !strings.Contains(string(body), "-allow-origin") {
		t.Errorf("status %d, body %q", resp.StatusCode, body)
	}
}

// writeTestCertificate writes a self-signed certificate for 127.0.0.1 and
// its key under dir, and returns their paths with a pool trusting it.
func writeTestCertificate(t *testing.T, dir string) (certFile, keyFile string, roots *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "sysml-mcp test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certFile = filepath.Join(dir, "server.crt")
	keyFile = filepath.Join(dir, "server.key")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots = x509.NewCertPool()
	roots.AddCert(cert)
	return certFile, keyFile, roots
}

func TestTheHTTPTransportServesTLS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	certFile, keyFile, roots := writeTestCertificate(t, t.TempDir())
	s := startHTTPServer(t, "https", "-tls-cert", certFile, "-tls-key", keyFile)
	if log := s.stderr.String(); !strings.Contains(log, "security=TLS") {
		t.Errorf("the server did not log that it serves TLS:\n%s", log)
	}

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}}
	cs := connectHTTP(ctx, t, s.url, client)
	if info := cs.InitializeResult().ServerInfo; info.Name != "sysml-mcp" {
		t.Errorf("server info = %+v", info)
	}
	var server struct {
		Version string `json:"version"`
	}
	callHTTP(ctx, t, cs, "server_info", map[string]any{}, &server)
	if server.Version == "" {
		t.Error("server_info reported no version")
	}

	// Cleartext at the TLS port is not the protocol.
	c := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	if plain, err := c.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: strings.Replace(s.url, "https://", "http://", 1)}, nil); err == nil {
		_ = plain.Close()
		t.Error("a cleartext client connected to the TLS server")
	}
	if code := s.stop(t); code != exitServed {
		t.Errorf("exit status = %d, want %d:\n%s", code, exitServed, s.stderr)
	}
}

func TestTheHTTPTransportRefusesOffLoopbackWithoutAToken(t *testing.T) {
	// A bound address that is not loopback is reachable from other machines,
	// so it is served only to clients that hold the token.
	cmd := exec.Command(serverBinary(t), "-http", "0.0.0.0:0", "-root", t.TempDir())
	cmd.Env = append(os.Environ(), mcp.TokenEnv+"=")
	out, err := cmd.CombinedOutput()
	if code := cmd.ProcessState.ExitCode(); err == nil || code != exitUnservable {
		t.Errorf("exit status = %d, want %d:\n%s", code, exitUnservable, out)
	}
	for _, want := range []string{"sysml-mcp: -http 0.0.0.0:0 binds an address other machines reach", "-token-file", "-token", mcp.TokenEnv, "bind loopback"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the refusal is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(string(out), "serving the Model Context Protocol") {
		t.Errorf("the server served before refusing:\n%s", out)
	}
}

// TestTheHTTPTransportRequiresTheToken checks every interface is served with a
// file's token to clients that send it, with the RFC 6750 challenge and metadata.
func TestTheHTTPTransportRequiresTheToken(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	tokenFile := filepath.Join(t.TempDir(), "mcp.token")
	if err := os.WriteFile(tokenFile, []byte("s3cret-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := startHTTPServer(t, "http", "-http", "0.0.0.0:0", "-token-file", tokenFile)
	if log := s.stderr.String(); !strings.Contains(log, "auth=bearer") || strings.Contains(log, "s3cret") {
		t.Errorf("log:\n%s", log)
	}
	// The bound address is every interface; the client reaches it by loopback.
	_, port, err := net.SplitHostPort(strings.TrimSuffix(strings.TrimPrefix(s.url, "http://"), mcp.HTTPPath))
	if err != nil {
		t.Fatal(err)
	}
	s.url = "http://" + net.JoinHostPort("127.0.0.1", port) + mcp.HTTPPath

	cs := connectHTTP(ctx, t, s.url, &http.Client{Transport: headerTransport{"Authorization", "Bearer s3cret-token"}})
	var server struct {
		Version string `json:"version"`
	}
	callHTTP(ctx, t, cs, "server_info", map[string]any{}, &server)
	if server.Version == "" {
		t.Error("server_info reported no version")
	}

	for _, bad := range []string{"", "Bearer wrong", "Basic czNjcmV0"} {
		c := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
		var client *http.Client
		if bad != "" {
			client = &http.Client{Transport: headerTransport{"Authorization", bad}}
		}
		if cs, err := c.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: s.url, HTTPClient: client}, nil); err == nil {
			_ = cs.Close()
			t.Errorf("Authorization %q was served", bad)
		}
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.url, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer wrong")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	metadataURL := strings.TrimSuffix(s.url, mcp.HTTPPath) + mcp.ProtectedResourceMetadataPath
	challenge := resp.Header.Get("WWW-Authenticate")
	if resp.StatusCode != http.StatusUnauthorized || !strings.HasPrefix(challenge, "Bearer ") || !strings.Contains(challenge, `error="invalid_token"`) || !strings.Contains(challenge, `resource_metadata="`+metadataURL+`"`) {
		t.Errorf("status %d, WWW-Authenticate %q, body %q", resp.StatusCode, challenge, body)
	}

	resp, err = http.Get(metadataURL)
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		Resource string   `json:"resource"`
		Methods  []string `json:"bearer_methods_supported"`
	}
	err = json.NewDecoder(resp.Body).Decode(&meta)
	resp.Body.Close()
	if err != nil || resp.StatusCode != http.StatusOK || meta.Resource != s.url || !slices.Equal(meta.Methods, []string{"header"}) {
		t.Errorf("metadata: status %d, %+v, %v", resp.StatusCode, meta, err)
	}
	if code := s.stop(t); code != exitServed {
		t.Errorf("exit status = %d, want %d:\n%s", code, exitServed, s.stderr)
	}
	if strings.Contains(s.stderr.String(), "s3cret") {
		t.Errorf("the token was logged:\n%s", s.stderr)
	}
}

// TestTheHTTPTransportReadsTheTokenFromTheEnvironment checks the environment
// variable stands in for the flags, on loopback too.
func TestTheHTTPTransportReadsTheTokenFromTheEnvironment(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	t.Setenv(mcp.TokenEnv, "from-env")
	s := startHTTPServer(t, "http")
	c := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	if cs, err := c.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: s.url}, nil); err == nil {
		_ = cs.Close()
		t.Error("a client with no token was served on loopback with the variable set")
	}
	cs := connectHTTP(ctx, t, s.url, &http.Client{Transport: headerTransport{"Authorization", "Bearer from-env"}})
	if _, err := cs.ListTools(ctx, nil); err != nil {
		t.Errorf("ListTools with the token: %v", err)
	}
	if code := s.stop(t); code != exitServed {
		t.Errorf("exit status = %d, want %d:\n%s", code, exitServed, s.stderr)
	}
}

// headerTransport sends one header on every request.
type headerTransport struct{ name, value string }

func (h headerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set(h.name, h.value)
	return http.DefaultTransport.RoundTrip(r)
}

func TestTheHTTPAddressIsRefusedWhenInUse(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()
	cmd := exec.Command(serverBinary(t), "-http", lis.Addr().String(), "-root", t.TempDir())
	out, err := cmd.CombinedOutput()
	if code := cmd.ProcessState.ExitCode(); err == nil || code != exitProtocol {
		t.Errorf("exit status = %d, want %d:\n%s", code, exitProtocol, out)
	}
	if !strings.Contains(string(out), "sysml-mcp: -http "+lis.Addr().String()) {
		t.Errorf("the address in use was not reported:\n%s", out)
	}
}
