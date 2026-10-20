package apijson

import (
	"fmt"
	"io"
	"strings"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	reports "github.com/Open-MBEE/OpenSysML/tools/oracle/report"
)

// exporterDescription names the pilot code path the bridge drives, so a reader
// of the baseline knows which export it compares against.
const exporterDescription = "org.omg.sysml.util.traversal.facade.impl.JsonElementProcessingFacade with setIsIncludeDerived(true), " +
	"each input resource's root elements visited through org.omg.sysml.util.traversal.Traversal, toJsonTree(true) " +
	"(the path of SysMLInteractive.export, the %export magic), via scripts/pilot-json-exporter/ExportJSON.java"

// normalizations lists what the comparison equates, each provably a
// representation of the same fact rather than a different fact.
var normalizations = []string{
	"The pilot's {payload, identity} envelope is unwrapped; the payload is the element.",
	"Every element is addressed by a stable key, not an id: a named element by its qualifiedName; an unnamed one by <ownerKey>/<metaclass>[i], i counting the owner's preceding non-implied owned relationships (a relationship) or owned elements (otherwise) of the same metaclass; an ownerless one by /<file>/<metaclass>#i over its document's root elements. The pilot mints a fresh UUID for every user element on each run, so its ids carry no information.",
	"A reference ({\"@id\": x}) compares as the key of x; the pilot bridge's key table covers library elements too. A reference no key addresses is counted under unkeyedReferences and makes the comparison indeterminate, a bucket apart from the four the scorecard ranks: two unknown targets are not known equal, nor different.",
	"elementId, memberElementId and ownedMemberElementId hold ids as strings and are mapped through the same keys; elementId itself is identity, not derivation, and is left out of the buckets.",
	"JSON null, a missing property and an empty collection all state nothing (absent); a property absent on both sides is absent-both, not present-equal.",
	"A Boolean attribute written at its metamodel default (model/kerml.ecore and model/SysML.ecore in the pinned jar: isUnique defaults to true, every other defaulted Boolean to false) against a side that leaves it out is absent-both, since leaving it out states the default; a value the other side lacks is missing. The derived Booleans (isConjugated, isLibraryElement, isModelLevelEvaluable, isReference, mayTimeVary), isVariation and a LiteralBoolean's value have no default, so leaving one out states nothing.",
	"Property order and the order of object keys are ignored; the order of collections is compared.",
	"Properties outside the metamodel (this repository's sysx: annotations) are left out and counted under extensionProperties.",
	"Identical copies of one element on one side (the pilot writes an element in every file whose traversal reaches it) are one element, counted under duplicateElements; elements two of which compute the same key but differ are dropped from matching and counted under ambiguousKeys; elements no key addresses are counted under unkeyableElements.",
	"The pilot's implied relationships (isImplied) have no counterpart in the notation and are counted under pilotOnlyImplied.",
	"Standard-library elements the pilot writes (isLibraryElement, one copy per file that refers to them) are left out and counted under pilotLibraryElements; this repository's writer refers to library elements and never writes them.",
}

// Report is what one run writes; it is also the committed baseline's shape.
type Report struct {
	Provenance     baseline.Record `json:"provenance"`
	Exporter       string          `json:"exporter"`
	Normalizations []string        `json:"normalizations"`
	Totals         Totals          `json:"totals"`
	Roots          []RootReport    `json:"roots"`
	// Scorecard maps "<metaclass>.<property>" to the row's six counts in the
	// order present-equal, present-different, ours-missing, pilot-missing,
	// absent-both, indeterminate; the text report lists the same rows ranked.
	Scorecard map[string][6]int `json:"scorecard"`
	rows      []Row
}

// scorecardKey names a row in the baseline.
func scorecardKey(row Row) string { return row.Metaclass + "." + row.Property }

func (r *Report) setScorecard(rows []Row) {
	r.rows = rows
	r.Scorecard = make(map[string][6]int, len(rows))
	for _, row := range rows {
		r.Scorecard[scorecardKey(row)] = [6]int{row.PresentEqual, row.PresentDifferent, row.OursMissing, row.PilotMissing, row.AbsentBoth, row.Indeterminate}
	}
}

