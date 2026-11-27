package export

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

const repeatedHeadTargetSource = `package P {
    classifier A;
    feature g : A;
    classifier B specializes A, A;
    feature h : A, A;
    feature s subsets g, g;
    feature r redefines g, g;
    classifier U unions A, A;
    classifier I intersects A, A;
    classifier D differences A, A;
    classifier C disjoint from A, A;
    feature f featured by A, A;
    feature x references g, g;
    feature c crosses g, g;
    feature t : A, A, B;
    feature m subsets g, g specializes g;
}
`

// A head clause naming one target twice (`: A, A`) owns one relationship element per name, each
// with its own id (KerML 8.2.1: `ownedRelationship += …`); the collapsed property states it once.
func TestRepeatedHeadTargetsOwnDistinctRelationships(t *testing.T) {
	graph := graphOfNotation(t, repeatedHeadTargetSource)
	for _, c := range []struct {
		owner    string
		kind     ast.RelationshipKind
		target   string
		suffixes []string
	}{
		{"B", ast.RelSpecializes, "A", []string{"_sc0", "_sc1"}},
		{"h", ast.RelTyping, "A", []string{"_ft0", "_ft1"}},
		{"s", ast.RelSubsets, "g", []string{"_ss0", "_ss1"}},
		{"r", ast.RelRedefines, "g", []string{"_rd0", "_rd1"}},
		{"U", ast.RelUnions, "A", []string{"_un0", "_un1"}},
		{"I", ast.RelIntersects, "A", []string{"_is0", "_is1"}},
		{"D", ast.RelDifferences, "A", []string{"_df0", "_df1"}},
		{"C", ast.RelDisjoint, "A", []string{"_dj0", "_dj1"}},
		{"f", ast.RelFeaturedBy, "A", []string{"_tf0", "_tf1"}},
		{"x", ast.RelReferences, "g", []string{"_rs0", "_rs1"}},
		{"c", ast.RelCrosses, "g", []string{"_cs0", "_cs1"}},
	} {
		owner := rdf.IRI(rdf.Element + "P__" + c.owner)
		var want []string
		for _, suffix := range c.suffixes {
			want = append(want, owner.Value+suffix)
		}
		var got []string
		for _, owned := range graph.Objects(owner, rdf.SysML+pOwnedRelationship) {
			got = append(got, owned.Value)
			if ends := graph.Objects(owned, rdf.SysML+pTarget); len(ends) != 1 || ends[0].Value != rdf.Element+"P__"+c.target {
				t.Errorf("%s targets %v, want %s", owned.Value, ends, c.target)
			}
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s owns %v, want %v", c.owner, got, want)
		}
		if collapsed := graph.Objects(owner, rdf.SysML+relationshipProperty[c.kind]); len(collapsed) != 1 {
			t.Errorf("%s: the collapsed %s states %v, want %s once", c.owner, relationshipProperty[c.kind], collapsed, c.target)
		}
	}
	// The repeat follows the kind's distinct targets in the numbering, and stands where it was written.
	t3 := rdf.IRI(rdf.Element + "P__t")
	var owned []string
	for _, relationship := range graph.Objects(t3, rdf.SysML+pOwnedRelationship) {
		owned = append(owned, rdf.LocalName(relationship.Value))
	}
	if want := []string{"P__t_ft0", "P__t_ft2", "P__t_ft1"}; !slices.Equal(owned, want) {
		t.Errorf("t owns %v, want %v", owned, want)
	}
}

// The repeated names read back from the relationship elements alone, and writing that
// notation again states the same graph.
func TestRepeatedHeadTargetsReadBackFromTheGraphAlone(t *testing.T) {
	alone := withoutSourceLayout(graphOfNotation(t, repeatedHeadTargetSource))
	notation, err := ToSysML(alone)
	if err != nil {
		t.Fatalf("ToSysML: %v", err)
	}
	for _, head := range []string{
		"classifier B specializes A, A;", "feature h : A, A;", "feature s subsets g, g;",
		"feature r redefines g, g;", "classifier U unions A, A;", "classifier I intersects A, A;",
		"classifier D differences A, A;", "classifier C disjoint from A, A;", "feature f featured by A, A;",
		"feature x references g, g;", "feature c crosses g, g;", "feature t : A, A, B;",
		"feature m subsets g, g specializes g;",
	} {
		if !strings.Contains(string(notation), head) {
			t.Errorf("%q does not come back:\n%s", head, notation)
		}
	}
	again := withoutSourceLayout(graphOfNotation(t, string(notation)))
	if got, want := fmt.Sprint(again.Triples()), fmt.Sprint(alone.Triples()); got != want {
		t.Errorf("the notation read back states another graph:\n got %s\nwant %s", got, want)
	}
}

// The heads written without relationshipWords' clauses name a repeated target as often too.
func TestRepeatedShorthandTargetsReadBackFromTheGraphAlone(t *testing.T) {
	alone := withoutSourceLayout(graphOfNotation(t, `package P {
    part def A;
    action b;
    part value : A;
    calc def G { return :> value, value; }
    action def H { perform a :>> b, b; }
    calc def F { in xs : A[*]; xs->collect { in a : A, A; a } }
}
`))
	notation, err := ToSysML(alone)
	if err != nil {
		t.Fatalf("ToSysML: %v", err)
	}
	for _, head := range []string{"return :> value, value;", "perform a :>> b, b;", "in a : A, A;"} {
		if !strings.Contains(string(notation), head) {
			t.Errorf("%q does not come back:\n%s", head, notation)
		}
	}
	again := withoutSourceLayout(graphOfNotation(t, string(notation)))
	if got, want := fmt.Sprint(again.Triples()), fmt.Sprint(alone.Triples()); got != want {
		t.Errorf("the notation read back states another graph:\n got %s\nwant %s", got, want)
	}
}
