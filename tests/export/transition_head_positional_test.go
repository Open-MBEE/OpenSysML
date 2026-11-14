package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// A state body's `then x;` after a state and a decision's branch are TransitionUsages whose source is the member
// before them (SysML.xtext TargetTransitionUsage): each owns, ahead of its guard, the Membership naming that
// source and an `in` link parameter through a ParameterMembership, as a `transition first a then b` does.
const positionalTransitionHeadModel = `package P {
    attribute g : ScalarValues::Boolean;
    state def S {
        state idle;
        then busy;
        state busy;
        if g then idle;
        transition back first busy if g then idle;
    }
    action def A {
        action a1;
        decide d;
        if g then a2;
        else a3;
        action a2;
        action a3;
    }
}
`

func TestPositionalTransitionOwnsItsSourceMembershipAndLinkParameter(t *testing.T) {
	turtle, back := graphOnlyRoundTrip(t, "p.sysml", []byte(positionalTransitionHeadModel))
	graph := string(turtle)
	for _, want := range []string{
		// The state body's `then busy;` after `state idle;`: its source Membership names idle.
		"sysml:ownedRelationship expr:P__S___401_psourcemember, elmt:P__S___401_linkparam_om, elmt:P__S___401_succession_om ;",
		"expr:P__S___401_psourcemember\n    a sysml:Membership ;",
		"sysml:memberElement elmt:P__S__idle ;",
		"elmt:P__S___401_linkparam\n    a sysml:ReferenceUsage ;",
		"elmt:P__S___401_linkparam_om\n    a sysml:ParameterMembership ;",
		// The guarded `if g then idle;` after `state busy;` leaves busy: the same head, then the guard.
		"sysml:ownedRelationship expr:P__S___403_psourcemember, elmt:P__S___403_linkparam_om, expr:P__S___403_pguard_om, elmt:P__S___403_succession_om ;",
		"sysml:memberElement elmt:P__S__busy ;",
		// The decision's branches leave the decision.
		"sysml:ownedRelationship expr:P__A___402_psourcemember, elmt:P__A___402_linkparam_om, expr:P__A___402_pguard_om, elmt:P__A___402_succession_om ;",
		"sysml:ownedRelationship expr:P__A___403_psourcemember, elmt:P__A___403_linkparam_om, elmt:P__A___403_succession_om ;",
		"sysml:memberElement elmt:P__A__d ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should state %q:\n%s", want, graph)
		}
	}
	// Every link parameter is a referential usage an action owns: it varies, with the transition's own `first` form.
	for _, id := range []string{"P__S___401", "P__S___403", "P__S__back", "P__A___402", "P__A___403"} {
		block := blockOf(graph, "elmt:"+id+"_linkparam\n")
		for _, want := range []string{`sysml:direction "in" ;`, `sysml:isVariable "true"^^xsd:boolean ;`, `sysml:mayTimeVary "true"^^xsd:boolean ;`} {
			if !strings.Contains(block, want) {
				t.Errorf("%s_linkparam should state %s:\n%s", id, want, block)
			}
		}
	}
	// The head members are implied: the notation reads back as written.
	if got, want := strings.TrimSpace(string(back)), strings.TrimSpace(positionalTransitionHeadModel); got != want {
		t.Errorf("round trip:\n%s\nwant:\n%s", got, want)
	}
	// Written with the keyword, `transition if g then idle;` after a state owns the same head.
	keyword := strings.Replace(positionalTransitionHeadModel, "        if g then idle;", "        transition if g then idle;", 1)
	turtle, err := convert.Convert("p.sysml", []byte(keyword), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if want := "sysml:ownedRelationship expr:P__S___403_psourcemember, elmt:P__S___403_linkparam_om, expr:P__S___403_pguard_om, elmt:P__S___403_succession_om ;"; !strings.Contains(string(turtle), want) {
		t.Errorf("the graph should state %q:\n%s", want, turtle)
	}
}

// blockOf returns the Turtle statements of the subject that opens with head, up to its closing period.
func blockOf(graph, head string) string {
	start := strings.Index(graph, "\n"+head)
	if start < 0 {
		return ""
	}
	rest := graph[start+1:]
	if end := strings.Index(rest, " .\n"); end >= 0 {
		return rest[:end]
	}
	return rest
}
