package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessTypedWrites covers writes refused by the target's type or multiplicity:
// wrong-typed, repeated, surplus and valueless values.
func TestRuntimeRobustnessTypedWrites(t *testing.T) {
	t.Run("write_of_a_wrong_typed_value_leaves_the_feature", testWriteOfAWrongTypedValueLeavesTheFeature)
	t.Run("write_of_too_many_values_leaves_the_feature", testWriteOfTooManyValuesLeavesTheFeature)
	t.Run("write_of_a_repeated_value_leaves_the_feature", testWriteOfARepeatedValueLeavesTheFeature)
	t.Run("write_of_no_value_where_one_is_required", testWriteOfNoValueWhereOneIsRequired)
	t.Run("state_entry_write_of_a_wrong_typed_value", testStateEntryWriteOfAWrongTypedValue)
	t.Run("performer_feature_write_of_a_wrong_typed_value", testPerformerFeatureWriteOfAWrongTypedValue)
	t.Run("standalone_action_naming_a_performer_feature", testStandaloneActionNamingAPerformerFeature)
	t.Run("standalone_action_writing_a_performer_feature", testStandaloneActionWritingAPerformerFeature)
	t.Run("standalone_action_naming_this_of_an_unowned_performance", testStandaloneActionNamingThisOfAnUnownedPerformance)
	t.Run("chained_write_of_a_wrong_typed_value", testChainedWriteOfAWrongTypedValue)
	t.Run("action_local_write_of_a_wrong_typed_value", testActionLocalWriteOfAWrongTypedValue)
	t.Run("action_output_write_of_a_wrong_typed_value", testActionOutputWriteOfAWrongTypedValue)
	t.Run("performance_occurrence_write_of_a_wrong_typed_value", testPerformanceOccurrenceWriteOfAWrongTypedValue)
}

// testWriteOfAWrongTypedValueLeavesTheFeature: a value the feature's type does
// not admit is rejected before the write, so the feature keeps what it held.
func testWriteOfAWrongTypedValueLeavesTheFeature(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		part def Rig {
			attribute reading : Real = 0.5;
		}
	}`
	ctx, inst, err := instantiateWithLibraries(t, src, "test::Rig")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	err = inst.SetFeatureValue(ctx, "reading", NewStringValue("not a number"))
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
	for _, want := range []string{"reading", "Real", "not a number"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want it to name %s", err, want)
		}
	}
	fv, err := inst.GetFeatureValue(ctx, "reading")
	if err != nil {
		t.Fatalf("read reading after the rejected write: %v", err)
	}
	if got := fv.HeldValue(); got.Kind != ValConst || got.Const.AsReal() != 0.5 {
		t.Errorf("reading = %v, want the 0.5 it held before the rejected write", FormatValue(got))
	}
}

// testWriteOfTooManyValuesLeavesTheFeature: a written collection the target's
// multiplicity does not admit is rejected, and the feature keeps its values.
func testWriteOfTooManyValuesLeavesTheFeature(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		part def Rig {
			attribute samples : Integer[2] = (1, 2);
		}
	}`
	ctx, inst, err := instantiateWithLibraries(t, src, "test::Rig")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	tooMany := sequenceOf([]Value{constInt(1), constInt(2), constInt(3)})
	if err := inst.SetFeatureValue(ctx, "samples", tooMany); !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("error = %v, want ErrMultiplicityViolation", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "samples")
	if err != nil {
		t.Fatalf("read samples after the rejected write: %v", err)
	}
	if got := len(elementsOf(fv.HeldValue())); got != 2 {
		t.Errorf("samples holds %d value(s), want the 2 it held before the rejected write", got)
	}
}

