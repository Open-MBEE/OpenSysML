package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// simulationProfile declares MagicDraw's simulation profile on an application,
// whose namespace ends in ".xmi" without being XMI's own.
const simulationProfile = `xmlns:SimulationProfile="http://www.magicdraw.com/schemas/SimulationProfile.xmi"`

// runConfigurations are run configurations of the weighted chooser: a class
// per configuration, its «SimulationConfig» naming the object it runs on.
const runConfigurations = weightedChooser + `
    <packagedElement xmi:type="uml:Package" xmi:id="_results" name="Results"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_g0" name="Group 0"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_g1" name="Group 1"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_g2" name="Group 2"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_g3" name="Group 3"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_o0" name="other" classifier="_other"/>`

// A «SimulationConfig» becomes an action def holding its execution target as a
// part and performing the target's classifier behavior, inherited or its own,
// on that part; its run settings become Simulation::Configuration metadata,
// the tool's own settings a comment. A run of the def runs the behavior on an
// object of the target, whose values weigh the decision.
func TestSimulationConfigBecomesARunnableActionDef(t *testing.T) {
	r := migrateDocument(t, runConfigurations, `
  <sysml:Block xmi:id="_s1" base_Class="_chooser"/>
  <sysml:Block xmi:id="_s2" base_Class="_sure"/>
  <sysml:Block xmi:id="_s3" base_Class="_other"/>
  <sysml:Probability xmi:id="_p1" base_ActivityEdge="_ea" probability="pA"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c0" base_Class="_g0"
      executionTarget="_s0" resultLocation="_results" numberOfRuns="5" durationSimulationMode="max"
      timeVariableName="simtime" startTime="0" stepSize="1.0" timeUnit="second" runForksInParallel="true"
      treatAllClassifiersAsActive="true" autostartActiveObjects="true" animationSpeed="95" silent="true"/>`)
	for _, line := range []string{
		"action def 'Group 0' {",
		"@Simulation::Configuration {",
		"runs = 5;",
		"draws = Simulation::DrawPolicy::max;",
		`timeVariable = "simtime";`,
		"startTime = 0.0;",
		"stepSize = 1.0;",
		`timeUnit = "second";`,
		"parallelForks = true;",
		"part target : sure;",
		"perform action run ::> target.choose;",
		"/* results of the simulation tool: 0 snapshot(s) in Results */",
		"/* «SimulationConfig» settings of the simulation tool: animationSpeed = 95; silent = true */",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "applied stereotype «SimulationConfig»") {
		t.Errorf("the consumed «SimulationConfig» is also kept as a comment:\n%s", r.Notation)
	}
	wantNote(t, r, "_g0", migrate.Mapped, "")
	wantNote(t, r, "_act", migrate.Approximated, "run by every object of Chooser as its usage choose")
	if errs := errors(t, "t.sysml", r.Notation); len(errs) > 0 {
		t.Errorf("the migrated configuration does not analyse clean: %v\n%s", errs, r.Notation)
	}

	// sure fixes pA at 1.0, so the run always takes branch a and holds.
	s := session(t, r)
	meta(t, s, "%seed 1")
	wantVerdict(t, s.RunAction("Group 0"))
	runs := s.RunRuns("Group 0", nil, 5, seedOf(1), nil)
	if lines := strings.Join(runs.Lines, "\n"); !runs.Holds() || !strings.Contains(lines, "5 run(s)") {
		t.Errorf("Monte Carlo runs of the configuration = %s:\n%s", runs.Status, lines)
	}
}

