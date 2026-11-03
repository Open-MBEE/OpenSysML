package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// satisfyModel is a model whose analysis context states satisfaction
// assertions about two parts, one satisfying the requirement and one not.
const satisfyModel = `
package Landing {
	part def Lander {
		attribute verticalSpeed;
	}

	requirement def TouchdownRequirement {
		subject lander : Lander;
		attribute maxVerticalSpeed;
		require constraint {
			lander.verticalSpeed <= maxVerticalSpeed
		}
	}

	requirement touchdown : TouchdownRequirement {
		attribute :>> maxVerticalSpeed = 1.5;
	}

	part slowLander : Lander {
		attribute :>> verticalSpeed = 1.2;
	}

	part fastLander : Lander {
		attribute :>> verticalSpeed = 2.4;
	}

	part analysisContext {
		assert satisfy touchdown by slowLander;
		assert satisfy touchdown by fastLander;
		assert not satisfy touchdown by fastLander;
	}
}
`

// satisfactionOf returns the assertion of src whose rendered text is want.
func satisfactionOf(t *testing.T, src, want string) (*Context, *SatisfyAssertion) {
	t.Helper()
	ctx, idx := contextForSource(t, src)
	assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>"))
	for _, a := range assertions {
		if a.Text() == want {
			return ctx, a
		}
	}
	texts := make([]string, 0, len(assertions))
	for _, a := range assertions {
		texts = append(texts, a.Text())
	}
	t.Fatalf("no assertion %q among [%s]", want, strings.Join(texts, ", "))
	return nil, nil
}

func TestSatisfactionHoldsWhenSubjectMeetsRequirement(t *testing.T) {
	ctx, a := satisfactionOf(t, satisfyModel, "satisfy touchdown by slowLander")
	satisfied, err := ctx.EvaluateSatisfaction(a)
	if err != nil {
		t.Fatalf("EvaluateSatisfaction: %v", err)
	}
	if !satisfied {
		t.Fatalf("satisfied = false, want true")
	}
}

func TestSatisfactionFailsWhenSubjectViolatesRequirement(t *testing.T) {
	ctx, a := satisfactionOf(t, satisfyModel, "satisfy touchdown by fastLander")
	satisfied, err := ctx.EvaluateSatisfaction(a)
	if satisfied {
		t.Fatalf("satisfied = true, want false")
	}
	var violation *ViolationError
	if !errors.As(err, &violation) {
		t.Fatalf("err = %v, want a *ViolationError", err)
	}
	if !strings.Contains(violation.Condition, "maxVerticalSpeed") {
		t.Errorf("violation names condition %q, want the failing require condition", violation.Condition)
	}
}

// TestSatisfactionSeesRequirementBindings checks that a value the requirement
// binds by name is visible to the conditions of a satisfaction assertion, as it
// is when the requirement is evaluated directly.
func TestSatisfactionSeesRequirementBindings(t *testing.T) {
	ctx, a := satisfactionOf(t, `
		package Landing {
			part def Lander { attribute verticalSpeed; }

			requirement def TouchdownRequirement {
				subject lander : Lander;
				attribute certifiedLimit = 1.5;
				actor operator = certifiedLimit;
				require constraint {
					lander.verticalSpeed <= operator
				}
			}

			requirement touchdown : TouchdownRequirement;

			part slowLander : Lander {
				attribute :>> verticalSpeed = 1.2;
			}

			part analysisContext {
				assert satisfy touchdown by slowLander;
			}
		}
	`, "satisfy touchdown by slowLander")

	satisfied, err := ctx.EvaluateSatisfaction(a)
	if err != nil {
		t.Fatalf("EvaluateSatisfaction: %v", err)
	}
	if !satisfied {
		t.Error("satisfied = false, want true: the actor binding supplies the limit")
	}
}

