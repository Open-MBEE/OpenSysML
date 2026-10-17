package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessFeatureValues covers feature values that cannot be derived: cycles,
// conflicting redefinitions, features valued under two names and shared defaults.
func TestRuntimeRobustnessFeatureValues(t *testing.T) {
	t.Run("cyclic_derived_feature_value", testCyclicDerivedFeatureValue)
	t.Run("write_into_cyclic_derived_feature_values", testWriteIntoCyclicDerivedFeatureValues)
	t.Run("cyclic_subsetting_of_default_collections", testCyclicSubsettingOfDefaultCollections)
	t.Run("derived_feature_value_over_missing_feature", testDerivedFeatureValueOverMissingFeature)
	t.Run("deep_specialization_chain_of_redefinitions", testDeepSpecializationChainOfRedefinitions)
	t.Run("conflicting_redefinitions_at_several_levels", testConflictingRedefinitionsAtSeveralLevels)
	t.Run("one_feature_valued_under_two_names", testOneFeatureValuedUnderTwoNames)
	t.Run("valued_feature_restated_in_a_body", testValuedFeatureRestatedInABody)
	t.Run("shared_default_over_a_cyclic_derivation", testSharedDefaultOverACyclicDerivation)
	t.Run("shared_default_taken_over_a_write_that_then_fails", testSharedDefaultTakenOverAWriteThatThenFails)
}

