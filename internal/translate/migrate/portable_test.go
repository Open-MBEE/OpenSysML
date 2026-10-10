package migrate

import (
	"strings"
	"testing"
)

// Inlining follows what an inlined library refers to in turn: MOSA imports
// DocumentQueries, so both are appended.
func TestInlineLibrariesFollowsWhatALibraryRefersTo(t *testing.T) {
	out, s := inlineLibraries([]byte("package P { private import MOSA::*; }\n"))
	if want := []string{"MOSA", "DocumentQueries"}; strings.Join(s.Inlined, ",") != strings.Join(want, ",") {
		t.Errorf("inlined %v, want %v", s.Inlined, want)
	}
	for _, decl := range []string{"library package MOSA {", "library package DocumentQueries {"} {
		if !strings.Contains(string(out), decl) {
			t.Errorf("output lacks %q", decl)
		}
	}
	if len(s.NotInlined) > 0 {
		t.Errorf("not inlined: %v", s.NotInlined)
	}
}

// A KerML library with no SysML spelling is left referenced and accounted for.
func TestInlineLibrariesLeavesAKerMLLibraryWithNoSysMLSpelling(t *testing.T) {
	notation := []byte("part p { ref s : StateActivity::StateAction; }\n")
	out, s := inlineLibraries(notation)
	if string(out) != string(notation) {
		t.Errorf("output changed:\n%s", out)
	}
	if len(s.Inlined) > 0 || s.NotInlined["StateActivity"] == "" {
		t.Errorf("inlined %v, not inlined %v; want StateActivity not inlined", s.Inlined, s.NotInlined)
	}
	if summary := (&Report{Libraries: s}).Summary(); !strings.Contains(summary, "StateActivity not inlined: a KerML library with no SysML spelling") {
		t.Errorf("summary = %s", summary)
	}
}

// A name that is a library's only as a nested member does not call for the library.
func TestInlineLibrariesIgnoresAnUnqualifiedName(t *testing.T) {
	out, s := inlineLibraries([]byte("package Stochastic;\npart p;\n"))
	if len(s.Inlined) > 0 || strings.Contains(string(out), "library package") {
		t.Errorf("inlined %v for a package merely named like a library", s.Inlined)
	}
}
