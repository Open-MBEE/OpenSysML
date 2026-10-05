package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// objectReads is a block whose behavior reads itself and its features: at the
// top level, inside a structured node, across the node's boundary, and through
// an association end the block does not own.
const objectReads = `
    <packagedElement xmi:type="uml:Class" xmi:id="_item" name="Item"/>
    <packagedElement xmi:type="uml:Association" xmi:id="_link" name="Link" memberEnd="_holder _held">
      <ownedEnd xmi:type="uml:Property" xmi:id="_holder" name="holder" type="_account" association="_link"/>
      <ownedEnd xmi:type="uml:Property" xmi:id="_held" name="held" type="_item" association="_link"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_account" name="Account" classifierBehavior="_update">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_balance" name="balance">` + integerHref + `</ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_update" name="Update">
        <node xmi:type="uml:ReadSelfAction" xmi:id="_self" name="self">
          <result xmi:type="uml:OutputPin" xmi:id="_selfOut" name="result" type="_account"/>
        </node>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_read" name="read balance" structuralFeature="_balance">
          <object xmi:type="uml:InputPin" xmi:id="_readObj" name="object" type="_account"/>
          <result xmi:type="uml:OutputPin" xmi:id="_readOut" name="result">` + integerHref + `</result>
        </node>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_readHeld" name="read held" structuralFeature="_held">
          <object xmi:type="uml:InputPin" xmi:id="_heldObj" name="object" type="_account"/>
          <result xmi:type="uml:OutputPin" xmi:id="_heldOut" name="result" type="_item"/>
        </node>
        <node xmi:type="uml:StructuredActivityNode" xmi:id="_body" name="body">
          <node xmi:type="uml:ReadSelfAction" xmi:id="_innerSelf" name="inner self">
            <result xmi:type="uml:OutputPin" xmi:id="_innerSelfOut" name="result" type="_account"/>
          </node>
          <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_innerRead" name="inner read" structuralFeature="_balance">
            <object xmi:type="uml:InputPin" xmi:id="_innerObj" name="object" type="_account"/>
            <result xmi:type="uml:OutputPin" xmi:id="_innerOut" name="result">` + integerHref + `</result>
          </node>
          <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_crossRead" name="cross read" structuralFeature="_balance">
            <object xmi:type="uml:InputPin" xmi:id="_crossObj" name="object" type="_account"/>
            <result xmi:type="uml:OutputPin" xmi:id="_crossOut" name="result">` + integerHref + `</result>
          </node>
          <edge xmi:type="uml:ObjectFlow" xmi:id="_f3" source="_innerSelfOut" target="_innerObj"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_f1" source="_selfOut" target="_readObj"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_f2" source="_selfOut" target="_heldObj"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_f4" source="_selfOut" target="_crossObj"/>
      </ownedBehavior>
    </packagedElement>`

const objectReadsApps = `
  <sysml:Block xmi:id="_s1" base_Class="_account"/>
  <sysml:Block xmi:id="_s2" base_Class="_item"/>`

// A read self action's result is bound to the object the behavior runs on, at
// the top level and inside a structured node.
func TestReadSelfActionResultIsThis(t *testing.T) {
	r := migrateDocument(t, objectReads, objectReadsApps)
	wantLine(t, r.Notation, "out result : Account[1] = this;")
	wantNote(t, r, "_selfOut", migrate.Mapped, "")
	wantNote(t, r, "_innerSelf", migrate.Mapped, "")
	wantNote(t, r, "_innerSelfOut", migrate.Mapped, "")
	wantClean(t, "object_reads.sysml", r)
}

// A read structural feature action is the feature chain on its object: this
// inside a structured node is the behavior's context, not the node, and an
// object a flow brings into the node is the object pin, not this.
func TestReadFeatureChainsOnTheObjectItReads(t *testing.T) {
	r := migrateDocument(t, objectReads, objectReadsApps)
	for _, line := range []string{
		"out result[1] = balance;",
		"in object : Account[1];",
		"out result[1] = object.balance;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_read", migrate.Mapped, "")
	wantNote(t, r, "_innerRead", migrate.Mapped, "")
	wantNote(t, r, "_innerOut", migrate.Mapped, "")
	wantNote(t, r, "_crossOut", migrate.Mapped, "")
	wantClean(t, "object_reads.sysml", r)
}

// An association-owned end is no feature of the type at its other end in v2,
// so a read of it through that type is left unmigrated rather than written
// as a chain that does not resolve.
func TestReadOfAnAssociationOwnedEndIsRefused(t *testing.T) {
	r := migrateDocument(t, objectReads, objectReadsApps)
	wantNote(t, r, "_readHeld", migrate.Unmapped, "the object read, this, is a Account, which has no feature held")
	wantNoLine(t, r.Notation, "out result : Item[1] = held;")
}

