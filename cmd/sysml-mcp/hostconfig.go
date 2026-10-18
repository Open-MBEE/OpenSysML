package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// serverName is the name the configuration blocks give the server; hosts
// prefix tool names with it, so an agent sees sysml/diagnostics and the like.
const serverName = "sysml"

// host is an agent host sysml-mcp writes a configuration block for.
type host struct {
	name string
	// file is where the host reads the block, relative to the project unless noted.
	file string
	// render writes the block declaring command as the server.
	render func(w io.Writer, command string) error
}

// hosts lists the hosts -print-config knows, in the order the list is shown.
var hosts = []host{
	{"opencode", "opencode.json", renderOpenCode},
	{"claude-code", ".mcp.json", renderMCPServers("stdio")},
	{"cursor", ".cursor/mcp.json", renderMCPServers("")},
	{"codex", "~/.codex/config.toml", renderCodex},
	{"vscode", ".vscode/mcp.json", renderVSCode},
	{"gemini", ".gemini/settings.json", renderMCPServers("")},
	{"devin", "Settings > MCP Marketplace > Add custom", renderMCPServers("")},
}

// hostNames is the names of hosts, for the help and for error messages.
func hostNames() []string {
	names := make([]string, len(hosts))
	for i, h := range hosts {
		names[i] = h.name
	}
	return names
}

// hostFlag is the -print-config flag: given alone it asks for the list of
// hosts, given a host (as -print-config=HOST, or as the argument that follows)
// it asks for that host's block.
type hostFlag struct {
	set  bool
	host string
}

func (f *hostFlag) String() string { return f.host }

// IsBoolFlag lets the flag stand alone, so the host can follow as an argument.
func (f *hostFlag) IsBoolFlag() bool { return true }

func (f *hostFlag) Set(s string) error {
	f.set = true
	// The flag package sets "true" for a bare boolean flag.
	if s != "true" {
		f.host = s
	}
	return nil
}

// printHostList writes the hosts and the files they read a block from.
func printHostList(w io.Writer) {
	fmt.Fprintln(w, "Hosts -print-config writes a configuration block for:")
	for _, h := range hosts {
		fmt.Fprintf(w, "  %-12s %s\n", h.name, h.file)
	}
	fmt.Fprintf(w, "Usage: %s -print-config HOST [-print-config-command CMD]\n", mcpCommand)
}

// printHostConfig writes the block declaring the server to host name, naming
// command as the executable, or this binary's resolved path when it is empty.
func printHostConfig(w io.Writer, name, command string) error {
	var h *host
	for i := range hosts {
		if hosts[i].name == name {
			h = &hosts[i]
		}
	}
	if h == nil {
		return fmt.Errorf("unknown host %q; the hosts are %s", name, strings.Join(hostNames(), ", "))
	}
	if command == "" {
		var err error
		if command, err = executablePath(); err != nil {
			return fmt.Errorf("resolving the path of this binary: %w (name the command with -print-config-command)", err)
		}
	}
	return h.render(w, command)
}

// executablePath is the absolute path of the running binary, symlinks resolved,
// so the block works from any working directory and after the binary is moved
// out of PATH.
func executablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Abs(exe)
}

// writeJSON writes v indented by two spaces, with a trailing newline, as the
// hosts' own editors format their configuration files.
func writeJSON(w io.Writer, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// orderedObject is a JSON object whose keys are written in the order given,
// so a block reads the way the host's documentation shows it.
type orderedObject []orderedField

type orderedField struct {
	key   string
	value any
}

func (o orderedObject) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, f := range o {
		if i > 0 {
			buf.WriteByte(',')
		}
		k, err := json.Marshal(f.key)
		if err != nil {
			return nil, err
		}
		v, err := json.Marshal(f.value)
		if err != nil {
			return nil, err
		}
		buf.Write(k)
		buf.WriteByte(':')
		buf.Write(v)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// renderOpenCode writes the "mcp" block of opencode.json: a local server named
// by its command array, enabled.
func renderOpenCode(w io.Writer, command string) error {
	return writeJSON(w, orderedObject{
		{"$schema", "https://opencode.ai/config.json"},
		{"mcp", orderedObject{
			{serverName, orderedObject{
				{"type", "local"},
				{"command", []string{command}},
				{"enabled", true},
			}},
		}},
	})
}

// renderMCPServers writes the "mcpServers" block Claude Code, Cursor, Gemini
// CLI and Devin read, with a transport type when the host wants one spelled.
func renderMCPServers(transport string) func(io.Writer, string) error {
	return func(w io.Writer, command string) error {
		return writeJSON(w, orderedObject{
			{"mcpServers", orderedObject{
				{serverName, serverObject(transport, command)},
			}},
		})
	}
}

// renderVSCode writes the "servers" block of .vscode/mcp.json.
func renderVSCode(w io.Writer, command string) error {
	return writeJSON(w, orderedObject{
		{"servers", orderedObject{
			{serverName, serverObject("stdio", command)},
		}},
	})
}

// serverObject is the common stdio server declaration: a command and its
// (empty) argument list, preceded by the transport type when one is wanted.
func serverObject(transport, command string) orderedObject {
	var o orderedObject
	if transport != "" {
		o = append(o, orderedField{"type", transport})
	}
	return append(o,
		orderedField{"command", command},
		orderedField{"args", []string{}},
	)
}

// renderCodex writes the [mcp_servers.sysml] table of Codex's config.toml. A
// JSON string is a valid TOML basic string, so the command is quoted as one.
func renderCodex(w io.Writer, command string) error {
	quoted, err := json.Marshal(command)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "[mcp_servers.%s]\ncommand = %s\nargs = []\n", serverName, quoted)
	return err
}
