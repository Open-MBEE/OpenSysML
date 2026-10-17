package hygiene

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// orchestrationPkgs are the tool-defined orchestrations layered on the runtime
// (runtimePkg), each of which imports it and none of which it imports.
var orchestrationPkgs = []string{
	"github.com/Open-MBEE/OpenSysML/internal/exec/sweep",
}

// orchestrationPrefixes name the features kept out of the runtime; the files still
// there are listed in orchestrationRemaining, with the reasons in docs/internals/architecture.md.
var orchestrationPrefixes = []string{"check", "sweep", "montecarlo", "replay", "explore", "adopt"}

var orchestrationRemaining = map[string]bool{
	"adopt.go":               true,
	"adopt_frame.go":         true,
	"check.go":               true,
	"check_args.go":          true,
	"check_body.go":          true,
	"check_invocation.go":    true,
	"check_moves.go":         true,
	"check_reduce.go":        true,
	"check_replay.go":        true,
	"check_run.go":           true,
	"check_schedule.go":      true,
	"check_state.go":         true,
	"explore.go":             true,
	"explore_probability.go": true,
	"explore_queue.go":       true,
	"montecarlo.go":          true,
	"montecarlo_case.go":     true,
	"replay.go":              true,
}

func TestRuntimeImportsNoOrchestration(t *testing.T) {
	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}} {{join .Imports \" \"}}", runtimePkg)
	cmd.Args = append(cmd.Args, orchestrationPkgs...)
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	imports := map[string][]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		imports[fields[0]] = fields[1:]
	}
	if _, ok := imports[runtimePkg]; !ok {
		t.Fatalf("%s not listed", runtimePkg)
	}
	for _, pkg := range orchestrationPkgs {
		deps, ok := imports[pkg]
		if !ok {
			t.Errorf("%s not listed", pkg)
			continue
		}
		if !contains(deps, runtimePkg) {
			t.Errorf("%s does not import %s: the orchestration is layered on the runtime", pkg, runtimePkg)
		}
		if contains(imports[runtimePkg], pkg) {
			t.Errorf("%s imports %s: the runtime must not depend on an orchestration layered on it", runtimePkg, pkg)
		}
	}
}

func TestRuntimeGainsNoOrchestrationFile(t *testing.T) {
	dir := filepath.Join("..", "..", "internal", "exec", "runtime")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	present := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		for _, prefix := range orchestrationPrefixes {
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			present[name] = true
			if !orchestrationRemaining[name] {
				t.Errorf("%s: a %s* file is orchestration, which lives in its own package under internal/exec, not in the runtime", name, prefix)
			}
		}
	}
	for name := range orchestrationRemaining {
		if !present[name] {
			t.Errorf("%s has left the runtime: remove its orchestrationRemaining entry", name)
		}
	}
}
