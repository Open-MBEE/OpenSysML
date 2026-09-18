// Package diff compares this implementation's diagnostics against the
// OMG SysML v2 Pilot Implementation over a corpus of models, and reports, per
// file, the diagnostics both agree on, the ones only we report (candidate false
// positives) and the ones only the pilot reports (candidate gaps).
//
// It is advisory: nothing in the build or the test suite depends on it, and it
// never touches tests/corpus/testdata/training_examples_expected.txt.
// Provision the reference validator with scripts/download-pilot-sysml-validator.sh,
// then run `go run -C tools ./cmd/pilot-diff`. See docs/project/pilot-differential.md.
package diff

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/errata"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// corpusRoot is one directory of models. Each language in it is compared as its
// own single batch: every file of that language is loaded into one resource set
// before any diagnostic is read, on both sides, because corpus files import
// each other.
type corpusRoot struct {
	Name string
	Dir  string
	// Skip lists sub-paths of Dir (slash-separated, relative to Dir) that
	// belong to another root.
	Skip []string
	// Pinned marks a root provisioned from the pilot pin rather than one this
	// repository owns: the two call for different actions when they move.
	Pinned bool
}

// languageBatch is one root's files in a single language, in comparison order.
type languageBatch struct {
	Kind  source.Kind
	Files []string
}

var defaultRoots = []corpusRoot{
	{Name: "training", Dir: "examples/sysml-v2-training", Pinned: true},
	{Name: "pilot-examples", Dir: "examples/pilot-corpora/sysml-examples", Pinned: true},
	{Name: "pilot-validation", Dir: "examples/pilot-corpora/sysml-validation", Pinned: true},
	{Name: "kerml-examples", Dir: "examples/pilot-corpora/kerml-examples", Pinned: true},
	{Name: "testdata", Dir: "tests/testdata"},
	{Name: "examples", Dir: "examples", Skip: []string{"sysml-v2-training", "pilot-corpora"}},
	// Hand-written models for behaviour classes the corpora do not cover, such
	// as redefining a feature inherited through an alias.
	{Name: "probes", Dir: "tools/referee/diff/testdata"},
}

