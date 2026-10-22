// Command sysml-mcp serves OpenSysML to coding agents over the Model Context
// Protocol: the language server's diagnostics, navigation and editing, the
// runtime's evaluation and execution, and persistent REPL sessions, as tools an
// agent calls. The protocol is spoken over stdin/stdout, so an agent host starts
// this server rather than a shell, or over Streamable HTTP to every client of
// one long-lived server when -http names an address.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/buildinfo"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp"
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/all" // registers every REPL extension
	"github.com/Open-MBEE/OpenSysML/internal/frontend/usage"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// Build metadata, set by the linker: the names match the -X flags the Makefile
// and the release build pass, so a released binary reports what it was built from.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
	GoVersion = "unknown"
)

// build is what this binary reports about itself: the linker's stamps, or the
// module version and VCS metadata the toolchain recorded when none were passed.
func build() buildinfo.Info {
	return buildinfo.Resolve(buildinfo.Stamps{Version: Version, Commit: Commit, BuildTime: BuildTime, GoVersion: GoVersion})
}

// Exit statuses: 0 for the protocol served to its end, 1 for a session that
// ended in a protocol error or a server that could not start, 2 for a command
// line the server cannot act on, as for sysml-lsp.
const (
	exitServed      = 0
	exitProtocol    = 1
	exitUnservable  = 2
	commandPrefix   = mcpCommand + ": "
	protocolMessage = "the protocol is spoken over stdin/stdout, so an agent host starts this server rather than a shell, or over Streamable HTTP at the address -http names"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// printConfig answers -print-config: the list of hosts when none is named, or
// the block for the host named with it.
func printConfig(opts *options, fs *flag.FlagSet, stdout, stderr io.Writer) int {
	name := opts.printConfig.host
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "%sunexpected argument %q after -print-config %s\n", commandPrefix, fs.Arg(0), name)
		printUsage(stderr, fs)
		return exitUnservable
	}
	if name == "" {
		printHostList(stdout)
		return exitServed
	}
	ep := endpoint{}
	if opts.configCommand != "" {
		ep.command = binaryCommand(opts.configCommand)
	}
	over, err := readHTTPOptions(opts)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", commandPrefix, err)
		return exitUnservable
	}
	if over != nil {
		for name, set := range map[string]bool{"-print-config-command": opts.configCommand != "", "-print-config-npx": opts.configNpx} {
			if set {
				fmt.Fprintf(stderr, "%s-print-config: %s and -http name different ways to reach the server; give one\n", commandPrefix, name)
				return exitUnservable
			}
		}
		ep.url = over.url()
		ep.token = over.tokenValue()
	}
	if err := printHostConfig(stdout, name, ep, opts.configNpx); err != nil {
		fmt.Fprintf(stderr, "%s-print-config: %v\n", commandPrefix, err)
		return exitUnservable
	}
	return exitServed
}

