package model

import (
	"strings"
	"testing"
)

// A qualified redefinition of a feature inherited from the standard library is
// legitimate: the library supertype is restored from the cache without a scope,
// so membership cannot be decided by comparing declaring scopes.
func TestRedefineInheritedLibraryFeatureQualified(t *testing.T) {
	ws := NewWorkspace()
	src := `package test {
	part def C {
		snapshot start :>> Parts::Part::start;
		part sub :>> Parts::Part::suboccurrences;
	}
}`
	ws.Open("test.sysml", []byte(src), 1)
	defer ws.Close("test.sysml")

	for _, d := range ws.Diagnostics("test.sysml") {
		t.Errorf("unexpected diagnostic: %v", d)
	}
}

// A qualified redefinition of a feature the owner does not inherit is still an
// error.
func TestRedefineUninheritedFeatureReported(t *testing.T) {
	ws := NewWorkspace()
	src := `package test {
	part def A { part w; }
	part def B { part w2 :>> A::w; }
}`
	ws.Open("test.sysml", []byte(src), 1)
	defer ws.Close("test.sysml")

	diags := ws.Diagnostics("test.sysml")
	for _, d := range diags {
		if d.Code == "redefinition-no-inherited" {
			return
		}
	}
	t.Errorf("expected a redefinition-no-inherited diagnostic, got %v", diags)
}

func TestFlowEndsRedefineImplicitMessageEnds(t *testing.T) {
	ws := NewWorkspace()
	src := `package test {
	action source {
		out attribute output { attribute voltage; }
	}
	action target {
		in attribute input { attribute voltage; }
	}
	flow {
		end ::> source {
			attribute :>> sourceOutput :>> output.voltage;
		}
		end ::> target {
			attribute :>> targetInput :>> input.voltage;
		}
	}
}`
	ws.Open("test.sysml", []byte(src), 1)
	defer ws.Close("test.sysml")

	for _, d := range ws.Diagnostics("test.sysml") {
		if strings.HasPrefix(d.Message, "unresolved reference: sourceOutput") ||
			strings.HasPrefix(d.Message, "unresolved reference: targetInput") {
			t.Errorf("unexpected diagnostic: %v", d)
		}
	}
}

// TestFlowEndsRedefineLibraryMessageEnds covers a flow whose ends borrow names
// other than source/target: each still redefines the library end at its
// position, reached through both general flows (Flows::Message :> MessageAction, Transfer).
func TestFlowEndsRedefineLibraryMessageEnds(t *testing.T) {
	ws := NewWorkspace()
	src := `package test {
	action battery {
		out attribute output { attribute voltage; }
	}
	action resister {
		in attribute input { attribute voltage; }
	}
	flow {
		end ::> battery {
			attribute :>> sourceOutput :>> output.voltage;
		}
		end ::> resister {
			attribute :>> targetInput :>> input.voltage;
		}
	}
}`
	ws.Open("test.sysml", []byte(src), 1)
	defer ws.Close("test.sysml")

	if diags := ws.Diagnostics("test.sysml"); len(diags) != 0 {
		t.Errorf("unexpected diagnostics: %v", diags)
	}
}

func TestFlowEndsDoNotCrossImplicitMessageEnds(t *testing.T) {
	ws := NewWorkspace()
	src := `package test {
	action source;
	action target;
	flow {
		end ::> source { attribute :>> targetInput; }
		end ::> target { attribute :>> sourceOutput; }
	}
}`
	ws.Open("test.sysml", []byte(src), 1)
	defer ws.Close("test.sysml")

	var unresolved int
	for _, d := range ws.Diagnostics("test.sysml") {
		if strings.HasPrefix(d.Message, "unresolved reference: targetInput") ||
			strings.HasPrefix(d.Message, "unresolved reference: sourceOutput") {
			unresolved++
		}
	}
	if unresolved != 2 {
		t.Errorf("got %d crossed-end unresolved diagnostics, want 2", unresolved)
	}
}
