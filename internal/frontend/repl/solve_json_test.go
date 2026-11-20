package repl

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
)

// quantityModel is an analysis case whose objective and attribute carry a unit.
const quantityModel = `
package Q {
	private import ISQ::*;
	private import SI::*;
	private import TradeStudies::*;

	analysis def MassBudget {
		attribute mass : MassValue;
		assert constraint { mass >= 780 [kg] }
		objective lightest : MinimizeObjective { subject :>> selectedAlternative; in calc :>> eval { mass } }
	}
}
`

// solveDocument runs a solving command under -json and decodes its one document.
func solveDocument(t *testing.T, s *Session, line string) SolveDocument {
	t.Helper()
	out := run(t, s, line)
	var doc SolveDocument
	dec := json.NewDecoder(strings.NewReader(out))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("%s wrote no JSON document: %v\n%s", line, err, out)
	}
	if dec.More() {
		t.Fatalf("%s wrote more than one document:\n%s", line, out)
	}
	return doc
}

// oneReport is the single report of a document.
func oneReport(t *testing.T, doc SolveDocument) SolveReport {
	t.Helper()
	if len(doc.Reports) != 1 {
		t.Fatalf("document holds %d reports, want 1", len(doc.Reports))
	}
	return doc.Reports[0]
}

// undecidedSolver puts a solver answering unknown on the solver path.
func undecidedSolver(t *testing.T) {
	t.Helper()
	script := t.TempDir() + "/z3-undecided"
	fake := "#!/bin/sh\nprintf 'unknown\\n(:reason-unknown \"incomplete\")\\n'\nwhile read -r line; do :; done\n"
	if err := os.WriteFile(script, []byte(fake), 0o700); err != nil { // #nosec G306 -- a test's own executable
		t.Fatalf("write fake solver: %v", err)
	}
	t.Setenv(solve.SolverEnv, script)
}

func TestOptimizeJSONReportsTheOptimumAndItsAssignment(t *testing.T) {
	requireOptimizingSolver(t)
	s := checkSession(t, optimizeModel)
	text := run(t, s, "%optimize Bounded")
	doc := solveDocument(t, s, "%optimize Bounded -json")
	if doc.Command != "optimize" {
		t.Errorf("command is %q, want optimize", doc.Command)
	}
	r := oneReport(t, doc)
	if r.Subject != "Bounded" || r.Status != SolveSat || r.Solver == "" {
		t.Errorf("report is %q %s by %q, want Bounded sat by a solver", r.Subject, r.Status, r.Solver)
	}
	if want := []SolveValue{{Name: "Trade::Bounded::size", Value: "3"}}; !reflect.DeepEqual(r.Values, want) {
		t.Errorf("values are %+v, want %+v", r.Values, want)
	}
	want := []SolveOptimum{{Objective: "smallest", Direction: "minimize", Expression: "size",
		Status: "attained", Value: "3", Feasible: "3"}}
	if !reflect.DeepEqual(r.Optima, want) {
		t.Errorf("optima are %+v, want %+v", r.Optima, want)
	}
	// The lines are the text report, but for the time the solver took.
	if got := strings.Join(r.Lines, "\n"); withoutTimings(got) != withoutTimings(text) {
		t.Errorf("lines differ from the text report:\n%s\n--\n%s", got, text)
	}
}

