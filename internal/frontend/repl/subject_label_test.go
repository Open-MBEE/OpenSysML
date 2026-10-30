package repl

import (
	"strings"
	"testing"
)

// nestedSubjectFixture redefines a nested feature on an object, so the
// declaration and the object answer a condition over it differently.
const nestedSubjectFixture = `package A {
    part def Inner {
        attribute c default = 5.0;
        constraint small { c < 10.0 }
    }
    part def Outer {
        part b : Inner;
    }
    part o : Outer {
        part :>> b {
            attribute :>> c = 50.0;
        }
    }
}
`

// TestVerdictNamesTheNestedSubject: a check answered about a nested carrier is
// labelled with that object, named by the features it was reached through, not
// with the object the command was resolved to.
func TestVerdictNamesTheNestedSubject(t *testing.T) {
	s := loadSource(t, nestedSubjectFixture)
	run(t, s, "%instantiate A::o")

	v := s.CheckConstraint("A::Inner::small")
	wantVerdict(t, v, VerdictFails,
		"✗ Constraint A::Inner::small failed (on A::o.b ID:")
	rejectVerdict(t, v, "(on A::o ID:")
}

// subsettingLabelFixture gathers objects of several declarations into one
// collection — `small` and the two `smalls` subset `subsystem` — each carrying
// an assertion and a satisfaction, under a root and one level down.
const subsettingLabelFixture = `package test {
    private import ScalarValues::Real;
    requirement def Fits {
        subject c : Component;
        require constraint { c.v < 10.0 }
    }
    requirement fits : Fits;
    part def Component {
        attribute v : Real default = 1.0;
        assert constraint ok { v < 10.0 }
        satisfy fits;
    }
    part def Assembly {
        part subsystem : Component[*];
        part small : Component :> subsystem {
            attribute :>> v = 99.0;
        }
        part smalls : Component[2] :> subsystem {
            attribute :>> v = 50.0;
        }
    }
    part def Outer {
        part inner : Assembly;
    }
    part assembly : Assembly;
    part outer : Outer;
}
`

// TestEverySurfaceLabelsASubsettingObjectByItsDeclaration: %validate, %satisfy,
// %constraint and %requirement name an object a subsetting usage materializes
// by that usage (`small`, `smalls[2]`), never by its position in the collection
// it subsets (`subsystem[3]`), at every segment of a nested path.
func TestEverySurfaceLabelsASubsettingObjectByItsDeclaration(t *testing.T) {
	s := loadSource(t, subsettingLabelFixture)
	run(t, s, "%instantiate test::assembly")
	labels := []string{
		"(on test::assembly.small ID: 2)",
		"(on test::assembly.smalls[1] ID: 3)",
		"(on test::assembly.smalls[2] ID: 4)",
	}
	validated := run(t, s, "%validate assembly")
	satisfied := run(t, s, "%satisfy")
	for name, out := range map[string]string{"%validate": validated, "%satisfy": satisfied} {
		wants(t, out, labels...)
		if strings.Contains(out, "subsystem[") || strings.Contains(out, "smalls ID") {
			t.Errorf("%s labels an object by the collection it subsets or by a bare declaration:\n%s", name, out)
		}
	}
	wants(t, run(t, s, "%constraint test::Component::ok assembly.small"), "(on test::assembly.small ID: 2)")
	wants(t, run(t, s, "%requirement test::fits assembly.smalls[2]"), "(on test::assembly.smalls[2] ID: 4)")

	run(t, s, "%instantiate test::outer")
	nested := run(t, s, "%validate outer")
	wants(t, nested,
		"✗ assert constraint ok fails (on test::outer.inner.small ID: 7)",
		"✗ satisfy fits fails (on test::outer.inner.small ID: 7)",
		"✗ assert constraint ok fails (on test::outer.inner.smalls[1] ID: 8)",
		"✗ assert constraint ok fails (on test::outer.inner.smalls[2] ID: 9)",
	)
	if strings.Contains(nested, "subsystem[") {
		t.Errorf("%%validate labels a nested object by the collection it subsets:\n%s", nested)
	}
}
