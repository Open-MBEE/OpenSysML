// Package xpect compares this implementation's behaviour against the OMG
// SysML v2 Pilot Implementation's own Xpect test suites: the .xt files declare,
// inline, the diagnostics and name-resolution results their implementers intend,
// so unlike the differential this is a comparison against declared expectations
// rather than observed output.
//
// It is advisory: nothing in the build or the test suite depends on it, for the
// same reason the differential is — it needs an unvendored corpus at the pinned
// tag. Provision it with scripts/download-pilot-xpect.sh, then run
// `go run -C tools ./cmd/pilot-xpect`. See docs/project/pilot-xpect.md.
package xpect

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/errata"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// suite is one Xpect plugin of the pilot repository, as the downloader lays it
// out under build/pilot-xpect-corpus.
type suite struct {
	Name string
	Dir  string
}

var defaultSuites = []suite{
	{Name: "kerml", Dir: "build/pilot-xpect-corpus/kerml"},
	{Name: "sysml", Dir: "build/pilot-xpect-corpus/sysml"},
}

// Main runs the pilot-xpect command over args and returns its exit status.
func Main(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("pilot-xpect", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoDir := flags.String("repo", "", "repository root (default: the module root containing this command)")
	out := flags.String("out", "", "output directory for the reports (default: <repo>/build/pilot-xpect)")
	jobs := flags.Int("jobs", runtime.NumCPU(), "number of .xt files to compare in parallel")
	update := flags.Bool("update", false, "record this run as "+committedBaseline)
	check := flags.Bool("check", false, "fail unless this run reproduces "+committedBaseline)
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	root, err := repo.Choose(*repoDir)
	if err != nil {
		fmt.Fprintf(stderr, "pilot-xpect: %v\n", err)
		return 1
	}

	if err := run(root, repo.Resolve(root, *out), *jobs, *update, *check, stderr); err != nil {
		fmt.Fprintf(stderr, "pilot-xpect: %v\n", err)
		return 1
	}
	return 0
}

// run compares every present suite and writes the reports under out; log
// receives the progress lines.
func run(repo, out string, jobs int, update, check bool, log io.Writer) error {
	if out == "" {
		out = filepath.Join(repo, "build", "pilot-xpect")
	}
	if jobs < 1 {
		jobs = 1
	}

	overlay, err := errata.Load()
	if err != nil {
		return err
	}

	pin, err := baseline.ReadPin(repo)
	if err != nil {
		return err
	}
	report := &Report{
		Pilot:   pin.Tag,
		Corpus:  "build/pilot-xpect-corpus",
		Library: "the suite's own /library* copies, loaded per fixture as its XPECT_SETUP declares them",
		Errata:  newErrataReport(overlay),
	}
	// The errata-applied figure is adjudicated in its own shadow run, so the
	// as-published report above is untouched by it.
	erratum := &Report{}
	for _, s := range defaultSuites {
		dir := filepath.Join(repo, filepath.FromSlash(s.Dir))
		if _, err := os.Stat(dir); err != nil {
			fmt.Fprintf(log, "skipping %s: %s is absent (run scripts/download-pilot-xpect.sh)\n", s.Name, s.Dir)
			continue
		}
		files, err := collectXT(dir)
		if err != nil {
			return err
		}
		published := compareAll(dir, files, jobs)
		report.Suites = append(report.Suites, SuiteReport{Name: s.Name, Dir: s.Dir, Files: published})

		corrected, applied, err := erratumSuite(s, overlay, repo, out, jobs, log)
		if err != nil {
			return err
		}
		if applied == 0 {
			corrected = append([]fileResult(nil), published...)
		}
		report.Errata.Applied += applied
		erratum.Suites = append(erratum.Suites, SuiteReport{Name: s.Name, Dir: s.Dir, Files: corrected})
	}
	if len(report.Suites) == 0 {
		return fmt.Errorf("no suite found under build/pilot-xpect-corpus; run scripts/download-pilot-xpect.sh")
	}
	inputs, err := suiteInputs(repo)
	if err != nil {
		return err
	}
	if report.Provenance, err = provenance(repo, overlay, inputs); err != nil {
		return err
	}
	// Only a recorded baseline is dated, so two plain runs stay byte-identical.
	if update {
		report.Provenance.Recorded = baseline.Today()
	}
	report.summarize()
	erratum.summarize()
	report.Errata.Totals = erratum.Totals
	report.Errata.Kinds = erratum.Kinds
	if report.Errata.Applied == 0 {
		report.Errata.Note = "no declared correction lies under build/pilot-xpect-corpus, so the errata-applied corpus is byte-identical to the published one and both figures coincide"
	} else {
		report.Errata.Note = "adjudicated again over a corrected copy of the suites; the published corpus is unchanged on disk"
	}
	fresh, err := writeReports(out, report, log)
	if err != nil {
		return err
	}
	committed := filepath.Join(repo, filepath.FromSlash(committedBaseline))
	if update {
		return baseline.Write(committed, fresh, log)
	}
	if check {
		return baseline.Reproduces(committed, fresh)
	}
	return nil
}

// compareAll adjudicates every file, in parallel but into a fixed order, so the
// report does not depend on the scheduler.
func compareAll(dir string, files []string, jobs int) []fileResult {
	results := make([]fileResult, len(files))
	libs := newLibraryCache()
	var wg sync.WaitGroup
	work := make(chan int)
	for range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				results[i] = compareOne(dir, files[i], libs)
			}
		}()
	}
	for i := range files {
		work <- i
	}
	close(work)
	wg.Wait()
	return results
}

func compareOne(dir, rel string, libs *libraryCache) fileResult {
	// #nosec G304 -- the suite directory is named on the command line.
	content, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		return fileResult{Path: rel, Problems: []string{err.Error()}}
	}
	language := "sysml"
	if source.KindOf(strings.TrimSuffix(rel, ".xt")) == source.KindKerML {
		language = "kerml"
	}
	return compareFile(dir, parseXT(rel, language, content), libs)
}

// collectXT lists a suite's .xt files, relative to its directory, sorted.
func collectXT(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".xt") {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
