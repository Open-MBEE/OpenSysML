package passes

import (
	"strings"
	"testing"
)

// A `classifier` declares a plain KerML Classifier and every definition is a
// Classifier, a DataType among them (KerML 1.0 §8.3.2), so a classifier may
// specialize a definition of any kind.
func TestTypeCheckClassifierSpecializesAnyDefinitionOK(t *testing.T) {
	for _, src := range []string{
		"datatype D; classifier C specializes D;",
		"attribute def A; classifier C specializes A;",
		"enum def Level; classifier C specializes Level;",
		"part def P; classifier C specializes P;",
		"classifier T; classifier C specializes T;",
		"datatype D; subclassifier C specializes D;",
	} {
		if diags := typeDiags(t, src); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
}

// The classifier row is the only one that loosens: a part definition is a
// Structure, disjoint with the data values a datatype classifies (SysML v2
// §8.4.5.1), so specializing one is still an error.
func TestTypeCheckPartDefSpecializesDataTypeStillRejected(t *testing.T) {
	diags := typeDiags(t, "datatype D; part def P specializes D;")
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
	if diags[0].Message != "Cannot specialize attribute definition" {
		t.Errorf("unexpected message %q", diags[0].Message)
	}
}

// A classifier may specialize a definition of any kind, not a feature: a
// specialization's general type is a Type, and a usage is a Feature.
func TestTypeCheckClassifierSpecializesUsageStillRejected(t *testing.T) {
	diags := typeDiags(t, "part def P { attribute a; } classifier C specializes P::a;")
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
	if !strings.Contains(diags[0].Message, "target is not a definition") {
		t.Errorf("unexpected message %q", diags[0].Message)
	}
}
