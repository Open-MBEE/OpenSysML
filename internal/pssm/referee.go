package pssm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// Meaning is what a pass means, and the first sentence of everything the
// referee prints about itself.
const Meaning = "A pass checks that the runtime reproduces UML behavior where the model has a " +
	"defensible SysML v2 mapping, provides a second opinion on the tool-choice rows, and is " +
	"never evidence of SysML v2 conformance."

// Bucket is the verdict the referee files a test under.
type Bucket string

// The five buckets, in report order.
const (
	BucketPass            Bucket = "pass"
	BucketFail            Bucket = "fail"
	BucketNotExpressible  Bucket = "not-expressible"
	BucketTerminateGap    Bucket = "terminate-gap"
	BucketDiffersByDesign Bucket = "differs-by-design"
)

// Buckets lists every bucket in report order.
var Buckets = []Bucket{BucketPass, BucketFail, BucketNotExpressible, BucketTerminateGap, BucketDiffersByDesign}

// Provenance identifies the suite a report measured.
type Provenance struct {
	Document string `json:"document"`
	Version  string `json:"version"`
	URL      string `json:"url"`
	// Digest is the sha256 of the suite file read, so a report is tied to the
	// bytes it measured and not to the pin's promise about them.
	Digest string `json:"suiteDigest"`
	Tests  int    `json:"tests"`
	// Recorded is the date -update stamped; a plain run leaves it empty.
	Recorded string `json:"recorded,omitempty"`
	// Develop is the develop commit whose runtime -update measured; a plain
	// run leaves it empty.
	Develop string `json:"develop,omitempty"`
}

// Report is one run of the referee over the suite: the buckets' counts, the
// gate CI compares, and one row per test, for the adjudicator.
type Report struct {
	Meaning    string         `json:"meaning"`
	Provenance Provenance     `json:"provenance"`
	Buckets    map[Bucket]int `json:"buckets"`
	Tests      []TestReport   `json:"tests"`
}

// TestReport is one test's row.
type TestReport struct {
	Name  string `json:"name"`
	Area  string `json:"area"`
	Class string `json:"class"`
	// Row is the alignment note row the test reports on, when the table maps it.
	Row     string `json:"row,omitempty"`
	RowKind string `json:"rowKind,omitempty"`
	Bucket  Bucket `json:"bucket"`
	// Reasons say why the test is in its bucket, one line each; a pass has none.
	Reasons []string `json:"reasons,omitempty"`
	// Expected and Reached are the admitted and reached trace sets of a test
	// that ran; Runs is how many linearizations exploration took.
	Expected []string `json:"expected,omitempty"`
	Reached  []string `json:"reached,omitempty"`
	Runs     int      `json:"runs,omitempty"`
}

// Options steer a run.
type Options struct {
	// Jobs is how many linearizations of one test explore at once; the report
	// is the same for any value.
	Jobs int
	// Keep, when set, is a directory each translated model is written to.
	Keep string
	// Filter, when set, keeps only the tests whose name contains it.
	Filter string
	// Budget bounds each test's exploration.
	Budget runtime.ExploreBudget
}

// Referee runs every test of the suite and files it. Only a problem with the
// harness itself, or a suite the reader could not read whole, is an error; a
// test that cannot be run is a row in its bucket.
func Referee(stop context.Context, s *Suite, prov Provenance, opts Options) (*Report, error) {
	if problems := s.Problems(); len(problems) > 0 {
		return nil, fmt.Errorf("the suite was not read whole, so its counts would not be its own:\n  %s", strings.Join(problems, "\n  "))
	}
	if opts.Jobs < 1 {
		opts.Jobs = 1
	}
	if opts.Budget.Runs == 0 {
		opts.Budget = DefaultBudget
	}
	if opts.Keep != "" {
		if err := os.MkdirAll(opts.Keep, 0o750); err != nil {
			return nil, err
		}
	}
	var tests []*Test
	for _, t := range s.Tests {
		if opts.Filter == "" || strings.Contains(t.Name, opts.Filter) {
			tests = append(tests, t)
		}
	}
	rows := make([]TestReport, 0, len(tests))
	for _, t := range tests {
		row, err := referee(stop, s, t, opts)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", t.Name, err)
		}
		rows = append(rows, row)
	}
	report := &Report{Meaning: Meaning, Provenance: prov, Buckets: make(map[Bucket]int, len(Buckets)), Tests: rows}
	for _, b := range Buckets {
		report.Buckets[b] = 0
	}
	for _, r := range rows {
		report.Buckets[r.Bucket]++
	}
	return report, nil
}

