package manifest

import (
	"fmt"
	"sort"
	"strings"
)

// DocumentCoverage is how far one pinned formal document's check*, derive*
// and validate* constraints are accounted for: how many the document states
// (its spec-sourced rows plus the pilot's rows citing one of its names),
// how many of those a pilot validator names, and of the rest how many are
// implemented and how many remain unknown.
type DocumentCoverage struct {
	Document    string
	ByKind      map[string]int
	Stated      int
	PilotNamed  int
	SpecSourced int
	Implemented int
	Unknown     int
}

// Coverage tallies the formal documents' constraints per declared
// specification, in key order.
func (m *Manifest) Coverage() []DocumentCoverage {
	byDocument := make(map[string]*DocumentCoverage, len(m.Specifications))
	for key := range m.Specifications {
		byDocument[key] = &DocumentCoverage{Document: key, ByKind: make(map[string]int, len(Kinds))}
	}
	for _, c := range m.Constraints {
		if c.Spec == nil || c.Spec.Name == "" {
			continue
		}
		d, ok := byDocument[c.Spec.Document]
		if !ok {
			continue
		}
		d.Stated++
		d.ByKind[c.Kind]++
		if c.Source != SourceSpec {
			d.PilotNamed++
			continue
		}
		d.SpecSourced++
		switch {
		case Implemented(c.Status):
			d.Implemented++
		case c.Status == StatusUnknown:
			d.Unknown++
		}
	}
	keys := make([]string, 0, len(byDocument))
	for key := range byDocument {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	coverage := make([]DocumentCoverage, 0, len(keys))
	for _, key := range keys {
		coverage = append(coverage, *byDocument[key])
	}
	return coverage
}

// CoverageColumns are the formal-coverage table's columns, in order.
var CoverageColumns = []string{"Document", "check*", "derive*", "validate*", "Stated", "Pilot-named", "Spec-sourced", "Implemented", "❔ unknown"}

// RenderCoverage renders the formal-coverage table and its summary sentence.
func (m *Manifest) RenderCoverage() string {
	var b strings.Builder
	b.WriteString("| " + strings.Join(CoverageColumns, " | ") + " |\n")
	b.WriteString("|" + strings.Repeat("---|", len(CoverageColumns)) + "\n")
	total := DocumentCoverage{ByKind: make(map[string]int, len(Kinds))}
	for _, d := range m.Coverage() {
		spec := m.Specifications[d.Document]
		cells := []string{fmt.Sprintf("%s %s (`%s`)", spec.Name, spec.Version, spec.Document)}
		for _, kind := range Kinds {
			cells = append(cells, fmt.Sprint(d.ByKind[kind]))
			total.ByKind[kind] += d.ByKind[kind]
		}
		cells = append(cells, fmt.Sprint(d.Stated), fmt.Sprint(d.PilotNamed), fmt.Sprint(d.SpecSourced), fmt.Sprint(d.Implemented), fmt.Sprint(d.Unknown))
		b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
		total.Stated += d.Stated
		total.PilotNamed += d.PilotNamed
		total.SpecSourced += d.SpecSourced
		total.Implemented += d.Implemented
		total.Unknown += d.Unknown
	}
	fmt.Fprintf(&b, "\n**Formal coverage:** the pinned documents state %d constraints in the Constraints subsections of their abstract-syntax clauses — %d `check*`, %d `derive*` and %d `validate*`. %d of them are named by a pilot validator and counted in the census above; %d are recorded from the documents alone, outside the census, of which %d are implemented and %d remain ❔ unknown.",
		total.Stated, total.ByKind["check"], total.ByKind["derive"], total.ByKind["validate"], total.PilotNamed, total.SpecSourced, total.Implemented, total.Unknown)
	return b.String()
}
