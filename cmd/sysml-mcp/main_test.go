package main

import (
	"bytes"
	"strings"
	"testing"
)

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
		stdout: []string{"Usage: sysml-mcp [options]", "-tools", "-root", "-stdio"},
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
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
		})
	}
}
