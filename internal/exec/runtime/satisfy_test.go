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