// A `=` value defined in terms of itself fails on every occurrence of the shape
// with the same typed error, and the failure is never shared as a value.
func testSharedDefaultOverACyclicDerivation(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<shared-cycle>", parseAndBuild(t, `package P {
		part def Sat {
			attribute a = b + 1;
			attribute b = a + 1;
		}
		part def Fleet { part sats : Sat[3]; }
	}`))
	ctx.setSharedDefaults(true)
	fleet, err := ctx.Instantiate(oneSymbol(t, idx, "P::Fleet"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	sats, err := fleet.GetFeatureValue(ctx, "sats")
	if err != nil {
		t.Fatalf("sats: %v", err)
	}
	var first string
	for i, held := range elementsOf(sats.HeldValue()) {
		id, _ := held.Object()
		sat, _ := ctx.Instance(id)
		_, err := sat.GetFeatureValue(ctx, "a")
		if !errors.Is(err, ErrCyclicFeatureValue) {
			t.Fatalf("sats#(%d).a: got %v, want %v", i+1, err, ErrCyclicFeatureValue)
		}
		if i == 0 {
			first = err.Error()
		} else if err.Error() != first {
			t.Errorf("sats#(%d).a fails as %q, the first as %q", i+1, err, first)
		}
	}
	if taken := ctx.sharedDefaultsTaken(); taken != 0 {
		t.Errorf("%d shared defaults taken from a derivation that never produced a value", taken)
	}
}

// A write under an occurrence that took a shared value re-derives it on that
// occurrence alone: a write the derivation cannot then use fails as a typed error
// there, while the other occurrences keep the shape's value.
func testSharedDefaultTakenOverAWriteThatThenFails(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<shared-then-fails>", parseAndBuild(t, `package P {
		part def Sat {
			attribute d = 2;
			attribute q = 10 / d;
		}
		part def Fleet { part sats : Sat[2]; }
	}`))
	ctx.setSharedDefaults(true)
	fleet, err := ctx.Instantiate(oneSymbol(t, idx, "P::Fleet"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	sats, err := fleet.GetFeatureValue(ctx, "sats")
	if err != nil {
		t.Fatalf("sats: %v", err)
	}
	held := elementsOf(sats.HeldValue())
	sat := func(i int) *Instance {
		id, _ := held[i].Object()
		inst, _ := ctx.Instance(id)
		return inst
	}
	for i := range held {
		fv, err := sat(i).GetFeatureValue(ctx, "q")
		if err != nil {
			t.Fatalf("sats#(%d).q: %v", i+1, err)
		}
		if got := FormatValue(fv.Value); got != "5.0" {
			t.Fatalf("sats#(%d).q = %s, want 5.0", i+1, got)
		}
	}
	if taken := ctx.sharedDefaultsTaken(); taken != 1 {
		t.Fatalf("shared defaults taken = %d, want 1", taken)
	}
	if err := sat(1).SetFeatureValue(ctx, "d", integerValue(0)); err != nil {
		t.Fatalf("sats#(2).d = 0: %v", err)
	}
	if _, err := sat(1).GetFeatureValue(ctx, "q"); err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("sats#(2).q after d = 0: got %v, want a division by zero", err)
	}
	fv, err := sat(0).GetFeatureValue(ctx, "q")
	if err != nil {
		t.Fatalf("sats#(1).q after the write to sats#(2): %v", err)
	}
	if got := FormatValue(fv.Value); got != "5.0" {
		t.Errorf("sats#(1).q = %s after the write to sats#(2), want 5.0", got)
	}
}

// testCyclicDerivedFeatureValue: two derived defaults that read each other are reported
// as a cycle instead of recursing until the step budget runs out.
func testCyclicDerivedFeatureValue(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part def Loop {
				attribute a = b + 1.0;
				attribute b = a + 1.0;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Loop", ast.DefPart)
	if sym == nil {
		t.Fatal("Loop part def not found")
	}

	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	done := make(chan struct{})
	var fvErr error
	go func() {
		defer close(done)
		_, fvErr = inst.GetFeatureValue(ctx, "a")
	}()
	select {
	case <-done:
	case <-watchdog(5 * time.Second):
		t.Fatal("GetFeatureValue hung on a cyclic derived feature value")
	}

	if !errors.Is(fvErr, ErrCyclicFeatureValue) {
		t.Fatalf("GetFeatureValue error = %v, want ErrCyclicFeatureValue", fvErr)
	}
}

// testWriteIntoCyclicDerivedFeatureValues: a write into a pair of values derived
// from each other neither hangs unmaterializing what read them nor unmaterializes
// the written value; it breaks the cycle, and a later write to the other end
// keeps the first write.
func testWriteIntoCyclicDerivedFeatureValues(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part def Loop {
				attribute a = b + 1.0;
				attribute b = a + 1.0;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Loop", ast.DefPart)
	if sym == nil {
		t.Fatal("Loop part def not found")
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if _, err := inst.GetFeatureValue(ctx, "a"); !errors.Is(err, ErrCyclicFeatureValue) {
		t.Fatalf("GetFeatureValue(a) error = %v, want ErrCyclicFeatureValue", err)
	}

	done := make(chan error, 1)
	go func() { done <- inst.SetFeatureValue(ctx, "b", constReal(1)) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("SetFeatureValue(b): %v", err)
		}
	case <-watchdog(5 * time.Second):
		t.Fatal("SetFeatureValue(b) hung unmaterializing a cycle of derived values")
	}
	if b := inst.FeatureValues["b"]; !b.Materialized || !b.Written {
		t.Fatalf("the write to b left it materialized %t, written %t", b.Materialized, b.Written)
	}
	fv, err := inst.GetFeatureValue(ctx, "a")
	if err != nil || !valueIdentical(fv.HeldValue(), constReal(2)) {
		t.Fatalf("a after b := 1.0 = %v, %v; want 2.0", fv, err)
	}

	if err := inst.SetFeatureValue(ctx, "a", constReal(5)); err != nil {
		t.Fatalf("SetFeatureValue(a): %v", err)
	}
	fv, err = inst.GetFeatureValue(ctx, "b")
	if err != nil || !valueIdentical(fv.HeldValue(), constReal(1)) {
		t.Fatalf("b after a := 5.0 = %v, %v; want the written 1.0", fv, err)
	}
}

// testCyclicSubsettingOfDefaultCollections: two `default null` collections that
// subset each other are reported as a cycle when either is read, rather than
// populating each other until the stack runs out.
func testCyclicSubsettingOfDefaultCollections(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part def Loop {
				part xs : Loop [*] :> ys default null;
				part ys : Loop [*] :> xs default null;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Loop", ast.DefPart)
	if sym == nil {
		t.Fatal("Loop part def not found")
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	for _, name := range []string{"xs", "ys"} {
		done := make(chan struct{})
		var fvErr error
		go func() {
			defer close(done)
			_, fvErr = inst.GetFeatureValue(ctx, name)
		}()
		select {
		case <-done:
		case <-watchdog(5 * time.Second):
			t.Fatalf("GetFeatureValue(%s) hung on collections subsetting each other", name)
		}
		if !errors.Is(fvErr, ErrCyclicFeatureValue) {
			t.Fatalf("GetFeatureValue(%s) error = %v, want ErrCyclicFeatureValue", name, fvErr)
		}
	}
}

// testDerivedFeatureValueOverMissingFeature: a derived default that names something the
// instance does not have fails with the feature value named, rather than silently
// leaving the feature value empty.
func testDerivedFeatureValueOverMissingFeature(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part def Broken {
				attribute derived = missing * 2.0;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Broken", ast.DefPart)
	if sym == nil {
		t.Fatal("Broken part def not found")
	}

	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	_, err = inst.GetFeatureValue(ctx, "derived")
	if err == nil {
		t.Fatal("GetFeatureValue succeeded on a default over an undeclared feature")
	}
	if !strings.Contains(err.Error(), "derived") {
		t.Errorf("error %q does not name the feature value", err)
	}
}

// testDeepSpecializationChainOfRedefinitions: a redefinition specializes the
// usage it redefines, so a long chain of them keeps every level's values and
// terminates instead of recursing while looking for the base's members.
func testDeepSpecializationChainOfRedefinitions(t *testing.T) {
	const depth = 60
	var b strings.Builder
	b.WriteString("package test {\n")
	b.WriteString("\tpart def Inner { attribute a; attribute b; }\n")
	b.WriteString("\tpart def Outer { part inner : Inner; attribute t = inner.b; }\n")
	b.WriteString("\tpart level0 : Outer { part :>> inner { attribute :>> b = 7.0; } }\n")
	for i := 1; i <= depth; i++ {
		fmt.Fprintf(&b, "\tpart level%d :> level%d { part :>> inner { attribute :>> a = %d.0; } }\n", i, i-1, i)
	}
	b.WriteString("}\n")

	done := make(chan struct{})
	var got Value
	var err error
	go func() {
		defer close(done)
		got, err = variationFeatureValueInSource(t, b.String(), fmt.Sprintf("test::level%d", depth), "t")
	}()
	select {
	case <-done:
	case <-watchdog(30 * time.Second):
		t.Fatal("reading an inherited value through a deep specialization chain hung")
	}
	if err != nil {
		t.Fatalf("t = %v", err)
	}
	if got.Kind != ValConst || got.Const.AsReal() != 7.0 {
		t.Errorf("t = %+v, want the base's 7.0", got)
	}
}

// testConflictingRedefinitionsAtSeveralLevels: when several levels restate the
// same nested feature, the innermost restatement is the value read, and the
// levels above still supply what they alone declare.
func testConflictingRedefinitionsAtSeveralLevels(t *testing.T) {
	src := `
		package test {
			part def Inner { attribute a; attribute b; attribute c; }
			part def Outer { part inner : Inner; attribute t = inner.c + inner.b + inner.a; }
			part base : Outer { part :>> inner { attribute :>> a = 1.0; attribute :>> c = 100.0; } }
			part middle :> base { part :>> inner { attribute :>> b = 20.0; attribute :>> c = 200.0; } }
			part leaf :> middle { part :>> inner { attribute :>> c = 300.0; } }
		}`
	got, err := variationFeatureValueInSource(t, src, "test::leaf", "t")
	if err != nil {
		t.Fatalf("t = %v", err)
	}
	if got.Kind != ValConst || got.Const.AsReal() != 321.0 {
		t.Errorf("t = %+v, want 321.0 (innermost c, middle b, base a)", got)
	}
}

// testOneFeatureValuedUnderTwoNames: a redefinition renames one feature, so a
// declaration valuing both names has to be reported instead of picking one.
func testOneFeatureValuedUnderTwoNames(t *testing.T) {
	src := `
		package test {
			part def Ring { attribute ringCost; }
			part def Band :> Ring { attribute bandCost :>> ringCost; }
			part conflicted : Band {
				attribute :>> bandCost = 400.0;
				attribute :>> Ring::ringCost = 500.0;
			}
		}`
	got, err := variationFeatureValueInSource(t, src, "test::conflicted", "bandCost")
	if !errors.Is(err, errConflictingRedefinition) {
		t.Fatalf("ringCost = %+v, err = %v, want errConflictingRedefinition", got, err)
	}
}

// testValuedFeatureRestatedInABody: a feature bound to a value takes its own
// features from that value, so a body restating one of them is reported instead
// of being dropped.
func testValuedFeatureRestatedInABody(t *testing.T) {
	src := `
		package test {
			attribute def Cost { attribute v = 1.0; }
			part def Ring { attribute ringCost : Cost; }
			part conflicted : Ring {
				attribute :>> ringCost = 400.0 { attribute :>> v = 9.0; }
			}
		}`
	got, err := variationFeatureValueInSource(t, src, "test::conflicted", "ringCost")
	if !errors.Is(err, errValuedFeatureRestated) {
		t.Fatalf("ringCost = %+v, err = %v, want errValuedFeatureRestated", got, err)
	}
}
