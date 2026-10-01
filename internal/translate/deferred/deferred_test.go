package deferred

import (
	"strings"
	"testing"
)

// textWriter renders an encoding as indented notation and records the names
// the encoding made up, the way the migrator's and the referee's sinks do.
type textWriter struct {
	sb     strings.Builder
	depth  int
	madeUp []string
}

func (w *textWriter) Line(s string) {
	w.sb.WriteString(strings.Repeat("    ", w.depth) + s + "\n")
}

func (w *textWriter) Block(header string, body func()) {
	w.Line(header + " {")
	w.depth++
	body()
	w.depth--
	w.Line("}")
}

func (w *textWriter) Raw(text string) { w.sb.WriteString(text) }

func (w *textWriter) MadeUp(name string) { w.madeUp = append(w.madeUp, name) }

func oneSignal() *Encoding {
	return &Encoding{
		Buffer: "keepDoor", Split: "keepDoorSplit", Flush: "flushDoor",
		Including: "SequenceFunctions::including",
		Signals: []Signal{{
			Ref: "Door", Buffer: "deferredDoor", Item: "kept", Clear: "clearDoor",
			Loops: []Loop{{Receive: "receiveDoor", Keep: "keepDoorOnce", Payload: "door", Accept: "Door"}},
		}},
	}
}

func TestOneSignalOneRouteIsALoopWithNoFork(t *testing.T) {
	w := &textWriter{}
	e := oneSignal()
	e.Items(w)
	e.Do(w, nil)
	e.Exit(w, nil)
	want := `item deferredDoor : Door[*] ordered;
do action keepDoor {
    first start then receiveDoor;
    action receiveDoor accept door : Door;
    then action keepDoorOnce { assign deferredDoor := SequenceFunctions::including(deferredDoor, receiveDoor.door); }
    then receiveDoor;
}
exit action flushDoor {
    for kept in deferredDoor { send kept to self; }
    then action clearDoor { assign deferredDoor := (); }
}
`
	if got := w.sb.String(); got != want {
		t.Errorf("encoding:\n%s\nwant:\n%s", got, want)
	}
	wantMadeUp := []string{"deferredDoor", "receiveDoor", "keepDoorOnce", "keepDoor", "clearDoor", "flushDoor"}
	if got := strings.Join(w.madeUp, " "); got != strings.Join(wantMadeUp, " ") {
		t.Errorf("made-up names %q, want %q", got, wantMadeUp)
	}
	if e.LoopCount() != 1 {
		t.Errorf("LoopCount = %d, want 1", e.LoopCount())
	}
}

func TestRoutesAndOwnBehaviorRunBesideEachOtherUnderAFork(t *testing.T) {
	w := &textWriter{}
	e := oneSignal()
	e.Signals[0].Kind = "attribute"
	e.Signals[0].Loops = append(e.Signals[0].Loops, Loop{
		Receive: "receiveDoorViaP", Keep: "keepDoorViaP", Payload: "door", Accept: "Door", Via: "p",
	})
	e.Items(w)
	e.Do(w, func() Own {
		return Own{Text: "    action work { }\n", Written: true, Run: "work", MadeUp: "work"}
	})
	e.Exit(w, func() Own {
		return Own{Text: "    action bye { }\n", Written: true, Run: "bye"}
	})
	want := `attribute deferredDoor : Door[*] ordered;
do action keepDoor {
    first start then keepDoorSplit;
    fork keepDoorSplit;
    then work;
    then receiveDoor;
    then receiveDoorViaP;
    action work { }
    action receiveDoor accept door : Door;
    then action keepDoorOnce { assign deferredDoor := SequenceFunctions::including(deferredDoor, receiveDoor.door); }
    then receiveDoor;
    action receiveDoorViaP accept door : Door via p;
    then action keepDoorViaP { assign deferredDoor := SequenceFunctions::including(deferredDoor, receiveDoorViaP.door); }
    then receiveDoorViaP;
}
exit action flushDoor {
    action bye { }
    then for kept in deferredDoor { send kept to self; }
    then action clearDoor { assign deferredDoor := (); }
}
`
	if got := w.sb.String(); got != want {
		t.Errorf("encoding:\n%s\nwant:\n%s", got, want)
	}
	if e.LoopCount() != 2 {
		t.Errorf("LoopCount = %d, want 2", e.LoopCount())
	}
	if got := strings.Join(w.madeUp, " "); !strings.Contains(got, "keepDoorSplit") || !strings.Contains(got, "work") {
		t.Errorf("made-up names %q lack the fork or the anonymous do behavior", got)
	}
}

// A rendering of comments alone declares no action to run, so one loop still
// needs no fork and the comments sit in the body ahead of it.
func TestUnwrittenOwnBehaviorAddsNoFork(t *testing.T) {
	w := &textWriter{}
	e := oneSignal()
	e.Do(w, func() Own { return Own{Text: "    // kept\n"} })
	got := w.sb.String()
	if strings.Contains(got, "fork") || !strings.Contains(got, "first start then receiveDoor;\n    // kept\n") {
		t.Errorf("encoding:\n%s", got)
	}
}
