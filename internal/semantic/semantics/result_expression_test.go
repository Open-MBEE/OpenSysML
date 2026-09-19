package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// ownerNames names the owner of each result expression membership of sym.
func ownerNames(m *Model, sym *symbols.Symbol) []string {
	var names []string
	for _, expr := range m.ResultExpressionMemberships(sym) {
		names = append(names, expr.Owner.Name)
	}
	return names
}

func TestResultExpressionOwnersInheritedOnly(t *testing.T) {
	m, root := buildModel(t,
		"constraint def Base { x > 0 } constraint def Sub :> Base { }")
	if got := ownerNames(m, sym(t, root, "Sub")); strings.Join(got, ",") != "Base" {
		t.Fatalf("an empty specialization inherits Base's result, got %v", got)
	}
}

func TestResultExpressionOwnersOwnedFirst(t *testing.T) {
	m, root := buildModel(t,
		"constraint def Base { x > 0 } constraint def Sub :> Base { x > 1 }")
	if got := ownerNames(m, sym(t, root, "Sub")); strings.Join(got, ",") != "Sub,Base" {
		t.Fatalf("the owned result comes first, then the inherited one, got %v", got)
	}
}

func TestResultExpressionOwnersDiamondCountsOnce(t *testing.T) {
	m, root := buildModel(t,
		"constraint def Base { x > 0 } constraint def L :> Base; constraint def R :> Base;"+
			" constraint def D :> L, R;")
	if got := ownerNames(m, sym(t, root, "D")); strings.Join(got, ",") != "Base" {
		t.Fatalf("Base reached along two paths contributes once, got %v", got)
	}
}

func TestResultExpressionOwnersTwoGenerals(t *testing.T) {
	m, root := buildModel(t,
		"constraint def A { x > 0 } constraint def B { x > 1 } constraint def C :> A, B;")
	if got := ownerNames(m, sym(t, root, "C")); strings.Join(got, ",") != "A,B" {
		t.Fatalf("each general owning a result is an owner, got %v", got)
	}
}

func TestResultExpressionOwnersThroughReferenceSubsetting(t *testing.T) {
	m, root := buildModel(t,
		"constraint c { x > 0 } constraint d ::> c { x > 1 } constraint kept ::> c;")
	if got := ownerNames(m, sym(t, root, "d")); strings.Join(got, ",") != "d,c" {
		t.Fatalf("a referenced constraint's result is inherited, got %v", got)
	}
	if got := ownerNames(m, sym(t, root, "kept")); strings.Join(got, ",") != "c" {
		t.Fatalf("a bodiless referencing constraint inherits the result alone, got %v", got)
	}
}

func TestResultExpressionManyConditionsOneOwner(t *testing.T) {
	m, root := buildModel(t,
		"constraint def Base { in x; x > 0 x < 10 }")
	base := sym(t, root, "Base")
	if got := len(OwnedResultExpressions(base)); got != 2 {
		t.Fatalf("two conditions are two result expressions, got %d", got)
	}
	if got := ownerNames(m, base); strings.Join(got, ",") != "Base" {
		t.Fatalf("both belong to one owner, got %v", got)
	}
	if m.ResultExpressionConflict(base) != nil {
		t.Fatal("the conditions of one constraint body are one membership")
	}
}

func TestResultExpressionSecondStatedIsAMembership(t *testing.T) {
	m, root := buildModel(t, "calc c { 1 2 } calc def C { in y; y + 1 y + 2 } calc def D :> C;")
	for _, name := range []string{"c", "C"} {
		s := sym(t, root, name)
		if got := ownerNames(m, s); strings.Join(got, ",") != name+","+name {
			t.Fatalf("%s: each expression of a calculation body is a membership, got %v", name, got)
		}
		conflict := m.ResultExpressionConflict(s)
		if conflict == nil || conflict.Stated != 2 || conflict.Node != OwnedResultExpressions(s)[1].Node {
			t.Fatalf("%s: the second stated expression is the fault, got %+v", name, conflict)
		}
	}
	d := sym(t, root, "D")
	if got := ownerNames(m, d); strings.Join(got, ",") != "C,C" {
		t.Fatalf("a bodiless specialization inherits both, got %v", got)
	}
	if conflict := m.ResultExpressionConflict(d); conflict == nil || conflict.Stated != 0 || conflict.Node != d.Decl {
		t.Fatalf("inheriting two faults the declaration, got %+v", conflict)
	}
}

func TestResultExpressionConflictStatedOverInherited(t *testing.T) {
	m, root := buildModel(t, "constraint def Base { x > 0 } constraint def Sub :> Base { x > 1 }")
	sub := sym(t, root, "Sub")
	conflict := m.ResultExpressionConflict(sub)
	if conflict == nil || conflict.Stated != 1 || conflict.Node != OwnedResultExpressions(sub)[0].Node {
		t.Fatalf("the body stated over the inherited result is the fault, got %+v", conflict)
	}
	if got := ownerNames(m, sym(t, root, "Base")); strings.Join(got, ",") != "Base" {
		t.Fatalf("Base owns one, got %v", got)
	}
	if m.ResultExpressionConflict(sym(t, root, "Base")) != nil {
		t.Fatal("one membership is no conflict")
	}
}

func TestResultExpressionNestedAssertionIsNotAResult(t *testing.T) {
	m, root := buildModel(t,
		"requirement def Margin { attribute x; require constraint enough { x > 0 } }"+
			" requirement def Tight :> Margin { require constraint :>> enough { assert constraint { x > 1 } } }")
	// A redefining requirement constraint references nothing, so it has no name.
	var redefining *symbols.Symbol
	for _, member := range sym(t, root, "Tight").Scope.AnonymousMembers() {
		if member.Kind == symbols.SymbolConstraintUsage {
			redefining = member
		}
	}
	if redefining == nil {
		t.Fatal("Tight owns no anonymous constraint usage")
	}
	if got := ownerNames(m, redefining); len(got) != 1 || got[0] != "enough" {
		t.Fatalf("a nested assertion adds no result; the redefined one is inherited, got %v", got)
	}
}
