package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp"
)

// TestHTTPAddress checks how -http is read: a port alone or an address with
// no host binds loopback, anything else is bound as written.
func TestHTTPAddress(t *testing.T) {
	for spec, want := range map[string]string{
		"8765":                   "localhost:8765",
		":8765":                  "localhost:8765",
		"127.0.0.1:8765":         "127.0.0.1:8765",
		"[::1]:8765":             "[::1]:8765",
		"0.0.0.0:8765":           "0.0.0.0:8765",
		"[::]:8765":              "[::]:8765",
		"models.example.org:443": "models.example.org:443",
		"localhost:0":            "localhost:0",
	} {
		got, err := httpAddress(spec)
		if err != nil || got != want {
			t.Errorf("httpAddress(%q) = %q, %v, want %q", spec, got, err, want)
		}
	}
	for _, bad := range []string{"", "nope", "localhost", "localhost:", "localhost:70000", "localhost:http", "http://localhost:8765", "127.0.0.1:8765/mcp"} {
		if got, err := httpAddress(bad); err == nil {
			t.Errorf("httpAddress(%q) = %q, want an error", bad, got)
		}
	}
}

// TestCommandLine checks what the server does with a command line before it
// speaks the protocol: every spelling of the version flag is answered, and
// anything it could not read is a usage on stderr rather than protocol mode.
func TestCommandLine(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		status int
		stdout []string
		stderr []string
	}{{
		name:   "-version reports the version",
		args:   []string{"-version"},
		status: exitServed,
		stdout: []string{"sysml-mcp ", "Commit:", "Build time:", "Go version:"},
	}, {
		name:   "-v reports the version",
		args:   []string{"-v"},
		status: exitServed,
		stdout: []string{"sysml-mcp "},
	}, {
		name:   "the help asked for is a result on stdout",
		args:   []string{"-h"},
		status: exitServed,
		stdout: []string{"Usage: sysml-mcp [options]", "-tools", "-root", "-stdio", "-http", "-allow-origin", "-token", "-token-file", "-tls-cert", "-tls-key", "/mcp", "OPENSYSML_MCP_TOKEN"},
	}, {
		name:   "a flag that is not defined is reported with the usage",
		args:   []string{"-nosuchflag"},
		status: exitUnservable,
		stderr: []string{"flag provided but not defined: -nosuchflag", "Usage: sysml-mcp [options]"},
	}, {
		name:   "an argument that is not a flag is reported with the usage",
		args:   []string{"model.sysml"},
		status: exitUnservable,
		stderr: []string{`sysml-mcp: unexpected argument "model.sysml"`, "Usage: sysml-mcp [options]"},
	}, {
		name:   "an unknown tool group is reported with the usage",
		args:   []string{"-tools", "lsp,debugger"},
		status: exitUnservable,
		stderr: []string{`sysml-mcp: -tools: unknown tool group "debugger"`, "Usage: sysml-mcp [options]"},
	}, {
		name:   "a root that is not a directory is reported",
		args:   []string{"-root", "main.go"},
		status: exitUnservable,
		stderr: []string{`sysml-mcp: -root "main.go" is not a directory`, "Usage: sysml-mcp [options]"},
	}, {
		name:   "a cache size with no room is reported with the usage",
		args:   []string{"-cache-size", "0"},
		status: exitUnservable,
		stderr: []string{"sysml-mcp: -cache-size 0: the runtime tools need room for at least one model", "Usage: sysml-mcp [options]"},
	}, {
		name:   "an unknown log level is reported with the usage",
		args:   []string{"-log-level", "loud"},
		status: exitUnservable,
		stderr: []string{`sysml-mcp: -log-level: unknown level "loud"`, "Usage: sysml-mcp [options]"},
	}, {
		name:   "an -http address that is not one is reported with the usage",
		args:   []string{"-http", "nope"},
		status: exitUnservable,
		stderr: []string{`sysml-mcp: -http: "nope" is not a port or a host:port address`, "Usage: sysml-mcp [options]"},
	}, {
		name:   "an -http port that is not one is reported with the usage",
		args:   []string{"-http", "localhost:http-alt"},
		status: exitUnservable,
		stderr: []string{`sysml-mcp: -http: "http-alt" is not a port`, "Usage: sysml-mcp [options]"},
	}, {
		name:   "-stdio beside -http is reported with the usage",
		args:   []string{"-stdio", "-http", "8765"},
		status: exitUnservable,
		stderr: []string{"sysml-mcp: -stdio and -http name different transports; give one", "Usage: sysml-mcp [options]"},
	}, {
		name:   "a TLS certificate without its key is reported with the usage",
		args:   []string{"-http", "8765", "-tls-cert", "server.crt"},
		status: exitUnservable,
		stderr: []string{"sysml-mcp: -tls-cert and -tls-key must be supplied together", "Usage: sysml-mcp [options]"},
	}, {
		name:   "a TLS pair that does not load is reported with the usage",
		args:   []string{"-http", "8765", "-tls-cert", "nosuch.crt", "-tls-key", "nosuch.key"},
		status: exitUnservable,
		stderr: []string{`sysml-mcp: -tls-cert "nosuch.crt" with -tls-key "nosuch.key": `, "Usage: sysml-mcp [options]"},
	}, {
		name:   "a TLS pair without -http is reported with the usage",
		args:   []string{"-tls-cert", "server.crt", "-tls-key", "server.key"},
		status: exitUnservable,
		stderr: []string{"sysml-mcp: -tls-cert applies to the HTTP transport; name its address with -http", "Usage: sysml-mcp [options]"},
	}, {
		name:   "-allow-origin without -http is reported with the usage",
		args:   []string{"-allow-origin", "https://agents.example.org"},
		status: exitUnservable,
		stderr: []string{"sysml-mcp: -allow-origin applies to the HTTP transport; name its address with -http", "Usage: sysml-mcp [options]"},
	}, {
		name:   "-token without -http is reported with the usage",
		args:   []string{"-token", "s3cret"},
		status: exitUnservable,
		stderr: []string{"sysml-mcp: -token applies to the HTTP transport; name its address with -http", "Usage: sysml-mcp [options]"},
	}, {
		name:   "-token-file without -http is reported with the usage",
		args:   []string{"-token-file", "mcp.token"},
		status: exitUnservable,
		stderr: []string{"sysml-mcp: -token-file applies to the HTTP transport; name its address with -http", "Usage: sysml-mcp [options]"},
	}, {
		name:   "-token beside -token-file is reported with the usage",
		args:   []string{"-http", "8765", "-token", "s3cret", "-token-file", "mcp.token"},
		status: exitUnservable,
		stderr: []string{"sysml-mcp: -token and -token-file both name the bearer token; give one", "Usage: sysml-mcp [options]"},
	}, {
		name:   "a token file that does not exist is reported with the usage",
		args:   []string{"-http", "8765", "-token-file", "nosuch.token"},
		status: exitUnservable,
		stderr: []string{"sysml-mcp: -token-file: open nosuch.token: ", "Usage: sysml-mcp [options]"},
	}, {
		name:   "a token with a space is reported with the usage, unquoted",
		args:   []string{"-http", "8765", "-token", "not a token"},
		status: exitUnservable,
		stderr: []string{"sysml-mcp: -token: a token is letters, digits and -._~+/ with trailing = only (RFC 6750 b64token)", "Usage: sysml-mcp [options]"},
	}, {
		name:   "the wildcard origin is reported with the usage",
		args:   []string{"-http", "8765", "-allow-origin", "*"},
		status: exitUnservable,
		stderr: []string{"sysml-mcp: -allow-origin: the wildcard origin is not allowed; name each origin", "Usage: sysml-mcp [options]"},
	}, {
		name:   "an origin that is not one is reported with the usage",
		args:   []string{"-http", "8765", "-allow-origin", "agents.example.org"},
		status: exitUnservable,
		stderr: []string{`sysml-mcp: -allow-origin: "agents.example.org" is not an origin`, "Usage: sysml-mcp [options]"},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(mcp.TokenEnv, "")
			os.Unsetenv(mcp.TokenEnv)
			var stdout, stderr bytes.Buffer
			status := run(tc.args, &stdout, &stderr)
			if status != tc.status {
				t.Errorf("exit status = %d, want %d\n%s%s", status, tc.status, stdout.String(), stderr.String())
			}
			for _, want := range tc.stdout {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("stdout is missing %q:\n%s", want, stdout.String())
				}
			}
			for _, want := range tc.stderr {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr is missing %q:\n%s", want, stderr.String())
				}
			}
			if strings.Contains(stderr.String(), "not a token") || strings.Contains(stderr.String(), "s3cret") {
				t.Errorf("stderr repeats the token:\n%s", stderr.String())
			}
		})
	}
}

