package apijson

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/tools/referee/baseline"
)

// FileSummary is one source file's share of its root's figures: what each side did
// with the file and how the elements attributed to it (see attributedFile) compared.
type FileSummary struct {
	OursRefused       bool `json:"oursRefused,omitempty"`
	OursUnconvertible bool `json:"oursUnconvertible,omitempty"`
	OursColliding     bool `json:"oursColliding,omitempty"`
	// OursSeparate marks a file converted alone because its root did not
	// convert as one model, so its cross-file references stay unresolved.
	OursSeparate         bool          `json:"oursSeparate,omitempty"`
	PilotFailed          bool          `json:"pilotFailed,omitempty"`
	PilotLibraryElements int           `json:"pilotLibraryElements"`
	Elements             ElementTotals `json:"elements"`
	Properties           Buckets       `json:"properties"`
	UnkeyedReferences    int           `json:"unkeyedReferences"`
	ExtensionProperties  int           `json:"extensionProperties"`
	DuplicateElements    Sides         `json:"duplicateElements"`
	AmbiguousKeys        Sides         `json:"ambiguousKeys"`
	UnkeyableElements    Sides         `json:"unkeyableElements"`
}

// Summary is what index.json carries beside the provenance: the exporter, the
// normalizations, the declared nondeterminism and the totals, a fold over the fragments.
type Summary struct {
	Exporter              string           `json:"exporter"`
	Normalizations        []string         `json:"normalizations"`
	PilotNondeterministic []Nondeterminism `json:"pilotNondeterministic"`
	Totals                Totals           `json:"totals"`
	Roots                 []RootReport     `json:"roots"`
}

// Counts are a row's seven buckets in the order present-equal, present-different,
// ours-missing, pilot-missing, absent-both, indeterminate, pilot-nondeterministic.
type Counts [7]int

func countsOf(b Buckets) Counts {
	return Counts{b.PresentEqual, b.PresentDifferent, b.OursMissing, b.PilotMissing, b.AbsentBoth, b.Indeterminate, b.PilotNondeterministic}
}

func (c Counts) buckets() Buckets {
	return Buckets{PresentEqual: c[0], PresentDifferent: c[1], OursMissing: c[2], PilotMissing: c[3], AbsentBoth: c[4], Indeterminate: c[5], PilotNondeterministic: c[6]}
}

// Fragment is one source file's record: its summary and its scorecard rows,
// keyed "<metaclass>.<property>".
type Fragment = baseline.Fragment[FileSummary, Counts]

// Committed is the whole baseline as the directory holds it.
type Committed = baseline.Baseline[FileSummary, Counts, Summary]

var commands = baseline.Commands{Refresh: refreshCommand, Reindex: reindexCommand}

// fragmentsOf assembles one fragment per file of a root and refuses an account
// filed under a file the root does not hold, which the totals would still count.
func fragmentsOf(root string, files []string, ours *oursExport, pilot *pilotExport, c *comparison) ([]Fragment, error) {
	known := make(map[string]bool, len(files))
	for _, file := range files {
		known[file] = true
	}
	compared := c.files[root]
	for _, side := range []struct {
		name  string
		filed []string
	}{{"ours", sortedKeys(ours.Files)}, {"pilot", sortedKeys(pilot.Files)}, {"comparison", sortedKeys(compared)}} {
		for _, file := range side.filed {
			if !known[file] {
				return nil, fmt.Errorf("root %s: the %s side files %q, which is not a file of the root", root, side.name, file)
			}
		}
	}
	out := make([]Fragment, 0, len(files))
	for _, file := range files {
		var s FileSummary
		if o := ours.Files[file]; o != nil {
			s.OursRefused, s.OursUnconvertible, s.OursColliding, s.OursSeparate = o.OursRefused, o.OursUnconvertible, o.OursColliding, o.OursSeparate
			s.DuplicateElements.Ours, s.AmbiguousKeys.Ours, s.UnkeyableElements.Ours = o.DuplicateElements.Ours, o.AmbiguousKeys.Ours, o.UnkeyableElements.Ours
		}
		if p := pilot.Files[file]; p != nil {
			s.PilotFailed, s.PilotLibraryElements = p.PilotFailed, p.PilotLibraryElements
			s.DuplicateElements.Pilot, s.AmbiguousKeys.Pilot, s.UnkeyableElements.Pilot = p.DuplicateElements.Pilot, p.AmbiguousKeys.Pilot, p.UnkeyableElements.Pilot
		}
		rows := make(map[string]Counts)
		if rec := compared[file]; rec != nil {
			s.Elements, s.Properties, s.UnkeyedReferences, s.ExtensionProperties = rec.elements, rec.buckets, rec.unkeyedReferences, rec.extensions
			for key, buckets := range rec.rows {
				rows[key] = countsOf(*buckets)
			}
		}
		out = append(out, Fragment{Root: root, File: file, Summary: s, Rows: rows})
	}
	return out, nil
}

