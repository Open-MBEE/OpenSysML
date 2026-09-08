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

// composedModel is a model of composed types — unions (one nested, one its own
// union), an intersection and a difference — with ordinary specializations to
// compare against.
func composedModel(t *testing.T) (*semantics.Model, *symbols.Index) {
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
		part def Electric;
		part def ElectricCar :> Car, Electric;
		part def ElectricVehicle intersects Vehicle, Electric;
		part def CombustionVehicle differences Vehicle, Electric;
		part def CycleA intersects CycleB, Vehicle;
		part def CycleB intersects CycleA, Electric;
		part def Burner :> CombustionVehicle;
		part def RoadBurner intersects Burner, Vehicle;
		part def Ping differences Ping, Pong;
		part def Pong differences Pong, Ping;
	}`))).ParseFile())
	idx.ExpandWildcardImports()
	return semantics.NewModel(resolve.New(idx)), idx
}

// TestClassifiesComposedTargets: a composed target classifies as its types do —
// any of a union at any nesting depth, every one of an intersection, the first of
// a difference and none of the rest — and a cyclic one neither loops nor lies.
func TestClassifiesComposedTargets(t *testing.T) {
	m, idx := composedModel(t)
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
		// An intersection's values are those of every type it intersects.
		{"ElectricVehicle", "ElectricCar", true},
		{"ElectricVehicle", "Car", false},
		{"ElectricVehicle", "Boat", false},
		{"CycleA", "ElectricCar", false},
		{"CycleB", "Boat", false},
		// A difference's are those of the first type that are none of the rest.
		{"CombustionVehicle", "Car", true},
		{"CombustionVehicle", "ElectricCar", false},
		{"CombustionVehicle", "Boat", false},
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

// TestSubtractedTypeExcludesADeclaredValue: a value whose declared type
// specializes a difference is still none of its values when another type it is of
// is one the difference subtracts, however the difference is reached.
func TestSubtractedTypeExcludesADeclaredValue(t *testing.T) {
	m, idx := composedModel(t)
	sym := func(name string) *symbols.Symbol { return dimensionSymbol(t, idx, "T::"+name) }
	for _, c := range []struct {
		name   string
		types  []string
		target string
		want   semantics.TypeClassification
	}{
		{"declared burner", []string{"Burner"}, "CombustionVehicle", semantics.ClassifiesAll},
		{"burner held as electric", []string{"Burner", "Electric"}, "CombustionVehicle", semantics.ClassifiesNone},
		{"burner held as an electric car", []string{"Burner", "ElectricCar"}, "CombustionVehicle", semantics.ClassifiesNone},
		{"through an intersection", []string{"RoadBurner", "Electric"}, "RoadBurner", semantics.ClassifiesNone},
		{"cyclic differences terminate", []string{"Ping"}, "Ping", semantics.ClassifiesAll},
	} {
		types := make([]*symbols.Symbol, 0, len(c.types))
		for _, name := range c.types {
			types = append(types, sym(name))
		}
		if got := m.ClassifiesTypes(types, sym(c.target)); got != c.want {
			t.Errorf("%s: ClassifiesTypes(%v, %s) = %v, want %v", c.name, c.types, c.target, got, c.want)
		}
	}
}

// TestMayShareValuesOfComposedTypes: a cast between a composed type and a type
// one of its operands relates to selects rather than being unrelated.
func TestMayShareValuesOfComposedTypes(t *testing.T) {
	m, idx := composedModel(t)
	cases := []struct {
		target, typ string
		want        bool
	}{
		{"Car", "Wheeled", true},
		{"Coupe", "Wheeled", true},
		{"Car", "Registered", true},
		{"Boat", "Wheeled", false},
		{"Boat", "Looped", true},
		{"Car", "Looped", false},
		{"Electric", "ElectricVehicle", true},
		{"Boat", "ElectricVehicle", false},
		// A cast to a composed type selects from a type it is composed of.
		{"ElectricVehicle", "Vehicle", true},
		{"ElectricVehicle", "Car", true},
		{"ElectricVehicle", "Boat", false},
		{"CycleA", "Electric", true},
		{"CycleA", "Boat", false},
		{"Wheeled", "Boat", false},
		// A difference holds values of the first type it names and none of the rest,
		// so a type the rest classify shares nothing with it.
		{"Car", "CombustionVehicle", true},
		{"Electric", "CombustionVehicle", false},
		{"ElectricCar", "CombustionVehicle", false},
	}
	for _, c := range cases {
		target := dimensionSymbol(t, idx, "T::"+c.target)
		typ := dimensionSymbol(t, idx, "T::"+c.typ)
		if got := m.MayShareValues(target, typ); got != c.want {
			t.Errorf("MayShareValues(%s, %s) = %v, want %v", c.target, c.typ, got, c.want)
		}
	}
}
