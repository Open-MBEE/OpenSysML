package apijson

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

var (
	docHeadlinePattern = regexp.MustCompile(`^\*\*Scorecard:\*\* ([0-9]+) files; ([0-9]+) elements matched \(([0-9]+) ours-only, ([0-9]+) pilot-only, ([0-9]+) of them implied\); properties: ([0-9]+) present-equal, ([0-9]+) present-different, ([0-9]+) ours-missing, ([0-9]+) pilot-missing, ([0-9]+) absent-both, ([0-9]+) indeterminate, ([0-9]+) pilot-nondeterministic\.$`)
	// docGapRowPattern matches a launch-ready gap row up to its two figures.
	docGapRowPattern = regexp.MustCompile("^\\| ([0-9]+) \\| `([A-Za-z]+)` \\| `([A-Za-z]+)` \\| ([0-9]+) \\| ([0-9]+) \\| ")
)

// TestRecordStatesTheBaselinesFigures holds the record to the baseline through the
// generator: the tree states no figure, the rendered page states the baseline's.
func TestRecordStatesTheBaselinesFigures(t *testing.T) {
	root, err := repo.Choose("")
	if err != nil {
		t.Fatal(err)
	}
	recorded := *committedFigures(t)
	committed, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(recordPath)))
	if err != nil {
		t.Fatal(err)
	}
	for number, line := range strings.Split(string(committed), "\n") {
		if docHeadlinePattern.MatchString(line) || docGapRowPattern.MatchString(line) {
			t.Errorf("%s:%d: states a figure the build renders from the baseline: %.60s", recordPath, number+1, line)
		}
	}
	blocks, err := renderBlocks(recorded, string(committed))
	if err != nil {
		t.Fatal(err)
	}
	record, err := splice(string(committed), blocks)
	if err != nil {
		t.Fatal(err)
	}
	totals := recorded.Totals
	want := []int{totals.Files, totals.Elements.Matched, totals.Elements.OursOnly, totals.Elements.PilotOnly, totals.Elements.PilotOnlyImplied,
		totals.Properties.PresentEqual, totals.Properties.PresentDifferent, totals.Properties.OursMissing, totals.Properties.PilotMissing, totals.Properties.AbsentBoth, totals.Properties.Indeterminate, totals.Properties.PilotNondeterministic}
	headlines, gapRows, lastOursMissing := 0, 0, -1
	for number, line := range strings.Split(record, "\n") {
		if m := docHeadlinePattern.FindStringSubmatch(line); m != nil {
			headlines++
			for i, field := range m[1:] {
				if got, _ := strconv.Atoi(field); got != want[i] {
					t.Errorf("%s:%d: headline figure %d is %d, the baseline says %d", recordPath, number+1, i+1, got, want[i])
				}
			}
		}
		if m := docGapRowPattern.FindStringSubmatch(line); m != nil {
			gapRows++
			if rank, _ := strconv.Atoi(m[1]); rank != gapRows {
				t.Errorf("%s:%d: gap row ranked %d, want %d", recordPath, number+1, rank, gapRows)
			}
			counts, ok := recorded.Scorecard[m[2]+"."+m[3]]
			if !ok {
				t.Errorf("%s:%d: no (%s, %s) row in the baseline", recordPath, number+1, m[2], m[3])
				continue
			}
			for i, index := range []int{2, 1} {
				if got, _ := strconv.Atoi(m[4+i]); got != counts[index] {
					t.Errorf("%s:%d: (%s, %s) column %d is %d, the baseline says %d", recordPath, number+1, m[2], m[3], 4+i, got, counts[index])
				}
			}
			if lastOursMissing >= 0 && counts[2] > lastOursMissing {
				t.Errorf("%s:%d: (%s, %s) is ranked below a row with fewer ours-missing", recordPath, number+1, m[2], m[3])
			}
			lastOursMissing = counts[2]
		}
	}
	if headlines != 1 {
		t.Errorf("%s states %d scorecard headlines, want exactly one matching %s", recordPath, headlines, docHeadlinePattern)
	}
	if gapRows < 10 {
		t.Errorf("%s lists %d launch-ready gap rows, want at least 10", recordPath, gapRows)
	}
	for _, launch := range launchReadyGaps(recorded, 25) {
		if !strings.Contains(record, fmt.Sprintf("| `%s` | `%s` | %d | %d |", launch.Metaclass, launch.Property, launch.OursMissing, launch.PresentDifferent)) {
			t.Errorf("%s does not adjudicate the gap (%s, %s), %d elements ours-missing", recordPath, launch.Metaclass, launch.Property, launch.OursMissing)
		}
	}
	for _, r := range recorded.Roots {
		if !strings.Contains(record, fmt.Sprintf("\n| `%s` | %d | %d | ", r.Dir, r.Files, r.Elements.Matched)) {
			t.Errorf("%s does not state the root %s (%d files, %d matched)", recordPath, r.Dir, r.Files, r.Elements.Matched)
		}
	}
}

