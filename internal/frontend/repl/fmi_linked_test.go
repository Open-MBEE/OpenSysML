//go:build !sysml_re && !sysml_nofmi

package repl

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
)

// %engines tables every registered engine in name order with its authority, the
// questions it answers and the state of its process.
func TestEnginesListsEveryRegisteredEngine(t *testing.T) {
	s := loadSource(t, engineCalcSource)
	out := run(t, s, "%engines")
	wantsInOrder(t, out, "engine", "authority", "answers", "status",
		"check", "explore", "run", "smt", "solve", "sweep", "tool:fmi")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 9 {
		t.Fatalf("expected a header, six built-in engines, tool:fmi and its origin line, got:\n%s", out)
	}
	wants(t, lines[1], "check", "bounded", "outcomes, holds", "ready")
	wants(t, lines[2], "explore", "proved", "outcomes", "ready")
	wants(t, lines[3], "run", "observed", "evaluate", "ready")
	wants(t, lines[4], "smt", "proved", "holds")
	wants(t, lines[5], "solve", "proved", "satisfiable")
	wants(t, lines[6], "sweep", "observed", "sweep", "ready")
	wants(t, lines[7], "tool:fmi", "tool", "fmi/1", "observed", "compute", "OPENSYSML_FMI_RUNNER")
}

// %engine shows the selection, sets it by name, to auto or to all, and reports a
// name no engine carries as a typed error that leaves the selection in force.
func TestEngineShowsAndSetsTheSelection(t *testing.T) {
	s := loadSource(t, engineCalcSource)
	wants(t, run(t, s, "%engine"), "engine: auto")
	wants(t, run(t, s, "%engine run"), "engine: run")
	if got := s.Engine(); got != analysis.Only("run") {
		t.Fatalf("Engine() = %v, want run", got)
	}
	out := run(t, s, "%engine nope")
	wants(t, out, `error: analysis: no engine named "nope"`, "check, explore, run, smt, solve, sweep, tool:fmi, or auto, or all")
	rejects(t, out, "engine: ")
	wants(t, run(t, s, "%engine"), "engine: run")
	wants(t, run(t, s, "%engine all"), "engine: all")
	wants(t, run(t, s, "%engine auto"), "engine: auto")

	var unknown *analysis.UnknownEngineError
	if err := s.SetEngine("nope"); !errors.As(err, &unknown) || unknown.Name != "nope" {
		t.Fatalf("SetEngine(nope) = %v, want UnknownEngineError", err)
	}
}
