package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestRuntimeRobustnessPartialBinding covers the failure modes of a binding that
// does not determine an end whole — the connector's own `binding [n]` bounding
// the links included: typed errors, never a panic or a hang.
func TestRuntimeRobustnessPartialBinding(t *testing.T) {
	t.Run("underdetermined end is a typed error", testPartialBindingUnderdeterminedEnd)
	t.Run("lower bound above the link count", testPartialBindingLowerBoundAboveLinks)
	t.Run("mutually partial bindings terminate", testPartialBindingMutual)
	t.Run("reading through the partially bound end", testPartialBindingReadThrough)
	t.Run("instantiating reports no undetermined end", testPartialBindingInstantiatesClean)
}

// rigScope parses a model around `part def Rig` and returns the scope of package
// test so `rig.<feature>` expressions evaluate against the rig instance.
func rigScope(t *testing.T, rig string) (*Context, *symbols.Scope) {
	t.Helper()
	ctx, idx := libraryShapeContext(t, `package test {
		part def Thing { attribute mass : Real; }
		`+rig+`
		part rig : Rig;
	}`)
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return ctx, pkg.Scope
}

// `binding [1]` links one value of each end, so the end whose feature must hold
// two is undetermined rather than bound whole: reading it is ErrBindingEnd.
func testPartialBindingUnderdeterminedEnd(t *testing.T) {
	ctx, scope := rigScope(t, `part def Rig {
		part a { part xs : Thing [1]; }
		part ys : Thing [2];
		binding [1] bind [0..*] a.xs = [0..*] ys;
	}`)
	if _, err := evalIn(t, ctx, scope, "rig.ys"); !errors.Is(err, ErrBindingEnd) {
		t.Fatalf("rig.ys = %v, want ErrBindingEnd", err)
	}
}

// A feature declaring a lower bound above the connector's link count can never
// be linked whole: reading it is the typed error without reading its value, and
// the other end, some unspecified value of it, is the same error.
func testPartialBindingLowerBoundAboveLinks(t *testing.T) {
	ctx, scope := rigScope(t, `part def Rig {
		part a { part xs : Thing [1]; }
		part ys3 : Thing [3..*];
		binding [1] bind [0..*] a.xs = [0..*] ys3;
	}`)
	for _, expr := range []string{"rig.ys3", "rig.a.xs"} {
		if _, err := evalIn(t, ctx, scope, expr); !errors.Is(err, ErrBindingEnd) {
			t.Fatalf("%s = %v, want ErrBindingEnd", expr, err)
		}
	}
}

// Two partial bindings each waiting on the other end resolve to a typed error,
// never a hang.
func testPartialBindingMutual(t *testing.T) {
	ctx, scope := rigScope(t, `part def Rig {
		part ys : Thing [2];
		part zs : Thing [2];
		binding [1] bind [0..*] ys = [0..*] zs;
		binding [1] bind [0..*] zs = [0..*] ys;
	}`)
	_, err := evalIn(t, ctx, scope, "rig.ys")
	if !errors.Is(err, ErrBindingEnd) && !errors.Is(err, ErrBindingCycle) {
		t.Fatalf("rig.ys = %v, want ErrBindingEnd or ErrBindingCycle", err)
	}
}

// Reading a member through a partially bound end is the same typed error, not
// an empty read of a value the binding did not determine.
func testPartialBindingReadThrough(t *testing.T) {
	ctx, scope := rigScope(t, `part def Rig {
		part a { part xs : Thing [1]; }
		part ys : Thing [2];
		binding [1] bind [0..*] a.xs = [0..*] ys;
	}`)
	if _, err := evalIn(t, ctx, scope, "rig.ys.mass"); !errors.Is(err, ErrBindingEnd) {
		t.Fatalf("rig.ys.mass = %v, want ErrBindingEnd", err)
	}
}

// Instantiating an object whose feature is pinned only by `[0..1]` bindings reports
// nothing for that feature — the model leaves it open — and every other end still
// answers; reading the feature itself is the typed error, not an empty or picked value.
func testPartialBindingInstantiatesClean(t *testing.T) {
	ctx, scope := rigScope(t, `part def Rig {
		part a { part x1 : Thing; part x2 : Thing; part xs : Thing [2] = (x1, x2); }
		part b { part y1 : Thing; part y2 : Thing; part ys : Thing [2] = (y1, y2); }
		part shared : Thing [1];
		binding [1] bind [0..1] a.xs = [0..1] shared;
		binding [1] bind [0..1] b.ys = [0..1] shared;
	}`)
	rig, err := evalIn(t, ctx, scope, "rig")
	if err != nil {
		t.Fatalf("rig = %v", err)
	}
	id, ok := rig.Object()
	if !ok {
		t.Fatalf("rig = %v, want an object", rig)
	}
	inst, ok := ctx.Instance(id)
	if !ok {
		t.Fatalf("object %d is not an instance", id)
	}
	errs, bounded := ctx.MaterializationErrors(inst)
	if len(errs) != 0 || bounded {
		t.Fatalf("MaterializationErrors = %v, bounded %v; want none, read in full", errs, bounded)
	}
	if v, err := evalIn(t, ctx, scope, "rig.a.xs"); err != nil || v.Sequence() == nil || len(v.Sequence().Elements()) != 2 {
		t.Fatalf("rig.a.xs = %v, %v; want two objects", v, err)
	}
	if _, err := evalIn(t, ctx, scope, "rig.shared"); !errors.Is(err, ErrBindingEnd) {
		t.Fatalf("rig.shared = %v, want ErrBindingEnd", err)
	}
}
