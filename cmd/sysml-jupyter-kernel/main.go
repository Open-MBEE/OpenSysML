// Command sysml-jupyter-kernel serves SysML v2 to Jupyter: a kernel over the
// REPL session, so a notebook cell is read as the prompt reads a line.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	engineset "github.com/Open-MBEE/OpenSysML/internal/exec/engines"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/buildinfo"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/jupyter"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/all" // registers every REPL extension
	"github.com/Open-MBEE/OpenSysML/internal/frontend/usage"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

const errPrefix = kernelCommand + ":"

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

func main() {
	os.Exit(run())
}

func run() int {
	opts := registerFlags(flag.CommandLine)
	flag.Usage = func() { doc().WriteText(flag.CommandLine.Output(), flag.CommandLine) }
	flag.Parse()

	if opts.showMan {
		doc().WriteRoff(os.Stdout, flag.CommandLine, usage.DefaultManMeta())
		return 0
	}
	info := build()
	if opts.showVersion {
		fmt.Printf("%s version %s\n", kernelCommand, info.Version)
		fmt.Printf("commit: %s\n", info.Commit)
		fmt.Printf("built: %s\n", info.BuildTime)
		return 0
	}
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "%s unexpected argument %q\n", errPrefix, flag.Arg(0))
		return 2
	}

	if opts.printKernelspec || opts.install {
		spec, err := kernelspecOf(opts)
		if err != nil {
			fmt.Fprintln(os.Stderr, errPrefix, err)
			return 2
		}
		if opts.printKernelspec {
			return printKernelspec(spec)
		}
		return installKernelspec(opts, spec)
	}

	if opts.connectionFile == "" {
		fmt.Fprintf(os.Stderr, "%s -connection-file names the file Jupyter wrote; see -help\n", errPrefix)
		return 2
	}
	return serve(opts, info)
}

// serve runs the kernel over the connection until the front end shuts it down.
func serve(opts *options, info buildinfo.Info) int {
	conn, err := jupyter.ReadConnectionFile(opts.connectionFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, errPrefix, err)
		return 2
	}
	sess, err := newSession(info)
	if err != nil {
		fmt.Fprintln(os.Stderr, errPrefix, err)
		return 2
	}
	logger := jupyter.Stderr()
	if !opts.verbose {
		logger = nil
	}
	kernel := jupyter.New(kernelInfo(info), conn, jupyter.NewREPLEngine(sess), logger)

	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	// SIGINT is how a front end without the control channel interrupts a cell;
	// SIGTERM ends the kernel, as closing the notebook does.
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, syscall.SIGINT)
	defer signal.Stop(interrupts)
	go func() {
		for range interrupts {
			kernel.Interrupt()
		}
	}()
	terms := make(chan os.Signal, 1)
	signal.Notify(terms, syscall.SIGTERM)
	defer signal.Stop(terms)
	go func() {
		<-terms
		stop()
	}()

	if _, err := kernel.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, errPrefix, err)
		return 1
	}
	return 0
}

// kernelInfo is what the kernel tells a front end about itself.
func kernelInfo(info buildinfo.Info) jupyter.Info {
	return jupyter.Info{
		Implementation:        kernelCommand,
		ImplementationVersion: info.Version,
		Language: jupyter.LanguageInfo{
			Name:           "sysml",
			Version:        "2.0",
			MIMEType:       "text/x-sysml",
			FileExtension:  ".sysml",
			PygmentsLexer:  "text",
			CodeMirrorMode: "text/plain",
		},
		Banner: fmt.Sprintf("OpenSysML %s — SysML v2 in Jupyter. Declarations accumulate into the session model; %%help lists the commands.", info.Version),
		HelpLinks: []jupyter.HelpLink{
			{Text: "OpenSysML REPL guide", URL: "https://open-mbee.github.io/OpenSysML/guide/04-repl/"},
			{Text: "REPL command reference", URL: "https://open-mbee.github.io/OpenSysML/reference/repl-commands/"},
		},
	}
}

// newSession is a REPL session under the run bounds the environment sets, as
// the sysml command starts one.
func newSession(info buildinfo.Info) (*repl.Session, error) {
	budgets, err := runtime.BudgetsFromEnv()
	if err != nil {
		return nil, err
	}
	jobs, err := analysis.JobsFromEnv()
	if err != nil {
		return nil, err
	}
	sess := repl.NewSessionWithSourceConverter(convert.ModelSource)
	sess.SetToolVersion(kernelCommand + " " + info.Version)
	if err := sess.SetBudgets(budgets); err != nil {
		return nil, err
	}
	if err := sess.SetEngines(engineset.Default()); err != nil {
		return nil, err
	}
	if err := sess.SetJobs(jobs); err != nil {
		return nil, err
	}
	cache, err := libs.OpenRecordCache(false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s record cache unavailable, holding every file loaded: %v\n", errPrefix, err)
	}
	sess.SetRecordCache(cache)
	return sess, nil
}
