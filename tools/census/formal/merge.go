package formal

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/tools/census/manifest"
)

// unassessed is the rationale of a row just read from the documents.
const unassessed = "stated by the formal document; not yet assessed against the tree"

// Merge records the extracted constraints in a copy of the manifest: a
// constraint a pilot row cites by its formal name is that row's counterpart
// and is left to it; every other one becomes a spec-sourced row, keeping the
// implementation, evidence and status a recorded row already states. It
// returns the next manifest, whether it differs from the given one, and the
// disagreements it found between the pilot's rows and the documents.
func Merge(m *manifest.Manifest, extracted []Constraint) (*manifest.Manifest, bool, []string) {
	byName := make(map[string]Constraint, len(extracted))
	counts := make(map[string]int)
	for _, e := range extracted {
		byName[e.Name] = e
		counts[e.Document]++
	}
	var problems []string
	counterparts := make(map[string]bool)
	recorded := make(map[string]manifest.Constraint)
	next := *m
	next.Constraints = nil
	for _, c := range m.Constraints {
		if c.Source == manifest.SourceSpec {
			recorded[c.Name] = c
			continue
		}
		next.Constraints = append(next.Constraints, c)
		if c.Spec == nil || c.Spec.Name == "" {
			continue
		}
		e, ok := byName[c.Spec.Name]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s cites the formal name %s, which the documents do not state", c.Name, c.Spec.Name))
		case e.Document != c.Spec.Document || e.Clause != c.Spec.Clause:
			problems = append(problems, fmt.Sprintf("%s cites %s clause %s, the documents state %s in %s clause %s", c.Name, c.Spec.Document, c.Spec.Clause, c.Spec.Name, e.Document, e.Clause))
		}
		counterparts[c.Spec.Name] = true
	}
	var rows []manifest.Constraint
	for _, e := range extracted {
		if counterparts[e.Name] {
			continue
		}
		row, ok := recorded[e.Name]
		if !ok {
			row = manifest.Constraint{Implementation: "—", Message: "—", NegativeCase: "none", Codes: []string{}, Passes: []string{},
				Status: manifest.StatusUnknown, Rationale: unassessed}
		}
		row.Name, row.Source, row.Kind, row.Metaclass = e.Name, manifest.SourceSpec, e.Kind, e.Metaclass
		row.Spec = &manifest.SpecRef{Document: e.Document, Clause: e.Clause, Name: e.Name}
		row.Checks, row.OCL = e.Checks, e.OCL
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	next.Constraints = append(next.Constraints, rows...)
	next.FormalExtraction = &manifest.FormalExtractionRecord{Method: Method, Constraints: counts}
	sort.Strings(problems)
	return &next, !bytes.Equal(manifest.Encode(m), manifest.Encode(&next)), problems
}
