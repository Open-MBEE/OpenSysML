// Command sysml-lsp is a stdio Language Server for SysML v2 / KerML.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/lsp"
	"github.com/Open-MBEE/OpenSysML/internal/usage"
)

var (
	// Version information - set via ldflags during build
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
	GoVersion = "unknown"
)

// Exit statuses: 0 for the protocol served to its end, 1 for a session that ended
// without one (an exit with no shutdown, or a protocol error), 2 for a command
// line the server cannot act on, as for sysml.
const (
	exitServed      = 0
	exitProtocol    = 1
	exitUnservable  = 2
	commandPrefix   = "sysml-lsp: "
	protocolMessage = "the protocol is spoken over stdin/stdout, so an editor starts this server rather than a shell"
)

// stdio adapts os.Stdin/os.Stdout into a single io.ReadWriteCloser.
type stdio struct{}

func (stdio) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (stdio) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (stdio) Close() error {
	return errors.Join(os.Stdin.Close(), os.Stdout.Close())
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run carries out what the command line asked for: the version, the help, or
// the protocol itself. A flag it could not read is reported with the usage,
// rather than entering protocol mode and dying on the first header line.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sysml-lsp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printUsage(fs.Output(), fs) }

	opts := registerFlags(fs)
	if err := fs.Parse(args); err != nil {
		// Parse has already reported the flag and printed the usage on stderr.
		return exitUnservable
	}

	if opts.showHelp {
		printUsage(stdout, fs)
		return exitServed
	}
	// The page asked for is the result of the run, like the help.
	if opts.showMan {
		doc().WriteRoff(stdout, fs, usage.DefaultManMeta())
		return exitServed
	}
	if opts.showVersion {
		fmt.Fprintf(stdout, "sysml-lsp %s\n", Version)
		fmt.Fprintf(stdout, "  Commit:     %s\n", Commit)
		fmt.Fprintf(stdout, "  Build time: %s\n", BuildTime)
		fmt.Fprintf(stdout, "  Go version: %s\n", GoVersion)
		return exitServed
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "%sunexpected argument %q; %s\n", commandPrefix, fs.Arg(0), protocolMessage)
		printUsage(stderr, fs)
		return exitUnservable
	}

	return serve(stderr, diag.ConformanceModeOf(opts.strict))
}

// printUsage writes the help to w, which the caller chooses: help asked for is
// a result, while help over a misuse belongs with the error.
func printUsage(w io.Writer, fs *flag.FlagSet) {
	doc().WriteText(w, fs)
}

// serve speaks the protocol over stdin/stdout until the client ends it, and
// reports the status the session earned: the one the client's exit notification
// asks for, or 1 for a session that ended in a protocol error.
func serve(stderr io.Writer, mode diag.ConformanceMode) int {
	ws := model.NewWorkspace(model.WithConformanceMode(mode))
	srv := lsp.NewServer(ws)
	err := srv.Run(context.Background(), stdio{})
	if err != nil && !endedWithTheStream(err) {
		fmt.Fprintf(stderr, "%s%v\n", commandPrefix, err)
		return exitProtocol
	}
	return srv.ExitCode()
}

// endedWithTheStream reports whether err is only the client's end of the stream:
// a client that closes it rather than exiting has still been served to its end.
func endedWithTheStream(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed)
}
