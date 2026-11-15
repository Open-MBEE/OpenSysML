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
}
