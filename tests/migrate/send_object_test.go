package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// mailNetwork is pingNetwork's net holding a Ping of its own, msg. Post sends
// the Ping msg holds to the node b.
var mailNetwork = strings.Replace(pingNetwork, `<ownedAttribute xmi:type="uml:Property" xmi:id="_b" name="b" type="_node" aggregation="composite"/>`,
	`<ownedAttribute xmi:type="uml:Property" xmi:id="_b" name="b" type="_node" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_msg" name="msg" type="_ping" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_post" name="Post">
        <node xmi:type="uml:InitialNode" xmi:id="_pinit"/>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_readMsg" name="read msg" structuralFeature="_msg">
          <result xmi:type="uml:OutputPin" xmi:id="_readMsgOut" name="result"/>
        </node>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_readB2" name="read b" structuralFeature="_b">
          <result xmi:type="uml:OutputPin" xmi:id="_readB2Out" name="result"/>
        </node>
        <node xmi:type="uml:SendObjectAction" xmi:id="_sendObj" name="post">
          <request xmi:type="uml:InputPin" xmi:id="_sendObjReq" name="request"/>
          <target xmi:type="uml:InputPin" xmi:id="_sendObjTgt" name="target"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_pfinal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe1" source="_pinit" target="_readMsg"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe2" source="_readMsg" target="_readB2"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_pof1" source="_readMsgOut" target="_sendObjReq"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_pof2" source="_readB2Out" target="_sendObjTgt"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe3" source="_sendObj" target="_pfinal"/>
      </ownedBehavior>`, 1)

// A send object action is written `send request to target`: the object its
// request pin holds, sent to the object its target pin holds. The result runs:
// the Ping msg holds reaches the node b, and no other.
func TestSendObjectDeliversTheRequestToItsTarget(t *testing.T) {
	r := migrateDocument(t, mailNetwork, pingNetworkApplications)
	for _, line := range []string{
		"item msg : Ping;",
		"in request[1];",
		"send request to b;",
		"flow 'read msg'.result to post.request;",
		"flow 'read b'.result to post.target;",
	} {
		wantLine(t, r.Notation, line)
	}
	for _, id := range []string{"_sendObjReq", "_sendObjTgt", "_pof1", "_pof2"} {
		wantNote(t, r, id, migrate.Mapped, "")
	}
	for _, e := range entriesFor(r, "_sendObj") {
		if e.Verdict == migrate.Unmapped || strings.Contains(e.Note, "sent to the sender") {
			t.Errorf("the send was not written to its target: %+v", e)
		}
	}

	s := session(t, r)
	meta(t, s, "%instantiate Net")
	meta(t, s, "%action Net::post #1")
	meta(t, s, "%continue")
	meta(t, s, "%advance 0")
	if out := meta(t, s, "%eval in #1 : b.hits"); !strings.Contains(out, "= 1") {
		t.Errorf("the node the target pin held did not take the ping: %s", out)
	}
	if out := meta(t, s, "%eval in #1 : a.hits"); !strings.Contains(out, "= 0") {
		t.Errorf("a node the target pin did not hold took the ping: %s", out)
	}
}

// A send object action with no request pin has nothing to send: it is a
// placeholder keeping its other pins, reported unmapped.
func TestSendObjectWithoutRequestIsAPlaceholder(t *testing.T) {
	doc := strings.Replace(mailNetwork, `<request xmi:type="uml:InputPin" xmi:id="_sendObjReq" name="request"/>`, "", 1)
	doc = strings.Replace(doc, `<edge xmi:type="uml:ObjectFlow" xmi:id="_pof1" source="_readMsgOut" target="_sendObjReq"/>`, "", 1)
	r := migrateDocument(t, doc, pingNetworkApplications)
	wantNote(t, r, "_sendObj", migrate.Unmapped, "the action has no request pin, so it has nothing to send")
	if strings.Contains(string(r.Notation), "send request") {
		t.Errorf("a send without a request pin was written:\n%s", r.Notation)
	}
	session(t, r)
}

// pingSelfFedRequest is pingSelfFedTarget with the send a send object action:
// it sends the Ping the node's msg holds to the object read self gives.
var pingSelfFedRequest = strings.NewReplacer(
	`<ownedBehavior xmi:type="uml:Activity" xmi:id="_notify" name="Notify">`,
	`<ownedAttribute xmi:type="uml:Property" xmi:id="_msg" name="msg" type="_ping" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_notify" name="Notify">
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_readMsg" name="read msg" structuralFeature="_msg">
          <result xmi:type="uml:OutputPin" xmi:id="_readMsgOut" name="result"/>
        </node>`,
	`<node xmi:type="uml:SendSignalAction" xmi:id="_send" name="send" signal="_ping">`,
	`<node xmi:type="uml:SendObjectAction" xmi:id="_send" name="send">
          <request xmi:type="uml:InputPin" xmi:id="_request" name="request"/>`,
	`<edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_readSelf"/>`,
	`<edge xmi:type="uml:ControlFlow" xmi:id="_e0" source="_init" target="_readMsg"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_readMsg" target="_readSelf"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of0" source="_readMsgOut" target="_request"/>`,
).Replace(pingSelfFedTarget)

// A send object action whose target pin read self fills binds the pin to the
// sender and sends to it: the node takes the Ping it sent itself.
func TestSendObjectTargetFedByReadSelfIsBoundToTheSender(t *testing.T) {
	r := migrateDocument(t, pingSelfFedRequest, pingSelfFedTargetApplications)
	wantLine(t, r.Notation, "in target : Node[1] = this;")
	wantLine(t, r.Notation, "send request to target;")
	if es := entriesFor(r, "_send"); len(es) != 1 || es[0].Verdict != migrate.Mapped || es[0].Note != "" {
		t.Errorf("entries for _send = %+v, want one mapped entry without a note", es)
	}
	if diags := errors(t, "self-fed-send-object.sysml", r.Notation); len(diags) != 0 {
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
