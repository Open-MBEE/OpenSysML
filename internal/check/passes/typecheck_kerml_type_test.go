package passes

import (
	"strings"
	"testing"
)

// A KerML type types a usage whose kind its metaclass satisfies: a class is a
// Class, as an occurrence usage demands, and a behavior a Behavior.
func TestTypeCheckKerMLTypeTargetNoMismatch(t *testing.T) {
	for _, src := range []string{
		"class T; part def Car { part p : T; }",
		"struct T; part def Car { item i : T; snapshot part s : T; }",
		"behavior B; action def Move { action a : B; }",
		"class T; part def Car { snapshot action a : T; timeslice part t : T; }",
		"datatype D; part def Car { attribute x : D; }",
		"struct S; part def Car specializes S;",
	} {
		if diags := typeDiags(t, src); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
}

// A KerML type of another metaclass is the kind rule's mismatch, declared or
// inherited: a plain classifier is no Class and a struct no DataType.
func TestTypeCheckKerMLTypeTargetMismatch(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"classifier T; part def Car { part p : T; }", "An occurrence, item or part must be typed by occurrence definitions."},
		{"classifier T; part def Car { snapshot part p : T; }", "An occurrence, item or part must be typed by occurrence definitions."},
		{"struct S; part def Car { attribute x : S; attribute y :> x; }", "An attribute must be typed by attribute definitions."},
		{"behavior B; part def Car { attribute x : B; }", "An attribute must be typed by attribute definitions."},
	} {
		diags := typeDiags(t, tc.src)
		want := strings.Count(tc.src, "attribute") + strings.Count(tc.src, "part p")
		if len(diags) != want {
			t.Fatalf("%s: expected %d type diagnostics, got %v", tc.src, want, diags)
		}
		for _, d := range diags {
			if d.Message != tc.want {
				t.Errorf("%s: got %q, want %q", tc.src, d.Message, tc.want)
			}
		}
	}
}

// A KerML type is not a requirement usage, so satisfying one is still an error:
// the exemption covers the kind taxonomy, not the shape rules.
func TestTypeCheckSatisfyKerMLType(t *testing.T) {
	diags := typeDiags(t, "classifier T; part def Car { satisfy T; }")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
	if !strings.Contains(diags[0].Message, "satisfy target must be a requirement usage") {
		t.Errorf("got %q", diags[0].Message)
	}
}

// A target of a kind that classifies nothing a declaration may be typed by
// still reports a mismatch: only KerML type declarations are exempt.
func TestTypeCheckUnclassifiedTargetStillMismatches(t *testing.T) {
	diags := typeDiags(t, "part def Car { attribute x; binding b of x = x; part p : b; }")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
	if diags[0].Message != "An occurrence, item or part must be typed by occurrence definitions." {
		t.Errorf("got %q", diags[0].Message)
	}
}
