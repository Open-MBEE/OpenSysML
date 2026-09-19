package repl

import "testing"

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
