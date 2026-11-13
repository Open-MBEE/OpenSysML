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

// An allocation definition inherits the library's valueless `source` and `target` ends as a connector does;
// past them it answers Relationship::source and target as the types of its own ends.
func TestAssociationSourceAndTargetAnswerPastInheritedEnds(t *testing.T) {
	fixture := loadExecutionFixture(t, `package P {
	part def A;
	part def B;
	allocation def F2P { end f : A; end p : B; }
}
calc def Q :> Query {
	in root : Element;
	Project(source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "AllocationDefinition"), properties = ("relatedElement", "source", "target"))
}`)
	result, err := fixture.execute(t, "Q", Bindings{"root": {ElementValue(fixture.symbol(t, "P"))}}, Options{})
	if err != nil {
		t.Fatalf("execute Q: %v", err)
	}
	for property, want := range map[string][]string{
		"relatedElement": {"Observatory::P::A", "Observatory::P::B"},
		"source":         {"Observatory::P::A"},
		"target":         {"Observatory::P::B"},
	} {
		if got := cellElementNames(t, result, property); len(got) != 1 || !slices.Equal(got[0], want) {
			t.Errorf("%s cells = %q, want %q", property, got, want)
		}
	}
}

// A definition's own valueless end named `source` or `target` yields to Relationship::source and target as an
// inherited one does; an own valueless member that is no end keeps shadowing the metaclass property.
func TestOwnEndNamedSourceYieldsToRelationshipEnds(t *testing.T) {
	fixture := loadExecutionFixture(t, `package P {
	port def OpticalPort;
	interface def OpticalInterface { end source : OpticalPort; end target : OpticalPort; }
	part def A { attribute isComposite : Boolean; }
	part a : A { attribute isComposite : Boolean; }
}
calc def Q :> Query {
	in root : Element;
	Project(source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "KerML::Kernel::Association"), properties = ("source", "target", "relatedElement"))
}
calc def R :> Query {
	in root : Element;
	Project(source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "SysML::Systems::PartUsage"), properties = ("isComposite"))
}`)
	result, err := fixture.execute(t, "Q", Bindings{"root": {ElementValue(fixture.symbol(t, "P"))}}, Options{})
	if err != nil {
		t.Fatalf("execute Q: %v", err)
	}
	for property, want := range map[string][]string{
		"source":         {"Observatory::P::OpticalPort"},
		"target":         {"Observatory::P::OpticalPort"},
		"relatedElement": {"Observatory::P::OpticalPort", "Observatory::P::OpticalPort"},
	} {
		if got := cellElementNames(t, result, property); !slices.EqualFunc(got, [][]string{want}, slices.Equal) {
			t.Errorf("%s cells = %q, want %q past the own end of that name", property, got, want)
		}
	}
	result, err = fixture.execute(t, "R", Bindings{"root": {ElementValue(fixture.symbol(t, "P"))}}, Options{})
	if err != nil {
		t.Fatalf("execute R: %v", err)
	}
	if got := cellElementNames(t, result, "isComposite"); !slices.EqualFunc(got, [][]string{nil}, slices.Equal) {
		t.Errorf("isComposite cells = %q, want the own valueless attribute, not the metaclass flag", got)
	}
}

// An own end that redefines a valueless feature explicitly still redefines the end at its position
// (KerML 8.3.3.3.4); a stated empty value there keeps shadowing the metaclass property.
func TestOwnEndKeepsShadowingPastStatedEmptyPositionalGeneral(t *testing.T) {
	fixture := loadExecutionFixture(t, `package P {
	part def A;
	part def B;
	connection def G { end a : A = null; end b : B; }
	connection def G2 { end q : A; end r : B; }
	connection def S :> G, G2 { end source : A redefines q; end target : B; }
	connection def T :> G2 { end source : A redefines q; end target : B; }
}
calc def Q :> Query {
	in root : Element;
	Project(source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "KerML::Kernel::Association"), properties = ("source"))
}`)
	result, err := fixture.execute(t, "Q", Bindings{"root": {ElementValue(fixture.symbol(t, "P"))}}, Options{})
	if err != nil {
		t.Fatalf("execute Q: %v", err)
	}
	want := [][]string{{"Observatory::P::A"}, {"Observatory::P::A"}, nil, {"Observatory::P::A"}}
	if got := cellElementNames(t, result, "source"); !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("source cells = %q, want %q: S keeps G::a's stated empty value, the others yield to the end's type", got, want)
	}
}
