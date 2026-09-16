package model

import (
	"reflect"
	"strings"
	"testing"
)

const readingModel = `package Machines {
	state def Ops {
		entry; then idle;
		state idle; // the machine's rest
		state busy; /* a comment, not a note */
		transition first idle then busy;
	}
}
package MachineViews {
	private import StandardViewDefinitions::*;
	view opsView : StateTransitionView { expose Machines::Ops; }
}
`

// A reading answers for one generation of the documents: the rendering, the
// runtime and the declarations it hands out are of the same documents, and an
// edit moves the workspace to a generation the reading's runtime is not of.
func TestReadingIsOfOneGeneration(t *testing.T) {
	ws := openDoc(t, "m.sysml", readingModel)
	before := ws.Generation()
	var rt *Runtime
	err := ws.Read(func(r *Reading) error {
		if r.Generation() != before {
			t.Errorf("reading is of generation %d, want %d", r.Generation(), before)
		}
		rendering, snapshot, err := r.RenderView("m.sysml", "MachineViews::opsView")
		if err != nil {
			return err
		}
		if snapshot.Rendered.Version != 1 || rendering == nil {
			t.Errorf("rendering of version %d, want 1", snapshot.Rendered.Version)
		}
		if rt, err = r.NewRuntime(); err != nil {
			return err
		}
		if rt.Generation() != r.Generation() {
			t.Errorf("runtime of generation %d, reading of %d", rt.Generation(), r.Generation())
		}
		target := r.Declared("m.sysml", "Machines::Ops")
		if target == nil {
			t.Fatal("Machines::Ops not declared")
		}
		if got, want := r.Dependencies(target), rt.Dependencies(rt.Declared("m.sysml", "Machines::Ops")); !reflect.DeepEqual(got, want) {
			t.Errorf("reading and runtime disagree on what Machines::Ops reads:\n%v\n%v", got, want)
		}
		if deps := r.Dependencies(target); len(deps) == 0 || deps[0].Text != r.DeclarationText(target) || !strings.HasSuffix(deps[0].Text, "}") {
			t.Errorf("the target's dependency text = %q, want its declaration less the trivia after it", deps)
		}
		if got := r.DeclarationText(r.Declared("m.sysml", "Machines::Ops::idle")); got != "state idle;" {
			t.Errorf("idle's declaration text = %q, want it cut before the note after it", got)
		}
		if got := r.DeclarationText(r.Declared("m.sysml", "Machines::Ops::busy")); got != "state busy;" {
			t.Errorf("busy's declaration text = %q, want it cut before the comment after it", got)
		}
		if r.DeclaredView("m.sysml", "MachineViews::opsView") == nil {
			t.Error("MachineViews::opsView not declared as a view")
		}
		if r.DeclaredView("m.sysml", "#state") != nil {
			t.Error("a pseudo-view is declared")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if ws.Generation() != before {
		t.Errorf("reading moved the workspace from generation %d to %d", before, ws.Generation())
	}

	// An edit of the same length leaves every span where it was; the generation
	// still tells the documents apart.
	ws.Update("m.sysml", []byte(strings.Replace(readingModel, "state busy;", "state lazy;", 1)), 2)
	if ws.Generation() == rt.Generation() {
		t.Errorf("the workspace is still at the runtime's generation %d after an edit", rt.Generation())
	}
	if v, _ := rt.Version("m.sysml"); v != 1 {
		t.Errorf("runtime is of version %d after the edit, want 1", v)
	}
}
