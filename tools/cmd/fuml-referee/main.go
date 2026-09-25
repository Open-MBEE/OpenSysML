// Command fuml-referee runs the fUML reference implementation's activity tests,
// translated by rule into SysML v2 textual notation, against this runtime and
// files each activity in a bucket by comparing its output parameters with the
// implementation's committed record. It is advisory: CI compares the committed
// bucket counts, never a pass/fail verdict.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
	"github.com/Open-MBEE/OpenSysML/tools/referee/fuml"
)

// options are the command's flags.
type options struct {
	repo, suite, keep, filter, develop string
	jobs                               int
	asJSON, update, check              bool
}

// usage is what -h prints: the meaning of a pass first, then the flags.
func usage(flags *flag.FlagSet) string {
	return fuml.Meaning + `

Usage: fuml-referee [flags]

Runs the fUML reference implementation's activity tests against the runtime and
files each activity as pass, fail, not-expressible or differs-by-design, holding
the output parameters against the committed record ` + fuml.ExpectedPath + `.
The suite is fetched by ./scripts/download-fuml-suite.sh; when it is absent the
referee reports so and exits 0, unless ` + fuml.RequireEnv + ` is set.

Each run is bounded by the runtime's budgets and their environment overrides
(` + runtime.MaxStepsEnvVar + ` and the others the sysml command honors); the pin
in scripts/fuml-pin.sh takes the same FUML_* overrides the downloader does.

Flags:
` + flagDefaults(flags)
}

func flagDefaults(flags *flag.FlagSet) string {
	var b bytes.Buffer
	out := flags.Output()
	flags.SetOutput(&b)
	flags.PrintDefaults()
	flags.SetOutput(out)
	return b.String()
}

// parse reads the flags into options.
func parse(flags *flag.FlagSet, args []string) (options, error) {
	var o options
	flags.StringVar(&o.repo, "repo", "", "repository root (default: the module root above the working directory)")
	flags.StringVar(&o.suite, "suite", "", "directory holding the suite files (default: <repo>/"+fuml.SuiteDir+")")
	flags.IntVar(&o.jobs, "jobs", 1, "how many linearizations of one activity to explore at once; the report is the same for any value")
	flags.StringVar(&o.keep, "keep", "", "directory to write every translated model to, for debugging")
	flags.StringVar(&o.filter, "filter", "", "run only the activities whose name contains this")
	flags.BoolVar(&o.asJSON, "json", false, "print the full report as JSON instead of the summary")
	flags.BoolVar(&o.update, "update", false, "record this run as "+fuml.BaselinePath)
	flags.StringVar(&o.develop, "develop", "", "the develop commit -update records as measured (default: git merge-base HEAD origin/develop)")
	flags.BoolVar(&o.check, "check", false, "fail unless this run's bucket counts reproduce "+fuml.BaselinePath)
	flags.Usage = func() { fmt.Fprint(flags.Output(), usage(flags)) }
	err := flags.Parse(args)
	return o, err
}

func main() {
	flags := flag.NewFlagSet("fuml-referee", flag.ExitOnError)
	o, err := parse(flags, os.Args[1:])
	if err != nil {
		os.Exit(2)
	}
	if err := run(os.Stdout, os.Stderr, o); err != nil {
		fmt.Fprintf(os.Stderr, "fuml-referee: %v\n", err)
		os.Exit(1)
	}
}

// run is the command: locate and verify the suite and the record, referee,
// print, and record or check the baseline as asked.
func run(out, log io.Writer, o options) error {
	if o.jobs < 1 {
		return fmt.Errorf("-jobs must be at least 1")
	}
	if o.update && o.filter != "" {
		return fmt.Errorf("-update records the whole suite; drop -filter")
	}
	if o.check && o.filter != "" {
		return fmt.Errorf("-check compares the whole suite's counts; drop -filter")
	}
	if o.develop != "" && !o.update {
		return fmt.Errorf("-develop is recorded by -update only")
	}
	root, err := repo.Choose(o.repo)
	if err != nil {
		return err
	}
	o.suite, o.keep = repo.Resolve(root, o.suite), repo.Resolve(root, o.keep)
	dir, err := locate(root, o.suite)
	if errors.Is(err, fuml.ErrSuiteAbsent) {
		if !fuml.Required() {
			fmt.Fprintln(out, err)
			return nil
		}
		return fmt.Errorf("%s is set: %w", fuml.RequireEnv, err)
	}
	if err != nil {
		return err
	}
	pin, err := fuml.ReadPin(root)
	if err != nil {
		return err
	}
	s, err := fuml.ReadSuite(dir, pin)
	if err != nil {
		return err
	}
	x, err := fuml.ReadExpected(filepath.Join(root, filepath.FromSlash(fuml.ExpectedPath)))
	if err != nil {
		return err
	}
	if err := pin.Check(x); err != nil {
		return err
	}
	report, err := fuml.Referee(context.Background(), s, x, pin.Provenance(s.Activities()), fuml.Options{Jobs: o.jobs, Keep: o.keep, Filter: o.filter})
	if err != nil {
		return err
	}
	// Only a recorded baseline is dated and stamped, so plain runs stay byte-identical.
	if o.update {
		report.Provenance.Recorded = baseline.Today()
		if report.Provenance.Develop, err = repo.DevelopCommit(root, o.develop); err != nil {
			return err
		}
	}
	if o.asJSON {
		encoded, err := report.Encode()
		if err != nil {
			return err
		}
		if _, err := out.Write(encoded); err != nil {
			return err
		}
	} else {
		fmt.Fprint(out, report.Summary())
	}
	committed := filepath.Join(root, filepath.FromSlash(fuml.BaselinePath))
	if o.update {
		if err := fuml.WriteBaseline(committed, report); err != nil {
			return err
		}
		fmt.Fprintf(log, "recorded %s (dated %s, develop %s)\n", fuml.BaselinePath, report.Provenance.Recorded, report.Provenance.Develop)
	}
	if o.check {
		was, err := fuml.ReadBaseline(committed)
		if err != nil {
			return err
		}
		return fuml.Reproduces(was, report)
	}
	return nil
}

// locate is the suite directory: the given one when every suite file is in
// it, else the one under root.
func locate(root, given string) (string, error) {
	if given == "" {
		return fuml.Locate(root)
	}
	for _, name := range []string{fuml.TestsFile, fuml.ExceptionTestsFile, fuml.LibraryFile, fuml.JarFile} {
		if _, err := os.Stat(filepath.Join(given, name)); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("%w: no %s at %s; run ./scripts/download-fuml-suite.sh to provision it", fuml.ErrSuiteAbsent, name, given)
			}
			return "", err
		}
	}
	return given, nil
}
