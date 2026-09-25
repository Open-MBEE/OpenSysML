package xpect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	oraclerepo "github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

const (
	xpectDocPath      = "docs/project/pilot-xpect.md"
	xpectBaselinePath = "docs/project/pilot-xpect-baseline.json"
)

var (
	xpectFilesLine      = regexp.MustCompile(`^(\d+) \.xt file\(s\), (\d+) unparsed, (\d+) missing declared resource\(s\)$`)
	xpectAssertionsLine = regexp.MustCompile(`^(\d+) assertion\(s\) declaring (\d+) expectation\(s\)$`)
	xpectAgreeLine      = regexp.MustCompile(`^agree (\d+) \(of which wording-only (\d+)\) \| disagree (\d+) \| unlocated (\d+) \| not adjudicated (\d+)$`)
	xpectKindRow        = regexp.MustCompile("^\\| `(errors|noErrors|linkedName|warnings|scope|exportedObjects)` \\|(.+)\\|$")
	xpectSuiteRow       = regexp.MustCompile("^\\| `(kerml|sysml)` \\|(.+)\\|$")
	xpectMovementRow    = regexp.MustCompile("^\\| `(errors|noErrors|linkedName|warnings|scope)` \\| [^|]+ \\| \\*\\*(\\d+) / (\\d+)\\*\\* \\|")
	xpectKindHeading    = regexp.MustCompile("^## `?(errors|noErrors|linkedName|warnings|scope)`? — (\\d+) of (\\d+)")
)

// TestPilotXpectDocumentCountsMatchBaseline guards this record's headline block,
// per-kind and per-suite tables, kind headings and the movement table's Now
// column against the committed baseline, so none of them can read as current
// while being stale. The baseline JSON is the only input: no Java, no corpus.
// Causal prose and the historical columns beside Now are out of scope.
func TestPilotXpectDocumentCountsMatchBaseline(t *testing.T) {
	report := xpectReadBaseline(t)
	lines := xpectReadDoc(t)

	xpectCheckHeadline(t, lines, report)
	xpectCheckKindTable(t, lines, report)
	xpectCheckSuiteTable(t, lines, report)
	xpectCheckKindHeadings(t, lines, report)
	xpectCheckMovementNow(t, lines, report)
}

func xpectCheckHeadline(t *testing.T, lines []string, report Report) {
	total := report.Totals
	seen := map[string]bool{}
	for i, line := range lines {
		text := strings.TrimSpace(line)
		switch {
		case xpectFilesLine.MatchString(text):
			seen["files"] = true
			m := xpectFilesLine.FindStringSubmatch(text)
			xpectWant(t, i, "files", m[1], total.Files)
			xpectWant(t, i, "unparsed", m[2], total.FilesUnparsed)
			xpectWant(t, i, "missing resources", m[3], total.MissingFiles)
		case xpectAssertionsLine.MatchString(text):
			seen["assertions"] = true
			m := xpectAssertionsLine.FindStringSubmatch(text)
			xpectWant(t, i, "assertions", m[1], total.Assertions)
			xpectWant(t, i, "expectations", m[2], total.Rows)
		case xpectAgreeLine.MatchString(text):
			seen["agree"] = true
			m := xpectAgreeLine.FindStringSubmatch(text)
			xpectWant(t, i, "agree", m[1], total.Agree)
			xpectWant(t, i, "wording-only", m[2], total.WordingOnly)
			xpectWant(t, i, "disagree", m[3], total.Disagree)
			xpectWant(t, i, "unlocated", m[4], total.Unlocated)
			xpectWant(t, i, "not adjudicated", m[5], total.NotAdjudicated)
		}
	}
	for _, want := range []string{"files", "assertions", "agree"} {
		if !seen[want] {
			t.Errorf("%s states no %s headline line", xpectDocPath, want)
		}
	}
}

// xpectCheckKindTable checks the per-kind table. Its last column counts the
// disagreements no tolerance would have accepted, which the report holds as the
// residue of the tolerance columns.
func xpectCheckKindTable(t *testing.T, lines []string, report Report) {
	found := 0
	for i, line := range lines {
		m := xpectKindRow.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		cells := xpectCells(m[2])
		if len(cells) != 10 {
			continue
		}
		kind := xpectKind(t, report, m[1], i)
		found++
		nothing := kind.Disagree - kind.SameLocation - kind.SameLine - kind.OtherSeverity - kind.Elsewhere
		for _, want := range []struct {
			cell  int
			field string
			value int
		}{
			{0, "expectations", kind.Rows},
			{1, "agree", kind.Agree},
			{2, "wording-only", kind.WordingOnly},
			{3, "disagree", kind.Disagree},
			{4, "not adjudicated", kind.NotAdjudicated},
			{5, "same-location", kind.SameLocation},
			{6, "same-line", kind.SameLine},
			{7, "severity-differs", kind.OtherSeverity},
			{8, "elsewhere", kind.Elsewhere},
			{9, "nothing", nothing},
		} {
			if cells[want.cell] == "—" {
				continue
			}
			xpectWant(t, i, fmt.Sprintf("kind %s %s", kind.Kind, want.field), cells[want.cell], want.value)
		}
	}
	if found != len(report.Kinds) {
		t.Errorf("%s states %d of the baseline's %d kind rows", xpectDocPath, found, len(report.Kinds))
	}
}

