package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestRuntimeRobustnessInstantiation covers instantiation: duplicate objects, nested and
// repeated parts, subsetted collections and valueless object features.
func TestRuntimeRobustnessInstantiation(t *testing.T) {
	t.Run("duplicate_objects_of_one_declaration", testDuplicateObjectsOfOneDeclaration)
	t.Run("duplicate_objects_holding_a_plain_part", testDuplicateObjectsHoldingAPlainPart)
	t.Run("nested_part_held_with_a_multiplicity", testNestedPartHeldWithAMultiplicity)
	t.Run("part_nested_inside_a_repeated_part", testPartNestedInsideARepeatedPart)
	t.Run("parts_subsetting_one_collection", testPartsSubsettingOneCollection)
	t.Run("object_feature_without_a_value", testObjectFeatureWithoutAValue)
	t.Run("second_instantiation_of_one_type", testSecondInstantiationOfOneType)
}

// testDuplicateObjectsOfOneDeclaration: materializing the same declaration twice
// leaves two live objects, each carrying the nested feature, so a check naming no
// object is a question between them — the name denotes the latest, but the check is
// not silently about it — answered once one of them is named.
func testDuplicateObjectsOfOneDeclaration(t *testing.T) {
	src := `
		package test {
			part def Leaf {
				attribute value = 1.0;
				constraint small { value < 10.0 }
			}
			part def Top {
				part leaf : Leaf;
			}
			part o : Top {
				part :>> leaf { attribute :>> value = 99.0; }
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	obj := memberPath(t, rootScope, "test", "o")
	var tops []*Instance
	for range 2 {
		inst, err := ctx.Instantiate(obj)
		if err != nil {
			t.Fatalf("instantiate o: %v", err)
		}
		tops = append(tops, inst)
	}
	small := memberPath(t, rootScope, "test", "Leaf", "small")
	_, err := ctx.EvaluateConstraint(small, small.OwnerScope)
	if !errors.Is(err, ErrAmbiguousSubject) {
		t.Fatalf("EvaluateConstraint over two objects of o: %v, want ErrAmbiguousSubject", err)
	}
	for _, want := range []string{fmt.Sprintf("(leaf of displaced #%d)", tops[0].ID), "(leaf)", "check it on one of them"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q does not say %q", err, want)
		}
	}
	for _, top := range tops {
		satisfied, err := ctx.EvaluateConstraintOn(small, small.OwnerScope, top)
		if err != nil && !errors.Is(err, ErrViolated) {
			t.Fatalf("EvaluateConstraintOn #%d: %v", top.ID, err)
		}
		if satisfied {
			t.Errorf("object #%d: satisfied = true, want its leaf's 99.0 to violate the constraint", top.ID)
		}
	}
}

// testDuplicateObjectsHoldingAPlainPart: a nested part typed by a definition
// rather than by a body of its own is reached through its holder, so the one
// holder's part is the subject, and a second holder's part makes two subjects.
func testDuplicateObjectsHoldingAPlainPart(t *testing.T) {
	src := `
		package test {
			part def Leaf {
				attribute value = 99.0;
				constraint small { value < 10.0 }
			}
			part def Top {
				part leaf : Leaf;
			}
			part o : Top;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	obj := memberPath(t, rootScope, "test", "o")
	small := memberPath(t, rootScope, "test", "Leaf", "small")
	first, err := ctx.Instantiate(obj)
	if err != nil {
		t.Fatalf("instantiate o: %v", err)
	}
	satisfied, err := ctx.EvaluateConstraint(small, small.OwnerScope)
	if err != nil && !errors.Is(err, ErrViolated) {
		t.Fatalf("EvaluateConstraint: %v", err)
	}
	if satisfied {
		t.Error("satisfied = true, want the object's 99.0 to violate the constraint")
	}
	second, err := ctx.Instantiate(obj)
	if err != nil {
		t.Fatalf("instantiate o again: %v", err)
	}
	if _, err := ctx.EvaluateConstraint(small, small.OwnerScope); !errors.Is(err, ErrAmbiguousSubject) {
		t.Fatalf("EvaluateConstraint over two objects of o: %v, want ErrAmbiguousSubject", err)
	}
	for _, top := range []*Instance{first, second} {
		if _, err := ctx.EvaluateConstraintOn(small, small.OwnerScope, top); !errors.Is(err, ErrViolated) {
			t.Errorf("EvaluateConstraintOn #%d: %v, want its leaf's violation", top.ID, err)
		}
	}
}

// testNestedPartHeldWithAMultiplicity: the objects one feature value materializes for a
// multiplicity are occurrences of one declaration, so a check answers a verdict
// rather than calling its subject ambiguous.
func testNestedPartHeldWithAMultiplicity(t *testing.T) {
	src := `
		package test {
			part def Wheel {
				attribute pressure = 99.0;
				constraint inflated { pressure < 10.0 }
			}
			part def Car {
				part wheels : Wheel[4];
			}
			part car : Car;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	if _, err := ctx.Instantiate(memberPath(t, rootScope, "test", "car")); err != nil {
		t.Fatalf("instantiate car: %v", err)
	}
	inflated := memberPath(t, rootScope, "test", "Wheel", "inflated")
	satisfied, err := ctx.EvaluateConstraint(inflated, inflated.OwnerScope)
	if err != nil && !errors.Is(err, ErrViolated) {
		t.Fatalf("EvaluateConstraint: %v", err)
	}
	if satisfied {
		t.Error("satisfied = true, want the wheels' 99.0 to violate the constraint")
	}
}

// testPartNestedInsideARepeatedPart: the declaration a check names may sit
// deeper inside the part a multiplicity repeated, and the objects reached along
// one declaration path are still one subject rather than an ambiguity.
func testPartNestedInsideARepeatedPart(t *testing.T) {
	src := `
		package test {
			part def Bolt {
				attribute torque = 99.0;
				constraint tight { torque < 10.0 }
			}
			part def Wheel {
				part bolt : Bolt;
			}
			part def Car {
				part wheels : Wheel[4];
			}
			part car : Car;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	if _, err := ctx.Instantiate(memberPath(t, rootScope, "test", "car")); err != nil {
		t.Fatalf("instantiate car: %v", err)
	}
	tight := memberPath(t, rootScope, "test", "Bolt", "tight")
	satisfied, err := ctx.EvaluateConstraint(tight, tight.OwnerScope)
	if err != nil && !errors.Is(err, ErrViolated) {
		t.Fatalf("EvaluateConstraint: %v", err)
	}
	if satisfied {
		t.Error("satisfied = true, want the bolts' 99.0 to violate the constraint")
	}
}

// testPartsSubsettingOneCollection: two declarations feeding one collection are
// two subjects, not repetitions of the collection, so the check reports the
// ambiguity rather than answering from whichever it reached first.
func testPartsSubsettingOneCollection(t *testing.T) {
	src := `
		package test {
			part def Component {
				attribute v = 1.0;
				constraint ok { v < 10.0 }
			}
			part def Assembly {
				part subsystem : Component[*];
				part small : Component :> subsystem {
					attribute :>> v = 5.0;
				}
				part large : Component :> subsystem {
					attribute :>> v = 99.0;
				}
			}
			part assembly : Assembly;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	if _, err := ctx.Instantiate(memberPath(t, rootScope, "test", "assembly")); err != nil {
		t.Fatalf("instantiate assembly: %v", err)
	}
	ok := memberPath(t, rootScope, "test", "Component", "ok")
	satisfied, err := ctx.EvaluateConstraint(ok, ok.OwnerScope)
	if !errors.Is(err, ErrAmbiguousSubject) {
		t.Fatalf("satisfied = %t, err = %v, want ErrAmbiguousSubject", satisfied, err)
	}
	if satisfied {
		t.Error("an ambiguous subject is no verdict")
	}
	// The two objects reached through one collection are told apart by the
	// declaration each materializes, not by the feature holding both.
	for _, want := range []string{"(small)", "(large)"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name a carrier %q", err, want)
		}
	}
}

// testObjectFeatureWithoutAValue: an operation over a feature an object holds no value
// for reports ErrNoValue naming it; only the model-level read is undetermined.
func testObjectFeatureWithoutAValue(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			part def Car { attribute mass : Real; }
			part car : Car;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", file)
	scope := oneSymbol(t, idx, "test").Scope
	if val, err := evalIn(t, ctx, scope, "car.mass + 1.0"); err != nil || val.Undetermined() == nil {
		t.Fatalf("car.mass + 1.0 at model level = %s, %v; want %s", FormatValue(val), err, UndeterminedText)
	}

	inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::car"))
	if err != nil {
		t.Fatalf("instantiate car: %v", err)
	}
	for _, expr := range []string{"mass + 1.0", "(mass > 1.0) and false", "car.mass + 1.0"} {
		val, err := ctx.EvalWithScopeOn(parseExpr(t, expr), scope, inst)
		if err == nil {
			t.Errorf("%s on the object = %s, want an error", expr, FormatValue(val))
			continue
		}
		if !errors.Is(err, ErrNoValue) {
			t.Errorf("%s on the object: expected ErrNoValue, got: %v", expr, err)
		}
		if !strings.Contains(err.Error(), "mass") {
			t.Errorf("%s on the object: error does not name the feature: %v", expr, err)
		}
	}
}

// testSecondInstantiationOfOneType: materializing a type twice builds two objects,
// each with its own execution of the machine the type exhibits, rather than reusing
// or replacing the first object's.
func testSecondInstantiationOfOneType(t *testing.T) {
	src := `
	package test {
		part def Light {
			attribute lit : Integer = 0;
			exhibit state modes {
				entry; then on;
				state on { entry action mark { assign lit := 1; } }
			}
		}
	}`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	sym := oneSymbol(t, idx, "test::Light")

	first, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("first instantiate: %v", err)
	}
	second, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("second instantiate: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("both objects have identity %d", first.ID)
	}
	firstMachine, ok := first.ExhibitedState()
	if !ok {
		t.Fatal("first object exhibits no machine")
	}
	secondMachine, ok := second.ExhibitedState()
	if !ok {
		t.Fatal("second object exhibits no machine")
	}
	if firstMachine.State == secondMachine.State {
		t.Error("both objects share one machine execution")
	}
	for _, obj := range []*Instance{first, second} {
		fv, err := obj.GetFeatureValue(ctx, "lit")
		if err != nil {
			t.Fatalf("lit of object #%d: %v", obj.ID, err)
		}
		if fv.HeldValue().Const.Int != 1 {
			t.Errorf("lit of object #%d = %v, want 1", obj.ID, fv.HeldValue().Const)
		}
	}
}
