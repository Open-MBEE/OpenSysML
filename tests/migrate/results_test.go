package migrate_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/repl"
)

// storedResults are the weighted chooser's run configuration with the result
// snapshots a simulation tool stored under its result package: two typed by
// the target's classifier, one classifier-less whose slots are of its
// features, one nested in a sub-package, and an instance of another block. The
// snapshots record pA at the 1.0 Sure fixes it to and observe pB, whose NaN
// default configures nothing.
const storedResults = weightedChooser + `
    <packagedElement xmi:type="uml:Class" xmi:id="_g0" name="Group 0"/>
    <packagedElement xmi:type="uml:Package" xmi:id="_results" name="Results">
      <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_r1" name="run 1" classifier="_sure">
        <slot xmi:type="uml:Slot" xmi:id="_r1a" definingFeature="_pa">
          <value xmi:type="uml:LiteralReal" xmi:id="_r1av" value="1.0"/>
        </slot>
        <slot xmi:type="uml:Slot" xmi:id="_r1b" definingFeature="_pb">
          <value xmi:type="uml:LiteralInteger" xmi:id="_r1bv" value="3"/>
        </slot>
        <slot xmi:type="uml:Slot" xmi:id="_r1f" definingFeature="_flag">
          <value xmi:type="uml:LiteralBoolean" xmi:id="_r1fv" value="true"/>
        </slot>
        <slot xmi:type="uml:Slot" xmi:id="_r1x" definingFeature="_pa">
          <value xmi:type="uml:LiteralReal" xmi:id="_r1xv" value="NaN"/>
        </slot>
        <slot xmi:type="uml:Slot" xmi:id="_r1n">
          <definingFeature href="Analysis.mdzip#_n">
            <xmi:Extension extender="MagicDraw UML 2024x">
              <referenceExtension referentPath="Analysis::MonteCarlo::N" referentType="Property"/>
            </xmi:Extension>
          </definingFeature>
          <value xmi:type="uml:LiteralInteger" xmi:id="_r1nv" value="5"/>
        </slot>
      </packagedElement>
      <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_r2" name="run 2" classifier="_sure">
        <slot xmi:type="uml:Slot" xmi:id="_r2a" definingFeature="_pa">
          <value xmi:type="uml:LiteralReal" xmi:id="_r2av" value="1.0"/>
        </slot>
        <slot xmi:type="uml:Slot" xmi:id="_r2b" definingFeature="_pb">
          <value xmi:type="uml:LiteralReal" xmi:id="_r2bv"/>
        </slot>
      </packagedElement>
      <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_r3">
        <slot xmi:type="uml:Slot" xmi:id="_r3b" definingFeature="_pb">
          <value xmi:type="uml:LiteralReal" xmi:id="_r3bv" value="0.25"/>
        </slot>
      </packagedElement>
      <packagedElement xmi:type="uml:Package" xmi:id="_more" name="More">
        <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_r4" name="run 4" classifier="_chooser">
          <slot xmi:type="uml:Slot" xmi:id="_r4b" definingFeature="_pb">
            <value xmi:type="uml:LiteralReal" xmi:id="_r4bv" value="0.75"/>
          </slot>
        </packagedElement>
      </packagedElement>
      <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_o1" name="other" classifier="_other">
        <slot xmi:type="uml:Slot" xmi:id="_o1a" definingFeature="_po">
          <value xmi:type="uml:LiteralReal" xmi:id="_o1av" value="9.0"/>
        </slot>
      </packagedElement>
    </packagedElement>`

