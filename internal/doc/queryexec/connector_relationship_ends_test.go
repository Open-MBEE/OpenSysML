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

// A member an element inherits with a stated value — `= null`, `= ()` — keeps answering that value
// past a metaclass property of its name; only a valueless inherited member yields to the metaclass.
func TestInheritedStatedEmptyValueKeepsShadowingMetaclassProperty(t *testing.T) {
	fixture := loadExecutionFixture(t, `package P {
    part def A { attribute isComposite = null; }
    part def B { attribute isComposite = (); }
    part a : A;
    part b : B;
}
calc def Q :> Query {
    in root : Element;
    Project(source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "SysML::Systems::PartUsage"), properties = ("isComposite"))
}`)
	result, err := fixture.execute(t, "Q", Bindings{"root": {ElementValue(fixture.symbol(t, "P"))}}, Options{})
	if err != nil {
		t.Fatalf("execute Q: %v", err)
	}
	if got := cellElementNames(t, result, "isComposite"); !slices.EqualFunc(got, [][]string{nil, nil}, slices.Equal) {
		t.Fatalf("isComposite cells = %q, want the inherited empty value on a and b, not the metaclass flag", got)
	}
}

// A member path ending in a connector's inherited `source` end reads the metaclass property
// where the end states no value, as the property read on the connector's own row does.
func TestMemberPathReadsConnectorSourcePastInheritedEnd(t *testing.T) {
	fixture := loadExecutionFixture(t, `package P {
    part def A;
    part def B;
    part sys {
        part a : A;
        part b : B;
        connection d connect a to b;
    }
}
calc def Q :> Query {
    in root : Element;
    Project(source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "SysML::Systems::PartUsage"), properties = ("d.source"))
}`)
	result, err := fixture.execute(t, "Q", Bindings{"root": {ElementValue(fixture.symbol(t, "P"))}}, Options{})
	if err != nil {
		t.Fatalf("execute Q: %v", err)
	}
	if got := cellElementNames(t, result, "d.source"); !slices.EqualFunc(got, [][]string{{"Observatory::P::sys::a"}}, slices.Equal) {
		t.Fatalf("d.source cells = %q, want sys::a", got)
	}
}