// Totals sums every root.
type Totals struct {
	Files                int           `json:"files"`
	OursRefused          int           `json:"oursRefused"`
	OursUnconvertible    int           `json:"oursUnconvertible"`
	OursColliding        int           `json:"oursColliding"`
	PilotFailed          int           `json:"pilotFailed"`
	PilotLibraryElements int           `json:"pilotLibraryElements"`
	Elements             ElementTotals `json:"elements"`
	Properties           Buckets       `json:"properties"`
	Rows                 int           `json:"rows"`
	UnkeyedReferences    int           `json:"unkeyedReferences"`
	ExtensionProperties  int           `json:"extensionProperties"`
	DuplicateElements    Sides         `json:"duplicateElements"`
	AmbiguousKeys        Sides         `json:"ambiguousKeys"`
	UnkeyableElements    Sides         `json:"unkeyableElements"`
}

// Sides holds one count per side.
type Sides struct {
	Ours  int `json:"ours"`
	Pilot int `json:"pilot"`
}

// RootReport is one corpus root's share of the totals.
type RootReport struct {
	Name        string `json:"name"`
	Dir         string `json:"dir"`
	Files       int    `json:"files"`
	OursRefused int    `json:"oursRefused"`
	PilotFailed int    `json:"pilotFailed"`
	// OursSeparate reports that the root did not convert as one model, so each
	// document was converted alone and cross-file references stay unresolved.
	OursSeparate         bool          `json:"oursSeparate,omitempty"`
	OursUnconvertible    int           `json:"oursUnconvertible,omitempty"`
	OursColliding        int           `json:"oursColliding,omitempty"`
	PilotLibraryElements int           `json:"pilotLibraryElements"`
	Elements             ElementTotals `json:"elements"`
	Properties           Buckets       `json:"properties"`
	DuplicateElements    Sides         `json:"duplicateElements"`
	AmbiguousKeys        Sides         `json:"ambiguousKeys"`
	UnkeyableElements    Sides         `json:"unkeyableElements"`
}

// Examples is the companion file: one concrete comparison per differing row.
type Examples struct {
	Examples  []Example   `json:"examples"`
	Unmatched []Unmatched `json:"unmatched"`
	// Unkeyed counts, per side and metaclass, the elements no key addressed.
	Unkeyed map[string]map[string]int `json:"unkeyed"`
	// Shared names, per root and side, the first keys two elements computed.
	Shared map[string]map[string][]string `json:"shared"`
}

// writeReports writes the report, its examples and a text rendering under out
// and returns the report's bytes for the baseline comparison.
func writeReports(out string, report *Report, examples Examples, logs map[string]string, w io.Writer) ([]byte, error) {
	files, err := reports.Open(out, toolName)
	if err != nil {
		return nil, err
	}
	fresh, err := files.JSON(report)
	if err != nil {
		return nil, err
	}
	if err := files.Text(renderText(report, examples, logs)); err != nil {
		return nil, err
	}
	exampleFiles, err := reports.Open(out, toolName+"-examples")
	if err != nil {
		return nil, err
	}
	if _, err := exampleFiles.JSON(examples); err != nil {
		return nil, err
	}
	files.Announce(w, headline(report))
	fmt.Fprintf(w, "  %s\n", exampleFiles.Path("json"))
	return fresh, nil
}

func headline(report *Report) string {
	t := report.Totals
	return fmt.Sprintf("%d files, %d elements matched (%d ours-only, %d pilot-only of which %d implied); properties: %d present-equal, %d present-different, %d ours-missing, %d pilot-missing, %d absent-both, %d indeterminate",
		t.Files, t.Elements.Matched, t.Elements.OursOnly, t.Elements.PilotOnly, t.Elements.PilotOnlyImplied,
		t.Properties.PresentEqual, t.Properties.PresentDifferent, t.Properties.OursMissing, t.Properties.PilotMissing, t.Properties.AbsentBoth, t.Properties.Indeterminate)
}

