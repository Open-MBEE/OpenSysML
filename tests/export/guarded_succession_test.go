package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// guardedSuccessionModel writes the guarded successions SysML.xtext types
// TransitionUsage: a keyword-less `first a if g then b` and a named
// `succession s first a if g then b` (GuardedSuccession) in an action body
// and in a state body, a decision's `if g then b` (GuardedTargetSuccession)
// and `else b` (DefaultTargetSuccession); the plain `then b` after a decision
// stays the SuccessionAsUsage TargetSuccession says it is.
const guardedSuccessionModel = `package Flow {
    private import ScalarValues::Integer;
    action def Route {
        action routeOrder;
        action processOrder;
        first routeOrder if true then processOrder;
        succession s first routeOrder if false then processOrder;
    }
    action def Charge {
        attribute level : Integer;
        action check;
        then decide;
            if level < 100 then addCharge;
            if level >= 100 then stop;
            else check;
        action addCharge;
        action stop;
    }
    state def Machine {
        state a;
        state b;
        first a if true then b;
        succession t first a if false then b;
    }
}
`

var guardedSuccessionLines = []string{
	"first routeOrder if true then processOrder;",
	"succession s first routeOrder if false then processOrder;",
	"then decide;",
	"if level < 100 then addCharge;",
	"if level >= 100 then stop;",
	"else check;",
	"first a if true then b;",
	"succession t first a if false then b;",
}

func guardedSuccessionGraph(t *testing.T) (string, *rdf.Graph) {
	t.Helper()
	turtle, err := convert.Convert("flow.sysml", []byte(guardedSuccessionModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	g, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatal(err)
	}
	return string(turtle), g
}

// Each guarded succession is a TransitionUsage whose guard is owned through a
// TransitionFeatureMembership of kind guard and which owns the Succession
// its `then` declares (SysML.xtext TransitionSuccessionMember); the unguarded
// target succession is a SuccessionAsUsage with no guard.
func TestGuardedSuccessionIsTransitionUsage(t *testing.T) {
	_, g := guardedSuccessionGraph(t)
	for _, name := range []string{
		"Flow__Route___402", "Flow__Route__s",
		"Flow__Charge___404", "Flow__Charge___405", "Flow__Charge___406",
		"Flow__Machine___402", "Flow__Machine__t",
	} {
		transition := element(name)
		if got := g.Type(transition); got != rdf.SysML+"TransitionUsage" {
			t.Errorf("%s is a %s, not a TransitionUsage", name, got)
			continue
		}
		if _, ok := g.Object(transition, rdf.SysML+"source"); !ok {
			t.Errorf("%s states no sysml:source", name)
		}
		if _, ok := g.Object(transition, rdf.SysML+"target"); !ok {
			t.Errorf("%s states no sysml:target", name)
		}
		succession, ok := g.Object(transition, rdf.SysML+"succession")
		if !ok {
			t.Errorf("%s owns no sysml:succession", name)
		} else {
			if got := g.Type(succession); got != rdf.SysML+"SuccessionAsUsage" {
				t.Errorf("%s's succession is a %s", name, got)
			}
			if owner, _ := g.Object(succession, rdf.SysML+"owner"); owner != transition {
				t.Errorf("%s's succession is owned by %v", name, owner)
			}
		}
		if name == "Flow__Charge___406" {
			// The default branch has no guard.
			if _, ok := g.Object(transition, rdf.SysML+"guardExpression"); ok {
				t.Errorf("the `else` branch %s states a guardExpression", name)
			}
			continue
		}
		guard, ok := g.Object(transition, rdf.SysML+"guardExpression")
		if !ok {
			t.Errorf("%s states no sysml:guardExpression", name)
			continue
		}
		membership, ok := g.Object(guard, rdf.SysML+"owningMembership")
		if !ok || g.Type(membership) != rdf.SysML+"TransitionFeatureMembership" {
			t.Errorf("%s's guard is not owned by a TransitionFeatureMembership", name)
			continue
		}
		if kind, _ := g.Lexical(membership, rdf.SysML+"kind"); kind != "guard" {
			t.Errorf("%s's guard membership has kind %q", name, kind)
		}
		if owned := g.Objects(transition, rdf.SysML+"ownedFeatureMembership"); !containsTerm(owned, membership) {
			t.Errorf("%s does not list its guard membership as an owned feature membership", name)
		}
	}
	plain := element("Flow__Charge___403")
	if got := g.Type(plain); got != rdf.SysML+"SuccessionAsUsage" {
		t.Errorf("the unguarded `then decide;` is a %s", got)
	}
	if _, ok := g.Object(plain, rdf.SysML+"guardExpression"); ok {
		t.Error("the unguarded `then decide;` states a guardExpression")
	}
	if got := g.Type(element("Flow__Charge___402")); got != rdf.SysML+"DecisionNode" {
		t.Errorf("`decide` is a %s", got)
	}
}

// The graph alone writes every guarded succession back as written: keyword-less
// where it was keyword-less, and never as `transition` in an action body.
func TestGuardedSuccessionComesBackFromTheGraphAlone(t *testing.T) {
	turtle, _ := guardedSuccessionGraph(t)
	back := backFromTheGraphAlone(t, turtle)
	for _, want := range guardedSuccessionLines {
		if !strings.Contains(back, want) {
			t.Errorf("the graph alone did not write %q\n--- notation ---\n%s", want, back)
		}
	}
	if strings.Contains(back, "transition") {
		t.Errorf("the graph alone wrote `transition` for a succession\n--- notation ---\n%s", back)
	}
}

// The API element form types the guarded successions TransitionUsage and
// reads back to the same notation with the source text dropped.
func TestGuardedSuccessionAPIJSON(t *testing.T) {
	_, g := guardedSuccessionGraph(t)
	document, err := export.WriteAPIJSON(g)
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{
		"Flow__Route___402":   "TransitionUsage",
		"Flow__Route__s":      "TransitionUsage",
		"Flow__Charge___403":  "SuccessionAsUsage",
		"Flow__Charge___404":  "TransitionUsage",
		"Flow__Charge___405":  "TransitionUsage",
		"Flow__Charge___406":  "TransitionUsage",
		"Flow__Machine___402": "TransitionUsage",
		"Flow__Machine__t":    "TransitionUsage",
	} {
		el := apiJSONElement(t, elements, id)
		if got := string(el["@type"]); got != `"`+want+`"` {
			t.Errorf("%s @type = %s, want %s", id, got, want)
		}
	}
	for i := range elements {
		delete(elements[i], "sysx:sourceText")
		delete(elements[i], "sysx:sourceTail")
	}
	stripped, err := json.Marshal(elements)
	if err != nil {
		t.Fatal(err)
	}
	back, err := convert.Convert("flow.json", stripped, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back from the element form alone: %v", err)
	}
	for _, want := range guardedSuccessionLines {
		if !strings.Contains(string(back), want) {
			t.Errorf("the element form alone did not write %q\n--- notation ---\n%s", want, back)
		}
	}
}
