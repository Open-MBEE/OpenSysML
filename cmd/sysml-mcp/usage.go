package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/usage"
)

const mcpCommand = "sysml-mcp"

// options is what the command line resolves to.
type options struct {
	root          string
	tools         string
	strict        bool
	noRecordCache bool
	cacheSize     int
	logLevel      string
	http          string
	allowOrigin   string
	token         string
	tokenFile     string
	publicURL     string
	stdio         bool
	tlsCert       string
	tlsKey        string
	showVersion   bool
	showHelp      bool
	showMan       bool
	printConfig   hostFlag
	configCommand string
	configNpx     bool
}

// registerFlags declares the server's flags on fs, so a run, the help and the
// man page all read one declaration of each. -h/-help are flags rather than the
// flag package's own so that help asked for is a result on stdout, and -stdio
// names the default transport: agent hosts pass it, so it is accepted and
// documented rather than rejected.
func registerFlags(fs *flag.FlagSet) *options {
	var o options
	fs.StringVar(&o.root, "root", ".",
		"Workspace folder whose model documents the server indexes and edits; default the current directory, which an agent host sets to the project")
	fs.StringVar(&o.tools, "tools", "all",
		"Tool groups to serve, comma-separated: lsp (diagnostics, navigation, editing, rendering), runtime (one-shot evaluation, execution, verification, conversion), session (persistent REPL sessions); default all")
	fs.BoolVar(&o.strict, "strict", false,
		"Serve strict conformance from the start: notation no pinned SysML v2 production admits is an error")
	fs.BoolVar(&o.noRecordCache, "no-record-cache", false,
		"Parse every document and hold it loaded, reading no interface record from the record cache and writing none; default off, or OPENSYSML_RECORD_CACHE=0")
	fs.IntVar(&o.cacheSize, "cache-size", 100, "Maximum number of parsed models the runtime tools hold; at least 1")
	fs.StringVar(&o.logLevel, "log-level", "warn", "Lowest level logged to stderr: debug, info, warn or error")
	fs.BoolVar(&o.stdio, "stdio", false, "Serve over stdin/stdout (the default transport; accepted for hosts that name it; not with -http)")
	fs.StringVar(&o.http, "http", "",
		"Serve over Streamable HTTP at the path /mcp of this address instead of stdin/stdout, as host:port or a port alone, which binds loopback; "+
			"every client shares the one workspace, runtime cache and set of REPL sessions. An address other machines reach is served only with a bearer token (-token-file, -token or "+mcp.TokenEnv+")")
	fs.StringVar(&o.allowOrigin, "allow-origin", "",
		"Origins, comma-separated as scheme://host[:port], granted cross-origin access to the HTTP transport (the CORS headers and preflight answer a browser needs); "+
			"loopback origins and requests with no Origin header are admitted without a grant, any other origin is refused")
	fs.StringVar(&o.tokenFile, "token-file", "",
		"File holding the bearer token every HTTP request must carry as Authorization: Bearer TOKEN (a trailing line break is not part of it); "+
			"required for an address other machines reach, enforced on loopback too when given")
	fs.StringVar(&o.token, "token", "",
		"The bearer token itself, for a shell whose command line is private; -token-file keeps it out of the process list, and "+mcp.TokenEnv+" is read when neither flag names one")
	fs.StringVar(&o.publicURL, "public-url", "",
		"URL clients reach the HTTP transport at when a proxy in front terminates TLS or rewrites the path, as https://models.example.org/mcp; "+
			"named as the protected resource and in the bearer challenge, and by the -print-config block; default each request's own scheme and Host")
	fs.StringVar(&o.tlsCert, "tls-cert", "", "TLS certificate file for the HTTP transport; with -tls-key, the transport serves https")
	fs.StringVar(&o.tlsKey, "tls-key", "", "TLS private key file for the HTTP transport")
	fs.Var(&o.printConfig, "print-config",
		"Write the configuration block declaring this server to an agent host — "+strings.Join(hostNames(), ", ")+" — to stdout and exit; alone, list the hosts")
	fs.StringVar(&o.configCommand, "print-config-command", "",
		"Command the -print-config block names instead of this binary's absolute path, such as sysml-mcp when it is on PATH")
	fs.BoolVar(&o.configNpx, "print-config-npx", false,
		"Make the -print-config block start the server as npx -y "+npmPackage+", which npm installs on first use, instead of naming this binary")
	fs.BoolVar(&o.showVersion, "version", false, "Show version information")
	fs.BoolVar(&o.showVersion, "v", false, "Show version (shorthand)")
	fs.BoolVar(&o.showHelp, "help", false, "Show this help and exit")
	fs.BoolVar(&o.showHelp, "h", false, "Show this help (shorthand)")
	fs.BoolVar(&o.showMan, "man", false, "Write this command's manual page, in roff, to stdout and exit")
	return &o
}

