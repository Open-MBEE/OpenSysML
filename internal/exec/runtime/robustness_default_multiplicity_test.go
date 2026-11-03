package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestRuntimeRobustnessDefaultMultiplicity: a feature that declares and inherits no
// multiplicity has the usual default 0..* (KerML 1.1 §7.4.12), so a default value of
// any count is held; only a declared bound, or the implicit [1..1] of an eligible SysML
// usage (SysML v2 §7.6.3), refuses one — with a typed error, never a panic.
func TestRuntimeRobustnessDefaultMultiplicity(t *testing.T) {
	t.Run("kerml_feature_holds_a_default_of_any_count", testKerMLFeatureHoldsADefaultOfAnyCount)
	t.Run("kerml_declared_bound_still_refuses_a_default", testKerMLDeclaredBoundStillRefusesADefault)
	t.Run("sysml_package_attribute_holds_a_default_of_any_count", testSysMLPackageAttributeHoldsADefaultOfAnyCount)
	t.Run("sysml_implicit_one_still_refuses_a_default", testSysMLImplicitOneStillRefusesADefault)
}

// instantiateKerML materializes the named type declared in a KerML document over
// an index carrying the standard library.
func instantiateKerML(t *testing.T, src, fqn string) (*Context, *Instance, error) {
	t.Helper()
	const name = "<test>.kerml"
	file := parser.New(source.New(name, []byte(src))).ParseFile()
	idx, _, ctx := buildRuntimeWithLibraries(t, name, file)
	inst, err := ctx.Instantiate(oneSymbol(t, idx, fqn))
	return ctx, inst, err
}

func testKerMLFeatureHoldsADefaultOfAnyCount(t *testing.T) {
	for _, tc := range []struct {
		name, decl string
		count      int
	}{
		{"two values", "feature xs : Real = (1.0, 2.0);", 2},
		{"two values of an untyped feature", "feature xs = (1.0, 2.0);", 2},
		{"one value", "feature xs : Real = 1.0;", 1},
		{"no values", "feature xs : Real = ();", 0},
		{"an expression producing two", "feature m : Real[2] = (1.0, 2.0); feature xs : Real = m;", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, inst, err := instantiateKerML(t, `
				package test {
					private import ScalarValues::Real;
					class Holder { `+tc.decl+` }
					feature holder : Holder;
				}
			`, "test::holder")
			if err != nil {
				t.Fatalf("Instantiate: %v", err)
			}
			fv, err := inst.GetFeatureValue(ctx, "xs")
			if err != nil {
				t.Fatalf("xs: %v", err)
			}
			val, err := fv.ReadValue("xs")
			if err != nil {
				t.Fatalf("read xs: %v", err)
			}
			if got := len(elementsOf(val)); got != tc.count {
				t.Errorf("xs holds %d value(s) (%v), want %d", got, val, tc.count)
			}
		})
	}
}

func testKerMLDeclaredBoundStillRefusesADefault(t *testing.T) {
	ctx, inst, err := instantiateKerML(t, `
		package test {
			private import ScalarValues::Real;
			class Holder { feature xs : Real[1] = (1.0, 2.0); }
			feature holder : Holder;
		}
	`, "test::holder")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if _, err := inst.GetFeatureValue(ctx, "xs"); !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("xs = %v, want ErrMultiplicityViolation", err)
	}
}

func testSysMLPackageAttributeHoldsADefaultOfAnyCount(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::Real;
			attribute xs : Real = (1.0, 2.0);
		}
	`))
	val, err := ctx.EvalDeclaredValue(oneSymbol(t, idx, "test::xs"))
	if err != nil {
		t.Fatalf("xs: %v", err)
	}
	if got := len(elementsOf(val)); got != 2 {
		t.Errorf("xs holds %d value(s) (%v), want 2", got, val)
	}
}

func testSysMLImplicitOneStillRefusesADefault(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def Holder { attribute xs : Real = (1.0, 2.0); }
		}
	`)
	if _, err := inst.GetFeatureValue(ctx, "xs"); !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("xs = %v, want ErrMultiplicityViolation", err)
	}
}
