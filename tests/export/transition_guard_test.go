package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// guardModel writes a guard on a state transition, on a triggered one, and on
// a guarded succession, which is a transition too (SysML-textual-bnf
// GuardedSuccession : TransitionUsage).
const guardModel = `package G {
    attribute def Sig;
    state def S {
        entry; then s1;
        state s1;
        state s2;
        transition t1 first s1 if true then s2;
        transition t2 first s2 accept Sig if 1 > 0 then s1;
    }
    action def A {
        action a;
        action b;
        succession g1 first a if true then b;
    }
}
`

// A transition's guard is its TransitionUsage::guardExpression, an Expression
// owned through a TransitionFeatureMembership of kind guard (SysML-textual-bnf
// GuardExpressionMember: 'if' { kind = 'guard' } ownedRelatedElement +=
// OwnedExpression), as its trigger and effect are owned through ones of kind
// trigger and effect.
func TestTransitionGuardIsMetamodelStructure(t *testing.T) {
	turtle, err := convert.Convert("g.sysml", []byte(guardModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	g, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"G__S__t1", "G__S__t2", "G__A__g1"} {
		transition := element(name)
		if got := g.Type(transition); got != rdf.SysML+"TransitionUsage" {
			t.Fatalf("%s is a %s", name, got)
		}
		guard, ok := g.Object(transition, rdf.SysML+"guardExpression")
		if !ok {
			t.Fatalf("%s states no sysml:guardExpression", name)
		}
		if stated, _ := g.Object(transition, rdf.OpenSysML+"guard"); stated != guard {
			t.Errorf("%s's sysx:guard names %v, its guardExpression %v", name, stated, guard)
		}
		membership, ok := g.Object(guard, rdf.SysML+"owningMembership")
		if !ok || g.Type(membership) != rdf.SysML+"TransitionFeatureMembership" {
			t.Fatalf("%s's guard is not owned by a TransitionFeatureMembership", name)
		}
		if kind, _ := g.Lexical(membership, rdf.SysML+"kind"); kind != "guard" {
			t.Errorf("%s's guard membership has kind %q", name, kind)
		}
		if feature, _ := g.Object(membership, rdf.SysML+"transitionFeature"); feature != guard {
			t.Errorf("%s's guard membership names %v as its transition feature", name, feature)
		}
		if owned := g.Objects(transition, rdf.SysML+"ownedFeatureMembership"); !containsTerm(owned, membership) {
			t.Errorf("%s does not list its guard membership as an owned feature membership", name)
		}
	}

	// The graph alone reads back to the guards as written.
	back, err := convert.Convert("g.ttl", withoutTriples(t, turtle, "sysx:sourceText"), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	for _, want := range []string{"if true then s2;", "accept Sig if 1 > 0 then s1;", "if true then b;"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the notation read back lacks %q:\n%s", want, back)
		}
	}
}

func containsTerm(terms []rdf.Term, want rdf.Term) bool {
	for _, term := range terms {
		if term == want {
			return true
		}
	}
	return false
}
