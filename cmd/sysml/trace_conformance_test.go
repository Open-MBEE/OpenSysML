package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checkTraceCLI(t *testing.T, binary, machine, trace, model string, depth string, exit int, status string, row int) {
	t.Helper()
	flags := []string{"-no-record-cache", "-json", "-state", machine, "-check-trace", trace}
	if depth != "" {
		flags = append(flags, "-check-depth", depth)
	}
	got := checkPaths(t, binary, append(flags, model)...)
	var verdict conformResult
	if err := json.Unmarshal([]byte(got.stdout), &verdict); err != nil {
		t.Fatalf("decode verdict: %v\n%s", err, got.output())
	}
	if got.status != exit || verdict.Status != status || verdict.Row != row {
		t.Fatalf("got exit=%d verdict=%+v; want exit=%d status=%s row=%d\n%s", got.status, verdict, exit, status, row, got.output())
	}
}

// These traces have the same observable prefix but require different hidden paths.
func TestTraceConformanceKeepsCompatibleBranches(t *testing.T) {
	binary := buildCLI(t)
	dir := filepath.Join("testdata", "trace-conformance")
	for _, name := range []string{"confirmed", "retry"} {
		t.Run(name, func(t *testing.T) {
			checkTraceCLI(t, binary, "TraceReview::ReviewSession::Review TraceReview::profile",
				filepath.Join(dir, name+".jsonl"), filepath.Join(dir, "review.sysml"), "", 0, "matched", 5)
		})
	}
}

func TestTraceConformanceChargingInterlock(t *testing.T) {
	binary := buildCLI(t)
	dir := filepath.Join("..", "..", "examples", "charging-interlock-design")
	machine := "ChargingInterlock::StationController::Interlock ChargingInterlock::rig.controller"
	model := filepath.Join(dir, "station.sysml")
	checkTraceCLI(t, binary, machine, filepath.Join(dir, "observed-normal.jsonl"), model, "", 0, "matched", 6)
	checkTraceCLI(t, binary, machine, filepath.Join(dir, "observed-early-output.jsonl"), model, "", 1, "no_match", 3)
	checkTraceCLI(t, binary, machine, filepath.Join(dir, "observed-normal.jsonl"), model, "1", 2, "inconclusive", 2)
}

func TestTraceConformanceRejectsNonmatchingAndUnsupportedPaths(t *testing.T) {
	binary := buildCLI(t)
	dir := filepath.Join("testdata", "trace-conformance")
	confirmed, err := os.ReadFile(filepath.Join(dir, "confirmed.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(confirmed), "\n")
	initial := lines[0]
	for _, tc := range []struct {
		name, trace, status string
		exit, row           int
	}{
		{"cross-branch splice", strings.Join(lines[:4], "\n") + "\n" +
			`{"signal":"TraceReview::Reload","observe":{"screen":"retry","confirmedVisible":false}}`, "no_match", 1, 5},
		{"unknown signal", initial + "\n" +
			`{"signal":"TraceReview::Unknown","observe":{"screen":"blank","confirmedVisible":false}}`, "invalid", 2, 2},
		{"wrong payload type", initial + "\n" +
			`{"signal":"TraceReview::Timed","args":{"attempt":"1"},"observe":{"screen":"blank","confirmedVisible":false}}`, "invalid", 2, 2},
		{"typed payload yields observed value", `{"observe":{"score":0}}` + "\n" +
			`{"signal":"TraceReview::Scored","args":{"score":17},"observe":{"score":17}}`, "matched", 0, 2},
		{"first order-dependent observation is inconclusive", `{"observe":{"derivedScore":12}}`, "inconclusive", 2, 0},
		{"alternate order-dependent observation is inconclusive", `{"observe":{"derivedScore":30}}`, "inconclusive", 2, 0},
		{"real observation is unsupported", `{"observe":{"fraction":0}}`, "inconclusive", 2, 0},
		{"pending timed work", initial + "\n" +
			`{"signal":"TraceReview::Timed","args":{"attempt":1},"observe":{"screen":"blank","confirmedVisible":false}}`, "inconclusive", 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			trace := filepath.Join(t.TempDir(), "observed.jsonl")
			if err := os.WriteFile(trace, []byte(tc.trace), 0600); err != nil {
				t.Fatal(err)
			}
			checkTraceCLI(t, binary, "TraceReview::ReviewSession::Review TraceReview::profile",
				trace, filepath.Join(dir, "review.sysml"), "", tc.exit, tc.status, tc.row)
		})
	}
}

func TestCheckTraceUsesCommonReportFormats(t *testing.T) {
	binary := buildCLI(t)
	dir := filepath.Join("..", "..", "examples", "charging-interlock-design")
	args := []string{"-no-record-cache", "-state", "ChargingInterlock::StationController::Interlock ChargingInterlock::rig.controller",
		"-check-trace", filepath.Join(dir, "observed-normal.jsonl"), filepath.Join(dir, "station.sysml")}
	jsonFlag := checkPaths(t, binary, append(append([]string{}, args...), "-json")...)
	jsonFormat := checkPaths(t, binary, append(append([]string{}, args...), "-format", "json")...)
	if jsonFlag.status != 0 || jsonFormat.status != 0 || jsonFlag.stdout != jsonFormat.stdout {
		t.Fatalf("expected equivalent JSON reports: %s; %s", jsonFlag.output(), jsonFormat.output())
	}
	plain := checkPaths(t, binary, args...)
	if plain.status != 0 || strings.TrimSpace(plain.stdout) != "Trace matches through row 6" {
		t.Fatalf("unexpected text report: %s", plain.output())
	}
	witness := filepath.Join(t.TempDir(), "schedule.witness")
	if err := os.WriteFile(witness, []byte("no choice points\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		flags       []string
		explanation string
	}{
		{[]string{"-schedule", "replay:" + witness}, "-check-trace cannot be combined with -schedule"},
		{[]string{"-schedule", "reverse"}, "-check-trace cannot be combined with -schedule"},
		{[]string{"-format", "junit"}, "-check-trace supports -format text or json"},
	} {
		got := checkPaths(t, binary, append(append([]string{}, args...), tc.flags...)...)
		if got.status != 2 || !strings.Contains(got.stdout+got.stderr, tc.explanation) {
			t.Errorf("wanted error containing %q: %s", tc.explanation, got.output())
		}
	}
}
