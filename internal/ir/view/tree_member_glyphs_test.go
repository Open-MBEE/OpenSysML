package view

import (
	"reflect"
	"strings"
	"testing"
)

// A tree draws the membership of a timeslice, snapshot or event occurrence
// under its owner with the portion or «event» glyph, and that edge stands for
// the nesting: no containment line is drawn beside it, as none is beside a
// membership diamond.
func TestTreeMembershipGlyphsStandForTheNesting(t *testing.T) {
	rendering := renderSource(t, "Views::portions", `package Model {
	occurrence def Flight {
		timeslice cruise;
		snapshot touchdown;
		event occurrence takeoff;
		part crew;
	}
}
package Views {
	view portions { expose Model::Flight; }
}`)
	want := []string{
		"Model::Flight portion cruise",
		"Model::Flight portion touchdown",
		"Model::Flight event takeoff: event",
		"Model::Flight composition crew",
	}
	if got := treeEdgeLines(rendering); !reflect.DeepEqual(got, want) {
		t.Errorf("edges =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
	if nesting := rendering.nestingEdges(); len(nesting) != 4 {
		t.Errorf("nesting edges = %v, want the four memberships", nesting)
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	for _, want := range []string{`"n0" -> "n1" [dir=back, arrowtail=dot];`, `"n0" -> "n3" [label="event", arrowhead=vee];`} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
	if strings.Contains(dot, "arrowhead=none") {
		t.Errorf("DOT draws a containment line beside a portion or event glyph:\n%s", dot)
	}
}

// A typed portion or event occurrence listed in a compartment row folds its
// glyph away with its typing: the row's `name : Type` stands for it, and no
// edge from the owner depicts the type as the portion or the event.
func TestFoldedPortionAndEventRowsDrawNoGlyphToTheType(t *testing.T) {
	rendering := renderSource(t, "Views::folded", `package Model {
	occurrence def Phase;
	occurrence def Flight {
		snapshot landing : Phase;
		event occurrence takeoff : Phase;
	}
}
package Views {
	view folded { expose Model::Flight; expose Model::Phase; }
}`)
	want := []string{
		"Model::Flight portion landing",
		"Model::Flight event takeoff: event",
		"landing typing Model::Phase",
		"takeoff typing Model::Phase",
	}
	if got := treeEdgeLines(rendering); !reflect.DeepEqual(got, want) {
		t.Errorf("edges =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
	folded := foldCompartments(rendering.showing(AllShown()), CompartmentsMost)
	if got := treeEdgeLines(folded); len(got) != 0 {
		t.Errorf("folded edges = %q, want none: the rows `landing : Phase` and `takeoff : Phase` stand for the folded members", got)
	}
}
