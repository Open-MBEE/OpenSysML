package runtime

import (
	"errors"
	"slices"
	"testing"
)

const redefinedCollectionModel = `
	package test {
		private import ScalarValues::Real;
		private import RealFunctions::*;
		part def Sub { attribute mass : Real; }
		part def System {
			part Subsystems : Sub[*];
			attribute totalmass : Real = sum(Subsystems.mass);
		}
		part def Sat :> System {
			part subsystems : Sub[*] :>> Subsystems;
		}
		part sat : Sat {
			part bus : Sub :> subsystems { attribute :>> mass = 4.0; }
			part sensor : Sub :> subsystems { attribute :>> mass = 3.0; }
		}
	}
`

// TestRedefinedCollectionReadsSubsetsUnderEitherName pins that a collection
// shared by a redefinition holds the parts subsetting it however it is reached:
// the subsetting parts name the redefining feature, not the name read here.
func TestRedefinedCollectionReadsSubsetsUnderEitherName(t *testing.T) {
	for _, first := range []string{"Subsystems", "subsystems"} {
		t.Run(first, func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, redefinedCollectionModel))
			matches := idx.LookupQualified("test::sat")
			if len(matches) != 1 {
				t.Fatalf("test::sat: %d matching symbols, want 1", len(matches))
			}
			inst, err := ctx.Instantiate(matches[0])
			if err != nil {
				t.Fatalf("Instantiate: %v", err)
			}
			// Reading the redefined collection is what materializes the shared
			// feature value, so each name has to be the one read first in turn.
			for _, name := range []string{first, "Subsystems", "subsystems"} {
				fv, err := inst.GetFeatureValue(ctx, name)
				if err != nil {
					t.Fatalf("GetFeatureValue(%s): %v", name, err)
				}
				if got := len(elementsOf(fv.HeldValue())); got != 2 {
					t.Fatalf("%s: %d elements, want 2", name, got)
				}
			}
			fv, err := inst.GetFeatureValue(ctx, "totalmass")
			if err != nil {
				t.Fatalf("GetFeatureValue(totalmass): %v", err)
			}
			if got := FormatTraceValue(fv.HeldValue()); got != "7.0" {
				t.Fatalf("totalmass = %s, want 7.0", got)
			}
		})
	}
}

// Redeclaring an inherited attribute under a new name keeps the value the
// redefined declaration wrote, and both names read it.
func TestRedefiningFeatureHoldsTheRedefinedDefault(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::Real;
			part def Vehicle { attribute mass : Real = 1000.0; }
			part def Truck :> Vehicle { attribute grossMass :>> mass; }
			part truck : Truck;
		}
	`))
	matches := idx.LookupQualified("test::truck")
	if len(matches) != 1 {
		t.Fatalf("test::truck: %d matching symbols, want 1", len(matches))
	}
	inst, err := ctx.Instantiate(matches[0])
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	for _, name := range []string{"grossMass", "mass"} {
		fv, err := inst.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("GetFeatureValue(%s): %v", name, err)
		}
		if got := FormatTraceValue(fv.HeldValue()); got != "1000.0" {
			t.Errorf("%s = %s, want 1000.0", name, got)
		}
	}
}

// A usage restating a redefinition writes the one feature it names, so the
// redefined name is not left holding the definition's default.
func TestRestatedRedefinitionWritesOneFeatureValue(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::Real;
			part def Comp { attribute mass : Real = 0.0; }
			part def Sys :> Comp { attribute own :>> mass = 1.5; }
			part sat : Sys { attribute :>> own = 10.0; }
		}
	`))
	matches := idx.LookupQualified("test::sat")
	if len(matches) != 1 {
		t.Fatalf("test::sat: %d matching symbols, want 1", len(matches))
	}
	inst, err := ctx.Instantiate(matches[0])
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	for _, name := range []string{"own", "mass"} {
		fv, err := inst.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("GetFeatureValue(%s): %v", name, err)
		}
		if got := FormatTraceValue(fv.HeldValue()); got != "10.0" {
			t.Errorf("%s = %s, want 10.0", name, got)
		}
	}
	if inst.FeatureValues["own"] != inst.FeatureValues["mass"] {
		t.Error("own and mass are separate feature values, want one shared feature value")
	}
}

