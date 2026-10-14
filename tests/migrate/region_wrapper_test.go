package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

const singleRegionWrapperMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_open" name="Open"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_openEvent" signal="_open"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_step" name="Step"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_stepEvent" signal="_step"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_leave" name="Leave"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_leaveEvent" signal="_leave"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_finish" name="Finish"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_finishEvent" signal="_finish"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_class" name="Rig" classifierBehavior="_machine">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_machine" name="Run">
      <region xmi:type="uml:Region" xmi:id="_outer" name="main">
        <subvertex xmi:type="uml:Pseudostate" xmi:id="_initial"/>
        <subvertex xmi:type="uml:State" xmi:id="_outside" name="Outside"/>
        <subvertex xmi:type="uml:State" xmi:id="_other" name="Other"/>
        <subvertex xmi:type="uml:State" xmi:id="_composite" name="S">
          <entry xmi:type="uml:OpaqueBehavior" xmi:id="_entry" name="remember">
            <language>JavaScript</language>
            <body>visits = visits + 1;</body>
          </entry>
          <exit xmi:type="uml:OpaqueBehavior" xmi:id="_exit" name="record">
            <language>JavaScript</language>
            <body>visits = visits + 10;</body>
          </exit>
          <region xmi:type="uml:Region" xmi:id="_inner" name="R">
            <subvertex xmi:type="uml:Pseudostate" xmi:id="_innerInitial"/>
            <subvertex xmi:type="uml:Pseudostate" xmi:id="_history" name="history" kind="shallowHistory"/>
            <subvertex xmi:type="uml:State" xmi:id="_a" name="A"/>
            <subvertex xmi:type="uml:State" xmi:id="_b" name="B"/>
            <subvertex xmi:type="uml:FinalState" xmi:id="_final"/>
            <transition xmi:type="uml:Transition" xmi:id="_innerStart" source="_innerInitial" target="_a"/>
            <transition xmi:type="uml:Transition" xmi:id="_advance" source="_a" target="_b">
              <trigger xmi:type="uml:Trigger" xmi:id="_advanceTrigger" event="_stepEvent"/>
            </transition>
            <transition xmi:type="uml:Transition" xmi:id="_leaveState" source="_b" target="_outside">
              <trigger xmi:type="uml:Trigger" xmi:id="_leaveTrigger" event="_leaveEvent"/>
            </transition>
            <transition xmi:type="uml:Transition" xmi:id="_finishState" source="_b" target="_final">
              <trigger xmi:type="uml:Trigger" xmi:id="_finishTrigger" event="_finishEvent"/>
            </transition>
          </region>
        </subvertex>
        <transition xmi:type="uml:Transition" xmi:id="_start" source="_initial" target="_outside"/>
        <transition xmi:type="uml:Transition" xmi:id="_openComposite" source="_outside" target="_a">
          <trigger xmi:type="uml:Trigger" xmi:id="_openTrigger" event="_openEvent"/>
        </transition>
        <transition xmi:type="uml:Transition" xmi:id="_complete" source="_composite" target="_other"/>
      </region>
      </ownedBehavior>
    </packagedElement>`

const singleRegionWrapperApplications = `
  <sysml:Block xmi:id="_block" base_Class="_class"/>`

func TestSingleRegionCompositeStateIsWrapped(t *testing.T) {
	r := migrateDocument(t, singleRegionWrapperMachine, singleRegionWrapperApplications)
	for _, line := range []string{
		"state S parallel {",
		"state R {",
		"entry; then A;",
		"#StateMachines::shallowHistory state history;",
		"transition first Outside accept Open then S.R.A;",
		"transition first A accept Step then B;",
		"transition first S.R.B accept Leave then Outside;",
		"transition first B accept Finish then done;",
		"entry action remember",
		"exit action record",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_inner", migrate.Mapped, "the region is written as the sub-state R of the parallel state S, as a composite state's regions are")
	wantClean(t, "single-region composite state wrapper", r)

	s := session(t, r)
	meta(t, s, "%instantiate Rig")
	meta(t, s, "%state Rig::Run #1")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Outside") {
		t.Fatalf("initial state: %s", out)
	}
	meta(t, s, "%send Open")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: A") {
		t.Fatalf("Open did not enter S.R.A:\n%s", out)
	}
	meta(t, s, "%send Step")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: B") {
		t.Fatalf("Step did not enter S.R.B:\n%s", out)
	}
	meta(t, s, "%send Leave")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Outside") {
		t.Fatalf("Leave did not exit S:\n%s", out)
	}
	meta(t, s, "%send Open")
	meta(t, s, "%step")
	meta(t, s, "%send Step")
	meta(t, s, "%step")
	meta(t, s, "%send Finish")
	meta(t, s, "%step")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Other") {
		t.Fatalf("R's final state did not complete S and fire its completion transition:\n%s", out)
	}
}

func TestSingleRegionCompositeStateConnectionPointsStayInRegion(t *testing.T) {
	xmi := `
    <packagedElement xmi:type="uml:Signal" xmi:id="_go" name="Go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_goEvent" signal="_go"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_class" name="Rig" classifierBehavior="_machine">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_machine" name="Run">
      <region xmi:type="uml:Region" xmi:id="_outer" name="main">
        <subvertex xmi:type="uml:Pseudostate" xmi:id="_initial"/>
        <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle"/>
        <subvertex xmi:type="uml:State" xmi:id="_composite" name="S">
          <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_in" name="in" kind="entryPoint"/>
          <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_out" name="out" kind="exitPoint"/>
          <region xmi:type="uml:Region" xmi:id="_inner" name="R">
            <subvertex xmi:type="uml:Pseudostate" xmi:id="_innerInitial"/>
            <subvertex xmi:type="uml:State" xmi:id="_active" name="Active"/>
            <transition xmi:type="uml:Transition" xmi:id="_innerStart" source="_innerInitial" target="_active"/>
            <transition xmi:type="uml:Transition" xmi:id="_entryRoute" source="_in" target="_active"/>
            <transition xmi:type="uml:Transition" xmi:id="_exitRoute" source="_active" target="_out"/>
          </region>
        </subvertex>
        <transition xmi:type="uml:Transition" xmi:id="_start" source="_initial" target="_idle"/>
        <transition xmi:type="uml:Transition" xmi:id="_enter" source="_idle" target="_in">
          <trigger xmi:type="uml:Trigger" xmi:id="_enterTrigger" event="_goEvent"/>
        </transition>
        <transition xmi:type="uml:Transition" xmi:id="_leave" source="_out" target="_idle"/>
      </region>
      </ownedBehavior>
    </packagedElement>`
	r := migrateDocument(t, xmi, singleRegionWrapperApplications)
	for _, line := range []string{"state S parallel {", "state R {", "#StateMachines::junction state 'in';", "#StateMachines::junction state 'out';", "transition first 'in' then Active;", "transition first Active then 'out';", "transition first Idle accept Go then S.R.'in';", "transition first S.R.'out' then Idle;"} {
		wantLine(t, r.Notation, line)
	}
	wantClean(t, "wrapped state connection points", r)
}

func TestStateMachineSingleRegionRemainsFlattened(t *testing.T) {
	xmi := `
    <packagedElement xmi:type="uml:Class" xmi:id="_class" name="Rig" classifierBehavior="_machine">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_machine" name="Run">
      <region xmi:type="uml:Region" xmi:id="_main" name="main">
        <subvertex xmi:type="uml:Pseudostate" xmi:id="_initial"/>
        <subvertex xmi:type="uml:State" xmi:id="_active" name="Active"/>
        <transition xmi:type="uml:Transition" xmi:id="_start" source="_initial" target="_active"/>
      </region>
      </ownedBehavior>
    </packagedElement>`
	r := migrateDocument(t, xmi, singleRegionWrapperApplications)
	wantLine(t, r.Notation, "entry; then Active;")
	wantLine(t, r.Notation, "state Active;")
	if strings.Contains(string(r.Notation), "parallel") || strings.Contains(string(r.Notation), "state main {") {
		t.Errorf("the StateMachine's single region was wrapped:\n%s", r.Notation)
	}
	wantClean(t, "single-region state machine", r)
}

func TestUnnamedCompositeRegionGetsFreshStateName(t *testing.T) {
	xmi := `
    <packagedElement xmi:type="uml:Class" xmi:id="_class" name="Rig" classifierBehavior="_machine">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_machine" name="Run">
      <region xmi:type="uml:Region" xmi:id="_outer">
        <subvertex xmi:type="uml:Pseudostate" xmi:id="_initial"/>
        <subvertex xmi:type="uml:State" xmi:id="_composite" name="S">
          <ownedAttribute xmi:type="uml:Property" xmi:id="_nameCollision" name="region">` + integerHref + `</ownedAttribute>
          <region xmi:type="uml:Region" xmi:id="_inner">
            <subvertex xmi:type="uml:Pseudostate" xmi:id="_innerInitial"/>
            <subvertex xmi:type="uml:State" xmi:id="_active" name="Active"/>
            <transition xmi:type="uml:Transition" xmi:id="_innerStart" source="_innerInitial" target="_active"/>
          </region>
        </subvertex>
        <transition xmi:type="uml:Transition" xmi:id="_start" source="_initial" target="_composite"/>
      </region>
      </ownedBehavior>
    </packagedElement>`
	r := migrateDocument(t, xmi, singleRegionWrapperApplications)
	for _, line := range []string{"state S parallel {", "state region2 {", "entry; then Active;"} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_inner", migrate.Mapped, "the region is written as the sub-state region2 of the parallel state S")
	wantClean(t, "unnamed composite region", r)
}
