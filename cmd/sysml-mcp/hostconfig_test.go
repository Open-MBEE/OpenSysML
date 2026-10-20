package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const testCommand = `C:\Tools\OpenSysML\sysml-mcp.exe`

// printed runs the command line and returns stdout, failing on any other
// outcome than a served result with an empty stderr.
func printed(t *testing.T, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if status := run(args, &stdout, &stderr); status != exitServed || stderr.Len() > 0 {
		t.Fatalf("%v: exit status %d, stderr:\n%s", args, status, stderr.String())
	}
	return stdout.String()
}

// decodeJSON parses a block as JSON and returns the value at a dotted path.
func decodeJSON(t *testing.T, block, path string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(block), &v); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, block)
	}
	for _, key := range strings.Split(path, ".") {
		obj, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("%s: %q is not an object in\n%s", path, key, block)
		}
		if v, ok = obj[key]; !ok {
			t.Fatalf("%s: no %q in\n%s", path, key, block)
		}
	}
	return v
}

// TestPrintConfigBlocks checks that every host's block parses in the format
// the host reads and declares the server under the keys the host looks up.
func TestPrintConfigBlocks(t *testing.T) {
	checkPrintConfigBlocks(t, []string{"-print-config-command", testCommand}, testCommand, []any{})
}

// TestPrintConfigNpxBlocks checks that -print-config-npx makes every host's
// block start the server through npm: npx -y @openmbee/sysml-mcp, with the
// package's arguments in the host's argument list.
func TestPrintConfigNpxBlocks(t *testing.T) {
	checkPrintConfigBlocks(t, []string{"-print-config-npx"}, "npx", []any{"-y", "@openmbee/sysml-mcp"})
}

// checkPrintConfigBlocks prints every host's block with the flags given and
// checks each parses in the format the host reads and declares the server
// under the keys the host looks up, as command with args.
func checkPrintConfigBlocks(t *testing.T, flags []string, command string, args []any) {
	t.Helper()
	argv := append([]any{command}, args...)
	server := func(block, path string) {
		t.Helper()
		if got := decodeJSON(t, block, path+".command"); got != command {
			t.Errorf("%s.command = %v, want %q", path, got, command)
		}
		if got := decodeJSON(t, block, path+".args"); !reflect.DeepEqual(got, args) {
			t.Errorf("%s.args = %v, want %v", path, got, args)
		}
	}
	cases := []struct {
		host  string
		check func(block string)
	}{{
		"opencode", func(block string) {
			if got := decodeJSON(t, block, "$schema"); got != "https://opencode.ai/config.json" {
				t.Errorf("$schema = %v", got)
			}
			if got := decodeJSON(t, block, "mcp.sysml.type"); got != "local" {
				t.Errorf("mcp.sysml.type = %v, want local", got)
			}
			if got := decodeJSON(t, block, "mcp.sysml.command"); !reflect.DeepEqual(got, argv) {
				t.Errorf("mcp.sysml.command = %v, want %v", got, argv)
			}
			if got := decodeJSON(t, block, "mcp.sysml.enabled"); got != true {
				t.Errorf("mcp.sysml.enabled = %v, want true", got)
			}
		},
	}, {
		"claude-code", func(block string) {
			if got := decodeJSON(t, block, "mcpServers.sysml.type"); got != "stdio" {
				t.Errorf("mcpServers.sysml.type = %v, want stdio", got)
			}
			server(block, "mcpServers.sysml")
		},
	}, {
		"cursor", func(block string) { server(block, "mcpServers.sysml") },
	}, {
		"codex", func(block string) { checkCodexTable(t, block, command, args) },
	}, {
		"vscode", func(block string) {
			if got := decodeJSON(t, block, "servers.sysml.type"); got != "stdio" {
				t.Errorf("servers.sysml.type = %v, want stdio", got)
			}
			server(block, "servers.sysml")
		},
	}, {
		"gemini", func(block string) { server(block, "mcpServers.sysml") },
	}, {
		"devin", func(block string) { server(block, "mcpServers.sysml") },
	}}
	if len(cases) != len(hosts) {
		t.Fatalf("%d hosts are checked, %d are known", len(cases), len(hosts))
	}
	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			block := printed(t, append([]string{"-print-config", tc.host}, flags...)...)
			if !strings.HasSuffix(block, "\n") || strings.HasSuffix(block, "\n\n") {
				t.Errorf("the block should end in exactly one newline:\n%q", block)
			}
			tc.check(block)
			// The host may be given as the flag's value as well.
			if again := printed(t, append([]string{"-print-config=" + tc.host}, flags...)...); again != block {
				t.Errorf("-print-config=%s printed a different block:\n%s", tc.host, again)
			}
		})
	}
}

