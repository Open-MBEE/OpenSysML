package migrate_test

import (
	"testing"
)

// A block whose reception's method takes an optional and a plural parameter,
// each bound to a signal attribute of the same shape, and whose constraint
// block declares an ordered input with v1's default 1..1 multiplicity.
const parameterShapesModel = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_sig" name="SetLevels">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_sigValues" name="values" isOrdered="true">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_sigValuesLo" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_sigValuesHi" value="*"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_sigSlack" name="slack">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_sigSlackLo" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_sigSlackHi" value="1"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_heater" name="Heater">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_level" name="level">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_level0" value="0.0"/>
      </ownedAttribute>
      <ownedReception xmi:type="uml:Reception" xmi:id="_rcv" name="SetLevels" signal="_sig" method="_leveling"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_leveling" name="Leveling" specification="_rcv">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_lpValues" name="values" direction="in" isOrdered="true">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
          <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_lpValuesLo" value="0"/>
          <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_lpValuesHi" value="*"/>
        </ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_lpSlack" name="slack" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
          <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_lpSlackLo" value="0"/>
          <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_lpSlackHi" value="1"/>
        </ownedParameter>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_lpnValues" name="values" parameter="_lpValues"/>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_lset" name="set level" structuralFeature="_level" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_lsetVal" name="value"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_lof" source="_lpnValues" target="_lsetVal"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_within" name="Within">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_wx" name="x" isOrdered="true">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_wmax" name="max">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_wrule" constrainedElement="_within">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_wspec">
          <body>x &lt;= max</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
    </packagedElement>
`

const parameterShapesApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_heater"/>
  <sysml:ConstraintBlock xmi:id="_s2" base_Class="_within"/>
`

// A reception's binding redeclares each method parameter at the multiplicity
// the method declares, so an empty or plural payload the signal admits is
// bound rather than refused by a forced [1].
func TestReceptionBindingsKeepTheMethodsParameterMultiplicity(t *testing.T) {
	r := migrateDocument(t, parameterShapesModel, parameterShapesApplications)
	wantLine(t, r.Notation, "in values : ScalarValues::Real[0..*] ordered;")
	wantLine(t, r.Notation, "in slack : ScalarValues::Real[0..1];")
	wantLine(t, r.Notation, "perform action run ::> leveling { in values[0..*] ordered = receive.setLevels.values; in slack[0..1] = receive.setLevels.slack; }")
	wantNoLine(t, r.Notation, "in values[1]")
	wantNoLine(t, r.Notation, "in slack[1]")
	session(t, r)
}

// A constraint input ordered by v1 keeps its single value: the ordering does
// not turn the default 1..1 into a bare, and so plural, v2 parameter.
func TestOrderedSingleInputsStaySingle(t *testing.T) {
	r := migrateDocument(t, parameterShapesModel, parameterShapesApplications)
	wantLine(t, r.Notation, "in attribute x : ScalarValues::Real[1] ordered;")
	wantLine(t, r.Notation, "in attribute max : ScalarValues::Real[1];")
	session(t, r)
}

// A call's parameter redeclarations around its context binding carry the
// callee's multiplicity, so an optional input stays optional.
func TestContextBodiesKeepTheCalleesParameterMultiplicity(t *testing.T) {
	r := migrateFixtureFile(t, "operation_context_out")
	wantLine(t, r.Notation, "in x : ScalarValues::Real[0..1];")
	wantLine(t, r.Notation, "action adjust : Adjust { in ref :>> context = Drive::context; in x[0..1]; out result[1]; }")
	session(t, r)
}
