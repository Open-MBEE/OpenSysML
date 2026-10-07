package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis/modelform"
)

// graphsModel declares an action whose steps are typed and flow into one
// another, and a part def to ask for by mistake.
const graphsModel = `package Pipeline {
    private import ScalarValues::*;
    part def Product;
    action def Step { in x : Integer; out y : Integer; }
    action terrain {
        first start;
        action rad : Step;
        action mesh : Step;
        done;
        succession first start then rad;
        succession first rad then mesh;
        succession first mesh then done;
        flow rad.y to mesh.x;
    }
    state Machine {
        entry; then idle;
        state idle;
        state busy;
        succession first idle then busy;
    }
}
`

// The graph is the run's result on stdout; everything about the run is on stderr.
func TestGraphsWritesTheFormOnStdout(t *testing.T) {
	binary := buildCLI(t)

	for subject, want := range map[string]func(g *modelform.Graphs) bool{
		// terrain's own graph and that of Step, which its steps perform.
		"Pipeline::terrain": func(g *modelform.Graphs) bool { return len(g.Actions) == 2 && len(g.States) == 0 },
		"Pipeline::Machine": func(g *modelform.Graphs) bool { return len(g.Actions) == 0 && len(g.States) == 1 },
	} {
		got := runStreams(t, binary, graphsModel, "-graphs", subject)
		if got.status != exitHolds {
			t.Fatalf("%s: exit status = %d, want %d\n%s", subject, got.status, exitHolds, got.output())
		}
		var graphs modelform.Graphs
		if err := json.Unmarshal([]byte(got.stdout), &graphs); err != nil {
			t.Fatalf("%s: stdout is not graphs JSON: %v\n%s", subject, err, got.stdout)
		}
		if graphs.Version != modelform.GraphsVersion || graphs.Subject != subject || !want(&graphs) {
			t.Errorf("%s: version %d, subject %q, %d actions, %d states", subject, graphs.Version, graphs.Subject, len(graphs.Actions), len(graphs.States))
		}
		if !strings.Contains(got.stderr, "package Pipeline") {
			t.Errorf("%s: stderr does not say what the load declared:\n%s", subject, got.stderr)
		}
	}
	got := runStreams(t, binary, graphsModel, "-graphs", "Pipeline::terrain")
	if !strings.Contains(got.stdout, `"flows"`) {
		t.Errorf("the flow from rad.y to mesh.x is missing:\n%s", got.stdout)
	}
}

func TestGraphsWritesToTheOutputFile(t *testing.T) {
	binary := buildCLI(t)

	out := filepath.Join(t.TempDir(), "terrain.json")
	got := runStreams(t, binary, graphsModel, "-graphs", "Pipeline::terrain", "-o", out)
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	if got.stdout != "" {
		t.Errorf("stdout should be empty when -o names a file:\n%s", got.stdout)
	}
	if !strings.Contains(got.stderr, "wrote "+out) || !strings.Contains(got.stderr, "graphs:1") {
		t.Errorf("stderr should name the file written and its form, got:\n%s", got.stderr)
	}
	written, err := os.ReadFile(out) // #nosec G304 -- the test wrote this path.
	if err != nil {
		t.Fatal(err)
	}
	var graphs modelform.Graphs
	if err := json.Unmarshal(written, &graphs); err != nil {
		t.Fatalf("the file is not graphs JSON: %v\n%s", err, written)
	}
	if graphs.Subject != "Pipeline::terrain" || !strings.HasSuffix(string(written), "}\n") {
		t.Errorf("the file carries other than the form:\n%s", written)
	}
}

func TestGraphsRefusals(t *testing.T) {
	binary := buildCLI(t)

	tests := []struct {
		name   string
		args   []string
		status int
		want   string
	}{
		{"not a behavior", []string{"-graphs", "Pipeline::Product"}, exitUnevaluable, "no lowered graph"},
		{"unknown subject", []string{"-graphs", "Pipeline::Missing"}, exitUnevaluable, "Pipeline::Missing"},
		{"with render", []string{"-graphs", "Pipeline::terrain", "-render", "Pipeline::terrain"}, 2, "cannot be combined"},
		{"with render-all", []string{"-graphs", "Pipeline::terrain", "-render-all", t.TempDir()}, 2, "one per run"},
		{"with render-documents", []string{"-graphs", "Pipeline::terrain", "-render-documents", t.TempDir()}, 2, "one per run"},
		{"with convert", []string{"-graphs", "Pipeline::terrain", "-convert", "ttl"}, 2, "one per run"},
		{"with migrate", []string{"-graphs", "Pipeline::terrain", "-migrate", "sysml"}, 2, "mutually exclusive"},
		{"with query", []string{"-graphs", "Pipeline::terrain", "-query", "Pipeline"}, 2, "-graphs"},
		{"with check", []string{"-graphs", "Pipeline::terrain", "-self-check"}, 2, "decides nothing about the model"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := runStreams(t, binary, graphsModel, test.args...)
			if got.status != test.status {
				t.Fatalf("exit status = %d, want %d\n%s", got.status, test.status, got.output())
			}
			if got.stdout != "" {
				t.Errorf("a refused run wrote to stdout:\n%s", got.stdout)
			}
			if !strings.Contains(got.stderr, test.want) {
				t.Errorf("stderr does not mention %q:\n%s", test.want, got.stderr)
			}
		})
	}
}
