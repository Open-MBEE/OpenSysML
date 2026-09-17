package main

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var spacecraftModel = filepath.Join("..", "..", "examples", "runtime-showcase", "spacecraft-comms.sysml")

// spacecraftMachine names the spacecraft's machine on the mission's part, as the
// runtime showcase explores it.
var spacecraftMachine = []string{"-instantiate", "SpacecraftComms::mission",
	"-state", "SpacecraftComms::SpacecraftVehicle::modes SpacecraftComms::mission.spacecraftVehicle", "-advance", "80"}

// spacecraftOutcomes are the two ends of the race at t=79 between the drain, the
// frame send and the charge as the checker steps it, one token a step: how many
// frames go out, and whether the charge lands, before `BatteryLow` interrupts.
var spacecraftOutcomes = []string{"battery 39, data 52224, 49 frames", "battery 41, data 53248, 48 frames"}

// TestEngineCheckWitnessesTheSpacecraftRaceAndReplaysEach checks -engine check on
// the showcase's spacecraft: the race at t=79, inside a state whose `do` body
// loops through timed waits, is a divergence of the battery and the data left,
// and every witness written replays to the values it claims.
func TestEngineCheckWitnessesTheSpacecraftRaceAndReplaysEach(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()

	got := runFiles(t, binary, []string{spacecraftModel}, append([]string{"-engine", "check", "-check-witness", dir}, spacecraftMachine...)...)
	wantReport(t, got, 1,
		"divergent: this.battery ends as 39 or 41",
		"divergent: this.data ends as 52224 or 53248",
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
