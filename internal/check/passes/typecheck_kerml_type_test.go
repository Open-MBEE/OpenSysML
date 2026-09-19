package passes

import (
	"strings"
	"testing"
)

// Every SysML definition specializes a KerML type, so a KerML type declaration
// is a valid target of any kind.
func TestTypeCheckKerMLTypeTargetNoMismatch(t *testing.T) {
	for _, src := range []string{
		"classifier T; part def Car { part p : T; }",
		"behavior B; action def Move { action a : B; }",
		"struct S; part def Car specializes S;",
	} {
		if diags := typeDiags(t, src); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
}

// That a definition may not subset or redefine a feature is a property of the
// declaration, so the target's kind does not excuse it.
func TestTypeCheckDefinitionSubsetsKerMLType(t *testing.T) {
	for _, src := range []string{
		"classifier T; part def Car subsets T;",
		"classifier T; part def Car redefines T;",
	} {
		diags := typeDiags(t, src)
		if len(diags) != 1 {
			t.Fatalf("%s: expected one type diagnostic, got %v", src, diags)
		}
		if !strings.Contains(diags[0].Message, "a definition may not") {
			t.Errorf("%s: got %q", src, diags[0].Message)
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
