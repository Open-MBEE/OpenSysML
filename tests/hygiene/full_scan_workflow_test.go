package hygiene

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The nightly full scan (.github/workflows/full-scan.yml) dispatches pr.yml on develop with tier=full
// unless scripts/ci-verified-tree.py finds a full-tier run; these tests hold the three files together.

func loadActionsWorkflow(t *testing.T, name string) (actionsWorkflow, string) {
	t.Helper()
	raw, err := os.ReadFile("../../.github/workflows/" + name)
	if err != nil {
		t.Fatalf("read .github/workflows/%s: %v", name, err)
	}
	var workflow actionsWorkflow
	if err := yaml.Unmarshal(raw, &workflow); err != nil {
		t.Fatalf("parse .github/workflows/%s: %v", name, err)
	}
	return workflow, string(raw)
}

func TestPRWorkflowAcceptsADispatchedTier(t *testing.T) {
	workflow, _ := loadPRWorkflow(t)
	var triggers struct {
		Dispatch struct {
			Inputs map[string]struct {
				Type    string   `yaml:"type"`
				Options []string `yaml:"options"`
				Default string   `yaml:"default"`
			} `yaml:"inputs"`
		} `yaml:"workflow_dispatch"`
	}
	if err := workflow.On.Decode(&triggers); err != nil {
		t.Fatalf("decode pr.yml on: %v", err)
	}
	tier, ok := triggers.Dispatch.Inputs["tier"]
	if !ok {
		t.Fatal("pr.yml workflow_dispatch has no tier input, so the full scan cannot ask for the full tier")
	}
	if tier.Type != "choice" || tier.Default != "full" || strings.Join(tier.Options, " ") != "full queue quick" {
		t.Errorf("pr.yml tier input = %+v, want a choice of full queue quick defaulting to full", tier)
	}
}

func TestFullScanDispatchesTheFullTierOnAnUnprovenHead(t *testing.T) {
	workflow, raw := loadActionsWorkflow(t, "full-scan.yml")
	if workflow.Permissions["actions"] != "write" {
		t.Errorf("full-scan.yml actions permission = %q, want write to dispatch pr.yml", workflow.Permissions["actions"])
	}
	if workflow.Env["BRANCH"] != "develop" || workflow.Env["WORKFLOW"] != "pr.yml" {
		t.Errorf("full-scan.yml env = %v, want BRANCH develop and WORKFLOW pr.yml", workflow.Env)
	}
	if !regexp.MustCompile(`(?m)^\s+- cron: '\d+ \d+ \* \* \*'$`).MatchString(raw) {
		t.Error("full-scan.yml has no daily cron schedule")
	}
	job, ok := workflow.Jobs["dispatch"]
	if !ok {
		t.Fatal("full-scan.yml has no dispatch job")
	}
	var run string
	for _, step := range job.Steps {
		if strings.Contains(step.Run, "gh workflow run") {
			run = step.Run
		}
	}
	if run == "" {
		t.Fatal("full-scan.yml dispatches nothing")
	}
	for _, want := range []string{
		`scripts/ci-verified-tree.py --sha "$sha" --full-tier`,
		`[ "${verdict%%:*}" = verified ]`,
		`gh workflow run "$WORKFLOW" --ref "$BRANCH" -f tier=full`,
	} {
		if !strings.Contains(run, want) {
			t.Errorf("full-scan.yml dispatch step lacks %q", want)
		}
	}
}

// The Go test job's name carries the tier and scripts/ci-verified-tree.py reads it back, so the
// two must spell the same prefixes.
func TestVerifiedTreeReadsTheGoTestJobNames(t *testing.T) {
	_, raw := loadPRWorkflow(t)
	name := regexp.MustCompile(`(?m)^    name: Go \$\{\{ needs\.changes\.outputs\.tier == 'full' && 'race ' \|\| needs\.changes\.outputs\.tier == 'queue' && 'queue ' \|\| '' \}\}tests \(\$\{\{ matrix\.shard \}\}\)$`)
	if !name.MatchString(raw) {
		t.Fatal("pr.yml go-tests is not named `Go race tests (…)` under full, `Go queue tests (…)` under queue and `Go tests (…)` under quick")
	}
	script := readRepoFile(t, "scripts/ci-verified-tree.py")
	if !strings.Contains(script, `TIER_PREFIXES = {"full": "Go race tests (", "queue": "Go queue tests ("}`) {
		t.Error("scripts/ci-verified-tree.py does not read the full and queue tiers by the pr.yml job-name prefixes")
	}
}
