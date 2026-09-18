package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testReport() *Report {
	return buildReport("2026-05", []GrammarReport{{
		Name: "Toy.xtext",
		Productions: []Row{
			{Grammar: "Toy.xtext", Name: "Part", Kind: KindRule, Line: 3, Bucket: BucketEvidence,
				Required: []string{"part", "def"}, File: "testdata/a.sysml",
				Evidence: []Citation{{Literal: "part", Root: "testdata", File: "testdata/a.sysml", Line: 1}},
				Branches: []Branch{{Literals: []string{"part"}, File: "testdata/a.sysml", Line: 1},
					{Literals: []string{"part", "variant"}, Missing: []string{"variant"}}}},
			{Grammar: "Toy.xtext", Name: "Port", Kind: KindRule, Line: 7, Bucket: BucketNoEvidence,
				Required: []string{"port"}, Missing: []string{"port"}, Reason: reasonAbsent},
			{Grammar: "Toy.xtext", Name: "Name", Kind: KindTerminal, Line: 11,
				Bucket: BucketIndistinguishable, Reason: reasonTerminal},
		},
	}}, []RootStat{{Name: "testdata", Dir: "testdata", Files: 1, Lines: 2}})
}

func TestBuildReportTotals(t *testing.T) {
	report := testReport()
	want := BucketTotals{Productions: 3, Evidence: 1, NoEvidence: 1, Indistinguishable: 1, Forms: 2, UnseenForms: 1}
	if report.Totals != want {
		t.Errorf("totals = %+v, want %+v", report.Totals, want)
	}
	if report.Grammars[0].Totals != want {
		t.Errorf("grammar totals = %+v, want %+v", report.Grammars[0].Totals, want)
	}
}

// The committed baseline keeps the counts but only the rows worth reading: the
// no-evidence productions and the ones with an unseen form.
func TestSummaryKeepsCountsAndGapsOnly(t *testing.T) {
	summary := testReport().Summary()
	if summary.Totals != testReport().Totals {
		t.Errorf("summary totals = %+v, want the full totals", summary.Totals)
	}
	var names []string
	for _, row := range summary.Grammars[0].Productions {
		names = append(names, row.Name)
	}
	if len(names) != 2 || names[0] != "Part" || names[1] != "Port" {
		t.Errorf("summary rows = %v, want the unseen-form and no-evidence rows", names)
	}
}

func TestWriteBaselineIsTheSummary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := writeBaseline(path, testReport()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path) // #nosec G304 -- the test wrote this path.
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"Name"`) {
		t.Error("baseline carries an indistinguishable row; it should hold only the gaps")
	}
	if !strings.Contains(string(data), `"unseenForms": 1`) {
		t.Error("baseline is missing the counts")
	}
}

// The report is diffed against a committed artifact, so rendering must be a
// function of the rows alone.
func TestWriteReportsDeterministic(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	if err := writeReports(first, testReport()); err != nil {
		t.Fatal(err)
	}
	if err := writeReports(second, testReport()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"grammar-coverage-tables.md", "grammar-coverage.json", "grammar-coverage.txt"} {
		a, err := os.ReadFile(filepath.Join(first, name))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(second, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(a) != string(b) {
			t.Errorf("%s differs between runs", name)
		}
	}
}

func TestReportRendering(t *testing.T) {
	text := testReport().Text()
	for _, want := range []string{
		"input-presence evidence, not execution coverage",
		"pilot tag 2026-05",
		`Toy.xtext:7 Port (rule) missing "port"`,
		`Toy.xtext:3 Part (rule) needs "part" "variant", never seen: "variant"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text output lacks %q:\n%s", want, text)
		}
	}

	markdown := testReport().Markdown()
	for _, want := range []string{
		"| `Part` | rule | `Toy.xtext:3` | evidence | 2 (1) |",
		"`part` at `testdata/a.sysml:1`",
		"| `Name` | terminal | `Toy.xtext:11` | indistinguishable | 0 (0) | — | " + reasonTerminal + " |",
	} {
		if !strings.Contains(markdown, want) {
			t.Errorf("markdown lacks %q:\n%s", want, markdown)
		}
	}
}