// The snapshots of a configuration's result location are indexed per
// configuration: those typed by the target's classifier, a general of it, or
// none at all with slots of its features, at any depth of the package, and
// only the slots holding one number, the rest noted.
func TestResultSnapshotsAreIndexedPerConfiguration(t *testing.T) {
	r := migrateDocument(t, storedResults, `
  <sysml:Block xmi:id="_s1" base_Class="_chooser"/>
  <sysml:Block xmi:id="_s2" base_Class="_sure"/>
  <sysml:Block xmi:id="_s3" base_Class="_other"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c0" base_Class="_g0"
      executionTarget="_s0" resultLocation="_results" numberOfRuns="4" durationSimulationMode="average"/>`)
	if r.Results == nil || len(r.Results.Configurations) != 1 {
		t.Fatalf("results = %+v, want one configuration", r.Results)
	}
	got := r.Results.Configurations[0]
	want := migrate.ConfigurationResults{
		ID: "_g0", Name: "'Group 0'", Runs: 4, Draws: "average",
		Target: "target", Behavior: "run", Location: "Results",
		Observables: []string{"pA", "pB"},
		Snapshots: []migrate.Snapshot{
			{ID: "_r1", Name: "run 1", Values: map[string]float64{"pA": 1.0, "pB": 3}},
			{ID: "_r2", Name: "run 2", Values: map[string]float64{"pA": 1.0, "pB": 0}},
			{ID: "_r3", Values: map[string]float64{"pB": 0.25}},
			{ID: "_r4", Name: "run 4", Values: map[string]float64{"pB": 0.75}},
		},
		Notes: []string{
			"the slot of Analysis::MonteCarlo::N is defined outside the document in 1 snapshot(s), so it is not among the results",
			"the slot of flag holds a LiteralBoolean, which is no number in 1 snapshot(s), so it is not among the results",
			`the slot of pA holds "NaN", which is no finite number in 1 snapshot(s), so it is not among the results`,
		},
	}
	if !reflect.DeepEqual(got, want) {
		gotJSON, _ := json.MarshalIndent(got, "", "  ")
		wantJSON, _ := json.MarshalIndent(want, "", "  ")
		t.Errorf("configuration results:\n%s\nwant:\n%s", gotJSON, wantJSON)
	}
	if values := got.Values("pB"); !reflect.DeepEqual(values, []float64{3, 0, 0.25, 0.75}) {
		t.Errorf("Values(pB) = %v", values)
	}
	wantLine(t, r.Notation, "/* results of the simulation tool: 4 snapshot(s) in Results holding pA, pB */")
	wantNote(t, r, "_g0", migrate.Mapped, "")
	wantNote(t, r, "_r1", migrate.Mapped, "")
	wantNote(t, r, "_r3", migrate.Approximated, "classified by Chooser, the owner of its slots' defining features, since it names no classifier")
	wantClean(t, "t.sysml", r)
}

// A snapshot recording another value of a feature the target configures — by a
// slot of its own, or by the default of its classifier — is of a run on another
// configuration sharing the result package, and is left out with a note; one not
// recording the feature at all cannot be told apart and stays. A default spelling
// no number configures nothing: it is how the tool leaves the observables its runs fill.
func TestResultSnapshotsOfAnotherConfigurationAreLeftOut(t *testing.T) {
	r := migrateDocument(t, storedResults+`
    <packagedElement xmi:type="uml:Class" xmi:id="_g1" name="Group 1"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_g2" name="Group 2"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_seen" name="Seen">
      <generalization xmi:type="uml:Generalization" xmi:id="_seen_g" general="_sure"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_pb2" name="pB" redefinedProperty="_pb">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_pb2v"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_s5" name="fixed" classifier="_sure">
      <slot xmi:type="uml:Slot" xmi:id="_s5b" definingFeature="_pb">
        <value xmi:type="uml:LiteralInteger" xmi:id="_s5bv" value="3"/>
      </slot>
      <slot xmi:type="uml:Slot" xmi:id="_s5f" definingFeature="_flag">
        <value xmi:type="uml:LiteralBoolean" xmi:id="_s5fv" value="false"/>
      </slot>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_s6" name="chooser" classifier="_chooser"/>`, `
  <sysml:Block xmi:id="_s1" base_Class="_chooser"/>
  <sysml:Block xmi:id="_s2" base_Class="_sure"/>
  <sysml:Block xmi:id="_s3" base_Class="_other"/>
  <sysml:Block xmi:id="_s4" base_Class="_seen"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c0" base_Class="_g0"
      executionTarget="_s5" resultLocation="_results"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c1" base_Class="_g1"
      executionTarget="_s6" resultLocation="_results"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c2" base_Class="_g2"
      executionTarget="_seen" resultLocation="_results"/>`)
	if len(r.Results.Configurations) != 3 {
		t.Fatalf("results index %d configuration(s), want 3", len(r.Results.Configurations))
	}
	// fixed sets pB to 3 by a slot: run 1 recorded it, the other three another pB.
	fixed := r.Results.Configurations[0]
	if ids := snapshotIDs(fixed); !reflect.DeepEqual(ids, []string{"_r1"}) {
		t.Errorf("the snapshots of fixed are %v, want run 1 alone", ids)
	}
	if want := "3 snapshot(s) record other values of pB than the target configures, so they are of another configuration and not among the results"; !slices.Contains(fixed.Notes, want) {
		t.Errorf("fixed notes %q, want %q among them", fixed.Notes, want)
	}
	// chooser leaves pA at Chooser's 0.25: the two runs recording 1.0 were on a
	// Sure, and the two recording no pA cannot be told apart.
	chooser := r.Results.Configurations[1]
	if ids := snapshotIDs(chooser); !reflect.DeepEqual(ids, []string{"_r3", "_r4"}) {
		t.Errorf("the snapshots of chooser are %v, want the two recording no pA", ids)
	}
	if want := "2 snapshot(s) record other values of pA than the target configures, so they are of another configuration and not among the results"; !slices.Contains(chooser.Notes, want) {
		t.Errorf("chooser notes %q, want %q among them", chooser.Notes, want)
	}
	// Seen runs as a class: Sure's pA is 1.0 as the snapshots record, and its
	// pB redefinition leaves the value blank for the run to fill.
	seen := r.Results.Configurations[2]
	if ids := snapshotIDs(seen); !reflect.DeepEqual(ids, []string{"_r1", "_r2", "_r3", "_r4"}) {
		t.Errorf("the snapshots of Seen are %v, want all four", ids)
	}
	for _, note := range seen.Notes {
		if strings.Contains(note, "another configuration") {
			t.Errorf("Seen leaves a snapshot out: %s", note)
		}
	}
	wantLine(t, r.Notation, "/* results of the simulation tool: 1 snapshot(s) in Results holding pA, pB */")
	wantLine(t, r.Notation, "/* results of the simulation tool: 2 snapshot(s) in Results holding pB */")
	wantLine(t, r.Notation, "/* results of the simulation tool: 4 snapshot(s) in Results holding pA, pB */")
	wantNote(t, r, "_g0", migrate.Mapped, "")
	wantNote(t, r, "_g1", migrate.Mapped, "")
	wantNote(t, r, "_g2", migrate.Mapped, "")
}

