package passes

import (
	"strings"
	"testing"
)

// The VerificationMethod bodies the standard library spells are checked like
// any metadata body: an imported or qualified name resolves, and a bound
// value conforms to VerificationMethodKind.
func TestVerificationMethodBodiesCheckCleanWhenImported(t *testing.T) {
	src := `package S8 {
	private import VerificationCases::*;
	verification def T {
		metadata VerificationMethod { kind = VerificationMethodKind::test; }
	}
	verification def U {
		@VerificationMethod { kind = (VerificationMethodKind::test, VerificationMethodKind::analyze); }
	}
}`
	if diags := w9cLibraryDiags(t, src, false); len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
}

// An enum literal outside VerificationMethodKind is not a kind a method takes.
func TestVerificationMethodKindLiteralMustResolve(t *testing.T) {
	src := `package S8 {
	private import VerificationCases::*;
	verification def T {
		metadata VerificationMethod { kind = VerificationMethodKind::bogus; }
	}
}`
	diags := w9cLibraryDiags(t, src, false)
	if len(diags) == 0 {
		t.Fatal("expected an unresolved diagnostic")
	}
	for _, d := range diags {
		if !strings.Contains(d.Message, "VerificationMethodKind::bogus") {
			t.Fatalf("unexpected diagnostic %q", d.Message)
		}
	}
}

// A kind from another enumeration and a bare number are refused as the
// feature's type names them.
func TestVerificationMethodBodyValueMustBeAVerificationMethodKind(t *testing.T) {
	src := `package S8 {
	private import VerificationCases::*;
	verification def T {
		metadata VerificationMethod { kind = VerdictKind::pass; }
	}
	verification def U {
		@VerificationMethod { kind = 3; }
	}
}`
	diags := w9cLibraryDiags(t, src, false)
	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics, got %v", diags)
	}
	for _, d := range diags {
		if !strings.Contains(d.Message, "VerificationMethodKind") {
			t.Fatalf("diagnostic %q names no VerificationMethodKind", d.Message)
		}
	}
}
