// Package reject checks the rejection direction the differential cannot
// see: it validates a hand-written negative corpus — models each violating one
// named rule — with both this implementation and the OMG SysML v2 Pilot
// Implementation, and buckets every case by who rejects it. A case the pilot
// rejects and we accept is a permissiveness gap.
//
// It is advisory: nothing in the build or the test suite depends on its
// verdicts. Provision the reference validators with
// scripts/download-pilot-sysml-validator.sh and
// scripts/download-pilot-kerml-validator.sh, then run
// `go run -C tools ./cmd/pilot-reject`. See docs/project/pilot-rejection.md.
package reject

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/errata"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// bucket names one quadrant of the two validators' verdicts.
const (
	bucketBothReject = "both-reject"
	bucketPilotOnly  = "pilot-only-rejects"
	bucketOursOnly   = "ours-only-rejects"
	bucketBothAccept = "both-accept"
)

// languageBatch is the corpus files in a single language, in comparison order.
type languageBatch struct {
	Kind  source.Kind
	Files []string
}

// defaultCorpus is the negative corpus, relative to the repository root.
const defaultCorpus = "tools/referee/reject/testdata/negative"

// Main runs the pilot-reject command over args and returns its exit status.
func Main(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("pilot-reject", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoDir := flags.String("repo", "", "repository root (default: the module root containing this command)")
	validator := flags.String("validator", "", "pilot SysML validator executable (default: <repo>/build/pilot-sysml-validator/validate-sysml-batch)")
	kermlValidator := flags.String("kerml-validator", "", "KerML pilot validator executable (default: <repo>/build/pilot-kerml-validator/validate-kerml)")
	corpus := flags.String("corpus", "", "negative corpus directory (default: <repo>/"+defaultCorpus+")")
	out := flags.String("out", "", "output directory for the reports (default: <repo>/build/pilot-reject)")
	timeout := flags.Duration("timeout", 0, "per-batch timeout for the pilot validator (0: no limit)")
	policy := flags.String("conformance", policyAuto,
		"conformance mode our verdicts are taken under: auto (extensions/ strictly, the rest by default), default, or strict")
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
		corpus:         *corpus,
		out:            *out,
		policy:         *policy,
		timeout:        *timeout,
		update:         *update,
		check:          *check,
		log:            stderr,
	}
	if err := run(opts); err != nil {
		fmt.Fprintf(stderr, "pilot-reject: %v\n", err)
		return 1
	}
	return 0
}

// options is one run's command line; log receives the progress lines.
type options struct {
	repo           string
	validator      string
	kermlValidator string
	corpus         string
	out            string
	policy         string
	timeout        time.Duration
	update         bool
	check          bool
	log            io.Writer
}

// adjudication is the corpus one run buckets, and the tools it buckets it with.
type adjudication struct {
	repo           string
	corpusDir      string
	policy         string
	validator      string
	kermlValidator string
	files          []string
	batches        []languageBatch
	timeout        time.Duration
	log            io.Writer
}

func run(opts options) error {
	policy, err := parsePolicy(opts.policy)
	if err != nil {
		return err
	}
	root, err := repo.Choose(opts.repo)
	if err != nil {
		return err
	}
	validator, kermlValidator := repo.Resolve(root, opts.validator), repo.Resolve(root, opts.kermlValidator)
	if validator == "" {
		validator = filepath.Join(root, "build", "pilot-sysml-validator", "validate-sysml-batch")
	}
	if kermlValidator == "" {
		kermlValidator = filepath.Join(root, "build", "pilot-kerml-validator", "validate-kerml")
	}
	corpusDir := defaultCorpus
	if opts.corpus != "" {
		corpusDir = relativeTo(root, repo.Resolve(root, opts.corpus))
	}
	out := repo.Resolve(root, opts.out)
	if out == "" {
		out = filepath.Join(root, "build", "pilot-reject")
	}
	files, err := collectCases(root, corpusDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no .sysml or .kerml files under %s", corpusDir)
	}

	batches := batchByLanguage(files)
	for _, batch := range batches {
		if _, err := os.Stat(pilotFor(batch.Kind, validator, kermlValidator)); err != nil {
			if batch.Kind == source.KindKerML {
				return fmt.Errorf("KerML pilot validator not found at %s: run ./scripts/download-pilot-kerml-validator.sh", kermlValidator)
			}
			return fmt.Errorf("pilot validator not found at %s: run ./scripts/download-pilot-sysml-validator.sh", validator)
		}
	}

	adj := adjudication{
		repo:           root,
		corpusDir:      corpusDir,
		policy:         policy,
		validator:      validator,
		kermlValidator: kermlValidator,
		files:          files,
		batches:        batches,
		timeout:        opts.timeout,
		log:            opts.log,
	}
	cases, err := adjudicate(adj)
	if err != nil {
		return err
	}

	overlay, err := errata.Load()
	if err != nil {
		return err
	}

	report := &Report{
		Validator:   relativeTo(root, validator),
		Corpus:      corpusDir,
		Conformance: policy,
		Errata:      newErrataReport(overlay),
	}
	if report.Pilot, err = pilotVersion(validator); err != nil {
		return err
	}
	if report.Provenance, err = provenance(root, report.Pilot, corpusDir, overlay, files); err != nil {
		return err
	}
	// Only a recorded baseline is dated, so two plain runs stay byte-identical.
	if opts.update {
		report.Provenance.Recorded = baseline.Today()
	}
	for _, rel := range files {
		report.Cases = append(report.Cases, *cases[rel])
	}
	report.summarize()

	if err := runErrata(report, overlay, adj, out); err != nil {
		return err
	}
	fresh, err := writeReports(out, report, opts.log)
	if err != nil {
		return err
	}
	committed := filepath.Join(root, filepath.FromSlash(committedBaseline))
	if opts.update {
		return baseline.Write(committed, fresh, opts.log)
	}
	if opts.check {
		return baseline.Reproduces(committed, fresh)
	}
	return nil
}

