package grpc

import (
	"slices"
	"testing"
)

// A transition target qualified with `::` names the state itself; only a `.` chains
// (SysML.xtext TransitionSuccession).
func TestElementPropertiesReadAQualifiedTransitionTarget(t *testing.T) {
	r := parseForElementProperties(t, `package P {
    state def S {
        state a;
        state b { state c; }
        transition qualified first a then S::b;
        transition chained first a then b.c;
    }
}`)
	if got := propertyElementIDs(r.property("P__S__qualified_succession", "target").Values); !slices.Equal(got, []string{"P__S__b"}) {
		t.Errorf("the qualified target is %v, want the state P__S__b", got)
	}
	if got := propertyElementIDs(r.property("P__S__chained_succession", "target").Values); !slices.Equal(got, []string{"P__S__chained_succession_pend1_pchain"}) {
		t.Errorf("the chained target is %v, want the chain the end owns", got)
	}
}
