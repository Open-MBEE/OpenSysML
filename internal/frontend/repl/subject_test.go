package repl

import (
	"errors"
	"strings"
	"testing"
)

// nestedRedefinitionModel redefines a feature two levels down in an object, so
// the value in effect for that object differs from the declared default.
const nestedRedefinitionModel = `package A {
    attribute def Spec { attribute c : ScalarValues::Real = 1.0; }
    part def Inner { attribute b : Spec; }
    part def Outer { part inner : Inner; }
    part o : Outer {
        redefines inner {
            attribute redefines b { attribute redefines c = 9.0; }
        }
    }
}`

// An expression reached through a declaration is about the object carrying that
// declaration, so a redefinition in effect for the object is honored however the
// subject was named.
func TestEvalThroughDeclarationHonorsNestedRedefinition(t *testing.T) {
	s := NewSession()
	res := s.Submit(nestedRedefinitionModel)
	if hasSyntaxError(res) {
		t.Fatalf("model does not parse: %v", res.Diagnostics)
	}
	if out, _, err := s.runMeta("%instantiate A::o"); err != nil {
		t.Fatalf("%%instantiate: %v (%v)", err, out)
	}
	for _, path := range []string{
		"A::o::inner::b::c",     // through the object
		"A::Outer::inner::b::c", // through the declaration
		"A::Inner::b::c",
		"A::Spec::c",
	} {
		lines, err := s.EvalExpr(path)
		if err != nil {
			t.Errorf("%%eval %s: %v", path, err)
			continue
		}
		if got := strings.Join(lines, "\n"); !strings.Contains(got, "9.0") {
			t.Errorf("%%eval %s = %v, want the redefined value", path, lines)
		}
	}
}

// A later unrelated declaration re-analyzes the document, giving the same
// declarations new symbols; the subject is still the object carrying them, so
// the answer does not fall back to the declared default.
func TestEvalThroughDeclarationKeepsItsSubjectAcrossSubmissions(t *testing.T) {
	s := NewSession()
	s.Submit(nestedRedefinitionModel)
	if _, _, err := s.runMeta("%instantiate A::o"); err != nil {
		t.Fatal(err)
	}
	s.Submit("package Extra { part def E; }")

	for _, path := range []string{"A::Spec::c", "A::Outer::inner::b::c", "A::o::inner::b::c"} {
		lines, err := s.EvalExpr(path)
		if err != nil {
			t.Errorf("%%eval %s after an unrelated submission: %v", path, err)
			continue
		}
		got := strings.Join(lines, "\n")
		if !strings.Contains(got, "9.0") {
			t.Errorf("%%eval %s = %v, want the redefined value", path, lines)
		}
		if !strings.Contains(got, "(on A::o.inner.b ID:") {
			t.Errorf("%%eval %s = %v, want the carrying object named", path, lines)
		}
	}
}

// Without an object of the type, the declaration's own default is the answer:
// nothing redefines it.
func TestEvalThroughDeclarationWithoutObjectUsesDeclaredValue(t *testing.T) {
	s := NewSession()
	s.Submit(nestedRedefinitionModel)

	lines, err := s.EvalExpr("A::Spec::c")
	if err != nil {
		t.Fatalf("%%eval A::Spec::c: %v", err)
	}
	if got := strings.Join(lines, "\n"); !strings.Contains(got, "1.0") {
		t.Errorf("%%eval A::Spec::c = %v, want the declared default", lines)
	}
}

// Two objects carrying the same feature make the subject a question, which is
// reported rather than answered about one of them.
func TestEvalThroughDeclarationWithTwoNestedCarriersIsAmbiguous(t *testing.T) {
	s := NewSession()
	s.Submit(nestedRedefinitionModel)
	s.Submit(`package B {
    part p : A::Outer {
        redefines inner {
            attribute redefines b { attribute redefines c = 5.0; }
        }
    }
}`)
	if _, _, err := s.runMeta("%instantiate A::o"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.runMeta("%instantiate B::p"); err != nil {
		t.Fatal(err)
	}

	_, err := s.EvalExpr("A::Spec::c")
	if err == nil {
		t.Fatal("two carriers should make the subject a question")
	}
	var ambiguous *AmbiguousSubjectError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("err = %v, want an AmbiguousSubjectError", err)
	}
	if len(ambiguous.Carriers) < 2 {
		t.Errorf("carriers = %v, want both objects named", ambiguous.Carriers)
	}
}

// A check of a condition and an evaluation of an expression share the subject,
// so the redefinition a nested object carries decides both.
func TestCheckHonorsNestedRedefinitionThroughDeclaration(t *testing.T) {
	s := NewSession()
	s.Submit(`package A {
    attribute def Spec { attribute c : ScalarValues::Real = 1.0; constraint high { c > 5.0 } }
    part def Inner { attribute b : Spec; }
    part def Outer { part inner : Inner; }
    part o : Outer {
        redefines inner {
            attribute redefines b { attribute redefines c = 9.0; }
        }
    }
}`)
	if _, _, err := s.runMeta("%instantiate A::o"); err != nil {
		t.Fatal(err)
	}
	v := s.CheckConstraint("A::Spec::high")
	if !v.Holds() {
		t.Errorf("constraint over the redefined value: %v", v.Lines)
	}
	// A re-analysis of the document leaves the check about the same object.
	s.Submit("package Extra { part def E; }")
	if v := s.CheckConstraint("A::Spec::high"); !v.Holds() {
		t.Errorf("constraint after an unrelated submission: %v", v.Lines)
	}
}
