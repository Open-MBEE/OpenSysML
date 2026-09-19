package model

import "testing"

// `that` names the featuring instance of a usage's value (SysML v2 §7.6), so it
// resolves inside a constraint asserted in a usage body — the shape starkit uses
// to bound an attribute.
func TestThatResolvesInAssertedConstraint(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte(`package P {
		private import ScalarValues::*;
		attribute def Orbit {
			attribute inclination : Real = 28.5 {
				assert constraint { that >= 0 and that <= 90 }
			}
		}
	}`), 1)
	if d := ws.Diagnostics("a.sysml"); len(d) != 0 {
		t.Fatalf("diagnostics = %d, want 0: %+v", len(d), d)
	}
}
