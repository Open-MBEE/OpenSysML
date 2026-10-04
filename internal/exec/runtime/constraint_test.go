package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestConstraintEvaluation_Assert(t *testing.T) {
	src := `
		package test {
			constraint PositiveValue {
				value > 0
			}
		}
	`

	// Parse
	file := parser.New(source.New("test.sysml", []byte(src))).ParseFile()

	// Build symbol index
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", file)

	// Create resolver and semantic model
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)

	// Create runtime context
	ctx := NewContext(typedModel(model, resolver), 10000)

	// Resolve constraint
	rootScope := idx.DocumentRoot("test.sysml")
	testPkg := rootScope.Children()[0]
	constraintSym, ok := testPkg.LookupLocal("PositiveValue")
	if !ok {
		t.Fatal("PositiveValue constraint not found")
	}

	// Note: This test will fail because 'value' is unbound
	// In real usage, constraints are evaluated with bindings
	_, err := ctx.EvaluateConstraint(constraintSym, testPkg)
	if err == nil {
		t.Fatal("Expected error for unbound 'value'")
	}

	if !errors.Is(err, ErrUnresolvedReference) {
		t.Errorf("err = %v; want it to be an unresolved reference", err)
	}
}

func TestConstraintEvaluation_AssertWithLiteral(t *testing.T) {
	src := `
		package test {
			constraint AlwaysTrue {
				5 > 3
			}
			
			constraint AlwaysFalse {
				2 > 10
			}
		}
	`

	// Parse
	file := parser.New(source.New("test.sysml", []byte(src))).ParseFile()

	// Build symbol index
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", file)

	// Create resolver and semantic model
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)

	// Create runtime context
	ctx := NewContext(typedModel(model, resolver), 10000)

	// Resolve constraints
	rootScope := idx.DocumentRoot("test.sysml")
	testPkg := rootScope.Children()[0]

	// Test AlwaysTrue
	alwaysTrue, ok := testPkg.LookupLocal("AlwaysTrue")
	if !ok {
		t.Fatal("AlwaysTrue not found")
	}

	satisfied, err := ctx.EvaluateConstraint(alwaysTrue, testPkg)
	if err != nil {
		t.Fatalf("AlwaysTrue evaluation failed: %v", err)
	}
	if !satisfied {
		t.Fatal("AlwaysTrue should be satisfied")
	}
	t.Logf("✓ AlwaysTrue: assertion passed")

	// Test AlwaysFalse
	alwaysFalse, ok := testPkg.LookupLocal("AlwaysFalse")
	if !ok {
		t.Fatal("AlwaysFalse not found")
	}

	_, err = ctx.EvaluateConstraint(alwaysFalse, testPkg)
	if err == nil {
		t.Fatal("AlwaysFalse should fail")
	}
	if !errors.Is(err, ErrViolated) {
		t.Fatalf("Expected a violation verdict, got: %v", err)
	}
	t.Logf("✓ AlwaysFalse: assertion failed (as expected)")
}

func TestConstraintEvaluation_Assume(t *testing.T) {
	src := `
		package test {
			constraint WithAssumption {
				assume constraint { 1 > 5 }  // false assumption, but should pass
			}
		}
	`

	// Parse
	file := parser.New(source.New("test.sysml", []byte(src))).ParseFile()

	// Build symbol index
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", file)

	// Create resolver and semantic model
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)

	// Create runtime context
	ctx := NewContext(typedModel(model, resolver), 10000)

	// Resolve constraint
	rootScope := idx.DocumentRoot("test.sysml")
	testPkg := rootScope.Children()[0]
	constraintSym, ok := testPkg.LookupLocal("WithAssumption")
	if !ok {
		t.Fatal("WithAssumption not found")
	}

	// Evaluate - should pass even though assumption is false
	satisfied, err := ctx.EvaluateConstraint(constraintSym, testPkg)
	if err != nil {
		t.Fatalf("Assumption should not fail: %v", err)
	}
	if !satisfied {
		t.Fatal("Constraint with assumption should be satisfied")
	}
	t.Logf("✓ Assumption passed (false assumptions are trusted)")
}