// referee files one test.
func referee(stop context.Context, s *Suite, t *Test, opts Options) (TestReport, error) {
	c := Classify(t)
	row := TestReport{Name: t.Name, Area: t.Area, Class: c.Class.String(), Expected: sortedCopy(t.Expected)}
	if r, ok := RowOf(t.Name); ok {
		row.Row = r.ID
		row.RowKind = string(r.Kind)
	}
	switch c.Class {
	case NotExpressible:
		row.Bucket = BucketNotExpressible
		row.Reasons = []string{c.Reason()}
		row.Expected = nil
		return row, nil
	case TerminateGap:
		row.Bucket = BucketTerminateGap
		row.Reasons = []string{c.Reason()}
		row.Expected = nil
		return row, nil
	}
	m, err := Emit(s, t)
	if err != nil {
		return row, fmt.Errorf("classified %s but not translated: %w", c.Class, err)
	}
	if opts.Keep != "" {
		if err := os.WriteFile(filepath.Join(opts.Keep, m.Name), []byte(m.Text), 0o600); err != nil {
			return row, err
		}
	}
	if problems := Validate(m); len(problems) > 0 {
		return row, fmt.Errorf("translated model does not validate: %s", strings.Join(problems, "; "))
	}
	x, err := Execute(stop, m, t.Expected, opts.Budget, opts.Jobs)
	if err != nil {
		return row, err
	}
	row.Reached, row.Runs = x.Reached, x.Runs
	if x.Passed() {
		row.Bucket = BucketPass
		return row, nil
	}
	row.Reasons = x.Reasons()
	row.Bucket = BucketFail
	if r, ok := RowOf(t.Name); ok {
		row.Reasons = append(row.Reasons, fmt.Sprintf("reports on %s (%s): %s", r.ID, r.Title, r.Kind))
		if r.Kind == RowDiffersByDesign {
			row.Bucket = BucketDiffersByDesign
		}
	}
	return row, nil
}

// sortedCopy is a sorted copy of a trace set.
func sortedCopy(traces []string) []string {
	out := append([]string(nil), traces...)
	sort.Strings(out)
	return out
}

// Summary renders the counts and the rows of every bucket but pass, one line
// per reason, for a terminal.
func (r *Report) Summary() string {
	var b strings.Builder
	b.WriteString(Meaning)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "%s %s (%s), %d tests\n", r.Provenance.Document, r.Provenance.Version, r.Provenance.Digest, r.Provenance.Tests)
	for _, bucket := range Buckets {
		fmt.Fprintf(&b, "  %-18s %3d\n", bucket, r.Buckets[bucket])
	}
	for _, bucket := range Buckets {
		if bucket == BucketPass {
			continue
		}
		first := true
		for _, t := range r.Tests {
			if t.Bucket != bucket {
				continue
			}
			if first {
				fmt.Fprintf(&b, "\n%s:\n", bucket)
				first = false
			}
			fmt.Fprintf(&b, "  %s", t.Name)
			if t.Row != "" {
				fmt.Fprintf(&b, " [%s]", t.Row)
			}
			b.WriteString("\n")
			for _, reason := range t.Reasons {
				fmt.Fprintf(&b, "    %s\n", reason)
			}
		}
	}
	return b.String()
}
