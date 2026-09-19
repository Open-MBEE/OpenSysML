package view

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

// A rendering names an element the way the notation declares it: several
// spellings share one kind, and a KerML classifier takes no `def`.
func TestRenderingsNameElementsAsWritten(t *testing.T) {
	_, idx := loadFixture(t, "notation.sysml")
	want := map[string]string{
		"Notation::M":           "metaclass",
		"Notation::T":           "datatype",
		"Notation::f":           "feature",
		"Notation::C":           "class",
		"Notation::BD":          "behavior def",
		"Notation::Wheel":       "part def",
		"Notation::my wheel":    "part",
		"Notation::Run::moving": "assert constraint",
		"Notation::Run::drive":  "perform action",
	}
	for fqn, kind := range want {
		if got := declKind(lookup(t, idx, fqn)); got != kind {
			t.Errorf("%s renders as %q, want %q", fqn, got, kind)
		}
	}
}

// A name holding `::` is one name, quoted whole, in every place a rendering
// writes one: a root's qualified name, a nested node's own name, a connector's
// label, a state and a table row.
func TestNamesHoldingTheSeparatorAreQuotedWhole(t *testing.T) {
	tree := render(t, "separators.sysml", "SepViews::partView")
	if names := nodeNames(tree.Roots); !names["'Sep::Pkg'::'x::y'"] || !names["'fuel::out'"] || names["out"] {
		t.Errorf("tree node names %v, want 'Sep::Pkg'::'x::y' and 'fuel::out', not out", slices.Sorted(maps.Keys(names)))
	}
	if got := edgeLabels(render(t, "separators.sysml", "SepViews::lineView")); !got["'fuel::line'"] {
		t.Errorf("connection labels %v lack 'fuel::line'", slices.Sorted(maps.Keys(got)))
	}
	if names := nodeNames(render(t, "separators.sysml", "SepViews::modeView").Roots); !names["'idle::state'"] {
		t.Errorf("state node names %v lack 'idle::state'", slices.Sorted(maps.Keys(names)))
	}
	table := render(t, "separators.sysml", "SepViews::tableView")
	if got := table.Text(); !strings.Contains(got, "'fuel::out'") || strings.Contains(got, "| 'out'") {
		t.Errorf("table names the port wrong:\n%s", got)
	}
}