// Main runs the pilot-diff command over args and returns its exit status.
func Main(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("pilot-diff", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoDir := flags.String("repo", "", "repository root (default: the module root containing this command)")
	validator := flags.String("validator", "", "pilot SysML validator executable (default: <repo>/build/pilot-sysml-validator/validate-sysml-batch)")
	kermlValidator := flags.String("kerml-validator", "", "KerML pilot validator executable (default: <repo>/build/pilot-kerml-validator/validate-kerml)")
	syside := flags.String("syside", "", "optional Sensmetry SysIDE launcher for a third column (default: <repo>/build/syside/validate-syside if present)")
	out := flags.String("out", "", "output directory for the reports (default: <repo>/build/pilot-diff)")
	timeout := flags.Duration("timeout", 0, "per-batch timeout for the pilot validator (0: no limit)")
	update := flags.Bool("update", false, "record this run as "+committedBaseline)
	check := flags.Bool("check", false, "fail unless this run reproduces "+committedBaseline)
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	opts := options{
		repo:           *repoDir,
		validator:      *validator,
		kermlValidator: *kermlValidator,
		syside:         *syside,
		out:            *out,
		timeout:        *timeout,
		update:         *update,
		check:          *check,
		log:            stderr,
	}
	if err := run(opts); err != nil {
		fmt.Fprintf(stderr, "pilot-diff: %v\n", err)
		return 1
	}
	return 0
}

// options is one run's command line, with the paths resolved by resolve; log
// receives the progress lines.
type options struct {
	repo           string
	validator      string
	kermlValidator string
	syside         string
	out            string
	timeout        time.Duration
	update         bool
	check          bool
	log            io.Writer
}

// resolve fills the paths left empty on the command line and reports the tools
// that are missing: the pilot validator is required, SysIDE only when named.
func (o *options) resolve() error {
	var err error
	if o.repo, err = repo.Choose(o.repo); err != nil {
		return err
	}
	o.validator = repo.Resolve(o.repo, o.validator)
	o.kermlValidator = repo.Resolve(o.repo, o.kermlValidator)
	o.syside = repo.Resolve(o.repo, o.syside)
	o.out = repo.Resolve(o.repo, o.out)
	if o.validator == "" {
		o.validator = filepath.Join(o.repo, "build", "pilot-sysml-validator", "validate-sysml-batch")
	}
	if o.kermlValidator == "" {
		o.kermlValidator = filepath.Join(o.repo, "build", "pilot-kerml-validator", "validate-kerml")
	}
	if o.out == "" {
		o.out = filepath.Join(o.repo, "build", "pilot-diff")
	}
	if _, err := os.Stat(o.validator); err != nil {
		return fmt.Errorf("pilot validator not found at %s: run ./scripts/download-pilot-sysml-validator.sh", o.validator)
	}

	// Named explicitly: fail loudly. Defaulted: the third column is optional,
	// and its absence must leave the two-way report byte-identical.
	requested := o.syside != ""
	if !requested {
		o.syside = filepath.Join(o.repo, "build", "syside", "validate-syside")
	}
	if _, err := os.Stat(o.syside); err != nil {
		if requested {
			return fmt.Errorf("SysIDE launcher not found at %s: run ./scripts/download-syside.sh", o.syside)
		}
		fmt.Fprintf(o.log, "comparing against the pilot only; run ./scripts/download-syside.sh for a third column\n")
		o.syside = ""
	}
	return nil
}

func run(opts options) error {
	if err := opts.resolve(); err != nil {
		return err
	}

	// The declared errata overlay: every root is compared a second time with
	// the corrections inside it applied to a copy, never to the corpus.
	overlay, err := errata.Load()
	if err != nil {
		return err
	}

	// Recorded relative to the repository where possible: the JSON is committed
	// as a baseline, so it must not carry a machine-specific path.
	report := &Report{Validator: relativeTo(opts.repo, opts.validator), Errata: newErrataReport(overlay)}
	if report.Pilot, err = pilotVersion(opts.validator); err != nil {
		return err
	}
	if report.Provenance, err = provenance(opts.repo, report.Pilot); err != nil {
		return err
	}
	// Only a recorded baseline is dated, so two plain runs stay byte-identical.
	if opts.update {
		report.Provenance.Recorded = baseline.Today()
	}
	if opts.syside != "" {
		version, library, err := sysideRelease(opts.syside)
		if err != nil {
			return err
		}
		report.Syside = &SysideInfo{
			Validator: relativeTo(opts.repo, opts.syside),
			Version:   version,
			Library:   library,
			Pilot:     report.Pilot,
			Scope:     sysideScope,
		}
	}

	for _, root := range defaultRoots {
		files, err := collectFiles(opts.repo, root)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			fmt.Fprintf(opts.log, "skipping %s: no .sysml or .kerml files (corpus not downloaded?)\n", root.Dir)
			continue
		}

		ours := make(map[string][]diagnostic, len(files))
		theirs := make(map[string][]diagnostic, len(files))
		for _, batch := range batchByLanguage(files) {
			fmt.Fprintf(opts.log, "%s: %d %s file(s)\n", root.Name, len(batch.Files), batch.Kind)

			pilot := opts.validator
			if batch.Kind == source.KindKerML {
				pilot = opts.kermlValidator
				if _, err := os.Stat(pilot); err != nil {
					return fmt.Errorf("KerML pilot validator not found at %s: run ./scripts/download-pilot-kerml-validator.sh", pilot)
				}
			}
			batchOurs, err := openSysMLDiagnostics(opts.repo, root.Dir, batch.Files)
			if err != nil {
				return err
			}
			batchTheirs, err := pilotDiagnostics(pilot, opts.repo, root.Dir, batch.Files, opts.timeout, opts.log)
			if err != nil {
				return err
			}
			for rel, diagnostics := range batchOurs {
				ours[rel] = diagnostics
			}
			for rel, diagnostics := range batchTheirs {
				theirs[rel] = diagnostics
			}
		}
		rootReport := compareRoot(root.Name, root.Dir, files, ours, theirs)
		if opts.syside != "" {
			fmt.Fprintf(opts.log, "%s: %d file(s) through syside\n", root.Name, len(files))
			third, err := sysideDiagnostics(opts.syside, opts.repo, root.Dir, files, opts.timeout, opts.log)
			if err != nil {
				return err
			}
			attachSyside(&rootReport, files, ours, theirs, third)
		}

		erratum, err := runErrata(root, files, overlay, ours, theirs, opts)
		if err != nil {
			return err
		}
		if erratum.applied > 0 {
			rootReport.ErrataTotals = &erratum.totals
			report.Errata.Applied += erratum.applied
			report.Errata.Findings = append(report.Errata.Findings, erratum.findings...)
		}
		report.Roots = append(report.Roots, rootReport)
	}

	report.summarize()
	// A mistyped -repo would otherwise look like a clean run.
	if report.Totals.Files == 0 {
		return fmt.Errorf("no model files found under %s", opts.repo)
	}
	fresh, err := writeReports(opts.out, report, opts.log)
	if err != nil {
		return err
	}
	committed := filepath.Join(opts.repo, filepath.FromSlash(committedBaseline))
	if opts.update {
		return baseline.Write(committed, fresh, opts.log)
	}
	if opts.check {
		return baseline.Reproduces(committed, fresh)
	}
	return nil
}

func relativeTo(repo, path string) string {
	rel, err := filepath.Rel(repo, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	return filepath.ToSlash(rel)
}

// batchByLanguage splits a root's files into one batch per language, SysML
// first. Each language is compared against its own reference validator, and
// batching per language rather than per file keeps the reference's cross-file
// reference resolution intact within a batch.
func batchByLanguage(files []string) []languageBatch {
	batches := []languageBatch{{Kind: source.KindSysML}, {Kind: source.KindKerML}}
	for _, rel := range files {
		for i := range batches {
			if batches[i].Kind == source.KindOf(rel) {
				batches[i].Files = append(batches[i].Files, rel)
			}
		}
	}
	out := make([]languageBatch, 0, len(batches))
	for _, batch := range batches {
		if len(batch.Files) > 0 {
			out = append(out, batch)
		}
	}
	return out
}

// collectFiles returns the root's model files, in either language, as sorted
// slash-separated paths relative to the root directory.
func collectFiles(repo string, root corpusRoot) ([]string, error) {
	dir := filepath.Join(repo, root.Dir)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	var files []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			for _, skip := range root.Skip {
				if rel == skip {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if source.KindOf(path) != source.KindUnknown {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", dir, err)
	}
	sort.Strings(files)
	return files, nil
}
