package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// bareDurations is an activity whose four steps are constrained by durations written
// four ways: a number with no unit, a string with none, an expression with none, and
// an expression with a unit.
const bareDurations = `
    <packagedElement xmi:type="uml:Class" xmi:id="_lens" name="Lens">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_settle" name="settle">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_settle0" value="250.0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_focusing" name="Focusing">
        <node xmi:type="uml:InitialNode" xmi:id="_fi"/>
        <node xmi:type="uml:OpaqueAction" xmi:id="_coarse" name="coarse"/>
        <node xmi:type="uml:OpaqueAction" xmi:id="_fine" name="fine"/>
        <node xmi:type="uml:OpaqueAction" xmi:id="_settling" name="settling"/>
        <node xmi:type="uml:OpaqueAction" xmi:id="_holding" name="holding"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_ff"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_fe1" source="_fi" target="_coarse"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_fe2" source="_coarse" target="_fine"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_fe3" source="_fine" target="_settling"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_fe4" source="_settling" target="_holding"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_fe5" source="_holding" target="_ff"/>
        <ownedRule xmi:type="uml:DurationConstraint" xmi:id="_dNum" name="coarse takes">
          <constrainedElement xmi:idref="_coarse"/>
          <specification xmi:type="uml:DurationInterval" xmi:id="_dNumI" min="_dNumD" max="_dNumD"/>
        </ownedRule>
        <ownedRule xmi:type="uml:DurationConstraint" xmi:id="_dStr" name="fine takes">
          <constrainedElement xmi:idref="_fine"/>
          <specification xmi:type="uml:DurationInterval" xmi:id="_dStrI" min="_dStrD" max="_dStrD"/>
        </ownedRule>
        <ownedRule xmi:type="uml:DurationConstraint" xmi:id="_dExpr" name="settling takes">
          <constrainedElement xmi:idref="_settling"/>
          <specification xmi:type="uml:DurationInterval" xmi:id="_dExprI" min="_dExprD" max="_dExprD"/>
        </ownedRule>
        <ownedRule xmi:type="uml:DurationConstraint" xmi:id="_dUnit" name="holding takes">
          <constrainedElement xmi:idref="_holding"/>
          <specification xmi:type="uml:DurationInterval" xmi:id="_dUnitI" min="_dUnitD" max="_dUnitD"/>
        </ownedRule>
        <observation xmi:type="uml:DurationObservation" xmi:id="_took" name="Time_Focus">
          <event xmi:idref="_coarse"/>
          <event xmi:idref="_holding"/>
        </observation>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Duration" xmi:id="_dNumD">
      <expr xmi:type="uml:LiteralInteger" xmi:id="_dNumV" value="200"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Duration" xmi:id="_dStrD">
      <expr xmi:type="uml:LiteralString" xmi:id="_dStrV" value="t = 1500"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Duration" xmi:id="_dExprD">
      <expr xmi:type="uml:OpaqueExpression" xmi:id="_dExprV"><body>settle</body></expr>
    </packagedElement>
    <packagedElement xmi:type="uml:Duration" xmi:id="_dUnitD">
      <expr xmi:type="uml:OpaqueExpression" xmi:id="_dUnitV"><body>settle s</body></expr>
    </packagedElement>`

const bareDurationApplications = `
  <sysml:Block xmi:id="_lensB" base_Class="_lens"/>`

// A duration with no time unit is in milliseconds, the simulation toolkit's default,
// whether it is a number, a string of one, or an expression; one with a unit is in
// that unit. Each is written as a wait in seconds, the reading is noted, and a run
// takes 0.2 + 1.5 + 0.25 + 250 s.
func TestDurationWithNoUnitIsInMilliseconds(t *testing.T) {
	r := migrateDocument(t, bareDurations, bareDurationApplications)
	for _, line := range []string{
		"action wait accept after 0.2 [SI::s];",
		"action wait2 accept after 1.5 [SI::s];",
		"action wait3 accept after (this.settle * 0.001) [SI::s];",
		"action wait4 accept after this.settle [SI::s];",
	} {
		wantLine(t, r.Notation, line)
	}
	wantClean(t, "t.sysml", r)
	wantNote(t, r, "_dNum", migrate.Approximated, "the duration 200 carries no unit and is read in milliseconds, the simulation toolkit's default")
	wantNote(t, r, "_dStr", migrate.Approximated, `the duration "t = 1500" carries no unit and is read in milliseconds, the simulation toolkit's default`)
	wantNote(t, r, "_dExpr", migrate.Approximated, `the duration "settle" is read as the expression this.settle * 0.001, in seconds; the expression carries no unit and is read in milliseconds, the simulation toolkit's default`)
	wantNote(t, r, "_dUnit", migrate.Approximated, `the duration "settle s" is read as the expression this.settle, in seconds`)

	s := session(t, r)
	meta(t, s, "%instantiate Lens")
	wantVerdict(t, s.RunAction("Lens::Focusing", "Lens"))
	runs := strings.Join(s.RunRuns("Lens::Focusing", []string{"Lens"}, 2, seedOf(1), []string{"Time_Focus"}).Lines, "\n")
	if want := "Time_Focus: 2 run(s), min 251.95, mean 251.95, max 251.95"; !strings.Contains(runs, want) || strings.Contains(runs, "error") {
		t.Errorf("runs lack %q:\n%s", want, runs)
	}
}
