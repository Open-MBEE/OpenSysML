package export

import "testing"

// A named argument of a library function is an argument in the symbol structure as in the graph:
// the parameter it names is found among the function's inputs (KerML 8.3.4.8.5 InvocationExpression).
func TestNamedArgumentOfALibraryFunctionIsAnArgumentInBothStructures(t *testing.T) {
	diff, ok := differentialOf("p.sysml", []byte(`package P {
		attribute n = SequenceFunctions::size(seq = (1, 2));
	}`))
	if !ok {
		t.Fatal("the model has no differential")
	}
	counts := diff.counts["InstantiationExpression::argument"]
	if counts == nil || counts.equal == 0 {
		t.Fatal("InstantiationExpression::argument was not compared")
	}
	if counts.different+counts.missing+counts.extra > 0 {
		t.Errorf("InstantiationExpression::argument: %+v, samples %v", *counts, diff.samples["InstantiationExpression::argument"])
	}
}
