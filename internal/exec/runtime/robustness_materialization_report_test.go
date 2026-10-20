package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessMaterializationReport covers what the check over a new object reports
// when feature values fail through one another: each failure once, an open value never.
func TestRuntimeRobustnessMaterializationReport(t *testing.T) {
	t.Run("cyclic_value_dependency_is_the_typed_cycle_error", testCyclicValueDependencyIsTheTypedCycleError)
	t.Run("derived_from_unset_is_no_finding_but_reads_as_no_value", testDerivedFromUnsetIsNoFindingButReadsAsNoValue)
	t.Run("failure_read_through_several_features_is_reported_once", testFailureReadThroughSeveralFeaturesIsReportedOnce)
	t.Run("binding_source_failing_default_is_reported_once", testBindingSourceFailingDefaultIsReportedOnce)
}

// testCyclicValueDependencyIsTheTypedCycleError: two features each computed from the other are
// ErrCyclicFeatureValue on every read, and the check reports the cycle once, naming both.
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

// testDerivedFromUnsetIsNoFindingButReadsAsNoValue: a feature derived from an unset one is
// passed over as the unset one is, while reading it is the typed no-value error naming it.
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
// fails every reader, and the check reports the failing feature value once, where it lies.
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

// testBindingSourceFailingDefaultIsReportedOnce: a binding reaches its source's default without
// a GetFeatureValue read, and the source's failure is still one finding, at the source.
func testBindingSourceFailingDefaultIsReportedOnce(t *testing.T) {
	ctx, inst, err := instantiateWithLibraries(t, `package test {
		private import ScalarValues::Real;
		part def Sys {
			attribute x : Real = 1.0;
			attribute a : Real[3] = x;
			attribute b : Real[3];
			binding bind b = a;
		}
	}`, "test::Sys")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	for _, name := range []string{"a", "b"} {
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
	if got := errs[0].Error(); !strings.HasPrefix(got, "feature value Sys.a:") || !errors.Is(errs[0], ErrMultiplicityViolation) {
		t.Errorf("materialization error = %q, want the multiplicity violation of Sys.a", got)
	}
}
