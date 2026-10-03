package migrate_test

import (
	"strings"
	"testing"
)

// Each object flow of the object_flows fixture is written as a succession
// flow of its payload type, through the object features of the fork, join and
// merge nodes it crosses, except the one into a streaming parameter; every
// activity runs and delivers the values its flows carry.
func TestObjectFlowsAreSuccessionFlowsThroughControlNodeObjects(t *testing.T) {
	r := migrateFixtureFile(t, "object_flows")
	notation := string(r.Notation)
	for _, want := range []string{
		"succession flow of ScalarValues::Real from produce.y to consume.v;",
		"succession flow of ScalarValues::Real from produce.y to left.v;\n        succession flow of ScalarValues::Real from produce.y to right.v;",
		"out ref outputObject2 : ScalarValues::Real = inputObject1;",
		"out ref outputObject1 : ScalarValues::Real nonunique = (inputObject1, inputObject2);",
		"succession flow of ScalarValues::Real from pick.outputObject1 to consume.v;",
		"first produce if on then 'produce.y to consume.v';\n" +
			"        succession flow 'produce.y to consume.v' of ScalarValues::Real from produce.y to consume.v;",
		"flow produce.y to feed.v;",
	} {
		if !strings.Contains(notation, want) {
			t.Errorf("notation lacks %q:\n%s", want, notation)
		}
	}
	if strings.Contains(notation, "if true") {
		t.Errorf("a guard of literal true is written:\n%s", notation)
	}
	s := session(t, r)
	meta(t, s, "%instantiate Pipeline")
	for action, want := range map[string]map[string]string{
		"Pipeline::relay":  {"consume.v": "2.5"},
		"Pipeline::gate":   {"consume.v": "4.0"},
		"Pipeline::spread": {"left.v": "3.0", "right.v": "3.0"},
		"Pipeline::fan":    {"left.v": "5.0", "right.v": "5.0"},
		"Pipeline::hold":   {"consume.v": "7.0"},
		"Pipeline::either": {"consume.v": "9.0"},
		"Pipeline::count":  {"step.x": "2", "step.y": "3"},
		"Pipeline::Pair":   {"keep.v": "[1.5, 8.5]"},
		"Pipeline::Stream": {"feed.v": "6.0"},
	} {
		t.Run(action, func(t *testing.T) {
			wantValues(t, runValues(t, s, action, "#1"), want)
		})
	}
}

// stampedFlow is an activity whose object flow leaves an action a duration
// observation reads the clock at the end of, up to the start of the one it feeds.
const stampedFlow = `
    <packagedElement xmi:type="uml:Class" xmi:id="_line" name="Line">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_la" name="a"><type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/><defaultValue xmi:type="uml:LiteralReal" xmi:id="_la0" value="0.0"/></ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_hand" name="Hand">
        <node xmi:type="uml:InitialNode" xmi:id="_hi"/>
        <node xmi:type="uml:OpaqueAction" xmi:id="_hp" name="produce">
          <outputValue xmi:type="uml:OutputPin" xmi:id="_hpy" name="y"><type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/></outputValue>
          <language>JavaScript</language><body>y = 2.5;</body>
        </node>
        <node xmi:type="uml:OpaqueAction" xmi:id="_hc" name="consume">
          <inputValue xmi:type="uml:InputPin" xmi:id="_hcv" name="v"><type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/></inputValue>
          <language>JavaScript</language><body>a = v;</body>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_hf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_he1" source="_hi" target="_hp"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_ho1" source="_hpy" target="_hcv"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_he2" source="_hc" target="_hf"/>
        <observation xmi:type="uml:DurationObservation" xmi:id="_gap" name="Gap" firstEvent="false true">
          <event xmi:idref="_hp"/>
          <event xmi:idref="_hc"/>
        </observation>
      </ownedBehavior>
    </packagedElement>`

