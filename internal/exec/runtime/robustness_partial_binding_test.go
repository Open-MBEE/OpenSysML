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
