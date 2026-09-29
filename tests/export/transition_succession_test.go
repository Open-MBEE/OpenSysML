package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// A `transition t first a then b;` owns its succession as a SuccessionAsUsage
// through an OwningMembership (SysML.xtext TransitionSuccessionMember): the
// succession's first end is the empty source end, its second end a connector
// end whose ReferenceSubsetting names the target. The transition still states
// sysml:source/sysml:target itself, and the owned succession is implied — it is
// never written back as a member.
const transitionSuccessionModel = `package T {
    attribute g : Boolean;
    state def S {
        state a;
        state b;
        state c;
        transition t first a then b;
        transition u first b if g then c;
        succession v first a if g then c;
    }
}
`

func TestTransitionOwnsItsSuccession(t *testing.T) {
	turtle, back := graphOnlyRoundTrip(t, "t.sysml", []byte(transitionSuccessionModel))
	graph := string(turtle)
	for _, want := range []string{
		"sysml:succession elmt:T__S__t_succession ;",
		"sysml:source elmt:T__S__a ;",
		"sysml:target elmt:T__S__b ;",
		"elmt:T__S__t_succession\n    a sysml:SuccessionAsUsage ;",
		"elmt:T__S__t_succession_om\n    a sysml:OwningMembership ;",
		"expr:T__S__t_succession_pend0\n    a sysml:ReferenceUsage ;",
		"expr:T__S__t_succession_pend1\n    a sysml:ReferenceUsage ;",
		"expr:T__S__t_succession_pend0_om\n    a sysml:EndFeatureMembership ;",
		"expr:T__S__t_succession_pend1_om\n    a sysml:EndFeatureMembership ;",
		"expr:T__S__t_succession_pend1_prs\n    a sysml:ReferenceSubsetting ;",
		"sysml:isEnd \"true\"^^xsd:boolean ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should state %q:\n%s", want, graph)
		}
	}
	// Every transition owns its succession: the guarded one and the `succession`
	// member alike.
	for _, want := range []string{
		"sysml:succession elmt:T__S__u_succession ;",
		"sysml:succession elmt:T__S__v_succession ;",
		"elmt:T__S__v_succession\n    a sysml:SuccessionAsUsage ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should state %q:\n%s", want, graph)
		}
	}
	notation := string(back)
	for _, want := range []string{
		"transition t first a then b;\n",
		"transition u first b if g then c;\n",
		"succession v first a if g then c;\n",
	} {
		if !strings.Contains(notation, want) {
			t.Errorf("the notation should contain %q:\n%s", want, notation)
		}
	}
	// The implied succession is not a member: no `succession` line other than
	// the `succession v` member may appear, and nothing spells its ends.
	if strings.Count(notation, "succession") != 1 {
		t.Errorf("the owned succession came back as a member:\n%s", notation)
	}
}

// The grammar's member order puts the TransitionSuccessionMember after the
// guard and effect memberships and before the body's (SysML.xtext
// TransitionUsage): the ownedRelationship list states it in that order.
func TestTransitionSuccessionFollowsEffectAndPrecedesBody(t *testing.T) {
	src := `package T {
    attribute g : Boolean;
    state def S {
        state a;
        state b;
        state c;
        transition t first a if g do send a to b then c {
            state inner;
        }
    }
}
`
	turtle, back := graphOnlyRoundTrip(t, "t.sysml", []byte(src))
	graph := string(turtle)
	if !strings.Contains(graph,
		"sysml:ownedRelationship expr:T__S__t_pguard_om, elmt:T__S__t___400_om, elmt:T__S__t_succession_om, elmt:T__S__t__inner_om ;") {
		t.Errorf("the memberships should order guard, effect, succession, body:\n%s", graph)
	}
	notation := string(back)
	for _, want := range []string{"transition t first a if g do send a to b then c {\n", "state inner;\n"} {
		if !strings.Contains(notation, want) {
			t.Errorf("the notation should contain %q:\n%s", want, notation)
		}
	}
}

// A succession whose target disagrees with the transition's sysml:target is no
// restatement of the head: the conversion refuses it rather than dropping one.
func TestTransitionSuccessionDisagreementIsRefused(t *testing.T) {
	turtle, err := convert.Convert("t.sysml", []byte(transitionSuccessionModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph := string(turtle)
	block := "expr:T__S__t_succession_pend1_prs\n"
	start := strings.Index(graph, block)
	if start < 0 {
		t.Fatalf("the graph should state %s:\n%s", block, graph)
	}
	end := strings.Index(graph[start:], "\n\n")
	if end < 0 {
		t.Fatalf("unterminated block:\n%s", graph[start:])
	}
	mutated := graph[:start] +
		strings.ReplaceAll(graph[start:start+end], "elmt:T__S__b", "elmt:T__S__c") +
		graph[start+end:]
	if _, err := convert.Convert("m.ttl", []byte(mutated), convert.FormatTurtle, convert.FormatSysML); err == nil {
		t.Errorf("a succession targeting c under a transition targeting b should be refused")
	}
}
