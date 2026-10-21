package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessDefaultBody: a body governing over an inherited `default` reads
// the default for what it leaves unvalued, so a default that cannot be evaluated is the
// typed error of its expression, surfacing where the object is read.
func TestRuntimeRobustnessDefaultBody(t *testing.T) {
	t.Run("default_the_body_governs_over_cannot_be_evaluated", testDefaultBodyUnevaluableDefault)
}

func testDefaultBodyUnevaluableDefault(t *testing.T) {
	ctx, inst, err := instantiateWithLibraries(t, `package test {
		private import ISQ::*;
		private import SI::*;
		attribute offset : LengthValue [0..*] default (1 / 0) [m];
		part def Shape { attribute :>> offset [1]; }
		part def Centered :> Shape { attribute :>> offset { attribute :>> num = 0; } }
		part def Rig { part centered : Centered; }
	}`, "test::Rig")
	if err != nil {
		t.Fatalf("instantiate Rig: %v", err)
	}
	_, err = featureValueAtPath(t, ctx, inst, "centered.offset.dimensions")
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("centered.offset.dimensions = %v, want ErrDivisionByZero from the default the body governs over", err)
	}
	errs, bounded := ctx.MaterializationErrors(inst)
	if bounded || len(errs) != 1 || !errors.Is(errs[0], ErrDivisionByZero) {
		t.Errorf("materialization = %v (bounded %v), want the one division-by-zero finding", errs, bounded)
	}
}