func snapshotIDs(c migrate.ConfigurationResults) []string {
	var ids []string
	for _, s := range c.Snapshots {
		ids = append(ids, s.ID)
	}
	return ids
}

// A configuration whose results cannot be read says so: a result location
// outside the document in the report, a location with no snapshot of the
// target or a target with no classifier in the sidecar's notes.
func TestResultSnapshotsReportWhatIsNotRead(t *testing.T) {
	r := migrateDocument(t, storedResults+`
    <packagedElement xmi:type="uml:Class" xmi:id="_g1" name="Group 1"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_g2" name="Group 2"/>
    <packagedElement xmi:type="uml:Package" xmi:id="_empty" name="Empty"/>`, `
  <sysml:Block xmi:id="_s1" base_Class="_chooser"/>
  <sysml:Block xmi:id="_s2" base_Class="_sure"/>
  <sysml:Block xmi:id="_s3" base_Class="_other"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c0" base_Class="_g0"
      executionTarget="_s0" resultLocation="_elsewhere"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c1" base_Class="_g1"
      executionTarget="_s0" resultLocation="_empty"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c2" base_Class="_g2"
      resultLocation="_results"/>`)
	wantNote(t, r, "_g0", migrate.Approximated, `the result location "_elsewhere" is outside the document, so its snapshots are not read`)
	wantNote(t, r, "_g1", migrate.Mapped, "")
	wantNote(t, r, "_g2", migrate.Approximated, "names no execution target")
	configs := r.Results.Configurations
	if len(configs) != 3 {
		t.Fatalf("results index %d configuration(s), want 3", len(configs))
	}
	if configs[0].Location != "" || len(configs[0].Snapshots) != 0 || len(configs[0].Notes) != 0 {
		t.Errorf("a location outside the document indexes %+v", configs[0])
	}
	if want := []string{"the result location Empty holds no snapshot of the target's classifier"}; !reflect.DeepEqual(configs[1].Notes, want) {
		t.Errorf("an empty location notes %q, want %q", configs[1].Notes, want)
	}
	if want := []string{"the snapshots in Results are not read: the configuration has no target classifier they could be of"}; !reflect.DeepEqual(configs[2].Notes, want) || len(configs[2].Snapshots) != 0 {
		t.Errorf("a configuration with no target indexes %+v", configs[2])
	}
	for _, c := range configs {
		if len(c.Observables) != 0 {
			t.Errorf("%s lists observables %v with no snapshot", c.Name, c.Observables)
		}
	}
}