// A configuration whose target runs no behavior, names no target, or sets a
// value the metadata cannot record is written with what it has and says why.
func TestSimulationConfigReportsWhatItCannotRun(t *testing.T) {
	r := migrateDocument(t, runConfigurations, `
  <sysml:Block xmi:id="_s1" base_Class="_chooser"/>
  <sysml:Block xmi:id="_s2" base_Class="_sure"/>
  <sysml:Block xmi:id="_s3" base_Class="_other"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c0" base_Class="_g0"
      executionTarget="_o0" numberOfRuns="3" durationSimulationMode="fastest"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c1" base_Class="_g1"
      numberOfRuns="1" durationSimulationMode="random" treatAllClassifiersAsActive="false"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c2" base_Class="_g2"
      executionTarget="_missing" numberOfRuns="9223372036854775808"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c3" base_Class="_g3"
      executionTarget="_s0 _o0"/>`)
	for _, line := range []string{
		"action def 'Group 0' {",
		"runs = 3;",
		"part target : other;",
		"/* «SimulationConfig» settings of the simulation tool: durationSimulationMode = fastest */",
		"action def 'Group 1' {",
		"draws = Simulation::DrawPolicy::random;",
		"/* «SimulationConfig» settings of the simulation tool: treatAllClassifiersAsActive = false */",
		"action def 'Group 2' {",
		"/* «SimulationConfig» settings of the simulation tool: numberOfRuns = 9223372036854775808 */",
		"action def 'Group 3' {",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "runs = 9223372036854775808;") {
		t.Errorf("a run count beyond int64 is recorded as metadata:\n%s", r.Notation)
	}
	if strings.Contains(string(r.Notation), "perform action run") {
		t.Errorf("a configuration performs a behavior no target has:\n%s", r.Notation)
	}
	if strings.Count(string(r.Notation), "part target") != 1 {
		t.Errorf("only the configuration with one resolved target holds a part:\n%s", r.Notation)
	}
	wantNote(t, r, "_g0", migrate.Approximated, `durationSimulationMode = "fastest" is not one of the draw policies random, min, max and average`)
	wantNote(t, r, "_g0", migrate.Approximated, "neither Other nor any general of it has a classifier behavior")
	wantNote(t, r, "_g1", migrate.Approximated, "names no execution target")
	wantNote(t, r, "_g1", migrate.Approximated, "treatAllClassifiersAsActive = false has no v2 form")
	wantNote(t, r, "_g2", migrate.Approximated, `the execution target "_missing" is outside the document`)
	wantNote(t, r, "_g2", migrate.Approximated, `numberOfRuns = "9223372036854775808" exceeds the runs a Monte Carlo can make, so it is not recorded as Simulation::Configuration::runs`)
	wantNote(t, r, "_g3", migrate.Approximated, "names 2 execution targets, and a run has one object to run on")
	if errs := errors(t, "t.sysml", r.Notation); len(errs) > 0 {
		t.Errorf("the migrated configurations do not analyse clean: %v\n%s", errs, r.Notation)
	}
}

