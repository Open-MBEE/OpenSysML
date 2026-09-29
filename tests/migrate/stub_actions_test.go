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

// mixedEndsNote explains an «Allocate» written as a plain dependency because an
// end is an action of an action def, which no allocate reaches: an allocate in a
// package relates package-level features and one in a definition its own.
const mixedEndsNote = "an allocation written in a package relates features of no definition, and one written in a definition relates its features; the ends are neither, so a plain dependency stands for it"

// A pin-bearing call naming no behavior in an operation's method is declared with
// its pins as parameters, an untyped pin left untyped and a [0..*] pin kept so, and
// the «Allocate» Dependency names it under the operation the method is the body of,
// as a plain dependency: an action of an action def is a feature no allocate reaches.
func TestStubActionsInMethodsKeepTheirPinsAndAllocation(t *testing.T) {
	r := migrateDocument(t, methodStubs, methodStubApplications)
	wantNote(t, r, "_sweep", migrate.Approximated, "a step with no behavior and no duration, which passes the token on; its pins are declared as its parameters, but the action computes nothing, so its output 'frames' holds no value; its «Allocate» to Cam::glass says where it runs, not what it does")
	wantNote(t, r, "_sweepOut", migrate.Approximated, "it is declared admitting no value: the action calls no behavior, so nothing computes it")
	wantNote(t, r, "_sweepRate", migrate.Mapped, "")
	wantNote(t, r, "_sweepMode", migrate.Mapped, "")
	wantNote(t, r, "_alloc", migrate.Approximated, mixedEndsNote)
	for _, line := range []string{
		"action sweep {",
		"in rate : ScalarValues::Real[1];",
		"in mode[1];",
		"out frames : ScalarValues::Integer[0..*];",
		"dependency Cam::Scan::sweep to Cam::glass;",
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
	wantLine(t, r.Notation, "dependency Rig::Run::call to Rig::probe;")
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
    <packagedElement xmi:type="uml:Abstraction" xmi:id="_alloc" name="wire">
      <client xmi:idref="_eye"/>
      <supplier xmi:idref="_scan"/>
      <supplier xmi:idref="_make"/>
    </packagedElement>`

const mixedAllocationApplications = `
  <sysml:Block xmi:id="_p1" base_Class="_probe"/>
  <sysml:Block xmi:id="_p2" base_Class="_rig"/>
  <sysml:Allocate xmi:id="_p3" base_Abstraction="_alloc"/>`

// A pair ending at a placeholder counts against the relationship as one failed pair,
// not as the whole: the report says how many pairs were written and points at the
// one that was, and the «Allocate» is unmapped only when every pair ends so.
func TestPlaceholderEndFailsOnlyItsOwnPair(t *testing.T) {
	r := migrateDocument(t, mixedAllocation, mixedAllocationApplications)
	wantNote(t, r, "_make", migrate.Unmapped, "no v2 form for a UML CreateObjectAction")
	wantNote(t, r, "_alloc", migrate.Approximated, "1 of 2 relationships written; "+mixedEndsNote+"; pair 2 is named wire 2 so the pairs stay distinct; its end Rig::Run::make is written only as a placeholder of a node that is not migrated")
	if e := entriesFor(r, "_alloc"); len(e) != 1 || e[0].Target != "wire" {
		t.Errorf("mixed allocation: got %+v, want target wire", e)
	}
	wantLine(t, r.Notation, "dependency wire from Rig::probe to Rig::Run::scan;")
	wantLine(t, r.Notation, "dependency 'wire 2' from Rig::probe to Rig::Run::make;")
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
		"in gain[1];",
		"out reading[0..1];",
	} {
		wantLine(t, r.Notation, line)
	}
	wantLine(t, r.Notation, "dependency Ctl::Run::lost to Ctl::eye;")
	wantNoLine(t, r.Notation, "flow odd.reading")
	if entries := entriesFor(r, "_of1"); len(entries) != 1 || entries[0].Verdict != migrate.Unmapped {
		t.Errorf("flow to a missing pin: got %+v, want one unmapped entry", entries)
	}
	wantClean(t, "t.sysml", r)
}

// ownerUsageAllocation is a block Ctl whose activity Run sets the block's own
// status, so it is written as an action usage of Ctl, «Allocate»d to the block
// Motor; a stereotype Tags::Tracked tags Motor with an element-valued `runs` naming Run.
const ownerUsageAllocation = `
    <packagedElement xmi:type="uml:Profile" xmi:id="_prof" name="Tags" URI="http://example.com/schemas/Tags.xmi">
      <packagedElement xmi:type="uml:Stereotype" xmi:id="_st" name="Tracked">
        <ownedAttribute xmi:type="uml:Property" xmi:id="_st_base" name="base_Class" association="_ext">
          <type xmi:type="uml:Class" href="http://www.omg.org/spec/UML/20131001/UML.xmi#Class"/>
        </ownedAttribute>
        <ownedAttribute xmi:type="uml:Property" xmi:id="_st_runs" name="runs">
          <type xmi:type="uml:Class" href="http://www.omg.org/spec/UML/20131001/UML.xmi#Element"/>
        </ownedAttribute>
      </packagedElement>
      <packagedElement xmi:type="uml:Extension" xmi:id="_ext" memberEnd="_st_base _ext_end">
        <ownedEnd xmi:type="uml:ExtensionEnd" xmi:id="_ext_end" name="extension_Tracked" type="_st" aggregation="composite"/>
      </packagedElement>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_motor" name="Motor"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_ctl" name="Ctl">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_status" name="status">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_status0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_ri"/>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_one" name="one">
          <value xmi:type="uml:LiteralInteger" xmi:id="_oneV" value="1"/>
          <result xmi:type="uml:OutputPin" xmi:id="_oneOut" name="result"/>
        </node>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_set" name="set status" structuralFeature="_status" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_setVal" name="value"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_rf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re1" source="_ri" target="_one"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_rof" source="_oneOut" target="_setVal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re2" source="_set" target="_rf"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Dependency" xmi:id="_alloc">
      <client xmi:idref="_run"/>
      <supplier xmi:idref="_motor"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Dependency" xmi:id="_alloc2">
      <client xmi:idref="_set"/>
      <supplier xmi:idref="_motor"/>
    </packagedElement>`

const ownerUsageAllocationApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_motor"/>
  <sysml:Block xmi:id="_b2" base_Class="_ctl"/>
  <sysml:Allocate xmi:id="_s1" base_Dependency="_alloc"/>
  <sysml:Allocate xmi:id="_s2" base_Dependency="_alloc2"/>
  <Tags:Tracked xmlns:Tags="http://example.com/schemas/Tags.xmi" xmi:id="_a_tr" base_Class="_motor" runs="_run"/>`

// An activity written as an action usage of its block is no definition: an
// «Allocate» from it, or from a node of it, to a block is a plain dependency,
// whose ends are qualified names, not an allocation def with an end typed by
// the usage; and a tag naming it casts it to SysML::ActionUsage, not a definition.
func TestBehaviorsWrittenAsUsagesAreNoDefinitionEnds(t *testing.T) {
	r := migrateDocument(t, ownerUsageAllocation, ownerUsageAllocationApplications)
	for _, line := range []string{
		"action run {",
		"dependency Ctl::run to Motor;",
		"dependency Ctl::run::'set status' to Motor;",
		"runs = Ctl::run meta SysML::ActionUsage;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "allocation def")
	wantNoLine(t, r.Notation, "meta SysML::ActionDefinition")
	wantClean(t, "t.sysml", r)
}

// unfedTargetCall is a block Ctl with one part motor : Motor, whose activity Run
// calls Motor's operation Spin — whose method sets Motor's own rpm, so it is
// written as an action usage of Motor — through a target pin no flow feeds.
const unfedTargetCall = `
    <packagedElement xmi:type="uml:Class" xmi:id="_motor" name="Motor">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_rpm" name="rpm">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_rpm0" value="0"/>
      </ownedAttribute>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_spin" name="Spin" method="_spinning"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_spinning" name="Spinning" specification="_spin">
        <node xmi:type="uml:InitialNode" xmi:id="_si"/>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_ten" name="ten">
          <value xmi:type="uml:LiteralInteger" xmi:id="_tenV" value="10"/>
          <result xmi:type="uml:OutputPin" xmi:id="_tenOut" name="result"/>
        </node>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_setRpm" name="set rpm" structuralFeature="_rpm" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_setRpmVal" name="value"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_sf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_se1" source="_si" target="_ten"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_sof" source="_tenOut" target="_setRpmVal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_se2" source="_setRpm" target="_sf"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_ctl" name="Ctl">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ctlMotor" name="motor" type="_motor" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_ri"/>
        <node xmi:type="uml:CallOperationAction" xmi:id="_call" name="spin" operation="_spin">
          <target xmi:type="uml:InputPin" xmi:id="_callTgt" name="target"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_rf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re1" source="_ri" target="_call"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re2" source="_call" target="_rf"/>
      </ownedBehavior>
    </packagedElement>`

const unfedTargetCallApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_motor"/>
  <sysml:Block xmi:id="_b2" base_Class="_ctl"/>`

// A call whose target pin no flow feeds names no object: it is not performed on
// the caller's sole part of the operation's block, which the model never chose;
// an empty step stands for it and the report says why.
func TestUnfedTargetPinDoesNotPickAPart(t *testing.T) {
	r := migrateDocument(t, unfedTargetCall, unfedTargetCallApplications)
	wantLine(t, r.Notation, "action spin;")
	wantNoLine(t, r.Notation, "::> motor.spin")
	wantNote(t, r, "_call", migrate.Approximated, "the call runs in the caller's context: no flow feeds its target pin; spin is an action of Motor, performed on an object of it, and the target pin names none read from this, so an empty step stands for the call")
	wantNote(t, r, "_callTgt", migrate.Approximated, "the target pin is not written: it names no object read from this")
	wantClean(t, "t.sysml", r)
}

// externalMethodCall is the block Motor of unfedTargetCall, whose operation Spin
// no call names, and the block Rig, whose activity Bench calls Spin's method
// Spinning directly while Rig holds no object of Motor.
const externalMethodCall = `
    <packagedElement xmi:type="uml:Class" xmi:id="_motor" name="Motor">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_rpm" name="rpm">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_rpm0" value="0"/>
      </ownedAttribute>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_spin" name="Spin" method="_spinning"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_spinning" name="Spinning" specification="_spin">
        <node xmi:type="uml:InitialNode" xmi:id="_si"/>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_ten" name="ten">
          <value xmi:type="uml:LiteralInteger" xmi:id="_tenV" value="10"/>
          <result xmi:type="uml:OutputPin" xmi:id="_tenOut" name="result"/>
        </node>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_setRpm" name="set rpm" structuralFeature="_rpm" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_setRpmVal" name="value"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_sf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_se1" source="_si" target="_ten"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_sof" source="_tenOut" target="_setRpmVal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_se2" source="_setRpm" target="_sf"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_rig" name="Rig">
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_bench" name="Bench">
        <node xmi:type="uml:InitialNode" xmi:id="_bi"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callSpinning" name="spin" behavior="_spinning"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_bf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_be1" source="_bi" target="_callSpinning"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_be2" source="_callSpinning" target="_bf"/>
      </ownedBehavior>
    </packagedElement>`

// A caller of an operation's method is a caller of the operation: one that
// reaches no object of the block keeps the operation a definition the call
// names, rather than a usage the call could only stand for by an empty step.
func TestMethodCallersKeepTheOperationADefinition(t *testing.T) {
	r := migrateDocument(t, externalMethodCall, `<sysml:Block xmi:id="_b1" base_Class="_motor"/><sysml:Block xmi:id="_b2" base_Class="_rig"/>`)
	wantLine(t, r.Notation, "action def Spin {")
	wantLine(t, r.Notation, "action spin : Spin;")
	wantNote(t, r, "_callSpinning", migrate.Approximated, "which is left unbound: the caller is a Rig, which is no Motor and has no part that is one")
	wantClean(t, "t.sysml", r)
}

// mixedGenerals is a block Ctl whose activities Active and Run set the block's
// status, so both are written as its action usages, Run generalized by Active
// and by the package activity Basis, which stays an action def.
const mixedGenerals = `
    <packagedElement xmi:type="uml:Activity" xmi:id="_base" name="Basis"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_ctl" name="Ctl">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_status" name="status">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_status0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_active" name="Active">
        <node xmi:type="uml:InitialNode" xmi:id="_ai"/>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_aone" name="one">
          <value xmi:type="uml:LiteralInteger" xmi:id="_aoneV" value="1"/>
          <result xmi:type="uml:OutputPin" xmi:id="_aoneOut" name="result"/>
        </node>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_aset" name="set status" structuralFeature="_status" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_asetVal" name="value"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_af"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ae1" source="_ai" target="_aone"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_aof" source="_aoneOut" target="_asetVal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ae2" source="_aset" target="_af"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <generalization xmi:type="uml:Generalization" xmi:id="_g1" general="_base"/>
        <generalization xmi:type="uml:Generalization" xmi:id="_g2" general="_active"/>
        <node xmi:type="uml:InitialNode" xmi:id="_ri"/>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_two" name="two">
          <value xmi:type="uml:LiteralInteger" xmi:id="_twoV" value="2"/>
          <result xmi:type="uml:OutputPin" xmi:id="_twoOut" name="result"/>
        </node>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_rset" name="set status" structuralFeature="_status" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_rsetVal" name="value"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_rf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re1" source="_ri" target="_two"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_rof" source="_twoOut" target="_rsetVal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re2" source="_rset" target="_rf"/>
      </ownedBehavior>
    </packagedElement>`

// An action usage's general written as an action def types it, and one written
// as an action usage is subsetted: each keeps its own relationship.
func TestUsageGeneralsAreTypedByDefinitionsAndSubsetUsages(t *testing.T) {
	r := migrateDocument(t, mixedGenerals, `<sysml:Block xmi:id="_b1" base_Class="_ctl"/>`)
	wantLine(t, r.Notation, "action def Basis;")
	wantLine(t, r.Notation, "action active {")
	wantLine(t, r.Notation, "action run : Basis :> active {")
	wantClean(t, "t.sysml", r)
}
