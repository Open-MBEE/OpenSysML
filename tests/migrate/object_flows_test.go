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
		"first produce if on then consume;",
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
