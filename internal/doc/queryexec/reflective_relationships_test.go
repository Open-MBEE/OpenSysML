package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A query navigates an element's reflected subsetting relationships and back to
// the elements they relate; two relationships of one owner stay distinct.
func TestExecuteReflectiveSubsettingNavigation(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def T {
	part a;
	part c;
	part b subsets a;
	part d subsets c;
}
calc def Subsettings :> Query {
	in root : Element;
	Project(
		source = Named(qualifiedName = "Observatory::T::b"),
		properties = ("name"),
		columns = (Column(name = "subsettings", expression = KerML::Feature::ownedSubsetting))
	)
}`)
	result, err := fixture.execute(t, "Subsettings", Bindings{
		"root": {ElementValue(fixture.symbol(t, "T"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute Subsettings: %v", err)
	}
	if rows := result.Rows(); len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	b := fixture.symbol(t, "T::b")
	owned, ok := fixture.model.ReflectiveElements(b, "ownedSubsetting")
	if !ok || len(owned) != 1 {
		t.Fatalf("b.ownedSubsetting = %v (supported %t), want one relationship", owned, ok)
	}
	values := result.Rows()[0].Cells()[1].Values()
	if len(values) != 1 {
		t.Fatalf("subsettings cell = %v, want one relationship", values)
	}
	got, ok := values[0].Element()
	if !ok || got != owned[0] {
		t.Fatalf("subsettings cell value = %v, want relationship %p", values[0], owned[0])
	}
	a := fixture.symbol(t, "T::a")
	if crossed, ok := fixture.model.ReflectiveElements(owned[0], "subsettedFeature"); !ok ||
		len(crossed) != 1 || crossed[0] != a {
		t.Fatalf("subsettedFeature = %v (supported %t), want a", crossed, ok)
	}
	if owners, ok := fixture.model.ReflectiveElements(owned[0], "owningRelatedElement"); !ok ||
		len(owners) != 1 || owners[0] != b {
		t.Fatalf("owningRelatedElement = %v (supported %t), want b", owners, ok)
	}
	if owners, ok := fixture.model.ReflectiveElements(owned[0], "owner"); !ok ||
		len(owners) != 1 || owners[0] != b {
		t.Fatalf("relationship.owner = %v (supported %t), want b", owners, ok)
	}

	d := fixture.symbol(t, "T::d")
	other, ok := fixture.model.ReflectiveElements(d, "ownedSubsetting")
	if !ok || len(other) != 1 {
		t.Fatalf("d.ownedSubsetting = %v (supported %t), want one relationship", other, ok)
	}
	if symbols.SameElement(owned[0], other[0]) ||
		symbols.KeyOf(owned[0]) == symbols.KeyOf(other[0]) {
		t.Fatal("distinct subsetting relationships compare equal")
	}
}
