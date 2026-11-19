package program

import "testing"

func TestVertexRefNamesOneTable(t *testing.T) {
	state := StateVertex(3)
	if state.State() != 3 || state.Pseudo() != 0 || state.Terminate() != 0 || state.IsZero() {
		t.Fatalf("StateVertex(3) = %+v; want a state reference to 3 and nothing else", state)
	}
	pseudo := PseudoVertex(2)
	if pseudo.Pseudo() != 2 || pseudo.State() != 0 || pseudo == state {
		t.Fatalf("PseudoVertex(2) = %+v; want a pseudostate reference distinct from a state's", pseudo)
	}
	stop := TerminateVertex(1)
	if stop.Terminate() != 1 || stop.State() != 0 || stop.Pseudo() != 0 {
		t.Fatalf("TerminateVertex(1) = %+v; want a terminate reference", stop)
	}
	if StateVertex(2) == PseudoVertex(2) {
		t.Fatal("a state and a pseudostate at the same index compare equal; want the kind to tell them apart")
	}
}

func TestVertexRefZeroNamesNothing(t *testing.T) {
	for _, ref := range []VertexRef{{}, StateVertex(0), PseudoVertex(0), TerminateVertex(0)} {
		if !ref.IsZero() || ref.Kind != VertexNone {
			t.Errorf("%+v is not the zero reference; want id 0 of every kind to name no vertex", ref)
		}
	}
	for kind, want := range map[VertexKind]string{VertexNone: "none", VertexState: "state", VertexPseudostate: "pseudostate", VertexTerminate: "terminate"} {
		if got := kind.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", kind, got, want)
		}
	}
}
