package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A library usage's implied generals — the base definition its kind implies and
// the base usage it subsets — are recorded with its facts, so every model reads
// them from the record instead of deriving them again over the library.
func TestLibraryImpliedGeneralsAreRecordedWithTheDefinition(t *testing.T) {
	ws := NewWorkspace()
	m := semantics.NewModel(resolve.New(ws.index))
	var recorded, withDefinition int
	var sample *symbols.Symbol
	for _, fqn := range ws.index.FQNs() {
		for _, sym := range ws.index.LookupQualified(fqn) {
			if sym.Facts == nil || sym.Facts.Supers == nil || len(sym.Facts.ImpliedSupers) == 0 || !sym.IsFeature() {
				continue
			}
			recorded++
			generals := m.ImplicitGenerals(sym)
			if len(generals) != len(sym.Facts.ImpliedSupers) {
				t.Fatalf("%s: ImplicitGenerals has %d entries, its record %d", fqn, len(generals), len(sym.Facts.ImpliedSupers))
			}
			for _, general := range generals {
				if !general.IsFeature() {
					withDefinition++
					if sample == nil {
						sample = sym
					}
					break
				}
			}
		}
	}
	if recorded == 0 || withDefinition == 0 {
		t.Fatalf("library usages with recorded implied generals: %d, of them with an implied definition: %d", recorded, withDefinition)
	}
	if allocs := testing.AllocsPerRun(10, func() { m.ImplicitGenerals(sample) }); allocs != 0 {
		t.Errorf("a repeated ImplicitGenerals query for %s allocates %v times, want 0", sample.Name, allocs)
	}
	fresh := semantics.NewModel(resolve.New(ws.index))
	if got, want := fresh.ImplicitGenerals(sample), m.ImplicitGenerals(sample); len(got) != len(want) {
		t.Errorf("a fresh model answers %d implied generals for %s, the first %d", len(got), sample.Name, len(want))
	}
}
