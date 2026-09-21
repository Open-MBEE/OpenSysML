package runtime

import (
	"errors"
	"strings"
	"testing"
)

// The connector's own multiplicity states how many links a binding declares, so
// `binding [1]` identifies at most one value of each end: an end whose feature
// may — or must — hold more is only partially bound, and reading it through the
// binding is the typed ErrBindingEnd rather than a whole-binding count check.
func TestConnectorMultiplicityBoundsLinks(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		part def Thing;
		part def Rig {
			part a { part xs : Thing [1]; }
			part ys : Thing [2];
			binding [1] bind [0..*] a.xs = [0..*] ys;
		}
		part rig : Rig;
	}`)
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}

	val, err := evalIn(t, ctx, pkg.Scope, "rig.a.xs")
	if err != nil {
		t.Fatalf("rig.a.xs: %v", err)
	}
	if val.Kind != ValInstance {
		t.Errorf("rig.a.xs = %s, want one Instance", FormatValue(val))
	}

	_, err = evalIn(t, ctx, pkg.Scope, "rig.ys")
	var undetermined *UndeterminedBindingError
	if !errors.As(err, &undetermined) {
		t.Fatalf("rig.ys = %v, want an UndeterminedBindingError", err)
	}
	if !strings.Contains(err.Error(), "binding [1] bind [0..*] a.xs = [0..*] ys") {
		t.Errorf("rig.ys error %q does not name the binding", err.Error())
	}
}

// Without a connector multiplicity the same ends form a whole binding, whose
// count check against each end's declared multiplicity is unchanged.
func TestWholeBindingWithoutConnectorMultiplicityStillChecksCounts(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		part def Thing;
		part def Rig {
			part a { part xs : Thing [1]; }
			part ys : Thing [2];
			bind a.xs = ys;
		}
		part rig : Rig;
	}`)
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	for _, expr := range []string{"rig.ys", "rig.a.xs"} {
		if _, err := evalIn(t, ctx, pkg.Scope, expr); !errors.Is(err, ErrMultiplicityViolation) {
			t.Errorf("%s = %v, want ErrMultiplicityViolation", expr, err)
		}
	}
}

// A connector declaring as many links as the ends' features hold is a whole
// binding again: `binding [2]` binds q to the two objects p holds.
func TestConnectorMultiplicityWideEnoughIsWhole(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		part def Thing;
		part def Rig {
			part p : Thing [2];
			part q : Thing [2];
			binding [2] bind [0..*] p = [0..*] q;
			part p1 : Thing [1];
			part q1 : Thing [1];
			binding [1] bind [0..*] p1 = [0..*] q1;
		}
		part rig : Rig;
	}`)
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	for _, ends := range [][2]string{{"rig.p", "rig.q"}, {"rig.p1", "rig.q1"}} {
		left, err := evalIn(t, ctx, pkg.Scope, ends[0])
		if err != nil {
			t.Fatalf("%s: %v", ends[0], err)
		}
		right, err := evalIn(t, ctx, pkg.Scope, ends[1])
		if err != nil {
			t.Fatalf("%s: %v", ends[1], err)
		}
		lv, rv := elementsOf(left), elementsOf(right)
		if len(lv) == 0 || len(lv) != len(rv) {
			t.Fatalf("%s = %s, %s = %s; want the same values", ends[0], FormatValue(left), ends[1], FormatValue(right))
		}
		for i := range lv {
			if lv[i].Instance != rv[i].Instance {
				t.Errorf("%s#(%d) and %s#(%d) are different objects", ends[0], i+1, ends[1], i+1)
			}
		}
	}
}
