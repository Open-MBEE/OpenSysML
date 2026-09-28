package view

import (
	"testing"
)

// typedStateBase declares the state definition typedStateDerived's states take
// their behaviors from, in a document of its own.
const typedStateBase = `package Lib {
    private import ScalarValues::*;
    attribute def Tick;
    state def Counting {
        attribute count : Integer = 0;
        entry action { assign count := 0; }
        do action { assign count := count + 1; }
        exit send Tick() to counter;
    }
    part counter;
}
`

// typedStateDerived types its state by Lib::Counting and steps out of it with
// an effect the state definition also declares.
const typedStateDerived = `package App {
    private import Lib::*;
    private import StandardViewDefinitions::*;
    state def Run {
        attribute total : Integer = 0;
        entry; then working;
        state working : Counting;
        transition first working accept Tick do action { assign total := total + 1; } then rest;
        state rest;
    }
    view runStates : StateTransitionView { expose App::Run; }
}
`

// A state typed by a definition in another document draws that definition's
// anonymous behaviors as written there, not by their spans in its own document.
func TestStateBehaviorTextFromTypingDocument(t *testing.T) {
	rendering, _, _ := renderModel(t, []string{"lib.sysml", "app.sysml"}, []string{typedStateBase, typedStateDerived}, "App::runStates", "App::Run")
	data := rendering.Data()
	var detail string
	for _, node := range data.Nodes {
		if node.Name == "working" {
			detail = node.Detail
		}
	}
	if want := "initial, entry / count := 0, do / count := count + 1, exit / Tick"; detail != want {
		t.Errorf("working detail = %q, want %q", detail, want)
	}
	var label string
	for _, edge := range data.Edges {
		if edge.Kind == EdgeTransition && edge.Label != "" {
			label = edge.Label
		}
	}
	if want := "accept Tick / total := total + 1"; label != want {
		t.Errorf("transition label = %q, want %q", label, want)
	}
}
