package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestRuntimeRobustnessMultiplicity covers multiplicity bounds, defaults that do not conform
// to them, feature chains and mutually subsetting features.
func TestRuntimeRobustnessMultiplicity(t *testing.T) {
	t.Run("multiplicity_infinite_lower_bound", testMultiplicityInfiniteLowerBound)
	t.Run("multiplicity_lower_bound_too_large", testMultiplicityLowerBoundTooLarge)
	t.Run("default_not_conforming_to_multiplicity", testDefaultNotConformingToMultiplicity)
	t.Run("default_against_an_undeclared_multiplicity", testDefaultAgainstAnUndeclaredMultiplicity)
	t.Run("feature_chain_through_an_unset_feature_value", testFeatureChainThroughAnUnsetFeatureValue)
	t.Run("feature_chain_spends_the_element_budget", testFeatureChainSpendsTheElementBudget)
	t.Run("mutually_subsetting_features", testMutuallySubsettingFeatures)
}

// testMultiplicityInfiniteLowerBound: `[*..*]` requires unboundedly many objects,
// which cannot be materialized, so the feature value reports a multiplicity violation
// rather than allocating until memory runs out.
func testMultiplicityInfiniteLowerBound(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def C { attribute m : Real = 1.0; }
			part def Holder { part p : C[*..*]; }
		}
	`)
	_, err := inst.GetFeatureValue(ctx, "p")
	if err == nil {
		t.Fatal("want a multiplicity violation, got a materialized feature value")
	}
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("expected ErrMultiplicityViolation, got: %v", err)
	}
}

// testMultiplicityLowerBoundTooLarge: a lower bound of thousands holds that many values
// without making that many objects up front.
func testMultiplicityLowerBoundTooLarge(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def C { attribute m : Real = 1.0; }
			part def Holder { part p : C[5000]; }
		}
	`)
	fv, err := inst.GetFeatureValue(ctx, "p")
	if err != nil {
		t.Fatalf("p: %v; want 5000 values", err)
	}
	if got := ElementCount(fv.Values); got != 5000 {
		t.Errorf("p holds %d values, want 5000", got)
	}
	if len(ctx.instances) > 100 {
		t.Errorf("materialized %d instances to hold a lower bound of 5000", len(ctx.instances))
	}
}

// testDefaultNotConformingToMultiplicity: a default whose element count is
// outside the feature's multiplicity is reported, rather than broadcast to fill
// the lower bound, truncated to the upper one, or dropped.
func testDefaultNotConformingToMultiplicity(t *testing.T) {
	for _, tc := range []struct {
		name string
		decl string
	}{
		{"one value against three", "attribute xs : Real[3] = 1.0;"},
		{"four values against three", "attribute xs : Real[3] = (1.0, 2.0, 3.0, 4.0);"},
		{"no values against one or more", "attribute xs : Real[1..3] = ();"},
		{"an expression producing too few", "attribute m : Real = 1.0; attribute xs : Real[2] = m;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inst, ctx := instantiateHolder(t, `
				package test {
					private import ScalarValues::Real;
					part def Holder { `+tc.decl+` }
				}
			`)
			done := make(chan error, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						done <- fmt.Errorf("panic: %v", r)
					}
				}()
				_, err := inst.GetFeatureValue(ctx, "xs")
				done <- err
			}()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("want a multiplicity violation, got a materialized feature value")
				}
				if !errors.Is(err, ErrMultiplicityViolation) {
					t.Errorf("expected ErrMultiplicityViolation, got: %v", err)
				}
			case <-watchdog(5 * time.Second):
				t.Fatal("materializing the default did not terminate")
			}
		})
	}
}

// testDefaultAgainstAnUndeclaredMultiplicity: a feature that declares no
// multiplicity holds exactly one value, so a default of any other number of
// values is reported rather than held under an unconstrained bound.
func testDefaultAgainstAnUndeclaredMultiplicity(t *testing.T) {
	for _, tc := range []struct {
		name     string
		decl     string
		reported bool
	}{
		{"one value", "attribute xs : Real = 1.0;", false},
		{"one value of an untyped feature", "attribute xs = 1.0;", false},
		{"two values", "attribute xs : Real = (1.0, 2.0);", true},
		{"two values of an untyped feature", "attribute xs = (1.0, 2.0);", true},
		{"no values", "attribute xs : Real = ();", true},
		{"an expression producing two", "attribute m : Real[2] = (1.0, 2.0); attribute xs : Real = m;", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inst, ctx := instantiateHolder(t, `
				package test {
					private import ScalarValues::Real;
					part def Holder { `+tc.decl+` }
				}
			`)
			_, err := inst.GetFeatureValue(ctx, "xs")
			switch {
			case tc.reported && err == nil:
				t.Fatalf("%s was held, want a multiplicity violation", tc.decl)
			case tc.reported && !errors.Is(err, ErrMultiplicityViolation):
				t.Errorf("expected ErrMultiplicityViolation, got: %v", err)
			case !tc.reported && err != nil:
				t.Errorf("%s was reported: %v", tc.decl, err)
			}
		})
	}
}

// testFeatureChainThroughAnUnsetFeatureValue: a chain over a collection whose objects
// hold no value for the last feature names that feature, rather than reading it
// as an empty collection.
func testFeatureChainThroughAnUnsetFeatureValue(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			private import RealFunctions::*;
			part def Sub { attribute volume : Real; }
			part def Holder {
				part subs : Sub[2];
				attribute total : Real = sum(subs.volume);
			}
		}
	`)
	_, err := inst.GetFeatureValue(ctx, "total")
	if err == nil {
		t.Fatal("want the unset feature value's error, got a value")
	}
	if !errors.Is(err, errUninitializedFeatureValue) {
		t.Errorf("expected errUninitializedFeatureValue, got: %v", err)
	}
	if !strings.Contains(err.Error(), "volume") {
		t.Errorf("error %q does not name the unset feature", err)
	}
}

// testFeatureChainSpendsTheElementBudget: navigating a chain through a collection
// counts what it collects, so a chain over a large collection ends within the
// element budget rather than growing unbounded.
func testFeatureChainSpendsTheElementBudget(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def Sub { attribute volume : Real = 1.0; }
			part def Holder {
				part subs : Sub[10];
				attribute volumes : Real[*] = subs.volume;
			}
		}
	`)
	ctx.maxElements = 15
	_, err := inst.GetFeatureValue(ctx, "volumes")
	if err == nil {
		t.Fatal("want the element budget's error, got a value")
	}
	if !errors.Is(err, ErrElementLimitExceeded) {
		t.Errorf("expected ErrElementLimitExceeded, got: %v", err)
	}
}

// testMutuallySubsettingFeatures: two features that subset each other have no
// well-founded set of values, so materializing one reports the cycle instead of
// recursing until the step budget runs out.
func testMutuallySubsettingFeatures(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def C { attribute m : Real = 1.0; }
			part def Holder {
				part a : C[*] :> b;
				part b : C[*] :> a;
			}
		}
	`)
	done := make(chan struct{})
	var fvErr error
	go func() {
		defer close(done)
		_, fvErr = inst.GetFeatureValue(ctx, "a")
	}()
	select {
	case <-done:
	case <-watchdog(5 * time.Second):
		t.Fatal("GetFeatureValue hung on mutually subsetting features")
	}
	if fvErr == nil {
		t.Fatal("want the cyclic feature value's error, got a materialized feature value")
	}
	if !errors.Is(fvErr, ErrCyclicFeatureValue) {
		t.Errorf("expected ErrCyclicFeatureValue, got: %v", fvErr)
	}
}
