package flexo

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// The live gate. It needs a running Flexo MMS stack, so it is opt-in exactly as
// the OMG corpus gates are: absent the stack it skips loudly, and it fails
// rather than skips once FLEXO_INTEROP says the stack is supposed to be there.
// .agents/skills/flexo-interop documents how to bring one up.

var updateExpectation = flag.Bool("update-flexo", false,
	"record the measured Flexo interop report as the new expectation")

// Each fixture is measured on its own: the original model, and the
// identity-carrying variant whose annotated UUIDs are the element ids.
var fixtures = []struct {
	name            string
	fixturePath     string
	referencePath   string
	expectationPath string
}{
	{
		name:            "model",
		fixturePath:     "testdata/model.sysml",
		referencePath:   "testdata/reference_changes.json",
		expectationPath: "testdata/interop_expected.txt",
	},
	{
		name:            "identity",
		fixturePath:     "testdata/identity_model.sysml",
		referencePath:   "testdata/identity_reference_changes.json",
		expectationPath: "testdata/identity_interop_expected.txt",
	},
}

const expectationHeader = `# What a real Flexo MMS stack does with this project's RDF, measured by
# TestFlexoInterop. This is a ratchet: every line is adjudicated, and a change to
# one is a change in interoperability, not a change in the harness.
#
# Regenerate against a running stack (see .agents/skills/flexo-interop):
#   FLEXO_INTEROP=1 FLEXO_INTEROP_TOKEN=<jwt> \
#     go test -count=1 ./internal/translate/interop/flexo -run TestFlexoInterop -update-flexo
#
# graph-load is the Turtle this project writes, loaded through Layer 1's graph
# endpoint. json-commit is the same model posted through the SysML v2 service's
# own commit path: the ground truth for what its read path can carry at all.
# Server-generated commit ids, timestamps and durations are not recorded.
`

// The apply gate's fixture pair and expectation.
const (
	applyFixturePath     = "testdata/identity_model.sysml"
	applyRevisionPath    = "testdata/identity_model_revised.sysml"
	applyExpectationPath = "testdata/identity_apply_expected.txt"
)

const applyExpectationHeader = `# What a real Flexo MMS stack does when the identity-keyed sync applies to it,
# measured by TestFlexoInteropApply. This is a ratchet: every line is
# adjudicated, and a change to one is a change in interoperability, not a change
# in the harness.
#
# Regenerate against a running stack (see .agents/skills/flexo-interop):
#   FLEXO_INTEROP=1 FLEXO_INTEROP_TOKEN=<jwt> \
#     go test -count=1 ./internal/translate/interop/flexo -run TestFlexoInterop -update-flexo
#
# initial applies the fixture to an empty branch; revision applies its revised
# sibling on top, refusing first without confirmed deletes; repository-changed
# diffs the revision again after a commit made behind the sync's back. Each
# round is read back at the commit the sync state recorded. Server-generated
# commit ids, timestamps and durations are not recorded.
`

// liveClient is the client of the stack the gate asked for, or the skip.
func liveClient(t *testing.T) (*Client, context.Context) {
	t.Helper()
	if os.Getenv(EnvGate) == "" {
		announceSkip(t, "the live Flexo interop gate is opt-in")
	}

	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("%s=%s but the stack is not configured: %v", EnvGate, os.Getenv(EnvGate), err)
	}
	client := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	t.Cleanup(cancel)
	if err := client.Reachable(ctx); err != nil {
		t.Fatalf("%s=%s but the stack does not answer: %v", EnvGate, os.Getenv(EnvGate), err)
	}
	return client, ctx
}

// checkExpectation compares a measured report with its recorded one, or
// re-records it under -update-flexo.
func checkExpectation(t *testing.T, path, got string) {
	t.Helper()
	if *updateExpectation {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Logf("recorded %s; review the diff", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(want) != got {
		t.Errorf("the stack no longer measures what %s records.\n"+
			"Review the difference, then re-record it with -update-flexo:\n%s",
			path, firstDifference(string(want), got))
	}
}

func TestFlexoInterop(t *testing.T) {
	client, ctx := liveClient(t)

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			model, err := os.ReadFile(fixture.fixturePath)
			if err != nil {
				t.Fatalf("read %s: %v", fixture.fixturePath, err)
			}
			reference, err := os.ReadFile(fixture.referencePath)
			if err != nil {
				t.Fatalf("read %s: %v", fixture.referencePath, err)
			}

			report, err := Measure(ctx, client, fixture.fixturePath, model, reference)
			if err != nil {
				t.Fatalf("measure the round trip: %v", err)
			}
			for _, finding := range report.Findings {
				t.Log(finding)
			}
			checkExpectation(t, fixture.expectationPath, report.Text(expectationHeader))
		})
	}
}

// TestFlexoInteropApply measures the sync's apply against the live stack: load,
// revision with a retained-id rename and gated deletes, then a refused conflict.
func TestFlexoInteropApply(t *testing.T) {
	client, ctx := liveClient(t)

	model, err := os.ReadFile(applyFixturePath)
	if err != nil {
		t.Fatalf("read %s: %v", applyFixturePath, err)
	}
	revised, err := os.ReadFile(applyRevisionPath)
	if err != nil {
		t.Fatalf("read %s: %v", applyRevisionPath, err)
	}

	report, err := MeasureApply(ctx, client, applyFixturePath, model, revised)
	if err != nil {
		t.Fatalf("measure the apply: %v", err)
	}
	for _, finding := range report.Findings {
		t.Log(finding)
	}
	checkExpectation(t, applyExpectationPath, report.Text(applyExpectationHeader))
}

// firstDifference reports the first line the recorded and measured reports
// disagree on, which is enough to see whether a change is movement or noise.
func firstDifference(want, got string) string {
	wantLines, gotLines := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		wantLine, gotLine := "<end of file>", "<end of file>"
		if i < len(wantLines) {
			wantLine = wantLines[i]
		}
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}
		if wantLine != gotLine {
			return fmt.Sprintf("line %d:\n recorded: %s\n measured: %s", i+1, wantLine, gotLine)
		}
	}
	return "no line differs"
}

// announceSkip skips on stderr as well as through the testing package, because
// a -v-less run hides skip reasons and a silently skipped gate proves nothing.
func announceSkip(t *testing.T, reason string) {
	t.Helper()
	fmt.Fprintf(os.Stderr, "\n!!! GATE NOT RUN: %s SKIPPED - %s.\n"+
		"!!! No claim about loading this project's RDF into Flexo MMS is measured by this run.\n"+
		"!!! Bring a stack up as .agents/skills/flexo-interop describes and re-run\n"+
		"!!!   FLEXO_INTEROP=1 FLEXO_INTEROP_TOKEN=<jwt> go test -count=1 ./internal/translate/interop/flexo\n"+
		"!!! With %s set, an absent stack fails instead of skipping.\n\n",
		t.Name(), reason, EnvGate)
	t.Skip("live Flexo stack not requested (set " + EnvGate + "=1)")
}
