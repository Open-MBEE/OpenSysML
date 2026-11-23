package hygiene

import (
	"os"
	"strings"
	"testing"
)

// Only the nightly reproduced the reference differential's baseline, so a change that moved it merged
// unnoticed; these tests hold pr.yml and the changed-areas selector to re-running it on a push.

const pilotDiffCheck = "go run -C tools ./cmd/pilot-diff -check"

func TestPRWorkflowRerunsTheReferenceDifferential(t *testing.T) {
	workflow, _ := loadPRWorkflow(t)

	changes, ok := workflow.Jobs["changes"]
	if !ok {
		t.Fatal("pr.yml has no changes job")
	}
	if !strings.Contains(changes.Outputs["pilotdiff"], "steps.areas.outputs.pilotdiff") {
		t.Fatalf("pr.yml changes.outputs.pilotdiff = %q, want the selector's pilotdiff area", changes.Outputs["pilotdiff"])
	}

	job, ok := workflow.Jobs["pilot-differential"]
	if !ok {
		t.Fatal("pr.yml has no pilot-differential job: nothing re-runs the reference differential before a change merges")
	}
	if strings.TrimSpace(job.If) != "needs.changes.outputs.pilotdiff == 'true'" {
		t.Errorf("pr.yml pilot-differential.if = %q, want it gated on the pilotdiff area alone", job.If)
	}
	var checks, java, sysml, kerml, corpora bool
	for _, step := range job.Steps {
		checks = checks || strings.TrimSpace(step.Run) == pilotDiffCheck
		java = java || (strings.HasPrefix(step.Uses, "actions/setup-java@") && step.With["java-version"] == "21")
		sysml = sysml || strings.Contains(step.Run, "download-pilot-sysml-validator.sh")
		kerml = kerml || strings.Contains(step.Run, "download-pilot-kerml-validator.sh")
		corpora = corpora || step.Uses == "./.github/actions/corpora"
	}
	if !checks {
		t.Errorf("pr.yml pilot-differential runs no step %q", pilotDiffCheck)
	}
	if !java || !sysml || !kerml || !corpora {
		t.Errorf("pr.yml pilot-differential: Java 21 = %v, SysML validator = %v, KerML validator = %v, corpora = %v", java, sysml, kerml, corpora)
	}
	requiredByTheGate(t, workflow, "pilot-differential", "PILOT_DIFFERENTIAL_RESULT")
}

// The merge queue pays nothing for the differential: the area is off under the queue tier whatever the diff.
func TestChangedAreasKeepTheReferenceDifferentialOutOfTheQueue(t *testing.T) {
	raw, err := os.ReadFile("../../scripts/ci-changed-areas.sh")
	if err != nil {
		t.Fatal(err)
	}
	var emit string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "emit pilotdiff ") {
			emit = line
		}
	}
	if emit == "" {
		t.Fatal("scripts/ci-changed-areas.sh emits no pilotdiff area")
	}
	if !strings.Contains(emit, `[[ "$tier" != queue ]] && {`) || !strings.Contains(emit, `matches "$pilotdiff_pattern"`) {
		t.Errorf("scripts/ci-changed-areas.sh emits pilotdiff as %q, want it off under the queue tier and on its own paths", emit)
	}
}
