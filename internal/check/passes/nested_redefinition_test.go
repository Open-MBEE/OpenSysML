package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// nestedDiags runs the pass over src at the given mode and returns its findings.
func nestedDiags(t *testing.T, src string, mode diag.ConformanceMode) []diag.Diagnostic {
	t.Helper()
	root, pd, idx := analyzeInputs(t, "n.sysml", src)
	if len(pd) != 0 {
		t.Fatalf("parse errors %+v", pd)
	}
	ctx := NewContextWithOptions("n.sysml", source.KindSysML, idx, pd, Options{Conformance: mode})
	return NestedRedefinitionPass{}.Run(ctx, "n.sysml", root)
}

// A chain redefinition applies below the member it names, which is spec
// semantics: the pass reports nothing for one.
func TestNestedRedefinitionSilentOnAPlainChain(t *testing.T) {
	src := `package P {
		private import ScalarValues::Real;
		part def Leaf { attribute value : Real; }
		part def Mid { part leaf : Leaf; }
		part def Top { part mid : Mid; }
		part top : Top { attribute :>> mid.leaf.value = 9.0; }
	}`
	if got := nestedDiags(t, src, diag.ConformanceDefault); len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
	if got := nestedDiags(t, src, diag.ConformanceStrict); len(got) != 0 {
		t.Fatalf("strict: got %+v, want no diagnostics", got)
	}
}

// A chain crossing a reference usage is an error in every mode: nothing below
// the reference owns an object to redefine on.
func TestNestedRedefinitionThroughReference(t *testing.T) {
	src := `package P {
		private import ScalarValues::Real;
		part def Leaf { attribute value : Real; }
		part def Mid { ref leaf : Leaf; }
		part def Top { part mid : Mid; }
		part top : Top { attribute :>> mid.leaf.value = 9.0; }
	}`
	got := nestedDiags(t, src, diag.ConformanceDefault)
	if len(got) != 1 {
		t.Fatalf("got %d diagnostics %+v, want one error", len(got), got)
	}
	if got[0].Severity != diag.SeverityError {
		t.Errorf("severity = %v, want error", got[0].Severity)
	}
	if got[0].Code != CodeRedefinitionThroughReference {
		t.Errorf("code = %q, want %q", got[0].Code, CodeRedefinitionThroughReference)
	}
	if !strings.Contains(got[0].Message, "through reference leaf") {
		t.Errorf("message = %q, want it to name the reference segment", got[0].Message)
	}
}

// A one-level redefinition and the nested-body form report nothing.
func TestNestedRedefinitionSilentOnStandard(t *testing.T) {
	src := `package P {
		private import ScalarValues::Real;
		part def Leaf { attribute value : Real; }
		part def Mid { part leaf : Leaf; }
		part def Top { part mid : Mid; }
		part top : Top {
			part :>> mid { part :>> leaf { attribute :>> value = 9.0; } }
		}
	}`
	if got := nestedDiags(t, src, diag.ConformanceDefault); len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}
