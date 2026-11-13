// Command hideo-suite holds this tree to its committed per-case outcomes on the
// cross-MDK suite hideo67/sysmlv2-tests; advisory, see docs/project/hideo-suite.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
	"github.com/Open-MBEE/OpenSysML/tools/referee/hideo"
)

// options are the command's flags.
type options struct {
	repo, suite                             string
	check, update, acceptRegressions, noRun bool
}

const usage = `Usage: hideo-suite [flags]

Builds bin/sysml-grpc, provisions the suite's Python environment with uv, runs
the suite's four OpenSysML readers and compares every case's outcome with
` + hideo.BaselinePath + `. The suite is fetched by
./scripts/download-hideo-suite.sh; when it is absent the command reports so and
exits 0, unless ` + hideo.RequireEnv + ` is set.

A pass is agreement with the suite's expectation, which is the OMG pilot's
answer, on one read of one model; it is never evidence of conformance.

Flags:
`

func parse(flags *flag.FlagSet, args []string) (options, error) {
	var o options
	flags.StringVar(&o.repo, "repo", "", "repository root (default: the module root above the working directory)")
	flags.StringVar(&o.suite, "suite", "", "the suite checkout (default: <repo>/"+hideo.DefaultRoot+")")
	flags.BoolVar(&o.check, "check", false, "fail unless every case reproduces "+hideo.BaselinePath)
	flags.BoolVar(&o.update, "update", false, "record this run as "+hideo.BaselinePath+" and rewrite the table of "+hideo.DocPath)
	flags.BoolVar(&o.acceptRegressions, "accept-regressions", false, "with -update, record a run in which a case got worse")
	flags.BoolVar(&o.noRun, "no-run", false, "compare the results already under <suite>/"+hideo.ResultsDir+" instead of running the suites")
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), usage)
		flags.PrintDefaults()
	}
	err := flags.Parse(args)
	return o, err
}

func main() {
	flags := flag.NewFlagSet("hideo-suite", flag.ExitOnError)
	o, err := parse(flags, os.Args[1:])
	if err != nil {
		os.Exit(2)
	}
	if err := run(context.Background(), os.Stdout, os.Stderr, o); err != nil {
		fmt.Fprintf(os.Stderr, "hideo-suite: %v\n", err)
		os.Exit(1)
	}
}

// run is the command: locate and verify the suite, run it, then print, check or record.
func run(ctx context.Context, out, log io.Writer, o options) error {
	if o.check && o.update {
		return errors.New("-check and -update are exclusive")
	}
	if o.acceptRegressions && !o.update {
		return errors.New("-accept-regressions is for -update only")
	}
	root, err := repo.Choose(o.repo)
	if err != nil {
		return err
	}
	dir := repo.Resolve(root, o.suite)
	if dir == "" {
		dir = filepath.Join(root, filepath.FromSlash(hideo.DefaultRoot))
	}
	switch err := hideo.Locate(dir, root); {
	case errors.Is(err, hideo.ErrSuiteAbsent) && hideo.Required():
		return fmt.Errorf("%w (%s is set)", err, hideo.RequireEnv)
	case errors.Is(err, hideo.ErrSuiteAbsent):
		fmt.Fprintln(out, err)
		return nil
	case err != nil:
		return err
	}
	pin, err := hideo.ReadPin(root)
	if err != nil {
		return err
	}
	if err := pin.Verify(dir); err != nil {
		return err
	}
	if !o.noRun {
		if err := hideo.Execute(ctx, dir, root, log); err != nil {
			return err
		}
	}
	fresh, err := hideo.ReadRun(dir)
	if err != nil {
		return err
	}
	committed := filepath.Join(root, filepath.FromSlash(hideo.BaselinePath))
	doc := filepath.Join(root, filepath.FromSlash(hideo.DocPath))
	if o.update {
		return record(out, committed, doc, pin, fresh, o.acceptRegressions)
	}
	was, err := hideo.ReadBaseline(committed)
	if err != nil {
		return err
	}
	if was.Pin != pin {
		return fmt.Errorf("%s was measured at %s of %s, the pin is now %s of %s: re-record it with -update",
			hideo.BaselinePath, was.Pin.Commit, was.Pin.Repository, pin.Commit, pin.Repository)
	}
	movement := hideo.Compare(was, fresh)
	if movement.Reproduces() {
		fmt.Fprint(out, was.Summary())
		return hideo.CheckDocTable(doc, was)
	}
	fmt.Fprint(out, movement.Describe())
	if o.check {
		return fmt.Errorf("%s does not reproduce", hideo.BaselinePath)
	}
	return nil
}

// record writes the run as the baseline, refusing a regression against the
// committed one unless it was accepted.
func record(out io.Writer, committed, doc string, pin hideo.Pin, fresh hideo.Run, acceptRegressions bool) error {
	recorded, err := hideo.Record(pin, fresh)
	if err != nil {
		return err
	}
	if was, err := hideo.ReadBaseline(committed); err == nil && was.Pin == pin {
		movement := hideo.Compare(was, fresh)
		fmt.Fprint(out, movement.Describe())
		if len(movement.Worse) > 0 && !acceptRegressions {
			return fmt.Errorf("%d case(s) got worse; fix them, or adjudicate each and pass -accept-regressions", len(movement.Worse))
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// The record first: it is the one that can be refused, for lacking the table's markers.
	if err := hideo.WriteDocTable(doc, recorded); err != nil {
		return err
	}
	if err := hideo.WriteBaseline(committed, recorded); err != nil {
		return err
	}
	fmt.Fprint(out, recorded.Summary())
	fmt.Fprintf(out, "recorded %s and the table of %s\n", hideo.BaselinePath, hideo.DocPath)
	return nil
}
