package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// pingNetwork is a net of two nodes whose machines count the pings they
// accept. Notify sends a Ping to whatever node its parameter is handed; Kick
// reads the part b and calls Notify with it.
const pingNetwork = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_ping" name="Ping"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_pingEv" signal="_ping"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_node" name="Node" classifierBehavior="_sm">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_hits" name="hits">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_hits0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Listening">
        <region xmi:type="uml:Region" xmi:id="_r0" name="main">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init0"/>
          <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle"/>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init0" target="_idle"/>
          <transition xmi:type="uml:Transition" xmi:id="_t1" name="hit" source="_idle" target="_idle">
            <trigger xmi:type="uml:Trigger" xmi:id="_tr1" event="_pingEv"/>
            <effect xmi:type="uml:OpaqueBehavior" xmi:id="_eff">
              <language>JavaScript</language>
              <body>hits = hits + 1;</body>
            </effect>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_net" name="Net">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_a" name="a" type="_node" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_b" name="b" type="_node" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_notify" name="Notify">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_rcpt" name="recipient" direction="in" type="_node"/>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_rcptN" name="recipient" parameter="_rcpt"/>
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:SendSignalAction" xmi:id="_send" name="ping" signal="_ping">
          <target xmi:type="uml:InputPin" xmi:id="_sendTgt" name="target"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_send"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of1" source="_rcptN" target="_sendTgt"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_send" target="_final"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_kick" name="Kick">
        <node xmi:type="uml:InitialNode" xmi:id="_kinit"/>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_readB" name="read b" structuralFeature="_b">
          <result xmi:type="uml:OutputPin" xmi:id="_readBOut" name="result"/>
        </node>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_call" name="notify" behavior="_notify">
          <argument xmi:type="uml:InputPin" xmi:id="_callIn" name="recipient"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_kfinal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ke1" source="_kinit" target="_readB"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_kof1" source="_readBOut" target="_callIn"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ke2" source="_call" target="_kfinal"/>
      </ownedBehavior>
    </packagedElement>`

const pingNetworkApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_node"/>
  <sysml:Block xmi:id="_s2" base_Class="_net"/>`

const pingSelfFedTarget = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_ping" name="Ping"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_pingEv" signal="_ping"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_node" name="Node">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_hits" name="hits">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_hits0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_notify" name="Notify">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:ReadSelfAction" xmi:id="_readSelf" name="read self">
          <result xmi:type="uml:OutputPin" xmi:id="_readSelfResult" name="result"/>
        </node>
        <node xmi:type="uml:SendSignalAction" xmi:id="_send" name="send" signal="_ping">
          <target xmi:type="uml:InputPin" xmi:id="_target" name="target" type="_node"/>
        </node>
        <node xmi:type="uml:AcceptEventAction" xmi:id="_accept" name="receive">
          <trigger xmi:type="uml:Trigger" xmi:id="_trigger" event="_pingEv"/>
        </node>
        <node xmi:type="uml:OpaqueAction" xmi:id="_mark" name="mark">
          <language>JavaScript</language>
          <body>hits = hits + 1;</body>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_readSelf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_readSelf" target="_send"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of1" source="_readSelfResult" target="_target"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_send" target="_accept"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e4" source="_accept" target="_mark"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e5" source="_mark" target="_final"/>
      </ownedBehavior>
    </packagedElement>`

const pingSelfFedTargetApplications = `<sysml:Block xmi:id="_s1" base_Class="_node"/>`

const pingUnfilledTarget = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_ping" name="Ping"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_node" name="Node">
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_notify" name="Notify">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:SendSignalAction" xmi:id="_send" name="send" signal="_ping">
          <target xmi:type="uml:InputPin" xmi:id="_target" name="target" type="_node">
            <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_targetLower" value="1"/>
          </target>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_send"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_send" target="_final"/>
      </ownedBehavior>
    </packagedElement>`

const pingUnfilledTargetApplications = `<sysml:Block xmi:id="_s1" base_Class="_node"/>`

