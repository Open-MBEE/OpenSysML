package migrate_test

import (
	"strings"
	"testing"
)

// nudgeNetwork is motorNetwork's net with an activity Pick returning m2, and an
// activity Nudge that hands what Pick returns to Spin's target pin through an
// object flow guarded by false, while a control flow from the same fork enters
// the call directly.
var nudgeNetwork = strings.Replace(motorNetwork, `<ownedBehavior xmi:type="uml:Activity" xmi:id="_kick" name="Kick">`,
	`<ownedBehavior xmi:type="uml:Activity" xmi:id="_pick" name="Pick">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_pickOut" name="motor" direction="out" type="_motor"/>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_pickOutN" name="motor" parameter="_pickOut"/>
        <node xmi:type="uml:InitialNode" xmi:id="_pki"/>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_pkread" name="read m2" structuralFeature="_m2">
          <result xmi:type="uml:OutputPin" xmi:id="_pkreadOut" name="result"/>
        </node>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pke1" source="_pki" target="_pkread"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_pkof" source="_pkreadOut" target="_pickOutN"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_nudge" name="Nudge">
        <node xmi:type="uml:InitialNode" xmi:id="_ni"/>
        <node xmi:type="uml:ForkNode" xmi:id="_nfork"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_npick" name="pick" behavior="_pick">
          <result xmi:type="uml:OutputPin" xmi:id="_npickOut" name="motor" type="_motor"/>
        </node>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_nten" name="ten">
          <value xmi:type="uml:LiteralInteger" xmi:id="_ntenV" value="10"/>
          <result xmi:type="uml:OutputPin" xmi:id="_ntenOut" name="result"/>
        </node>
        <node xmi:type="uml:CallOperationAction" xmi:id="_ncall" name="spin" operation="_spin">
          <argument xmi:type="uml:InputPin" xmi:id="_ncallTo" name="to"/>
          <target xmi:type="uml:InputPin" xmi:id="_ncallTgt" name="target"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_nf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ne1" source="_ni" target="_nfork"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ne2" source="_nfork" target="_npick"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ne3" source="_nfork" target="_nten"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ne4" source="_nfork" target="_ncall"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_nof1" source="_ntenOut" target="_ncallTo"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_nof2" source="_npickOut" target="_ncallTgt">
          <guard xmi:type="uml:LiteralBoolean" xmi:id="_nguard" value="false"/>
        </edge>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ne5" source="_ncall" target="_nf"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_kick" name="Kick">`, 1)

// guardedMail is mailNetwork with the flow into post's request pin guarded by
// false, and a control flow from 'read b' entering post directly.
var guardedMail = strings.Replace(mailNetwork, `<edge xmi:type="uml:ObjectFlow" xmi:id="_pof1" source="_readMsgOut" target="_sendObjReq"/>`,
	`<edge xmi:type="uml:ObjectFlow" xmi:id="_pof1" source="_readMsgOut" target="_sendObjReq">
          <guard xmi:type="uml:LiteralBoolean" xmi:id="_pguard" value="false"/>
        </edge>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe4" source="_readB2" target="_sendObj"/>`, 1)

// A guarded flow into a call's target pin is the target of its guarded succession,
// so a false guard delivers no object: the call, entered also by control, never
// runs on the object its guard rejected.
func TestGuardedFlowIntoACallTargetDeliversNoRejectedObject(t *testing.T) {
	r := migrateDocument(t, nudgeNetwork, motorNetworkApplications)
	for _, line := range []string{
		"action spin : Motor::Spin { in ref :>> context = target; in 'to'[1]; in target : Motor[1]; }",
		"first pick if false then 'pick.motor to spin.target';",
		"succession flow 'pick.motor to spin.target' of Motor from pick.motor to spin.target;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantClean(t, "t.sysml", r)
	s := session(t, r)
	meta(t, s, "%instantiate Net")
	meta(t, s, "%action Net::nudge #1")
	meta(t, s, "%continue")
	meta(t, s, "%advance 0")
	if out := meta(t, s, "%eval in #1 : m2.rpm"); !strings.Contains(out, "= 0") {
		t.Errorf("the call ran on the object its guard rejected: %s", out)
	}
}

// A guarded flow into a send object action's request pin is gated the same way: a
// false guard sends nothing, though control enters the send directly.
func TestGuardedFlowIntoASendObjectRequestSendsNoRejectedObject(t *testing.T) {
	r := migrateDocument(t, guardedMail, pingNetworkApplications)
	for _, line := range []string{
		"first 'read msg' if false then 'read msg.result to post.request';",
		"succession flow 'read msg.result to post.request' from 'read msg'.result to post.request;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantClean(t, "t.sysml", r)
	s := session(t, r)
	meta(t, s, "%instantiate Net")
	meta(t, s, "%action Net::post #1")
	meta(t, s, "%continue")
	meta(t, s, "%advance 0")
	if out := meta(t, s, "%eval in #1 : b.hits"); !strings.Contains(out, "= 0") {
		t.Errorf("the send delivered the object its guard rejected: %s", out)
	}
}
