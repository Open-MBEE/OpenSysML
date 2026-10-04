package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// ovenMachine is a block whose classifier behavior is a state machine with a
// state of two orthogonal regions, entry and exit behaviors, a guarded pair of
// transitions leaving a choice, one guard that is a v2 expression and one that
// is not, a change event on an attribute an operation writes, an internal
// transition, an absolute time event and a transition with two triggers.
const ovenMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_on" name="TurnOn"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_off" name="TurnOff"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_door" name="Door"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_onEv" signal="_on"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_offEv" signal="_off"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_doorEv" signal="_door"/>
    <packagedElement xmi:type="uml:ChangeEvent" xmi:id="_hotEv">
      <changeExpression xmi:type="uml:OpaqueExpression" xmi:id="_hotX"><body>temperature > 200.0</body></changeExpression>
    </packagedElement>
    <packagedElement xmi:type="uml:TimeEvent" xmi:id="_noon" isRelative="false">
      <when xmi:type="uml:TimeExpression" xmi:id="_noonw">
        <expr xmi:type="uml:LiteralString" xmi:id="_noonl" value="12h"/>
      </when>
    </packagedElement>
    <packagedElement xmi:type="uml:TimeEvent" xmi:id="_tick" isRelative="true">
      <when xmi:type="uml:TimeExpression" xmi:id="_tickw">
        <expr xmi:type="uml:LiteralString" xmi:id="_tickl" value="500ms"/>
      </when>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_oven" name="Oven" classifierBehavior="_sm">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_temp" name="temperature">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_temp0" value="20.0"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_cycles" name="cycles">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_cycles0" value="0"/>
      </ownedAttribute>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_heat" name="heat" method="_heatM"/>
      <ownedBehavior xmi:type="uml:OpaqueBehavior" xmi:id="_heatM" name="heat" specification="_heat">
        <language>JavaScript</language>
        <body>temperature = 250.0;</body>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Baking">
        <region xmi:type="uml:Region" xmi:id="_r0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init0"/>
          <subvertex xmi:type="uml:State" xmi:id="_off_s" name="Off">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dDoor" event="_doorEv"/>
            <entry xmi:type="uml:OpaqueBehavior" xmi:id="_offEntry" name="cool">
              <language>JavaScript</language>
              <body>temperature = 20.0;</body>
            </entry>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_on_s" name="On">
            <exit xmi:type="uml:OpaqueBehavior" xmi:id="_onExit" name="count">
              <language>JavaScript</language>
              <body>cycles = cycles + 1;</body>
            </exit>
            <region xmi:type="uml:Region" xmi:id="_rHeat" name="heating">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_initH"/>
              <subvertex xmi:type="uml:State" xmi:id="_warm" name="Warming"/>
              <subvertex xmi:type="uml:State" xmi:id="_hot" name="Hot"/>
              <transition xmi:type="uml:Transition" xmi:id="_tHi" source="_initH" target="_warm"/>
              <transition xmi:type="uml:Transition" xmi:id="_tHot" source="_warm" target="_hot">
                <trigger xmi:type="uml:Trigger" xmi:id="_trHot" event="_hotEv"/>
              </transition>
            </region>
            <region xmi:type="uml:Region" xmi:id="_rLight" name="lighting">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_initL"/>
              <subvertex xmi:type="uml:State" xmi:id="_lit" name="Lit"/>
              <subvertex xmi:type="uml:State" xmi:id="_dark" name="Dark"/>
              <transition xmi:type="uml:Transition" xmi:id="_tLi" source="_initL" target="_lit"/>
              <transition xmi:type="uml:Transition" xmi:id="_tDark" source="_lit" target="_dark">
                <trigger xmi:type="uml:Trigger" xmi:id="_trDark" event="_tick"/>
              </transition>
              <transition xmi:type="uml:Transition" xmi:id="_tLit" source="_dark" target="_lit">
                <trigger xmi:type="uml:Trigger" xmi:id="_trLit" event="_tick"/>
              </transition>
            </region>
          </subvertex>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_pick" kind="choice"/>
          <subvertex xmi:type="uml:State" xmi:id="_rest" name="Resting">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dNoon" event="_noon"/>
          </subvertex>
          <subvertex xmi:type="uml:FinalState" xmi:id="_fin"/>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init0" target="_off_s"/>
          <transition xmi:type="uml:Transition" xmi:id="_tOn" source="_off_s" target="_on_s">
            <trigger xmi:type="uml:Trigger" xmi:id="_trOn" event="_onEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tOff" source="_on_s" target="_pick">
            <trigger xmi:type="uml:Trigger" xmi:id="_trOff" event="_offEv"/>
            <trigger xmi:type="uml:Trigger" xmi:id="_trDoor" event="_doorEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tWorn" source="_pick" target="_fin">
            <guard xmi:type="uml:Constraint" xmi:id="_gWorn">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_gWornX"><body>cycles >= 3</body></specification>
            </guard>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tFresh" source="_pick" target="_rest">
            <guard xmi:type="uml:Constraint" xmi:id="_gFresh">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_gFreshX"><body>else</body></specification>
            </guard>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tRested" source="_rest" target="_off_s">
            <trigger xmi:type="uml:Trigger" xmi:id="_trNoon" event="_noon"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tSelf" kind="internal" source="_rest" target="_rest">
            <trigger xmi:type="uml:Trigger" xmi:id="_trSelf" event="_doorEv"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`

const ovenApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_oven"/>`

