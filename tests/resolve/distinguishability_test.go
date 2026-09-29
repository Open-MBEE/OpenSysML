package resolve_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
)

// Two owned members of one name are each reported as a warning, not an error:
// the pilot's validateNamespaceDistinguishability warns at both declarations
// and the model still resolves (KerML 7.2.2, SysML 7.6.1).
func TestDuplicateOwnedMemberNamesAreWarnings(t *testing.T) {
	const src = `package P {
	private import ScalarValues::*;
	part def A { attribute x : Real; }
	part def A { attribute y : Real; }
}`
	r := resolvedWithLibraries(t, src)
	if len(r.Diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %v", len(r.Diagnostics), r.Diagnostics)
	}
	for _, d := range r.Diagnostics {
		if !d.Warning {
			t.Errorf("%q reported as an error, want a warning", d.Message)
		}
		if d.Code != resolve.CodeNameConflict || d.Message != "Duplicate of other owned member name" {
			t.Errorf("got %s %q, want %s %q", d.Code, d.Message, resolve.CodeNameConflict, "Duplicate of other owned member name")
		}
		if got := src[d.Span.Offset:d.Span.End()]; got != "A" {
			t.Errorf("diagnostic sits on %q, want the repeated name", got)
		}
	}
}