// adjudicate buckets every case of a corpus directory: ours in the policy's
// mode, ours in the default mode, and the pilot's, per language batch.
func adjudicate(adj adjudication) (map[string]*Case, error) {
	files := adj.files
	cases := make(map[string]*Case, len(files))
	for _, rel := range files {
		c, err := readCase(adj.repo, adj.corpusDir, rel)
		if err != nil {
			return nil, err
		}
		c.Mode = modeFor(adj.policy, c.Source).String()
		cases[rel] = c
	}
	modes := make(map[string]diag.ConformanceMode, len(cases))
	// The default mode is evaluated for every case as well, so a case asked
	// strictly reports what the default mode says instead of implying it agreed.
	defaults := make(map[string]diag.ConformanceMode, len(cases))
	for rel, c := range cases {
		modes[rel] = modeFor(adj.policy, c.Source)
		defaults[rel] = diag.ConformanceDefault
	}

	for _, batch := range adj.batches {
		fmt.Fprintf(adj.log, "negative corpus: %d %s case(s)\n", len(batch.Files), batch.Kind)
		pilot := pilotFor(batch.Kind, adj.validator, adj.kermlValidator)
		ours, err := openSysMLErrors(adj.repo, adj.corpusDir, batch.Files, modes)
		if err != nil {
			return nil, err
		}
		oursDefault, err := openSysMLErrors(adj.repo, adj.corpusDir, batch.Files, defaults)
		if err != nil {
			return nil, err
		}
		theirs, err := pilotErrors(pilot, adj.repo, adj.corpusDir, batch.Files, adj.timeout, adj.log)
		if err != nil {
			return nil, err
		}
		for _, rel := range batch.Files {
			classify(cases[rel], ours[rel], oursDefault[rel], theirs[rel])
		}
	}
	return cases, nil
}

// pilotFor picks the reference validator for a language batch.
func pilotFor(kind source.Kind, validator, kermlValidator string) string {
	if kind == source.KindKerML {
		return kermlValidator
	}
	return validator
}

// classify fills the case's verdicts. A side rejects when it reports at least
// one error-severity diagnostic; warnings do not count. oursDefault is what the
// default mode said, recorded for a case asked strictly so that a strict
// agreement does not read as a default one.
func classify(c *Case, ours, oursDefault, theirs []string) {
	c.OursErrors = len(ours)
	c.PilotErrors = len(theirs)
	c.Bucket = bucketOf(len(ours), len(theirs))
	switch c.Bucket {
	case bucketPilotOnly:
		c.Pilot = theirs
	case bucketOursOnly:
		c.Ours = ours
	}
	if c.Mode == diag.ConformanceStrict.String() {
		c.DefaultErrors = len(oursDefault)
		c.DefaultBucket = bucketOf(len(oursDefault), len(theirs))
	}
}

// bucketOf names the quadrant a pair of error counts falls in.
func bucketOf(ours, theirs int) string {
	switch {
	case theirs > 0 && ours > 0:
		return bucketBothReject
	case theirs > 0:
		return bucketPilotOnly
	case ours > 0:
		return bucketOursOnly
	default:
		return bucketBothAccept
	}
}

// readCase reads one corpus file and its mandatory header: the first line must
// state the violated rule and its citation, so no case is anecdotal.
func readCase(repo, dir, rel string) (*Case, error) {
	// #nosec G304 -- the corpus root to validate is named on the command line.
	content, err := os.ReadFile(filepath.Join(repo, dir, filepath.FromSlash(rel)))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	first, _, _ := strings.Cut(string(content), "\n")
	rule, ok := strings.CutPrefix(first, "// Invalid: ")
	if !ok {
		return nil, fmt.Errorf("%s: first line must be `// Invalid: <rule> (<citation>).`", rel)
	}
	src, _, _ := strings.Cut(rel, "/")
	return &Case{Path: rel, Source: src, Rule: strings.TrimSpace(rule)}, nil
}

// relativeTo is the corpus directory as the report names it: slash-separated
// and relative to the repository, which every path reaching it is under or beside.
func relativeTo(repo, path string) string {
	rel, err := filepath.Rel(repo, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

// batchByLanguage splits the corpus into one batch per language, SysML first.
// Each language runs against its own reference validator; the cases are
// mutually independent, so batching only amortizes the validator's startup.
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

// collectCases returns the corpus files, in either language, as sorted
// slash-separated paths relative to the corpus directory.
func collectCases(repo, dir string) ([]string, error) {
	root := filepath.Join(repo, filepath.FromSlash(dir))
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if source.KindOf(path) == source.KindUnknown {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", root, err)
	}
	sort.Strings(files)
	return files, nil
}
