package model

import (
	"strings"
	"testing"
)

// The OMG view/diagram library is vendored, so a view typed by one of its
// definitions resolves against the bundled stdlib (SysML v2 §10.5).
func TestStandardViewDefinitionsBundled(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte(`package P {
		view Model : StandardViewDefinitions::gv;
		view Interconnect : StandardViewDefinitions::InterconnectionView;
	}`), 1)
	if d := ws.Diagnostics("a.sysml"); len(d) != 0 {
		t.Fatalf("diagnostics = %d, want 0: %+v", len(d), d)
	}
}

// 'SysML Standard Diagrams' is a vendor namespace, not part of the standard
// library; the diagnostic must say the namespace is not loaded and point at the
// standard declaration rather than read as a misspelled member.
func TestVendorViewNamespaceDiagnostic(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte(`package P {
		view Model : 'SysML Standard Diagrams'::gv;
	}`), 1)
	d := ws.Diagnostics("a.sysml")
	if len(d) == 0 {
		t.Fatal("expected a diagnostic for an unloaded namespace")
	}
	for _, want := range []string{"no namespace", "SysML Standard Diagrams", "StandardViewDefinitions::gv"} {
		if !strings.Contains(d[0].Message, want) {
			t.Errorf("diagnostic %q does not mention %q", d[0].Message, want)
		}
	}
}
