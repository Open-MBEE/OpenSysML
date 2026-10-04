package migrate_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// valueActions is an activity whose value actions declare their result pins
// with v1 multiplicities, one of them fed to nothing and started by no edge.
const valueActions = `
    <packagedElement xmi:type="uml:Activity" xmi:id="_run" name="Run">
      <node xmi:type="uml:InitialNode" xmi:id="_init"/>
      <node xmi:type="uml:ValueSpecificationAction" xmi:id="_one" name="one">
        <value xmi:type="uml:LiteralInteger" xmi:id="_oneV" value="1"/>
        <result xmi:type="uml:OutputPin" xmi:id="_oneOut" name="result">` + integerHref + `</result>
      </node>
      <node xmi:type="uml:ValueSpecificationAction" xmi:id="_maybe" name="maybe">
        <value xmi:type="uml:LiteralInteger" xmi:id="_maybeV" value="2"/>
        <result xmi:type="uml:OutputPin" xmi:id="_maybeOut" name="result">` + integerHref + `
          <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_maybeLo"/>
          <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_maybeUp" value="1"/>
        </result>
      </node>
      <node xmi:type="uml:ValueSpecificationAction" xmi:id="_many" name="many">
        <value xmi:type="uml:LiteralInteger" xmi:id="_manyV" value="3"/>
        <result xmi:type="uml:OutputPin" xmi:id="_manyOut" name="result" isOrdered="true">` + integerHref + `
          <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_manyLo" value="1"/>
          <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_manyUp" value="*"/>
        </result>
      </node>
      <node xmi:type="uml:ValueSpecificationAction" xmi:id="_typed" name="typed">
        <value xmi:type="uml:LiteralString" xmi:id="_typedV" value="2"/>
        <result xmi:type="uml:OutputPin" xmi:id="_typedOut" name="result">` + integerHref + `</result>
      </node>
      <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_one"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_one" target="_maybe"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_maybe" target="_typed"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_e4" source="_typed" target="_final"/>
    </packagedElement>`

// A value action's result pin keeps the multiplicity and ordering v1 declares,
// and an action no edge leads to is started by the activity, as v1 starts it.
func TestValueActionResultKeepsThePinsMultiplicity(t *testing.T) {
	r := migrateDocument(t, valueActions, "")
	for _, line := range []string{
		"out result : ScalarValues::Integer[1] = 1;",
		"out result : ScalarValues::Integer[0..1] = 2;",
		"out result : ScalarValues::Integer[1..*] ordered = 3;",
		"first 'fork' then many;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_one", migrate.Mapped, "")
	wantNote(t, r, "_maybe", migrate.Mapped, "")
	wantNote(t, r, "_maybeOut", migrate.Mapped, "")
	wantNote(t, r, "_manyOut", migrate.Mapped, "")
	wantNote(t, r, "_many", migrate.Mapped, "no edge leads to the node, so it starts with the activity")
	wantClean(t, "value_actions.sysml", r)
}

// A typed-in string is written as the scalar the result pin holds, since the
// string itself would not conform to the pin's type.
func TestValueActionLiteralTakesTheResultsScalarType(t *testing.T) {
	r := migrateDocument(t, valueActions, "")
	wantLine(t, r.Notation, "out result : ScalarValues::Integer[1] = 2;")
	wantNote(t, r, "_typed", migrate.Approximated, `the string "2" is written as the Integer the feature holds`)
	wantClean(t, "value_actions.sysml", r)
}

// countedValueActions declares result pins that cannot hold the one value the
// action gives: one needs two values, the other admits none.
const countedValueActions = `
    <packagedElement xmi:type="uml:Activity" xmi:id="_run" name="Run">
      <node xmi:type="uml:InitialNode" xmi:id="_init"/>
      <node xmi:type="uml:ValueSpecificationAction" xmi:id="_pair" name="pair">
        <value xmi:type="uml:LiteralInteger" xmi:id="_pairV" value="5"/>
        <result xmi:type="uml:OutputPin" xmi:id="_pairOut" name="result">` + integerHref + `
          <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_pairLo" value="2"/>
          <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_pairUp" value="2"/>
        </result>
      </node>
      <node xmi:type="uml:ValueSpecificationAction" xmi:id="_none" name="none">
        <value xmi:type="uml:LiteralInteger" xmi:id="_noneV" value="6"/>
        <result xmi:type="uml:OutputPin" xmi:id="_noneOut" name="result">` + integerHref + `
          <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_noneLo"/>
          <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_noneUp" value="0"/>
        </result>
      </node>
      <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_pair"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_pair" target="_none"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_none" target="_final"/>
    </packagedElement>`

// A result pin that cannot hold the action's one value leaves it unbound, since
// binding one value to it would fail validation.
func TestValueActionResultCountingOtherThanOneIsNotBound(t *testing.T) {
	r := migrateDocument(t, countedValueActions, "")
	wantNoStatement(t, r.Notation, "out result : ScalarValues::Integer[2] = 5;")
	wantNoStatement(t, r.Notation, "out result : ScalarValues::Integer[0] = 6;")
	wantNote(t, r, "_pair", migrate.Approximated,
		"the value 5 is not written: the result holds 2 values and the action gives one")
	wantNote(t, r, "_none", migrate.Approximated,
		"the value 6 is not written: the result holds 0 values and the action gives one")
	wantClean(t, "counted_value_actions.sysml", r)
}