func TestConstraintEvaluation_Negation(t *testing.T) {
	src := `
		package test {
			constraint NotNegative {
				not (3 < 0)
			}
		}
	`

	// Parse
	file := parser.New(source.New("test.sysml", []byte(src))).ParseFile()

	// Build symbol index
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", file)

	// Create resolver and semantic model
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)

	// Create runtime context
	ctx := NewContext(typedModel(model, resolver), 10000)

	// Resolve constraint
	rootScope := idx.DocumentRoot("test.sysml")
	testPkg := rootScope.Children()[0]
	constraintSym, ok := testPkg.LookupLocal("NotNegative")
	if !ok {
		t.Fatal("NotNegative not found")
	}

	// Evaluate - assert not (3 < 0) → assert not false → assert true → pass
	satisfied, err := ctx.EvaluateConstraint(constraintSym, testPkg)
	if err != nil {
		t.Fatalf("Negated assertion failed: %v", err)
	}
	if !satisfied {
		t.Fatal("Negated assertion should pass")
	}
	t.Logf("✓ Negated assertion passed")
}

// A constraint with nothing to check has no verdict: reporting one would claim
// a check that never ran.
func TestConstraintWithoutConditionsIsNotAVerdict(t *testing.T) {
	src := `
		package test {
			constraint def Empty { }
			part def Rig {
				constraint nothing : Empty;
			}
		}
	`
	file := parser.New(source.New("test.sysml", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", file)
	resolver := resolve.New(idx)
	ctx := NewContext(typedModel(semantics.NewModel(resolver), resolver), 10000)

	testPkg := idx.DocumentRoot("test.sysml").Children()[0]
	rig, ok := testPkg.LookupLocal("Rig")
	if !ok {
		t.Fatal("Rig not found")
	}
	feat := featureNamed(ctx, rig, "nothing")
	if feat == nil || feat.Symbol == nil {
		t.Fatal("constraint feature not found")
	}

	satisfied, err := ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), nil)
	if !errors.Is(err, ErrNoConditions) {
		t.Fatalf("err = %v, want ErrNoConditions", err)
	}
	if satisfied {
		t.Error("an unevaluated constraint reported as satisfied")
	}
}

func TestConstraintBodyStepsRun(t *testing.T) {
	// The body's statements are steps of one performance, run before its
	// conditions are evaluated: the assignment makes the condition hold. A
	// write of the constrained part's feature is refused — the implicit target
	// of an assignment is the constraint's own performance (SysML v2 §7.17.9).
	src := `
		package test {
			constraint def Reassigned {
				attribute y = 1;
				assign y := 10;
				y > 5
			}
			part def Rig {
				attribute z = 1;
				constraint branched { if true { assign z := 10; } z > 5 }
				constraint failedFirst { z > 100; assign z := 200; z > 5 }
				assert not constraint denied { z > 100; assign z := 200; z > 5 }
				assert constraint grouped { z > 100; assert not constraint { assign z := 200; z > 5 } }
			}
		}
	`
	file := parser.New(source.New("test.sysml", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", file)
	resolver := resolve.New(idx)
	ctx := NewContext(typedModel(semantics.NewModel(resolver), resolver), 10000)

	testPkg := idx.DocumentRoot("test.sysml").Children()[0]
	reassigned, ok := testPkg.LookupLocal("Reassigned")
	if !ok {
		t.Fatal("Reassigned not found")
	}
	satisfied, err := ctx.EvaluateConstraint(reassigned, testPkg)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !satisfied {
		t.Error("a constraint whose steps make its condition hold is not satisfied")
	}

	rig, ok := testPkg.LookupLocal("Rig")
	if !ok {
		t.Fatal("Rig not found")
	}
	for _, name := range []string{"branched", "failedFirst", "denied"} {
		feat := featureNamed(ctx, rig, name)
		if feat == nil || feat.Symbol == nil {
			t.Fatalf("constraint %s not found", name)
		}
		satisfied, err := ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), nil)
		if !errors.Is(err, ErrConstraintExternalAssignment) {
			t.Errorf("%s: err = %v, want ErrConstraintExternalAssignment", name, err)
		}
		if want := "z is not one of its features"; err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want it to say %s", name, err, want)
		}
		if satisfied {
			t.Errorf("%s: reported as satisfied with its refused write", name)
		}
	}

	// A required condition failing first reports the violation: the nested
	// body's steps are never reached.
	feat := featureNamed(ctx, rig, "grouped")
	if feat == nil || feat.Symbol == nil {
		t.Fatal("constraint grouped not found")
	}
	satisfied, err = ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), nil)
	var violation *ViolationError
	if !errors.As(err, &violation) {
		t.Fatalf("grouped: err = %v, want a *ViolationError", err)
	}
	if violation.Condition != "z > 100" {
		t.Errorf("grouped: violated condition = %q, want z > 100", violation.Condition)
	}
	if satisfied {
		t.Error("grouped: reported as satisfied")
	}
}

