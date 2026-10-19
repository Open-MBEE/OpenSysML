package runtime

import "testing"

// A tracked Kernel collection unions the object with what every classifier's shape
// subsetters hold: a port a later classifier adds is in `timeEnclosedOccurrences`.
func TestKernelCollectionUnionAcrossClassifiers(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package C {
		port def Q;
		part def Bare;
		part def WithPort :> Bare { port q : Q; }
		part b : Bare;
		part w : WithPort = b;
	}`)
	root := idx.DocumentRoot("<test>")
	ports := mustEvalIn(t, ctx, root, "C::w.ownedPorts")
	enclosed := mustEvalIn(t, ctx, root, "C::w.timeEnclosedOccurrences")
	if got, want := len(elementsOf(enclosed)), 1+len(elementsOf(ports)); got != want || len(elementsOf(ports)) != 1 {
		t.Fatalf("w.timeEnclosedOccurrences = %s with ownedPorts %s, want the object and its port", FormatValue(enclosed), FormatValue(ports))
	}
	self := mustEvalIn(t, ctx, root, "C::w.timeEnclosedOccurrences#(1) == C::w")
	if FormatValue(self) != "true" {
		t.Errorf("w.timeEnclosedOccurrences#(1) == w = %s, want true", FormatValue(self))
	}
}

// A model-level read through a subsetter of open count (a port usage declared `[1..*]`)
// is open, yet the object leads it: position one is fixed and the port is known.
func TestKernelCollectionUnionOpenKeepsReceiverPosition(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package P {
		port def Q;
		part def Hull { port q1 : Q; port qs : Q[1..*]; }
		part hull : Hull;
	}`)
	root := idx.DocumentRoot("<test>")
	enclosed := mustEvalIn(t, ctx, root, "P::hull.timeEnclosedOccurrences")
	u := enclosed.Undetermined()
	if u == nil || len(u.knownValues()) < 2 || fewestOf(u.Count()) < 2 {
		t.Fatalf("hull.timeEnclosedOccurrences = %s, want undetermined of at least two with the part and q1 known", FormatValue(enclosed))
	}
	first := mustEvalIn(t, ctx, root, "P::hull.timeEnclosedOccurrences#(1) == P::hull")
	if FormatValue(first) != "true" {
		t.Errorf("hull.timeEnclosedOccurrences#(1) == hull = %s, want true", FormatValue(first))
	}
	second := mustEvalIn(t, ctx, root, "P::hull.timeEnclosedOccurrences#(2) == P::hull.q1")
	if FormatValue(second) != "true" {
		t.Errorf("hull.timeEnclosedOccurrences#(2) == q1 = %s, want true", FormatValue(second))
	}
	// The port's own read leads with the port, open through `subports [0..*]` or closed.
	port := mustEvalIn(t, ctx, root, "P::hull.q1.timeEnclosedOccurrences")
	if u := port.Undetermined(); u != nil && (len(u.knownValues()) != 1 || fewestOf(u.Count()) < 1) {
		t.Errorf("hull.q1.timeEnclosedOccurrences = %s, want the port known and a count of at least one", FormatValue(port))
	} else if u == nil && FormatValue(mustEvalIn(t, ctx, root, "P::hull.q1.timeEnclosedOccurrences->SequenceFunctions::size()")) != "1" {
		t.Errorf("hull.q1.timeEnclosedOccurrences = %s, want the port alone", FormatValue(port))
	}
	portFirst := mustEvalIn(t, ctx, root, "P::hull.q1.timeEnclosedOccurrences#(1) == P::hull.q1")
	if FormatValue(portFirst) != "true" {
		t.Errorf("hull.q1.timeEnclosedOccurrences#(1) == q1 = %s, want true", FormatValue(portFirst))
	}
}