// run carries out what the command line asked for: the version, the help, or
// the protocol itself. A flag it could not read is reported with the usage,
// rather than entering protocol mode and dying on the first frame.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(mcpCommand, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printUsage(fs.Output(), fs) }

	opts := registerFlags(fs)
	if err := fs.Parse(args); err != nil {
		// Parse has already reported the flag and printed the usage on stderr.
		return exitUnservable
	}
	if opts.printConfig.set && opts.printConfig.host == "" && fs.NArg() > 0 {
		// The host follows the bare flag; flags after it are read as well.
		opts.printConfig.host = fs.Arg(0)
		if err := fs.Parse(fs.Args()[1:]); err != nil {
			return exitUnservable
		}
	}

	if opts.showHelp {
		printUsage(stdout, fs)
		return exitServed
	}
	if opts.showMan {
		doc().WriteRoff(stdout, fs, usage.DefaultManMeta())
		return exitServed
	}
	if opts.showVersion {
		fmt.Fprint(stdout, build().Report(mcpCommand))
		return exitServed
	}
	if opts.printConfig.set {
		return printConfig(opts, fs, stdout, stderr)
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "%sunexpected argument %q; %s\n", commandPrefix, fs.Arg(0), protocolMessage)
		printUsage(stderr, fs)
		return exitUnservable
	}
	groups, err := mcp.ParseGroups(opts.tools)
	if err != nil {
		fmt.Fprintf(stderr, "%s-tools: %v\n", commandPrefix, err)
		printUsage(stderr, fs)
		return exitUnservable
	}
	root, err := filepath.Abs(opts.root)
	if err != nil {
		fmt.Fprintf(stderr, "%s-root %q: %v\n", commandPrefix, opts.root, err)
		printUsage(stderr, fs)
		return exitUnservable
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		fmt.Fprintf(stderr, "%s-root %q is not a directory\n", commandPrefix, opts.root)
		printUsage(stderr, fs)
		return exitUnservable
	}
	if opts.cacheSize < 1 {
		fmt.Fprintf(stderr, "%s-cache-size %d: the runtime tools need room for at least one model\n", commandPrefix, opts.cacheSize)
		printUsage(stderr, fs)
		return exitUnservable
	}
	level, err := parseLogLevel(opts.logLevel)
	if err != nil {
		fmt.Fprintf(stderr, "%s-log-level: %v\n", commandPrefix, err)
		printUsage(stderr, fs)
		return exitUnservable
	}
	httpOpts, err := readHTTPOptions(opts)
	if err == nil {
		err = httpOpts.loadTLS()
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", commandPrefix, err)
		printUsage(stderr, fs)
		return exitUnservable
	}

	logger := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	cache, err := libs.OpenRecordCache(opts.noRecordCache)
	if err != nil {
		fmt.Fprintf(stderr, "%srecord cache unavailable, holding every document loaded: %v\n", commandPrefix, err)
	}
	info := build()
	cfg := mcp.Config{
		Root:                    root,
		Groups:                  groups,
		Version:                 info.Version,
		Workspace:               model.NewWorkspace(model.WithConformanceMode(diag.ConformanceModeOf(opts.strict)), model.WithRecordCache(cache)),
		CacheSize:               opts.cacheSize,
		UnavailableCapabilities: sysmlgrpc.WithheldCapabilitiesForTesting(),
		NewSession:              sessionFactory(info, cache, stderr),
		Logger:                  logger,
		Instructions:            instructions,
	}
	return serve(stderr, cfg, httpOpts)
}

// printUsage writes the help to w, which the caller chooses: help asked for is
// a result, while help over a misuse belongs with the error.
func printUsage(w io.Writer, fs *flag.FlagSet) {
	doc().WriteText(w, fs)
}

// serve speaks the protocol over stdin/stdout until the client ends it or the
// process is told to stop, or with http over that address until the process is
// told to stop, and reports how the session ended.
func serve(stderr io.Writer, cfg mcp.Config, over *httpOptions) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var lis net.Listener
	if over != nil {
		var err error
		if lis, err = net.Listen("tcp", over.addr); err != nil {
			fmt.Fprintf(stderr, "%s-http %s: %v\n", commandPrefix, over.addr, err)
			return exitProtocol
		}
		if !mcp.IsLoopbackAddr(lis.Addr()) && !over.token.Set() {
			_ = lis.Close()
			fmt.Fprintf(stderr, "%s-http %s binds an address other machines reach: %v\n", commandPrefix, over.addr, mcp.ErrNoToken)
			return exitUnservable
		}
	}
	srv, err := mcp.New(ctx, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", commandPrefix, err)
		return exitProtocol
	}
	defer srv.Close(context.Background())

	if lis != nil {
		err = srv.ServeStreamableHTTP(ctx, mcp.HTTPConfig{Listener: lis, Origins: over.origins, Token: over.token, CertFile: over.certFile, KeyFile: over.keyFile})
		if err != nil {
			fmt.Fprintf(stderr, "%s%v\n", commandPrefix, err)
			return exitProtocol
		}
		return exitServed
	}

	err = srv.Run(ctx, &sdk.StdioTransport{})
	if err != nil && !endedWithTheStream(err) && ctx.Err() == nil {
		fmt.Fprintf(stderr, "%s%v\n", commandPrefix, err)
		return exitProtocol
	}
	return exitServed
}

// endedWithTheStream reports whether err is only the client's end of the stream:
// a client that closes it has still been served to its end.
func endedWithTheStream(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed) || errors.Is(err, sdk.ErrConnectionClosed)
}

// parseLogLevel reads the -log-level flag.
func parseLogLevel(name string) (slog.Level, error) {
	switch name {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return 0, fmt.Errorf("unknown level %q; the levels are debug, info, warn and error", name)
}

// instructions is what the server tells a connecting client about using it.
const instructions = "OpenSysML serves SysML v2 and KerML models. The workspace tools read and edit the model documents under the server's root folder and report diagnostics as the language server does; the runtime tools parse, evaluate, instantiate, execute, verify and convert models in one call; the session tools keep a REPL session whose declarations, instances and runs persist across calls. Positions in tool arguments are 1-based line and column numbers."
