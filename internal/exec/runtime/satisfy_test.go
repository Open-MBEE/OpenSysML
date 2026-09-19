package runtime

import (
	"errors"
	"strings"
	"testing"
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