func TestConstraintBodyPerformIsRefused(t *testing.T) {
	// A performed action is a usage, not a statement node, and it is an effect
	// outside the constraint's own performance, which a verdict refuses.
	src := `
		package test {
			action def Bump { inout n; assign n := n + 10; }
			constraint def Performed {
				attribute y = 1;
				perform action bump : Bump { inout n = y; }
				y > 5
			}
			part def Rig {
				attribute z = 1;
				action bump : Bump { inout n = z; }
				constraint shorthand { perform bump; z > 5 }
				constraint nested { assert constraint { perform bump; z > 5 } }
				requirement required { require constraint { perform bump; z > 5 } }
			}
		}
	`
	file := parser.New(source.New("test.sysml", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", file)
	resolver := resolve.New(idx)
	ctx := NewContext(typedModel(semantics.NewModel(resolver), resolver), 10000)

	testPkg := idx.DocumentRoot("test.sysml").Children()[0]
	performed, ok := testPkg.LookupLocal("Performed")
	if !ok {
		t.Fatal("Performed not found")
	}
	satisfied, err := ctx.EvaluateConstraint(performed, testPkg)
	if !errors.Is(err, ErrConstraintEffect) {
		t.Fatalf("err = %v, want ErrConstraintEffect", err)
	}
	if want := "`perform` statement"; err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v, want it to name the %s", err, want)
	}
	if satisfied {
		t.Error("a constraint whose performed action was skipped reported as satisfied")
	}

	rig, ok := testPkg.LookupLocal("Rig")
	if !ok {
		t.Fatal("Rig not found")
	}
	for _, name := range []string{"shorthand", "nested", "required"} {
		feat := featureNamed(ctx, rig, name)
		if feat == nil || feat.Symbol == nil {
			t.Fatalf("%s not found", name)
		}
		evaluate := ctx.EvaluateConstraintOn
		if name == "required" {
			evaluate = ctx.EvaluateRequirementOn
		}
		satisfied, err := evaluate(feat.Symbol, feat.DeclScope(), nil)
		if !errors.Is(err, ErrConstraintEffect) {
			t.Errorf("%s: err = %v, want ErrConstraintEffect", name, err)
		}
		if want := "`perform` statement"; err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want it to name the %s", name, err, want)
		}
		if satisfied {
			t.Errorf("%s: reported as satisfied with its performed action skipped", name)
		}
	}
}

