package modelform

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// parallelMachineFixture is a parallel state definition: each substate of the
// definition itself stands for an orthogonal region, so the lowered graph owns
// region vertices at the top of the machine, under no parent state.
func parallelMachineFixture(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", "parallel_machine.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// A parallel definition's region owners have no parent state; the export names
// them, numbers them and writes their regions as top regions rather than
// dereferencing the missing parent.
func TestGraphsStateOfAParallelDefinitionHasTopRegions(t *testing.T) {
	model, idx := graphsModelOf(t, parallelMachineFixture(t))
	for _, subject := range []string{"Orthogonal::Machine", "Orthogonal::Tug::modes"} {
		t.Run(subject, func(t *testing.T) {
			g, _ := exportGraphs(t, model, idx, subject)
			if len(g.States) != 1 || len(g.Actions) != 0 {
				t.Fatalf("%d states and %d actions, want one state machine", len(g.States), len(g.Actions))
			}
			form := g.States[0]
			if len(form.Regions) != 2 {
				t.Fatalf("regions = %d, want the two orthogonal regions", len(form.Regions))
			}
			hidden := 0
			for _, v := range form.Vertices {
				if v.Name == "" {
					t.Errorf("vertex %d (%s) has no name", v.ID, v.Kind)
				}
				if v.Hidden {
					hidden++
					if v.Parent != nil {
						t.Errorf("region owner %s has parent %d, want none at the top of the machine", v.Name, *v.Parent)
					}
					if v.HiddenRegion == nil {
						t.Errorf("region owner %s stands for no region", v.Name)
					}
				}
			}
			if hidden != 2 {
				t.Errorf("hidden region owners = %d, want one per region", hidden)
			}
			for _, r := range form.Regions {
				if !r.Top || r.State == nil {
					t.Errorf("region %s: top %v, state %v; want a top region standing in a vertex", r.Name, r.Top, r.State)
				}
			}
		})
	}
}

func TestGraphsParallelMachineMatchesTheGolden(t *testing.T) {
	model, idx := graphsModelOf(t, parallelMachineFixture(t))
	_, got := exportGraphs(t, model, idx, "Orthogonal::Machine")
	golden := filepath.Join("testdata", "graphs_parallel_machine.golden")
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatalf("write %s: %v", golden, err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read %s: %v", golden, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("graphs bytes differ from %s", golden)
	}
}
