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

// npmPackage is the npm package whose sysml-mcp command runs this server from
// the release binary for the host's platform, so `npx -y` installs it on
// first use with nothing preinstalled.
const npmPackage = "@openmbee/sysml-mcp"

// serverCommand is what a block tells the host to start: an executable and
// the arguments before any the host adds.
type serverCommand struct {
	exe  string
	args []string
}

// npxCommand is the serverCommand of the npm form: npx installs npmPackage
// without asking and runs its sysml-mcp command.
var npxCommand = serverCommand{exe: "npx", args: []string{"-y", npmPackage}}

// binaryCommand is the serverCommand that starts exe with no arguments.
func binaryCommand(exe string) serverCommand { return serverCommand{exe: exe} }

// argv is the command as one array, the way OpenCode spells it.
func (c serverCommand) argv() []string {
	return append([]string{c.exe}, c.args...)
}

// arguments is the argument list, never nil, so it is written as [] not null.
func (c serverCommand) arguments() []string {
	if c.args == nil {
		return []string{}
	}
	return c.args
}

// host is an agent host sysml-mcp writes a configuration block for.
type host struct {
	name string
	// file is where the host reads the block, relative to the project unless noted.
	file string
	// render writes the block declaring command as the server.
	render func(w io.Writer, command serverCommand) error
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
	fmt.Fprintf(w, "Usage: %s -print-config HOST [-print-config-command CMD | -print-config-npx]\n", mcpCommand)
}

// printHostConfig writes the block declaring the server to host name. The
// block names command as the executable; with npx instead, it names the npm
// form; with neither, this binary's resolved path.
func printHostConfig(w io.Writer, name, command string, npx bool) error {
	var h *host
	for i := range hosts {
		if hosts[i].name == name {
			h = &hosts[i]
		}
	}
	if h == nil {
		return fmt.Errorf("unknown host %q; the hosts are %s", name, strings.Join(hostNames(), ", "))
	}
	switch {
	case npx && command != "":
		return fmt.Errorf("-print-config-npx names npx -y %s as the command, so it excludes -print-config-command %q", npmPackage, command)
	case npx:
		return h.render(w, npxCommand)
	case command != "":
		return h.render(w, binaryCommand(command))
	}
	exe, err := executablePath()
	if err != nil {
		return fmt.Errorf("resolving the path of this binary: %w (name the command with -print-config-command)", err)
	}
	return h.render(w, binaryCommand(exe))
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
func renderOpenCode(w io.Writer, command serverCommand) error {
	return writeJSON(w, orderedObject{
		{"$schema", "https://opencode.ai/config.json"},
		{"mcp", orderedObject{
			{serverName, orderedObject{
				{"type", "local"},
				{"command", command.argv()},
				{"enabled", true},
			}},
		}},
	})
}

// renderMCPServers writes the "mcpServers" block Claude Code, Cursor, Gemini
// CLI and Devin read, with a transport type when the host wants one spelled.
func renderMCPServers(transport string) func(io.Writer, serverCommand) error {
	return func(w io.Writer, command serverCommand) error {
		return writeJSON(w, orderedObject{
			{"mcpServers", orderedObject{
				{serverName, serverObject(transport, command)},
			}},
		})
	}
}

// renderVSCode writes the "servers" block of .vscode/mcp.json.
func renderVSCode(w io.Writer, command serverCommand) error {
	return writeJSON(w, orderedObject{
		{"servers", orderedObject{
			{serverName, serverObject("stdio", command)},
		}},
	})
}

// serverObject is the common stdio server declaration: a command and its
// argument list, preceded by the transport type when one is wanted.
func serverObject(transport string, command serverCommand) orderedObject {
	var o orderedObject
	if transport != "" {
		o = append(o, orderedField{"type", transport})
	}
	return append(o,
		orderedField{"command", command.exe},
		orderedField{"args", command.arguments()},
	)
}

// renderCodex writes the [mcp_servers.sysml] table of Codex's config.toml. A
// JSON string is a valid TOML basic string and a JSON array of them a valid
// TOML array, so the command and its arguments are written as those.
func renderCodex(w io.Writer, command serverCommand) error {
	quoted, err := json.Marshal(command.exe)
	if err != nil {
		return err
	}
	args, err := json.Marshal(command.arguments())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "[mcp_servers.%s]\ncommand = %s\nargs = %s\n", serverName, quoted, args)
	return err
}