// selfFlows is a behavior whose read self action, named this, feeds an opaque
// action's input and a call's argument.
const selfFlows = `
    <packagedElement xmi:type="uml:Class" xmi:id="_acct" name="Acct" classifierBehavior="_run">
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_notify" name="Notify">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_who" name="who" direction="in" type="_acct"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:ReadSelfAction" xmi:id="_rs" name="this">
          <result xmi:type="uml:OutputPin" xmi:id="_rsOut" name="result" type="_acct"/>
        </node>
        <node xmi:type="uml:OpaqueAction" xmi:id="_op" name="use">
          <inputValue xmi:type="uml:InputPin" xmi:id="_opIn" name="argument" type="_acct"/>
        </node>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_cb" name="call" behavior="_notify">
          <argument xmi:type="uml:InputPin" xmi:id="_cbIn" name="who" type="_acct"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_g1" source="_rsOut" target="_opIn"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_g2" source="_rsOut" target="_cbIn"/>
      </ownedBehavior>
    </packagedElement>`

// A read self action's result flows to the pins it feeds that are written, and
// a node named this is renamed, since every action has a this of its own.
func TestReadSelfResultFlowsToTheActionsItFeeds(t *testing.T) {
	r := migrateDocument(t, selfFlows, `<sysml:Block xmi:id="_s9" base_Class="_acct"/>`)
	for _, line := range []string{
		"action this2 {",
		"out result : Acct[1] = this;",
		"flow this2.result to 'use'.argument;",
		"flow this2.result to call.who;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_g1", migrate.Mapped, "")
	wantNote(t, r, "_g2", migrate.Mapped, "")
	wantClean(t, "self_flows.sysml", r)
}

const guardedSelfFlow = `
    <packagedElement xmi:type="uml:Class" xmi:id="_acct" name="Acct" classifierBehavior="_run">
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:ReadSelfAction" xmi:id="_rs" name="me">
          <result xmi:type="uml:OutputPin" xmi:id="_rsOut" name="result" type="_acct"/>
        </node>
        <node xmi:type="uml:OpaqueAction" xmi:id="_op" name="use">
          <inputValue xmi:type="uml:InputPin" xmi:id="_opIn" name="argument" type="_acct"/>
        </node>
        <edge xmi:type="uml:ControlFlow" xmi:id="_c1" source="_init" target="_op"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_g1" source="_rsOut" target="_opIn">
          <guard xmi:type="uml:LiteralBoolean" xmi:id="_g1Guard" value="false"/>
        </edge>
      </ownedBehavior>
    </packagedElement>`

// A guard on the flow from a self read must gate the value, and a flow written from
// the result would deliver it even when a separate path starts the receiver.
func TestGuardedReadSelfResultFlowIsNotWrittenUnguarded(t *testing.T) {
	r := migrateDocument(t, guardedSelfFlow,
		`<sysml:Block xmi:id="_s9" base_Class="_acct"/>`)
	wantLine(t, r.Notation, "out result : Acct[1] = this;")
	wantNoLine(t, r.Notation, "flow me.result to 'use'.argument;")
	wantNote(t, r, "_g1", migrate.Approximated, "the flow carries this under the guard [false]")
	wantClean(t, "guarded_self_flow.sysml", r)
}

const mergedGuardedSelfFlow = `
    <packagedElement xmi:type="uml:Class" xmi:id="_acct" name="Acct" classifierBehavior="_run">
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_produce" name="Produce">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_made" name="made" direction="out" type="_acct"/>
        <node xmi:type="uml:ReadSelfAction" xmi:id="_prs" name="self read">
          <result xmi:type="uml:OutputPin" xmi:id="_prsOut" name="result" type="_acct"/>
        </node>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_madeNode" name="made" parameter="_made" type="_acct"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_pf" source="_prsOut" target="_madeNode"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:ReadSelfAction" xmi:id="_rs" name="me">
          <result xmi:type="uml:OutputPin" xmi:id="_rsOut" name="result" type="_acct"/>
        </node>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_mk" name="make" behavior="_produce">
          <result xmi:type="uml:OutputPin" xmi:id="_mkOut" name="made" type="_acct"/>
        </node>
        <node xmi:type="uml:MergeNode" xmi:id="_merge" name="either"/>
        <node xmi:type="uml:OpaqueAction" xmi:id="_op" name="use">
          <inputValue xmi:type="uml:InputPin" xmi:id="_opIn" name="argument" type="_acct"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_m1" source="_rsOut" target="_merge"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_m2" source="_mkOut" target="_merge"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_g1" source="_merge" target="_opIn">
          <guard xmi:type="uml:LiteralBoolean" xmi:id="_g1Guard" value="false"/>
        </edge>
      </ownedBehavior>
    </packagedElement>`

// A guarded flow carrying a self read and another value still reports the guard
// the other value's flow is written without.
func TestMergedGuardedReadSelfFlowReportsTheUnwrittenGuard(t *testing.T) {
	r := migrateDocument(t, mergedGuardedSelfFlow,
		`<sysml:Block xmi:id="_s9" base_Class="_acct"/>`)
	wantNoLine(t, r.Notation, "flow me.result to 'use'.argument;")
	wantLine(t, r.Notation, "flow make.made to 'use'.argument;")
	var notes []string
	for _, e := range entriesFor(r, "_g1") {
		notes = append(notes, e.Note)
	}
	got := strings.Join(notes, "; ")
	for _, want := range []string{"the flow carries this under the guard [false]", "the guard [false] on an object flow is not written"} {
		if !strings.Contains(got, want) {
			t.Errorf("notes for _g1 = %q, want one noting %q", got, want)
		}
	}
	wantClean(t, "merged_guarded_self_flow.sysml", r)
}
