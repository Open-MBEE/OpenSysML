package hygiene

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// gateRunnerOutput drives the runner as CI and a developer would, from the
// repository root; the runner discovers the shards itself from the script.
func gateRunnerOutput(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run", "-C", "tools", "./cmd/gate-runner"}, args...)...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gate runner %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// TestGateRunnerListsEveryRaceShard checks the runner covers every shard the
// Makefile and CI matrices read from scripts/race-shard.sh.
func TestGateRunnerListsEveryRaceShard(t *testing.T) {
	got := strings.Fields(gateRunnerOutput(t, "-list"))
	want := slices.Sorted(slices.Values(raceShards(t)))
	if !slices.Equal(got, want) {
		t.Errorf("gate runner lists %v, want every declared shard %v", got, want)
	}
}

// TestGateRunnerPlansEveryRaceShard checks each tier plans one invocation per
// shard with its Makefile target and the shared snapshot guard.
func TestGateRunnerPlansEveryRaceShard(t *testing.T) {
	want := slices.Sorted(slices.Values(raceShards(t)))
	for tier, target := range map[string]string{"quick": "test-quick", "queue": "test-queue", "full": "test-shard"} {
		t.Run(tier, func(t *testing.T) {
			lines := strings.Split(strings.TrimSpace(gateRunnerOutput(t, "-tier", tier, "-plan")), "\n")
			if len(lines) != len(want)+1 {
				t.Fatalf("%s: plan has %d commands, want the snapshot and %d shards:\n%s", tier, len(lines), len(want), strings.Join(lines, "\n"))
			}
			if !strings.Contains(lines[0], "stdlib-snapshot") || strings.Contains(lines[0], "test-") {
				t.Errorf("%s: first command %q is not the snapshot on its own", tier, lines[0])
			}
			var planned []string
			for _, line := range lines[1:] {
				if !strings.HasPrefix(line, "SKIP_STDLIB_SNAPSHOT=1 make ") {
					t.Errorf("%s: %q does not tell the shard the snapshot is current", tier, line)
				}
				if !strings.Contains(line, " "+target+" ") {
					t.Errorf("%s: %q does not call %s", tier, line, target)
				}
			}
			for _, line := range lines[1:] {
				for _, field := range strings.Fields(line) {
					if strings.HasPrefix(field, "SHARD=") || strings.HasPrefix(field, "SHARDS=") {
						planned = append(planned, strings.TrimPrefix(strings.TrimPrefix(field, "SHARDS="), "SHARD="))
					}
				}
			}
			slices.Sort(planned)
			if !slices.Equal(planned, want) {
				t.Errorf("%s: plan covers %v, want %v", tier, planned, want)
			}
			for _, shard := range want {
				count := 0
				for _, got := range planned {
					if got == shard {
						count++
					}
				}
				if count != 1 {
					t.Errorf("%s: %s appears in %d planned commands, want one", tier, shard, count)
				}
			}
		})
	}
}

// TestShardTargetsSkipTheSnapshotWhenTheRunnerDidIt pins the recursive make
// guard that stops parallel shards regenerating the snapshot.
func TestShardTargetsSkipTheSnapshotWhenTheRunnerDidIt(t *testing.T) {
	makefile := readRepoFile(t, "Makefile")
	guard := "$(if $(SKIP_STDLIB_SNAPSHOT),,stdlib-snapshot)"
	if !strings.Contains(makefile, "SKIP_STDLIB_SNAPSHOT ?=") {
		t.Fatal("Makefile does not declare SKIP_STDLIB_SNAPSHOT")
	}
	for _, target := range []string{"test-shard", "test-quick", "test-queue"} {
		if !strings.Contains(makefile, "\n"+target+": "+guard+" ") {
			t.Errorf("Makefile %s does not guard stdlib-snapshot with SKIP_STDLIB_SNAPSHOT", target)
		}
	}
}

// TestGateRunnerPlanGivesToolsModuleOneOwner pins that a local all-shards plan
// runs the tools module once, while a rest-only plan keeps its coverage.
func TestGateRunnerPlanGivesToolsModuleOneOwner(t *testing.T) {
	lines := strings.Split(strings.TrimSpace(gateRunnerOutput(t, "-tier", "quick", "-shards", "grpc,rest,tools", "-plan")), "\n")
	var rest, tools string
	for _, line := range lines {
		if strings.Contains(line, "SHARDS=rest ") || strings.HasSuffix(line, "SHARDS=rest") {
			rest = line
		}
		if strings.Contains(line, "SHARDS=tools ") || strings.HasSuffix(line, "SHARDS=tools") {
			tools = line
		}
	}
	if rest == "" || tools == "" {
		t.Fatalf("plan lacks rest or tools:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(rest, "SKIP_TOOLS_TESTS=1") {
		t.Errorf("rest command %q still runs the tools module", rest)
	}
	if strings.Contains(tools, "SKIP_TOOLS_TESTS=1") {
		t.Errorf("tools command %q lost the tools module", tools)
	}
	if plan := gateRunnerOutput(t, "-tier", "quick", "-shards", "rest", "-plan"); strings.Contains(plan, "SKIP_TOOLS_TESTS=1") {
		t.Errorf("rest-only plan dropped the tools module:\n%s", plan)
	}
}

// TestShardRecipeHonoursTheRunnerToolsOwner pins the make guard that keeps the
// tools module with one shard without changing any CI job's coverage.
func TestShardRecipeHonoursTheRunnerToolsOwner(t *testing.T) {
	makefile := readRepoFile(t, "Makefile")
	if !strings.Contains(makefile, "SKIP_TOOLS_TESTS ?=") {
		t.Fatal("Makefile does not declare SKIP_TOOLS_TESTS")
	}
	for _, line := range strings.Split(makefile, "\n") {
		if !strings.Contains(line, `case "$(SHARD)" in rest|tools)`) {
			continue
		}
		if !strings.Contains(line, "$(if $(SKIP_TOOLS_TESTS),") {
			t.Errorf("tools module recipe does not honour SKIP_TOOLS_TESTS:\n%s", line)
		}
		return
	}
	t.Fatal("Makefile has no tools module recipe")
}

// TestShardScriptAllListsEveryDeclaredShard pins the discovery surface the
// runner uses to the list the hygiene checks already read from the script.
func TestShardScriptAllListsEveryDeclaredShard(t *testing.T) {
	cmd := exec.Command("bash", "scripts/race-shard.sh", "--all")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("race-shard.sh --all: %v\n%s", err, out)
	}
	got := slices.Sorted(slices.Values(strings.Fields(string(out))))
	want := slices.Sorted(slices.Values(raceShards(t)))
	if !slices.Equal(got, want) {
		t.Errorf("--all printed %v, want every declared shard %v", got, want)
	}
}