// An object flow out of an action whose end a duration observation stamps leads
// on through its stamps: they read the clock before the target starts, and
// the value still reaches it.
func TestObjectFlowFromStampedActionFollowsTheStamp(t *testing.T) {
	r := migrateDocument(t, stampedFlow, `
  <sysml:Block xmi:id="_lineB" base_Class="_line"/>`)
	for _, line := range []string{
		"first produce then stamp;",
		"first stamp then stamp2;",
		"first stamp2 then consume;",
		"flow produce.y to consume.v;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "succession flow") {
		t.Errorf("the flow bypasses the stamp:\n%s", r.Notation)
	}
	wantClean(t, "t.sysml", r)
	s := session(t, r)
	meta(t, s, "%instantiate Line")
	wantValues(t, runValues(t, s, "Line::hand", "Line"), map[string]string{"consume.v": "2.5"})
}

// rejectedFlow is an activity whose guarded object flow into consume is false,
// while another action's flow also reaches the same pin.
const rejectedFlow = `
    <packagedElement xmi:type="uml:Class" xmi:id="_line" name="Line">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_la" name="a"><type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/><defaultValue xmi:type="uml:LiteralReal" xmi:id="_la0" value="0.0"/></ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ln" name="n"><type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/><defaultValue xmi:type="uml:LiteralInteger" xmi:id="_ln0" value="0"/></ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_lo" name="on"><type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Boolean"/><defaultValue xmi:type="uml:LiteralBoolean" xmi:id="_lo0" value="false"/></ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_pick" name="Pick">
        <node xmi:type="uml:InitialNode" xmi:id="_pi"/>
        <node xmi:type="uml:ForkNode" xmi:id="_pk" name="both"/>
        <node xmi:type="uml:OpaqueAction" xmi:id="_pr" name="rejected">
          <outputValue xmi:type="uml:OutputPin" xmi:id="_pry" name="y"><type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/></outputValue>
          <language>JavaScript</language><body>y = 1.0;</body>
        </node>
        <node xmi:type="uml:OpaqueAction" xmi:id="_pa" name="accepted">
          <outputValue xmi:type="uml:OutputPin" xmi:id="_pay" name="y"><type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/></outputValue>
          <language>JavaScript</language><body>y = 2.0;</body>
        </node>
        <node xmi:type="uml:OpaqueAction" xmi:id="_pc" name="consume">
          <inputValue xmi:type="uml:InputPin" xmi:id="_pcv" name="v"><type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/></inputValue>
          <language>JavaScript</language><body>a = v; n = n + 1;</body>
        </node>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe1" source="_pi" target="_pk"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe2" source="_pk" target="_pr"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe3" source="_pk" target="_pa"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_po1" source="_pry" target="_pcv">
          <guard xmi:type="uml:OpaqueExpression" xmi:id="_pg"><language>JavaScript</language><body>on</body></guard>
        </edge>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_po2" source="_pay" target="_pcv"/>
      </ownedBehavior>
    </packagedElement>`

// A guarded object flow is the target of its guarded succession, so a false guard
// moves no value: consume, reached also from accepted, runs once on accepted's value.
func TestGuardedObjectFlowMovesNoValueWhenItsGuardIsFalse(t *testing.T) {
	r := migrateDocument(t, rejectedFlow, `
  <sysml:Block xmi:id="_lineB" base_Class="_line"/>`)
	for _, line := range []string{
		"first rejected if on then 'rejected.y to consume.v';",
		"succession flow 'rejected.y to consume.v' of ScalarValues::Real from rejected.y to consume.v;",
		"succession flow of ScalarValues::Real from accepted.y to consume.v;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "first rejected if on then consume;") {
		t.Errorf("the guarded succession bypasses the flow:\n%s", r.Notation)
	}
	wantClean(t, "t.sysml", r)
	s := session(t, r)
	meta(t, s, "%instantiate Line")
	wantValues(t, runValues(t, s, "Line::pick", "#1"), map[string]string{"consume.v": "2.0"})
	if out := meta(t, s, "%eval in #1 : n"); !strings.Contains(out, "= 1") {
		t.Errorf("consume ran other than once: %s", out)
	}
	if out := meta(t, s, "%eval in #1 : a"); !strings.Contains(out, "= 2.0") {
		t.Errorf("consume took the rejected value: %s", out)
	}
}
