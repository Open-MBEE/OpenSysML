package semantics_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// unionModel is a model of unioning types, one of them nested and one of them
// its own union, with two ordinary specializations to compare against.
func unionModel(t *testing.T) (*semantics.Model, *symbols.Index) {
	t.Helper()
	idx := libs.NewModelIndex()
	idx.AddDocument("<t>", parser.New(source.New("<t>", []byte(`package T {
		part def Vehicle;
		part def Car :> Vehicle;
		part def Coupe :> Car;
		part def Truck :> Vehicle;
		part def Boat;
		part def Wheeled unions Car, Truck;
		part def Registered unions Wheeled;
		part def Looped unions Looped, Boat;
	}`))).ParseFile())
	idx.ExpandWildcardImports()
	return semantics.NewModel(resolve.New(idx)), idx
}

// TestClassifiesUnionTargets: a union classifies every value of the types it
// unions, at any nesting depth, and a cyclic union neither loops nor lies.
func TestClassifiesUnionTargets(t *testing.T) {
	m, idx := unionModel(t)
	cases := []struct {
		target, typ string
		want        bool
	}{
		{"Wheeled", "Car", true},
		{"Wheeled", "Truck", true},
		{"Wheeled", "Coupe", true},
		{"Wheeled", "Vehicle", false},
		{"Wheeled", "Boat", false},
		{"Registered", "Car", true},
		{"Registered", "Truck", true},
		{"Registered", "Boat", false},
		{"Looped", "Boat", true},
		{"Looped", "Car", false},
		{"Vehicle", "Car", true},
		{"Car", "Vehicle", false},
	}
	for _, c := range cases {
		target := dimensionSymbol(t, idx, "T::"+c.target)
		typ := dimensionSymbol(t, idx, "T::"+c.typ)
		if got := m.Classifies(target, typ); got != c.want {
			t.Errorf("Classifies(%s, %s) = %v, want %v", c.target, c.typ, got, c.want)
		}
		verdict := m.ClassifiesTypes([]*symbols.Symbol{typ}, target)
		if c.want && verdict != semantics.ClassifiesAll {
			t.Errorf("ClassifiesTypes(%s, %s) = %v, want all", c.typ, c.target, verdict)
		}
		if !c.want && verdict == semantics.ClassifiesAll {
			t.Errorf("ClassifiesTypes(%s, %s) = all, want less", c.typ, c.target)
		}
	}
}
