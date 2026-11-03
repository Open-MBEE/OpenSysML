package runtime

import (
	"errors"
	"strings"
	"testing"
)

func TestRequirementEvaluation_RequireWithLiteral(t *testing.T) {
	src := `
		package test {
			requirement SafeSpeed {
				require constraint { 50 < 100 }
			}
			
			requirement UnsafeSpeed {
				require constraint { 150 < 100 }
			}
		}
	`

	ctx, testPkg := constraintPackage(t, src)

	// Test SafeSpeed
	safeSpeed, ok := testPkg.LookupLocal("SafeSpeed")
	if !ok {
		t.Fatal("SafeSpeed not found")
	}

	satisfied, err := ctx.EvaluateRequirement(safeSpeed, testPkg)
	if err != nil {
		t.Fatalf("SafeSpeed evaluation failed: %v", err)
	}
	if !satisfied {
		t.Fatal("SafeSpeed should be satisfied")
	}
	t.Logf("✓ SafeSpeed: requirement satisfied")

	// Test UnsafeSpeed
	unsafeSpeed, ok := testPkg.LookupLocal("UnsafeSpeed")
	if !ok {
		t.Fatal("UnsafeSpeed not found")
	}

	_, err = ctx.EvaluateRequirement(unsafeSpeed, testPkg)
	if err == nil {
		t.Fatal("UnsafeSpeed should fail")
	}
	if !errors.Is(err, ErrViolated) {
		t.Fatalf("Expected a violation verdict, got: %v", err)
	}
	t.Logf("✓ UnsafeSpeed: requirement failed (as expected)")
}

func TestRequirementEvaluation_Assume(t *testing.T) {
	src := `
		package test {
			requirement WithAssumption {
				assume constraint { 1 > 100 }  // false assumption: the implication holds
				require constraint { 5 > 3 }   // true requirement
			}
		}
	`

	ctx, testPkg := constraintPackage(t, src)
	reqSym, ok := testPkg.LookupLocal("WithAssumption")
	if !ok {
		t.Fatal("WithAssumption not found")
	}

	// Evaluate - holds: a false assumption makes `assumptions implies constraints` true
	satisfied, err := ctx.EvaluateRequirement(reqSym, testPkg)
	if err != nil {
		t.Fatalf("Assumption should not cause failure: %v", err)
	}
	if !satisfied {
		t.Fatal("Requirement with assumption should be satisfied")
	}
	t.Logf("✓ Assumption failed, so the requirement holds (assumptions imply constraints)")
}

func TestRequirementEvaluation_SubjectNotFound(t *testing.T) {
	t.Skip("TODO: typed subject validation not yet implemented (only binding form supported)")
	src := `
		package test {
			requirement NeedsSubject {
				subject vehicle : Vehicle;
			}
		}
	`

	ctx, testPkg := constraintPackage(t, src)
	reqSym, ok := testPkg.LookupLocal("NeedsSubject")
	if !ok {
		t.Fatal("NeedsSubject not found")
	}

	// Evaluate - should fail because 'vehicle' not in scope
	_, err := ctx.EvaluateRequirement(reqSym, testPkg)
	if err == nil {
		t.Fatal("Should fail when subject not found")
	}
	if !strings.Contains(err.Error(), "subject 'vehicle' not found") {
		t.Fatalf("Expected subject error, got: %v", err)
	}
	t.Logf("✓ Subject validation: correctly detected missing subject")
}

func TestRequirementEvaluation_Complete(t *testing.T) {
	src := `
		package test {
			part def Vehicle;
			part def Driver;
			
			part car : Vehicle;
			part pilot : Driver;
			
			requirement SafetyReq {
				subject vehicle : Vehicle = car;
				actor driver : Driver = pilot;
				assume constraint { vehicle != null }
				require constraint { driver != null and 50 < 100 }
			}
		}
	`

	ctx, testPkg := constraintPackage(t, src)
	reqSym, ok := testPkg.LookupLocal("SafetyReq")
	if !ok {
		t.Fatal("SafetyReq not found")
	}

	// Evaluate - should pass (subject and actor are bound, assume passes, require is true)
	satisfied, err := ctx.EvaluateRequirement(reqSym, testPkg)
	if err != nil {
		t.Fatalf("Complete requirement evaluation failed: %v", err)
	}
	if !satisfied {
		t.Fatal("Complete requirement should be satisfied")
	}
	t.Logf("✓ Complete requirement: subject + actor + assume + require validated")
}
