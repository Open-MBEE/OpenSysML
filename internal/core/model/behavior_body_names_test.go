package model

import (
	"strings"
	"testing"
)

// A body written in the part performing it is inside that part's namespace, so
// a name only the part declares resolves.
func TestInlinePerformedActionNamesThePerformersFeature(t *testing.T) {
	src := `package P {
		private import ScalarValues::*;
		part def Host {
			attribute touched : Integer = 0;
			perform action t {
				action step { assign touched := touched + 1; }
				first step;
			}
		}
	}`
	if got := diagnoseSource(t, "inline-bare.sysml", src); len(got) != 0 {
		t.Fatalf("diagnostics = %v, want none", got)
	}
}

// `this` in a body a part owns names that part, so a chain through it reaches
// the part's features.
func TestInlinePerformedActionNamesThePerformerThroughThis(t *testing.T) {
	src := `package P {
		private import ScalarValues::*;
		part def Host {
			attribute touched : Integer = 0;
			perform action t {
				action step { assign this.touched := this.touched + 1; }
				first step;
			}
		}
	}`
	if got := diagnoseSource(t, "inline-this.sysml", src); len(got) != 0 {
		t.Fatalf("diagnostics = %v, want none", got)
	}
}

// A body written on its own resolves in its own namespace: a feature only the
// performing object declares is not visible there, and the runtime refuses the
// same name (runtime.TestRuntimeRobustness/standalone_action_*).
func TestStandaloneActionDoesNotNameThePerformersFeature(t *testing.T) {
	src := `package P {
		private import ScalarValues::*;
		action def Touch {
			action step { assign touched := touched + 1; }
			first step;
		}
		part def Host {
			attribute touched : Integer = 0;
			perform action t : Touch;
		}
	}`
	got := diagnoseSource(t, "standalone-bare.sysml", src)
	if len(got) == 0 {
		t.Fatalf("diagnostics = none, want an unresolved reference for touched")
	}
	for _, msg := range got {
		if !strings.Contains(msg, "touched") {
			t.Errorf("diagnostic %q, want every one to name touched", msg)
		}
	}
}

// `this` in a body written on its own is the performance itself, which holds no
// feature of the object performing it.
func TestStandaloneActionDoesNotReachThePerformerThroughThis(t *testing.T) {
	src := `package P {
		private import ScalarValues::*;
		action def Touch {
			action step { assign this.touched := 1; }
			first step;
		}
		part def Host {
			attribute touched : Integer = 0;
			perform action t : Touch;
		}
	}`
	got := diagnoseSource(t, "standalone-this.sysml", src)
	if len(got) == 0 {
		t.Fatalf("diagnostics = none, want an unresolved member for touched")
	}
}

// The parameters a binding binds are how an object hands values to a behaviour
// written on its own, and are visible to analysis on both sides.
func TestStandaloneActionExchangesValuesThroughParameters(t *testing.T) {
	src := `package P {
		private import ScalarValues::*;
		action def Mark {
			in seed : Integer;
			out counted : Integer;
			action step { assign counted := seed + 1; }
			first step;
		}
		part def Host {
			attribute seen : Integer = 4;
			perform action marking : Mark { in seed = seen; }
			attribute total : Integer = marking.counted;
		}
	}`
	if got := diagnoseSource(t, "standalone-parameters.sysml", src); len(got) != 0 {
		t.Fatalf("diagnostics = %v, want none", got)
	}
}