// `:> ISQ::mass` specializes the library feature, so it contributes nothing to
// the object's own same-named `mass` collection.
func TestSubsettingIgnoresALibraryFeatureOfTheSameName(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import SI::*;
			part def Component { attribute v : Real; }
			part def System {
				part mass : Component[*];
				attribute totalmass :> ISQ::mass = 4 [kg];
			}
			part sat : System;
		}
	`))
	matches := idx.LookupQualified("test::sat")
	if len(matches) != 1 {
		t.Fatalf("test::sat: %d matching symbols, want 1", len(matches))
	}
	inst, err := ctx.Instantiate(matches[0])
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "mass")
	if err != nil {
		t.Fatalf("GetFeatureValue(mass): %v", err)
	}
	if got := len(elementsOf(fv.HeldValue())); got != 0 {
		t.Errorf("mass holds %d elements, want 0", got)
	}
}

// defaultNullCollectionModel: collections written `default null` that members
// subset at the type, the usage, an inheriting and a redefining declaration.
const defaultNullCollectionModel = `
	package test {
		private import ScalarValues::*;
		private import SI::*;
		private import ISQ::*;
		private import NumericalFunctions::*;
		part def Massed {
			part subcomponents : Massed [*] default null;
			attribute mass :> ISQ::mass;
			attribute totalMass :> ISQ::mass = mass + sum(subcomponents.totalMass);
		}
		part def Leaf :> Massed { attribute :>> mass = 100 [kg]; }
		part def Heavy :> Leaf { attribute :>> mass = 500 [kg]; }
		part def Stack :> Massed {
			attribute :>> mass = 10 [kg];
			part a : Leaf :> subcomponents;
			part b : Leaf subsets subcomponents;
		}
		part def Stack3 :> Stack { part c : Leaf :> subcomponents; }
		part def StackR :> Stack { part :>> a : Heavy; }
		part def Tower :> Massed {
			attribute :>> mass = 1 [kg];
			part s : Stack :> subcomponents;
			part t : Stack3 :> subcomponents;
		}
		part def Alone :> Massed { attribute :>> mass = 7 [kg]; }
		part def Engine;
		part def Bound {
			part engines : Engine [*] = ();
			part engine : Engine :> engines;
		}
		part def Optional {
			part engine : Engine [0..1] default null;
			part main : Engine :> engine;
		}
		part def Crowded {
			part engine : Engine [0..1] default null;
			part pair : Engine [2] :> engine;
		}
		part def Rated {
			attribute rating : Integer default 5;
			attribute measured : Integer :> rating = 7;
			attribute limit : Integer default 9;
		}
		part def Plain;
		part def Scored { attribute score : Integer default 5; }
		part def Measuring :> Scored { attribute measured : Integer :> score = 7; }
		part def Gauged { attribute level : Integer default 5; attribute reading : Integer = 7; }
		part def Reporting :> Gauged { attribute :>> reading :> level; }
		part def Wheel;
		part def Mistyped {
			part engines : Engine [*] default null;
			part wheel : Wheel :> engines;
		}
		part def Sized {
			attribute size : Integer default 1;
			attribute exact : Real :> size = 2.5;
		}
		part leaf : Leaf;
		part stack : Stack;
		part stack3 : Stack3;
		part stackR : StackR;
		part tower : Tower;
		part alone : Alone;
		part usage : Massed {
			attribute :>> mass = 1 [kg];
			part p : Leaf :> subcomponents;
			part q : Leaf :> subcomponents;
		}
		part bound : Bound;
		part optional : Optional;
		part crowded : Crowded;
		part rated : Rated;
		part plain : Plain;
		part scored : Scored;
		part gauged : Gauged;
		part mistyped : Mistyped;
		part sized : Sized;
	}