// testWriteOfARepeatedValueLeavesTheFeature: a repeat written to a unique feature
// is refused after count and type, leaving its value; nonunique takes it, a set drops it.
func testWriteOfARepeatedValueLeavesTheFeature(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		private import Collections::*;
		part def Rig {
			attribute xs : Integer[*] = (1, 2);
			attribute ordered : Integer[*] ordered = (1, 2);
			attribute bounded : Integer[0..2] = (1, 2);
			attribute repeats : Integer[*] nonunique = (1, 1);
			attribute members : Set { :>> elements = (1, 2); }
		}
	}`
	ctx, inst, err := instantiateWithLibraries(t, src, "test::Rig")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	repeated := sequenceOf([]Value{constInt(3), constInt(4), constInt(3)})
	for _, feature := range []string{"xs", "ordered"} {
		err := inst.SetFeatureValue(ctx, feature, repeated)
		if !errors.Is(err, ErrUniquenessViolation) {
			t.Fatalf("%s: error = %v, want ErrUniquenessViolation", feature, err)
		}
		if want := "3 (an Integer) is written at positions 1 and 3 of a unique feature"; !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error = %v, want it to say %q", feature, err, want)
		}
		fv, err := inst.GetFeatureValue(ctx, feature)
		if err != nil {
			t.Fatalf("read %s after the rejected write: %v", feature, err)
		}
		if got := intsOf(t, fv.HeldValue()); !equalInts(got, []int64{1, 2}) {
			t.Errorf("%s = %v, want the (1, 2) it held before the rejected write", feature, got)
		}
	}
	if err := inst.SetFeatureValue(ctx, "bounded", repeated); !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("bounded: error = %v, want ErrMultiplicityViolation before the repeat is judged", err)
	}
	wrongType := sequenceOf([]Value{NewStringValue("a"), NewStringValue("a")})
	if err := inst.SetFeatureValue(ctx, "xs", wrongType); !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("xs: error = %v, want ErrTypeMismatch before the repeat is judged", err)
	}
	if err := inst.SetFeatureValue(ctx, "repeats", repeated); err != nil {
		t.Errorf("repeats: error = %v, want a nonunique feature to take the repeat", err)
	}
	if err := inst.SetFeatureValue(ctx, "ordered", sequenceOf([]Value{constInt(3), constInt(1), constInt(2)})); err != nil {
		t.Fatalf("ordered: error = %v, want distinct values to be written", err)
	}
	if fv, _ := inst.GetFeatureValue(ctx, "ordered"); !equalInts(intsOf(t, fv.HeldValue()), []int64{3, 1, 2}) {
		t.Errorf("ordered = %s, want (3, 1, 2) in the order written", FormatValue(fv.HeldValue()))
	}
	members, err := inst.GetFeatureValue(ctx, "members")
	if err != nil {
		t.Fatalf("members: %v", err)
	}
	set, ok := members.HeldValue().Object()
	if !ok {
		t.Fatalf("members = %s, want a Set object", FormatValue(members.HeldValue()))
	}
	if err := ctx.instances[set].SetFeatureValue(ctx, "elements", repeated); err != nil {
		t.Errorf("members.elements: error = %v, want a set to drop the repeat", err)
	}
	if fv, _ := ctx.instances[set].GetFeatureValue(ctx, "elements"); fv.HeldValue().Kind != ValSet || len(elementsOf(fv.HeldValue())) != 2 {
		t.Errorf("members.elements = %s, want the set {3, 4}", FormatValue(fv.HeldValue()))
	}

	actionSrc := `
	package test {
		private import ScalarValues::*;
		action w {
			attribute xs : Integer[*] ordered = (1, 2);
			attribute n : Integer = 5;
			first start;
			action step { assign xs := (n, 6, n); }
			done;
			succession first start then step;
			succession first step then done;
		}
	}`
	idx, _, actx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, actionSrc))
	if _, err := actx.ExecuteAction(findSymbolByName(idx.DocumentRoot("<test>"), "w", ast.DefAction)); !errors.Is(err, ErrUniquenessViolation) {
		t.Errorf("assign: error = %v, want ErrUniquenessViolation", err)
	}

	calcSrc := `
	package test {
		private import ScalarValues::*;
		private import SequenceFunctions::size;
		calc def Pass { in xs : Integer[*]; return : Integer[*] = xs; }
		calc def Twice { in x : Integer; return : Integer[*] = (x, x); }
		calc def Local { in xs : Integer[*] nonunique; attribute ys : Integer[*] = xs; return : Integer = size(ys); }
		calc def LocalRepeats { in xs : Integer[*] nonunique; attribute ys : Integer[*] nonunique = xs; return : Integer = size(ys); }
	}`
	for name, args := range map[string][]Value{"Pass": {repeated}, "Twice": {constInt(7)}, "Local": {repeated}} {
		if err := calcErrorWithLibraries(t, calcSrc, name, args, 10000); !errors.Is(err, ErrUniquenessViolation) {
			t.Errorf("%s: error = %v, want ErrUniquenessViolation", name, err)
		}
	}
	cidx, _, cctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, calcSrc))
	sym, scope := calcByName(t, cidx.DocumentRoot("<test>"), "test", "LocalRepeats")
	if result, err := invokeValue(cctx, sym, []Value{repeated}, scope); err != nil || FormatTraceValue(result) != "3" {
		t.Errorf("LocalRepeats = %s, %v; want 3: a nonunique local takes the repeat", FormatTraceValue(result), err)
	}
}

// testWriteOfNoValueWhereOneIsRequired: an empty collection written to a feature
// whose lower bound is one is a multiplicity violation, not an unset feature.
func testWriteOfNoValueWhereOneIsRequired(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		part def Rig {
			attribute reading : Real[1] = 0.5;
		}
	}`
	ctx, inst, err := instantiateWithLibraries(t, src, "test::Rig")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if err := inst.SetFeatureValue(ctx, "reading", sequenceOf(nil)); !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("error = %v, want ErrMultiplicityViolation", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "reading")
	if err != nil {
		t.Fatalf("read reading after the rejected write: %v", err)
	}
	if got := fv.HeldValue(); got.Kind != ValConst || got.Const.AsReal() != 0.5 {
		t.Errorf("reading = %v, want the 0.5 it held before the rejected write", FormatValue(got))
	}
}