// A configuration whose startTime sets the tool's internal clock going records
// that clock's step in the sidecar: stepSize, 1.0 unless stated, in timeUnit —
// read in seconds, as the model's bare durations are, when the unit is unstated,
// and not at all, the runs' clock continuous, when the step or the unit is no step
// or the step in the unit is more seconds than a number holds or fewer than it tells
// from none. One without
// startTime ran on the tool's real-time clock and records none.
func TestSimulationConfigRecordsTheClockStepOfTheToolsInternalClock(t *testing.T) {
	r := migrateDocument(t, runConfigurations+`
    <packagedElement xmi:type="uml:Class" xmi:id="_g4" name="Group 4"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_g5" name="Group 5"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_g6" name="Group 6"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_g7" name="Group 7"/>`, `
  <sysml:Block xmi:id="_s1" base_Class="_chooser"/>
  <sysml:Block xmi:id="_s2" base_Class="_sure"/>
  <sysml:Block xmi:id="_s3" base_Class="_other"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c0" base_Class="_g0"
      executionTarget="_s0" numberOfRuns="2" startTime="0" stepSize="0.5" timeUnit="minute"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c1" base_Class="_g1"
      executionTarget="_s0" numberOfRuns="2" startTime="0" timeUnit="second"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c2" base_Class="_g2"
      executionTarget="_s0" numberOfRuns="2" startTime="0" stepSize="2.5"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c3" base_Class="_g3"
      executionTarget="_s0" numberOfRuns="2" startTime="0" stepSize="0" timeUnit="second"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c4" base_Class="_g4"
      executionTarget="_s0" numberOfRuns="2" startTime="0" stepSize="1.0" timeUnit="tick"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c5" base_Class="_g5"
      executionTarget="_s0" numberOfRuns="2" stepSize="3.0" timeUnit="second"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c6" base_Class="_g6"
      executionTarget="_s0" numberOfRuns="2" startTime="0" stepSize="1e308" timeUnit="week"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c7" base_Class="_g7"
      executionTarget="_s0" numberOfRuns="2" startTime="0" stepSize="5e-324" timeUnit="nanosecond"/>`)
	if r.Results == nil || len(r.Results.Configurations) != 8 {
		t.Fatalf("results = %+v, want eight configurations", r.Results)
	}
	for i, want := range []float64{30, 1, 2.5, 0, 0, 0, 0, 0} {
		if got := r.Results.Configurations[i].ClockStep; got != want {
			t.Errorf("configuration %d: clockStep = %v, want %v", i, got, want)
		}
	}
	for _, line := range []string{"stepSize = 0.5;", `timeUnit = "minute";`, "stepSize = 0.0;", `timeUnit = "tick";`, "stepSize = 3.0;"} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_g0", migrate.Mapped, "")
	wantNote(t, r, "_g1", migrate.Mapped, "")
	wantNote(t, r, "_g2", migrate.Approximated, "timeUnit is unstated, so stepSize = 2.5 is read in seconds, as the model's bare durations are, where the tool's default is the millisecond")
	wantNote(t, r, "_g3", migrate.Approximated, "stepSize = 0.0 is no step the clock can tick by, so the runs' clock is continuous")
	wantNote(t, r, "_g4", migrate.Approximated, `timeUnit = "tick" is no fixed number of seconds, so the clock's step is not derived and the runs' clock is continuous`)
	wantNote(t, r, "_g5", migrate.Mapped, "")
	wantNote(t, r, "_g6", migrate.Approximated, `stepSize = 1e+308 in timeUnit = "week" is more seconds than a number holds, so the clock's step is not derived and the runs' clock is continuous`)
	wantNote(t, r, "_g7", migrate.Approximated, `stepSize = 5e-324 in timeUnit = "nanosecond" is fewer seconds than a number tells from none, so the clock's step is not derived and the runs' clock is continuous`)
	if errs := errors(t, "t.sysml", r.Notation); len(errs) > 0 {
		t.Errorf("the migrated configurations do not analyse clean: %v\n%s", errs, r.Notation)
	}
}

