package apijson

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

const recordPath = "docs/project/api-json-differential.md"

var (
	// The headline the record states, re-derived from the baseline's totals.
	docHeadlinePattern = regexp.MustCompile(`^\*\*Scorecard:\*\* ([0-9]+) files; ([0-9]+) elements matched \(([0-9]+) ours-only, ([0-9]+) pilot-only, ([0-9]+) of them implied\); properties: ([0-9]+) present-equal, ([0-9]+) present-different, ([0-9]+) ours-missing, ([0-9]+) pilot-missing, ([0-9]+) absent-both, ([0-9]+) indeterminate\.$`)
	// A row of the launch-ready gaps table: rank, metaclass, property, the
	// ours-missing and present-different counts, then prose columns.
	docGapRowPattern = regexp.MustCompile("^\\| ([0-9]+) \\| `([A-Za-z]+)` \\| `([A-Za-z]+)` \\| ([0-9]+) \\| ([0-9]+) \\|")
)

// The record's figures are the baseline's: `make docs-counts` runs this test so
// a re-recorded baseline cannot leave the prose behind.
func TestRecordStatesTheBaselinesFigures(t *testing.T) {
	root, err := repo.Choose("")
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(committedBaseline)))
	if err != nil {
		t.Fatal(err)
	}
	var recorded Report
	if err := json.Unmarshal(content, &recorded); err != nil {
		t.Fatal(err)
	}
	record, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(recordPath)))
	if err != nil {
		t.Fatal(err)
	}
	totals := recorded.Totals
	want := []int{totals.Files, totals.Elements.Matched, totals.Elements.OursOnly, totals.Elements.PilotOnly, totals.Elements.PilotOnlyImplied,
		totals.Properties.PresentEqual, totals.Properties.PresentDifferent, totals.Properties.OursMissing, totals.Properties.PilotMissing, totals.Properties.AbsentBoth, totals.Properties.Indeterminate}
	headlines, gapRows := 0, 0
	for number, line := range strings.Split(string(record), "\n") {
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
		}
	}
	if headlines != 1 {
		t.Errorf("%s states %d scorecard headlines, want exactly one matching %s", recordPath, headlines, docHeadlinePattern)
	}
	if gapRows < 10 {
		t.Errorf("%s lists %d launch-ready gap rows, want at least 10", recordPath, gapRows)
	}
	for _, launch := range launchReadyGaps(recorded, 25) {
		if !strings.Contains(string(record), fmt.Sprintf("| `%s` | `%s` |", launch.Metaclass, launch.Property)) {
			t.Errorf("%s does not list the gap (%s, %s), %d elements ours-missing", recordPath, launch.Metaclass, launch.Property, launch.OursMissing)
		}
	}
}

// launchReadyGaps ranks the baseline's rows by the elements left ours-missing,
// the way the record's launch-ready gaps table does.
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
