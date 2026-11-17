package runtime

import (
	"testing"
)

// checkObjectsModel states checks whose conditions, bindings and subjects construct objects, at
// package level and in a part, beside features the model holds a constructed object in.
const checkObjectsModel = `
package test {
	private import ScalarValues::*;
	part def P { attribute x : Integer = 1; }
	part def Q { part p : P; }
	part def Behaving { attribute x : Integer = 1; state def S { entry; then idle; state idle; } exhibit state s : S; }
	part def Checked { attribute x : Integer = 1; assert constraint { (new P()).x > 0 } }
	calc def Make { return : P = new P(); }
	part def W { ref p : P = new P(); assert constraint ok { p.x > 0 } }
	part held : P = new P();
	part lander : P;
	requirement def R { require constraint { true } }
	requirement def Bound { requirement s[0..1] : R = new R(); }
	requirement def Several { requirement s[*] : R = (new R(), new R()); }
	requirement def Constructed { require constraint { (new P()).x > 0 } }
	requirement def Nested { require constraint { (new Q()).p.x > 0 } }
	requirement def ViaCalc { require constraint { Make().x > 0 } }
	requirement def BehavingObject { require constraint { (new Behaving()).x > 0 } }
	requirement def CheckedObject { require constraint { (new Checked()).x > 0 } }
	requirement def Subject { subject v : P; require constraint { v.x > 0 } }
	requirement def Violated { require constraint { (new P()).x > 5 } }
	requirement def Unevaluable { require constraint { (new P()).x / 0 > 5 } }
	requirement def ReadsHeld { require constraint { held.x > 0 } }
	requirement def Compared { subject v : P; require constraint { (new P()).x >= v.x } }
	constraint def C { (new P()).x > 0 }
	requirement bound : Bound;
	requirement several : Several;
	requirement constructed : Constructed;
	requirement nested : Nested;
	requirement viaCalc : ViaCalc;
	requirement behavingObject : BehavingObject;
	requirement checkedObject : CheckedObject;
	requirement subject : Subject { subject v = new P(); }
	requirement violated : Violated;
	requirement unevaluable : Unevaluable;
	requirement readsHeld : ReadsHeld;
	satisfy Compared by lander;
	part w : W;
	part box {
		requirement bound : Bound;
		requirement several : Several;
		requirement constructed : Constructed;
		requirement nested : Nested;
		requirement viaCalc : ViaCalc;
		requirement behavingObject : BehavingObject;
		requirement checkedObject : CheckedObject;
		requirement subject : Subject { subject v = new P(); }
		requirement violated : Violated;
		requirement unevaluable : Unevaluable;
		requirement readsHeld : ReadsHeld;
		constraint c : C;
	}
}
`

// checkObjectsUsages are the requirement usages checkObjectsModel declares at both levels.
var checkObjectsUsages = []string{
	"bound", "several", "constructed", "nested", "viaCalc", "behavingObject", "checkedObject",
	"subject", "violated", "unevaluable", "readsHeld",
}

// heldAfter runs check once, for what the model comes to hold on a first read, then five times
// more, and reports any object or behavior those leave in the context.
func heldAfter(t *testing.T, ctx *Context, what string, check func() string) {
	t.Helper()
	first := check()
	objects, behaviors := ctx.InstanceCount(), len(ctx.objectBehaviors)
	for range 5 {
		if got := check(); got != first {
			t.Errorf("%s: answered %q, then %q", what, first, got)
		}
	}
	if got := ctx.InstanceCount(); got != objects {
		t.Errorf("%s: five more checks left %d objects in the context", what, got-objects)
	}
	if got := len(ctx.objectBehaviors); got != behaviors {
		t.Errorf("%s: five more checks left %d behaviors in the context", what, got-behaviors)
	}
}

// verdictText words a check's outcome for comparison across repetitions.
func verdictText(result CheckResult, err error) string {
	if err != nil {
		return "error: " + err.Error()
	}
	if result.Holds {
		return "holds"
	}
	return "fails"
}

// A check about no object leaves no object it constructed: a bound subrequirement's values, a
// condition's operand, a subject's binding, the result of a calc, whatever the verdict.
func TestCheckAboutNoObjectReleasesWhatItConstructs(t *testing.T) {
	ctx, idx := libraryShapeContext(t, checkObjectsModel)
	for _, name := range append([]string{"Bound", "Several", "Constructed", "Nested", "ViaCalc", "Violated", "Unevaluable"}, checkObjectsUsages...) {
		sym := oneSymbol(t, idx, "test::"+name)
		heldAfter(t, ctx, name, func() string {
			return verdictText(ctx.CheckRequirementOn(sym, sym.OwnerScope, nil))
		})
	}
	c := oneSymbol(t, idx, "test::C")
	heldAfter(t, ctx, "C", func() string { return verdictText(ctx.CheckConstraintOn(c, c.OwnerScope, nil)) })
	// Only what the model holds stands: the package-level part a check read.
	if got := ctx.InstanceCount(); got != 1 {
		t.Errorf("the checks left %d objects, want the one the model's usage denotes", got)
	}
	lander := mustInstantiate(t, ctx, oneSymbol(t, idx, "test::lander"))
	for _, a := range ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>")) {
		heldAfter(t, ctx, a.Text(), func() string { return verdictText(ctx.CheckSatisfactionOn(a, lander)) })
	}
	if got := ctx.InstanceCount(); got != 2 {
		t.Errorf("the satisfaction's checks left %d objects beside its subject", got-2)
	}
}