// TestReadToken checks where the token comes from: a file (its line break
// dropped), the flag, or the environment, each validated the same way.
func TestReadToken(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	t.Setenv(mcp.TokenEnv, "")
	os.Unsetenv(mcp.TokenEnv)
	for _, tc := range []struct {
		name  string
		opts  options
		env   *string
		value string
		err   string
	}{
		{name: "none", opts: options{}},
		{name: "flag", opts: options{token: "s3cret"}, value: "s3cret"},
		{name: "file", opts: options{tokenFile: write("lf", "s3cret\n")}, value: "s3cret"},
		{name: "file with CRLF", opts: options{tokenFile: write("crlf", "s3cret\r\n")}, value: "s3cret"},
		{name: "file with no line break", opts: options{tokenFile: write("bare", "s3cret")}, value: "s3cret"},
		{name: "empty file", opts: options{tokenFile: write("empty", "\n")}, err: "the token is empty"},
		{name: "file with two lines", opts: options{tokenFile: write("two", "a\nb\n")}, err: "no spaces or line breaks"},
		{name: "environment", env: ptr("from-env"), value: "from-env"},
		{name: "empty environment", env: ptr("")},
		{name: "environment with a space", env: ptr("not a token"), err: mcp.TokenEnv + ": a token is letters"},
		{name: "flag over environment", opts: options{token: "s3cret"}, env: ptr("from-env"), value: "s3cret"},
		{name: "file over environment", opts: options{tokenFile: write("over", "s3cret\n")}, env: ptr("from-env"), value: "s3cret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env != nil {
				t.Setenv(mcp.TokenEnv, *tc.env)
			}
			token, text, err := readToken(&tc.opts)
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("readToken: %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if text != tc.value || token.Set() != (tc.value != "") {
				t.Errorf("readToken = %q (set %v), want %q", text, token.Set(), tc.value)
			}
		})
	}
}

func ptr(s string) *string { return &s }