// Orthogonal regions, a choice with an else branch, v2 guards, change and absolute time
// events, an internal transition and a two-trigger transition are written executably. The result runs.
func TestStateMachineWithOrthogonalRegionsAndGuards(t *testing.T) {
	r := migrateDocument(t, ovenMachine, ovenApplications)
	for _, line := range []string{
		"state def Baking {",
		"attribute instant : Time::TimeInstantValue = 43200.0 [SI::s];",
		"entry; then Off;",
		"state Off {",
		"@MigrationMetadata::DeferredEvent { ref :>> signal : Door; }",
		"item deferred : Door[*] ordered;",
		"entry action cool {",
		"assign context.temperature := 20.0;",
		"state On {",
		"exit action count {",
		"assign context.cycles := context.cycles + 1;",
		"entry; then regions;",
		"state regions parallel {",
		"state heating {",
		"entry; then Warming;",
		"transition first Warming accept when context.temperature > 200.0 then Hot;",
		"state lighting {",
		"entry; then Lit;",
		"transition first Lit accept after 0.5 [SI::s] then Dark;",
		"transition first Dark accept after 0.5 [SI::s] then Lit;",
		"transition first regions then done;",
		"#StateMachines::choice state choice;",
		"private import StateMachines::*;",
		"state Resting;",
		"transition first Off accept TurnOn then On;",
		"transition first On accept TurnOff then choice;",
		"transition first On accept Door then choice;",
		"transition first choice if context.cycles >= 3 then done;",
		"transition first choice then Resting;",
		"transition first Resting accept at instant then Off;",
		"transition first Resting accept Door then Resting;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantDeferredDoorEncoding(t, r)
	wantNote(t, r, "_rHeat", migrate.Mapped, "an orthogonal region is written as a sub-state of the parallel state regions")
	wantNote(t, r, "_offEntry", migrate.Approximated, "the JavaScript body is written as v2 assignments")
	wantNote(t, r, "_onExit", migrate.Approximated, "the JavaScript body is written as v2 assignments")
	wantNote(t, r, "_pick", migrate.Mapped, "written as a #StateMachines::choice state pseudostate, whose guarded transitions the runtime reads when it is reached")
	wantNote(t, r, "_gWorn", migrate.Mapped, "")
	wantNote(t, r, "_gFresh", migrate.Mapped, "an else guard is written as the unguarded transition out of the choice")
	wantNote(t, r, "_tOff", migrate.Approximated, "written as 2 transitions, one per trigger")
	wantNote(t, r, "_tRested", migrate.Mapped, "")
	wantNote(t, r, "_tSelf", migrate.Approximated, "an internal transition is written as a self transition, which exits and re-enters Resting")
	wantNote(t, r, "_hotEv", migrate.Mapped, "written where a trigger refers to it, as accept when context.temperature > 200.0")
	wantNote(t, r, "_noon", migrate.Approximated, "written where a trigger refers to it, as accept at instant; the absolute time is an instant on the simulation clock")
	wantNote(t, r, "_dNoon", migrate.Unmapped, "only a signal event can be deferred, not a TimeEvent")

	// Strict output carries the deferral in the same standard notation; the
	// choice pseudostate is written as metadata in both modes.
	strict := migrateDocumentOptions(t, ovenMachine, ovenApplications, migrate.Options{Strict: true})
	wantDeferredDoorEncoding(t, strict)
	wantNote(t, strict, "_dNoon", migrate.Unmapped, "only a signal event can be deferred, not a TimeEvent")
	wantNote(t, strict, "_pick", migrate.Mapped, "written as a #StateMachines::choice state pseudostate")
	wantNoLine(t, strict.Notation, "choice choice;")
	noExtensionStatement(t, strict.Notation)
	wantNote(t, strict, "_gWorn", migrate.Mapped, "")

	r = migrateDocument(t, ovenMachine, ovenApplications)
	s := session(t, r)
	meta(t, s, "%instantiate Oven")
	meta(t, s, "%state Oven::Baking")
	if out := meta(t, s, "%send TurnOn"); !strings.Contains(out, "transition Off -> On fires on it") {
		t.Errorf("%%send TurnOn: %s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Warming") || !strings.Contains(out, "Lit") {
		t.Errorf("On did not enter both regions:\n%s", out)
	}
	if out := meta(t, s, "%advance 0.5"); !strings.Contains(out, "Advanced to 0.5") {
		t.Errorf("%%advance 0.5: %s", out)
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Dark") {
		t.Errorf("the tick did not switch the light off:\n%s", out)
	}
	meta(t, s, "%invoke #1 heat")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Hot") {
		t.Errorf("the change event did not fire on the raised temperature:\n%s", out)
	}
	if out := meta(t, s, "%send Door"); !strings.Contains(out, "transition On -> choice fires on it") {
		t.Errorf("%%send Door: %s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Resting") {
		t.Errorf("the first cycle did not rest through the choice:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : cycles"); !strings.Contains(out, "= 1") {
		t.Errorf("the exit action did not count the cycle: %s", out)
	}
	if out := meta(t, s, "%send Door"); !strings.Contains(out, "transition Resting -> Resting fires on it") {
		t.Errorf("%%send Door while Resting: %s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Resting") {
		t.Errorf("the internal transition left Resting:\n%s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Off") || !strings.Contains(out, "Time: 43200.0") {
		t.Errorf("the clock did not reach the instant and return to Off:\n%s", out)
	}

	// A Door sent while Off is kept there, and taken once On is entered.
	s = session(t, r)
	meta(t, s, "%instantiate Oven")
	meta(t, s, "%state Oven::Baking")
	meta(t, s, "%send Door")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Off") || !strings.Contains(out, "Off.deferred = [Instance") {
		t.Errorf("the Door sent while Off was not kept there:\n%s", out)
	}
	meta(t, s, "%send TurnOn")
	for i := 0; i < 4 && !strings.Contains(meta(t, s, "%current"), "Current state: Resting"); i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Resting") {
		t.Errorf("the deferred Door did not leave On once it was entered:\n%s", out)
	}
}

// wantDeferredDoorEncoding checks the oven's Off state keeps Door through the
// standard encoding: a buffer the do action's accept loop fills, which the exit
// action sends back to self. Both modes write it.
func wantDeferredDoorEncoding(t *testing.T, r *migrate.Result) {
	t.Helper()
	wantNoLine(t, r.Notation, "defer Door;")
	for _, line := range []string{
		"@MigrationMetadata::DeferredEvent { ref :>> signal : Door; }",
		"item deferred : Door[*] ordered;",
		"do action buffer {",
		"first start then receive;",
		"#MigrationMetadata::DeferredKeeper action receive accept kept : Door;",
		"then action keep { assign deferred := SequenceFunctions::including(deferred, receive.kept); }",
		"then receive;",
		"metadata MigrationMetadata::SynthesizedName about receive, keep;",
		"exit action flush {",
		"for kept in deferred { send kept to self; }",
		"then action clear { assign deferred := (); }",
		"metadata MigrationMetadata::SynthesizedName about deferred, buffer, flush;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_dDoor", migrate.Approximated, "kept in the item deferred by the accept loop of the do action buffer while the state is active, and sent to self by the exit action flush: the standard SysML v2 encoding of a deferred signal, which the state's @MigrationMetadata::DeferredEvent annotation records")
	wantNote(t, r, "_doorEv", migrate.Approximated, "deferred by 'Off' through the standard SysML v2 encoding, an accept loop keeping the signal while the state is active and an exit action sending it to self")
}

// deferringMachine is a block whose state machine defers two signals in a state
// that has a do and an exit behavior of its own, and there also defers the
// signal its outgoing transition accepts; a state before it defers a signal
// while leaving by a completion transition. The deferred signals are taken by
// a substate of the composite state entered next and by that state itself.
const deferringMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_alarm" name="Alarm"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_beep" name="Beep"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_go" name="Go"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_back" name="Back"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_backEv" signal="_back"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_alarmEv" signal="_alarm"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_beepEv" signal="_beep"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_goEv" signal="_go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_goDeferredEv" signal="_go"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_panel" name="Panel" classifierBehavior="_psm">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ticks" name="ticks">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_ticks0" value="0"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_exits" name="exits">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_exits0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_psm" name="Watch">
        <region xmi:type="uml:Region" xmi:id="_pr0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_pinit"/>
          <subvertex xmi:type="uml:State" xmi:id="_prep" name="Prep">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dPrepAlarm" event="_alarmEv"/>
            <doActivity xmi:type="uml:OpaqueBehavior" xmi:id="_prepDo" name="reset">
              <language>JavaScript</language>
              <body>ticks = 0;</body>
            </doActivity>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_wait" name="Waiting">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dAlarm" event="_alarmEv"/>
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dBeep" event="_beepEv"/>
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dGo" event="_goDeferredEv"/>
            <doActivity xmi:type="uml:OpaqueBehavior" xmi:id="_waitDo">
              <language>JavaScript</language>
              <body>ticks = ticks + 1;</body>
            </doActivity>
            <exit xmi:type="uml:OpaqueBehavior" xmi:id="_waitExit" name="leave">
              <language>JavaScript</language>
              <body>exits = exits + 1;</body>
            </exit>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_work" name="Working">
            <region xmi:type="uml:Region" xmi:id="_prw">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_winit"/>
              <subvertex xmi:type="uml:State" xmi:id="_busy" name="Busy"/>
              <subvertex xmi:type="uml:State" xmi:id="_alarmed" name="Alarmed"/>
              <transition xmi:type="uml:Transition" xmi:id="_tw0" source="_winit" target="_busy"/>
              <transition xmi:type="uml:Transition" xmi:id="_tAlarm" source="_busy" target="_alarmed">
                <trigger xmi:type="uml:Trigger" xmi:id="_trAlarm" event="_alarmEv"/>
              </transition>
            </region>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_done" name="Done"/>
          <transition xmi:type="uml:Transition" xmi:id="_tp0" source="_pinit" target="_prep"/>
          <transition xmi:type="uml:Transition" xmi:id="_tPrep" source="_prep" target="_wait"/>
          <transition xmi:type="uml:Transition" xmi:id="_tGo" source="_wait" target="_work">
            <trigger xmi:type="uml:Trigger" xmi:id="_trGo" event="_goEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tBeep" source="_work" target="_done">
            <trigger xmi:type="uml:Trigger" xmi:id="_trBeep" event="_beepEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tBack" source="_done" target="_wait">
            <trigger xmi:type="uml:Trigger" xmi:id="_trBack" event="_backEv"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`

const deferringApplications = `
  <sysml:Block xmi:id="_ps1" base_Class="_panel"/>`

// Deferred signals are kept by an accept loop per signal run beside the
// state's own do behavior, and sent back to self after its own exit behavior;
// a signal a transition out of the state accepts is not kept, and a state a
// completion transition leaves keeps none. Both modes write the same result,
// which runs: signals sent while Waiting are dispatched, in order, once it
// exits, and a later visit to Waiting replays only what that visit kept.
func TestStrictDeferredSignalsAreKeptAndReplayed(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts migrate.Options
	}{{"default", migrate.Options{}}, {"strict", migrate.Options{Strict: true}}} {
		t.Run(mode.name, func(t *testing.T) {
			checkDeferredSignalsAreKeptAndReplayed(t, migrateDocumentOptions(t, deferringMachine, deferringApplications, mode.opts))
		})
	}
	if d, s := migrateDocument(t, deferringMachine, deferringApplications), migrateDocumentOptions(t, deferringMachine, deferringApplications, migrate.Options{Strict: true}); string(d.Notation) != string(s.Notation) {
		t.Errorf("default and strict migrations of the deferring machine differ:\n--- default\n%s\n--- strict\n%s", d.Notation, s.Notation)
	}
}

func checkDeferredSignalsAreKeptAndReplayed(t *testing.T, r *migrate.Result) {
	t.Helper()
	for _, stmt := range []string{"defer Alarm;", "defer Beep;", "defer Go;"} {
		wantNoStatement(t, r.Notation, stmt)
	}
	for _, line := range []string{
		"state Prep {",
		"@MigrationMetadata::DeferredEvent { ref :>> signal : Alarm; }",
		"/* not migrated: deferrableTrigger (_dPrepAlarm) on Alarm — the completion transition (_tPrep) leaves the state once its do action ends, which the accept loop that would keep Alarm never lets it, so the deferral is dropped */",
		"state Waiting {",
		"@MigrationMetadata::DeferredEvent { ref :>> signal : Alarm; }",
		"@MigrationMetadata::DeferredEvent { ref :>> signal : Beep; }",
		"@MigrationMetadata::DeferredEvent { ref :>> signal : Go; }",
		"item deferredAlarm : Alarm[*] ordered;",
		"item deferredBeep : Beep[*] ordered;",
		"do action buffer {",
		"first start then split;",
		"fork split;",
		"then run;",
		"then receiveAlarm;",
		"then receiveBeep;",
		"action run {",
		"assign context.ticks := context.ticks + 1;",
		"#MigrationMetadata::DeferredKeeper action receiveAlarm accept keptAlarm : Alarm;",
		"then action keepAlarm { assign deferredAlarm := SequenceFunctions::including(deferredAlarm, receiveAlarm.keptAlarm); }",
		"then receiveAlarm;",
		"#MigrationMetadata::DeferredKeeper action receiveBeep accept keptBeep : Beep;",
		"then action keepBeep { assign deferredBeep := SequenceFunctions::including(deferredBeep, receiveBeep.keptBeep); }",
		"then receiveBeep;",
		"metadata MigrationMetadata::SynthesizedName about split, run, receiveAlarm, keepAlarm, receiveBeep, keepBeep;",
		"exit action flush {",
		"action leave {",
		"assign context.exits := context.exits + 1;",
		"then for keptAlarm in deferredAlarm { send keptAlarm to self; }",
		"then action clearAlarm { assign deferredAlarm := (); }",
		"then for keptBeep in deferredBeep { send keptBeep to self; }",
		"then action clearBeep { assign deferredBeep := (); }",
		"metadata MigrationMetadata::SynthesizedName about clearAlarm, clearBeep;",
		"metadata MigrationMetadata::SynthesizedName about deferredAlarm, deferredBeep, buffer, flush;",
		"transition first Waiting accept Go then Working;",
		"transition first Busy accept Alarm then Alarmed;",
		"transition first Working accept Beep then Done;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_dGo", migrate.Approximated, "the transition (_tGo) out of the state accepts the signal, which in v1 takes precedence over deferring it, so the state does not keep it; its @MigrationMetadata::DeferredEvent annotation records the deferral")
	wantNoLine(t, r.Notation, "item deferredGo : Go[*] ordered;")
	wantNote(t, r, "_goDeferredEv", migrate.Approximated, "deferred by 'Waiting', which the state's @MigrationMetadata::DeferredEvent annotation records; the state does not keep the signal, the transition (_tGo) accepting it")
	wantNote(t, r, "_dPrepAlarm", migrate.Unmapped, "the completion transition (_tPrep) leaves the state once its do action ends, which the accept loop that would keep Alarm never lets it, so the deferral is dropped")
	wantNote(t, r, "_dAlarm", migrate.Approximated, "kept in the item deferredAlarm by the accept loop of the do action buffer while the state is active, and sent to self by the exit action flush")
	wantNote(t, r, "_dBeep", migrate.Approximated, "kept in the item deferredBeep by the accept loop of the do action buffer while the state is active, and sent to self by the exit action flush")
	wantNote(t, r, "_dBeep", migrate.Approximated, "the order between occurrences of different signals is not kept; UML leaves the order of the event pool open")
	wantNote(t, r, "_waitDo", migrate.Approximated, "the JavaScript body is written as v2 assignments")
	wantNote(t, r, "_waitExit", migrate.Approximated, "the JavaScript body is written as v2 assignments")
	for id, target := range map[string]string{"_waitDo": "Panel::Watch::Waiting::buffer::run", "_waitExit": "Panel::Watch::Waiting::flush::leave"} {
		if es := entriesFor(r, id); len(es) != 1 || es[0].Target != target {
			t.Errorf("%s is named where the generated action nests it: want %s, got %+v", id, target, es)
		}
	}
	wantClean(t, "deferring", r)

	s := session(t, r)
	meta(t, s, "%instantiate Panel")
	meta(t, s, "%state Panel::Watch")
	for i := 0; i < 4 && !strings.Contains(meta(t, s, "%current"), "Current state: Waiting"); i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Waiting") {
		t.Fatalf("Prep's completion transition did not reach Waiting:\n%s", out)
	}
	meta(t, s, "%send Alarm")
	meta(t, s, "%step")
	meta(t, s, "%send Beep")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Waiting") || !strings.Contains(out, "Waiting.deferredAlarm = [Instance") || !strings.Contains(out, "Waiting.deferredBeep = [Instance") {
		t.Errorf("the Alarm and Beep sent while Waiting were not kept there:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : ticks"); !strings.Contains(out, "= 1") {
		t.Errorf("Waiting's own do behavior did not run beside the accept loops: %s", out)
	}
	if out := meta(t, s, "%send Go"); !strings.Contains(out, "transition Waiting -> Working fires on it") {
		t.Errorf("%%send Go while Waiting: %s", out)
	}
	for i := 0; i < 6 && !strings.Contains(meta(t, s, "%current"), "Current state: Done"); i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Done") {
		t.Errorf("the kept Alarm and Beep were not taken by Working once Waiting exited:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : exits"); !strings.Contains(out, "= 1") {
		t.Errorf("Waiting's own exit behavior did not run before the flush: %s", out)
	}
	meta(t, s, "%send Back")
	for i := 0; i < 4 && !strings.Contains(meta(t, s, "%current"), "Current state: Waiting"); i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Waiting") || !strings.Contains(out, "Waiting.deferredAlarm = null") || !strings.Contains(out, "Waiting.deferredBeep = null") {
		t.Errorf("Waiting's buffers were not emptied by its first exit:\n%s", out)
	}
	meta(t, s, "%send Go")
	for i := 0; i < 6; i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Busy") || strings.Contains(out, "Alarmed") {
		t.Errorf("the Alarm and Beep of the first visit were replayed again by the second:\n%s", out)
	}
}

// portDeferringMachine is a unit whose state machine defers, while Waiting, a
// command that a connector delivers to its inbox port and a ping its trigger
// takes at the side port only, and leaves Waiting on the command only once
// armed; a console issues the command over the connector.
const portDeferringMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_cmd" name="Cmd"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_ping" name="Ping"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_arm" name="Arm"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_cmdEv" signal="_cmd"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_pingEv" signal="_ping"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_armEv" signal="_arm"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_unit" name="Unit" classifierBehavior="_duty">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_inbox" name="inbox" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_side" name="side" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_armed" name="armed">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Boolean"/>
        <defaultValue xmi:type="uml:LiteralBoolean" xmi:id="_armed0" value="false"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_got" name="got">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_got0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_duty" name="Duty">
        <region xmi:type="uml:Region" xmi:id="_dr0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_dinit"/>
          <subvertex xmi:type="uml:State" xmi:id="_uwait" name="Waiting">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dCmd" event="_cmdEv"/>
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dPing" event="_pingEv" port="_side"/>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_uwork" name="Working">
            <entry xmi:type="uml:OpaqueBehavior" xmi:id="_workEntry">
              <language>JavaScript</language>
              <body>got = got + 1;</body>
            </entry>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_td0" source="_dinit" target="_uwait"/>
          <transition xmi:type="uml:Transition" xmi:id="_tArm" source="_uwait" target="_uwait">
            <trigger xmi:type="uml:Trigger" xmi:id="_trArm" event="_armEv"/>
            <effect xmi:type="uml:OpaqueBehavior" xmi:id="_armEff">
              <language>JavaScript</language>
              <body>armed = true;</body>
            </effect>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tArmed" source="_uwait" target="_uwork">
            <trigger xmi:type="uml:Trigger" xmi:id="_trCmd" event="_cmdEv"/>
            <guard xmi:type="uml:Constraint" xmi:id="_gArmed">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_gArmedX"><body>armed</body></specification>
            </guard>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tAgain" source="_uwork" target="_uwork">
            <trigger xmi:type="uml:Trigger" xmi:id="_trAgain" event="_cmdEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tPinged" source="_uwork" target="_uwork">
            <trigger xmi:type="uml:Trigger" xmi:id="_trPinged" event="_pingEv"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_console" name="Console">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_out" name="out" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_issue" name="Issue">
        <node xmi:type="uml:InitialNode" xmi:id="_isInit"/>
        <node xmi:type="uml:SendSignalAction" xmi:id="_isSend" name="send cmd" signal="_cmd" onPort="_out"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_isFinal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_isE1" source="_isInit" target="_isSend"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_isE2" source="_isSend" target="_isFinal"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_site" name="Site">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_siteC" name="console" type="_console" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_siteU" name="unit" type="_unit" aggregation="composite"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_siteLink">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_siteLink1" role="_out" partWithPort="_siteC"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_siteLink2" role="_inbox" partWithPort="_siteU"/>
      </ownedConnector>
    </packagedElement>`

const portDeferringApplications = `
  <sysml:Block xmi:id="_pb1" base_Class="_unit"/>
  <sysml:Block xmi:id="_pb2" base_Class="_console"/>
  <sysml:Block xmi:id="_pb3" base_Class="_site"/>`

// A port named only by a deferred trigger still makes the state def's context
// necessary, even when no transition or activity uses it.
func TestStrictDeferredPortRouteDeclaresAndBindsContext(t *testing.T) {
	const machine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_event" name="Event"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_eventEv" signal="_event"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_receiver" name="Receiver" classifierBehavior="_life">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_p" name="p" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_life" name="Life">
        <region xmi:type="uml:Region" xmi:id="_region">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_initial"/>
          <subvertex xmi:type="uml:State" xmi:id="_waiting" name="Waiting">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_deferred" event="_eventEv" port="_p"/>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_initialTransition" source="_initial" target="_waiting"/>
        </region>
      </ownedBehavior>
    </packagedElement>`
	const applications = `<sysml:Block xmi:id="_receiverBlock" base_Class="_receiver"/>`
	r := migrateDocumentOptions(t, machine, applications, migrate.Options{Strict: true})
	for _, line := range []string{
		"in ref context : Receiver[1];",
		"accept 'kept via p' : Event via context.p;",
		"exhibit state life : Life { in ref :>> context = this; }",
	} {
		wantLine(t, r.Notation, line)
	}
	wantClean(t, "deferredPortContext", r)
}

func TestClassifierBehaviorContextFollowsInputParameter(t *testing.T) {
	const machine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_event" name="Event"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_eventEv" signal="_event"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_receiver" name="Receiver" classifierBehavior="_life">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_p" name="p" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_life" name="Life">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_x" name="x" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedParameter>
        <region xmi:type="uml:Region" xmi:id="_region">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_initial"/>
          <subvertex xmi:type="uml:State" xmi:id="_waiting" name="Waiting">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_deferred" event="_eventEv" port="_p"/>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_initialTransition" source="_initial" target="_waiting"/>
        </region>
      </ownedBehavior>
    </packagedElement>`
	const applications = `<sysml:Block xmi:id="_receiverBlock" base_Class="_receiver"/>`
	r := migrateDocumentOptions(t, machine, applications, migrate.Options{Strict: true})
	for _, line := range []string{
		"in x : ScalarValues::Integer[1];",
		"in ref context : Receiver[1];",
		"exhibit state life : Life { in x[1]; in ref :>> context = this; }",
	} {
		wantLine(t, r.Notation, line)
	}
	wantClean(t, "classifierBehaviorParameterContext", r)
}

func TestReceptionPortContextForInterfaceBlock(t *testing.T) {
	const members = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_ping" name="Ping">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_pingValue" name="value">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_contract" name="Contract">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_carriedPing" name="ping" type="_ping"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_interfaceOwner" name="IB">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_interfacePort" name="p" type="_contract" aggregation="composite"/>
      <ownedReception xmi:type="uml:Reception" xmi:id="_interfaceReception" name="R" signal="_ping">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_receptionValue" name="value" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedParameter>
      </ownedReception>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_blockOwner" name="Owner">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_blockPort" name="p" type="_contract" aggregation="composite"/>
      <ownedReception xmi:type="uml:Reception" xmi:id="_blockReception" name="R" signal="_ping"/>
    </packagedElement>`
	const applications = `
    <sysml:InterfaceBlock xmi:id="_contractInterface" base_Class="_contract"/>
    <sysml:FlowProperty xmi:id="_carriedPingFlow" base_Property="_carriedPing" direction="in"/>
    <sysml:InterfaceBlock xmi:id="_interfaceBlock" base_Class="_interfaceOwner"/>
    <sysml:Block xmi:id="_block" base_Class="_blockOwner"/>`
	r := migrateDocumentOptions(t, members, applications, migrate.Options{Strict: true})
	for _, line := range []string{
		"action def R {",
		"in ref context : IB[1];",
		"action 'receive via p' accept 'ping via p' : Ping via context.p;",
		"perform action r : R { in ref :>> context = this; }",
		"perform action r {",
		"action 'receive via p' accept 'ping via p' : Ping via p;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "in value[1];")
	for _, d := range errorsMode(t, "interfaceBlockReception.sysml", r.Notation, diag.ConformanceStrict) {
		t.Errorf("%v", d)
	}
}

func TestStrictAcceptViaContextPortFixture(t *testing.T) {
	r := migrateFixtureFileOptions(t, "accept_via_context_port", migrate.Options{Strict: true})
	for _, line := range []string{
		"in ref context : Receiver[1];",
		"in ref context : ActivityReceiver[1];",
		"action receive accept Ping via context.p;",
		"transition first Waiting accept Go via context.p then Done;",
		"#MigrationMetadata::DeferredKeeper action 'receive via p' accept 'kept via p' : Ping via context.p;",
		"action 'receive via p' accept 'ping via p' : Ping via p;",
		"exhibit state life : Life { in ref :>> context = this; }",
		"perform action await : Await { in ref :>> context = this; }",
	} {
		wantLine(t, r.Notation, line)
	}
	wantClean(t, "acceptViaContextPortFixture", r)
}

// Under -strict, a deferred signal is kept by whichever route it reaches the
// object: from the object itself and via each port a connector delivers it to,
// or via the ports the deferrable trigger names alone. A transition out of the
// state accepting the same signal under a guard does not stop the deferral:
// the signal is kept while the guard is false and taken by the transition once
// it holds. The result runs: a command sent over the connector while unarmed
// is kept at the inbox port, and replayed once Waiting exits it takes the
// transition that its guard now lets fire.
func TestStrictDeferredSignalsAreKeptByEveryRoute(t *testing.T) {
	r := migrateDocumentOptions(t, portDeferringMachine, portDeferringApplications, migrate.Options{Strict: true})
	wantNoLine(t, r.Notation, "defer Cmd;")
	wantNoLine(t, r.Notation, "defer Ping;")
	for _, line := range []string{
		"state Waiting {",
		"item deferredCmd : Cmd[*] ordered;",
		"item deferredPing : Ping[*] ordered;",
		"do action buffer {",
		"first start then split;",
		"fork split;",
		"then receiveCmd;",
		"then 'receiveCmd via inbox';",
		"then 'receivePing via side';",
		"#MigrationMetadata::DeferredKeeper action receiveCmd accept keptCmd : Cmd;",
		"then action keepCmd { assign deferredCmd := SequenceFunctions::including(deferredCmd, receiveCmd.keptCmd); }",
		"then receiveCmd;",
		"#MigrationMetadata::DeferredKeeper action 'receiveCmd via inbox' accept 'keptCmd via inbox' : Cmd via context.inbox;",
		"then action 'keepCmd via inbox' { assign deferredCmd := SequenceFunctions::including(deferredCmd, 'receiveCmd via inbox'.'keptCmd via inbox'); }",
		"then 'receiveCmd via inbox';",
		"#MigrationMetadata::DeferredKeeper action 'receivePing via side' accept 'keptPing via side' : Ping via context.side;",
		"then action 'keepPing via side' { assign deferredPing := SequenceFunctions::including(deferredPing, 'receivePing via side'.'keptPing via side'); }",
		"then 'receivePing via side';",
		"exit action flush {",
		"for keptCmd in deferredCmd { send keptCmd to self; }",
		"then action clearCmd { assign deferredCmd := (); }",
		"then for keptPing in deferredPing { send keptPing to self; }",
		"then action clearPing { assign deferredPing := (); }",
		"transition first Waiting accept Cmd if context.armed then Working;",
		"transition first Waiting accept Cmd via context.inbox if context.armed then Working;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "action receivePing accept keptPing : Ping;")
	wantNote(t, r, "_dCmd", migrate.Approximated, "kept in the item deferredCmd by the accept loop of the do action buffer while the state is active, and sent to self by the exit action flush")
	wantNote(t, r, "_dCmd", migrate.Approximated, "the signal arrives at the port inbox over the document's connectors or declarations")
	wantNote(t, r, "_dCmd", migrate.Approximated, "an occurrence that arrived at a port is sent back to the object itself, which a trigger naming no port accepts")
	wantNote(t, r, "_dCmd", migrate.Approximated, "the transition (_tArmed) out of the state accepts the signal too, which in v1 takes precedence over deferring it only while its guard holds")
	wantNote(t, r, "_dPing", migrate.Approximated, "the trigger accepts via the port side it names")
	wantClean(t, "portDeferring", r)

	s := session(t, r)
	meta(t, s, "%instantiate Site")
	meta(t, s, "%action Console::issue #1.console")
	meta(t, s, "%continue")
	meta(t, s, "%advance 0")
	meta(t, s, "%state Unit::Duty #1.unit")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Waiting") || !strings.Contains(out, "Waiting.deferredCmd = [Instance") {
		t.Fatalf("the Cmd sent over the connector while unarmed was not kept at the inbox:\n%s", out)
	}
	if out := meta(t, s, "%send Arm to #1.unit"); !strings.Contains(out, "transition Waiting -> Waiting fires on it") {
		t.Errorf("%%send Arm while Waiting: %s", out)
	}
	meta(t, s, "%advance 0")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Working") || !strings.Contains(out, "Waiting.deferredCmd = null") {
		t.Errorf("the kept Cmd, replayed once armed, did not take the guarded transition:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : unit.got"); !strings.Contains(out, "= 1") {
		t.Errorf("Working was not entered once by the replayed Cmd: %s", out)
	}
}

// Under -strict, a transition out of the state that accepts the signal via one
// port alone takes the deferral from that route only: the state keeps the
// signal by the routes the transition does not accept by. The result runs: a
// Cmd sent to the unit itself while Waiting is kept, and replayed once the Cmd
// at the inbox has taken the transition, where Working accepts it again.
func TestStrictDeferralKeepsRoutesAPortTransitionSkips(t *testing.T) {
	machine := strings.Replace(portDeferringMachine,
		`<trigger xmi:type="uml:Trigger" xmi:id="_trCmd" event="_cmdEv"/>
            <guard xmi:type="uml:Constraint" xmi:id="_gArmed">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_gArmedX"><body>armed</body></specification>
            </guard>`,
		`<trigger xmi:type="uml:Trigger" xmi:id="_trCmd" event="_cmdEv" port="_inbox"/>`, 1)
	if machine == portDeferringMachine {
		t.Fatal("the fixture's guarded transition was not found")
	}
	r := migrateDocumentOptions(t, machine, portDeferringApplications, migrate.Options{Strict: true})
	wantNoLine(t, r.Notation, "defer Cmd;")
	for _, line := range []string{
		"item deferredCmd : Cmd[*] ordered;",
		"#MigrationMetadata::DeferredKeeper action receiveCmd accept keptCmd : Cmd;",
		"then action keepCmd { assign deferredCmd := SequenceFunctions::including(deferredCmd, receiveCmd.keptCmd); }",
		"for keptCmd in deferredCmd { send keptCmd to self; }",
		"transition first Waiting accept Cmd via context.inbox then Working;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "action 'receiveCmd via inbox' accept 'keptCmd via inbox' : Cmd via inbox;")
	wantNoLine(t, r.Notation, "transition first Waiting accept Cmd then Working;")
	wantNote(t, r, "_dCmd", migrate.Approximated, "kept in the item deferredCmd by the accept loop of the do action buffer while the state is active")
	wantNote(t, r, "_dCmd", migrate.Approximated, "the transition (_tArmed) out of the state accepts the signal via inbox, which in v1 takes precedence over deferring it, so no loop keeps it there")
	wantClean(t, "portDeferringSkipped", r)

	s := session(t, r)
	meta(t, s, "%instantiate Site")
	meta(t, s, "%state Unit::Duty #1.unit")
	meta(t, s, "%send Cmd to #1.unit")
	meta(t, s, "%advance 0")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Waiting") || !strings.Contains(out, "Waiting.deferredCmd = [Instance") {
		t.Fatalf("the Cmd sent to the unit itself was not kept:\n%s", out)
	}
	meta(t, s, "%action Console::issue #1.console")
	meta(t, s, "%continue")
	meta(t, s, "%advance 0")
	meta(t, s, "%state Unit::Duty #1.unit")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Working") || !strings.Contains(out, "Waiting.deferredCmd = null") {
		t.Fatalf("the Cmd at the inbox did not take the transition and flush the kept one:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : unit.got"); !strings.Contains(out, "= 2") {
		t.Errorf("Working was not entered by the Cmd at the inbox and again by the replayed one: %s", out)
	}
}

// Under -strict, a transition accepting the signal out of a substate does not
// stop the composite state deferring it: v1 lets the transition win only while
// that substate is active, so the composite keeps the signal the rest of the
// time. The result runs: a Door sent while the substate with the transition is
// active takes the transition alone, and one sent while the sibling without it
// is active is kept, and replayed once the composite exits.
func TestStrictDeferralSurvivesInactiveSubstateTransition(t *testing.T) {
	const machine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_door" name="Door"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_go" name="Go"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_stop" name="Stop"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_doorEv" signal="_door"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_goEv" signal="_go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_stopEv" signal="_stop"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_oven" name="Oven" classifierBehavior="_sm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Run">
        <region xmi:type="uml:Region" xmi:id="_r0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init"/>
          <subvertex xmi:type="uml:State" xmi:id="_busy" name="Busy">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dDoor" event="_doorEv"/>
            <region xmi:type="uml:Region" xmi:id="_rb">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_binit"/>
              <subvertex xmi:type="uml:State" xmi:id="_heat" name="Heating"/>
              <subvertex xmi:type="uml:State" xmi:id="_cool" name="Cooling"/>
              <subvertex xmi:type="uml:State" xmi:id="_open" name="Opened"/>
              <transition xmi:type="uml:Transition" xmi:id="_tb0" source="_binit" target="_heat"/>
              <transition xmi:type="uml:Transition" xmi:id="_tGo" source="_heat" target="_cool">
                <trigger xmi:type="uml:Trigger" xmi:id="_trGo" event="_goEv"/>
              </transition>
              <transition xmi:type="uml:Transition" xmi:id="_tDoor" source="_heat" target="_open">
                <trigger xmi:type="uml:Trigger" xmi:id="_trDoor" event="_doorEv"/>
              </transition>
            </region>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_ajar" name="Ajar"/>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init" target="_busy"/>
          <transition xmi:type="uml:Transition" xmi:id="_tStop" source="_busy" target="_idle">
            <trigger xmi:type="uml:Trigger" xmi:id="_trStop" event="_stopEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tAjar" source="_idle" target="_ajar">
            <trigger xmi:type="uml:Trigger" xmi:id="_trAjar" event="_doorEv"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`
	r := migrateDocumentOptions(t, machine, `<sysml:Block xmi:id="_b1" base_Class="_oven"/>`, migrate.Options{Strict: true})
	wantNoLine(t, r.Notation, "defer Door;")
	for _, line := range []string{
		"state Busy {",
		"item deferred : Door[*] ordered;",
		"#MigrationMetadata::DeferredKeeper action receive accept kept : Door;",
		"transition first Heating accept Door then Opened;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_dDoor", migrate.Approximated, "kept in the item deferred by the accept loop of the do action buffer while the state is active, and sent to self by the exit action flush")
	wantNote(t, r, "_dDoor", migrate.Approximated, "the transition (_tDoor) out of a substate accepts the signal too, which in v1 takes precedence over deferring it only while that substate is active; the standard leaves open which of the transition and the accept loop takes the signal, which the runtime settles for the transition when it can fire and the loop otherwise")
	wantClean(t, "deferralInactiveSubstate", r)

	s := session(t, r)
	meta(t, s, "%instantiate Oven")
	meta(t, s, "%state Oven::Run")
	meta(t, s, "%send Door")
	for i := 0; i < 4 && !strings.Contains(meta(t, s, "%current"), "Opened"); i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Opened") || strings.Contains(out, "Busy.deferred = [Instance") {
		t.Fatalf("the Door sent while Heating did not take the transition alone:\n%s", out)
	}

	s = session(t, r)
	meta(t, s, "%instantiate Oven")
	meta(t, s, "%state Oven::Run")
	meta(t, s, "%send Go")
	for i := 0; i < 4 && !strings.Contains(meta(t, s, "%current"), "Cooling"); i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Cooling") {
		t.Fatalf("Go did not reach Cooling:\n%s", out)
	}
	meta(t, s, "%send Door")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Cooling") || !strings.Contains(out, "Busy.deferred = [Instance") {
		t.Errorf("the Door sent while Cooling was not kept by Busy:\n%s", out)
	}
	meta(t, s, "%send Stop")
	for i := 0; i < 6 && !strings.Contains(meta(t, s, "%current"), "Current state: Ajar"); i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Ajar") {
		t.Errorf("the kept Door was not replayed once Busy exited:\n%s", out)
	}
}

// Under -strict, a transition the migration refuses to write, its target
// having no v2 form, does not take the signal from the deferral, nor does a
// completion transition it refuses drop it: the state keeps the signal and
// replays it once it exits.
func TestStrictDeferralOutlivesTransitionWithNoForm(t *testing.T) {
	const machine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_door" name="Door"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_stop" name="Stop"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_doorEv" signal="_door"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_stopEv" signal="_stop"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_oven" name="Oven" classifierBehavior="_sm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Run">
        <region xmi:type="uml:Region" xmi:id="_r0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init"/>
          <subvertex xmi:type="uml:State" xmi:id="_off" name="Off">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dDoor" event="_doorEv"/>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_ajar" name="Ajar"/>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init" target="_off"/>
          <transition xmi:type="uml:Transition" xmi:id="_tPick" source="_off" target="_far">
            <trigger xmi:type="uml:Trigger" xmi:id="_trPick" event="_doorEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tDone" source="_off" target="_far"/>
          <transition xmi:type="uml:Transition" xmi:id="_tFar" source="_off" target="_far">
            <trigger xmi:type="uml:Trigger" xmi:id="_trFar" event="_doorEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tStop" source="_off" target="_idle">
            <trigger xmi:type="uml:Trigger" xmi:id="_trStop" event="_stopEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tAjar" source="_idle" target="_ajar">
            <trigger xmi:type="uml:Trigger" xmi:id="_trAjar" event="_doorEv"/>
          </transition>
        </region>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm2" name="Aside">
        <region xmi:type="uml:Region" xmi:id="_r1">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init2"/>
          <subvertex xmi:type="uml:State" xmi:id="_far" name="Far"/>
          <transition xmi:type="uml:Transition" xmi:id="_t2" source="_init2" target="_far"/>
        </region>
      </ownedBehavior>
    </packagedElement>`
	r := migrateDocumentOptions(t, machine, `<sysml:Block xmi:id="_b1" base_Class="_oven"/>`, migrate.Options{Strict: true})
	wantNoLine(t, r.Notation, "defer Door;")
	wantNoLine(t, r.Notation, "accept Door then Far;")
	wantNoLine(t, r.Notation, "/* not migrated: defer Door; — the completion transition (_tDone) leaves the state once its do action ends, which the accept loop that would keep Door never lets it, so the deferral is dropped */")
	for _, line := range []string{
		"state Off {",
		"item deferred : Door[*] ordered;",
		"#MigrationMetadata::DeferredKeeper action receive accept kept : Door;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_dDoor", migrate.Approximated, "kept in the item deferred by the accept loop of the do action buffer while the state is active, and sent to self by the exit action flush")
	wantNote(t, r, "_tPick", migrate.Unmapped, "the target 'Far' is a State outside the machine, or one with no v2 form")
	wantNote(t, r, "_tDone", migrate.Unmapped, "the target 'Far' is a State outside the machine, or one with no v2 form")
	wantNote(t, r, "_tFar", migrate.Unmapped, "")
	wantClean(t, "deferralTargetNoForm", r)

	s := session(t, r)
	meta(t, s, "%instantiate Oven")
	meta(t, s, "%state Oven::Run")
	meta(t, s, "%send Door")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Off") || !strings.Contains(out, "Off.deferred = [Instance") {
		t.Errorf("the Door sent while Off was not kept:\n%s", out)
	}
	meta(t, s, "%send Stop")
	for i := 0; i < 6 && !strings.Contains(meta(t, s, "%current"), "Current state: Ajar"); i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Ajar") {
		t.Errorf("the kept Door was not replayed once Off exited:\n%s", out)
	}
}

// choiceDeferringMachine is an oven whose Off state defers Door while its own
// transition on Door leads into a choice pseudostate; Stop leaves Off too, and
// Idle takes Door.
const choiceDeferringMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_door" name="Door"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_stop" name="Stop"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_doorEv" signal="_door"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_stopEv" signal="_stop"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_oven" name="Oven" classifierBehavior="_sm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Run">
        <region xmi:type="uml:Region" xmi:id="_r0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init"/>
          <subvertex xmi:type="uml:State" xmi:id="_off" name="Off">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dDoor" event="_doorEv"/>
          </subvertex>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_pick" name="pick" kind="choice"/>
          <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_ajar" name="Ajar"/>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init" target="_off"/>
          <transition xmi:type="uml:Transition" xmi:id="_tPick" source="_off" target="_pick">
            <trigger xmi:type="uml:Trigger" xmi:id="_trPick" event="_doorEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tPicked" source="_pick" target="_idle"/>
          <transition xmi:type="uml:Transition" xmi:id="_tStop" source="_off" target="_idle">
            <trigger xmi:type="uml:Trigger" xmi:id="_trStop" event="_stopEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tAjar" source="_idle" target="_ajar">
            <trigger xmi:type="uml:Trigger" xmi:id="_trAjar" event="_doorEv"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`

// A transition into a choice pseudostate takes the deferred signal only where
// the migration writes it. The default migration writes the choice, so the
// transition wins and Off keeps nothing: a Door sent while Off leaves it for
// Idle. The choice is written as metadata under -strict too, so the transition
// into it is written and Off keeps nothing there either.
func TestDeferralYieldsToChoiceTransitionOnlyWhereWritten(t *testing.T) {
	const applications = `<sysml:Block xmi:id="_b1" base_Class="_oven"/>`
	r := migrateDocument(t, choiceDeferringMachine, applications)
	for _, line := range []string{
		"#StateMachines::choice state pick;",
		"private import StateMachines::*;",
		"transition first Off accept Door then pick;",
		"transition first pick then Idle;",
		"@MigrationMetadata::DeferredEvent { ref :>> signal : Door; }",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "defer Door;")
	wantNoLine(t, r.Notation, "item deferred : Door[*] ordered;")
	wantNote(t, r, "_dDoor", migrate.Approximated, "the transition (_tPick) out of the state accepts the signal, which in v1 takes precedence over deferring it, so the state does not keep it; its @MigrationMetadata::DeferredEvent annotation records the deferral")
	wantNote(t, r, "_tPick", migrate.Mapped, "")
	wantClean(t, "choiceDeferralDefault", r)

	s := session(t, r)
	meta(t, s, "%instantiate Oven")
	meta(t, s, "%state Oven::Run")
	if out := meta(t, s, "%send Door"); !strings.Contains(out, "transition Off -> pick fires on it") {
		t.Errorf("%%send Door while Off: %s", out)
	}
	for i := 0; i < 4 && !strings.Contains(meta(t, s, "%current"), "Current state: Idle"); i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Idle") || strings.Contains(out, "Off.deferred") {
		t.Errorf("the Door sent while Off did not take the transition through the choice:\n%s", out)
	}

	strict := migrateDocumentOptions(t, choiceDeferringMachine, applications, migrate.Options{Strict: true})
	wantNoLine(t, strict.Notation, "choice pick;")
	wantNoLine(t, strict.Notation, "item deferred : Door[*] ordered;")
	for _, line := range []string{
		"#StateMachines::choice state pick;",
		"transition first Off accept Door then pick;",
		"transition first pick then Idle;",
		"@MigrationMetadata::DeferredEvent { ref :>> signal : Door; }",
	} {
		wantLine(t, strict.Notation, line)
	}
	wantNote(t, strict, "_dDoor", migrate.Approximated, "the transition (_tPick) out of the state accepts the signal, which in v1 takes precedence over deferring it, so the state does not keep it; its @MigrationMetadata::DeferredEvent annotation records the deferral")
	wantNote(t, strict, "_tPick", migrate.Mapped, "")
	wantClean(t, "choiceDeferralStrict", strict)

	s = session(t, strict)
	meta(t, s, "%instantiate Oven")
	meta(t, s, "%state Oven::Run")
	if out := meta(t, s, "%send Door"); !strings.Contains(out, "transition Off -> pick fires on it") {
		t.Errorf("%%send Door while Off: %s", out)
	}
	for i := 0; i < 4 && !strings.Contains(meta(t, s, "%current"), "Current state: Idle"); i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Idle") || strings.Contains(out, "Off.deferred") {
		t.Errorf("the Door sent while Off did not take the transition through the choice:\n%s", out)
	}
}

// Under -strict, a guarded completion transition out of a deferring state may
// leave it active while the guard is false, so the state keeps the signal: the
// accept loop stays, and the completion transition never fires. The result
// runs: a Door sent while Off is active is kept, Off leaves by the triggered
// transition only, and the Door is replayed then.
func TestStrictDeferralOutlivesGuardedCompletionTransition(t *testing.T) {
	const machine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_door" name="Door"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_stop" name="Stop"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_doorEv" signal="_door"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_stopEv" signal="_stop"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_oven" name="Oven" classifierBehavior="_sm">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ready" name="ready">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Boolean"/>
        <defaultValue xmi:type="uml:LiteralBoolean" xmi:id="_ready0" value="true"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Run">
        <region xmi:type="uml:Region" xmi:id="_r0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init"/>
          <subvertex xmi:type="uml:State" xmi:id="_off" name="Off">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dDoor" event="_doorEv"/>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_done" name="Done"/>
          <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_ajar" name="Ajar"/>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init" target="_off"/>
          <transition xmi:type="uml:Transition" xmi:id="_tDone" source="_off" target="_done">
            <guard xmi:type="uml:Constraint" xmi:id="_gReady">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_gReadyX"><body>ready</body></specification>
            </guard>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tStop" source="_off" target="_idle">
            <trigger xmi:type="uml:Trigger" xmi:id="_trStop" event="_stopEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tAjar" source="_idle" target="_ajar">
            <trigger xmi:type="uml:Trigger" xmi:id="_trAjar" event="_doorEv"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`
	r := migrateDocumentOptions(t, machine, `<sysml:Block xmi:id="_b1" base_Class="_oven"/>`, migrate.Options{Strict: true})
	wantNoLine(t, r.Notation, "defer Door;")
	for _, line := range []string{
		"state Off {",
		"item deferred : Door[*] ordered;",
		"#MigrationMetadata::DeferredKeeper action receive accept kept : Door;",
		"transition first Off if context.ready then Done;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_dDoor", migrate.Approximated, "kept in the item deferred by the accept loop of the do action buffer while the state is active, and sent to self by the exit action flush")
	wantNote(t, r, "_dDoor", migrate.Approximated, "the guarded completion transition (_tDone) leaves the state once its do action ends, which the accept loop never lets it: the state keeps the signal, and leaves only by a transition a trigger fires")
	wantClean(t, "deferralGuardedCompletion", r)

	s := session(t, r)
	meta(t, s, "%instantiate Oven")
	meta(t, s, "%state Oven::Run")
	for i := 0; i < 4; i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Off") {
		t.Fatalf("Off did not stay active with the accept loop running, its guarded completion transition notwithstanding:\n%s", out)
	}
	meta(t, s, "%send Door")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Off") || !strings.Contains(out, "Off.deferred = [Instance") {
		t.Errorf("the Door sent while Off was not kept:\n%s", out)
	}
	meta(t, s, "%send Stop")
	for i := 0; i < 6 && !strings.Contains(meta(t, s, "%current"), "Current state: Ajar"); i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Ajar") {
		t.Errorf("the kept Door was not replayed once Off exited:\n%s", out)
	}
}

// Under -strict, an internal transition of a deferring state, written as a
// self transition, exits and re-enters the state: the exit action sends the
// kept occurrences to self, and the restarted accept loop keeps them again, so
// they are replayed once the state truly exits.
func TestStrictDeferralSurvivesInternalTransition(t *testing.T) {
	const machine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_door" name="Door"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_tick" name="Tick"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_stop" name="Stop"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_doorEv" signal="_door"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_tickEv" signal="_tick"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_stopEv" signal="_stop"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_oven" name="Oven" classifierBehavior="_sm">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ticks" name="ticks">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_ticks0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Run">
        <region xmi:type="uml:Region" xmi:id="_r0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init"/>
          <subvertex xmi:type="uml:State" xmi:id="_off" name="Off">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dDoor" event="_doorEv"/>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_ajar" name="Ajar"/>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init" target="_off"/>
          <transition xmi:type="uml:Transition" xmi:id="_tTick" kind="internal" source="_off" target="_off">
            <trigger xmi:type="uml:Trigger" xmi:id="_trTick" event="_tickEv"/>
            <effect xmi:type="uml:OpaqueBehavior" xmi:id="_tickFx">
              <language>JavaScript</language>
              <body>ticks = ticks + 1;</body>
            </effect>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tStop" source="_off" target="_idle">
            <trigger xmi:type="uml:Trigger" xmi:id="_trStop" event="_stopEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tAjar" source="_idle" target="_ajar">
            <trigger xmi:type="uml:Trigger" xmi:id="_trAjar" event="_doorEv"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`
	r := migrateDocumentOptions(t, machine, `<sysml:Block xmi:id="_b1" base_Class="_oven"/>`, migrate.Options{Strict: true})
	wantNoLine(t, r.Notation, "defer Door;")
	for _, line := range []string{
		"item deferred : Door[*] ordered;",
		"#MigrationMetadata::DeferredKeeper action receive accept kept : Door;",
		"transition first Off accept Tick",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_dDoor", migrate.Approximated, "the internal transition (_tTick) is written as a self transition, which exits and re-enters the state where v1 stayed in it: the exit action sends the kept occurrences to self, and the accept loop, started again, keeps them again unless a transition then accepts them")
	wantNote(t, r, "_tTick", migrate.Approximated, "an internal transition is written as a self transition, which exits and re-enters Off")
	wantClean(t, "deferralInternalTransition", r)
	s := session(t, r)
	meta(t, s, "%instantiate Oven")
	meta(t, s, "%state Oven::Run")
	for i := 0; i < 4; i++ {
		meta(t, s, "%step")
	}
	meta(t, s, "%send Door")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Off.deferred = [Instance") {
		t.Fatalf("the Door sent while Off was not kept:\n%s", out)
	}
	meta(t, s, "%send Tick")
	for i := 0; i < 8; i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%eval in #1 : ticks"); !strings.Contains(out, "= 1") {
		t.Fatalf("the internal transition did not run its effect:\n%s", out)
	}
	out := meta(t, s, "%current")
	if !strings.Contains(out, "Current state: Off") {
		t.Fatalf("the internal transition did not stay in Off:\n%s", out)
	}
	if !strings.Contains(out, "Off.deferred = [Instance") {
		t.Errorf("the Door kept before the internal transition was not kept again after it:\n%s", out)
	}
	meta(t, s, "%send Stop")
	for i := 0; i < 6 && !strings.Contains(meta(t, s, "%current"), "Current state: Ajar"); i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Ajar") {
		t.Errorf("the kept Door was not replayed once Off exited:\n%s", out)
	}
}