// A tool writing a reference tag as one IDREFS attribute lists every id in one
// value: a configuration naming two result locations that way reads both, one
// naming two execution targets has too many to run on.
func TestReferenceTagsListingSeveralIDsAreSplit(t *testing.T) {
	r := migrateDocument(t, storedResults+`
    <packagedElement xmi:type="uml:Class" xmi:id="_g1" name="Group 1"/>
    <packagedElement xmi:type="uml:Package" xmi:id="_late" name="Late">
      <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_r5" name="run 5" classifier="_sure">
        <slot xmi:type="uml:Slot" xmi:id="_r5b" definingFeature="_pb">
          <value xmi:type="uml:LiteralReal" xmi:id="_r5bv" value="0.5"/>
        </slot>
      </packagedElement>
    </packagedElement>`, `
  <sysml:Block xmi:id="_s1" base_Class="_chooser"/>
  <sysml:Block xmi:id="_s2" base_Class="_sure"/>
  <sysml:Block xmi:id="_s3" base_Class="_other"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c0" base_Class="_g0"
      executionTarget="_s0" resultLocation="_results _late"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c1" base_Class="_g1"
      executionTarget="_s0 _o1" resultLocation="_results"/>`)
	wantNote(t, r, "_g0", migrate.Mapped, "")
	wantNote(t, r, "_g1", migrate.Approximated, "names 2 execution targets, and a run has one object to run on")
	configs := r.Results.Configurations
	if len(configs) != 2 {
		t.Fatalf("results index %d configuration(s), want 2", len(configs))
	}
	var ids []string
	for _, s := range configs[0].Snapshots {
		ids = append(ids, s.ID)
	}
	if want := []string{"_r1", "_r2", "_r3", "_r4", "_r5"}; configs[0].Location != "Results, Late" || !slices.Equal(ids, want) {
		t.Errorf("two locations index %q holding %v, want Results, Late holding %v", configs[0].Location, ids, want)
	}
	if values := configs[0].Values("pB"); !reflect.DeepEqual(values, []float64{3, 0, 0.25, 0.75, 0.5}) {
		t.Errorf("Values(pB) = %v", values)
	}
	wantLine(t, r.Notation, "/* results of the simulation tool: 5 snapshot(s) in Results, Late holding pA, pB */")
	if configs[1].Target != "" || len(configs[1].Snapshots) != 0 {
		t.Errorf("a configuration with two targets indexes %+v", configs[1])
	}
	wantClean(t, "t.sysml", r)
}

// The sidecar is read back as written, and anything else is refused: JSON of
// another shape, a field the sidecar never writes, or no source at all.
func TestResultsSidecarRoundTrip(t *testing.T) {
	r := migrateDocument(t, storedResults, `
  <sysml:Block xmi:id="_s1" base_Class="_chooser"/>
  <sysml:Block xmi:id="_s2" base_Class="_sure"/>
  <sysml:Block xmi:id="_s3" base_Class="_other"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c0" base_Class="_g0"
      executionTarget="_s0" resultLocation="_results" numberOfRuns="4" durationSimulationMode="average"/>`)
	if want := "results of 1 run configuration(s): 1 with 4 stored snapshot(s)"; r.Results.Summary() != want {
		t.Errorf("Summary() = %q, want %q", r.Results.Summary(), want)
	}
	data, err := json.Marshal(r.Results)
	if err != nil {
		t.Fatal(err)
	}
	read, err := migrate.ReadResults(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("ReadResults: %v", err)
	}
	if !reflect.DeepEqual(read, r.Results) {
		t.Errorf("the sidecar read back is not the one written:\n%s", data)
	}
	if _, err := migrate.ReadResults(strings.NewReader(string(data) + "\n\n")); err != nil {
		t.Errorf("ReadResults(document then whitespace) error = %v", err)
	}
	for name, text := range map[string]string{
		"not JSON":          "results: none",
		"another shape":     `[1, 2]`,
		"an unknown key":    `{"source": "t.xmi", "configurations": [], "tuned": true}`,
		"no source":         `{"configurations": []}`,
		"no configurations": `{"source": "t.xmi"}`,
		"a second document": string(data) + "\n" + string(data),
		"trailing garbage":  string(data) + " results: none",
		"an unclosed tail":  string(data) + `{"source": "t.xmi"`,
	} {
		if _, err := migrate.ReadResults(strings.NewReader(text)); err == nil || !strings.Contains(err.Error(), "not the JSON -migration-results writes") {
			t.Errorf("ReadResults(%s) error = %v", name, err)
		}
	}
}

