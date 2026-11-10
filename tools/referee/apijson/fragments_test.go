package apijson

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
	"github.com/Open-MBEE/OpenSysML/tools/referee/baseline"
)

// committedFigures reads the committed directory and sums it the way the
// monolithic baseline once stated it: totals, roots and one scorecard.
func committedFigures(t *testing.T) *Report {
	t.Helper()
	root, err := repo.Choose("")
	if err != nil {
		t.Fatal(err)
	}
	committed, err := readCommitted(root)
	if err != nil {
		t.Fatal(err)
	}
	report := &Report{Provenance: committed.Provenance, Exporter: committed.Summary.Exporter, Normalizations: committed.Summary.Normalizations,
		PilotNondeterministic: committed.Summary.PilotNondeterministic, Totals: committed.Summary.Totals, Roots: committed.Summary.Roots}
	report.setScorecard(scorecardOf(committed.Fragments))
	return report
}

// The Java-free half of the ratchet: index.json is the fold of the committed
// fragments, each present, listed and filed under its own file, its buckets its rows' sum.
func TestCommittedIndexSumsItsFragments(t *testing.T) {
	root, err := repo.Choose("")
	if err != nil {
		t.Fatal(err)
	}
	if err := baseline.Verify(committedDir(root), fold); err != nil {
		t.Fatal(err)
	}
	committed, err := readCommitted(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range committed.Fragments {
		var sum Buckets
		for _, counts := range f.Rows {
			sum.addAll(counts.buckets())
		}
		if sum != f.Summary.Properties {
			t.Errorf("%s/%s: properties %+v, rows sum to %+v", f.Root, f.File, f.Summary.Properties, sum)
		}
	}
	if committed.Summary.Totals.Rows != len(scorecardOf(committed.Fragments)) {
		t.Errorf("index states %d rows, the fragments hold %d", committed.Summary.Totals.Rows, len(scorecardOf(committed.Fragments)))
	}
}

// One root, one file, without Java: the fragment carries the whole comparison,
// so the fold and the direct sums agree.
func TestProbeFragmentCarriesTheWholeComparison(t *testing.T) {
	ours, document := probeSides(t)
	pilot := &pilotExport{Unkeyed: make(map[string]int)}
	var duplicates, ambiguous map[string]int
	pilot.Elements, duplicates, ambiguous, _ = dropAmbiguous(keyPilot(document, pilot))
	if len(duplicates)+len(ambiguous) != 0 {
		t.Fatalf("pilot probe: duplicates %v, ambiguous %v", duplicates, ambiguous)
	}
	c := newComparison()
	elements, buckets := c.compareRoot("probe", ours.Elements, pilot.Elements)
	fragments, err := fragmentsOf("probe", []string{probeFile}, ours, pilot, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(fragments) != 1 || fragments[0].Root != "probe" || fragments[0].File != probeFile {
		t.Fatalf("fragments %+v", fragments)
	}
	f := fragments[0]
	if f.Summary.Elements != elements || f.Summary.Properties != buckets {
		t.Errorf("fragment summary %+v, the comparison counted %+v %+v", f.Summary, elements, buckets)
	}
	if f.Summary.UnkeyedReferences != c.unkeyedReferences || f.Summary.ExtensionProperties != c.extensions {
		t.Errorf("fragment counts %d unkeyed references and %d extensions, the comparison %d and %d", f.Summary.UnkeyedReferences, f.Summary.ExtensionProperties, c.unkeyedReferences, c.extensions)
	}
	if f.Summary.Elements.Matched == 0 || len(f.Rows) == 0 {
		t.Fatalf("the probe matched nothing: %+v", f.Summary)
	}
	rows := c.scorecard()
	if folded := scorecardOf(fragments); !reflect.DeepEqual(folded, rows) {
		t.Errorf("scorecard folded from the fragment:\n%+v\nsummed directly:\n%+v", folded, rows)
	}
	report := &Report{Totals: Totals{Files: 1, PilotLibraryElements: pilot.Library, Elements: elements, Properties: buckets, Rows: len(rows), UnkeyedReferences: c.unkeyedReferences, ExtensionProperties: c.extensions},
		Roots: []RootReport{{Name: "probe", Dir: rootDir("probe"), Files: 1, PilotLibraryElements: pilot.Library, Elements: elements, Properties: buckets}}}
	report.setScorecard(rows)
	if err := sameFigures(report, fragments); err != nil {
		t.Error(err)
	}
	report.Totals.Elements.Matched++
	if err := sameFigures(report, fragments); err == nil || !strings.Contains(err.Error(), "totals") {
		t.Errorf("a figure the fragments do not carry passed: %v", err)
	}
}

// An account filed under a file the root does not hold would leave the
// fragments and still reach the totals; the assembly refuses it.
func TestFragmentsRefuseAnAccountOutsideTheRootsFiles(t *testing.T) {
	ours, document := probeSides(t)
	pilot := &pilotExport{Unkeyed: make(map[string]int)}
	pilot.Elements, _, _, _ = dropAmbiguous(keyPilot(document, pilot))
	c := newComparison()
	c.compareRoot("probe", ours.Elements, pilot.Elements)
	_, err := fragmentsOf("probe", []string{"other.sysml"}, ours, pilot, c)
	if err == nil || !strings.Contains(err.Error(), probeFile) || !strings.Contains(err.Error(), "not a file of the root") {
		t.Fatalf("want a refusal naming %s, got %v", probeFile, err)
	}
	pilot.file("elsewhere.sysml").PilotFailed = true
	_, err = fragmentsOf("probe", []string{probeFile}, ours, pilot, c)
	if err == nil || !strings.Contains(err.Error(), "elsewhere.sysml") {
		t.Fatalf("want a refusal naming the pilot's stray file, got %v", err)
	}
}
