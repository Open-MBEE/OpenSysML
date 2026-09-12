// Command pssm-referee runs the OMG PSSM state-machine test suite, translated by
// rule into SysML v2 textual notation, against this runtime and files each test
// in a bucket. It is advisory: CI compares the committed bucket counts, never a
// pass/fail verdict.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/baseline"
	"github.com/Open-MBEE/OpenSysML/internal/pssm"
)

// options are the command's flags.
type options struct {
	repo, suite, keep, filter, develop string
	jobs                               int
	asJSON, update, check              bool
}

// usage is what -h prints: the meaning of a pass first, then the flags.
func usage(flags *flag.FlagSet) string {
	return pssm.Meaning + `

Usage: pssm-referee [flags]

Runs the PSSM test suite (OMG ptc/18-11-06) against the runtime and files each
test as pass, fail, not-expressible, terminate-gap or differs-by-design. The
suite is fetched by ./scripts/download-pssm-suite.sh; when it is absent the
referee reports so and exits 0, unless ` + pssm.RequireEnv + ` is set.

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
	flags.StringVar(&o.suite, "suite", "", "directory holding "+pssm.SuiteFile+" (default: <repo>/"+pssm.DefaultRoot+")")
	flags.IntVar(&o.jobs, "jobs", 1, "how many linearizations of one test to explore at once; the report is the same for any value")
	flags.StringVar(&o.keep, "keep", "", "directory to write every translated model to, for debugging")
	flags.StringVar(&o.filter, "filter", "", "run only the tests whose name contains this")
	flags.BoolVar(&o.asJSON, "json", false, "print the full report as JSON instead of the summary")
	flags.BoolVar(&o.update, "update", false, "record this run as "+pssm.BaselinePath)
	flags.StringVar(&o.develop, "develop", "", "the develop commit -update records as measured (default: git merge-base HEAD origin/develop)")
	flags.BoolVar(&o.check, "check", false, "fail unless this run's bucket counts reproduce "+pssm.BaselinePath)
	flags.Usage = func() { fmt.Fprint(flags.Output(), usage(flags)) }
	err := flags.Parse(args)
	return o, err
}

func main() {
	flags := flag.NewFlagSet("pssm-referee", flag.ExitOnError)
	o, err := parse(flags, os.Args[1:])
	if err != nil {
		os.Exit(2)
	}
	if err := run(os.Stdout, os.Stderr, o); err != nil {
		fmt.Fprintf(os.Stderr, "pssm-referee: %v\n", err)
		os.Exit(1)
	}
}

// run is the command: locate and verify the suite, referee it, print, and
// record or check the baseline as asked.
func run(out, log io.Writer, o options) error {
	if o.jobs < 1 {
		return fmt.Errorf("-jobs must be at least 1")
	}
	if o.update && o.filter != "" {
		return fmt.Errorf("-update records the whole suite; drop -filter")
	}
	if o.develop != "" && !o.update {
		return fmt.Errorf("-develop is recorded by -update only")
	}
	repo, err := chooseRepo(o.repo)
	if err != nil {
		return err
	}
	suiteRoot := o.suite
	if suiteRoot == "" {
		suiteRoot = filepath.Join(repo, filepath.FromSlash(pssm.DefaultRoot))
	}
	path, err := pssm.Locate(suiteRoot)
	if errors.Is(err, pssm.ErrSuiteAbsent) {
		if !pssm.Required() {
			fmt.Fprintln(out, err)
			return nil
		}
		return fmt.Errorf("%s is set: %w", pssm.RequireEnv, err)
	}
	if err != nil {
		return err
	}
	pin, err := pssm.ReadPin(repo)
	if err != nil {
		return err
	}
	if err := pin.Verify(path); err != nil {
		return err
	}
	s, err := pssm.ReadFile(path)
	if err != nil {
		return err
	}
	report, err := pssm.Referee(context.Background(), s, pin.Provenance(len(s.Tests)), pssm.Options{Jobs: o.jobs, Keep: o.keep, Filter: o.filter})
	if err != nil {
		return err
	}
	// Only a recorded baseline is dated and stamped, so plain runs stay byte-identical.
	if o.update {
		report.Provenance.Recorded = baseline.Today()
		if report.Provenance.Develop, err = developCommit(repo, o.develop); err != nil {
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
	committed := filepath.Join(repo, filepath.FromSlash(pssm.BaselinePath))
	if o.update {
		if err := pssm.WriteBaseline(committed, report); err != nil {
			return err
		}
		fmt.Fprintf(log, "recorded %s (dated %s, develop %s)\n", pssm.BaselinePath, report.Provenance.Recorded, report.Provenance.Develop)
	}
	if o.check {
		was, err := pssm.ReadBaseline(committed)
		if err != nil {
			return err
		}
		return pssm.Reproduces(was, report)
	}
	return nil
}

// developCommit is the given commit, or the develop commit the checkout is based on.
func developCommit(repo, given string) (string, error) {
	if given != "" {
		return given, nil
	}
	cmd := exec.Command("git", "-C", repo, "merge-base", "HEAD", "origin/develop")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git merge-base HEAD origin/develop: %w; pass -develop", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// chooseRepo is the given root, or the module root above the working directory.
func chooseRepo(path string) (string, error) {
	if path != "" {
		return filepath.Abs(path)
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod above %s; pass -repo", dir)
		}
		dir = parent
	}
}
