package queryexec

import (
	"slices"
	"testing"
)

// A connector's Relationship::source and target answer on the query leg although the connector inherits
// the library's valueless `source` and `target` ends (Links::BinaryLink, Allocations::Allocation).
func TestConnectorSourceAndTargetAnswerPastInheritedEnds(t *testing.T) {
	fixture := loadExecutionFixture(t, `package P {
	part def A { part x : A; }
	part def B;
	part a : A;
	part b : B;
	connection d connect a to b;
	allocation def F2P { end f : A; end p : B; }
	allocation al : F2P allocate a to b.x;
}
calc def Q :> Query {
	in root : Element;
	Project(source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "KerML::Kernel::Connector"), properties = ("source", "target"))
}`)
	result, err := fixture.execute(t, "Q", Bindings{"root": {ElementValue(fixture.symbol(t, "P"))}}, Options{})
	if err != nil {
		t.Fatalf("execute Q: %v", err)
	}
	if got := cellElementNames(t, result, "source"); !slices.EqualFunc(got, [][]string{{"Observatory::P::a"}, {"Observatory::P::a"}}, slices.Equal) {
		t.Fatalf("source cells = %q, want P::a for the connection and the allocation", got)
	}
	if got := cellElementNames(t, result, "target"); len(got) != 2 || !slices.Equal(got[0], []string{"Observatory::P::b"}) {
		t.Fatalf("target cells = %q, want P::b for the connection", got)
	}
}
