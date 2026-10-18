package llm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

func TestCorpusShape(t *testing.T) {
	m, err := LoadCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Check(); err != nil {
		t.Fatal(err)
	}
	if len(m.Tasks) != 50 {
		t.Fatalf("corpus has %d tasks, want 50", len(m.Tasks))
	}
	families := map[string]int{}
	difficulties := map[string]int{}
	for _, task := range m.Tasks {
		families[task.Family]++
		difficulties[task.Difficulty]++
		if task.Task == "" || task.Title == "" {
			t.Errorf("%s: empty task or title", task.ID)
		}
	}
	if len(m.Families) != 5 {
		t.Errorf("manifest names %d families, want 5", len(m.Families))
	}
	for f := range m.Families {
		if families[f] != 10 {
			t.Errorf("family %s has %d tasks, want 10", f, families[f])
		}
	}
	for _, d := range Difficulties {
		if difficulties[d] == 0 {
			t.Errorf("difficulty %s has no tasks", d)
		}
	}
}

// testToolchain finds the built sysml binary and, when provisioned, the pilot
// validator; it skips when the binary is absent unless CI requires it.
func testToolchain(t *testing.T) Toolchain {
	t.Helper()
	root, err := repo.Choose("")
	if err != nil {
		t.Skip(err)
	}
	tc := Toolchain{Sysml: filepath.Join(root, "bin", "sysml")}
	if _, err := os.Stat(tc.Sysml); err != nil {
		if os.Getenv("OPENSYSML_REQUIRE_LLM_BENCH_TOOLCHAIN") != "" {
			t.Fatalf("bin/sysml is required: %v", err)
		}
		t.Skipf("bin/sysml not built: %v (run make build)", err)
	}
	pilot := filepath.Join(root, "build", "pilot-sysml-validator", "validate-sysml-batch")
	if _, err := os.Stat(pilot); err == nil {
		tc.Pilot = pilot
		tc.PilotLibrary = filepath.Join(root, "internal", "workspace", "libs", "stdlib", "OpenSysML Libraries")
	} else if os.Getenv("OPENSYSML_REQUIRE_PILOT_VALIDATOR") != "" {
		t.Fatalf("the pilot validator is required: %v", err)
	}
	return tc
}

// TestCorpusGoldValidates asserts every gold model is clean under sysml
// -validate and, when the pinned pilot validator is provisioned, under it too.
func TestCorpusGoldValidates(t *testing.T) {
	tc := testToolchain(t)
	m, err := LoadCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if tc.Pilot == "" {
		t.Log("pilot validator not provisioned; checking sysml -validate only (scripts/download-pilot-sysml-validator.sh)")
	}
	ctx := context.Background()
	for _, task := range m.Tasks {
		task := task
		t.Run(task.ID, func(t *testing.T) {
			t.Parallel()
			src, err := m.GoldSource(task)
			if err != nil {
				t.Fatal(err)
			}
			v, err := tc.Validate(ctx, src)
			if err != nil {
				t.Fatal(err)
			}
			if !v.OK {
				t.Fatalf("sysml -validate rejects the gold: %v", v.Errors())
			}
			if tc.Pilot == "" {
				return
			}
			ok, out, err := tc.PilotAccepts(ctx, src)
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				t.Fatalf("pilot rejects the gold:\n%s", out)
			}
		})
	}
}