// testStateEntryWriteOfAWrongTypedValue: a write in a state's entry action is
// judged against the feature it names, however deep in the machine it stands.
func testStateEntryWriteOfAWrongTypedValue(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		part def Rig {
			attribute reading : Real = 0.0;
			exhibit state run {
				entry; then go;
				state go {
					entry action set {
						assign reading := "not a number";
					}
				}
			}
		}
	}`
	_, _, err := instantiateWithLibraries(t, src, "test::Rig")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "Real") {
		t.Errorf("error = %v, want it to name the declared type", err)
	}
}

// testPerformerFeatureWriteOfAWrongTypedValue: a write reaching a feature of the
// object performing the behavior is judged against that feature's declaration.
func testPerformerFeatureWriteOfAWrongTypedValue(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		part def Rig {
			attribute label : String = "a";
			perform action set {
				action step {
					assign label := 7;
				}
				first step;
			}
		}
	}`
	_, _, err := instantiateWithLibraries(t, src, "test::Rig")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "String") {
		t.Errorf("error = %v, want it to name the declared type", err)
	}
}

// A body written on its own resolves in its own namespace, so a name only the
// performing object declares is unresolved at run time as it is to analysis.
func testStandaloneActionNamingAPerformerFeature(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		action def Touch {
			action step {
				assign touched := touched + 1;
			}
			first step;
		}
		part def Host {
			attribute touched : Integer = 0;
			perform action t : Touch;
		}
	}`
	_, _, err := instantiateWithLibraries(t, src, "test::Host")
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("error = %v, want ErrUnresolvedReference", err)
	}
	if !strings.Contains(err.Error(), "touched") {
		t.Errorf("error = %v, want it to name the unresolved feature", err)
	}
}

// The write side refuses the same name the read side refuses, rather than
// reaching into the performing object for a name the body cannot see.
func testStandaloneActionWritingAPerformerFeature(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		action def Touch {
			action step {
				assign touched := 1;
			}
			first step;
		}
		part def Host {
			attribute touched : Integer = 0;
			perform action t : Touch;
		}
	}`
	_, _, err := instantiateWithLibraries(t, src, "test::Host")
	if !errors.Is(err, errPerformerFeatureNotInScope) {
		t.Fatalf("error = %v, want errPerformerFeatureNotInScope", err)
	}
	if !strings.Contains(err.Error(), "touched") {
		t.Errorf("error = %v, want it to name the refused feature", err)
	}
}

// `this` in a body written on its own names the performance itself, so a write
// through it lands on the performance — and is refused when the performed
// action declares no such feature; the performer's like-named feature is not
// reachable through it.
func testStandaloneActionNamingThisOfAnUnownedPerformance(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		action def Touch {
			action step {
				assign this.touched := 1;
			}
			first step;
		}
		part def Host {
			attribute touched : Integer = 0;
			perform action t : Touch;
		}
	}`
	_, _, err := instantiateWithLibraries(t, src, "test::Host")
	if err == nil {
		t.Fatal("expected writing a feature the performance does not declare to fail")
	}
	if !strings.Contains(err.Error(), "touched") {
		t.Errorf("error should name the refused feature, got: %v", err)
	}
}

// testChainedWriteOfAWrongTypedValue: a write through a feature chain answers to
// the declaration of the feature the chain reaches.
func testChainedWriteOfAWrongTypedValue(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		part def Cell {
			attribute mark : Integer = 0;
		}
		part def Rig {
			part cell : Cell;
			exhibit state run {
				entry; then marking;
				state marking {
					entry action set {
						assign cell.mark := "seven";
					}
				}
			}
		}
	}`
	_, _, err := instantiateWithLibraries(t, src, "test::Rig")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "mark") {
		t.Errorf("error = %v, want it to name the feature written", err)
	}
}

// testActionLocalWriteOfAWrongTypedValue: a value a body declares of its own is
// declared with a type, so a write to it answers to that type.
func testActionLocalWriteOfAWrongTypedValue(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		action def Compute {
			out total : Integer;
			action step {
				attribute held : Integer = 0;
				assign held := "seven";
				assign total := held;
			}
			first step;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	_, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Compute"))
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
}

// testActionOutputWriteOfAWrongTypedValue: binding an output binds a feature, so
// the value bound conforms to the type the output was declared with.
func testActionOutputWriteOfAWrongTypedValue(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		action def Compute {
			out total : Integer;
			action step {
				assign total := "seven";
			}
			first step;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	_, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Compute"))
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
}

// testPerformanceOccurrenceWriteOfAWrongTypedValue: an attribute of a performed
// action is a feature of its performance occurrence, and a write to it is judged
// against that feature's declaration.
func testPerformanceOccurrenceWriteOfAWrongTypedValue(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		action def Record {
			attribute sample : Integer = 0;
			action step {
				assign sample := "seven";
			}
			first step;
		}
		part def Logger {
			perform action recording : Record;
		}
	}`
	_, _, err := instantiateWithLibraries(t, src, "test::Logger")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
}
