package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessMaterializationReport covers what the check over a new object
// reports when feature values fail through one another: a genuine cycle stays the typed
// cycle error, a value derived from an unset feature is no finding, and one failure read
// through several features is reported once, where it lies.
func TestRuntimeRobustnessMaterializationReport(t *testing.T) {
	t.Run("cyclic_value_dependency_is_the_typed_cycle_error", testCyclicValueDependencyIsTheTypedCycleError)
	t.Run("derived_from_unset_is_no_finding_but_reads_as_no_value", testDerivedFromUnsetIsNoFindingButReadsAsNoValue)
	t.Run("failure_read_through_several_features_is_reported_once", testFailureReadThroughSeveralFeaturesIsReportedOnce)
}

// testCyclicValueDependencyIsTheTypedCycleError: two features each computed from the other
// have no well-founded value; reading either is ErrCyclicFeatureValue, and the check reports
// the cycle once, naming both features.
func testCyclicValueDependencyIsTheTypedCycleError(t *testing.T) {
	ctx, inst, err := instantiateWithLibraries(t, `package test {
		private import ScalarValues::Real;
		part def Loop {
			attribute a : Real = b + 1.0;
			attribute b : Real = a;
		}
	}`, "test::Loop")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	for _, name := range []string{"a", "b"} {
		_, err := inst.GetFeatureValue(ctx, name)
		if !errors.Is(err, ErrCyclicFeatureValue) {
			t.Errorf("reading %s: got %v, want ErrCyclicFeatureValue", name, err)
		}
	}
	errs, bounded := ctx.MaterializationErrors(inst)
	if bounded {
		t.Error("bounded = true, want a small object read in full")
	}
	if len(errs) != 1 {
		t.Fatalf("materialization errors = %v, want the one cycle", errs)
	}
	if !errors.Is(errs[0], ErrCyclicFeatureValue) {
		t.Errorf("materialization error = %v, want ErrCyclicFeatureValue", errs[0])
	}
	for _, want := range []string{"Loop.a", "Loop.b"} {
		if !strings.Contains(errs[0].Error(), want) {
			t.Errorf("materialization error %q does not name %s", errs[0], want)
		}
	}
}

// testDerivedFromUnsetIsNoFindingButReadsAsNoValue: a feature derived from one nothing
// values holds no value as that feature does, so the check passes over it as it passes over
// the unset attribute, while reading it is the typed no-value error naming the unset feature.
func testDerivedFromUnsetIsNoFindingButReadsAsNoValue(t *testing.T) {
	ctx, inst, err := instantiateWithLibraries(t, `package test {
		private import ScalarValues::Real;
		part def Beam {
			attribute span : Real;
			attribute halfSpan : Real = span / 2.0;
			attribute quarterSpan : Real = halfSpan / 2.0;
		}
	}`, "test::Beam")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	errs, bounded := ctx.MaterializationErrors(inst)
	if bounded {
		t.Error("bounded = true, want a small object read in full")
	}
	if len(errs) != 0 {
		t.Errorf("materialization errors = %v, want none for values open for an unset attribute", errs)
	}
	for _, name := range []string{"halfSpan", "quarterSpan"} {
		_, err := inst.GetFeatureValue(ctx, name)
		var noValue *NoValueError
		if !errors.As(err, &noValue) {
			t.Fatalf("reading %s: got %v, want *NoValueError", name, err)
		}
		if noValue.Feature != "span" {
			t.Errorf("reading %s: no value for %q, want the unset feature span", name, noValue.Feature)
		}
	}
}

// testFailureReadThroughSeveralFeaturesIsReportedOnce: a default its feature does not admit
// fails every feature that reads through it, and the check reports the failing feature value
// once, where it lies, not again for each reader.
func testFailureReadThroughSeveralFeaturesIsReportedOnce(t *testing.T) {
	ctx, inst, err := instantiateWithLibraries(t, `package test {
		private import ScalarValues::Real;
		part def Reader {
			attribute got : Real[3];
		}
		part def Source {
			attribute xs : Real[3] = 1.0;
			attribute ys : Real[3] = xs;
			part reader : Reader {
				attribute :>> got = Source::xs;
			}
		}
	}`, "test::Source")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	for _, name := range []string{"xs", "ys"} {
		_, err := inst.GetFeatureValue(ctx, name)
		if !errors.Is(err, ErrMultiplicityViolation) {
			t.Errorf("reading %s: got %v, want ErrMultiplicityViolation", name, err)
		}
	}
	errs, bounded := ctx.MaterializationErrors(inst)
	if bounded {
		t.Error("bounded = true, want a small object read in full")
	}
	if len(errs) != 1 {
		t.Fatalf("materialization errors = %v, want the one failing default", errs)
	}
	if !errors.Is(errs[0], ErrMultiplicityViolation) {
		t.Errorf("materialization error = %v, want ErrMultiplicityViolation", errs[0])
	}
	if got := errs[0].Error(); !strings.HasPrefix(got, "feature value Source.xs:") {
		t.Errorf("materialization error = %q, want the failing feature value Source.xs named first", got)
	}
}
