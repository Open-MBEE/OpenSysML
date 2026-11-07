package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// TestConnectorEndDefaultMultiplicityHasItsOwnElementID: the Multiplicity stating an end's implicit
// [1..1] and its OwningMembership are identified as the conversion mints them, named connection or not.
func TestConnectorEndDefaultMultiplicityHasItsOwnElementID(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	part def A { port o; port i; }
	part def B { port i; port o; }
	part a : A;
	part b : B;
	connection c connect a.o to b.i;
	connect a.i to b.o;
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	elementID := func(sym *symbols.Symbol) (string, error) { return ElementIDOf(read, fixture.index, sym) }
	seen := 0
	members, _ := fixture.model.ReflectiveElements(fixture.symbol(t, "a").OwnerScope.Owner(), "ownedMember")
	for _, connection := range members {
		if connection.Kind != symbols.SymbolConnectionUsage {
			continue
		}
		ends, _ := fixture.model.ReflectiveElements(connection, "connectorEnd")
		if len(ends) != 2 {
			t.Fatalf("%s: connectorEnd = %d ends, want 2", connection.Name, len(ends))
		}
		for i, end := range ends {
			endID, err := elementID(end)
			if err != nil || endID == "" {
				t.Fatalf("%s end %d id: %q, %v", connection.Name, i, endID, err)
			}
			multiplicities, _ := fixture.model.ReflectiveElements(end, "multiplicity")
			if len(multiplicities) != 1 || !semantics.IsDefaultMultiplicity(multiplicities[0]) {
				t.Fatalf("%s end %d: multiplicity = %v, want its default Multiplicity", connection.Name, i, multiplicities)
			}
			multiplicity := multiplicities[0]
			if id, err := elementID(multiplicity); err != nil || id != endID+"_pmultiplicity" {
				t.Errorf("%s end %d: Multiplicity id %q, %v; want %q", connection.Name, i, id, err, endID+"_pmultiplicity")
			}
			owned, _ := fixture.model.ReflectiveElements(end, "ownedRelationship")
			memberships := 0
			for _, rel := range owned {
				members, _ := fixture.model.ReflectiveElements(rel, "memberElement")
				if len(members) != 1 || members[0] != multiplicity {
					continue
				}
				memberships++
				if id, err := elementID(rel); err != nil || id == "" {
					t.Errorf("%s end %d: the Multiplicity's OwningMembership id %q, %v; want one", connection.Name, i, id, err)
				}
			}
			if memberships != 1 {
				t.Errorf("%s end %d: %d memberships own the Multiplicity, want 1", connection.Name, i, memberships)
			}
			seen++
		}
	}
	if seen != 4 {
		t.Fatalf("checked %d ends, want 4", seen)
	}
}
