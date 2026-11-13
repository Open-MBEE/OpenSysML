package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp"
)

// TestTheHTTPTransportAnswersHealthAndReady checks the probes answer open,
// outside the token and the origin check, and the protocol does not.
func TestTheHTTPTransportAnswersHealthAndReady(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "mcp.token")
	if err := os.WriteFile(tokenFile, []byte("s3cret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := startHTTPServer(t, "http", "-token-file", tokenFile)
	base := strings.TrimSuffix(s.url, "/mcp")
	get := func(path, origin string) (int, map[string]string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, base+path, nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		var body map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&body)
		return resp.StatusCode, body
	}
	if code, body := get("/health", "https://evil.example"); code != http.StatusOK || body["status"] != "ok" || body["service"] != "sysml-mcp" || body["version"] == "" {
		t.Errorf("GET /health from a foreign origin, no token: %d %v", code, body)
	}
	if code, body := get("/ready", ""); code != http.StatusOK || body["status"] != "ready" {
		t.Errorf("GET /ready once serving: %d %v", code, body)
	}
	if code, _ := get("/mcp", ""); code != http.StatusUnauthorized {
		t.Errorf("GET /mcp with no token: %d, want 401", code)
	}
	if code := s.stop(t); code != exitServed {
		t.Errorf("exit status %d:\n%s", code, s.stderr)
	}
	if !strings.Contains(s.stderr.String(), "listening for Streamable HTTP") {
		t.Errorf("the listener was not logged before the server was built:\n%s", s.stderr)
	}
}

// TestTheHTTPTransportBoundsTheRequestBody checks -max-body-bytes is enforced
// at /mcp with 413.
func TestTheHTTPTransportBoundsTheRequestBody(t *testing.T) {
	s := startHTTPServer(t, "http", "-max-body-bytes", "128")
	req, _ := http.NewRequest(http.MethodPost, s.url, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"`+strings.Repeat("x", 200)+`","version":"0"}}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("a body over -max-body-bytes: %d, want 413", resp.StatusCode)
	}
	s.stop(t)
}

