package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderRunExampleMatchesGoldens(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "examples", "run-timeline", "run-timeline.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	binary := buildCLI(t)
	dir := t.TempDir()
	outputs := map[string]string{
		"example-timeline.text.golden":     filepath.Join(dir, "timeline.txt"),
		"example-timeline.mermaid.golden":  filepath.Join(dir, "timeline.mmd"),
		"example-timeline.plantuml.golden": filepath.Join(dir, "timeline.puml"),
		"example-sequence.text.golden":     filepath.Join(dir, "sequence.txt"),
		"example-sequence.mermaid.golden":  filepath.Join(dir, "sequence.mmd"),
		"example-sequence.plantuml.golden": filepath.Join(dir, "sequence.puml"),
	}
	args := []string{
		"-instantiate", "RunTimeline::mission",
		"-state", "RunTimeline::Sender::modes RunTimeline::mission.sender",
		"-state", "RunTimeline::Receiver::modes RunTimeline::mission.sender.receiver",
		"-advance", "6",
	}
	for golden, path := range outputs {
		kind := "timeline"
		if strings.Contains(golden, "sequence") {
			kind = "sequence"
		}
		args = append(args, "-render-run", kind+"="+path)
	}
	got := check(t, binary, string(data), args...)
	if got.status != 0 {
		t.Fatalf("exit status = %d\n%s", got.status, got.output())
	}
	for golden, path := range outputs {
		actual, err := os.ReadFile(path) // #nosec G304 -- the path is created by the test.
		if err != nil {
			t.Fatal(err)
		}
		goldenPath := filepath.Join("..", "..", "internal", "exec", "runtrace", "testdata", golden)
		want, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(actual) != string(want) {
			t.Errorf("%s differs from %s", path, goldenPath)
		}
	}
}

func TestRenderRunWritesEachFormAfterTheVerdict(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	timelinePath := filepath.Join(dir, "timeline.txt")
	sequencePath := filepath.Join(dir, "sequence.mmd")
	got := check(t, binary, behaviorModel,
		"-state", "Mission::Cycle", "-advance", "15",
		"-render-run", "timeline="+timelinePath,
		"-render-run", "sequence="+sequencePath,
	)
	if got.status != 0 {
		t.Fatalf("exit status = %d\n%s", got.status, got.output())
	}
	if !strings.Contains(got.stdout, `Started state machine executor for "Mission::Cycle"`) {
		t.Errorf("the run verdict is missing from stdout:\n%s", got.stdout)
	}
	if strings.Contains(got.stdout, "[trace]") {
		t.Errorf("silent recording printed trace lines:\n%s", got.stdout)
	}
	timeline, err := os.ReadFile(timelinePath) // #nosec G304 -- this path is created by the test.
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(timeline), "run - timeline rendering") ||
		!strings.Contains(string(timeline), "working") {
		t.Errorf("timeline output is missing the run's state occupancy:\n%s", timeline)
	}
	sequence, err := os.ReadFile(sequencePath) // #nosec G304 -- this path is created by the test.
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sequence), "sequenceDiagram") ||
		!strings.Contains(string(sequence), "the run recorded no message") {
		t.Errorf("sequence output is missing its empty-run rendering:\n%s", sequence)
	}
	if !strings.Contains(got.stderr, "wrote "+timelinePath) || !strings.Contains(got.stderr, "wrote "+sequencePath) {
		t.Errorf("stderr does not report both artifacts:\n%s", got.stderr)
	}
}

func TestRenderRunWritesStdoutWithoutLosingTheVerdict(t *testing.T) {
	binary := buildCLI(t)
	got := check(t, binary, behaviorModel,
		"-state", "Mission::Cycle", "-advance", "1",
		"-render-run", "timeline=-", "-render-form", "text",
	)
	if got.status != 0 {
		t.Fatalf("exit status = %d\n%s", got.status, got.output())
	}
	for _, want := range []string{`Started state machine executor for "Mission::Cycle"`, "run - timeline rendering", "waiting"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, got.stdout)
		}
	}
	if strings.Contains(got.stdout, "[trace]") {
		t.Errorf("silent recording printed trace lines:\n%s", got.stdout)
	}
}

func TestRenderRunRejectsUnsupportedModesAndForms(t *testing.T) {
	binary := buildCLI(t)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no behavior", []string{"-render-run", "timeline=run.txt"}, "needs -action, -state or -advance"},
		{"unknown kind", []string{"-state", "Mission::Cycle", "-render-run", "other=run.txt"}, "unknown -render-run kind"},
		{"unknown extension", []string{"-state", "Mission::Cycle", "-render-run", "timeline=run.svg"}, "use -render-form"},
		{"dot refused", []string{"-state", "Mission::Cycle", "-render-run", "timeline=run.dot"}, "not written as dot"},
		{"model rendering conflict", []string{"-state", "Mission::Cycle", "-render-run", "timeline=run.txt", "-render", "Demo::view"}, "cannot be combined with -render"},
		{"query conflict", []string{"-state", "Mission::Cycle", "-render-run", "timeline=run.txt", "-query", "sysml:name=*"}, "cannot be combined with -query"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := check(t, binary, behaviorModel, tc.args...)
			if got.status != 2 || !strings.Contains(got.stderr, tc.want) {
				t.Errorf("status = %d, want 2 with %q:\n%s", got.status, tc.want, got.output())
			}
		})
	}
}
