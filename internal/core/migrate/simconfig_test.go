package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
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
      executionTarget="_missing"/>
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
		"action def 'Group 3' {",
	} {
		wantLine(t, r.Notation, line)
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
	wantNote(t, r, "_g3", migrate.Approximated, "names 2 execution targets, and a run has one object to run on")
	if errs := errors(t, "t.sysml", r.Notation); len(errs) > 0 {
		t.Errorf("the migrated configurations do not analyse clean: %v\n%s", errs, r.Notation)
	}
}

// seedOf is a seed as RunRuns takes it, named.
func seedOf(seed uint64) *uint64 { return &seed }
