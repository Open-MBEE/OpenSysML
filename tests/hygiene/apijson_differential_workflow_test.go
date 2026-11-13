package hygiene

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// The api-json baseline records no digest of our writer, so pr.yml's apijson-differential job is what
// keeps it true; these tests hold the workflow and the changed-areas selector to that.

const apijsonCheck = "go run -C tools ./cmd/pilot-apijson-diff -check"

func TestPRWorkflowRerunsTheDerivedPropertyDifferential(t *testing.T) {
	workflow, _ := loadPRWorkflow(t)

	changes, ok := workflow.Jobs["changes"]
	if !ok {
		t.Fatal("pr.yml has no changes job")
	}
	if !strings.Contains(changes.Outputs["apijson"], "steps.areas.outputs.apijson") {
		t.Fatalf("pr.yml changes.outputs.apijson = %q, want the selector's apijson area", changes.Outputs["apijson"])
	}

	job, ok := workflow.Jobs["apijson-differential"]
	if !ok {
		t.Fatal("pr.yml has no apijson-differential job: nothing re-runs the differential before a writer change merges")
	}
	if !strings.Contains(job.If, "needs.changes.outputs.apijson == 'true'") {
		t.Errorf("pr.yml apijson-differential.if = %q, want it gated on the apijson area", job.If)
	}
	var checks, java, exporter bool
	for _, step := range job.Steps {
		checks = checks || strings.TrimSpace(step.Run) == apijsonCheck
		java = java || (strings.HasPrefix(step.Uses, "actions/setup-java@") && step.With["java-version"] == "21")
		exporter = exporter || strings.Contains(step.Run, "download-pilot-json-exporter.sh")
	}
	if !checks {
		t.Errorf("pr.yml apijson-differential runs no step %q", apijsonCheck)
	}
	if !java {
		t.Error("pr.yml apijson-differential sets up no Java 21, which the pinned pilot jar and the oracle-reproduction workflow use")
	}
	if !exporter {
		t.Error("pr.yml apijson-differential downloads no pilot JSON exporter")
	}

	gate := prWorkflowStep(t, workflow, "build-and-test", "Verify required checks")
	if !strings.Contains(gate.Env["APIJSON_DIFFERENTIAL_RESULT"], "needs.apijson-differential.result") {
		t.Errorf("pr.yml Verify required checks APIJSON_DIFFERENTIAL_RESULT = %q, want the job's result", gate.Env["APIJSON_DIFFERENTIAL_RESULT"])
	}
	if !strings.Contains(gate.Run, `ok "$APIJSON_DIFFERENTIAL_RESULT"`) {
		t.Error("pr.yml Verify required checks does not require the apijson-differential result")
	}
	var needed bool
	for _, need := range workflow.Jobs["build-and-test"].needs() {
		needed = needed || need == "apijson-differential"
	}
	if !needed {
		t.Error("pr.yml build-and-test does not need apijson-differential, so its result is never seen")
	}
}

// The selector claims every path the baseline depends on, so a change to one of
// them runs the differential rather than only the Java-free provenance guard.
func TestChangedAreasClaimWhatMovesTheAPIJSONBaseline(t *testing.T) {
	raw, err := os.ReadFile("../../scripts/ci-changed-areas.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(raw)
	if !strings.Contains(script, "emit apijson ") {
		t.Fatal("scripts/ci-changed-areas.sh emits no apijson area")
	}
	for _, claimed := range []string{
		"internal/translate/export/",
		"tools/referee/apijson/",
		"tools/referee/baseline/",
		"scripts/pilot-json-exporter/",
		`scripts/pilot-pin\.sh$`,
		"docs/project/api-json-differential/",
	} {
		if !strings.Contains(script, claimed) {
			t.Errorf("scripts/ci-changed-areas.sh's apijson area does not claim %s", claimed)
		}
	}
}

// The fragments are recorded, not written: .gitattributes marks them generated so
// that GitHub collapses a re-record in a diff, as it did the monolithic baseline.
func TestFragmentsAreMarkedGenerated(t *testing.T) {
	raw, err := os.ReadFile("../../.gitattributes")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "docs/project/api-json-differential/**/*.json" && slices.Contains(fields[1:], "linguist-generated=true") {
			return
		}
	}
	t.Error(".gitattributes does not mark docs/project/api-json-differential/**/*.json linguist-generated=true")
}
