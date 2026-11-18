package hygiene

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestWasmCachePrunesOnlyAfterAGateThatRan fails when wasm-cache.yml could prune or save a build cache
// its gate did not run over: every entry is then still marked unused, and the save replaces the warm one.
func TestWasmCachePrunesOnlyAfterAGateThatRan(t *testing.T) {
	raw, err := os.ReadFile("../../.github/workflows/wasm-cache.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				ID   string `yaml:"id"`
				If   string `yaml:"if"`
				Uses string `yaml:"uses"`
				Run  string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(raw, &workflow); err != nil {
		t.Fatal(err)
	}
	steps := workflow.Jobs["warm"].Steps
	index := map[string]int{}
	for i, step := range steps {
		switch {
		case strings.TrimSpace(step.Run) == "scripts/ci-go-build-cache.sh age":
			index["age"] = i
		case step.ID != "":
			index[step.ID] = i
		}
	}
	for _, name := range []string{"age", "gate", "prune", "save"} {
		if _, ok := index[name]; !ok {
			t.Fatalf("wasm-cache.yml has no %s step", name)
		}
	}
	// A step between the two that fails would leave every entry marked and the gate not run.
	if index["gate"] != index["age"]+1 {
		t.Errorf("the entries must be marked unused in the step right before the gate, not %d steps before", index["gate"]-index["age"])
	}
	if !strings.Contains(steps[index["gate"]].Run, "make wasm-check") {
		t.Errorf("the gate step no longer runs make wasm-check: %q", steps[index["gate"]].Run)
	}
	prune, save := steps[index["prune"]], steps[index["save"]]
	if !strings.Contains(prune.Run, "scripts/ci-go-build-cache.sh prune") || prune.If != "steps.gate.outcome == 'success'" {
		t.Errorf("the pruning must run scripts/ci-go-build-cache.sh prune after a gate that passed only, got if: %s", prune.If)
	}
	if !strings.HasPrefix(save.Uses, "actions/cache/save@") ||
		save.If != "always() && (steps.prune.outcome == 'success' || steps.gate.outcome == 'failure')" {
		t.Errorf("the save must follow a pruned cache or a failed gate only, got if: %s", save.If)
	}
	if index["prune"] > index["save"] {
		t.Error("the pruning must come before the save")
	}

	pr, err := os.ReadFile("../../.github/workflows/pr.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pr), "run: scripts/ci-go-build-cache-test.sh") {
		t.Error("pr.yml no longer runs scripts/ci-go-build-cache-test.sh, the only check of the pruning before a develop push")
	}
}