func TestConstraintBodyActionFlowIsRefused(t *testing.T) {
	// Action nodes perform actions — an effect outside the performance — and
	// the successions between them are a stated flow a verdict does not order.
	src := `
		package test {
			constraint def Flowed {
				attribute y = 1;
				action a; action b;
				first a then b;
				y > 5
			}
			part def Rig {
				attribute z = 1;
				constraint edge { action a; action b; first a then b; z > 5 }
				constraint attached { action a then b; action b; z > 5 }
				constraint named { action a; action b; succession s first a then b; z > 5 }
				constraint node { action a; action b; fork f; first a then f; first f then b; z > 5 }
				constraint plain { action a; z > 5 }
				constraint nested { assert constraint { action a; action b; first a then b; z > 5 } }
				requirement required { require constraint { action a; action b; first a then b; z > 5 } }
			}
		}
	`
	file := parser.New(source.New("test.sysml", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", file)
	resolver := resolve.New(idx)
	ctx := NewContext(typedModel(semantics.NewModel(resolver), resolver), 10000)

	testPkg := idx.DocumentRoot("test.sysml").Children()[0]
	flowed, ok := testPkg.LookupLocal("Flowed")
	if !ok {
		t.Fatal("Flowed not found")
	}
	satisfied, err := ctx.EvaluateConstraint(flowed, testPkg)
	if !errors.Is(err, ErrConstraintEffect) {
		t.Fatalf("err = %v, want ErrConstraintEffect", err)
	}
	if want := "`perform` statement"; err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v, want it to name the %s", err, want)
	}
	if satisfied {
		t.Error("a constraint whose action flow was skipped reported as satisfied")
	}

	rig, ok := testPkg.LookupLocal("Rig")
	if !ok {
		t.Fatal("Rig not found")
	}
	for _, name := range []string{"edge", "attached", "named", "node", "plain", "nested", "required"} {
		feat := featureNamed(ctx, rig, name)
		if feat == nil || feat.Symbol == nil {
			t.Fatalf("%s not found", name)
		}
		evaluate := ctx.EvaluateConstraintOn
		if name == "required" {
			evaluate = ctx.EvaluateRequirementOn
		}
		satisfied, err := evaluate(feat.Symbol, feat.DeclScope(), nil)
		if !errors.Is(err, ErrConstraintEffect) {
			t.Errorf("%s: err = %v, want ErrConstraintEffect", name, err)
		}
		if want := "`perform` statement"; err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want it to name the %s", name, err, want)
		}
		if satisfied {
			t.Errorf("%s: reported as satisfied with its action flow skipped", name)
		}
	}
}

func TestConstraintBodySuccessionIsRefused(t *testing.T) {
	// A succession between actions declared outside the body is a stated flow
	// a verdict does not order, and it is named by the keyword written.
	src := `
		package test {
			part def Rig {
				attribute z = 1;
				action a; action b;
				constraint edge { first a then b; z > 5 }
				constraint attached { then b; z > 5 }
				constraint named { succession s first a then b; z > 5 }
				constraint flow { succession flow from a to b; z > 5 }
				constraint final { done; z > 5 }
			}
		}
	`
	file := parser.New(source.New("test.sysml", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", file)
	resolver := resolve.New(idx)
	ctx := NewContext(typedModel(semantics.NewModel(resolver), resolver), 10000)

	testPkg := idx.DocumentRoot("test.sysml").Children()[0]
	rig, ok := testPkg.LookupLocal("Rig")
	if !ok {
		t.Fatal("Rig not found")
	}
	for name, want := range map[string]string{
		"edge":     "`first` statement",
		"attached": "`then` statement",
		"named":    "`succession` statement",
		"flow":     "`succession flow` statement",
		"final":    "`done` statement",
	} {
		feat := featureNamed(ctx, rig, name)
		if feat == nil || feat.Symbol == nil {
			t.Fatalf("%s not found", name)
		}
		satisfied, err := ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), nil)
		if !errors.Is(err, ErrStatementNotExecutable) {
			t.Errorf("%s: err = %v, want ErrStatementNotExecutable", name, err)
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want it to name the %s", name, err, want)
		}
		if satisfied {
			t.Errorf("%s: reported as satisfied with its succession skipped", name)
		}
	}
}