// TestFlagsReadTheEnvironment checks every flag is read from OPENSYSML_MCP_<FLAG>,
// the flag wins over the variable, and a refused variable exits 2 with the flag's message.
func TestFlagsReadTheEnvironment(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "mcp.token")
	if err := os.WriteFile(tokenFile, []byte("from-the-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	host := hostNames()[0]
	for _, tc := range []struct {
		name   string
		env    map[string]string
		args   []string
		status int
		stdout []string
		stderr []string
	}{
		{name: "an output flag from the environment", env: map[string]string{"OPENSYSML_MCP_VERSION": "true"},
			args: nil, status: exitServed, stdout: []string{"sysml-mcp"}},
		{name: "an output flag on the command line beats a refused variable", env: map[string]string{"OPENSYSML_MCP_CACHE_SIZE": "many"},
			args: []string{"-version"}, status: exitServed, stdout: []string{"sysml-mcp"}},
		{name: "-print-config from the environment takes the host that follows", env: map[string]string{"OPENSYSML_MCP_PRINT_CONFIG": "true", "OPENSYSML_MCP_HTTP": "127.0.0.1:9999"},
			args: []string{host}, status: exitServed, stdout: []string{"http://127.0.0.1:9999/mcp"}},
		{name: "a flag after the host wins over a refused variable", env: map[string]string{"OPENSYSML_MCP_PRINT_CONFIG": "true", "OPENSYSML_MCP_CACHE_SIZE": "many"},
			args: []string{host, "-cache-size", "100"}, status: exitServed, stdout: []string{`"command"`}},
		{name: "an empty argument is refused", env: nil,
			args: []string{""}, status: exitUnservable, stderr: []string{`unexpected argument ""`}},
		{name: "an empty argument after the host is refused", env: nil,
			args: []string{"-print-config=" + host, ""}, status: exitUnservable, stderr: []string{`unexpected argument "" after -print-config ` + host}},
		{name: "an empty host is refused", env: nil,
			args: []string{"-print-config", ""}, status: exitUnservable, stderr: []string{`unexpected argument "" after -print-config`}},
		{name: "a second argument after the host is refused", env: map[string]string{"OPENSYSML_MCP_PRINT_CONFIG": "true"},
			args: []string{host, "extra"}, status: exitUnservable, stderr: []string{`unexpected argument "extra" after -print-config ` + host}},
		{name: "a flag after the host wins over the variable", env: map[string]string{"OPENSYSML_MCP_HTTP": "127.0.0.1:9999"},
			args: []string{"-print-config", host, "-http", "127.0.0.1:8888"}, status: exitServed, stdout: []string{"http://127.0.0.1:8888/mcp"}},
		{name: "a variable sets the flag", env: map[string]string{"OPENSYSML_MCP_HTTP": "127.0.0.1:9999"},
			args: []string{"-print-config", host}, status: exitServed, stdout: []string{"http://127.0.0.1:9999/mcp"}},
		{name: "the flag wins", env: map[string]string{"OPENSYSML_MCP_HTTP": "127.0.0.1:9999"},
			args: []string{"-http", "127.0.0.1:8888", "-print-config", host}, status: exitServed, stdout: []string{"http://127.0.0.1:8888/mcp"}},
		{name: "an empty variable is unset", env: map[string]string{"OPENSYSML_MCP_LOG_LEVEL": ""},
			args: []string{"-print-config", host}, status: exitServed},
		{name: "a refused value exits 2", env: map[string]string{"OPENSYSML_MCP_CACHE_SIZE": "many"},
			args: []string{"-print-config", host}, status: exitUnservable, stderr: []string{`invalid value "many" for OPENSYSML_MCP_CACHE_SIZE`, "-cache-size", "parse error"}},
		{name: "a refused value exits 2 with the flag's own check", env: map[string]string{"OPENSYSML_MCP_LOG_LEVEL": "loud", "OPENSYSML_MCP_HTTP": "127.0.0.1:0"},
			args: nil, status: exitUnservable, stderr: []string{"-log-level"}},
		{name: "a bool from the environment", env: map[string]string{"OPENSYSML_MCP_PRINT_CONFIG_NPX": "true", "OPENSYSML_MCP_HTTP": "127.0.0.1:0"},
			args: []string{"-print-config", host}, status: exitUnservable, stderr: []string{"-print-config-npx and -http"}},
		{name: "the token file from the environment", env: map[string]string{"OPENSYSML_MCP_TOKEN_FILE": tokenFile, "OPENSYSML_MCP_HTTP": "127.0.0.1:9999"},
			args: []string{"-print-config", host}, status: exitServed, stdout: []string{"Bearer from-the-file"}},
		{name: "the token flag wins over the file variable", env: map[string]string{"OPENSYSML_MCP_TOKEN_FILE": tokenFile, "OPENSYSML_MCP_HTTP": "127.0.0.1:9999"},
			args: []string{"-token", "from-the-flag", "-print-config", host}, status: exitServed, stdout: []string{"Bearer from-the-flag"}},
		{name: "the file variable wins over the token variable", env: map[string]string{"OPENSYSML_MCP_TOKEN_FILE": tokenFile, mcp.TokenEnv: "from-the-variable", "OPENSYSML_MCP_HTTP": "127.0.0.1:9999"},
			args: []string{"-print-config", host}, status: exitServed, stdout: []string{"Bearer from-the-file"}},
		{name: "a missing token file from the environment", env: map[string]string{"OPENSYSML_MCP_TOKEN_FILE": filepath.Join(dir, "none"), "OPENSYSML_MCP_HTTP": "127.0.0.1:9999"},
			args: []string{"-print-config", host}, status: exitUnservable, stderr: []string{"OPENSYSML_MCP_TOKEN_FILE", "-token-file"}},
		{name: "the shutdown timeout must be positive", env: map[string]string{"OPENSYSML_MCP_SHUTDOWN_TIMEOUT": "0s", "OPENSYSML_MCP_HTTP": "127.0.0.1:9999"},
			args: []string{"-print-config", host}, status: exitUnservable, stderr: []string{"-shutdown-timeout 0s"}},
		{name: "the body bound must be positive", env: map[string]string{"OPENSYSML_MCP_MAX_BODY_BYTES": "0", "OPENSYSML_MCP_HTTP": "127.0.0.1:9999"},
			args: []string{"-print-config", host}, status: exitUnservable, stderr: []string{"-max-body-bytes 0"}},
		{name: "the bounds apply to the HTTP transport", env: map[string]string{"OPENSYSML_MCP_MAX_BODY_BYTES": "1024"},
			args: []string{"-print-config", host}, status: exitUnservable, stderr: []string{"-max-body-bytes applies to the HTTP transport"}},
		{name: "the bounds apply to the HTTP transport, as flags", env: nil,
			args: []string{"-shutdown-timeout", "5s", "-print-config", host}, status: exitUnservable, stderr: []string{"-shutdown-timeout applies to the HTTP transport"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, name := range []string{"OPENSYSML_MCP_HTTP", "OPENSYSML_MCP_TOKEN_FILE", mcp.TokenEnv} {
				t.Setenv(name, "")
				os.Unsetenv(name)
			}
			for name, value := range tc.env {
				t.Setenv(name, value)
			}
			var stdout, stderr bytes.Buffer
			if got := run(tc.args, &stdout, &stderr); got != tc.status {
				t.Fatalf("exit %d, want %d\nstdout: %s\nstderr: %s", got, tc.status, &stdout, &stderr)
			}
			for _, want := range tc.stdout {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("stdout lacks %q:\n%s", want, &stdout)
				}
			}
			for _, want := range tc.stderr {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr lacks %q:\n%s", want, &stderr)
				}
			}
		})
	}
}
