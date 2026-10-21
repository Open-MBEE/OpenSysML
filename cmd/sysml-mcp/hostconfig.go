package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp"
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
	// render writes the block declaring the server at ep.
	render func(w io.Writer, ep endpoint) error
}

// endpoint is how a host reaches the server: a command it starts and speaks
// stdio with, or the URL of one already serving over Streamable HTTP.
type endpoint struct {
	command serverCommand
	url     string
}

// hosts lists the hosts -print-config knows, in the order the list is shown.
var hosts = []host{
	{"opencode", "opencode.json", renderOpenCode},
	{"claude-code", ".mcp.json", renderMCPServers("stdio", "http", "url")},
	{"cursor", ".cursor/mcp.json", renderMCPServers("", "", "url")},
	{"codex", "~/.codex/config.toml", renderCodex},
	{"vscode", ".vscode/mcp.json", renderVSCode},
	{"gemini", ".gemini/settings.json", renderMCPServers("", "", "httpUrl")},
	{"devin", "Customize > MCPs > Add custom MCP", renderMCPServers("", "http", "url")},
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
	fmt.Fprintf(w, "Usage: %s -print-config HOST [-print-config-command CMD | -print-config-npx | -http ADDR [-tls-cert FILE -tls-key FILE]]\n", mcpCommand)
}

// printHostConfig writes the block declaring the server to host name: at
// ep.url, or started by ep.command, the npm form, or this binary's path.
func printHostConfig(w io.Writer, name string, ep endpoint, npx bool) error {
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
	case ep.url != "":
	case npx && ep.command.exe != "":
		return fmt.Errorf("-print-config-npx names npx -y %s as the command, so it excludes -print-config-command %q", npmPackage, ep.command.exe)
	case npx:
		ep.command = npxCommand
	case ep.command.exe == "":
		exe, err := executablePath()
		if err != nil {
			return fmt.Errorf("resolving the path of this binary: %w (name the command with -print-config-command)", err)
		}
		ep.command = binaryCommand(exe)
	}
	return h.render(w, ep)
}

// serverURL is where clients reach a server bound to addr: https with TLS, and
// localhost where addr binds every interface or loopback alone.
func serverURL(addr string, tls bool) string {
	host, port, _ := net.SplitHostPort(addr)
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		host = "localhost"
	}
	scheme := "http"
	if tls {
		scheme = "https"
	}
	return scheme + "://" + net.JoinHostPort(host, port) + mcp.HTTPPath
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
// by its command array, or a remote one by its URL, enabled.
func renderOpenCode(w io.Writer, ep endpoint) error {
	server := orderedObject{{"type", "local"}, {"command", ep.command.argv()}}
	if ep.url != "" {
		server = orderedObject{{"type", "remote"}, {"url", ep.url}}
	}
	return writeJSON(w, orderedObject{
		{"$schema", "https://opencode.ai/config.json"},
		{"mcp", orderedObject{
			{serverName, append(server, orderedField{"enabled", true})},
		}},
	})
}

// renderMCPServers writes the "mcpServers" block Claude Code, Cursor, Gemini
// CLI and Devin read: a stdio server typed stdioType, or a URL under urlKey typed httpType.
func renderMCPServers(stdioType, httpType, urlKey string) func(io.Writer, endpoint) error {
	return func(w io.Writer, ep endpoint) error {
		return writeJSON(w, orderedObject{
			{"mcpServers", orderedObject{
				{serverName, serverObject(ep, stdioType, httpType, urlKey)},
			}},
		})
	}
}

// renderVSCode writes the "servers" block of .vscode/mcp.json.
func renderVSCode(w io.Writer, ep endpoint) error {
	return writeJSON(w, orderedObject{
		{"servers", orderedObject{
			{serverName, serverObject(ep, "stdio", "http", "url")},
		}},
	})
}

// serverObject is the common server declaration: a command and its arguments,
// or a URL under urlKey, each preceded by its type when the host wants one spelled.
func serverObject(ep endpoint, stdioType, httpType, urlKey string) orderedObject {
	var o orderedObject
	if ep.url != "" {
		if httpType != "" {
			o = append(o, orderedField{"type", httpType})
		}
		return append(o, orderedField{urlKey, ep.url})
	}
	if stdioType != "" {
		o = append(o, orderedField{"type", stdioType})
	}
	return append(o,
		orderedField{"command", ep.command.exe},
		orderedField{"args", ep.command.arguments()},
	)
}

// renderCodex writes the [mcp_servers.sysml] table of Codex's config.toml as
// TOML basic strings and arrays, which JSON strings and arrays of them are.
func renderCodex(w io.Writer, ep endpoint) error {
	if ep.url != "" {
		quoted, err := json.Marshal(ep.url)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "[mcp_servers.%s]\nurl = %s\n", serverName, quoted)
		return err
	}
	quoted, err := json.Marshal(ep.command.exe)
	if err != nil {
		return err
	}
	args, err := json.Marshal(ep.command.arguments())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "[mcp_servers.%s]\ncommand = %s\nargs = %s\n", serverName, quoted, args)
	return err
}