// TestSatisfactionSubjectBindingWinsOverTheRequirementsOwn checks that the object
// `by` names supplies the subject even when the requirement binds it itself.
func TestSatisfactionSubjectBindingWinsOverTheRequirementsOwn(t *testing.T) {
	ctx, a := satisfactionOf(t, `
		package Landing {
			part def Lander { attribute verticalSpeed; }

			part slowLander : Lander {
				attribute :>> verticalSpeed = 1.2;
			}

			part fastLander : Lander {
				attribute :>> verticalSpeed = 2.4;
			}

			requirement def TouchdownRequirement {
				subject lander : Lander;
				attribute maxVerticalSpeed = 1.5;
				require constraint {
					lander.verticalSpeed <= maxVerticalSpeed
				}
			}

			requirement touchdown : TouchdownRequirement {
				subject = slowLander;
			}

			part analysisContext {
				assert satisfy touchdown by fastLander;
			}
		}
	`, "satisfy touchdown by fastLander")

	satisfied, err := ctx.EvaluateSatisfaction(a)
	if !errors.Is(err, ErrViolated) {
		t.Fatalf("satisfied = %v, err = %v; want ErrViolated: `by fastLander` supplies the subject", satisfied, err)
	}
}

func TestNegatedSatisfactionInvertsTheVerdict(t *testing.T) {
	ctx, a := satisfactionOf(t, satisfyModel, "not satisfy touchdown by fastLander")
	satisfied, err := ctx.EvaluateSatisfaction(a)
	if err != nil {
		t.Fatalf("EvaluateSatisfaction: %v", err)
	}
	if !satisfied {
		t.Fatalf("satisfied = false, want true: the requirement it denies does not hold")
	}
}

func TestReadInSatisfactionReadsThroughTheSubject(t *testing.T) {
	src := `package P {
		private import ScalarValues::*;
		part def Inner { attribute power : Real; }
		part def Sub { part inner : Inner; }
		part def Thing { part sub : Sub; }
		requirement def Deep {
			subject vehicle : Thing;
			require constraint { vehicle.sub.inner.power > 0.0 }
		}
		requirement deep : Deep;
		part craft : Thing {
			part :>> sub {
				part :>> inner { attribute :>> power = 7.0; }
			}
		}
		part analysis { assert satisfy deep by craft; }
	}`
	ctx, a := satisfactionOf(t, src, "satisfy deep by craft")
	subject, err := ctx.SatisfySubject(a)
	if err != nil {
		t.Fatalf("SatisfySubject: %v", err)
	}
	conditions := ctx.ConditionsOf(a.Symbol, a.Symbol.OwnerScope)
	if len(conditions) != 1 || conditions[0].Expr == nil {
		t.Fatalf("conditions = %+v, want one expression", conditions)
	}
	expression, ok := conditions[0].Expr.(*ast.OperatorExpr)
	if !ok || len(expression.Operands) != 2 {
		t.Fatalf("condition expression = %#v, want a binary operator", conditions[0].Expr)
	}
	value, err := ctx.ReadInSatisfaction(a, subject, conditions[0].Scope, expression.Operands[0])
	if err != nil {
		t.Fatalf("ReadInSatisfaction: %v", err)
	}
	// Unresolved without the libraries, Real does not hold the literal as a binary64.
	if value.Kind != ValConst || value.Const.Kind != semantics.ValRational || value.Const.FormatRational() != "7.0" {
		t.Fatalf("ReadInSatisfaction = %+v, want 7.0", value)
	}
}

// TestReadInSatisfactionReadsTheCheckChain checks that a read in a satisfaction sees the
// chain redefinition the satisfy usage declares through its subject, as the check does,
// and that the object reads its own value again once either is over.
func TestReadInSatisfactionReadsTheCheckChain(t *testing.T) {
	src := `package ChainProbe {
		private import ScalarValues::Integer;
		part def Vehicle { attribute fuel : Integer default 0; }
		part vehicle1 : Vehicle;
		requirement def Full {
			subject vehicle : Vehicle;
			attribute target : Integer default 1300;
			require constraint { vehicle.fuel == target }
		}
		satisfy requirement check : Full by vehicle1 {
			attribute redefines vehicle1.fuel = 1300;
		}
	}`
	ctx, a := satisfactionOf(t, src, "satisfy check by vehicle1")
	subject, err := ctx.SatisfySubject(a)
	if err != nil {
		t.Fatalf("SatisfySubject: %v", err)
	}
	conditions := ctx.ConditionsOf(a.Symbol, a.Symbol.OwnerScope)
	expression, ok := conditions[0].Expr.(*ast.OperatorExpr)
	if len(conditions) != 1 || !ok || len(expression.Operands) != 2 {
		t.Fatalf("conditions = %+v, want one binary operator", conditions)
	}
	fuel := func(where string, want string) {
		t.Helper()
		got, err := ctx.readFeatureValue(subject.FeatureValues["fuel"], "fuel")
		if err != nil || FormatValue(got) != want {
			t.Fatalf("%s: vehicle1.fuel = %s, %v; want %s", where, FormatValue(got), err, want)
		}
	}
	fuel("before", "0")
	value, err := ctx.ReadInSatisfaction(a, subject, conditions[0].Scope, expression.Operands[0])
	if err != nil || FormatValue(value) != "1300" {
		t.Fatalf("ReadInSatisfaction = %s, %v; want 1300", FormatValue(value), err)
	}
	fuel("after the read", "0")
	result, err := ctx.CheckSatisfactionOn(a, subject)
	if err != nil || !result.Holds {
		t.Fatalf("CheckSatisfactionOn = %+v, %v; want holds", result, err)
	}
	fuel("after the check", "0")
}