// A check on an object, and a read of its result, leave what the object holds and nothing else.
func TestCheckOnObjectReleasesWhatItConstructs(t *testing.T) {
	ctx, idx := libraryShapeContext(t, checkObjectsModel)
	root := idx.DocumentRoot("<test>")
	box := mustInstantiate(t, ctx, oneSymbol(t, idx, "test::box"))
	for _, name := range checkObjectsUsages {
		sym := requirementUsage(t, idx, "test::box::"+name)
		heldAfter(t, ctx, "box."+name, func() string {
			return verdictText(ctx.CheckRequirementOn(sym, box.Type.Scope, box))
		})
		heldAfter(t, ctx, "box."+name+".result", func() string {
			val, err := evalOn(t, ctx, box, name+".result")
			return FormatValue(val) + errText(err)
		})
		heldAfter(t, ctx, "test::box."+name+".result", func() string {
			val, err := evalIn(t, ctx, root, "test::box."+name+".result")
			return FormatValue(val) + errText(err)
		})
	}
	c := memberNamed(t, box.Type, "c")
	heldAfter(t, ctx, "box.c", func() string { return verdictText(ctx.CheckConstraintOn(c, box.Type.Scope, box)) })
	for src, want := range map[string]string{
		"constructed.result": "true", "bound.result": "true", "several.result": "true",
		"subject.result": "true", "violated.result": "false", "c.result": "true",
	} {
		if got := FormatValue(mustEvalOn(t, ctx, box, src)); got != want {
			t.Errorf("%s = %s, want %s", src, got, want)
		}
	}
}

// An object a check constructs into a feature of the model stays the feature's value: the
// reference an object holds, a package-level usage's binding, a bound subrequirement an object holds.
func TestCheckKeepsConstructedObjectsTheModelHolds(t *testing.T) {
	ctx, idx := libraryShapeContext(t, checkObjectsModel)
	root := idx.DocumentRoot("<test>")
	w := mustInstantiate(t, ctx, oneSymbol(t, idx, "test::w"))
	ok := memberNamed(t, oneSymbol(t, idx, "test::W"), "ok")
	heldAfter(t, ctx, "w.ok", func() string { return verdictText(ctx.CheckConstraintOn(ok, ok.OwnerScope, w)) })
	held := mustEvalOn(t, ctx, w, "p")
	if _, live := ctx.Instance(held.Instance); held.Kind != ValInstance || !live {
		t.Fatalf("w.p = %s after its check, want the object the check constructed for it", FormatValue(held))
	}
	heldAfter(t, ctx, "w.ok", func() string { return verdictText(ctx.CheckConstraintOn(ok, ok.OwnerScope, w)) })
	if again := mustEvalOn(t, ctx, w, "p"); again.Instance != held.Instance {
		t.Errorf("w.p = %s after more checks, want %s still", FormatValue(again), FormatValue(held))
	}

	reads := oneSymbol(t, idx, "test::readsHeld")
	heldAfter(t, ctx, "readsHeld", func() string { return verdictText(ctx.CheckRequirementOn(reads, reads.OwnerScope, nil)) })
	bound := mustEvalIn(t, ctx, root, "test::held")
	if _, live := ctx.Instance(bound.Instance); bound.Kind != ValInstance || !live {
		t.Fatalf("test::held = %s after the check that bound it, want a standing object", FormatValue(bound))
	}

	box := mustInstantiate(t, ctx, oneSymbol(t, idx, "test::box"))
	sym := requirementUsage(t, idx, "test::box::bound")
	heldAfter(t, ctx, "box.bound", func() string { return verdictText(ctx.CheckRequirementOn(sym, box.Type.Scope, box)) })
	sub := mustEvalOn(t, ctx, box, "bound.s")
	if _, live := ctx.Instance(sub.Instance); sub.Kind != ValInstance || !live {
		t.Errorf("box.bound.s = %s, want the requirement object the usage holds", FormatValue(sub))
	}
}

// errText is err's text after a separator, "" for none.
func errText(err error) string {
	if err == nil {
		return ""
	}
	return " / " + err.Error()
}
