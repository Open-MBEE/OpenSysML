package migrate_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// A guard spelled as a uml:Expression with symbol "else" and no operands is the
// else guard, as the OpaqueExpression and LiteralString spellings are.
const elseExprMachine = `
    <packagedElement xmi:type="uml:Class" xmi:id="_esw" name="Switch" classifierBehavior="_esm">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_eready" name="ready">` + booleanHref + `</ownedAttribute>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_esm" name="Switching">
        <region xmi:type="uml:Region" xmi:id="_ereg" name="Main">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_einit"/>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_epick" kind="choice"/>
          <subvertex xmi:type="uml:State" xmi:id="_eon" name="On"/>
          <subvertex xmi:type="uml:State" xmi:id="_eoff" name="Off"/>
          <transition xmi:type="uml:Transition" xmi:id="_et0" source="_einit" target="_epick"/>
          <transition xmi:type="uml:Transition" xmi:id="_etOn" source="_epick" target="_eon">
            <guard xmi:type="uml:Constraint" xmi:id="_egOn">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_egOns">
                <body>ready</body>
                <language>JavaScript</language>
              </specification>
            </guard>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_etOff" source="_epick" target="_eoff">
            <guard xmi:type="uml:Constraint" xmi:id="_egOff">
              <specification xmi:type="uml:Expression" xmi:id="_egOffs" symbol="else"/>
            </guard>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`

func TestElseExpressionGuardOnAChoice(t *testing.T) {
	r := migrateDocument(t, elseExprMachine, `<sysml:Block xmi:id="_es1" base_Class="_esw"/>`)
	wantLine(t, r.Notation, "transition first choice if context.ready then On;")
	wantLine(t, r.Notation, "transition first choice then Off;")
	wantNote(t, r, "_egOff", migrate.Mapped, "an else guard is written as the unguarded transition out of the choice")
}

// An activity edge's guard is the value specification itself; an Expression
// with symbol "else" there is the else branch of a decision.
const elseExprActivity = `
    <packagedElement xmi:type="uml:Class" xmi:id="_eblk" name="Pump" classifierBehavior="_eact">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_eempty" name="empty">` + booleanHref + `</ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_eact" name="Pumping">
        <node xmi:type="uml:InitialNode" xmi:id="_en0"/>
        <node xmi:type="uml:DecisionNode" xmi:id="_edec" name="pick"/>
        <node xmi:type="uml:OpaqueAction" xmi:id="_eprime" name="prime"/>
        <node xmi:type="uml:MergeNode" xmi:id="_emerge" name="merge"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ee0" source="_en0" target="_edec"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ee1" source="_edec" target="_eprime">
          <guard xmi:type="uml:OpaqueExpression" xmi:id="_eg1"><body>empty</body></guard>
        </edge>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ee2" source="_edec" target="_emerge">
          <guard xmi:type="uml:Expression" xmi:id="_eg2" symbol="else"/>
        </edge>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ee3" source="_eprime" target="_emerge"/>
      </ownedBehavior>
    </packagedElement>`

func TestElseExpressionGuardOnADecision(t *testing.T) {
	r := migrateDocument(t, elseExprActivity, `<sysml:Block xmi:id="_ea1" base_Class="_eblk"/>`)
	wantLine(t, r.Notation, "else done;")
}