// TestReadInSatisfactionKeepsTheObjectsItMakes checks that a read or a check under a
// chain redefinition keeps the objects it materializes below the subject: a read of a
// lazy part returns an object the context still knows, and only the chain is undone.
func TestReadInSatisfactionKeepsTheObjectsItMakes(t *testing.T) {
	src := `package LazyProbe {
		private import ScalarValues::Integer;
		part def Inner { attribute power : Integer default 0; }
		part def Sub { part inner : Inner; }
		part def Thing { part sub : Sub; }
		part craft : Thing;
		requirement def Deep {
			subject vehicle : Thing;
			require constraint { vehicle.sub.inner.power == 7 }
		}
		satisfy requirement check : Deep by craft {
			attribute redefines craft.sub.inner.power = 7;
		}
	}`
	ctx, a := satisfactionOf(t, src, "satisfy check by craft")
	subject, err := ctx.SatisfySubject(a)
	if err != nil {
		t.Fatalf("SatisfySubject: %v", err)
	}
	conditions := ctx.ConditionsOf(a.Symbol, a.Symbol.OwnerScope)
	expression, ok := conditions[0].Expr.(*ast.OperatorExpr)
	if len(conditions) != 1 || !ok || len(expression.Operands) != 2 {
		t.Fatalf("conditions = %+v, want one binary operator", conditions)
	}
	power, err := ctx.ReadInSatisfaction(a, subject, conditions[0].Scope, expression.Operands[0])
	if err != nil || FormatValue(power) != "7" {
		t.Fatalf("ReadInSatisfaction(power) = %s, %v; want 7", FormatValue(power), err)
	}
	// vehicle.sub.inner.power nests as ((vehicle.sub).inner).power.
	var sub ast.Node = expression.Operands[0]
	for range 2 {
		chain, ok := sub.(*ast.FeatureChainExpr)
		if !ok {
			t.Fatalf("operand = %#v, want a feature chain", sub)
		}
		sub = chain.Operand
	}
	held, err := ctx.ReadInSatisfaction(a, subject, conditions[0].Scope, sub)
	if err != nil {
		t.Fatalf("ReadInSatisfaction(sub): %v", err)
	}
	id, ok := held.Object()
	if !ok {
		t.Fatalf("ReadInSatisfaction(sub) = %s, want an object", FormatValue(held))
	}
	subObject, ok := ctx.Instance(id)
	if !ok {
		t.Fatalf("object %d the read returned is gone", id)
	}
	result, err := ctx.CheckSatisfactionOn(a, subject)
	if err != nil || !result.Holds {
		t.Fatalf("CheckSatisfactionOn = %+v, %v; want holds", result, err)
	}
	if again, _ := subject.FeatureValues["sub"].Value.Object(); again != id {
		t.Fatalf("craft.sub is object %d after the check, was %d", again, id)
	}
	innerID, _ := subObject.FeatureValues["inner"].Value.Object()
	inner, ok := ctx.Instance(innerID)
	if !ok {
		t.Fatalf("craft.sub.inner is gone after the check")
	}
	got, err := ctx.readFeatureValue(inner.FeatureValues["power"], "power")
	if err != nil || FormatValue(got) != "0" {
		t.Fatalf("craft.sub.inner.power after the check = %s, %v; want 0", FormatValue(got), err)
	}
}