// docFlags is a flag set holding the server's flags, for rendering the help or
// the man page without a command line to parse.
func docFlags() *flag.FlagSet {
	fs := flag.NewFlagSet(mcpCommand, flag.ContinueOnError)
	registerFlags(fs)
	return fs
}

// hostList names the hosts -print-config knows, each with the file it reads.
func hostList() string {
	parts := make([]string, len(hosts))
	for i, h := range hosts {
		parts[i] = h.name + " (" + h.file + ")"
	}
	return strings.Join(parts, ", ")
}

// doc describes the server for both the terminal help and the man page, so a
// setting documented for one is documented for the other.
func doc() usage.Doc {
	return usage.Doc{
		Command:    mcpCommand,
		ManSection: 1,
		Summary:    "Model Context Protocol server for SysML v2 and KerML",
		Synopsis:   []string{"[options]", "-http ADDR [options]"},
		Description: []string{
			"sysml-mcp serves OpenSysML to coding agents over the Model Context " +
				"Protocol, as tools an agent calls: the language server's " +
				"diagnostics, navigation, editing and rendering over the model " +
				"documents under the workspace root; the runtime's one-shot " +
				"parsing, evaluation, instantiation, execution, verification, " +
				"querying and conversion; and named REPL sessions whose " +
				"declarations, instances and runs persist across calls; " +
				protocolMessage + ".",
			"The documents under the root are managed as one workspace, so a " +
				"declaration in one resolves against the others, and the standard " +
				"library is the copy embedded in this binary unless " +
				"OPENSYSML_LIBRARY_PATH names another.",
		},
		Sections: []usage.Section{{
			Title: "Starting the server",
			Examples: []usage.Example{
				usage.Ex(mcpCommand, "What an agent host runs, in the project directory"),
				usage.Ex("sysml-mcp -root models", "Serve the models under another folder"),
				usage.Ex("sysml-mcp -tools lsp,runtime", "Serve without persistent sessions"),
			},
			Paragraphs: []string{
				"An agent host declares the server in its configuration, as " +
					"{\"mcp\": {\"sysml\": {\"type\": \"local\", \"command\": [\"sysml-mcp\"]}}} " +
					"in OpenCode's opencode.json, and starts it with the project as the " +
					"working directory.",
			},
		}, {
			Title: "Transports",
			Lead: []string{
				"Without -http the protocol is spoken over stdin and stdout: the host " +
					"starts the server, which serves that one client and exits when " +
					"the stream closes. With -http the server is started once and " +
					"serves every client that connects to its address over the " +
					"Streamable HTTP transport, at the path /mcp, until it is told to " +
					"stop with SIGINT or SIGTERM; stopping closes the sessions and " +
					"waits up to 30 seconds for the connections to drain. The clients " +
					"share one workspace, one runtime cache and one set of REPL " +
					"sessions, so a session_* name opened by one is the session every " +
					"other sees under that name.",
			},
			Examples: []usage.Example{
				usage.Ex("sysml-mcp -http 8765", "Serve http://localhost:8765/mcp to this machine"),
				usage.Ex("sysml-mcp -http 0.0.0.0:8765 -token-file mcp.token -tls-cert server.crt -tls-key server.key", "Serve every interface over https to clients that hold the token"),
				usage.Ex("sysml-mcp -http 0.0.0.0:8765 -token-file mcp.token -allow-origin https://agents.example.org", "Answer one browser origin besides loopback"),
				usage.Ex("sysml-mcp -http 8765 -token-file mcp.token -public-url https://models.example.org/mcp", "Serve loopback behind a proxy that terminates TLS at that URL"),
			},
			Paragraphs: []string{
				"An address with no host, or a port alone, binds loopback, which only " +
					"this machine reaches, and is served as it is. Any other address is " +
					"served only with a bearer token, named by -token-file, -token or " +
					mcp.TokenEnv + ": the server is an OAuth 2.1 resource server in the " +
					"minimal form, with one token issued by whoever runs it, so every " +
					"request must carry Authorization: Bearer TOKEN, and one that does " +
					"not is refused with 401 and a WWW-Authenticate challenge naming the " +
					"resource metadata at /.well-known/oauth-protected-resource/mcp, " +
					"both naming the endpoint as the request reached it, or as " +
					"-public-url says when a proxy in front terminates TLS. " +
					"Starting on such an address with no token is refused. A token given " +
					"for a loopback address is enforced the same way. The token is " +
					"letters, digits and -._~+/ (RFC 6750), is never logged, and is best " +
					"generated, as by openssl rand -base64 32, and kept in a file only " +
					"the server's user reads. -tls-cert and -tls-key, both given, serve " +
					"https, which a token sent over a network needs; one without the " +
					"other is refused.",
				"A request is answered when its Origin header is absent, names a " +
					"loopback origin (localhost, 127.0.0.0/8 or ::1 over any scheme " +
					"and port) or names an origin listed in -allow-origin exactly; any " +
					"other origin is refused with 403, which keeps a page on another " +
					"site from reaching the server through a DNS name rebound to this " +
					"machine. The wildcard origin is not accepted. A request whose " +
					"MCP-Protocol-Version header names a version the server does not " +
					"speak is refused with 400.",
			},
		}, {
			Title: "Configuring a host",
			Lead: []string{
				"-print-config writes the block a host reads, naming this binary by " +
					"its absolute path so the host finds it wherever it was installed; " +
					"-print-config-command names another command instead, such as " +
					"sysml-mcp when it is on PATH, and -print-config-npx names " +
					"npx -y " + npmPackage + ", which installs the release binary for the " +
					"host's platform from npm on first use, so a machine with Node needs " +
					"nothing else. With -http (and -tls-cert for https) the block names " +
					"the URL of a server already serving that address instead, and with " +
					"its token (-token-file, -token or " + mcp.TokenEnv + ") the " +
					"Authorization header the host sends, written in clear, so such a " +
					"block belongs in a file the host reads privately, not in version " +
					"control; every host reads the URL form. The hosts, with the file " +
					"each reads, are " + hostList() + ".",
			},
			Examples: []usage.Example{
				usage.Ex("sysml-mcp -print-config", "List the hosts"),
				usage.Ex("sysml-mcp -print-config opencode", "The mcp block of opencode.json"),
				usage.Ex("sysml-mcp -print-config codex >> ~/.codex/config.toml", "Append the [mcp_servers.sysml] table"),
				usage.Ex("sysml-mcp -print-config cursor -print-config-command sysml-mcp", "A block that finds the server on PATH"),
				usage.Ex("sysml-mcp -print-config vscode -print-config-npx", "A block that runs the server from npm"),
				usage.Ex("sysml-mcp -print-config vscode -http 8765", "A block reaching http://localhost:8765/mcp"),
				usage.Ex("sysml-mcp -print-config cursor -http models.example.org:8443 -tls-cert server.crt -tls-key server.key -token-file mcp.token", "A block reaching a remote server with its token"),
			},
		}, toolsSection(), {
			Title: "Exit status",
			Lead:  []string{"The status reports how the session ended:"},
			Items: []usage.Item{
				usage.Entry("0", "The protocol was served to its end: the client closed the stream, or the process was told to stop."),
				usage.Entry("1", "The server could not start, as when the -http address is in use, or the session ended in a protocol error."),
				usage.Entry("2", "The command line could not be acted on."),
			},
		}, {
			Title:      "Environment",
			ManOnly:    true,
			Items:      usage.BudgetEnvironment(),
			Paragraphs: []string{usage.LegacyPrefixNote, usage.BudgetScopeNote},
		}, {
			Title:   "Reporting bugs",
			ManOnly: true,
			Paragraphs: []string{
				"Report bugs at https://github.com/Open-MBEE/OpenSysML/issues.",
			},
		}},
		SeeAlso: []string{"sysml(1)", "sysml-lsp(1)", "sysml-grpc(1)", "https://github.com/Open-MBEE/OpenSysML"},
	}
}

// toolsSection lists every tool the server can serve, from the catalog that
// also drives registration and the tool_catalog tool, so the manual page
// cannot drift from the tools (`make man` regenerates it, `make man-check`
// fails when it was not).
func toolsSection() usage.Section {
	sec := usage.Section{
		Title: "Tools",
		Lead: []string{
			"The tools, by the group that serves them (-tools), each with the " +
				"sysml-grpc RPC or sysml flag it stands for. The tool_catalog tool " +
				"answers this list to the client, narrowed to the groups served.",
		},
	}
	for _, e := range catalog.Entries(catalog.Groups()...) {
		group := string(e.Group)
		if group == "" {
			group = "every group"
		}
		text := fmt.Sprintf("%s: %s", group, e.Summary)
		if op := e.Operation(); op != "none" {
			text = fmt.Sprintf("%s: %s (%s).", group, strings.TrimSuffix(e.Summary, "."), op)
		}
		sec.Items = append(sec.Items, usage.Entry(e.Name, text))
	}
	return sec
}
