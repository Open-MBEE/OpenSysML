package validation

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/tools/census/manifest"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
)

// censusDocPath is the page the manifest's table is rendered into.
const censusDocPath = "docs/project/validation-constraints.md"

// recordedDatePattern is the ISO calendar date the manifest's recorded field must hold.
var recordedDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// validRecordedDate reports whether s is an ISO calendar date that exists.
func validRecordedDate(s string) bool {
	if !recordedDatePattern.MatchString(s) {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func extractionRecord() manifest.ExtractionRecord {
	classes := make([]string, 0, len(validatorClasses))
	for _, class := range validatorClasses {
		classes = append(classes, class.Path)
	}
	return manifest.ExtractionRecord{Classes: classes, Method: extractionMethod}
}

// validateRecord checks the manifest on its own: its own invariants, the
// recording date, the extraction record this program would write and the
// pilot's raw identifiers.
func validateRecord(m *manifest.Manifest) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if !validRecordedDate(m.Recorded) {
		return fmt.Errorf("%s: records no ISO recording date (%q); re-record with -update", manifest.Path, m.Recorded)
	}
	if m.Extraction.Method != extractionMethod {
		return fmt.Errorf("%s: extraction method differs from this program's; re-record with -update", manifest.Path)
	}
	want := extractionRecord().Classes
	if strings.Join(m.Extraction.Classes, "\n") != strings.Join(want, "\n") {
		return fmt.Errorf("%s: extraction classes differ from this program's; re-record with -update", manifest.Path)
	}
	for _, c := range m.Constraints {
		if c.Raw != "" {
			match := constraintIdentifierPattern.FindStringSubmatch(c.Raw)
			if match == nil || match[1] != c.Name || c.Raw == c.Name {
				return fmt.Errorf("%s: %s records raw identifier %q", manifest.Path, c.Name, c.Raw)
			}
		}
	}
	return nil
}

// matches compares the recorded pilot rows with a fresh extraction, naming
// every name only one side has and every source or raw identifier that moved;
// the spec-sourced rows are not the jar's to name.
func matches(m *manifest.Manifest, extracted []Extracted) error {
	pilot := m.PilotRows()
	recorded := make(map[string]manifest.Constraint, len(pilot))
	for _, c := range pilot {
		recorded[c.Name] = c
	}
	fresh := make(map[string]Extracted, len(extracted))
	var problems []string
	for _, e := range extracted {
		fresh[e.Name] = e
		c, ok := recorded[e.Name]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s is in the jar but not in the manifest", e.Name))
		case c.Source != e.Source:
			problems = append(problems, fmt.Sprintf("%s has source %s in the jar, %s in the manifest", e.Name, e.Source, c.Source))
		case c.Raw != e.Raw:
			problems = append(problems, fmt.Sprintf("%s is compiled as %q, the manifest records %q", e.Name, e.Raw, c.Raw))
		}
	}
	for name := range recorded {
		if _, ok := fresh[name]; !ok {
			problems = append(problems, fmt.Sprintf("%s is in the manifest but not in the jar", name))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("%s does not list what the pinned jar contains (re-record with -update, then adjudicate every new name):\n  %s",
		manifest.Path, strings.Join(problems, "\n  "))
}

// recordedDate keeps a valid previous date when nothing but the date would
// change, so -update on a current tree is byte-identical across days.
func recordedDate(previous, next *manifest.Manifest) string {
	if previous != nil && validRecordedDate(previous.Recorded) {
		dated := *next
		dated.Recorded = previous.Recorded
		if reflect.DeepEqual(&dated, previous) {
			return previous.Recorded
		}
	}
	return baseline.Today()
}
