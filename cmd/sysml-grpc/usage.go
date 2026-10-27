package main

import (
	"flag"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/usage"
)

const grpcCommand = "sysml-grpc"

// options is what the command line resolves to.
type options struct {
	host                 string
	port                 int
	healthPort           int
	tokenFile            string
	token                string
	allowUnauthenticated bool
	allowInsecureToken   bool
	cacheSize            int
	logLevel             string
	showVersion          bool
	showMan              bool
	reportAddress        bool
	exitWithParent       bool
	transport            string
	corsOrigins          string
	tlsCert              string
	tlsKey               string
	serveExternal        string
}

// registerFlags declares the command's flags on fs, so a run, the help and the
// man page all read one declaration of each.
func registerFlags(fs *flag.FlagSet) *options {
	var o options
	fs.StringVar(&o.host, "host", "",
		"Host to bind, for the health listener too; empty binds loopback, so only this "+
			"machine reaches the service. An address other machines reach (0.0.0.0, a "+
			"host name) is served only with a bearer token (-token-file, -token or "+tokenEnv+") "+
			"or -allow-unauthenticated")
	fs.IntVar(&o.port, "port", 50051, "gRPC server port")
	fs.IntVar(&o.healthPort, "health-port", 8081, "Health check HTTP port")
	fs.StringVar(&o.tokenFile, "token-file", "",
		"File holding the bearer token every call must carry as Authorization: Bearer TOKEN "+
			"(one trailing line break is dropped); with it the token stays out of the process list")
	fs.StringVar(&o.token, "token", "",
		"The bearer token itself, for a shell whose command line is private; -token-file keeps "+
			"it out of the process list, and "+tokenEnv+" is read when neither flag names one")
	fs.BoolVar(&o.allowUnauthenticated, "allow-unauthenticated", false,
		"Serve an address other machines reach with no token at all: every host that can "+
			"open the port may parse, run and edit models here, and the log shows peers but "+
			"never who they are. Only for a trusted network behind a proxy that authenticates "+
			"for this service; never implied")
	fs.BoolVar(&o.allowInsecureToken, "allow-insecure-token-transport", false,
		"Serve a bearer token on an address other machines reach without -tls-cert and "+
			"-tls-key: the token crosses the network in clear, so anyone observing it can "+
			"replay it. Only behind a TLS-terminating proxy on a trusted network; never implied")
	fs.IntVar(&o.cacheSize, "cache-size", 100, "Maximum number of cached parsed files")
	fs.StringVar(&o.logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	fs.BoolVar(&o.showVersion, "version", false, "Show version and exit")
	fs.BoolVar(&o.showMan, "man", false, "Write this command's manual page, in roff, to stdout and exit")
	fs.BoolVar(&o.reportAddress, "report-address", false,
		"Print the address to dial on stdout, as one line, once the listener is "+
			"bound; with -port 0 this is how a client learns the port the kernel chose")
	fs.BoolVar(&o.exitWithParent, "exit-with-parent", false,
		"Exit at end of file on stdin, so a child cannot outlive the process that "+
			"holds the write end of a pipe on it, SIGKILL included")
	fs.StringVar(&o.transport, "transport", transportConnect,
		"Transport to serve: connect (default; gRPC, gRPC-Web and Connect on one port), "+
			"grpc (grpc-go only), or stdio (an evaluation prototype over stdin/stdout)")
	fs.StringVar(&o.corsOrigins, "cors-allowed-origins", "", "Comma-separated exact origins allowed for browser CORS")
	fs.StringVar(&o.tlsCert, "tls-cert", "", "TLS certificate file for the main server")
	fs.StringVar(&o.tlsKey, "tls-key", "", "TLS private key file for the main server")
	fs.StringVar(&o.serveExternal, "serve-external-engines", "",
		"Comma-separated names of the OPENSYSML_ENGINES manifest engines this service runs "+
			"for its clients, or all; without it they are listed by ListEngines but a request "+
			"naming one is refused, since it would run a program on the server")
	return &o
}

// docFlags is a flag set holding the command's flags, for rendering the help or
// the man page without a command line to parse.
func docFlags() *flag.FlagSet {
	fs := flag.NewFlagSet(grpcCommand, flag.ContinueOnError)
	registerFlags(fs)
	return fs
}

// doc describes the service for both the terminal help and the man page, so a
// transport documented for one is documented for the other.
func doc() usage.Doc {
	return usage.Doc{
		Command:    grpcCommand,
		ManSection: 1,
		Summary:    "serve SysML v2 parsing, resolution and semantic services over gRPC",
		Synopsis:   []string{"[options]"},
		Description: []string{
			"sysml-grpc exposes the parser, name resolution and semantic services " +
				"as a network service, so a client in any language reaches the " +
				"same engine the command line and the language server use. It " +
				"serves gRPC, gRPC-Web and Connect on one port by default, and " +
				"caches the files it has parsed.",
			"The standard library is the copy embedded in this binary unless " +
				"OPENSYSML_LIBRARY_PATH names another, and it is indexed in the " +
				"background at startup so the first model to arrive does not wait " +
				"for it.",
		},
		Sections: []usage.Section{{
			Title: "Serving",
			Examples: []usage.Example{
				usage.Ex(grpcCommand, "Serve gRPC, gRPC-Web and Connect on localhost:50051"),
				usage.Ex("sysml-grpc -port 0 -report-address", "Let the kernel choose the port and report it"),
				usage.Ex("sysml-grpc -transport grpc -port 50051", "Serve grpc-go only"),
				usage.Ex("sysml-grpc -transport stdio", "Speak the protocol over a pipe (a prototype)"),
				usage.Ex("sysml-grpc -host 0.0.0.0 -token-file grpc.token -tls-cert server.crt -tls-key server.key", "Serve other machines over TLS, each call carrying the token"),
				usage.Ex("sysml-grpc -host 0.0.0.0 -token-file grpc.token -allow-insecure-token-transport", "The same in clear, behind a proxy that terminates TLS"),
				usage.Ex("sysml-grpc -host 0.0.0.0 -allow-unauthenticated", "Serve other machines open, behind a proxy that authenticates"),
			},
			Paragraphs: []string{
				"Under -transport connect the health endpoint is served as /health on " +
					"the main port, and -health-port 0 disables the separate " +
					"listener, which is deprecated.",
				"The service binds loopback unless -host names another address, and " +
					"an address other machines reach is served only with a bearer " +
					"token (-token-file, -token or " + tokenEnv + "), which every call " +
					"carries as Authorization: Bearer TOKEN on gRPC, gRPC-Web and " +
					"Connect alike and is compared in constant time; a call without " +
					"it is refused with the unauthenticated status and a " +
					"WWW-Authenticate challenge. Such an address carries the token only " +
					"over TLS (-tls-cert and -tls-key) unless -allow-insecure-token-transport " +
					"accepts that it travels in clear, behind a proxy that terminates TLS; " +
					"-allow-unauthenticated serves such an address with no token at " +
					"all, for a trusted network whose proxy authenticates. Neither " +
					"opt-out is ever implied. /health answers without a token, so probes " +
					"need none, and -transport stdio binds nothing, so none of this " +
					"applies to it.",
				"Every call is logged at the info level with the peer address, " +
					"method, protocol, status code, duration and whether it was " +
					"authenticated; a refused call is logged at the warning level " +
					"with the peer. The token itself is never logged.",
				"A browser client needs its origin allowed: pass the exact origins " +
					"to -cors-allowed-origins, separated by commas.",
			},
		}, {
			Title: "Ending the service",
			Paragraphs: []string{
				"The service shuts down gracefully on SIGINT or SIGTERM. With " +
					"-exit-with-parent it also exits at end of file on stdin, so a " +
					"service started as a child cannot outlive the process holding " +
					"the write end of that pipe, however that process dies.",
			},
		}, {
			Title: "Exit status",
			Lead:  []string{"The status reports how the service ended:"},
			Items: []usage.Item{
				usage.Entry("0", "It shut down when asked, or reported what was asked of it."),
				usage.Entry("1", "It could not serve: a port already bound, an unreadable certificate, a service that failed."),
				usage.Entry("2", "The command line could not be acted on."),
			},
		}, {
			Title:      "Environment",
			ManOnly:    true,
			Items:      append(append(usage.BudgetEnvironment(), usage.JobsEnvironment()...), usage.ToolEnvironment()...),
			Paragraphs: []string{usage.LegacyPrefixNote, usage.BudgetScopeNote},
		}, {
			Title:   "Reporting bugs",
			ManOnly: true,
			Paragraphs: []string{
				"Report bugs at https://github.com/Open-MBEE/OpenSysML/issues.",
			},
		}},
		SeeAlso: []string{"sysml(1)", "sysml-lsp(1)", "https://github.com/Open-MBEE/OpenSysML"},
	}
}