func xpectCheckSuiteTable(t *testing.T, lines []string, report Report) {
	found := 0
	for i, line := range lines {
		m := xpectSuiteRow.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		cells := xpectCells(m[2])
		if len(cells) != 5 {
			continue
		}
		suite := xpectSuite(t, report, m[1], i)
		found++
		for _, want := range []struct {
			cell  int
			field string
			value int
		}{
			{0, "files", suite.Totals.Files},
			{1, "expectations", suite.Totals.Rows},
			{2, "agree", suite.Totals.Agree},
			{3, "disagree", suite.Totals.Disagree},
			{4, "not adjudicated", suite.Totals.NotAdjudicated},
		} {
			xpectWant(t, i, fmt.Sprintf("suite %s %s", suite.Name, want.field), cells[want.cell], want.value)
		}
	}
	if found != len(report.Suites) {
		t.Errorf("%s states %d of the baseline's %d suite rows", xpectDocPath, found, len(report.Suites))
	}
}

// xpectCheckKindHeadings guards the per-kind section titles, which quote the
// same agreement as the table and are what a reader skims first.
func xpectCheckKindHeadings(t *testing.T, lines []string, report Report) {
	found := 0
	for i, line := range lines {
		m := xpectKindHeading.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		kind := xpectKind(t, report, m[1], i)
		found++
		xpectWant(t, i, "heading "+kind.Kind+" agree", m[2], kind.Agree)
		xpectWant(t, i, "heading "+kind.Kind+" expectations", m[3], kind.Rows)
	}
	if found != len(kindOrder) {
		t.Errorf("%s states %d of the %d adjudicated kind headings", xpectDocPath, found, len(kindOrder))
	}
}

// xpectCheckMovementNow guards only the Now column: the columns beside it are
// each round's own measurement and are labelled as such in the document.
func xpectCheckMovementNow(t *testing.T, lines []string, report Report) {
	found := 0
	for i, line := range lines {
		m := xpectMovementRow.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		kind := xpectKind(t, report, m[1], i)
		found++
		xpectWant(t, i, "movement "+kind.Kind+" agree", m[2], kind.Agree)
		xpectWant(t, i, "movement "+kind.Kind+" expectations", m[3], kind.Rows)
	}
	if found != len(kindOrder) {
		t.Errorf("%s states %d of the %d adjudicated movement rows", xpectDocPath, found, len(kindOrder))
	}
}

func xpectKind(t *testing.T, report Report, name string, line int) KindTotals {
	t.Helper()
	for _, kind := range report.Kinds {
		if kind.Kind == name {
			return kind
		}
	}
	t.Fatalf("%s:%d: %s states kind %q, which the baseline does not", xpectDocPath, line+1, xpectDocPath, name)
	return KindTotals{}
}

func xpectSuite(t *testing.T, report Report, name string, line int) SuiteReport {
	t.Helper()
	for _, suite := range report.Suites {
		if suite.Name == name {
			return suite
		}
	}
	t.Fatalf("%s:%d: %s states suite %q, which the baseline does not", xpectDocPath, line+1, xpectDocPath, name)
	return SuiteReport{}
}

func xpectCells(row string) []string {
	cells := strings.Split(row, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(strings.ReplaceAll(cells[i], "*", ""))
	}
	return cells
}

func xpectWant(t *testing.T, line int, field, got string, want int) {
	t.Helper()
	value, err := strconv.Atoi(got)
	if err != nil {
		t.Errorf("%s:%d: %s is not a count: %q", xpectDocPath, line+1, field, got)
		return
	}
	if value != want {
		t.Errorf("%s:%d: %s: want %d (%s), got %d — re-run %s and update this figure",
			xpectDocPath, line+1, field, want, xpectBaselinePath, value, refreshCommand)
	}
}

func xpectReadDoc(t *testing.T) []string {
	t.Helper()
	repo, err := oraclerepo.Root()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(xpectDocPath)))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(string(content), "\n")
}

func xpectReadBaseline(t *testing.T) Report {
	t.Helper()
	repo, err := oraclerepo.Root()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(xpectBaselinePath)))
	if err != nil {
		t.Fatal(err)
	}
	var report Report
	if err := json.Unmarshal(content, &report); err != nil {
		t.Fatal(err)
	}
	return report
}
