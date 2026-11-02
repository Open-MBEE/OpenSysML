package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessVerificationReturnReads exercises the failure modes of a
// verification's returned expression reading a package-level requirement usage as
// an object: a requirement nothing declares, one whose subject nothing binds, a
// constraint of it called with arguments, and a member of it that is no function.
// Each is a typed error — the run's or its error verdict's — never a panic.
func TestRuntimeRobustnessVerificationReturnReads(t *testing.T) {
	for _, tc := range []struct {
		name, condition string
		want            error
	}{
		{"missing_requirement_is_unresolved", "allTrue(missingRequirement.assumptions())", ErrUnresolvedReference},
		{"unbound_requirement_subject_has_no_value", "unboundRequirement.nonnegativeFuel.result", ErrNoValue},
		{"constraint_called_with_arguments_is_an_arity_error", "allTrue(fullTankRequirement.assumptions(1))", errCalcArity},
		{"calling_a_member_that_is_no_function", "fullTankRequirement.vehicle()", errNotAFunction},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, verdict, err := verificationReturnRun(t, tc.condition)
			if err == nil {
				if verdict.Kind != verdictError {
					t.Fatalf("verdict = %s (%q); want an error", verdict.Kind, verdict.Detail)
				}
				err = verdict.Err
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v; want one wrapping %v", err, tc.want)
			}
		})
	}
	t.Run("probe_leaves_no_object_standing", func(t *testing.T) {
		ctx, verdict, err := verificationReturnRun(t, "allTrue(fullTankRequirement.assumptions())")
		if err != nil || verdict.Kind != VerdictPass {
			t.Fatalf("verdict = %s (%q), err = %v; want pass", verdict.Kind, verdict.Detail, err)
		}
		for id, inst := range ctx.instances {
			if name := ctx.QualifiedName(inst.Type); strings.HasSuffix(name, "Requirement") {
				t.Errorf("object #%d of %s stands after the run; the probe should take it back", id, name)
			}
		}
	})
}

// verificationReturnModel is the reporter's model with the verification's returned
// expression left to each case: a requirement bound to a part, and one bound to nothing.
const verificationReturnModel = `package test {
	private import ScalarValues::Integer;
	private import ControlFunctions::allTrue;
	private import VerificationCases::VerdictKind;
	private import VerificationCases::PassIf;
	part def Vehicle { attribute fuelMass : Integer; attribute fuelFullMass : Integer; }
	part vehicleUnderTest : Vehicle { attribute redefines fuelMass = -200; attribute redefines fuelFullMass = -200; }
	requirement def FuelRequirement {
		subject vehicle : Vehicle;
		assume constraint nonnegativeFuel { vehicle.fuelMass >= 0 }
		require constraint { vehicle.fuelMass <= 2000 }
	}
	requirement fullTankRequirement : FuelRequirement {
		subject vehicle : Vehicle = vehicleUnderTest;
		assume constraint fullTank { vehicle.fuelMass == vehicle.fuelFullMass }
	}
	requirement unboundRequirement : FuelRequirement;
	verification check {
		subject vehicle = vehicleUnderTest;
		objective { verify fullTankRequirement; }
		return verdict : VerdictKind = if CONDITION ? PassIf(fullTankRequirement()) else VerdictKind::inconclusive;
	}
}`

// verificationReturnRun runs the verification with condition as the returned
// expression's test, answering the run's context, its verdict and its error.
func verificationReturnRun(t *testing.T, condition string) (*Context, VerificationVerdict, error) {
	t.Helper()
	src := strings.Replace(verificationReturnModel, "CONDITION", condition, 1)
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := oneSymbol(t, idx, "test::check")
	result, err := ctx.RunVerification(sym, AnalysisArgs{}, sym.OwnerScope, nil)
	return ctx, result.Verdict, err
}
