package resolve_test

import (
	"strings"
	"testing"
)

const loopVariableDoc = `package Repro {
	part def Component {
		action run;
		attribute mass : ScalarValues::Real;
	}
	part def Assembly {
		part components : Component [1..*];
		attribute total : ScalarValues::Real;
		action runAll {
			for component : Component in components {
				perform component.run;
			}
		}
		action sum for c : Component in components {
			assign total := total + c.mass;
		}
	}
}`

// A `for` variable is typed by what its declaration names, so a chain through it
// reaches the members of that type, in a bare loop and in a named loop node alike.
func TestResolveMemberChainThroughTypedLoopVariable(t *testing.T) {
	for _, d := range resolvedWithModel(t, loopVariableDoc) {
		t.Errorf("unexpected diagnostic: %s", d)
	}
}

// The variable's type bounds the chain: a member the type does not hold stays unresolved.
func TestResolveUnknownMemberThroughTypedLoopVariable(t *testing.T) {
	got := resolvedWithModel(t, strings.Replace(loopVariableDoc, "perform component.run;", "perform component.missing;", 1))
	if len(got) != 1 || !strings.Contains(got[0], "unresolved member: missing") {
		t.Fatalf("diagnostics = %v, want one unresolved member: missing", got)
	}
}

// The variable stays local to its loop: the action holding the loop does not see it.
func TestResolveLoopVariableIsNotVisibleOutsideItsLoop(t *testing.T) {
	got := resolvedWithModel(t, strings.Replace(loopVariableDoc, "action runAll {", "action runAll {\n\t\t\tperform component.run;", 1))
	if len(got) == 0 {
		t.Fatal("a reference to the loop variable outside its loop resolved")
	}
}