`

// TestDefaultNullCollectionHoldsTheMembersSubsettingIt: `default null` is the
// value of a collection only where nothing populates it; the members subsetting
// it do, however the subsetting is declared, and a rollup over the collection
// reads them recursively.
func TestDefaultNullCollectionHoldsTheMembersSubsettingIt(t *testing.T) {
	cases := []struct {
		usage   string
		members []string
		total   string
	}{
		{"leaf", nil, "100 [kg]"},
		{"stack", []string{"a", "b"}, "210 [kg]"},
		{"stack3", []string{"a", "b", "c"}, "310 [kg]"},
		{"stackR", []string{"a", "b"}, "610 [kg]"},
		{"tower", []string{"s", "t"}, "521 [kg]"},
		{"alone", nil, "7 [kg]"},
		{"usage", []string{"p", "q"}, "201 [kg]"},
	}
	for _, tc := range cases {
		t.Run(tc.usage, func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, defaultNullCollectionModel))
			inst := instantiateNamed(t, ctx, idx, "test::"+tc.usage)
			fv, err := inst.GetFeatureValue(ctx, "subcomponents")
			if err != nil {
				t.Fatalf("GetFeatureValue(subcomponents): %v", err)
			}
			members := elementsOf(fv.HeldValue())
			if len(members) != len(tc.members) {
				t.Fatalf("subcomponents = %s, want %d members", FormatValue(fv.HeldValue()), len(tc.members))
			}
			for _, name := range tc.members {
				member, err := inst.GetFeatureValue(ctx, name)
				if err != nil {
					t.Fatalf("GetFeatureValue(%s): %v", name, err)
				}
				if !slices.Contains(members, member.HeldValue()) {
					t.Errorf("subcomponents = %s, want it to hold %s = %s", FormatValue(fv.HeldValue()), name, FormatValue(member.HeldValue()))
				}
			}
			total, err := inst.GetFeatureValue(ctx, "totalMass")
			if err != nil {
				t.Fatalf("GetFeatureValue(totalMass): %v", err)
			}
			if got := FormatValue(total.HeldValue()); got != tc.total {
				t.Errorf("totalMass = %s, want %s", got, tc.total)
			}
		})
	}
}

// TestDefaultIsFallbackOnlyWhereWrittenDefault: a collection bound with `=`
// holds what the binding states whatever subsets it; a scalar `default null`
// yields to the one member subsetting it, and two members violate its
// multiplicity rather than being dropped. A constant scalar default yields
// to its subsetter too, while one nothing subsets is still folded.
func TestDefaultIsFallbackOnlyWhereWrittenDefault(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, defaultNullCollectionModel))

	bound := instantiateNamed(t, ctx, idx, "test::bound")
	fv, err := bound.GetFeatureValue(ctx, "engines")
	if err != nil {
		t.Fatalf("GetFeatureValue(engines): %v", err)
	}
	if len(elementsOf(fv.HeldValue())) != 0 {
		t.Errorf("engines = %s, want the bound empty sequence", FormatValue(fv.HeldValue()))
	}

	optional := instantiateNamed(t, ctx, idx, "test::optional")
	fv, err = optional.GetFeatureValue(ctx, "engine")
	if err != nil {
		t.Fatalf("GetFeatureValue(engine): %v", err)
	}
	main, err := optional.GetFeatureValue(ctx, "main")
	if err != nil {
		t.Fatalf("GetFeatureValue(main): %v", err)
	}
	if fv.HeldValue() != main.HeldValue() || fv.HeldValue().Kind != ValInstance {
		t.Errorf("engine = %s, want main = %s", FormatValue(fv.HeldValue()), FormatValue(main.HeldValue()))
	}

	crowded := instantiateNamed(t, ctx, idx, "test::crowded")
	if _, err := crowded.GetFeatureValue(ctx, "engine"); !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("GetFeatureValue(engine) error = %v, want ErrMultiplicityViolation", err)
	}

	rated := instantiateNamed(t, ctx, idx, "test::rated")
	for name, want := range map[string]string{"rating": "7", "measured": "7", "limit": "9"} {
		fv, err := rated.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("GetFeatureValue(%s): %v", name, err)
		}
		if got := FormatValue(fv.HeldValue()); got != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}
}

// TestClassifierSubsetterSupersedesAFoldedDefault: a classifier added to an object
// brings its subsetters with it, whether the constant default it supersedes comes
// along with it or was folded when the object was created, and whether the
// subsetter is a new feature or redefines one the object already has.
func TestClassifierSubsetterSupersedesAFoldedDefault(t *testing.T) {
	cases := []struct{ usage, classifier, feature string }{
		{"plain", "Rated", "rating"},
		{"scored", "Measuring", "score"},
		{"gauged", "Reporting", "level"},
	}
	for _, tc := range cases {
		t.Run(tc.usage, func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, defaultNullCollectionModel))
			inst := instantiateNamed(t, ctx, idx, "test::"+tc.usage)
			if err := ctx.classify(inst, idx.LookupQualified("test::" + tc.classifier)[0]); err != nil {
				t.Fatalf("classify(%s): %v", tc.classifier, err)
			}
			fv, err := inst.GetFeatureValue(ctx, tc.feature)
			if err != nil {
				t.Fatalf("GetFeatureValue(%s): %v", tc.feature, err)
			}
			if got := FormatValue(fv.HeldValue()); got != "7" {
				t.Errorf("%s = %s after classifying as %s, want the subsetter's 7", tc.feature, got, tc.classifier)
			}
		})
	}
}

// TestSubsetterContributionIsTypedByTheSubsettedFeature: what a subsetter
// contributes is judged by the subsetted feature's own type, an object or a
// scalar alike, and a refused contribution leaves the feature unmaterialized.
func TestSubsetterContributionIsTypedByTheSubsettedFeature(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, defaultNullCollectionModel))
	for usage, feature := range map[string]string{"mistyped": "engines", "sized": "size"} {
		inst := instantiateNamed(t, ctx, idx, "test::"+usage)
		if _, err := inst.GetFeatureValue(ctx, feature); !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s.GetFeatureValue(%s) error = %v, want ErrTypeMismatch", usage, feature, err)
		}
		if fv := inst.FeatureValues[feature]; fv.Materialized || fv.Value.Kind != ValInvalid || fv.Values.Kind != ValInvalid {
			t.Errorf("%s.%s = %+v after the refusal, want it left unmaterialized", usage, feature, fv)
		}
	}
}
