package queryexec

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// A constraint body's closing bare condition reads the ids the conversion mints for the
// member stating it — `<owner>___40N` and `___40N_om` — on a definition and a `require constraint`.
func TestConstraintResultExpressionReadsTheConversionIDs(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
    constraint def MassBudget { in total : Real; total <= 30.0 }
    part def Subsystem { attribute mass : Real; }
    requirement def MassLimit { subject sub : Subsystem; require constraint { sub.mass <= 20.0 } }
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	elementID := func(sym *symbols.Symbol) string {
		id, err := ElementIDOf(read, fixture.index, sym)
		if err != nil {
			t.Fatalf("%v id: %v", sym.Decl, err)
		}
		return id
	}
	budget := fixture.symbol(t, "MassBudget")
	owned, _ := fixture.model.ReflectiveElements(budget, "ownedRelationship")
	if got, want := strings.Join(elementIDs(elementID, owned), " "), "Observatory__MassBudget__total_om Observatory__MassBudget___401_om"; got != want {
		t.Fatalf("MassBudget ownedRelationship ids = %q, want %q", got, want)
	}
	results, _ := fixture.model.ReflectiveElements(budget, "ownedMember")
	if got, want := strings.Join(elementIDs(elementID, results), " "), "Observatory__MassBudget__total Observatory__MassBudget___401"; got != want {
		t.Fatalf("MassBudget ownedMember ids = %q, want %q", got, want)
	}
	limit := fixture.symbol(t, "MassLimit")
	members, _ := fixture.model.ReflectiveElements(limit, "ownedMember")
	if len(members) != 2 {
		t.Fatalf("MassLimit owns %d members", len(members))
	}
	owned, _ = fixture.model.ReflectiveElements(members[1], "ownedRelationship")
	if got, want := strings.Join(elementIDs(elementID, owned), " "), "Observatory__MassLimit___401___400_om"; got != want {
		t.Fatalf("the require constraint's ownedRelationship ids = %q, want %q", got, want)
	}
	// The condition's own parts — parameters, operands and their memberships — read the ids minted under it.
	parameters, _ := fixture.model.ReflectiveElements(results[1], "ownedFeature")
	if got, want := strings.Join(elementIDs(elementID, parameters), " "), "Observatory__MassBudget___401_pin0 Observatory__MassBudget___401_pin1 Observatory__MassBudget___401_pout"; got != want {
		t.Fatalf("the condition's ownedFeature ids = %q, want %q", got, want)
	}
	var walk func(sym *symbols.Symbol, depth int)
	walk = func(sym *symbols.Symbol, depth int) {
		owned, _ := fixture.model.ReflectiveElements(sym, "ownedRelationship")
		for _, rel := range owned {
			if elementID(rel) == "" {
				t.Errorf("a %s under the condition reads no id", fixture.model.MetaclassOf(rel).Name)
			}
			if member := rel.Implicit.Member; member != nil && depth < 4 {
				if elementID(member) == "" {
					t.Errorf("a %s under the condition reads no id", fixture.model.MetaclassOf(member).Name)
				}
				walk(member, depth+1)
			}
		}
	}
	walk(results[1], 0)
}