// TestSatisfactionOverlayKeepsAWriteMadeUnderIt checks that removing a satisfaction's
// chain overlay reinstalls the feature's own declaration but keeps a value written to the
// feature while the overlay stood, as a condition's step may write it.
func TestSatisfactionOverlayKeepsAWriteMadeUnderIt(t *testing.T) {
	src := `package ChainProbe {
		private import ScalarValues::Integer;
		part def Vehicle { attribute fuel : Integer default 0; }
		part vehicle1 : Vehicle;
		requirement def Full {
			subject vehicle : Vehicle;
			require constraint { vehicle.fuel == 1300 }
		}
		satisfy requirement check : Full by vehicle1 {
			attribute redefines vehicle1.fuel = 1300;
		}
	}`
	ctx, a := satisfactionOf(t, src, "satisfy check by vehicle1")
	subject, err := ctx.SatisfySubject(a)
	if err != nil {
		t.Fatalf("SatisfySubject: %v", err)
	}
	_, bindings, err := ctx.satisfactionBindings(a, a.Symbol, subject)
	if err != nil {
		t.Fatalf("satisfactionBindings: %v", err)
	}
	fv := subject.FeatureValues["fuel"]
	own := fv.Feature
	undo, err := ctx.overlaySatisfierChains(a, subject, bindings)
	if err != nil {
		t.Fatalf("overlaySatisfierChains: %v", err)
	}
	if got, err := ctx.readFeatureValue(fv, "fuel"); err != nil || FormatValue(got) != "1300" {
		t.Fatalf("fuel under the overlay = %s, %v; want 1300", FormatValue(got), err)
	}
	nine := Value{Kind: ValConst, Const: semantics.IntValue(9)}
	if err := subject.storeFeatureValue(ctx, fv, "fuel", nine); err != nil {
		t.Fatalf("write fuel: %v", err)
	}
	if err := undo(); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if fv.Feature != own {
		t.Fatalf("fuel is declared by %v after the overlay, want its own %v", fv.Feature.Symbol, own.Symbol)
	}
	if got, err := ctx.readFeatureValue(fv, "fuel"); err != nil || FormatValue(got) != "9" || !fv.Written {
		t.Fatalf("fuel after the overlay = %s, written %v, %v; want the written 9", FormatValue(got), fv.Written, err)
	}
}

// statedSatisfyModel states satisfactions with no `by`: one by a definition,
// about every object of it, and one by a usage, about the usage's own object.
const statedSatisfyModel = `
package test {
	part def Lander {
		attribute verticalSpeed;
	}

	requirement def Touchdown {
		subject lander : Lander;
		require constraint { lander.verticalSpeed <= 1.5 }
	}
	requirement touchdown : Touchdown;

	part def SafeLander :> Lander {
		satisfy touchdown;
	}
	part slow : SafeLander {
		attribute :>> verticalSpeed = 1.2;
	}
	part fast : SafeLander {
		attribute :>> verticalSpeed = 2.4;
	}

	part gentle : Lander {
		attribute :>> verticalSpeed = 0.5;
		satisfy touchdown;
	}
}
`