// renderText is the human-readable report: totals, roots, the scorecard with
// one example per differing row, and the pilot's diagnostics.
func renderText(report *Report, examples Examples, logs map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: derived-property differential against pilot %s (%s)\n", toolName, report.Provenance.PilotTag, report.Provenance.PilotArtifact)
	fmt.Fprintf(&b, "exporter: %s\n\n", report.Exporter)
	fmt.Fprintf(&b, "%s\n", headline(report))
	t := report.Totals
	fmt.Fprintf(&b, "ours refused %d files, could not convert %d alone and converted %d alone for declaring an element another file declares; pilot failed %d (those files left out of ours) and wrote %d library elements; type mismatches %d; unkeyed references %d; extension properties %d; duplicate copies ours %d pilot %d; ambiguous keys ours %d pilot %d; unkeyable elements ours %d pilot %d\n\n",
		t.OursRefused, t.OursUnconvertible, t.OursColliding, t.PilotFailed, t.PilotLibraryElements, t.Elements.TypeMismatch,
		t.UnkeyedReferences, t.ExtensionProperties, t.DuplicateElements.Ours, t.DuplicateElements.Pilot,
		t.AmbiguousKeys.Ours, t.AmbiguousKeys.Pilot, t.UnkeyableElements.Ours, t.UnkeyableElements.Pilot)
	b.WriteString("roots:\n")
	for _, root := range report.Roots {
		var notes []string
		if root.OursColliding > 0 {
			notes = append(notes, fmt.Sprintf("%d colliding documents converted alone", root.OursColliding))
		}
		if root.OursSeparate {
			notes = append(notes, "ours converted per document")
		}
		if root.OursUnconvertible > 0 {
			notes = append(notes, fmt.Sprintf("%d unconvertible", root.OursUnconvertible))
		}
		note := ""
		if len(notes) > 0 {
			note = "  (" + strings.Join(notes, ", ") + ")"
		}
		fmt.Fprintf(&b, "  %-18s %4d files  matched %6d  ours-only %5d  pilot-only %5d  equal %7d  different %6d  ours-missing %7d  pilot-missing %5d%s\n",
			root.Name, root.Files, root.Elements.Matched, root.Elements.OursOnly, root.Elements.PilotOnly,
			root.Properties.PresentEqual, root.Properties.PresentDifferent, root.Properties.OursMissing, root.Properties.PilotMissing, note)
	}
	byRow := make(map[string]Example, len(examples.Examples))
	for _, example := range examples.Examples {
		byRow[example.Metaclass+"\x00"+example.Property] = example
	}
	fmt.Fprintf(&b, "\nscorecard (%d rows, most differing first):\n", len(report.rows))
	fmt.Fprintf(&b, "  %-32s %-28s %7s %9s %12s %13s %11s %13s\n", "metaclass", "property", "equal", "different", "ours-missing", "pilot-missing", "absent-both", "indeterminate")
	for _, row := range report.rows {
		fmt.Fprintf(&b, "  %-32s %-28s %7d %9d %12d %13d %11d %13d\n", row.Metaclass, row.Property,
			row.PresentEqual, row.PresentDifferent, row.OursMissing, row.PilotMissing, row.AbsentBoth, row.Indeterminate)
		if example, ok := byRow[row.Metaclass+"\x00"+row.Property]; ok {
			fmt.Fprintf(&b, "      e.g. %s %s/%s (%s): ours %s, pilot %s\n", example.Bucket, example.Root, example.File, example.Element, example.Ours, example.Pilot)
		}
	}
	for _, side := range []string{"ours", "pilot"} {
		if counts := examples.Unkeyed[side]; len(counts) > 0 {
			fmt.Fprintf(&b, "\nunkeyable %s elements by metaclass:", side)
			for _, typ := range sortedKeys(counts) {
				fmt.Fprintf(&b, " %s %d", typ, counts[typ])
			}
			b.WriteString("\n")
		}
	}
	for _, root := range sortedKeys(examples.Shared) {
		for _, side := range sortedKeys(examples.Shared[root]) {
			if keys := examples.Shared[root][side]; len(keys) > 0 {
				fmt.Fprintf(&b, "\nambiguous %s keys in %s, e.g.: %s\n", side, root, strings.Join(keys, "; "))
			}
		}
	}
	fmt.Fprintf(&b, "\nunmatched elements (%d metaclasses):\n", len(examples.Unmatched))
	for _, entry := range examples.Unmatched {
		fmt.Fprintf(&b, "  %-5s %-32s %6d  e.g. %s\n", entry.Side, entry.Metaclass, entry.Count, strings.Join(entry.Elements, "; "))
	}
	for _, name := range sortedKeys(logs) {
		if logs[name] != "" {
			fmt.Fprintf(&b, "\npilot diagnostics (%s):\n%s\n", name, logs[name])
		}
	}
	return b.String()
}
