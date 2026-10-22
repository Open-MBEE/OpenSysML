package model

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

var (
	subjectOrderBase = []byte("package Reqs { part def P; requirement def R { actor a : P; subject s : P; } concern def C { stakeholder owner : P; } }")
	subjectOrderUser = []byte("package Use { private import Reqs::*; requirement r : R {} concern c : C {} }")
)

// The subject-position rule reads the inherited parameter order from a definition
// held as a record exactly as from one loaded: a usage of a recorded definition
// whose first input is an actor or stakeholder is reported either way.
func TestSubjectPositionReadsRecordedInheritedOrder(t *testing.T) {
	cache, err := libs.NewCacheIn(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	inputs := []Input{
		{Name: "base.sysml", Content: subjectOrderBase, Version: 1},
		{Name: "user.sysml", Content: subjectOrderUser, Version: 1},
	}
	cold := NewWorkspace(WithRecordCache(cache))
	cold.OpenAll(inputs)
	all := cold.DiagnosticsAll([]string{"base.sysml", "user.sysml"})
	want := messagesOf(all[1])
	if len(want) != 2 {
		t.Fatalf("loaded user.sysml reports %v, want two subject-position errors", want)
	}

	ws := NewWorkspace(WithRecordCache(cache))
	ws.OpenAll(inputs)
	if !ws.Recorded("base.sysml") {
		t.Fatal("base.sysml is not recorded from a warm cache")
	}
	ws.Update("user.sysml", append(append([]byte{}, subjectOrderUser...), " // edited"...), 2)
	if !ws.Recorded("base.sysml") {
		t.Fatal("editing user.sysml hydrated base.sysml, so the recorded path is not exercised")
	}
	if got := messagesOf(ws.Diagnostics("user.sysml")); !reflect.DeepEqual(got, want) {
		t.Fatalf("user.sysml over recorded base.sysml reports %v, loaded %v", got, want)
	}
}