// Only MagicDraw's own simulation profile, at its schemas path on either of the
// vendor's hosts, makes a «SimulationConfig» a run configuration; a stereotype
// of that name from a profile elsewhere — a custom one under the vendor's host
// included — is any other profile's: kept as a comment on a plain class.
func TestSimulationConfigOfAnotherProfileIsNotARunConfiguration(t *testing.T) {
	r := migrateDocument(t, runConfigurations, `
  <sysml:Block xmi:id="_s1" base_Class="_chooser"/>
  <sysml:Block xmi:id="_s2" base_Class="_sure"/>
  <sysml:Block xmi:id="_s3" base_Class="_other"/>
  <Sim:SimulationConfig xmlns:Sim="https://nomagic.com/Schemas/simulationprofile.xmi" xmi:id="_c0" base_Class="_g0" executionTarget="_s0" numberOfRuns="2"/>
  <acme:SimulationConfig xmlns:acme="https://magicdraw.com/acme/SimulationProfile.xmi" xmi:id="_c1" base_Class="_g1" executionTarget="_s0" numberOfRuns="2"/>
  <deep:SimulationConfig xmlns:deep="http://www.magicdraw.com/schemas/custom/SimulationProfile.xmi" xmi:id="_c2" base_Class="_g2" executionTarget="_s0" numberOfRuns="2"/>
  <other:SimulationConfig xmlns:other="https://profiles.example/schemas/SimulationProfile.xmi" xmi:id="_c3" base_Class="_g3" executionTarget="_s0" numberOfRuns="2"/>`)
	wantLine(t, r.Notation, "action def 'Group 0' {")
	wantNote(t, r, "_g0", migrate.Mapped, "")
	for _, id := range []string{"_g1", "_g2", "_g3"} {
		wantNote(t, r, id, migrate.Approximated, "a plain UML class without «Block» is written as a part def")
	}
	for _, line := range []string{"part def 'Group 1' {", "part def 'Group 2' {", "part def 'Group 3' {"} {
		wantLine(t, r.Notation, line)
	}
	for _, line := range []string{"action def 'Group 1'", "action def 'Group 2'", "action def 'Group 3'"} {
		wantNoLine(t, r.Notation, line)
	}
	if n := strings.Count(string(r.Notation), "applied stereotype «SimulationConfig»"); n != 3 {
		t.Errorf("%d «SimulationConfig» comment(s), want one per lookalike:\n%s", n, r.Notation)
	}
	if errs := errors(t, "t.sysml", r.Notation); len(errs) > 0 {
		t.Errorf("the migrated document does not analyse clean: %v\n%s", errs, r.Notation)
	}
}

// testBench is a block whose classifier behavior is a «TestCase» interaction
// storing a reply; Idle Bench specializes it with a test case with no message.
const testBench = `
    <packagedElement xmi:type="uml:Class" xmi:id="_motor" name="Motor">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_speed" name="speed">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_speed0" value="0.0"/>
      </ownedAttribute>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_spin" name="Spin" method="_spinning">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_spRpm" name="rpm" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_spRes" name="result" direction="return">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
      </ownedOperation>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_spinning" name="Spinning" specification="_spin">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_spRpm2" name="rpm" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_spRes2" name="result" direction="return">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_apnRpm" name="rpm" parameter="_spRpm2"/>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_apnRes" name="result" parameter="_spRes2"/>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_set" name="set speed" structuralFeature="_speed" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_setVal" name="value"/>
        </node>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_read" name="read speed" structuralFeature="_speed">
          <result xmi:type="uml:OutputPin" xmi:id="_readOut" name="result"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_ofRpm" source="_apnRpm" target="_setVal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_cfSet" source="_set" target="_read"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_ofRes" source="_readOut" target="_apnRes"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_ctrl" name="Controller">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_got" name="got">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_got0" value="0.0"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_bench" name="Bench" classifierBehavior="_tc">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bCtrl" name="ctrl" type="_ctrl" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bMotor" name="motor" type="_motor" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bGot" name="got">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_bind">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_bind1" role="_bGot"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_bind2" role="_got" partWithPort="_bCtrl"/>
      </ownedConnector>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_tc" name="Bench Test">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_lc" name="c" represents="_bCtrl"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_lm" name="m" represents="_bMotor"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sSpin" covered="_lc" message="_mSpin"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rSpin" covered="_lm" message="_mSpin"/>
        <fragment xmi:type="uml:BehaviorExecutionSpecification" xmi:id="_exec" covered="_lm" start="_rSpin" finish="_sRet"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sRet" covered="_lm" message="_mRet"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rRet" covered="_lc" message="_mRet"/>
        <message xmi:type="uml:Message" xmi:id="_mSpin" name="spin" messageSort="synchCall" signature="_spin" sendEvent="_sSpin" receiveEvent="_rSpin">
          <argument xmi:type="uml:LiteralReal" xmi:id="_mSpinRpm" value="12.0"/>
        </message>
        <message xmi:type="uml:Message" xmi:id="_mRet" name="spun" messageSort="reply" signature="_spin" sendEvent="_sRet" receiveEvent="_rRet">
          <argument xmi:type="uml:Expression" xmi:id="_mRetArg" symbol="=">
            <operand xmi:type="uml:LiteralString" xmi:id="_mRetTarget" value="got"/>
            <operand xmi:type="uml:LiteralReal" xmi:id="_mRetValue" value="12.0"/>
          </argument>
        </message>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_idle" name="Idle Bench" classifierBehavior="_idling">
      <generalization xmi:type="uml:Generalization" xmi:id="_gIdle" general="_bench"/>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_idling" name="Idling">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_lIdle" name="m" represents="_bMotor"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_g0" name="Group 0"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_g1" name="Group 1"/>
    <packagedElement xmi:type="uml:Package" xmi:id="_results" name="Results">
      <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_r1" name="Bench at 2024.03.01 10.15" classifier="_bench">
        <slot xmi:type="uml:Slot" xmi:id="_r1g" definingFeature="_bGot">
          <value xmi:type="uml:LiteralReal" xmi:id="_r1gv" value="12.0"/>
        </slot>
      </packagedElement>
    </packagedElement>`

