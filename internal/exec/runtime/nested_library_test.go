package runtime

import (
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// nestedLibrarySource declares usages of each nesting kind with no written subsetting.
const nestedLibrarySource = `package Lib {
	part def Wheel;
	part def Vehicle {
		part wheels : Wheel[4];
		part engine { part pistons[2]; }
		item fuel;
		port fuelIn;
		action drive { action steer; action brake; }
		state mode;
	}
	item def Crate { item lid; item walls[4]; }
	port def Socket { port pins[3]; }
	action def Trip { action leave; action arrive; }
	part fleetCar : Vehicle;
}`

const nestedLibraryModel = `package test {
	private import SequenceFunctions::*;
	part def Car :> Lib::Vehicle { part spare : Lib::Wheel; }
	item def Box :> Lib::Crate;
	port def Plug :> Lib::Socket;
	action def Commute :> Lib::Trip;
	part taxi :> Lib::fleetCar;
	part garage { part held : Lib::Vehicle; }
}`

// nestedLibraryContext is a runtime over the standard library, Lib marked as tier, and the model.
func nestedLibraryContext(t *testing.T, tier symbols.LibraryTier) (*Context, *symbols.Index) {
	t.Helper()
	idx := libs.NewModelIndex()
	idx.AddDocument("Lib.sysml", parseAndBuild(t, nestedLibrarySource))
	idx.MarkLibraryTier("Lib.sysml", tier)
	idx.AddDocument("<test>", parseAndBuild(t, nestedLibraryModel))
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	return NewContext(typedModel(semantics.NewModel(resolver), resolver), 10000), idx
}

// A usage a library declares subsets the collection its kind nests under like a model's own
// (SysML v2 §8.3): the object counts the same nested usages whichever document declared them.
func TestLibraryDeclaredUsagesFillImpliedNestingCollections(t *testing.T) {
	cases := []struct{ typ, expr, want string }{
		{"test::Car", "subparts->size()", "6"},
		{"test::Car", "subitems->size()", "7"},
		{"test::Car", "ownedPorts->size()", "1"},
		{"test::Car", "ownedActions->size()", "2"},
		{"test::Car", "ownedStates->size()", "1"},
		{"test::Car", "engine.subparts->size()", "2"},
		{"test::Car", "drive.subactions->size()", "2"},
		{"test::taxi", "subparts->size()", "5"},
		{"test::garage", "held.subparts->size()", "5"},
		{"test::Box", "subitems->size()", "5"},
		{"test::Plug", "subports->size()", "3"},
		{"test::Commute", "subactions->size()", "2"},
	}
	tiers := []symbols.LibraryTier{symbols.TierNone, symbols.TierSystems, symbols.TierDomain, symbols.TierOpenSysML}
	for _, tier := range tiers {
		ctx, idx := nestedLibraryContext(t, tier)
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/%s.%s", tier, tc.typ, tc.expr), func(t *testing.T) {
				obj, err := ctx.Instantiate(oneSymbol(t, idx, tc.typ))
				if err != nil {
					t.Fatalf("instantiate %s: %v", tc.typ, err)
				}
				got, err := evalOn(t, ctx, obj, tc.expr)
				if err != nil {
					t.Fatalf("%s: %v", tc.expr, err)
				}
				if FormatValue(got) != tc.want {
					t.Errorf("%s.%s = %s, want %s", tc.typ, tc.expr, FormatValue(got), tc.want)
				}
			})
		}
	}
}

// A frame tier's usages are no features of the object (the shape leaves them out), so its
// implied collections hold what the shape holds: the model's own nested usages.
func TestFrameTierUsagesStayOutOfImpliedNestingCollections(t *testing.T) {
	for _, tier := range []symbols.LibraryTier{symbols.TierLibrary, symbols.TierKernelSemantic} {
		ctx, idx := nestedLibraryContext(t, tier)
		obj, err := ctx.Instantiate(oneSymbol(t, idx, "test::Car"))
		if err != nil {
			t.Fatalf("%s: instantiate Car: %v", tier, err)
		}
		if names := sortedFeatureNames(ctx.FeaturesOf(obj.Type)); containsString(names, "wheels") || !containsString(names, "spare") {
			t.Fatalf("%s: Car shape = %v, want spare without the frame's wheels", tier, names)
		}
		if got := FormatValue(mustEvalOn(t, ctx, obj, "subparts->size()")); got != "1" {
			t.Errorf("%s: Car.subparts->size() = %s, want 1 (spare)", tier, got)
		}
	}
}