// A transition out of the state on a general of the deferred signal takes its
// occurrences as a v2 accept typed by the general does, so under -strict the
// state does not keep the signal; a transition on a specialization takes only
// the occurrences of that specialization, so the state keeps the general.
func TestStrictDeferralYieldsToTransitionOnGeneralSignal(t *testing.T) {
	const signals = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_notif" name="Notification"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_alarm" name="Alarm">
      <generalization xmi:type="uml:Generalization" xmi:id="_gAlarm" general="_notif"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Signal" xmi:id="_stop" name="Stop"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_notifEv" signal="_notif"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_alarmEv" signal="_alarm"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_stopEv" signal="_stop"/>`
	const machine = `
    <packagedElement xmi:type="uml:Class" xmi:id="_oven" name="Oven" classifierBehavior="_sm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Run">
        <region xmi:type="uml:Region" xmi:id="_r0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init"/>
          <subvertex xmi:type="uml:State" xmi:id="_busy" name="Busy">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dDeferred" event="_DEFERRED"/>
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dStop" event="_stopEv"/>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle"/>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init" target="_busy"/>
          <transition xmi:type="uml:Transition" xmi:id="_tOut" source="_busy" target="_idle">
            <trigger xmi:type="uml:Trigger" xmi:id="_trOut" event="_ACCEPTED"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`
	const block = `<sysml:Block xmi:id="_b1" base_Class="_oven"/>`

	general := strings.NewReplacer("_DEFERRED", "_alarmEv", "_ACCEPTED", "_notifEv").Replace(machine)
	r := migrateDocumentOptions(t, signals+general, block, migrate.Options{Strict: true})
	for _, line := range []string{
		"@MigrationMetadata::DeferredEvent { ref :>> signal : Alarm; }",
		"item deferred : Stop[*] ordered;",
		"#MigrationMetadata::DeferredKeeper action receive accept kept : Stop;",
		"transition first Busy accept Notification then Idle;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "item deferredAlarm : Alarm[*] ordered;")
	wantNote(t, r, "_dDeferred", migrate.Approximated, "the transition (_tOut) out of the state accepts the signal, which in v1 takes precedence over deferring it, so the state does not keep it")
	wantClean(t, "deferralGeneralTaken", r)

	s := session(t, r)
	meta(t, s, "%instantiate Oven")
	meta(t, s, "%state Oven::Run")
	meta(t, s, "%send Alarm")
	for i := 0; i < 4 && !strings.Contains(meta(t, s, "%current"), "Current state: Idle"); i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Idle") {
		t.Errorf("the Alarm did not take the transition on Notification:\n%s", out)
	}

	special := strings.NewReplacer("_DEFERRED", "_notifEv", "_ACCEPTED", "_alarmEv").Replace(machine)
	r = migrateDocumentOptions(t, signals+special, block, migrate.Options{Strict: true})
	for _, line := range []string{
		"item deferredNotification : Notification[*] ordered;",
		"#MigrationMetadata::DeferredKeeper action receiveNotification accept keptNotification : Notification;",
		"transition first Busy accept Alarm then Idle;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_dDeferred", migrate.Approximated, "the transition (_tOut) out of the state accepts a specialization of the signal, which in v1 takes precedence over deferring those occurrences")
	wantClean(t, "deferralSpecialContested", r)
}

// A state deferring a signal and a general of it, or the same signal twice,
// keeps each occurrence once: the general's accept loop takes every occurrence
// of the specialization, so the specialization gets no loop of its own, and a
// second trigger on one signal folds into the first. Through a chain of
// generals the note names the deferral whose loop is written.
func TestStrictOverlappingDeferralsKeepEachOccurrenceOnce(t *testing.T) {
	const machine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_event" name="Event"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_notif" name="Notification">
      <generalization xmi:type="uml:Generalization" xmi:id="_gNotif" general="_event"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Signal" xmi:id="_alarm" name="Alarm">
      <generalization xmi:type="uml:Generalization" xmi:id="_gAlarm" general="_notif"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Signal" xmi:id="_stop" name="Stop"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_eventEv" signal="_event"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_notifEv" signal="_notif"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_notifEv2" signal="_notif"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_alarmEv" signal="_alarm"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_stopEv" signal="_stop"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_oven" name="Oven" classifierBehavior="_sm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Run">
        <region xmi:type="uml:Region" xmi:id="_r0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init"/>
          <subvertex xmi:type="uml:State" xmi:id="_busy" name="Busy">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dAlarm" event="_alarmEv"/>
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dNotif" event="_notifEv"/>
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dNotif2" event="_notifEv2"/>
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dEvent" event="_eventEv"/>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_once" name="Once"/>
          <subvertex xmi:type="uml:State" xmi:id="_twice" name="Twice"/>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init" target="_busy"/>
          <transition xmi:type="uml:Transition" xmi:id="_tStop" source="_busy" target="_idle">
            <trigger xmi:type="uml:Trigger" xmi:id="_trStop" event="_stopEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tOnce" source="_idle" target="_once">
            <trigger xmi:type="uml:Trigger" xmi:id="_trOnce" event="_notifEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tTwice" source="_once" target="_twice">
            <trigger xmi:type="uml:Trigger" xmi:id="_trTwice" event="_notifEv"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`
	r := migrateDocumentOptions(t, machine, `<sysml:Block xmi:id="_b1" base_Class="_oven"/>`, migrate.Options{Strict: true})
	for _, line := range []string{
		"@MigrationMetadata::DeferredEvent { ref :>> signal : Alarm; }",
		"@MigrationMetadata::DeferredEvent { ref :>> signal : Notification; }",
		"item deferred : Event[*] ordered;",
		"#MigrationMetadata::DeferredKeeper action receive accept kept : Event;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "item deferredAlarm : Alarm[*] ordered;")
	wantNoLine(t, r.Notation, "item deferredNotification : Notification[*] ordered;")
	wantNoLine(t, r.Notation, "item deferredEvent : Event[*] ordered;")
	wantNote(t, r, "_dEvent", migrate.Approximated, "kept in the item deferred by the accept loop")
	wantNote(t, r, "_dAlarm", migrate.Approximated, "kept by the accept loop of the deferral of Event in the same state, which accepts every occurrence of the signal too; a loop of its own would keep each occurrence twice")
	wantNote(t, r, "_dNotif", migrate.Approximated, "kept by the accept loop of the deferral of Event in the same state")
	wantNote(t, r, "_dNotif2", migrate.Approximated, "kept by the accept loop of the deferral of Event in the same state")
	wantNote(t, r, "_alarmEv", migrate.Approximated, "deferred by 'Busy' through the deferral of Event")
	wantClean(t, "deferralOverlap", r)

	s := session(t, r)
	meta(t, s, "%instantiate Oven")
	meta(t, s, "%state Oven::Run")
	for i := 0; i < 4; i++ {
		meta(t, s, "%step")
	}
	meta(t, s, "%send Alarm")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Busy.deferred = [Instance") {
		t.Fatalf("the Alarm sent while Busy was not kept:\n%s", out)
	}
	meta(t, s, "%send Stop")
	for i := 0; i < 8; i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Once") {
		t.Errorf("the kept Alarm was not replayed exactly once after Busy exited:\n%s", out)
	}
}