// fold sums the fragments into the index's summary: the totals, and one root
// report per root in corpus order.
func fold(fragments []Fragment) Summary {
	summary := Summary{Exporter: exporterDescription, Normalizations: normalizations, PilotNondeterministic: pilotNondeterminism}
	byRoot := make(map[string]*RootReport)
	rows := make(map[string]bool)
	t := &summary.Totals
	for _, f := range fragments {
		r := byRoot[f.Root]
		if r == nil {
			r = &RootReport{Name: f.Root, Dir: rootDir(f.Root)}
			byRoot[f.Root] = r
		}
		s := f.Summary
		t.Files++
		r.Files++
		if s.OursRefused {
			t.OursRefused++
			r.OursRefused++
		}
		if s.OursUnconvertible {
			t.OursUnconvertible++
			r.OursUnconvertible++
		}
		if s.OursColliding {
			t.OursColliding++
			r.OursColliding++
		}
		if s.OursSeparate {
			r.OursSeparate = true
		}
		if s.PilotFailed {
			t.PilotFailed++
			r.PilotFailed++
		}
		t.PilotLibraryElements += s.PilotLibraryElements
		r.PilotLibraryElements += s.PilotLibraryElements
		t.Elements.addAll(s.Elements)
		r.Elements.addAll(s.Elements)
		t.Properties.addAll(s.Properties)
		r.Properties.addAll(s.Properties)
		t.UnkeyedReferences += s.UnkeyedReferences
		t.ExtensionProperties += s.ExtensionProperties
		for _, pair := range []struct {
			total, root *Sides
			file        Sides
		}{
			{&t.DuplicateElements, &r.DuplicateElements, s.DuplicateElements},
			{&t.AmbiguousKeys, &r.AmbiguousKeys, s.AmbiguousKeys},
			{&t.UnkeyableElements, &r.UnkeyableElements, s.UnkeyableElements},
		} {
			pair.total.Ours += pair.file.Ours
			pair.total.Pilot += pair.file.Pilot
			pair.root.Ours += pair.file.Ours
			pair.root.Pilot += pair.file.Pilot
		}
		for key := range f.Rows {
			rows[key] = true
		}
	}
	t.Rows = len(rows)
	for _, root := range roots() {
		if r := byRoot[root.Name]; r != nil {
			summary.Roots = append(summary.Roots, *r)
			delete(byRoot, root.Name)
		}
	}
	// A root the corpus no longer names still sums, after the known ones.
	for _, name := range sortedKeys(byRoot) {
		summary.Roots = append(summary.Roots, *byRoot[name])
	}
	return summary
}

func rootDir(name string) string {
	for _, root := range roots() {
		if root.Name == name {
			return root.Dir
		}
	}
	return ""
}

// scorecardOf sums the fragments' rows into the scorecard, most differing first.
func scorecardOf(fragments []Fragment) []Row {
	byKey := make(map[string]*Row)
	for _, f := range fragments {
		for key, counts := range f.Rows {
			row := byKey[key]
			if row == nil {
				metaclass, property, _ := splitScorecardKey(key)
				row = &Row{Metaclass: metaclass, Property: property}
				byKey[key] = row
			}
			row.addAll(counts.buckets())
		}
	}
	rows := make([]Row, 0, len(byKey))
	for _, row := range byKey {
		rows = append(rows, *row)
	}
	sortRows(rows, Row.Differing)
	return rows
}

// splitScorecardKey undoes scorecardKey: neither a metaclass nor a property holds a dot.
func splitScorecardKey(key string) (metaclass, property string, ok bool) {
	for i := 0; i < len(key); i++ {
		if key[i] == '.' {
			return key[:i], key[i+1:], true
		}
	}
	return key, "", false
}

// committedDir is where the baseline directory lives in a checkout.
func committedDir(checkout string) string {
	return filepath.Join(checkout, filepath.FromSlash(committedBaseline))
}

// committedIndex is the committed directory's index.json.
func committedIndex(checkout string) string {
	return filepath.Join(committedDir(checkout), baseline.IndexFile)
}

// readCommitted loads the committed baseline directory of a checkout.
func readCommitted(checkout string) (Committed, error) {
	return baseline.Read[FileSummary, Counts, Summary](committedDir(checkout))
}

// reindex rebuilds index.json from the committed fragments with this checkout's
// provenance, without the pilot: the mend for an index two branches both moved.
func reindex(checkout, libraries string, log io.Writer) error {
	record, err := provenance(checkout, libraries)
	if err != nil {
		return err
	}
	return baseline.Rewrite[FileSummary, Counts, Summary](committedDir(checkout), nil, nil, record, fold, log)
}

// sameFigures holds the run's directly summed figures to the fold of its
// fragments, so no figure the fragments do not carry leaves the baseline silently.
func sameFigures(direct *Report, fragments []Fragment) error {
	folded := fold(fragments)
	var differences []string
	if folded.Totals != direct.Totals {
		differences = append(differences, fmt.Sprintf("totals: folded %+v, summed %+v", folded.Totals, direct.Totals))
	}
	if len(folded.Roots) != len(direct.Roots) {
		differences = append(differences, fmt.Sprintf("%d roots folded, %d summed", len(folded.Roots), len(direct.Roots)))
	}
	for i := 0; i < len(folded.Roots) && i < len(direct.Roots); i++ {
		if folded.Roots[i] != direct.Roots[i] {
			differences = append(differences, fmt.Sprintf("root %s: folded %+v, summed %+v", direct.Roots[i].Name, folded.Roots[i], direct.Roots[i]))
		}
	}
	rows := scorecardOf(fragments)
	if len(rows) != len(direct.rows) {
		differences = append(differences, fmt.Sprintf("%d rows folded, %d summed", len(rows), len(direct.rows)))
	}
	for i := 0; i < len(rows) && i < len(direct.rows); i++ {
		if rows[i] != direct.rows[i] {
			differences = append(differences, fmt.Sprintf("row %s: folded %+v, summed %+v", scorecardKey(direct.rows[i]), rows[i], direct.rows[i]))
		}
	}
	if len(differences) == 0 {
		return nil
	}
	return fmt.Errorf("the per-file fragments do not sum to the run's figures:\n  %s", strings.Join(differences, "\n  "))
}