// withoutTimings drops the solver timing a report's heading carries.
func withoutTimings(s string) string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if i := strings.LastIndex(line, ", "); i >= 0 && strings.HasSuffix(line, "ms)") {
			line = line[:i] + ")"
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func TestOptimizeJSONSetsAQuantitysUnitApart(t *testing.T) {
	requireOptimizingSolver(t)
	s := checkSession(t, quantityModel)
	wants(t, run(t, s, "%optimize Q::MassBudget"), "minimize lightest = `mass`: 780000.0 [gram]")
	r := oneReport(t, solveDocument(t, s, "%optimize -json Q::MassBudget"))
	if want := []SolveValue{{Name: "Q::MassBudget::mass", Value: "780000.0", Unit: "gram", Dimension: "M"}}; !reflect.DeepEqual(r.Values, want) {
		t.Errorf("values are %+v, want %+v", r.Values, want)
	}
	if len(r.Optima) != 1 || r.Optima[0].Value != "780000.0" || r.Optima[0].Unit != "gram" || r.Optima[0].Dimension != "M" {
		t.Errorf("optima are %+v, want 780000.0 gram of dimension M", r.Optima)
	}
}

func TestOptimizeJSONKeepsStatusAndExplanation(t *testing.T) {
	requireOptimizingSolver(t)
	s := checkSession(t, optimizeModel)
	unsat := oneReport(t, solveDocument(t, s, "%optimize Impossible -json"))
	if unsat.Status != SolveUnsat || len(unsat.Values) != 0 || len(unsat.Optima) != 0 {
		t.Errorf("an impossible case reports %s with %+v %+v", unsat.Status, unsat.Values, unsat.Optima)
	}
	if text := run(t, s, "%optimize Impossible"); withoutTimings(strings.Join(unsat.Lines, "\n")) != withoutTimings(text) {
		t.Errorf("lines lose the explanation:\n%v\n--\n%s", unsat.Lines, text)
	}
	unbounded := oneReport(t, solveDocument(t, s, "%optimize Unbounded -json"))
	if unbounded.Status != SolveUnbounded || len(unbounded.Optima) != 1 || unbounded.Optima[0].Status != "unbounded" || unbounded.Optima[0].Value != "" {
		t.Errorf("an unbounded case reports %s with %+v", unbounded.Status, unbounded.Optima)
	}
	missing := oneReport(t, solveDocument(t, s, "%optimize Nope -json"))
	if missing.Status != SolveUnavailable || missing.Subject != "Nope" {
		t.Errorf("an unresolved name reports %q %s", missing.Subject, missing.Status)
	}
	wants(t, strings.Join(missing.Lines, "\n"), "error:", "Nope")
}

func TestOptimizeJSONReportsAnUndecidedAnswer(t *testing.T) {
	undecidedSolver(t)
	s := checkSession(t, optimizeModel)
	r := oneReport(t, solveDocument(t, s, "%optimize Bounded -json"))
	if r.Status != SolveUnknown || len(r.Optima) != 0 {
		t.Errorf("an undecided answer reports %s with %+v", r.Status, r.Optima)
	}
	wants(t, strings.Join(r.Lines, "\n"), "is undecided", "Reason: incomplete")
}

func TestConfigureJSONReportsTheSelections(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, familyModel)
	doc := solveDocument(t, s, "%configure Fam::fam all -json")
	if doc.Command != "configure" {
		t.Errorf("command is %q, want configure", doc.Command)
	}
	all := oneReport(t, doc)
	if all.Subject != "Fam::fam" || all.Status != SolveSat || all.Truncated {
		t.Errorf("report is %q %s truncated=%v", all.Subject, all.Status, all.Truncated)
	}
	got := map[string]bool{}
	for _, selection := range all.Selections {
		var parts []string
		for _, v := range selection {
			parts = append(parts, v.Name+"="+v.Value)
		}
		got[strings.Join(parts, " ")] = true
	}
	want := map[string]bool{
		"Fam::fam::engine=Fam::EngineChoices::e4 Fam::fam::trans=Fam::fam::trans::man":  true,
		"Fam::fam::engine=Fam::EngineChoices::e6 Fam::fam::trans=Fam::fam::trans::auto": true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("selections are %v, want %v", got, want)
	}

	cut := oneReport(t, solveDocument(t, s, "%configure Fam::fam -json all 1"))
	if len(cut.Selections) != 1 || !cut.Truncated {
		t.Errorf("a bound of one reports %d selections, truncated=%v", len(cut.Selections), cut.Truncated)
	}

	one := oneReport(t, solveDocument(t, s, "%configure Fam::fam trans=man -json"))
	wantOne := []SolveValue{
		{Name: "Fam::fam::engine", Value: "Fam::EngineChoices::e4"},
		{Name: "Fam::fam::trans", Value: "Fam::fam::trans::man"},
	}
	if len(one.Selections) != 1 || !reflect.DeepEqual(one.Selections[0], wantOne) {
		t.Errorf("the checked selection is %+v, want %+v", one.Selections, wantOne)
	}
}

func TestConfigureJSONKeepsStatusAndExplanation(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, familyModel)
	line := "%configure Fam::fam engine=e6 trans=man"
	r := oneReport(t, solveDocument(t, s, line+" -json"))
	if r.Status != SolveUnsat || len(r.Selections) != 0 || len(r.Values) != 0 {
		t.Errorf("a refused selection reports %s with %+v", r.Status, r.Selections)
	}
	if text := run(t, s, line); withoutTimings(strings.Join(r.Lines, "\n")) != withoutTimings(text) {
		t.Errorf("lines lose the explanation:\n%v\n--\n%s", r.Lines, text)
	}
	wants(t, strings.Join(r.Lines, "\n"), "In the conflict:", "Against:")
}

func TestConfigureJSONReportsAnUndecidedAnswer(t *testing.T) {
	undecidedSolver(t)
	s := checkSession(t, familyModel)
	r := oneReport(t, solveDocument(t, s, "%configure Fam::fam -json"))
	if r.Status != SolveUnknown || len(r.Selections) != 0 {
		t.Errorf("an undecided answer reports %s with %+v", r.Status, r.Selections)
	}
	wants(t, strings.Join(r.Lines, "\n"), "incomplete")
}

// The wire names are the contract: every report writes them, an empty list as [].
func TestSolveJSONFieldNamesAreStable(t *testing.T) {
	s := checkSession(t, optimizeModel)
	for _, line := range []string{"%optimize Nope -json", "%configure Nope -json"} {
		var doc map[string]any
		if err := json.Unmarshal([]byte(run(t, s, line)), &doc); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
		report := doc["reports"].([]any)[0].(map[string]any)
		want := map[string]any{"subject": "Nope", "status": "unavailable", "solver": "", "values": []any{}}
		for name, value := range want {
			if !reflect.DeepEqual(report[name], value) {
				t.Errorf("%s: %s is %#v, want %#v", line, name, report[name], value)
			}
		}
		if lines, ok := report["lines"].([]any); !ok || len(lines) == 0 {
			t.Errorf("%s: lines are %#v, want the error", line, report["lines"])
		}
		if len(report) != len(want)+1 {
			t.Errorf("%s: report writes %v", line, report)
		}
	}
}

func TestSolvingCommandsNameTheirUsage(t *testing.T) {
	s := NewSession()
	for line, usage := range map[string]string{
		"%optimize": optimizeUsage, "%optimize -json": optimizeUsage,
		"%configure": configureUsage, "%configure -json": configureUsage,
	} {
		if got := run(t, s, line); got != usage {
			t.Errorf("%s prints %q, want %q", line, got, usage)
		}
	}
}
