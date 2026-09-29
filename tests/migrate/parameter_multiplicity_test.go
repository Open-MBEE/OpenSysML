package migrate_test

import (
	"testing"
)

// A call's parameter redeclarations keep the multiplicity the v1 callee
// declares: an optional input stays [0..1] rather than becoming required, and
// one writing none still takes the [1] a v1 parameter means.
const redeclaredParameters = `
    <packagedElement xmi:type="uml:Class" xmi:id="_plant" name="Plant">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_level" name="level">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_level0" value="0.0"/>
      </ownedAttribute>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_adjust" name="Adjust" method="_adjusting">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_opX" name="x" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
          <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_opXl" value="0"/>
          <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_opXu" value="1"/>
        </ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_opY" name="y" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
      </ownedOperation>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_adjusting" name="Adjusting" specification="_adjust">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_mX" name="x" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
          <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_mXl" value="0"/>
          <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_mXu" value="1"/>
        </ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_mY" name="y" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_mXNode" name="x" parameter="_mX"/>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_setLevel" name="set level" structuralFeature="_level" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_setValue" name="value"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_mInput" source="_mXNode" target="_setValue"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_drive" name="Drive">
        <node xmi:type="uml:InitialNode" xmi:id="_initial"/>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_one" name="one">
          <value xmi:type="uml:LiteralReal" xmi:id="_oneV" value="1.0"/>
          <result xmi:type="uml:OutputPin" xmi:id="_oneOut" name="result"/>
        </node>
        <node xmi:type="uml:CallOperationAction" xmi:id="_call" name="adjust" operation="_adjust">
          <argument xmi:type="uml:InputPin" xmi:id="_argX" name="x"/>
          <argument xmi:type="uml:InputPin" xmi:id="_argY" name="y"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_done"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_first" source="_initial" target="_one"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_start" source="_one" target="_call"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_feedY" source="_oneOut" target="_argY"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_finish" source="_call" target="_done"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Activity" xmi:id="_reach" name="Reach">
      <node xmi:type="uml:InitialNode" xmi:id="_reachInitial"/>
      <node xmi:type="uml:ValueSpecificationAction" xmi:id="_reachTwo" name="two">
        <value xmi:type="uml:LiteralReal" xmi:id="_reachTwoV" value="2.0"/>
        <result xmi:type="uml:OutputPin" xmi:id="_reachTwoOut" name="result"/>
      </node>
      <node xmi:type="uml:CallOperationAction" xmi:id="_reachCall" name="adjust" operation="_adjust">
        <argument xmi:type="uml:InputPin" xmi:id="_reachArgX" name="x"/>
        <argument xmi:type="uml:InputPin" xmi:id="_reachArgY" name="y"/>
      </node>
      <node xmi:type="uml:ActivityFinalNode" xmi:id="_reachFinal"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_reachFirst" source="_reachInitial" target="_reachTwo"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_reachStart" source="_reachTwo" target="_reachCall"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_reachFeedX" source="_reachTwoOut" target="_reachArgX"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_reachFeedY" source="_reachTwoOut" target="_reachArgY"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_reachFinish" source="_reachCall" target="_reachFinal"/>
    </packagedElement>`

const redeclaredParametersApps = `
  <sysml:Block xmi:id="_s1" base_Class="_plant"/>`

func TestRedeclaredParametersKeepTheirDeclaredMultiplicity(t *testing.T) {
	r := migrateDocument(t, redeclaredParameters, redeclaredParametersApps)
	for _, line := range []string{
		"in x : ScalarValues::Real[0..1];",
		"in y : ScalarValues::Real[1];",
		"action adjust : Adjust { in ref :>> context = Drive::context; in x[0..1]; in y[1]; }",
	} {
		wantLine(t, r.Notation, line)
	}
}

// A reception binds the method's in parameters to the signal's attributes of the
// same name: a plural attribute fitting a plural parameter binds at [0..*], not
// the [1] a single value would take.
const pluralReception = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_readings" name="Readings">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_sValues" name="values">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_sVl" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_sVu" value="*"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_meter" name="Meter">
      <ownedReception xmi:type="uml:Reception" xmi:id="_rcv" name="OnReadings" signal="_readings" method="_collecting"/>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_collect" name="Collect" method="_collecting">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_opValues" name="values" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
          <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_opVl" value="0"/>
          <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_opVu" value="*"/>
        </ownedParameter>
      </ownedOperation>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_collecting" name="Collecting" specification="_collect">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_mValues" name="values" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
          <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_mVl" value="0"/>
          <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_mVu" value="*"/>
        </ownedParameter>
      </ownedBehavior>
    </packagedElement>`

const pluralReceptionApps = `
  <sysml:Block xmi:id="_s1" base_Class="_meter"/>`

func TestReceptionBindingKeepsAPluralParametersMultiplicity(t *testing.T) {
	r := migrateDocument(t, pluralReception, pluralReceptionApps)
	for _, line := range []string{
		"in values : ScalarValues::Real[0..*];",
		"{ in values[0..*] = ",
	} {
		wantLine(t, r.Notation, line)
	}
}

// An ordered flow property keeps its order modifier after the multiplicity the
// v1 property declares: a 1..1 directed property is [1] ordered, a [0..*] one
// is [0..*] ordered.
const directedOrdered = `
    <packagedElement xmi:type="uml:Class" xmi:id="_intf" name="Intf">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_cmd" name="cmd" isOrdered="true">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_args" name="args" isOrdered="true">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_argsl" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_argsu" value="*"/>
      </ownedAttribute>
    </packagedElement>`

const directedOrderedApps = `
  <sysml:InterfaceBlock xmi:id="_s1" base_Class="_intf"/>
  <sysml:FlowProperty xmi:id="_s2" base_Property="_cmd" direction="in"/>
  <sysml:FlowProperty xmi:id="_s3" base_Property="_args" direction="in"/>`

func TestDirectedOrderedPropertyKeepsItsDeclaredMultiplicity(t *testing.T) {
	r := migrateDocument(t, directedOrdered, directedOrderedApps)
	for _, line := range []string{
		"in attribute cmd : ScalarValues::Real[1] ordered;",
		"in attribute args : ScalarValues::Real[0..*] ordered;",
	} {
		wantLine(t, r.Notation, line)
	}
}
