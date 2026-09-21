package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/simresults"
)

// simconfigXMI is a v1 model with a «SimulationConfig» whose result package holds
// the snapshots a simulation tool stored of its runs.
var simconfigXMI = filepath.Join("..", "..", "tests", "migrate", "testdata", "xmi", "simconfig.xmi")

// montecarloXMI is a v1 model whose target inherits the MagicDraw customization's
// MonteCarloAnalysis: its result package holds runs one by one and summarised.
var montecarloXMI = filepath.Join("..", "..", "tests", "migrate", "testdata", "xmi", "montecarlo.xmi")

// TestSummarisedMigrationResultsThroughCLI checks a configuration whose tool
// summarised runs is compared by the count and mean the summary kept, pooled with
// the runs stored one by one; that one storing no run is run and told so; and
// that one stating no numberOfRuns is run once.
func TestSummarisedMigrationResultsThroughCLI(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model, sidecar := filepath.Join(dir, "model.sysml"), filepath.Join(dir, "results.json")

	migrated := runCommand(t, exec.Command(binary, montecarloXMI, "-convert", "sysml", "-o", model, "-migration-results", sidecar))
	if migrated.status != 0 {
		t.Fatalf("migrating failed: %s", migrated.output())
	}
	if !strings.Contains(migrated.stderr, "(results of 3 run configuration(s): 2 with 11 stored snapshot(s) standing for 15 run(s))") {
		t.Errorf("the sidecar summary counts no summarised runs:\n%s", migrated.output())
	}
	compared := runCommand(t, exec.Command(binary, model, "-compare-results", sidecar, "-seed", "1"))
	if compared.status != 0 {
		t.Fatalf("exit status = %d, want 0\n%s", compared.status, compared.output())
	}
	for _, want := range []string{
		"compare 'Group 0' — 14 stored run(s) over 10 snapshot(s) in Results; 3 run(s) by OpenSysML, draws average, seed 1\n",
		"p          | tool                 | 1    | 0.5   | 0.5               | 0.5   | 0.5   | 0.5",
		"t          | tool                 | 12   |       | 4.333333333333333 |       |       |",
		"           | OpenSysML (target.t) | 3    | 3.0   | 3.0               | 3.0   | 3.0   | 3.0",
		"           | difference           |      |       | -30.8%            |       |       |",
		`note: "analysis of 4 runs" summarises 4 run(s) of t: mean 3.5, deviation 0.5, 1 out of specification`,
		`note: "analysis without a deviation" summarises 2 run(s) of t: mean 7.0` + "\n",
		"note: the slot of MonteCarloAnalysis::Mean holds a LiteralString, which is no number in 1 snapshot(s), so it is not among the results",
		"note: 1 snapshot(s) record a MonteCarloAnalysis statistic that is no number, so they hold no statistics",
		"u          | tool                 | 1    | 9.0   | 9.0               | 9.0   | 9.0   | 9.0",
		"note: 2 snapshot(s) record MonteCarloAnalysis statistics whose Mean no value of t holds, though the analysis binds the two, so the statistics are not read",
		"compare 'Group 1' — no stored run in Empty; 1 run(s) by OpenSysML, draws average, seed 1\n",
		"t          | tool (no stored result to compare) | 0    |     |      |     |     |",
		"           | OpenSysML (target.t)               | 1    | 3.0 | 3.0  | 3.0 | 3.0 | 3.0",
		"note: the configuration states no numberOfRuns, so one run is made, as its tool makes without one; -runs <number> makes more",
		"compare 'Group 2' — 1 stored run(s) in Unbound Results; 1 run(s) by OpenSysML, draws average, seed 1\n",
		"note: 'Unbound Analysis' inherits MonteCarloAnalysis but binds its Mean to no feature, so its statistics summarise no observable",
	} {
		if !strings.Contains(compared.stdout, want) {
			t.Errorf("the comparison lacks %q:\n%s", want, compared.output())
		}
	}
}