// The members the strict encoding adds must not hide what it refers to from
// where it is written: a deferred signal named like one of them, or the
// library package a member of an enclosing scope is named after.
func TestStrictDeferralNamesShadowNothingItRefersTo(t *testing.T) {
	const machine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_recv" name="receive"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_stop" name="Stop"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_recvEv" signal="_recv"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_stopEv" signal="_stop"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_oven" name="Oven" classifierBehavior="_sm">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_sfAttr" name="SequenceFunctions" type="_stop"/>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Run">
        <region xmi:type="uml:Region" xmi:id="_r0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init"/>
          <subvertex xmi:type="uml:State" xmi:id="_busy" name="Busy">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dRecv" event="_recvEv"/>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle"/>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init" target="_busy"/>
          <transition xmi:type="uml:Transition" xmi:id="_tStop" source="_busy" target="_idle">
            <trigger xmi:type="uml:Trigger" xmi:id="_trStop" event="_stopEv"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`
	r := migrateDocumentOptions(t, machine, `<sysml:Block xmi:id="_b1" base_Class="_oven"/>`, migrate.Options{Strict: true})
	for _, line := range []string{
		"ref item SequenceFunctions : Stop;",
		"item deferred : receive[*] ordered;",
		"#MigrationMetadata::DeferredKeeper action receive2 accept kept : receive;",
		"then action keep { assign deferred := $::SequenceFunctions::including(deferred, receive2.kept); }",
		"then receive2;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantClean(t, "deferralShadowing", r)

	s := session(t, r)
	meta(t, s, "%instantiate Oven")
	meta(t, s, "%state Oven::Run")
	meta(t, s, "%send receive")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Busy.deferred = [Instance") {
		t.Errorf("the receive signal was not kept:\n%s", out)
	}
}

// Under -strict, a deferring state whose do behavior cannot run as an action
// (here a StateMachine) gets the accept loop alone: the generated do action
// forks into the behavior only when the behavior is written.
func TestStrictDeferralWithoutRunnableDoBehavior(t *testing.T) {
	const machine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_door" name="Door"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_go" name="Go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_doorEv" signal="_door"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_goEv" signal="_go"/>
    <packagedElement xmi:type="uml:StateMachine" xmi:id="_aux" name="Aux">
      <region xmi:type="uml:Region" xmi:id="_ar0">
        <subvertex xmi:type="uml:State" xmi:id="_aidle" name="AuxIdle"/>
      </region>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_oven" name="Oven" classifierBehavior="_sm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Run">
        <region xmi:type="uml:Region" xmi:id="_r0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init"/>
          <subvertex xmi:type="uml:State" xmi:id="_off" name="Off" doActivity="_aux">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dDoor" event="_doorEv"/>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_on" name="On"/>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init" target="_off"/>
          <transition xmi:type="uml:Transition" xmi:id="_t1" source="_off" target="_on">
            <trigger xmi:type="uml:Trigger" xmi:id="_tr1" event="_goEv"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`
	r := migrateDocumentOptions(t, machine, `<sysml:Block xmi:id="_b1" base_Class="_oven"/>`, migrate.Options{Strict: true})
	wantNoLine(t, r.Notation, "defer Door;")
	wantNoLine(t, r.Notation, "fork split;")
	wantNoLine(t, r.Notation, "then run;")
	for _, line := range []string{
		"state Off {",
		"do action buffer {",
		"first start then receive;",
		"/* do action Aux is written as a state def, which no state runs */",
		"#MigrationMetadata::DeferredKeeper action receive accept kept : Door;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantClean(t, "deferralWithoutDo", r)
}

// pipelineActivity is a package-owned activity taking a parameter, which it
// hands to a called activity's parameter through object flows; the called
// activity doubles it through a function behavior, returns it, and the
// caller writes the result to its out parameter. A partition groups the call.
const pipelineActivity = `
    <packagedElement xmi:type="uml:Activity" xmi:id="_double" name="Double">
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_dIn" name="x" direction="in">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedParameter>
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_dOut" name="y" direction="out">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedParameter>
      <node xmi:type="uml:ActivityParameterNode" xmi:id="_dInN" name="x" parameter="_dIn"/>
      <node xmi:type="uml:ActivityParameterNode" xmi:id="_dOutN" name="y" parameter="_dOut"/>
      <node xmi:type="uml:CallBehaviorAction" xmi:id="_callTwice" name="twice" behavior="_twice">
        <argument xmi:type="uml:InputPin" xmi:id="_twiceIn" name="v"/>
        <result xmi:type="uml:OutputPin" xmi:id="_twiceOut" name="result"/>
      </node>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_dOf1" source="_dInN" target="_twiceIn"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_dOf2" source="_twiceOut" target="_dOutN"/>
    </packagedElement>
    <packagedElement xmi:type="uml:FunctionBehavior" xmi:id="_twice" name="Twice">
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_tv" name="v" direction="in">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedParameter>
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_tr" name="result" direction="return">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedParameter>
      <body>v * 2.0</body>
    </packagedElement>
    <packagedElement xmi:type="uml:Activity" xmi:id="_pipe" name="Pipeline">
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_pIn" name="seed" direction="in">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedParameter>
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_pOut" name="total" direction="out">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedParameter>
      <group xmi:type="uml:ActivityPartition" xmi:id="_lane" name="Compute" node="_callD"/>
      <node xmi:type="uml:ActivityParameterNode" xmi:id="_pInN" name="seed" parameter="_pIn"/>
      <node xmi:type="uml:ActivityParameterNode" xmi:id="_pOutN" name="total" parameter="_pOut"/>
      <node xmi:type="uml:CallBehaviorAction" xmi:id="_callD" name="double" behavior="_double" inPartition="_lane">
        <argument xmi:type="uml:InputPin" xmi:id="_callDIn" name="x"/>
        <result xmi:type="uml:OutputPin" xmi:id="_callDOut" name="y"/>
      </node>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_pOf1" source="_pInN" target="_callDIn"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_pOf2" source="_callDOut" target="_pOutN"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Activity" xmi:id="_run" name="Run">
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_rOut" name="answer" direction="out">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedParameter>
      <node xmi:type="uml:ActivityParameterNode" xmi:id="_rOutN" name="answer" parameter="_rOut"/>
      <node xmi:type="uml:ValueSpecificationAction" xmi:id="_seedV" name="seed">
        <value xmi:type="uml:LiteralReal" xmi:id="_seedL" value="21.0"/>
        <result xmi:type="uml:OutputPin" xmi:id="_seedOut" name="result"/>
      </node>
      <node xmi:type="uml:CallBehaviorAction" xmi:id="_callP" name="pipeline" behavior="_pipe">
        <argument xmi:type="uml:InputPin" xmi:id="_callPIn" name="seed"/>
        <result xmi:type="uml:OutputPin" xmi:id="_callPOut" name="total"/>
      </node>
      <node xmi:type="uml:CallBehaviorAction" xmi:id="_stray" name="stray" behavior="_double">
        <argument xmi:type="uml:InputPin" xmi:id="_strayIn" name="x"/>
        <result xmi:type="uml:OutputPin" xmi:id="_strayOut" name="y"/>
      </node>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_rOf1" source="_seedOut" target="_callPIn"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_rOf2" source="_callPOut" target="_rOutN"/>
    </packagedElement>`

// An activity's parameters become the action def's parameters, a flow from a
// parameter node into a call's pin the binding of the call's input, a flow
// from the call's result to a parameter node the binding of the out
// parameter, a function behavior a calc def the call evaluates, and a
// partition a comment naming its nodes. A driver feeding the pipeline a
// literal through a value specification action runs and yields the double; a
// call nothing leads to whose input pin nothing feeds never fires, so no
// succession starts it.
func TestActivityParametersFlowThroughNestedCalls(t *testing.T) {
	r := migrateDocument(t, pipelineActivity, "")
	for _, line := range []string{
		"action def Double {",
		"in x : ScalarValues::Real[1];",
		"out y : ScalarValues::Real[1];",
		"calc def Twice {",
		"in v : ScalarValues::Real[1];",
		"out result : ScalarValues::Real[1];",
		"v * 2.0",
		"action def Pipeline {",
		"in seed : ScalarValues::Real[1];",
		"out total : ScalarValues::Real[1];",
		"action double : Double;",
		"bind double.x = seed;",
		"bind total = double.y;",
		"/* partition 'Compute': double */",
		"action def Run {",
		"out answer : ScalarValues::Real[1];",
		"action pipeline : Pipeline;",
		"out result[1] = 21.0;",
		"flow seed.result to pipeline.seed;",
		"bind answer = pipeline.total;",
		"action stray : Double;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "then stray;") {
		t.Errorf("a succession starts the call whose input nothing feeds:\n%s", r.Notation)
	}
	wantNote(t, r, "_lane", migrate.Approximated, "")
	wantNote(t, r, "_pInN", migrate.Mapped, "")
	wantNote(t, r, "_pOf1", migrate.Mapped, "")
	wantNote(t, r, "_twice", migrate.Mapped, "")
	wantNote(t, r, "_seedV", migrate.Mapped, "no edge leads to the node, so it starts with the activity")
	wantNote(t, r, "_stray", migrate.Approximated, "the action never fires: its input pin 'x' must hold a value, but no object flow feeds it")
	wantNote(t, r, "_rOf1", migrate.Mapped, "")

	s := session(t, r)
	v := s.RunAction("Run")
	wantVerdict(t, v)
	if out := strings.Join(v.Lines, "\n"); !strings.Contains(out, "answer = 42.0") {
		t.Errorf("the pipeline did not double its seed:\n%s", out)
	}
}

// handshakeInteraction is a block with two parts whose interaction sends two
// signals between lifelines standing for the parts, listed out of occurrence
// order, with a duration constraint; a second interaction carries a call.
const handshakeInteraction = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_req" name="Request">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_reqN" name="n">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Signal" xmi:id="_rep" name="Reply"/>
    <packagedElement xmi:type="uml:Duration" xmi:id="_hsMin">
      <expr xmi:type="uml:LiteralString" xmi:id="_hsMinV" value="2s"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Duration" xmi:id="_hsMax">
      <expr xmi:type="uml:LiteralString" xmi:id="_hsMaxV" value="4s"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_node" name="Node"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_net" name="Net">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_a" name="a" type="_node" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_b" name="b" type="_node" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_hs" name="Handshake">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_la" name="a" represents="_a" coveredBy="_sReq _rRep"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_lb" name="b" represents="_b" coveredBy="_rReq _sRep"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sReq" covered="_la" message="_mReq"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rReq" covered="_lb" message="_mReq"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sRep" covered="_lb" message="_mRep"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rRep" covered="_la" message="_mRep"/>
        <message xmi:type="uml:Message" xmi:id="_mRep" name="reply" messageSort="asynchSignal" signature="_rep" sendEvent="_sRep" receiveEvent="_rRep"/>
        <message xmi:type="uml:Message" xmi:id="_mReq" name="request" messageSort="asynchSignal" signature="_req" sendEvent="_sReq" receiveEvent="_rReq">
          <argument xmi:type="uml:LiteralInteger" xmi:id="_mReqArg" value="7"/>
        </message>
        <ownedRule xmi:type="uml:DurationConstraint" xmi:id="_hsDur">
          <constrainedElement xmi:idref="_sReq"/>
          <constrainedElement xmi:idref="_rRep"/>
          <specification xmi:type="uml:DurationInterval" xmi:id="_hsDI" min="_hsMin" max="_hsMax"/>
        </ownedRule>
        <ownedRule xmi:type="uml:DurationConstraint" xmi:id="_hsDur2">
          <constrainedElement xmi:idref="_mReq"/>
        </ownedRule>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_rpc" name="RemoteCall">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_la2" name="a" represents="_a" coveredBy="_sCall"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_lb2" name="b" represents="_b" coveredBy="_rCall"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sCall" covered="_la2" message="_mCall"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rCall" covered="_lb2" message="_mCall"/>
        <message xmi:type="uml:Message" xmi:id="_mCall" name="call" messageSort="synchCall" sendEvent="_sCall" receiveEvent="_rCall"/>
      </ownedBehavior>
    </packagedElement>`

const handshakeApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_node"/>
  <sysml:Block xmi:id="_s2" base_Class="_net"/>`

// A signal-only interaction becomes a scenario action def of sends in occurrence order with
// duration waits; a malformed duration or a call of no operation is reported. The scenario runs.
func TestInteractionMigratesToAScenarioOfSends(t *testing.T) {
	r := migrateDocument(t, handshakeInteraction, handshakeApplications)
	for _, line := range []string{
		"action handshake {",
		"/* duration constraint on request not migrated — the duration constraint has no interval */",
		"action request send new Request(n = 7) to b;",
		"first start then request;",
		"action wait accept after RandomFunctions::uniform(2.0, 4.0) [SI::s];",
		"first request then wait;",
		"action reply send new Reply() to a;",
		"first wait then reply;",
		"first reply then done;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "action def RemoteCall {") {
		t.Errorf("an interaction carrying a call was written as a scenario:\n%s", r.Notation)
	}
	wantNote(t, r, "_hs", migrate.Approximated, "written as a scenario of 2 steps, one per message in occurrence order")
	wantNote(t, r, "_mReq", migrate.Mapped, "written as a send to b")
	wantNote(t, r, "_la", migrate.Mapped, "the lifeline stands for a, which the steps address")
	wantNote(t, r, "_hsDur", migrate.Approximated, "the time from request, written as the wait wait before reply")
	wantNote(t, r, "_hsDur2", migrate.Unmapped, "the duration constraint has no interval")
	wantNote(t, r, "_rpc", migrate.Unmapped, "the message 'call' names no operation")

	s := session(t, r)
	meta(t, s, "%instantiate Net")
	meta(t, s, "%seed 1")
	meta(t, s, "%action Net::handshake #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "Completed") {
		t.Errorf("the scenario did not run to completion:\n%s", out)
	}
}

// timedInteraction sends a request, a probe that takes 4 s, and a reply that must come 10 s
// after the request; a second interaction spans the same constraint into an opt fragment.
const timedInteraction = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_treq" name="Request"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_tprb" name="Probe"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_trep" name="Reply"/>
    <packagedElement xmi:type="uml:Duration" xmi:id="_tTen">
      <expr xmi:type="uml:LiteralString" xmi:id="_tTenV" value="10s"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Duration" xmi:id="_tFour">
      <expr xmi:type="uml:LiteralString" xmi:id="_tFourV" value="4s"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_tnode" name="Node"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_tnet" name="Net">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ta" name="a" type="_tnode" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_tb" name="b" type="_tnode" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_timed" name="Timed">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_tla" name="a" represents="_ta" coveredBy="_tsReq _tsPrb _trRep"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_tlb" name="b" represents="_tb" coveredBy="_trReq _trPrb _tsRep"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_tsReq" covered="_tla" message="_tmReq"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_trReq" covered="_tlb" message="_tmReq"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_tsPrb" covered="_tla" message="_tmPrb"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_trPrb" covered="_tlb" message="_tmPrb"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_tsRep" covered="_tlb" message="_tmRep"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_trRep" covered="_tla" message="_tmRep"/>
        <message xmi:type="uml:Message" xmi:id="_tmReq" name="request" messageSort="asynchSignal" signature="_treq" sendEvent="_tsReq" receiveEvent="_trReq"/>
        <message xmi:type="uml:Message" xmi:id="_tmPrb" name="probe" messageSort="asynchSignal" signature="_tprb" sendEvent="_tsPrb" receiveEvent="_trPrb"/>
        <message xmi:type="uml:Message" xmi:id="_tmRep" name="reply" messageSort="asynchSignal" signature="_trep" sendEvent="_tsRep" receiveEvent="_trRep"/>
        <ownedRule xmi:type="uml:DurationConstraint" xmi:id="_tSpan">
          <constrainedElement xmi:idref="_tsReq"/>
          <constrainedElement xmi:idref="_trRep"/>
          <specification xmi:type="uml:DurationInterval" xmi:id="_tSpanI" min="_tTen" max="_tTen"/>
        </ownedRule>
        <ownedRule xmi:type="uml:DurationConstraint" xmi:id="_tOwn">
          <constrainedElement xmi:idref="_tmPrb"/>
          <specification xmi:type="uml:DurationInterval" xmi:id="_tOwnI" min="_tFour" max="_tFour"/>
        </ownedRule>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_split" name="Split">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_sla" name="a" represents="_ta" coveredBy="_ssReq _ssPrb _srRep"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_slb" name="b" represents="_tb" coveredBy="_srReq _srPrb _ssRep"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_ssReq" covered="_sla" message="_smReq"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_srReq" covered="_slb" message="_smReq"/>
        <fragment xmi:type="uml:CombinedFragment" xmi:id="_sOpt" interactionOperator="opt">
          <operand xmi:type="uml:InteractionOperand" xmi:id="_sOptOp">
            <guard xmi:type="uml:InteractionConstraint" xmi:id="_sOptG">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_sOptS"><body>1 &lt; 2</body></specification>
            </guard>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_ssPrb" covered="_sla" message="_smPrb"/>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_srPrb" covered="_slb" message="_smPrb"/>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_ssRep" covered="_slb" message="_smRep"/>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_srRep" covered="_sla" message="_smRep"/>
          </operand>
        </fragment>
        <message xmi:type="uml:Message" xmi:id="_smReq" name="request" messageSort="asynchSignal" signature="_treq" sendEvent="_ssReq" receiveEvent="_srReq"/>
        <message xmi:type="uml:Message" xmi:id="_smPrb" name="probe" messageSort="asynchSignal" signature="_tprb" sendEvent="_ssPrb" receiveEvent="_srPrb"/>
        <message xmi:type="uml:Message" xmi:id="_smRep" name="reply" messageSort="asynchSignal" signature="_trep" sendEvent="_ssRep" receiveEvent="_srRep"/>
        <ownedRule xmi:type="uml:DurationConstraint" xmi:id="_sSpan">
          <constrainedElement xmi:idref="_ssReq"/>
          <constrainedElement xmi:idref="_srRep"/>
          <specification xmi:type="uml:DurationInterval" xmi:id="_sSpanI" min="_tTen" max="_tTen"/>
        </ownedRule>
      </ownedBehavior>
    </packagedElement>`

const timedApplications = `
  <sysml:Block xmi:id="_ts1" base_Class="_tnode"/>
  <sysml:Block xmi:id="_ts2" base_Class="_tnet"/>`

// A duration constraint spanning two messages with steps between them is a wait forked after the
// first and joined before the second, so the steps between count toward it; one that spans into
// another fragment is reported. The scenario completes at the constraint's bound, not after it.
func TestSpanningDurationConstraintCountsTheStepsBetween(t *testing.T) {
	r := migrateDocument(t, timedInteraction, timedApplications)
	for _, line := range []string{
		"action request send new Request() to b;",
		"first start then request;",
		"fork timing;",
		"first request then timing;",
		"action wait accept after 10.0 [SI::s];",
		"first timing then wait;",
		"action wait2 accept after 4.0 [SI::s];",
		"first timing then wait2;",
		"action probe send new Probe() to b;",
		"first wait2 then probe;",
		"join waitEnd;",
		"first probe then waitEnd;",
		"first wait then waitEnd;",
		"action reply send new Reply() to a;",
		"first waitEnd then reply;",
		"first reply then done;",
		"/* duration constraint on reply not migrated — the time it measures from request to reply is not written: steps of other fragments lie between them, so no wait forked after the one can be joined before the other */",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_tSpan", migrate.Approximated, "the time from request, written as the wait wait forked after it and joined before reply; a fixed wait of 10.0 s")
	wantNote(t, r, "_tOwn", migrate.Approximated, "the message's duration; a v2 send arrives at once, so the step waits for it first, written as the wait wait2 before probe")
	wantNote(t, r, "_sSpan", migrate.Unmapped, "the time it measures from request to reply is not written: steps of other fragments lie between them")

	s := session(t, r)
	meta(t, s, "%instantiate Net")
	meta(t, s, "%action Net::timed #1")
	if out := meta(t, s, "%advance 9.9"); strings.Contains(out, "completed") {
		t.Errorf("the scenario completed before the 10 s the reply must come after the request:\n%s", out)
	}
	if out := meta(t, s, "%advance 0.1"); !strings.Contains(out, "Advanced to 10.0") || !strings.Contains(out, "completed") {
		t.Errorf("the scenario did not complete at 10 s, the constraint's bound:\n%s", out)
	}
}

// nestedCalls is an interaction that calls Spin twice on the motor before either reply comes
// back: the first reply answers the second call and the second reply the first.
const nestedCalls = `
    <packagedElement xmi:type="uml:Class" xmi:id="_nctl" name="Controller">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_nGot" name="got">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_nGot0" value="0.0"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_nFirst" name="first">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_nFirst0" value="0.0"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_nmotor" name="Motor">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_nSpeed" name="speed">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_nSpeed0" value="0.0"/>
      </ownedAttribute>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_nspin" name="Spin" method="_nspinning">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_nspRpm" name="rpm" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_nspRes" name="result" direction="return">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
      </ownedOperation>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_nspinning" name="Spinning" specification="_nspin">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_nspRpm2" name="rpm" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_nspRes2" name="result" direction="return">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_napnRpm" name="rpm" parameter="_nspRpm2"/>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_napnRes" name="result" parameter="_nspRes2"/>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_nset" name="set speed" structuralFeature="_nSpeed" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_nsetVal" name="value"/>
        </node>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_nread" name="read speed" structuralFeature="_nSpeed">
          <result xmi:type="uml:OutputPin" xmi:id="_nreadOut" name="result"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_nofRpm" source="_napnRpm" target="_nsetVal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ncfSet" source="_nset" target="_nread"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_nofRes" source="_nreadOut" target="_napnRes"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_nrig" name="Rig">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_nCtrl" name="ctrl" type="_nctl" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_nMotor" name="motor" type="_nmotor" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_nMode" name="mode">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_nMode0" value="2"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_nested" name="Nested">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_nlc" name="c" represents="_nCtrl" coveredBy="_nsA _nsB _nrRb _nrRa"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_nlm" name="m" represents="_nMotor" coveredBy="_nrA _nrB _nsRb _nsRa"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_nsA" covered="_nlc" message="_nmA"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_nrA" covered="_nlm" message="_nmA"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_nsB" covered="_nlc" message="_nmB"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_nrB" covered="_nlm" message="_nmB"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_nsRb" covered="_nlm" message="_nmRb"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_nrRb" covered="_nlc" message="_nmRb"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_nsRa" covered="_nlm" message="_nmRa"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_nrRa" covered="_nlc" message="_nmRa"/>
        <message xmi:type="uml:Message" xmi:id="_nmA" name="outer" messageSort="synchCall" signature="_nspin" sendEvent="_nsA" receiveEvent="_nrA">
          <argument xmi:type="uml:LiteralReal" xmi:id="_nmARpm" value="30.0"/>
        </message>
        <message xmi:type="uml:Message" xmi:id="_nmB" name="inner" messageSort="synchCall" signature="_nspin" sendEvent="_nsB" receiveEvent="_nrB">
          <argument xmi:type="uml:LiteralReal" xmi:id="_nmBRpm" value="40.0"/>
        </message>
        <message xmi:type="uml:Message" xmi:id="_nmRb" name="innerDone" messageSort="reply" signature="_nspin" sendEvent="_nsRb" receiveEvent="_nrRb">
          <argument xmi:type="uml:LiteralString" xmi:id="_nmRbArg" value="got ="/>
        </message>
        <message xmi:type="uml:Message" xmi:id="_nmRa" name="outerDone" messageSort="reply" signature="_nspin" sendEvent="_nsRa" receiveEvent="_nrRa">
          <argument xmi:type="uml:LiteralString" xmi:id="_nmRaArg" value="first ="/>
        </message>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_either" name="Either">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_elc" name="c" represents="_nCtrl"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_elm" name="m" represents="_nMotor"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_esS" covered="_elc" message="_emS"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_erS" covered="_elm" message="_emS"/>
        <fragment xmi:type="uml:CombinedFragment" xmi:id="_ealt" interactionOperator="alt">
          <operand xmi:type="uml:InteractionOperand" xmi:id="_ealtFast">
            <guard xmi:type="uml:InteractionConstraint" xmi:id="_ealtFastG">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_ealtFastS"><body>mode == 1</body></specification>
            </guard>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_esR1" covered="_elm" message="_emR1"/>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_erR1" covered="_elc" message="_emR1"/>
          </operand>
          <operand xmi:type="uml:InteractionOperand" xmi:id="_ealtElse">
            <guard xmi:type="uml:InteractionConstraint" xmi:id="_ealtElseG">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_ealtElseS"><body>else</body></specification>
            </guard>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_esR2" covered="_elm" message="_emR2"/>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_erR2" covered="_elc" message="_emR2"/>
          </operand>
        </fragment>
        <message xmi:type="uml:Message" xmi:id="_emS" name="spin" messageSort="synchCall" signature="_nspin" sendEvent="_esS" receiveEvent="_erS">
          <argument xmi:type="uml:LiteralReal" xmi:id="_emSRpm" value="20.0"/>
        </message>
        <message xmi:type="uml:Message" xmi:id="_emR1" name="fast" messageSort="reply" signature="_nspin" sendEvent="_esR1" receiveEvent="_erR1">
          <argument xmi:type="uml:LiteralString" xmi:id="_emR1Arg" value="got ="/>
        </message>
        <message xmi:type="uml:Message" xmi:id="_emR2" name="slow" messageSort="reply" signature="_nspin" sendEvent="_esR2" receiveEvent="_erR2">
          <argument xmi:type="uml:LiteralString" xmi:id="_emR2Arg" value="first ="/>
        </message>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_twice" name="Twice">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_tlc" name="c" represents="_nCtrl"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_tlm" name="m" represents="_nMotor"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_tsS" covered="_tlc" message="_tmS"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_trS" covered="_tlm" message="_tmS"/>
        <fragment xmi:type="uml:CombinedFragment" xmi:id="_topt" interactionOperator="opt">
          <operand xmi:type="uml:InteractionOperand" xmi:id="_toptOp">
            <guard xmi:type="uml:InteractionConstraint" xmi:id="_toptG">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_toptS"><body>mode == 1</body></specification>
            </guard>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_tsR1" covered="_tlm" message="_tmR1"/>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_trR1" covered="_tlc" message="_tmR1"/>
          </operand>
        </fragment>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_tsR2" covered="_tlm" message="_tmR2"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_trR2" covered="_tlc" message="_tmR2"/>
        <message xmi:type="uml:Message" xmi:id="_tmS" name="spin" messageSort="synchCall" signature="_nspin" sendEvent="_tsS" receiveEvent="_trS">
          <argument xmi:type="uml:LiteralReal" xmi:id="_tmSRpm" value="20.0"/>
        </message>
        <message xmi:type="uml:Message" xmi:id="_tmR1" name="early" messageSort="reply" signature="_nspin" sendEvent="_tsR1" receiveEvent="_trR1">
          <argument xmi:type="uml:LiteralString" xmi:id="_tmR1Arg" value="got ="/>
        </message>
        <message xmi:type="uml:Message" xmi:id="_tmR2" name="late" messageSort="reply" signature="_nspin" sendEvent="_tsR2" receiveEvent="_trR2">
          <argument xmi:type="uml:LiteralString" xmi:id="_tmR2Arg" value="first ="/>
        </message>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_split" name="Split">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_plc" name="c" represents="_nCtrl"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_plm" name="m" represents="_nMotor"/>
        <fragment xmi:type="uml:CombinedFragment" xmi:id="_ppar" interactionOperator="par">
          <operand xmi:type="uml:InteractionOperand" xmi:id="_pparOp1">
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_psS" covered="_plc" message="_pmS"/>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_prS" covered="_plm" message="_pmS"/>
          </operand>
          <operand xmi:type="uml:InteractionOperand" xmi:id="_pparOp2"/>
        </fragment>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_psR" covered="_plm" message="_pmR"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_prR" covered="_plc" message="_pmR"/>
        <message xmi:type="uml:Message" xmi:id="_pmS" name="spin" messageSort="synchCall" signature="_nspin" sendEvent="_psS" receiveEvent="_prS">
          <argument xmi:type="uml:LiteralReal" xmi:id="_pmSRpm" value="20.0"/>
        </message>
        <message xmi:type="uml:Message" xmi:id="_pmR" name="joined" messageSort="reply" signature="_nspin" sendEvent="_psR" receiveEvent="_prR">
          <argument xmi:type="uml:LiteralString" xmi:id="_pmRArg" value="got ="/>
        </message>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_crossed" name="Crossed">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_xlc" name="c" represents="_nCtrl"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_xlm" name="m" represents="_nMotor"/>
        <fragment xmi:type="uml:CombinedFragment" xmi:id="_xpar" interactionOperator="par">
          <operand xmi:type="uml:InteractionOperand" xmi:id="_xparOp1">
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_xsS" covered="_xlc" message="_xmS"/>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_xrS" covered="_xlm" message="_xmS"/>
          </operand>
          <operand xmi:type="uml:InteractionOperand" xmi:id="_xparOp2">
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_xsR" covered="_xlm" message="_xmR"/>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_xrR" covered="_xlc" message="_xmR"/>
          </operand>
        </fragment>
        <message xmi:type="uml:Message" xmi:id="_xmS" name="spin" messageSort="synchCall" signature="_nspin" sendEvent="_xsS" receiveEvent="_xrS">
          <argument xmi:type="uml:LiteralReal" xmi:id="_xmSRpm" value="20.0"/>
        </message>
        <message xmi:type="uml:Message" xmi:id="_xmR" name="beside" messageSort="reply" signature="_nspin" sendEvent="_xsR" receiveEvent="_xrR">
          <argument xmi:type="uml:LiteralString" xmi:id="_xmRArg" value="got ="/>
        </message>
      </ownedBehavior>
    </packagedElement>`

const nestedApplications = `
  <sysml:Block xmi:id="_nb1" base_Class="_nctl"/>
  <sysml:Block xmi:id="_nb2" base_Class="_nmotor"/>
  <sysml:Block xmi:id="_nb3" base_Class="_nrig"/>`

// Each reply answers the latest call of its operation between its lifelines that no earlier reply
// has answered, so nested calls pair with their replies stack-like. Alternative operands each
// resolve from the calls open before their fragment, so every branch may answer the same call,
// while a reply after a fragment that may already have answered its call answers none, and the
// operands of a par, unordered between themselves, do not answer each other's calls.
func TestNestedRepliesAnswerTheirOwnCalls(t *testing.T) {
	r := migrateDocument(t, nestedCalls, nestedApplications)
	for _, line := range []string{
		"perform action outer ::> motor.spin { in rpm[1] = 30.0; }",
		"perform action inner ::> motor.spin { in rpm[1] = 40.0; }",
		"assign ctrl.got := inner.result;",
		"assign ctrl.'first' := outer.result;",
		"action either {",
		"if mode == 1 {",
		"assign ctrl.got := spin.result;",
		"else {",
		"assign ctrl.'first' := spin.result;",
		"/* not migrated: Interaction 'Twice' — the message 'late' answers no call of Spin between its lifelines before it */",
		"/* not migrated: Interaction 'Crossed' — the combined fragment (_xpar) the message 'beside' answers no call of Spin between its lifelines before it */",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_nmRb", migrate.Mapped, "written as the assignment of the call inner's results to ctrl")
	wantNote(t, r, "_nmRa", migrate.Mapped, "written as the assignment of the call outer's results to ctrl")
	wantNote(t, r, "_emR1", migrate.Mapped, "written as the assignment of the call spin's results to ctrl")
	wantNote(t, r, "_emR2", migrate.Mapped, "written as the assignment of the call spin's results to ctrl")
	wantNote(t, r, "_twice", migrate.Unmapped, "the message 'late' answers no call of Spin between its lifelines before it")
	wantNote(t, r, "_pmR", migrate.Approximated, "the result result is not bound: the reply is not in the fragment of the call it answers")
	wantNote(t, r, "_crossed", migrate.Unmapped, "the message 'beside' answers no call of Spin between its lifelines before it")

	s := session(t, r)
	meta(t, s, "%instantiate Rig")
	meta(t, s, "%action Rig::nested #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "completed") {
		t.Errorf("the scenario did not complete:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : ctrl.got"); !strings.Contains(out, "= 40.0") {
		t.Errorf("the inner reply did not store the inner call's result:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : ctrl.'first'"); !strings.Contains(out, "= 30.0") {
		t.Errorf("the outer reply did not store the outer call's result:\n%s", out)
	}
	meta(t, s, "%action Rig::either #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "completed") {
		t.Errorf("the alternative scenario did not complete:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : ctrl.'first'"); !strings.Contains(out, "= 20.0") {
		t.Errorf("the else branch's reply did not store the call's result:\n%s", out)
	}
}

// loggingMachine is a state machine whose state names its entry behavior, its
// exit behavior and a nested state alike, as UML allows and v2 does not.
const loggingMachine = `
    <packagedElement xmi:type="uml:Class" xmi:id="_logger" name="Logger" classifierBehavior="_sm">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_count" name="count">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_count0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Logging">
        <region xmi:type="uml:Region" xmi:id="_r0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init0"/>
          <subvertex xmi:type="uml:State" xmi:id="_on_s" name="On">
            <entry xmi:type="uml:OpaqueBehavior" xmi:id="_onEntry" name="log">
              <language>JavaScript</language>
              <body>count = count + 1;</body>
            </entry>
            <exit xmi:type="uml:OpaqueBehavior" xmi:id="_onExit" name="log">
              <language>JavaScript</language>
              <body>count = count + 10;</body>
            </exit>
            <region xmi:type="uml:Region" xmi:id="_rIn">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_initIn"/>
              <subvertex xmi:type="uml:State" xmi:id="_log_s" name="log"/>
              <transition xmi:type="uml:Transition" xmi:id="_tIn" source="_initIn" target="_log_s"/>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init0" target="_on_s"/>
        </region>
      </ownedBehavior>
    </packagedElement>`

const loggingApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_logger"/>`

// A state's entry and exit behaviors and the states of its one region are
// members of one v2 body, so those sharing a name are told apart the way any
// clashing members are; the region's entry follows the state's own entry action.
func TestStateBehaviorsSharingANameAreDistinguished(t *testing.T) {
	r := migrateDocument(t, loggingMachine, loggingApplications)
	for _, line := range []string{
		"entry action log {",
		"exit action 'log 2' {",
		"then 'log 3';",
		"state 'log 3';",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "entry; then 'log 3'") {
		t.Errorf("the region's entry is written as a second entry action:\n%s", r.Notation)
	}
	wantNote(t, r, "_onExit", migrate.Approximated, "written as v2 assignments")
	if es := entriesFor(r, "_onExit"); len(es) == 1 && es[0].Target != "Logger::Logging::On::'log 2'" {
		t.Errorf("the exit behavior's report target is %q, not the name written", es[0].Target)
	}
	if es := entriesFor(r, "_log_s"); len(es) != 1 || es[0].Target != "'log 3'" {
		t.Errorf("the nested state's report entries are %+v, want one naming 'log 3'", es)
	}
	s := session(t, r)
	meta(t, s, "%instantiate Logger")
	meta(t, s, "%state Logger::Logging")
	meta(t, s, "%step")
	if out := meta(t, s, "%eval in #1 : count"); !strings.Contains(out, "= 1") {
		t.Errorf("the entry action did not run: %s", out)
	}
}

// inoutCall is a bench whose panel calls the counter's Bump, an operation with an inout
// parameter its method reads into count and then values anew, passing the bench's level.
const inoutCall = `
    <packagedElement xmi:type="uml:Class" xmi:id="_ipanel" name="Panel"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_icounter" name="Counter">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_iCount" name="count">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_iCount0" value="0"/>
      </ownedAttribute>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_ibump" name="Bump" method="_ibumping">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_ibLevel" name="level" direction="inout">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedParameter>
      </ownedOperation>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_ibumping" name="Bumping" specification="_ibump">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_ibLevel2" name="level" direction="inout">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedParameter>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_iapnIn" name="level" parameter="_ibLevel2"/>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_iapnOut" name="level" parameter="_ibLevel2"/>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_iset" name="set count" structuralFeature="_iCount" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_isetVal" name="value"/>
        </node>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_inine" name="nine">
          <value xmi:type="uml:LiteralInteger" xmi:id="_inineV" value="9"/>
          <result xmi:type="uml:OutputPin" xmi:id="_inineOut" name="result"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_iofIn" source="_iapnIn" target="_isetVal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_icfSet" source="_iset" target="_inine"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_iofOut" source="_inineOut" target="_iapnOut"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_ibench" name="Bench">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_iPanel" name="panel" type="_ipanel" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_iCounter" name="counter" type="_icounter" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_iLevel" name="level">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_iLevel0" value="3"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_iraise" name="Raise">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_ilp" name="p" represents="_iPanel" coveredBy="_isB"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_ilc" name="c" represents="_iCounter" coveredBy="_irB"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_isB" covered="_ilp" message="_imB"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_irB" covered="_ilc" message="_imB"/>
        <message xmi:type="uml:Message" xmi:id="_imB" name="bump" messageSort="synchCall" signature="_ibump" sendEvent="_isB" receiveEvent="_irB">
          <argument xmi:type="uml:OpaqueExpression" xmi:id="_imBArg"><body>level</body></argument>
        </message>
      </ownedBehavior>
    </packagedElement>`

const inoutApplications = `
  <sysml:Block xmi:id="_ib1" base_Class="_ipanel"/>
  <sysml:Block xmi:id="_ib2" base_Class="_icounter"/>
  <sysml:Block xmi:id="_ib3" base_Class="_ibench"/>`

// A call's argument for an inout parameter is bound with the parameter's direction, so the
// value the callee gives the parameter is written back to what the argument named.
func TestCallArgumentsKeepTheParameterDirection(t *testing.T) {
	r := migrateDocument(t, inoutCall, inoutApplications)
	wantLine(t, r.Notation, "perform action bump ::> counter.bump { inout level[1] = Bench::level; }")
	wantNoLine(t, r.Notation, "{ in level[1] = this.level; }")
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}

	s := session(t, r)
	meta(t, s, "%instantiate Bench")
	meta(t, s, "%action Bench::raise #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "completed") {
		t.Errorf("the scenario did not complete:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : counter.count"); !strings.Contains(out, "= 3") {
		t.Errorf("the call did not pass the level in:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : level"); !strings.Contains(out, "= 9") {
		t.Errorf("the call did not write the level back:\n%s", out)
	}
}

// openIntervals is an activity whose four steps each carry a duration interval
// lacking a bound: an expressionless max, no max, a max of *, an expressionless min.
const openIntervals = `
    <packagedElement xmi:type="uml:Duration" xmi:id="_oSixty">
      <expr xmi:type="uml:LiteralString" xmi:id="_oSixtyV" value="60s"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Duration" xmi:id="_oFive">
      <expr xmi:type="uml:LiteralString" xmi:id="_oFiveV" value="5s"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Duration" xmi:id="_oEight">
      <expr xmi:type="uml:LiteralString" xmi:id="_oEightV" value="8s"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Duration" xmi:id="_oBlank"/>
    <packagedElement xmi:type="uml:Duration" xmi:id="_oStar">
      <expr xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_oStarV" value="*"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_olab" name="Lab">
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_orun" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_oinit"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_oSoak" name="Soak" behavior="_ostep"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_oSettle" name="Settle" behavior="_ostep"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_oDrift" name="Drift" behavior="_ostep"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_oCool" name="Cool" behavior="_ostep"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_ofinal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_oe1" source="_oinit" target="_oSoak"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_oe2" source="_oSoak" target="_oSettle"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_oe3" source="_oSettle" target="_oDrift"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_oe4" source="_oDrift" target="_oCool"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_oe5" source="_oCool" target="_ofinal"/>
        <ownedRule xmi:type="uml:DurationConstraint" xmi:id="_odcOne" name="one">
          <constrainedElement xmi:idref="_oSoak"/>
          <specification xmi:type="uml:DurationInterval" xmi:id="_odiOne" min="_oSixty" max="_oBlank"/>
        </ownedRule>
        <ownedRule xmi:type="uml:DurationConstraint" xmi:id="_odcAbove" name="above">
          <constrainedElement xmi:idref="_oSettle"/>
          <specification xmi:type="uml:DurationInterval" xmi:id="_odiAbove" min="_oFive"/>
        </ownedRule>
        <ownedRule xmi:type="uml:DurationConstraint" xmi:id="_odcStar" name="star">
          <constrainedElement xmi:idref="_oDrift"/>
          <specification xmi:type="uml:DurationInterval" xmi:id="_odiStar" min="_oFive" max="_oStar"/>
        </ownedRule>
        <ownedRule xmi:type="uml:DurationConstraint" xmi:id="_odcBelow" name="below">
          <constrainedElement xmi:idref="_oCool"/>
          <specification xmi:type="uml:DurationInterval" xmi:id="_odiBelow" min="_oBlank" max="_oEight"/>
        </ownedRule>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_ostep" name="Step">
        <node xmi:type="uml:InitialNode" xmi:id="_osinit"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_osfinal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ose" source="_osinit" target="_osfinal"/>
      </ownedBehavior>
    </packagedElement>`

const openApplications = `
  <sysml:Block xmi:id="_ob1" base_Class="_olab"/>`

// An interval open on one side is reported, not waited for; only MagicDraw's
// expressionless max beside a min — its one-valued `{60s}` — is that value's wait.
func TestOneSidedDurationIntervalsAreNotFixedWaits(t *testing.T) {
	r := migrateDocument(t, openIntervals, openApplications)
	for _, line := range []string{
		"/* duration constraint on 'Soak' not migrated — the interval's max is not written: the duration has no expression, so the interval is open above and no one wait of at least 60.0 s stands for it */",
		"/* duration constraint on 'Settle' not migrated — the interval has no max, so the interval is open above and no one wait of at least 5.0 s stands for it */",
		"/* duration constraint on 'Drift' not migrated — the interval's max is not written: the duration * is unbounded, so the interval is open above and no one wait of at least 5.0 s stands for it */",
		"/* duration constraint on 'Cool' not migrated — the interval's min is not written: the duration has no expression, so the interval is open below and no one wait of at most 8.0 s stands for it */",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "accept after")
	wantNote(t, r, "_odcOne", migrate.Unmapped, "the interval's max is not written: the duration has no expression, so the interval is open above")
	wantNote(t, r, "_odcAbove", migrate.Unmapped, "the interval has no max, so the interval is open above and no one wait of at least 5.0 s stands for it")
	wantNote(t, r, "_odcStar", migrate.Unmapped, "the duration * is unbounded, so the interval is open above")
	wantNote(t, r, "_odcBelow", migrate.Unmapped, "the interval's min is not written: the duration has no expression, so the interval is open below and no one wait of at most 8.0 s stands for it")
	wantClean(t, "t.sysml", r)
	s := session(t, r)
	wantVerdict(t, s.RunAction("Lab::Run"))

	r = migrateDocument(t, openIntervals+`
    <xmi:Extension extender="MagicDraw UML 2024x"/>`, openApplications)
	wantLine(t, r.Notation, "action wait accept after 60.0 [SI::s];")
	wantLine(t, r.Notation, "first wait then Soak;")
	wantNote(t, r, "_odcOne", migrate.Approximated, "the max is a duration without an expression, MagicDraw's form of the one-valued constraint {60.0 s}; so the wait is a fixed 60.0 s before 'Soak'")
	wantNote(t, r, "_odcAbove", migrate.Unmapped, "the interval has no max, so the interval is open above")
	wantNote(t, r, "_odcStar", migrate.Unmapped, "the duration * is unbounded, so the interval is open above")
	wantNote(t, r, "_odcBelow", migrate.Unmapped, "the interval's min is not written: the duration has no expression, so the interval is open below")
	wantClean(t, "t.sysml", r)
}

// A transition out of an enclosing state, or out of a state in a region
// beside the deferring state's, that accepts the deferred signal is one v1's
// deferral takes precedence over; the standard encoding cannot hold the
// signal back from it, so the note names the transition, and the model runs
// as the note says: the enclosing state's transition takes the occurrence,
// and the sibling region's fires on one the loop keeps as well.
func TestDeferralNotesTransitionsItCannotOutrank(t *testing.T) {
	const enclosing = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_ping" name="Ping"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_go" name="Go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_pingEv" signal="_ping"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_goEv" signal="_go"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_mach" name="Machine" classifierBehavior="_sm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Run">
        <region xmi:type="uml:Region" xmi:id="_r0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init"/>
          <subvertex xmi:type="uml:State" xmi:id="_outer" name="Outer">
            <region xmi:type="uml:Region" xmi:id="_ro">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_oinit"/>
              <subvertex xmi:type="uml:State" xmi:id="_wait" name="Waiting">
                <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dPing" event="_pingEv"/>
              </subvertex>
              <subvertex xmi:type="uml:State" xmi:id="_open" name="Opened"/>
              <transition xmi:type="uml:Transition" xmi:id="_to0" source="_oinit" target="_wait"/>
              <transition xmi:type="uml:Transition" xmi:id="_tGo" source="_wait" target="_open">
                <trigger xmi:type="uml:Trigger" xmi:id="_trGo" event="_goEv"/>
              </transition>
            </region>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_pinged" name="Pinged"/>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init" target="_outer"/>
          <transition xmi:type="uml:Transition" xmi:id="_tPing" source="_outer" target="_pinged">
            <trigger xmi:type="uml:Trigger" xmi:id="_trPing" event="_pingEv"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`
	r := migrateDocumentOptions(t, enclosing, `<sysml:Block xmi:id="_b1" base_Class="_mach"/>`, migrate.Options{})
	wantNoLine(t, r.Notation, "defer Ping;")
	wantLine(t, r.Notation, "item deferred : Ping[*] ordered;")
	wantNote(t, r, "_dPing", migrate.Approximated, "kept in the item deferred by the accept loop of the do action buffer while the state is active, and sent to self by the exit action flush")
	wantNote(t, r, "_dPing", migrate.Approximated, "the transition (_tPing) out of the enclosing state 'Outer' accepts the signal too, which in v1 the deferral takes precedence over while the state is active; the standard encoding cannot hold a signal back from a transition of an enclosing state, so that transition takes each occurrence it can fire on and the accept loop keeps the rest")
	wantClean(t, "deferralEnclosingTransition", r)

	s := session(t, r)
	meta(t, s, "%instantiate Machine")
	meta(t, s, "%state Machine::Run")
	meta(t, s, "%send Ping")
	for i := 0; i < 4 && !strings.Contains(meta(t, s, "%current"), "Pinged"); i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Pinged") || strings.Contains(out, "deferred = [Instance") {
		t.Errorf("the Ping sent while Waiting did not take the enclosing state's transition alone:\n%s", out)
	}

	const sibling = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_ping" name="Ping"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_go" name="Go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_pingEv" signal="_ping"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_goEv" signal="_go"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_mach" name="Machine" classifierBehavior="_sm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Run">
        <region xmi:type="uml:Region" xmi:id="_r0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init"/>
          <subvertex xmi:type="uml:State" xmi:id="_both" name="Both">
            <region xmi:type="uml:Region" xmi:id="_left" name="left">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_linit"/>
              <subvertex xmi:type="uml:State" xmi:id="_lwait" name="LWait">
                <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dPing" event="_pingEv"/>
              </subvertex>
              <subvertex xmi:type="uml:State" xmi:id="_ldone" name="LDone"/>
              <transition xmi:type="uml:Transition" xmi:id="_tl0" source="_linit" target="_lwait"/>
              <transition xmi:type="uml:Transition" xmi:id="_tGo" source="_lwait" target="_ldone">
                <trigger xmi:type="uml:Trigger" xmi:id="_trGo" event="_goEv"/>
              </transition>
            </region>
            <region xmi:type="uml:Region" xmi:id="_right" name="right">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_rinit"/>
              <subvertex xmi:type="uml:State" xmi:id="_rwait" name="RWait"/>
              <subvertex xmi:type="uml:State" xmi:id="_rping" name="RPing"/>
              <transition xmi:type="uml:Transition" xmi:id="_tr0" source="_rinit" target="_rwait"/>
              <transition xmi:type="uml:Transition" xmi:id="_tPing" source="_rwait" target="_rping">
                <trigger xmi:type="uml:Trigger" xmi:id="_trPing" event="_pingEv"/>
              </transition>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init" target="_both"/>
        </region>
      </ownedBehavior>
    </packagedElement>`
	r = migrateDocumentOptions(t, sibling, `<sysml:Block xmi:id="_b1" base_Class="_mach"/>`, migrate.Options{})
	wantNoLine(t, r.Notation, "defer Ping;")
	wantLine(t, r.Notation, "item deferred : Ping[*] ordered;")
	wantNote(t, r, "_dPing", migrate.Approximated, "the transition (_tPing) out of 'RWait', in a region beside the state's, accepts the signal too, which in v1 the deferral takes precedence over while the state is active; the standard encoding cannot hold a signal back from a transition of another region, so that transition fires on each occurrence it can, which the accept loop keeps as well")
	wantClean(t, "deferralSiblingRegionTransition", r)

	s = session(t, r)
	meta(t, s, "%instantiate Machine")
	meta(t, s, "%state Machine::Run")
	meta(t, s, "%send Ping")
	for i := 0; i < 4 && !strings.Contains(meta(t, s, "%current"), "RPing"); i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "RPing") || !strings.Contains(out, "deferred = [Instance") {
		t.Errorf("the Ping sent while LWait was active did not both fire the sibling region's transition and stay kept:\n%s", out)
	}
}

// A transition to a terminate pseudostate, written as a terminate action, or
// through a submachine state by a connection point reference counts as written
// for deferral analysis: an unguarded completion transition drops the deferral,
// since its accept loop would never let it fire, and a triggered one takes it.
func TestDeferralCountsTransitionsToUnnamedTargets(t *testing.T) {
	const terminating = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_door" name="Door"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_doorEv" signal="_door"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_oven" name="Oven" classifierBehavior="_sm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Run">
        <region xmi:type="uml:Region" xmi:id="_r0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init"/>
          <subvertex xmi:type="uml:State" xmi:id="_off" name="Off">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dDoor" event="_doorEv"/>
          </subvertex>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_end" kind="terminate"/>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init" target="_off"/>
          <transition xmi:type="uml:Transition" xmi:id="_tEnd" source="_off" target="_end"/>
        </region>
      </ownedBehavior>
    </packagedElement>`
	r := migrateDocumentOptions(t, terminating, `<sysml:Block xmi:id="_b1" base_Class="_oven"/>`, migrate.Options{})
	wantLine(t, r.Notation, "action terminated terminate;")
	wantLine(t, r.Notation, "transition first Off then terminated;")
	wantNoLine(t, r.Notation, "item deferred : Door[*] ordered;")
	wantNote(t, r, "_dDoor", migrate.Unmapped, "the completion transition (_tEnd) leaves the state once its do action ends, which the accept loop that would keep Door never lets it, so the deferral is dropped")

	deferring := strings.Replace(nestedMachines, `<subvertex xmi:type="uml:State" xmi:id="_nfIdle" name="Idle"/>`,
		`<subvertex xmi:type="uml:State" xmi:id="_nfIdle" name="Idle">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dGo" event="_ngoEv"/>
          </subvertex>`, 1)
	r = migrateDocumentOptions(t, deferring, nestedMachinesApplications, migrate.Options{})
	wantLine(t, r.Notation, "transition first Idle accept Go then Sub.start2;")
	wantNoLine(t, r.Notation, "item deferred : Go[*] ordered;")
	wantNote(t, r, "_dGo", migrate.Approximated, "the transition (_nfT1) out of the state accepts the signal, which in v1 takes precedence over deferring it, so the state does not keep it")

	completing := strings.Replace(deferring, `<transition xmi:type="uml:Transition" xmi:id="_nfT1" source="_nfIdle" target="_nfRef">
            <trigger xmi:type="uml:Trigger" xmi:id="_nfTr1" event="_ngoEv"/>
          </transition>`, `<transition xmi:type="uml:Transition" xmi:id="_nfT1" source="_nfIdle" target="_nfRef"/>`, 1)
	if completing == deferring {
		t.Fatal("the triggered transition into the connection point reference was not made a completion transition")
	}
	r = migrateDocumentOptions(t, completing, nestedMachinesApplications, migrate.Options{})
	wantLine(t, r.Notation, "transition first Idle then Sub.start2;")
	wantNoLine(t, r.Notation, "item deferred : Go[*] ordered;")
	wantNote(t, r, "_dGo", migrate.Unmapped, "the completion transition (_nfT1) leaves the state once its do action ends, which the accept loop that would keep Go never lets it, so the deferral is dropped")
	wantClean(t, "deferralCompletionIntoSubmachine", r)

	s := session(t, r)
	meta(t, s, "%instantiate Rig")
	meta(t, s, "%state Rig::Front")
	for i := 0; i < 6 && !strings.Contains(meta(t, s, "%current"), "Sub"); i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Sub") {
		t.Errorf("Idle did not complete into the submachine state:\n%s", out)
	}
}
