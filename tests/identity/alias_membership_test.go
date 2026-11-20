package identity_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A library alias is a Membership its namespace owns (KerML 8.2.3.4.1), which no qualified
// name catalogues; it is placed among the owner's Memberships and its id is its own.
func TestLibraryAliasMembershipIsIdentified(t *testing.T) {
	base := libs.SharedBase()
	resolver := resolve.New(base)
	model := semantics.NewModel(resolver)
	catalog := identity.LibraryCatalogWithOpenSysML(base)
	owner := resolver.Index().LookupQualified("ISQThermodynamics")[0]
	seen := map[string]string{}
	index := 0
	for _, member := range owner.Scope.Members() {
		if member.Kind != symbols.SymbolAlias {
			if _, ok := identity.AliasMembershipOf(model, catalog, member); ok {
				t.Errorf("%s is no alias but has an alias membership", member.Name)
			}
			continue
		}
		alias, ok := identity.AliasMembershipOf(model, catalog, member)
		if !ok {
			t.Fatalf("AliasMembershipOf refuses the alias %s", member.Name)
		}
		if other, dup := seen[alias.ID]; dup {
			t.Errorf("aliases %s and %s share the id %s", other, member.Name, alias.ID)
		}
		seen[alias.ID] = member.Name
		if _, clash := catalog.Element(alias.ID); clash {
			t.Errorf("alias %s has the id of a catalogued element", member.Name)
		}
		if _, clash := catalog.OwningMembership(alias.ID); clash {
			t.Errorf("alias %s has the id of a catalogued owning membership", member.Name)
		}
		if alias.Owner == nil || alias.Owner.FQN != "ISQThermodynamics" {
			t.Errorf("alias %s is owned by %+v, want ISQThermodynamics", member.Name, alias.Owner)
		}
		index++
	}
	if index == 0 {
		t.Fatal("ISQThermodynamics declares no alias; the test needs another library package")
	}
	// The pilot's exporter keys the same membership by this path (its second alias).
	value, found := owner.Scope.LookupLocal("TemperatureValue")
	if !found {
		t.Fatal("ISQThermodynamics has no member TemperatureValue")
	}
	alias, ok := identity.AliasMembershipOf(model, catalog, value)
	if !ok || alias.Path != "ISQThermodynamics/Membership[1]" {
		t.Errorf("TemperatureValue is %q, %v; want ISQThermodynamics/Membership[1]", alias.Path, ok)
	}
}