// TestRenderBlocksFillsTheFiguresAndRefusesACommittedOne pins the rendering on a
// small report and the refusals: a figure in the tree, an unknown row, bad markers.
func TestRenderBlocksFillsTheFiguresAndRefusesACommittedOne(t *testing.T) {
	report := Report{Roots: []RootReport{{Dir: "examples", Files: 2}}}
	report.Totals.Files = 2
	report.Totals.Elements.Matched = 7
	report.setScorecard([]Row{
		{Metaclass: "Feature", Property: "type", Buckets: Buckets{PresentEqual: 10, PresentDifferent: 1, OursMissing: 5, PilotMissing: 2}},
		{Metaclass: "Feature", Property: "name", Buckets: Buckets{PresentEqual: 20, PresentDifferent: 3, OursMissing: 9}},
	})
	page := "<!-- doc-counts:begin apijson-scorecard -->the headline<!-- doc-counts:end apijson-scorecard -->\n" +
		"<!-- doc-counts:begin apijson-roots -->the roots<!-- doc-counts:end apijson-roots -->\n" +
		"<!-- doc-counts:begin apijson-present-different -->\n| metaclass | property |\n|---|---|\n| `Feature` | `type` |\n<!-- doc-counts:end apijson-present-different -->\n" +
		"<!-- doc-counts:begin apijson-pilot-missing -->\n| metaclass | property | clause |\n|---|---|---|\n| `Feature` | `type` | typing |\n<!-- doc-counts:end apijson-pilot-missing -->\n" +
		"<!-- doc-counts:begin apijson-launch-ready-gaps -->\n| metaclass | property | in-flight overlap | derivation clause |\n|---|---|---|---|\n" +
		"| `Feature` | `type` | none | `Feature::type` |\n| `Feature` | `name` | PR #1 | `Element::name` |\n<!-- doc-counts:end apijson-launch-ready-gaps -->\n"
	blocks, err := renderBlocks(report, page)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		scorecardBlock:        "**Scorecard:** 2 files; 7 elements matched (0 ours-only, 0 pilot-only, 0 of them implied); properties: 0 present-equal, 0 present-different, 0 ours-missing, 0 pilot-missing, 0 absent-both, 0 indeterminate, 0 pilot-nondeterministic.",
		rootsBlock:            "| root | files | matched | ours-only | pilot-only | present-equal | present-different | ours-missing | pilot-missing |\n|---|---:|---:|---:|---:|---:|---:|---:|---:|\n| `examples` | 2 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |",
		presentDifferentBlock: "| metaclass | property | present-equal | present-different |\n|---|---|---:|---:|\n| `Feature` | `type` | 10 | 1 |",
		pilotMissingBlock:     "| metaclass | property | pilot-missing | clause |\n|---|---|---:|---|\n| `Feature` | `type` | 2 | typing |",
		launchReadyBlock:      "| # | metaclass | property | ours-missing | present-different | in-flight overlap | derivation clause |\n|---:|---|---|---:|---:|---|---|\n| 1 | `Feature` | `name` | 9 | 3 | PR #1 | `Element::name` |\n| 2 | `Feature` | `type` | 5 | 1 | none | `Feature::type` |",
	}
	for name, text := range want {
		if blocks[name] != text {
			t.Errorf("block %s rendered\n%s\nwant\n%s", name, blocks[name], text)
		}
	}
	rendered, err := splice(page, blocks)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendered, "doc-counts:") || !strings.HasPrefix(rendered, want[scorecardBlock]+"\n"+want[rootsBlock]+"\n") {
		t.Errorf("splice left a marker or moved a block:\n%s", rendered)
	}
	for name, broken := range map[string]string{
		"a figure in the headline block": strings.Replace(page, "the headline", "935 files", 1),
		"a row the baseline lacks":       strings.Replace(page, "| `Feature` | `type` |\n", "| `Feature` | `owner` |\n", 1),
		"a figure column committed":      strings.Replace(page, "| metaclass | property |\n|---|---|\n| `Feature` | `type` |", "| metaclass | property | present-equal |\n|---|---|---:|\n| `Feature` | `type` | 10 |", 1),
		"a duplicated block":             page + page,
		"a missing end marker":           strings.Replace(page, "<!-- doc-counts:end apijson-roots -->", "", 1),
	} {
		if _, err := renderBlocks(report, broken); err == nil {
			t.Errorf("%s: renderBlocks accepted it", name)
		}
	}
}

// splice is the page as the site builds it: each block replaced by its
// rendered text, markers dropped (scripts/mkdocs_suite_figures.py).
func splice(page string, blocks map[string]string) (string, error) {
	for name, text := range blocks {
		begin, end := "<!-- doc-counts:begin "+name+" -->", "<!-- doc-counts:end "+name+" -->"
		start, stop := strings.Index(page, begin), strings.Index(page, end)
		if start < 0 || stop < start {
			return "", fmt.Errorf("no block named %q", name)
		}
		page = page[:start] + text + page[stop+len(end):]
	}
	return page, nil
}

// launchReadyGaps ranks the scorecard rows by the elements on which the pilot
// states a value and we state nothing, the record's launch-ready order.
func launchReadyGaps(recorded Report, limit int) []Row {
	rows := make([]Row, 0, len(recorded.Scorecard))
	for key, counts := range recorded.Scorecard {
		metaclass, property, _ := strings.Cut(key, ".")
		rows = append(rows, Row{Metaclass: metaclass, Property: property, Buckets: Buckets{
			PresentEqual: counts[0], PresentDifferent: counts[1], OursMissing: counts[2], PilotMissing: counts[3], AbsentBoth: counts[4], Indeterminate: counts[5]}})
	}
	sortRows(rows, func(r Row) int { return r.OursMissing })
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}
