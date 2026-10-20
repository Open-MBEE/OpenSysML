package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessKernelMembers: a read of a member the object's type lacks is a
// typed error naming the type, and distinct from a Kernel Semantic Library member the
// shape leaves out, which reads on every surface; writing one stays refused.
func TestRuntimeRobustnessKernelMembers(t *testing.T) {
	t.Run("nonexistent member names the type", testKernelNonexistentMemberNamesTheType)
	t.Run("masked name names the redefining member", testKernelMaskedNameNamesRedefinition)
	t.Run("kernel member is no missing feature", testKernelMemberIsNoMissingFeature)
	t.Run("feature value reader agrees with the chain", testKernelMemberValueReaderAgrees)
	t.Run("writing a kernel member is refused", testKernelMemberWriteRefused)
}

func kernelProbeObjects(t *testing.T) (*Context, *Instance, *Instance, func(string) (Value, error)) {
	t.Helper()
	ctx, idx := libraryShapeContext(t, kernelProbeModel)
	root := idx.DocumentRoot("<test>")
	eval := func(src string) (Value, error) { return evalIn(t, ctx, root, src) }
	p1 := mustEvalIn(t, ctx, root, "Probe::p1")
	pa := mustEvalIn(t, ctx, root, "Probe::p1.pa")
	return ctx, ctx.instances[p1.Instance], ctx.instances[pa.Instance], eval
}

func testKernelNonexistentMemberNamesTheType(t *testing.T) {
	_, _, _, eval := kernelProbeObjects(t)
	_, err := eval("Probe::p1.ownedParts")
	if !errors.Is(err, ErrNoSuchFeature) || !strings.Contains(err.Error(), "p1 has no member ownedParts") {
		t.Fatalf("Probe::p1.ownedParts = %v, want ErrNoSuchFeature naming p1", err)
	}
	if strings.Contains(err.Error(), "not found in instance") {
		t.Errorf("error %q still reads as a materialization gap", err)
	}
}

func testKernelMaskedNameNamesRedefinition(t *testing.T) {
	_, _, _, eval := kernelProbeObjects(t)
	_, err := eval("Probe::p1.startShot")
	if !errors.Is(err, ErrNoSuchFeature) || !strings.Contains(err.Error(), "p1 has no member startShot: Parts::Part::start redefines it") {
		t.Fatalf("Probe::p1.startShot = %v, want ErrNoSuchFeature naming the redefinition", err)
	}
	if val, err := eval("Probe::p1.start"); err != nil || val.Kind != ValUndetermined {
		t.Errorf("Probe::p1.start = %s, %v; want an undetermined read of the redefinition", FormatValue(val), err)
	}
}

func testKernelMemberIsNoMissingFeature(t *testing.T) {
	_, p1, _, eval := kernelProbeObjects(t)
	for src, want := range map[string]ValueKind{
		"Probe::p1.self": ValInstance, "Probe::p1.timeSlices": ValSequence, "Probe::p1.predecessors": ValSequence,
		"Probe::p1.superoccurrence": ValNull, "Probe::p1.snapshots": ValUndetermined, "Probe::p1.localClock": ValInstance,
	} {
		val, err := eval(src)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if val.Kind != want {
			t.Errorf("%s = %s, want a %v", src, FormatValue(val), want)
		}
		if val.Kind == ValInstance && src == "Probe::p1.self" && val.Instance != p1.ID {
			t.Errorf("Probe::p1.self is object %d, want the receiver %d", val.Instance, p1.ID)
		}
	}
}

func testKernelMemberValueReaderAgrees(t *testing.T) {
	ctx, p1, pa, _ := kernelProbeObjects(t)
	fv, err := pa.MemberValue(ctx, "superoccurrence")
	if err != nil || fv.HeldValue().Kind != ValInstance || fv.HeldValue().Instance != p1.ID {
		t.Fatalf("pa.MemberValue(superoccurrence) = %v, %v; want the owner %d", fv, err, p1.ID)
	}
	if _, err := pa.MemberValue(ctx, "ownedParts"); !errors.Is(err, ErrNoSuchFeature) || !strings.Contains(err.Error(), "has no member ownedParts") {
		t.Errorf("pa.MemberValue(ownedParts) = %v, want ErrNoSuchFeature naming the type", err)
	}
	// The shape probe is unchanged: a Kernel member is no feature value of the object.
	if _, err := p1.GetFeatureValue(ctx, "self"); !errors.Is(err, ErrNoSuchFeature) {
		t.Errorf("p1.GetFeatureValue(self) = %v, want ErrNoSuchFeature", err)
	}
}

func testKernelMemberWriteRefused(t *testing.T) {
	ctx, p1, _, eval := kernelProbeObjects(t)
	before, err := eval("Probe::p1.localClock")
	if err != nil {
		t.Fatal(err)
	}
	if err := p1.SetFeatureValue(ctx, "localClock", Value{Kind: ValInstance, Instance: p1.ID}); err == nil {
		t.Fatal("writing localClock succeeded, want a refusal")
	}
	after, err := eval("Probe::p1.localClock")
	if err != nil || after.Instance != before.Instance {
		t.Errorf("localClock after the refused write = %s, %v; want %s unchanged", FormatValue(after), err, FormatValue(before))
	}
}
