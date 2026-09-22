package main

import (
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

var spacecraftModel = filepath.Join("..", "..", "examples", "runtime-showcase", "spacecraft-comms.sysml")

// spacecraftMachine names the spacecraft's machine on the mission's part, as the
// runtime showcase explores it.
var spacecraftMachine = []string{"-instantiate", "SpacecraftComms::mission",
	"-state", "SpacecraftComms::SpacecraftVehicle::modes SpacecraftComms::mission.spacecraftVehicle", "-advance", "80"}

// spacecraftOutcomes are the ends of the race at t=79 between the drain, the
// frame send and the charge that the check's witnesses replay to, one witness
// per divergent value of the spacecraft's battery and data: whether the charge
// lands before the drain's test decides how many frames are consumed before
// `BatteryLow` interrupts, and a dispatch cutting the flow between a frame's
// consume and its send leaves the station one frame short. The fixed policies'
// whole-round values, 39 with 51200 left, are among them.
var spacecraftOutcomes = []string{
	"battery 39, data 51200, 49 frames",
	"battery 39, data 52224, 49 frames",
	"battery 41, data 52224, 48 frames",
	"battery 41, data 53248, 48 frames",
}

// spacecraftPair names both machines of the mission, so the station's count of
// frames is a checked feature too.
var spacecraftPair = append(slices.Clone(spacecraftMachine[:len(spacecraftMachine)-2]),
	"-state", "SpacecraftComms::GroundStation::modes SpacecraftComms::mission.groundStation", "-advance", "80")

// TestEngineCheckWitnessesTheSpacecraftRaceAndReplaysEach checks -engine check on
// the showcase's spacecraft: the race at t=79, inside a state whose `do` body
// loops through timed waits, is a divergence of the battery and the data left,
// and every witness written replays to the values it claims. Checked with the
// station's machine, the frames it counts end as 48, 49 or 50: the fixed
// policies' whole-round run, 50 frames with 39 and 51200, is enumerated.
func TestEngineCheckWitnessesTheSpacecraftRaceAndReplaysEach(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()

	got := runFiles(t, binary, []string{spacecraftModel}, append([]string{"-engine", "check", "-check-witness", dir}, spacecraftMachine...)...)
	wantReport(t, got, 1,
		"divergent: this.battery ends as 39 or 41",
		"divergent: this.data ends as 51200 or 52224 or 53248",
		"standing: sensitive (witnessed:", "replayed)")
	rejectReport(t, got, "disagrees with the witness", "not covered")

	witnesses, err := filepath.Glob(filepath.Join(dir, "*.witness"))
	if err != nil {
		t.Fatal(err)
	}
	if len(witnesses) < 2 {
		t.Fatalf("check wrote %d witnesses, want one per divergent value:\n%s", len(witnesses), got.output())
	}
	reached := map[string][]string{}
	for _, witness := range witnesses {
		replayed := runBinary(t, binary, strings.Join([]string{
			"%schedule replay:" + witness,
			"%instantiate SpacecraftComms::mission",
			"%state SpacecraftComms::mission.spacecraftVehicle",
			"%advance 80",
			"%eval in SpacecraftComms::mission.spacecraftVehicle : battery",
			"%eval in SpacecraftComms::mission.spacecraftVehicle : data",
			"%eval in SpacecraftComms::mission.groundStation : framesReceived",
		}, "\n")+"\n", []string{"-quiet", spacecraftModel})
		wantReport(t, replayed, 0, "✓ Advanced to 80.0", "Current state: lowPower | recharging")
		rejectReport(t, replayed, "replay refused", "disagrees with the witness", "names no move to follow")
		outcome := spacecraftValues(t, replayed)
		reached[outcome] = append(reached[outcome], filepath.Base(witness))
	}
	outcomes := make([]string, 0, len(reached))
	for outcome := range reached {
		outcomes = append(outcomes, outcome)
	}
	sort.Strings(outcomes)
	if strings.Join(outcomes, "; ") != strings.Join(spacecraftOutcomes, "; ") {
		t.Errorf("the witnesses replay to %v, want %v", reached, spacecraftOutcomes)
	}

	pair := runFiles(t, binary, []string{spacecraftModel}, append([]string{"-engine", "check"}, spacecraftPair...)...)
	wantReport(t, pair, 1,
		"divergent: SpacecraftComms::mission.groundStation.framesReceived ends as 48 or 49 or 50",
		"divergent: SpacecraftComms::mission.spacecraftVehicle.battery ends as 39 or 41",
		"divergent: SpacecraftComms::mission.spacecraftVehicle.data ends as 51200 or 52224 or 53248",
		"groundStation.framesReceived = 50; SpacecraftComms::mission.groundStation.isSolid = true; SpacecraftComms::mission.spacecraftVehicle.battery = 39; SpacecraftComms::mission.spacecraftVehicle.chargePerSecond = 1; SpacecraftComms::mission.spacecraftVehicle.data = 51200;")
	rejectReport(t, pair, "framesReceived = 50; SpacecraftComms::mission.groundStation.isSolid = true; SpacecraftComms::mission.spacecraftVehicle.battery = 41", "not covered")
}

// TestExploreTablesTheSpacecraftRaceWithinItsBudget checks -schedule explore on the
// showcase's spacecraft: a run to t=80 meets more choice points than the default
// depth, so the moves at t=79 are varied only once depth covers them, one at a
// time from the first run's, in one table under any -jobs; the entry order of
// `modes`' two regions is drawn first, telling the 41 outcome's visit orders
// apart. The 39 outcome needs the charge's wait to end and the charge to land
// before the drain — two of the first run's moves at t=79 varied together — so
// it lies past a budget of one variation a run; `check` reaches it.
func TestExploreTablesTheSpacecraftRaceWithinItsBudget(t *testing.T) {
	binary := buildCLI(t)
	explore := func(budget, jobs string) runOutcome {
		return runFiles(t, binary, []string{spacecraftModel}, append([]string{"-jobs", jobs, "-schedule", "explore:" + budget}, spacecraftMachine...)...)
	}

	shallow := explore("runs=300", "1")
	wantReport(t, shallow, 2, "? explored SpacecraftComms::SpacecraftVehicle::modes: 2 outcomes",
		"this.battery = 41; this.chargePerSecond = 1; this.data = 52224;",
		"entering modes: notRecharging(entry) first of waitingGSPing(entry), notRecharging(entry)",
		"at t=79.0: do transmitting or recharging first of do transmitting or recharging, dispatch accept BatteryLow",
		"incomplete: runs budget 300 and depth budget 64 hit after 300 runs")
	rejectReport(t, shallow, "this.battery = 39;", "this.data = 53248;")

	got := explore("runs=500,depth=1024", "1")
	wantReport(t, got, 2, "? explored SpacecraftComms::SpacecraftVehicle::modes: 3 outcomes",
		"this.battery = 41; this.chargePerSecond = 1; this.data = 52224;",
		"this.battery = 41; this.chargePerSecond = 1; this.data = 53248;",
		"do round at t=79.0: transmitting first of transmitting, recharging",
		"at t=79.0: do recharging first of do recharging, dispatch accept BatteryLow",
		"incomplete: runs budget 500 hit after 500 runs")
	rejectReport(t, got, "depth budget", "this.battery = 39;")
	if again := explore("runs=500,depth=1024", "4"); again.output() != got.output() {
		t.Errorf("under -jobs 4:\n%s\nwant\n%s", again.output(), got.output())
	}

	// The first run meets 516 choice points, varied once each, earliest first: run 496
	// varies the 495th, the first draw of `BatteryLow` against a do move at t=79, and
	// reaches 53248; by run 517 every one is varied and none alone reached 39.
	before := explore("runs=495,depth=1024", "1")
	wantReport(t, before, 2, "? explored SpacecraftComms::SpacecraftVehicle::modes: 2 outcomes", "incomplete: runs budget 495 hit after 495 runs")
	rejectReport(t, before, "this.data = 53248;")
	wantReport(t, explore("runs=496,depth=1024", "1"), 2,
		"? explored SpacecraftComms::SpacecraftVehicle::modes: 3 outcomes",
		"this.data = 53248; this.drainPerFrame = 2; this.frameSize = 1024; this.fullLevel = 100; this.isSolid = true; this.lowLevel = 40; this.resumeLevel = 80 | 1              |",
		"at t=79.0: dispatch accept BatteryLow first of do transmitting or recharging, dispatch accept BatteryLow",
		"incomplete: runs budget 496 hit after 496 runs")
	each := explore("runs=517,depth=1024", "1")
	wantReport(t, each, 2, "? explored SpacecraftComms::SpacecraftVehicle::modes: 3 outcomes", "incomplete: runs budget 517 hit after 517 runs")
	rejectReport(t, each, "this.battery = 39;")
}

var evaluated = regexp.MustCompile(`✓ (battery|data|framesReceived) \(on [^)]*\)\n  = (\d+)`)

// spacecraftValues reads the battery, the data left and the frames received off a
// session's `%eval` answers.
func spacecraftValues(t *testing.T, got runOutcome) string {
	t.Helper()
	values := map[string]string{}
	for _, m := range evaluated.FindAllStringSubmatch(got.output(), -1) {
		values[m[1]] = m[2]
	}
	if len(values) != 3 {
		t.Fatalf("the session answered %v, want battery, data and framesReceived:\n%s", values, got.output())
	}
	return "battery " + values["battery"] + ", data " + values["data"] + ", " + values["framesReceived"] + " frames"
}