// A «TestCase» classifier behavior is performed as a verification with the target
// as its subject; a nearer one that is not migrated is named as the reason nothing is.
func TestSimulationConfigPerformsATestCaseClassifierBehavior(t *testing.T) {
	r := migrateDocument(t, testBench, `
  <sysml:Block xmi:id="_s1" base_Class="_motor"/>
  <sysml:Block xmi:id="_s2" base_Class="_ctrl"/>
  <sysml:Block xmi:id="_s3" base_Class="_bench"/>
  <sysml:Block xmi:id="_s4" base_Class="_idle"/>
  <sysml:BindingConnector xmi:id="_s5" base_Connector="_bind"/>
  <sysml:TestCase xmi:id="_s6" base_Behavior="_tc"/>
  <sysml:TestCase xmi:id="_s7" base_Behavior="_idling"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c0" base_Class="_g0"
      executionTarget="_bench" resultLocation="_results" numberOfRuns="1"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c1" base_Class="_g1"
      executionTarget="_idle" numberOfRuns="1"/>`)
	for _, line := range []string{
		"verification def 'Bench Test' {",
		"subject context : Bench;",
		"part target : Bench;",
		"verification run : Bench::'Bench Test' { subject context = target; }",
		"part target : 'Idle Bench';",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "perform action run")
	wantNote(t, r, "_g0", migrate.Mapped, "")
	wantNote(t, r, "_g1", migrate.Approximated, "the classifier behavior of Idle Bench is a test case whose scenario is not migrated, so no action is performed; the configuration only holds 'Idle Bench': the interaction has no message")
	if errs := errors(t, "t.sysml", r.Notation); len(errs) > 0 {
		t.Errorf("the migrated configurations do not analyse clean: %v\n%s", errs, r.Notation)
	}
	if cfg := r.Results.Configurations[0]; cfg.Behavior != "run" || cfg.Target != "target" {
		t.Errorf("the sidecar of Group 0 = behavior %q on target %q, want run on target", cfg.Behavior, cfg.Target)
	}
	if cfg := r.Results.Configurations[1]; cfg.Behavior != "" {
		t.Errorf("the sidecar of Group 1 performs %q, want nothing", cfg.Behavior)
	}

	s := session(t, r)
	verdicts := s.CompareResults(r.Results, repl.CompareOptions{Seed: seedOf(1), Only: []string{"Group 0"}})
	if len(verdicts) != 1 {
		t.Fatalf("CompareResults = %d verdict(s), want 1", len(verdicts))
	}
	lines := strings.Join(verdicts[0].Lines, "\n")
	if !verdicts[0].Holds() {
		t.Fatalf("the comparison = %s:\n%s", verdicts[0].Status, lines)
	}
	for _, want := range []string{
		"got        | tool                   | 1    | 12.0  | 12.0  | 12.0  | 12.0  | 12.0",
		"           | OpenSysML (target.got) | 1    | 12.0  | 12.0  | 12.0  | 12.0  | 12.0",
	} {
		if !strings.Contains(lines, want) {
			t.Errorf("the comparison lacks %q:\n%s", want, lines)
		}
	}
}