// TestMigrationResultsThroughCLI checks -migration-results writes the sidecar
// -compare-results reads: the configuration's runs and draws, its target and
// behavior, and the numbers of every snapshot; then that the migrated model is
// run against it — under the configured count and policy, and under -runs,
// -seed, -draws, -clock-step, -observe and -action instead — and that misuse is refused.
func TestMigrationResultsThroughCLI(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model, sidecar := filepath.Join(dir, "model.sysml"), filepath.Join(dir, "results.json")

	migrated := runCommand(t, exec.Command(binary, simconfigXMI, "-convert", "sysml", "-o", model, "-migration-results", sidecar))
	if migrated.status != 0 {
		t.Fatalf("migrating failed: %s", migrated.output())
	}
	if !strings.Contains(migrated.stderr, "wrote "+sidecar+" (results of 1 run configuration(s): 1 with 4 stored snapshot(s))") {
		t.Errorf("the sidecar summary belongs on stderr:\n%s", migrated.output())
	}
	body, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	var results simresults.Results
	if err := json.Unmarshal(body, &results); err != nil {
		t.Fatalf("the sidecar is not JSON: %v\n%s", err, body)
	}
	if results.Source != simconfigXMI || len(results.Configurations) != 1 {
		t.Fatalf("sidecar = %s", body)
	}
	cfg := results.Configurations[0]
	if cfg.Name != "'Group 0'" || cfg.Runs != 4 || cfg.Draws != "average" || cfg.Target != "target" || cfg.Behavior != "run" ||
		cfg.Location != "Results" || strings.Join(cfg.Observables, ",") != "pA,pB" || len(cfg.Snapshots) != 4 {
		t.Errorf("configuration = %+v", cfg)
	}

	compare := func(args ...string) runOutcome {
		return runCommand(t, exec.Command(binary, append([]string{model, "-compare-results", sidecar}, args...)...))
	}
	configured := compare()
	if configured.status != 0 {
		t.Fatalf("exit status = %d, want 0\n%s", configured.status, configured.output())
	}
	for _, want := range []string{
		"compare 'Group 0' — 4 stored run(s) in Results; 4 run(s) by OpenSysML, draws average\n",
		"pA         | tool                  | 2    | 1.0   | 1.0   | 1.0   | 1.0   | 1.0",
		"           | OpenSysML (target.pA) | 4    | 1.0   | 1.0   | 1.0   | 1.0   | 1.0",
		"           | difference            |      | +0.0% | +0.0% | +0.0% | +0.0% | +0.0%",
		"pB         | tool                  | 4    | 0.0   | 1.0   | 0.25  | 3.0   | 3.0",
		"           | OpenSysML (target.pB) | 0    |",
		"note: target.pB holds no number in any completed run, so pB is not compared",
		"note: the slot of flag holds a LiteralBoolean, which is no number in 1 snapshot(s), so it is not among the results",
	} {
		if !strings.Contains(configured.stdout, want) {
			t.Errorf("the comparison lacks %q:\n%s", want, configured.output())
		}
	}
	if strings.Contains(configured.output(), "SysML v2 REPL") {
		t.Errorf("-compare-results left a prompt:\n%s", configured.output())
	}

	overridden := compare("-runs", "3", "-seed", "5", "-draws", "random", "-clock-step", "0.5", "-observe", "pA", "-observe", "pB=target.pA", "-action", "Group 0")
	if overridden.status != 0 {
		t.Fatalf("exit status = %d, want 0\n%s", overridden.status, overridden.output())
	}
	for _, want := range []string{
		"compare 'Group 0' — 4 stored run(s) in Results; 3 run(s) by OpenSysML, draws random, seed 5, clock step 0.5 s\n",
		"           | OpenSysML (target.pA) | 3    | 1.0       | 1.0   | 1.0     | 1.0    | 1.0",
		"pB         | tool                  | 4    | 0.0       | 1.0   | 0.25    | 3.0    | 3.0",
		"           | difference            |      | +1 (of 0) | +0.0% | +300.0% | -66.7% | -66.7%",
	} {
		if !strings.Contains(overridden.stdout, want) {
			t.Errorf("the overridden comparison lacks %q:\n%s", want, overridden.output())
		}
	}

	asJSON := compare("-json")
	if asJSON.status != 0 {
		t.Fatalf("exit status = %d, want 0\n%s", asJSON.status, asJSON.output())
	}
	var report struct {
		Status string `json:"status"`
		Checks []struct {
			Subject string   `json:"subject"`
			Status  string   `json:"status"`
			Lines   []string `json:"lines"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(asJSON.stdout), &report); err != nil {
		t.Fatalf("-json wrote no JSON: %v\n%s", err, asJSON.output())
	}
	if report.Status != "holds" || len(report.Checks) != 1 || report.Checks[0].Subject != "compare 'Group 0'" || len(report.Checks[0].Lines) < 8 {
		t.Errorf("-json report = %s", asJSON.stdout)
	}

	for name, tc := range map[string]struct {
		args []string
		want string
	}{
		"unknown configuration":   {[]string{model, "-compare-results", sidecar, "-action", "Group 9"}, "no configuration is named Group 9"},
		"missing sidecar":         {[]string{model, "-compare-results", filepath.Join(dir, "none.json")}, "-compare-results: open"},
		"sidecar not JSON":        {[]string{model, "-compare-results", model}, "the results are not the JSON -migration-results writes"},
		"with convert":            {[]string{model, "-compare-results", sidecar, "-convert", "ttl"}, "cannot be combined with -convert"},
		"results without xmi":     {[]string{model, "-convert", "ttl", "-migration-results", sidecar}, "-migration-results indexes the result snapshots of a SysML v1 migration"},
		"results without convert": {[]string{model, "-migration-results", sidecar}, "-migration-results accompanies -convert"},
		"results over the model":  {[]string{simconfigXMI, "-convert", "sysml", "-o", sidecar, "-migration-results", sidecar}, "-migration-results and -o both name"},
		"results over the report": {[]string{simconfigXMI, "-convert", "sysml", "-migration-report", sidecar, "-migration-results", sidecar}, "-migration-results and -migration-report both name"},
		"results over the input":  {[]string{simconfigXMI, "-convert", "sysml", "-migration-results", simconfigXMI}, "names the model being migrated"},
		"empty results path":      {[]string{simconfigXMI, "-convert", "sysml", "-migration-results="}, "-migration-results is empty"},
	} {
		t.Run(name, func(t *testing.T) {
			got := runCommand(t, exec.Command(binary, tc.args...))
			if got.status == 0 {
				t.Fatalf("expected a non-zero exit, got:\n%s", got.output())
			}
			if !strings.Contains(got.output(), tc.want) {
				t.Errorf("expected %q in the error, got:\n%s", tc.want, got.output())
			}
		})
	}

	// An explicitly empty path is a usage error, not the REPL the model alone would open.
	for name, args := range map[string][]string{
		"empty":       {model, "-compare-results="},
		"empty first": {"-compare-results=", model},
		"with action": {model, "-compare-results=", "-action", "Group 0"},
	} {
		t.Run("empty compare path "+name, func(t *testing.T) {
			got := runCommand(t, exec.Command(binary, args...))
			if got.status != 2 || !strings.Contains(got.stderr, "-compare-results is empty; name the JSON file -migration-results wrote") {
				t.Fatalf("expected a usage error, got status %d:\n%s", got.status, got.output())
			}
			if got.stdout != "" {
				t.Errorf("a usage error writes nothing to stdout, got:\n%s", got.stdout)
			}
		})
	}
}