// TestSatisfactionStatedByADefinitionIsAboutItsObjects checks that a definition's
// `satisfy r;` is about the objects of the definition the runtime holds, asks for
// one when there is none, and decides each object on its own values.
func TestSatisfactionStatedByADefinitionIsAboutItsObjects(t *testing.T) {
	ctx, idx := contextForSource(t, statedSatisfyModel)
	var byDef *SatisfyAssertion
	for _, a := range ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>")) {
		if typ := a.StatingType(); typ != nil && typ.Name == "SafeLander" {
			byDef = a
		}
	}
	if byDef == nil {
		t.Fatal("no assertion stated by SafeLander")
	}
	if got := ctx.SatisfactionSubjects(byDef, ctx.LiveRoots()); len(got) != 0 {
		t.Fatalf("subjects before any object = %d, want none", len(got))
	}
	_, err := ctx.EvaluateSatisfaction(byDef)
	if !errors.Is(err, errNoSubject) || !strings.Contains(err.Error(), "an object of test::SafeLander is needed") {
		t.Fatalf("evaluated with no object: %v, want ErrNoSubject naming the definition", err)
	}

	slow, err := ctx.Instantiate(oneSymbol(t, idx, "test::slow"))
	if err != nil {
		t.Fatal(err)
	}
	fast, err := ctx.Instantiate(oneSymbol(t, idx, "test::fast"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ctx.Instantiate(oneSymbol(t, idx, "test::gentle")); err != nil {
		t.Fatal(err)
	}
	subjects := ctx.SatisfactionSubjects(byDef, ctx.LiveRoots())
	if len(subjects) != 2 || subjects[0].Object != slow || subjects[1].Object != fast {
		t.Fatalf("subjects = %v, want slow then fast", subjects)
	}
	if subjects[0].Root != slow || len(subjects[0].Path) != 0 {
		t.Errorf("slow reached from %v via %v, want itself", subjects[0].Root, subjects[0].Path)
	}
	if holds, err := ctx.evaluateSatisfactionOn(byDef, slow); err != nil || !holds {
		t.Errorf("slow: holds = %v, err = %v; want true", holds, err)
	}
	if holds, err := ctx.evaluateSatisfactionOn(byDef, fast); holds || !errors.Is(err, ErrViolated) {
		t.Errorf("fast: holds = %v, err = %v; want a violation", holds, err)
	}
}

// TestSatisfactionStatedByAUsageIsAboutItsOwnObject checks that a usage's
// `satisfy r;` instantiates the usage when no object is supplied, as `by p` does.
func TestSatisfactionStatedByAUsageIsAboutItsOwnObject(t *testing.T) {
	ctx, idx := contextForSource(t, statedSatisfyModel)
	gentle := oneSymbol(t, idx, "test::gentle")
	assertions := ctx.SatisfyAssertionsIn(gentle.Scope)
	if len(assertions) != 1 || assertions[0].StatingType() != gentle {
		t.Fatalf("assertions of gentle = %v", assertions)
	}
	inst, err := ctx.SatisfySubject(assertions[0])
	if err != nil || inst.Type != gentle {
		t.Fatalf("subject = %v, %v; want an object of gentle", inst, err)
	}
	if holds, err := ctx.EvaluateSatisfaction(assertions[0]); err != nil || !holds {
		t.Errorf("holds = %v, err = %v; want true", holds, err)
	}
}

// TestSatisfactionStatedByADefinitionSweepsEveryObject checks that the sweep is
// about each object of the definition: a declaration instantiated twice is two
// subjects, and an object classified by the definition since it was made is one.
func TestSatisfactionStatedByADefinitionSweepsEveryObject(t *testing.T) {
	ctx, idx := contextForSource(t, statedSatisfyModel+`
package classified {
	requirement def SafeTouchdown {
		subject lander : test::SafeLander;
		require constraint { lander.verticalSpeed <= 1.5 }
	}
	requirement safe : SafeTouchdown;
	part plain : test::Lander {
		attribute :>> verticalSpeed = 0.9;
	}
	part context {
		assert satisfy safe by plain;
	}
}
`)
	var byDef *SatisfyAssertion
	for _, a := range ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>")) {
		if typ := a.StatingType(); typ != nil && typ.Name == "SafeLander" {
			byDef = a
		}
	}
	if byDef == nil {
		t.Fatal("no assertion stated by SafeLander")
	}
	first, err := ctx.Instantiate(oneSymbol(t, idx, "test::slow"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := ctx.Instantiate(oneSymbol(t, idx, "test::slow"))
	if err != nil {
		t.Fatal(err)
	}
	subjects := ctx.SatisfactionSubjects(byDef, ctx.LiveRoots())
	if len(subjects) != 2 || subjects[0].Object != first || subjects[1].Object != second {
		t.Fatalf("subjects after two objects of slow = %v, want both", subjects)
	}

	// Satisfying safe by plain holds plain as a SafeLander, so it is one of the
	// objects SafeLander's own assertion is about from then on.
	context := oneSymbol(t, idx, "classified::context")
	byPlain := ctx.SatisfyAssertionsIn(context.Scope)
	if len(byPlain) != 1 {
		t.Fatalf("assertions of context = %v", byPlain)
	}
	plain, err := ctx.SatisfySubject(byPlain[0])
	if err != nil {
		t.Fatal(err)
	}
	if holds, err := ctx.evaluateSatisfactionOn(byPlain[0], plain); err != nil || !holds {
		t.Fatalf("safe by plain: holds = %v, err = %v", holds, err)
	}
	subjects = ctx.SatisfactionSubjects(byDef, ctx.LiveRoots())
	if len(subjects) != 3 || subjects[2].Object != plain {
		t.Fatalf("subjects after classifying plain = %v, want slow, slow and plain", subjects)
	}
}