// The comparison runs the configuration as the tool did — on its target, for
// its numberOfRuns, under its draw policy — and sets the stored distribution of
// each observable beside the runs' own, read from the target's feature of the
// same name; a stored observable no run answers is noted, not dropped.
func TestComparisonRunsEachConfigurationBesideItsStoredResults(t *testing.T) {
	r := migrateDocument(t, storedResults, `
  <sysml:Block xmi:id="_s1" base_Class="_chooser"/>
  <sysml:Block xmi:id="_s2" base_Class="_sure"/>
  <sysml:Block xmi:id="_s3" base_Class="_other"/>
  <sysml:Probability xmi:id="_p1" base_ActivityEdge="_ea" probability="pA"/>
  <SimulationProfile:SimulationConfig `+simulationProfile+` xmi:id="_c0" base_Class="_g0"
      executionTarget="_s0" resultLocation="_results" numberOfRuns="4" durationSimulationMode="average"/>`)
	s := session(t, r)
	verdicts := s.CompareResults(r.Results, repl.CompareOptions{})
	if len(verdicts) != 1 {
		t.Fatalf("CompareResults = %d verdict(s), want 1", len(verdicts))
	}
	v := verdicts[0]
	lines := strings.Join(v.Lines, "\n")
	if !v.Holds() {
		t.Fatalf("the comparison = %s:\n%s", v.Status, lines)
	}
	// sure fixes pA at 1.0, which the runs' snapshots record, and pB stays NaN, so pA compares and pB is noted;
	// average draws need no seed, so the header names none.
	for _, want := range []string{
		"compare 'Group 0' — 4 stored run(s) in Results; 4 run(s) by OpenSysML, draws average",
		"observable | source                | runs | min   | mean  | p50   | p90   | max",
		"pA         | tool                  | 2    | 1.0   | 1.0   | 1.0   | 1.0   | 1.0",
		"           | OpenSysML (target.pA) | 4    | 1.0   | 1.0   | 1.0   | 1.0   | 1.0",
		"           | difference            |      | +0.0% | +0.0% | +0.0% | +0.0% | +0.0%",
		"pB         | tool                  | 4    | 0.0   | 1.0   | 0.25  | 3.0   | 3.0",
		"           | OpenSysML (target.pB) | 0    |       |       |       |       |",
		"note: target.pB holds no number in any completed run, so pB is not compared",
	} {
		if !strings.Contains(lines, want) {
			t.Errorf("the comparison lacks %q:\n%s", want, lines)
		}
	}
	if strings.Contains(lines, "seed") {
		t.Errorf("a comparison under average draws names a seed:\n%s", lines)
	}

	// -runs, -draws and -seed override the configuration; -action selects it.
	random := runtime.DrawRandom
	seed := uint64(7)
	overridden := s.CompareResults(r.Results, repl.CompareOptions{Runs: 2, Seed: &seed, Draws: &random, Only: []string{"Group 0"}})
	if lines := strings.Join(overridden[0].Lines, "\n"); !overridden[0].Holds() || !strings.Contains(lines, "2 run(s) by OpenSysML, draws random, seed 7") {
		t.Errorf("the overridden comparison = %s:\n%s", overridden[0].Status, lines)
	}
	missing := s.CompareResults(r.Results, repl.CompareOptions{Only: []string{"Group 9"}})
	if lines := strings.Join(missing[0].Lines, "\n"); missing[0].Holds() || !strings.Contains(lines, "no configuration is named Group 9") {
		t.Errorf("a comparison of no configuration = %s:\n%s", missing[0].Status, lines)
	}
	// -observe pairs a stored observable with the run feature answering it.
	paired := s.CompareResults(r.Results, repl.CompareOptions{Observe: []repl.ObservablePair{{Stored: "pB", Feature: "target.pA"}, {Stored: "pC", Feature: "clock"}}})
	lines = strings.Join(paired[0].Lines, "\n")
	for _, want := range []string{
		"pB         | tool",
		"OpenSysML (target.pA)",
		"note: the tool stored no observable named pC, which clock was to answer",
	} {
		if !strings.Contains(lines, want) {
			t.Errorf("the paired comparison lacks %q:\n%s", want, lines)
		}
	}
}
