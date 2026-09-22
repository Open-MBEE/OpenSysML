package migrate_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// methodStubs is a block whose operation Scan is implemented by the activity
// Scanning, in which a call behavior action naming no behavior owns a typed input
// pin, an untyped input pin and a result pin declared [0..*]; it is «Allocate»d
// to a part of the block through a Dependency.
const methodStubs = `
    <packagedElement xmi:type="uml:Class" xmi:id="_lens" name="Lens"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_cam" name="Cam">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_glass" name="glass" type="_lens" aggregation="composite"/>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_scan" name="Scan" method="_scanning"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_scanning" name="Scanning" specification="_scan">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_sweep" name="sweep">
          <argument xmi:type="uml:InputPin" xmi:id="_sweepRate" name="rate">
            <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
          </argument>
          <argument xmi:type="uml:InputPin" xmi:id="_sweepMode" name="mode"/>
          <result xmi:type="uml:OutputPin" xmi:id="_sweepOut" name="frames">
            <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
            <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_sweepLo" value="0"/>
            <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_sweepHi" value="*"/>
          </result>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_sweep"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_sweep" target="_final"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Dependency" xmi:id="_alloc">
      <client xmi:idref="_sweep"/>
      <supplier xmi:idref="_glass"/>
    </packagedElement>`

const methodStubApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_lens"/>
  <sysml:Block xmi:id="_b2" base_Class="_cam"/>
  <sysml:Allocate xmi:id="_s1" base_Dependency="_alloc"/>`

// A pin-bearing call naming no behavior in an operation's method is declared with
// its pins as parameters, an untyped pin left untyped and a [0..*] pin kept so, and
// the «Allocate» Dependency names it under the operation the method is the body of.
func TestStubActionsInMethodsKeepTheirPinsAndAllocation(t *testing.T) {
	r := migrateDocument(t, methodStubs, methodStubApplications)
	wantNote(t, r, "_sweep", migrate.Approximated, "a step with no behavior and no duration, which passes the token on; its pins are declared as its parameters, but the action computes nothing, so its output 'frames' holds no value; its «Allocate» to Cam::glass says where it runs, not what it does")
	wantNote(t, r, "_sweepOut", migrate.Approximated, "it is declared admitting no value: the action calls no behavior, so nothing computes it")
	wantNote(t, r, "_sweepRate", migrate.Mapped, "")
	wantNote(t, r, "_sweepMode", migrate.Mapped, "")
	wantNote(t, r, "_alloc", migrate.Mapped, "")
	for _, line := range []string{
		"action sweep {",
		"in rate : ScalarValues::Real;",
		"in mode;",
		"out frames : ScalarValues::Integer[0..*];",
		"allocate Cam::Scan::sweep to Cam::glass;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "Cam::Scanning::sweep")
	wantClean(t, "t.sysml", r)
}

// danglingStubs is an activity whose call behavior actions are incompletely
// serialized: one names a behavior the document has no element for, one owns a
// pin typed by nothing the document resolves, and an object flow leaves a pin
// for a target the document has no element for.
const danglingStubs = `
    <packagedElement xmi:type="uml:Class" xmi:id="_probe" name="Probe"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_ctl" name="Ctl">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_eye" name="eye" type="_probe" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_lost" name="lost" behavior="_nowhere">
          <result xmi:type="uml:OutputPin" xmi:id="_lostOut" name="reading"/>
        </node>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_odd" name="odd">
          <argument xmi:type="uml:InputPin" xmi:id="_oddIn" name="gain" type="_missingType"/>
          <result xmi:type="uml:OutputPin" xmi:id="_oddOut" name="reading"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_lost"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_lost" target="_odd"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_odd" target="_final"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of1" source="_oddOut" target="_gonePin"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_allocLost">
      <client xmi:idref="_lost"/>
      <supplier xmi:idref="_eye"/>
    </packagedElement>`

const danglingApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_probe"/>
  <sysml:Block xmi:id="_b2" base_Class="_ctl"/>
  <sysml:Allocate xmi:id="_s1" base_Abstraction="_allocLost"/>`

// A call whose behavior reference resolves to nothing is not a stub: it is left a
// placeholder, and its allocation is migrated no better than it. A stub whose pin's
// type resolves to nothing declares the pin untyped, and a flow from it to a missing
// target is dropped with a reason; the output still validates.
func TestIncompletelySerializedCallsAreRefusedNotStubbed(t *testing.T) {
	r := migrateDocument(t, danglingStubs, danglingApplications)
	wantNote(t, r, "_lost", migrate.Unmapped, "1 behavior reference(s) resolve to nothing in the document (_nowhere); the action calls no behavior, yet has the pins 'reading', which nothing then computes; its «Allocate» to Ctl::eye says where it runs, not what it does")
	wantNote(t, r, "_allocLost", migrate.Unmapped, "its end Ctl::Run::lost is written only as a placeholder of a node that is not migrated")
	wantNote(t, r, "_odd", migrate.Approximated, "a step with no behavior and no duration, which passes the token on; its pins are declared as its parameters, but the action computes nothing, so its output 'reading' holds no value")
	wantNote(t, r, "_oddOut", migrate.Approximated, "it is declared admitting no value: the action calls no behavior, so nothing computes it")
	for _, line := range []string{
		"action odd {",
		"in gain;",
		"out reading[0..1];",
	} {
		wantLine(t, r.Notation, line)
	}
	wantLine(t, r.Notation, "allocate Ctl::Run::lost to Ctl::eye;")
	wantNoLine(t, r.Notation, "flow odd.reading")
	if entries := entriesFor(r, "_of1"); len(entries) != 1 || entries[0].Verdict != migrate.Unmapped {
		t.Errorf("flow to a missing pin: got %+v, want one unmapped entry", entries)
	}
	wantClean(t, "t.sysml", r)
}