// A send whose target pin is fed by an object flow is written as a send to the
// pin, bound to what feeds it — an activity parameter here, in turn fed by a
// structural read in the caller. The result runs: the ping reaches the node
// the parameter holds, and no other.
func TestSendTargetFedByParameterReachesTheObjectItHolds(t *testing.T) {
	r := migrateDocument(t, pingNetwork, pingNetworkApplications)
	for _, line := range []string{
		"in recipient : Node[1];",
		"in target[1];",
		"send new Ping() to target;",
		"bind ping.target = recipient;",
		"out result[1] = b;",
		"flow 'read b'.result to notify.recipient;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "to this.target") {
		t.Errorf("the target pin was written as a feature of the sender:\n%s", r.Notation)
	}
	for _, id := range []string{"_send", "_of1", "_kof1", "_readB"} {
		wantNote(t, r, id, migrate.Mapped, "")
	}
	wantNote(t, r, "_call", migrate.Mapped, "")

	s := session(t, r)
	meta(t, s, "%instantiate Net")
	meta(t, s, "%action Net::kick #1")
	meta(t, s, "%continue")
	meta(t, s, "%advance 0")
	if out := meta(t, s, "%eval in #1 : b.hits"); !strings.Contains(out, "= 1") {
		t.Errorf("the node the parameter held did not take the ping: %s", out)
	}
	if out := meta(t, s, "%eval in #1 : a.hits"); !strings.Contains(out, "= 0") {
		t.Errorf("a node the parameter did not hold took the ping: %s", out)
	}
}

func TestSendTargetFedByReadSelfIsBoundToTheSender(t *testing.T) {
	r := migrateDocument(t, pingSelfFedTarget, pingSelfFedTargetApplications)
	wantLine(t, r.Notation, "in target : Node[1] = this;")
	wantLine(t, r.Notation, "send new Ping() to target;")
	bindingAt, sendAt := strings.Index(string(r.Notation), "in target : Node[1] = this;"), strings.Index(string(r.Notation), "send new Ping() to target;")
	if bindingAt < 0 || sendAt < 0 || bindingAt > sendAt {
		t.Fatalf("the target pin binding must precede the send:\n%s", r.Notation)
	}
	if es := entriesFor(r, "_send"); len(es) != 1 || es[0].Verdict != migrate.Mapped || es[0].Note != "" {
		t.Errorf("entries for _send = %+v, want one mapped entry without a note", es)
	}
	if diags := errors(t, "self-fed-send.sysml", r.Notation); len(diags) != 0 {
		t.Fatalf("migrated notation has validation errors: %v\n%s", diags, r.Notation)
	}

	s := session(t, r)
	meta(t, s, "%instantiate Node")
	started := meta(t, s, "%action Node::notify #1")
	continued := meta(t, s, "%continue")
	if out := meta(t, s, "%eval in #1 : hits"); !strings.Contains(out, "= 1") {
		t.Errorf("the accept did not complete after the sender received the ping: %s\naction: %s\ncontinue: %s\n%s", out, started, continued, r.Notation)
	}
}

func TestSendTargetNothingFillsIsStillTheReceiver(t *testing.T) {
	r := migrateDocument(t, pingUnfilledTarget, pingUnfilledTargetApplications)
	wantLine(t, r.Notation, "send new Ping() to target;")
	if es := entriesFor(r, "_send"); len(es) != 1 || strings.Contains(es[0].Note, "sent to the sender") {
		t.Errorf("entries for _send = %+v, want no note that the signal is sent to the sender", es)
	}
	neverFires := false
	for _, e := range r.Report.Entries {
		neverFires = neverFires || strings.Contains(e.Note, "never fires")
	}
	if !neverFires {
		t.Errorf("migration report does not say the action never fires:\n%+v", r.Report.Entries)
	}
	if diags := errors(t, "unfilled-target-send.sysml", r.Notation); len(diags) != 0 {
		t.Errorf("migrated notation has validation errors: %v\n%s", diags, r.Notation)
	}
}
