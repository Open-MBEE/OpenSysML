package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

func TestTerminatePseudostateEndsOrthogonalMachine(t *testing.T) {
	r := migrateFixtureFile(t, "terminate_orthogonal")
	for _, line := range []string{
		"state def Run {",
		"entry; then regions;",
		"state regions parallel {",
		"state R1 {",
		"state R2 {",
		"action terminated terminate;",
		"transition first A accept Stop then terminated;",
		"transition first B accept Ping then C;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_end", migrate.Mapped, "written as a terminate action, which ends the state machine's performance in every region")
	wantClean(t, "terminateOrthogonal", r)

	s := session(t, r)
	meta(t, s, "%instantiate Machine")
	meta(t, s, "%state Machine::Run #1")
	if out := meta(t, s, "%send Stop"); !strings.Contains(out, "transition A -> terminated fires on it") {
		t.Fatalf("%%send Stop: %s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Execution state: Terminated") {
		t.Fatalf("Stop did not end the machine:\n%s", out)
	}
	if out := meta(t, s, "%send Ping"); strings.Contains(out, "Accepted by") || strings.Contains(out, "transition B -> C fires") {
		t.Fatalf("the terminated machine still accepted Ping:\n%s", out)
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Execution state: Terminated") {
		t.Fatalf("the sibling region remained active after Stop:\n%s", out)
	}

	prior := &migrate.Result{Notation: []byte(`
item def Stop;
item def Ping;
part def Machine {
    state def Run {
        entry; then regions;
        state regions parallel {
            state R1 {
                entry; then A;
                state A;
                transition first A accept Stop then done;
            }
            state R2 {
                entry; then B;
                state B;
                state C;
                transition first B accept Ping then C;
            }
        }
        transition first regions then done;
    }
    exhibit state run : Run;
}`)}
	s = session(t, prior)
	meta(t, s, "%instantiate Machine")
	meta(t, s, "%state Machine::Run #1")
	stop := meta(t, s, "%send Stop")
	step := meta(t, s, "%step")
	ping := meta(t, s, "%send Ping")
	if !strings.Contains(ping, "transition B -> C fires on it") {
		t.Fatalf("the prior done-target form did not preserve sibling behavior:\nsend Stop:\n%s\nstep:\n%s\nsend Ping:\n%s", stop, step, ping)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "C") {
		t.Fatalf("the prior done-target form did not advance the sibling region:\n%s", out)
	}
}

func TestTerminatePseudostateInsideCompositeState(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Signal" xmi:id="_stop" name="Stop"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_stopEvent" signal="_stop"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_machine" name="Machine" classifierBehavior="_run">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_run" name="Run">
        <region xmi:type="uml:Region" xmi:id="_outer">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_start"/>
          <subvertex xmi:type="uml:State" xmi:id="_composite" name="Composite">
            <region xmi:type="uml:Region" xmi:id="_inner">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_innerStart"/>
              <subvertex xmi:type="uml:State" xmi:id="_a" name="A"/>
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_end" kind="terminate"/>
              <transition xmi:type="uml:Transition" xmi:id="_innerEntry" source="_innerStart" target="_a"/>
              <transition xmi:type="uml:Transition" xmi:id="_stopTransition" source="_a" target="_end">
                <trigger xmi:type="uml:Trigger" xmi:id="_stopTrigger" event="_stopEvent"/>
              </transition>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_outerEntry" source="_start" target="_composite"/>
        </region>
      </ownedBehavior>
    </packagedElement>`,
		`<sysml:Block xmi:id="_block" base_Class="_machine"/>`)
	wantLine(t, r.Notation, "action terminated terminate;")
	wantLine(t, r.Notation, "transition first A accept Stop then terminated;")
	wantNote(t, r, "_end", migrate.Mapped, "written as a terminate action")
	wantClean(t, "terminateInsideComposite", r)
}

func TestUntargetedTerminatePseudostateIsWrittenCleanly(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_machine" name="Machine" classifierBehavior="_run">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_run" name="Run">
        <region xmi:type="uml:Region" xmi:id="_region">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_start"/>
          <subvertex xmi:type="uml:State" xmi:id="_active" name="Active"/>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_end" kind="terminate"/>
          <transition xmi:type="uml:Transition" xmi:id="_entry" source="_start" target="_active"/>
        </region>
      </ownedBehavior>
    </packagedElement>`,
		`<sysml:Block xmi:id="_block" base_Class="_machine"/>`)
	wantLine(t, r.Notation, "action terminated terminate;")
	wantNote(t, r, "_end", migrate.Mapped, "written as a terminate action")
	wantClean(t, "untargetedTerminate", r)
}