// parametricTarget is a block with no behavior, holding constraint properties
// itself, through a general and through a composite part.
const parametricTarget = `
    <packagedElement xmi:type="uml:Class" xmi:id="_ohm" name="Ohm">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ohmV" name="v">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ohmI" name="i">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ohmR" name="r">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_ohmRule" constrainedElement="_ohm">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_ohmSpec">
          <body>v == i * r</body>
          <language>SysML</language>
        </specification>
      </ownedRule>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_peak" name="Peak">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_peakA" name="a">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_peakB" name="b">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_peakP" name="p">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedRule xmi:type="uml:Constraint" xmi:id="_peakRule" constrainedElement="_peak">
        <specification xmi:type="uml:OpaqueExpression" xmi:id="_peakSpec">
          <body>p=max(a,b)</body>
          <language>Javascript Rhino</language>
        </specification>
      </ownedRule>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_network" name="Network">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_netOhm" name="law" type="_ohm" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_load" name="Load">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_loadPeak" type="_peak" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_circuit" name="Circuit">
      <generalization xmi:type="uml:Generalization" xmi:id="_gCircuit" general="_network"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_cLoad" name="load" type="_load" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_cPeak" name="worst" type="_peak" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_c1" name="circuit" classifier="_circuit"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_g0" name="Group 0"/>`

// A target with no behavior is one the tool's parametric solver evaluates, so
// the note names each constraint it holds and whether its rule is migrated.
func TestSimulationConfigNamesTheConstraintsOfAParametricTarget(t *testing.T) {
	r := migrateDocument(t, parametricTarget, `
  <sysml:ConstraintBlock xmi:id="_s1" base_Class="_ohm"/>
  <sysml:ConstraintBlock xmi:id="_s2" base_Class="_peak"/>
  <sysml:Block xmi:id="_s3" base_Class="_network"/>
  <sysml:Block xmi:id="_s4" base_Class="_load"/>
  <sysml:Block xmi:id="_s5" base_Class="_circuit"/>
  <sysml:ConstraintProperty xmi:id="_s6" base_Property="_netOhm"/>
  <sysml:ConstraintProperty xmi:id="_s7" base_Property="_loadPeak"/>
  <sysml:ConstraintProperty xmi:id="_s8" base_Property="_cPeak"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c0" base_Class="_g0"
      executionTarget="_c1" numberOfRuns="1"/>`)
	wantLine(t, r.Notation, "part target : circuit;")
	wantNoLine(t, r.Notation, "perform action run")
	wantNote(t, r, "_g0", migrate.Approximated, "neither Circuit nor any general of it has a classifier behavior, so the configuration only holds 'circuit'"+
		"; the tool solves the constraints it holds for values, which a v2 run checks and does not solve: "+
		`Circuit::worst : 'Peak', whose rule {Javascript Rhino} p=max(a,b) is not migrated: the call "max" is not in the translated function table`+
		"; Network::law : 'Ohm', whose rule is migrated as a constraint"+
		"; Load::peak : 'Peak', as above")
	if errs := errors(t, "t.sysml", r.Notation); len(errs) > 0 {
		t.Errorf("the migrated configuration does not analyse clean: %v\n%s", errs, r.Notation)
	}
}

// seedOf is a seed as RunRuns takes it, named.
func seedOf(seed uint64) *uint64 { return &seed }
