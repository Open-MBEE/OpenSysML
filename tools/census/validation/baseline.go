package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	baselinePath  = "docs/project/validation-constraints-baseline.json"
	censusDocPath = "docs/project/validation-constraints.md"
)

// Census statuses as the baseline records them. The document shows each one
// with the marker spec-compliance.md uses for the same meaning.
const (
	StatusFaithful       = "faithful"
	StatusApproximate    = "approximate"
	StatusNotImplemented = "not-implemented"
	StatusDeliberate     = "deliberate"
	StatusKnownFailure   = "known-failure"
	StatusUnknown        = "unknown"
)

// statusMarkers maps each recorded status to the exact text of its status
// cell in the census table, in the order the summary line states them.
var statusMarkers = []struct {
	Status string
	Marker string
}{
	{StatusFaithful, "✅ faithful"},
	{StatusApproximate, "⚠️ approximate"},
	{StatusNotImplemented, "❌ not implemented"},
	{StatusDeliberate, "⛔ deliberate"},
	{StatusKnownFailure, "🚧 known failure"},
	{StatusUnknown, "❔ unknown — no case and no identifiable pass yet"},
}

// recordedDatePattern is the ISO calendar date the baseline's recorded field must hold.
var recordedDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// validRecordedDate reports whether s is an ISO calendar date that exists.
func validRecordedDate(s string) bool {
	if !recordedDatePattern.MatchString(s) {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func markerFor(status string) (string, bool) {
	for _, m := range statusMarkers {
		if m.Status == status {
			return m.Marker, true
		}
	}
	return "", false
}

// implemented reports whether a status claims we report the constraint, which
// is what a probe must back.
func implemented(status string) bool {
	return status == StatusFaithful || status == StatusApproximate
}

// Baseline is the committed record: the pin and jar the names came from, how
// they were read, and each name's census status.
type Baseline struct {
	PilotTag      string           `json:"pilotTag"`
	PilotCommit   string           `json:"pilotCommit"`
	PilotArtifact string           `json:"pilotArtifact"`
	Jar           JarRecord        `json:"jar"`
	Extraction    ExtractionRecord `json:"extraction"`
	Recorded      string           `json:"recorded,omitempty"`
	Constraints   []Constraint     `json:"constraints"`
}

// JarRecord identifies the artifact the names were extracted from.
type JarRecord struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

// ExtractionRecord says how the names were read so the extraction can be
// repeated by hand.
type ExtractionRecord struct {
	Classes []string `json:"classes"`
	Method  string   `json:"method"`
}

func extractionRecord() ExtractionRecord {
	classes := make([]string, 0, len(validatorClasses))
	for _, class := range validatorClasses {
		classes = append(classes, class.Path)
	}
	return ExtractionRecord{Classes: classes, Method: extractionMethod}
}

// Constraint is one named pilot constraint and its census status.
type Constraint struct {
	Name string `json:"name"`
	// Raw is the compiled identifier when it differs from Name (a trailing
	// underscore, or the `in` prefix of invalidateMetadataFeatureBody).
	Raw    string `json:"raw,omitempty"`
	Source string `json:"source"`
	Status string `json:"status"`
}

func loadBaseline(root string) (*Baseline, error) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(baselinePath))) // #nosec G304 -- fixed repository path
	if err != nil {
		return nil, err
	}
	var base Baseline
	if err := json.Unmarshal(content, &base); err != nil {
		return nil, fmt.Errorf("%s: %w", baselinePath, err)
	}
	return &base, nil
}

func writeBaseline(root string, base *Baseline) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(base); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, filepath.FromSlash(baselinePath)), buf.Bytes(), 0o644) // #nosec G306 -- documentation baseline
}

// validate checks the baseline on its own: sorted unique names, known sources
// and statuses, and the extraction record this program would write.
func (b *Baseline) validate() error {
	if !validRecordedDate(b.Recorded) {
		return fmt.Errorf("%s: records no ISO recording date (%q); re-record with -update", baselinePath, b.Recorded)
	}
	if b.Extraction.Method != extractionMethod {
		return fmt.Errorf("%s: extraction method differs from this program's; re-record with -update", baselinePath)
	}
	want := extractionRecord().Classes
	if strings.Join(b.Extraction.Classes, "\n") != strings.Join(want, "\n") {
		return fmt.Errorf("%s: extraction classes differ from this program's; re-record with -update", baselinePath)
	}
	seen := make(map[string]bool, len(b.Constraints))
	for i, c := range b.Constraints {
		if !constraintNamePattern.MatchString(c.Name) {
			return fmt.Errorf("%s: %q is not a normalized constraint name", baselinePath, c.Name)
		}
		if seen[c.Name] {
			return fmt.Errorf("%s: %s is listed twice", baselinePath, c.Name)
		}
		seen[c.Name] = true
		if i > 0 && b.Constraints[i-1].Name > c.Name {
			return fmt.Errorf("%s: %s is out of order (the list is sorted by name)", baselinePath, c.Name)
		}
		switch c.Source {
		case "kerml", "sysml", "both":
		default:
			return fmt.Errorf("%s: %s has source %q", baselinePath, c.Name, c.Source)
		}
		if _, ok := markerFor(c.Status); !ok {
			return fmt.Errorf("%s: %s has status %q", baselinePath, c.Name, c.Status)
		}
		if c.Raw != "" {
			match := constraintIdentifierPattern.FindStringSubmatch(c.Raw)
			if match == nil || match[1] != c.Name || c.Raw == c.Name {
				return fmt.Errorf("%s: %s records raw identifier %q", baselinePath, c.Name, c.Raw)
			}
		}
	}
	return nil
}

// matches compares the recorded list with a fresh extraction, naming every
// name only one side has and every source or raw identifier that moved.
func (b *Baseline) matches(extracted []Extracted) error {
	recorded := make(map[string]Constraint, len(b.Constraints))
	for _, c := range b.Constraints {
		recorded[c.Name] = c
	}
	fresh := make(map[string]Extracted, len(extracted))
	var problems []string
	for _, e := range extracted {
		fresh[e.Name] = e
		c, ok := recorded[e.Name]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s is in the jar but not in the baseline", e.Name))
		case c.Source != e.Source:
			problems = append(problems, fmt.Sprintf("%s has source %s in the jar, %s in the baseline", e.Name, e.Source, c.Source))
		case c.Raw != e.Raw:
			problems = append(problems, fmt.Sprintf("%s is compiled as %q, the baseline records %q", e.Name, e.Raw, c.Raw))
		}
	}
	for name := range recorded {
		if _, ok := fresh[name]; !ok {
			problems = append(problems, fmt.Sprintf("%s is in the baseline but not in the jar", name))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("%s does not list what the pinned jar contains (re-record with -update, then adjudicate every new name):\n  %s",
		baselinePath, strings.Join(problems, "\n  "))
}

// counts tallies the statuses in the order the summary line states them.
type counts struct {
	Total   int
	ByState map[string]int
}

func (b *Baseline) counts() counts {
	c := counts{Total: len(b.Constraints), ByState: make(map[string]int, len(statusMarkers))}
	for _, constraint := range b.Constraints {
		c.ByState[constraint.Status]++
	}
	return c
}

// Implemented is the number of constraints the census claims we report.
func (c counts) Implemented() int {
	return c.ByState[StatusFaithful] + c.ByState[StatusApproximate]
}