// checkCodexTable reads the TOML table the way Codex's parser does for this
// shape: one table header, then key = value lines of basic strings and arrays
// of them, every key exactly once.
func checkCodexTable(t *testing.T, block string, command string, args []any) {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(block, "\n"), "\n")
	if lines[0] != "[mcp_servers.sysml]" {
		t.Fatalf("the table is %q, want [mcp_servers.sysml]", lines[0])
	}
	values := map[string]any{}
	for _, line := range lines[1:] {
		key, raw, ok := strings.Cut(line, " = ")
		if !ok || strings.ContainsAny(key, " \t=\"") {
			t.Fatalf("not a key = value line: %q", line)
		}
		if _, dup := values[key]; dup {
			t.Fatalf("key %q is defined twice", key)
		}
		switch {
		case strings.HasPrefix(raw, "["):
			// A TOML array of basic strings; a JSON array of strings is one.
			var items []any
			if err := json.Unmarshal([]byte(raw), &items); err != nil {
				t.Fatalf("%s: not an array of basic strings: %v", key, err)
			}
			values[key] = items
		case strings.HasPrefix(raw, `"`):
			// TOML basic strings use the escapes Go's quoted strings do.
			s, err := strconv.Unquote(raw)
			if err != nil {
				t.Fatalf("%s: not a basic string: %v", key, err)
			}
			values[key] = s
		default:
			t.Fatalf("%s: value %q is neither a basic string nor an array", key, raw)
		}
	}
	want := map[string]any{"command": command, "args": args}
	if !reflect.DeepEqual(values, want) {
		t.Errorf("table = %v, want %v", values, want)
	}
}

// TestPrintConfigNamesThisBinary checks the default command is the running
// binary's absolute path, as the hosts need to start it from any directory.
func TestPrintConfigNamesThisBinary(t *testing.T) {
	exe, err := executablePath()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(exe) {
		t.Fatalf("executablePath() = %q, not absolute", exe)
	}
	block := printed(t, "-print-config", "cursor")
	if got := decodeJSON(t, block, "mcpServers.sysml.command"); got != exe {
		t.Errorf("command = %v, want %q", got, exe)
	}
}

// TestPrintConfigCommandLine checks the flag's spellings: alone it lists the
// hosts, and what it cannot act on is reported with the usage.
func TestPrintConfigCommandLine(t *testing.T) {
	list := printed(t, "-print-config")
	for _, h := range hosts {
		if !strings.Contains(list, h.name+" ") || !strings.Contains(list, h.file) {
			t.Errorf("the list is missing %s (%s):\n%s", h.name, h.file, list)
		}
	}
	for _, tc := range []struct {
		args   []string
		stderr string
	}{
		{[]string{"-print-config", "nope"}, `-print-config: unknown host "nope"; the hosts are opencode, claude-code`},
		{[]string{"-print-config", "cursor", "extra"}, `unexpected argument "extra" after -print-config cursor`},
		{[]string{"-print-config=cursor", "extra"}, `unexpected argument "extra" after -print-config cursor`},
		{[]string{"-print-config", "cursor", "-nosuchflag"}, "flag provided but not defined: -nosuchflag"},
		{[]string{"-print-config", "cursor", "-print-config-npx", "-print-config-command", "sysml-mcp"}, `-print-config-npx names npx -y @openmbee/sysml-mcp as the command, so it excludes -print-config-command "sysml-mcp"`},
	} {
		var stdout, stderr bytes.Buffer
		if status := run(tc.args, &stdout, &stderr); status != exitUnservable {
			t.Errorf("%v: exit status %d, want %d", tc.args, status, exitUnservable)
		}
		if !strings.Contains(stderr.String(), tc.stderr) {
			t.Errorf("%v: stderr is missing %q:\n%s", tc.args, tc.stderr, stderr.String())
		}
		if stdout.Len() > 0 {
			t.Errorf("%v wrote to stdout:\n%s", tc.args, stdout.String())
		}
	}
}
