package migrate_test

import (
	"strings"
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

// An activity output fed only by a stub's output is declared admitting no value,
// since the stub computes none, so the activity completes with it empty.
func TestActivityOutputFedByStubAdmitsNoValue(t *testing.T) {
	r := migrateFixtureFile(t, "stub_actions")
	wantNote(t, r, "_sample", migrate.Approximated, "it is declared admitting no value: the pin 'reading' of 'measure', which feeds it, admits no value: the action calls no behavior, so nothing computes it")
	wantLine(t, r.Notation, "out sample : ScalarValues::Real[0..1];")
	wantLine(t, r.Notation, "bind sample = measure.reading;")
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

// earlyAllocation is an «Allocate» written before the activity whose anonymous
// node it ends at, so that node is named ahead of its graph's writer; an anonymous
// sibling of the same kind then competes for the same synthesized name.
const earlyAllocation = `
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_alloc">
      <client xmi:idref="_second"/>
      <supplier xmi:idref="_eye"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_probe" name="Probe"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_rig" name="Rig">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_eye" name="probe" type="_probe" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_first"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_second"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_first"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_first" target="_second"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_second" target="_final"/>
      </ownedBehavior>
    </packagedElement>`

const earlyAllocationApplications = `
  <sysml:Block xmi:id="_p1" base_Class="_probe"/>
  <sysml:Block xmi:id="_p2" base_Class="_rig"/>
  <sysml:Allocate xmi:id="_p3" base_Abstraction="_alloc"/>`

// A node named ahead of its graph's writer keeps that name, and the writer numbers
// the sibling that would otherwise take it, so the allocation names the right node.
func TestNodeNamedAheadOfItsWriterKeepsSiblingsDistinct(t *testing.T) {
	r := migrateDocument(t, earlyAllocation, earlyAllocationApplications)
	wantLine(t, r.Notation, "allocate Rig::Run::call to Rig::probe;")
	wantLine(t, r.Notation, "first call2 then call;")
	wantLine(t, r.Notation, "action call2;")
	wantLine(t, r.Notation, "action call;")
	wantNote(t, r, "_second", migrate.Approximated, "a step with no behavior and no duration; it passes the token on")
	if e := entriesFor(r, "_second"); len(e) != 1 || e[0].Target != "call" {
		t.Errorf("node named ahead of its writer: got %+v, want target call", e)
	}
	if e := entriesFor(r, "_first"); len(e) != 1 || e[0].Target != "call2" {
		t.Errorf("its sibling: got %+v, want target call2", e)
	}
	wantClean(t, "t.sysml", r)
}

// mixedAllocation is an «Allocate» from a part to two nodes of an activity: a stub
// call the migrator writes, and an action of a kind it has no v2 form for, which it
// writes only as a placeholder.
const mixedAllocation = `
    <packagedElement xmi:type="uml:Class" xmi:id="_probe" name="Probe"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_rig" name="Rig">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_eye" name="probe" type="_probe" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_scan" name="scan"/>
        <node xmi:type="uml:CreateObjectAction" xmi:id="_make" name="make" classifier="_probe"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_scan"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_scan" target="_make"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_make" target="_final"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_alloc">
      <client xmi:idref="_eye"/>
      <supplier xmi:idref="_scan"/>
      <supplier xmi:idref="_make"/>
    </packagedElement>`

const mixedAllocationApplications = `
  <sysml:Block xmi:id="_p1" base_Class="_probe"/>
  <sysml:Block xmi:id="_p2" base_Class="_rig"/>
  <sysml:Allocate xmi:id="_p3" base_Abstraction="_alloc"/>`

// A pair ending at a placeholder counts against the relationship as one pair, not as
// the whole: the pair to the written node keeps the «Allocate» approximated, and it
// is unmapped only when every pair ends so.
func TestPlaceholderEndFailsOnlyItsOwnPair(t *testing.T) {
	r := migrateDocument(t, mixedAllocation, mixedAllocationApplications)
	wantNote(t, r, "_make", migrate.Unmapped, "no v2 form for a UML CreateObjectAction")
	wantNote(t, r, "_alloc", migrate.Approximated, "written as 2 relationships, one per client–supplier pair; its end Rig::Run::make is written only as a placeholder of a node that is not migrated")
	wantLine(t, r.Notation, "allocate Rig::probe to Rig::Run::scan;")
	wantLine(t, r.Notation, "allocate Rig::probe to Rig::Run::make;")
	wantClean(t, "t.sysml", r)

	only := strings.Replace(mixedAllocation, `<supplier xmi:idref="_scan"/>`, "", 1)
	r = migrateDocument(t, only, mixedAllocationApplications)
	wantNote(t, r, "_alloc", migrate.Unmapped, "its end Rig::Run::make is written only as a placeholder of a node that is not migrated")
}

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
