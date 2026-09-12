package solve

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// The portability harness asks whatever OPENSYSML_SMT names for one query per
// feature of the subset this layer emits, and reports each as
//
//	pass    the backend answered what the feature needs;
//	refuse  the backend lacks the capability and said so, which the layer reports
//	        as a typed error rather than degrading the question;
//	fail    the script was rejected, the answer was wrong, or the backend broke —
//	        our non-conformance to fix in the writer, not to excuse here.
//
// Run it against a backend with:
//
//	OPENSYSML_SMT=cvc5 go test ./internal/core/solve -run TestPortability -v

// outcome is what a portability case reported.
type outcome int

const (
	outcomePass outcome = iota
	outcomeRefuse
	outcomeFail
)

// String names the outcome as the report prints it.
func (o outcome) String() string {
	switch o {
	case outcomePass:
		return "pass"
	case outcomeRefuse:
		return "refuse"
	}
	return "fail"
}

// portabilityCase is one feature of the subset: the capabilities a backend needs
// for it, and the query and operation that exercise it.
type portabilityCase struct {
	// feature names the SMT-LIB feature the case exercises.
	feature string

	// needs are the capabilities the case cannot run without, so a backend
	// lacking one is expected to refuse rather than answer.
	needs []Capability

	// run asks the backend, returning what it answered or why it did not.
	run func(t *testing.T, solver *Solver) error
}

// portabilityCases are the designated portability subset: one case per feature the
// writer emits, each translated from the notation so the script under test is the
// one the layer really produces.
func portabilityCases() []portabilityCase {
	return []portabilityCase{{
		feature: "linear integer arithmetic and a model",
		needs:   []Capability{CapModels},
		run: func(t *testing.T, solver *Solver) error {
			q := constraintQuery(t, `
				package test {
					private import ScalarValues::*;
					constraint def Window {
						in start : Integer;
						assert constraint { start + 2 <= 9 }
					}
				}`, "test::Window")
			return wantSat(solver, q, "test::Window::start")
		},
	}, {
		feature: "linear real arithmetic and a model",
		needs:   []Capability{CapModels},
		run: func(t *testing.T, solver *Solver) error {
			q := constraintQuery(t, `
				package test {
					private import ScalarValues::*;
					constraint def Descent {
						in rate : Real;
						assert constraint { rate * 2.0 <= 4.5 }
					}
				}`, "test::Descent")
			return wantSat(solver, q, "test::Descent::rate")
		},
	}, {
		feature: "truncating integer remainder (div and mod)",
		needs:   []Capability{CapModels, CapIntegerDivision},
		run: func(t *testing.T, solver *Solver) error {
			q := constraintQuery(t, `
				package test {
					private import ScalarValues::*;
					constraint def Split {
						in total : Integer;
						assert constraint { total % 5 == -3 and total >= -5 and total <= -1 }
					}
				}`, "test::Split")
			result, err := solver.Solve(context.Background(), q)
			if err != nil {
				return err
			}
			if result.Status != StatusSat {
				return fmt.Errorf("answered %s, want sat", result.Status)
			}
			// -3 is the only integer in [-5, -1] whose truncating remainder by 5
			// is -3 (its Euclidean remainder is 2), so the backend agrees with
			// the evaluator or it does not.
			for _, a := range result.Model {
				if a.Var.Name == "test::Split::total" && a.Value != "-3" {
					return fmt.Errorf("assigned total = %s, want -3", a.Value)
				}
			}
			return nil
		},
	}, {
		feature: "nonlinear arithmetic",
		needs:   []Capability{CapNonlinearArith},
		run: func(t *testing.T, solver *Solver) error {
			q := constraintQuery(t, `
				package test {
					private import ScalarValues::*;
					constraint def Area {
						in side : Integer;
						in other : Integer;
						assert constraint { side * other == 6 }
					}
				}`, "test::Area")
			result, err := solver.Solve(context.Background(), q)
			if err != nil {
				return err
			}
			// A backend may give up on nonlinear arithmetic; saying so is an answer.
			if result.Status == StatusUnsat {
				return fmt.Errorf("answered unsat, want sat or unknown")
			}
			return nil
		},
	}, {
		feature: "mixed integer and real arithmetic",
		needs:   []Capability{CapMixedArith},
		run: func(t *testing.T, solver *Solver) error {
			q := constraintQuery(t, `
				package test {
					private import ScalarValues::*;
					constraint def Mixed {
						in count : Integer;
						in ratio : Real;
						assert constraint { count <= ratio }
					}
				}`, "test::Mixed")
			result, err := solver.Solve(context.Background(), q)
			if err != nil {
				return err
			}
			if result.Status == StatusUnsat {
				return fmt.Errorf("answered unsat, want sat or unknown")
			}
			return nil
		},
	}, {
		feature: "the strings theory",
		needs:   []Capability{CapModels, CapStrings, CapNonStandardLogic},
		run: func(t *testing.T, solver *Solver) error {
			q := constraintQuery(t, `
				package test {
					private import ScalarValues::*;
					constraint def Phase {
						in phase : String;
						assert constraint { phase == "descent" }
					}
				}`, "test::Phase")
			return wantSat(solver, q, "test::Phase::phase")
		},
	}, {
		feature: "algebraic datatypes (declare-datatypes)",
		needs:   []Capability{CapModels, CapDatatypes, CapNonStandardLogic},
		run: func(t *testing.T, solver *Solver) error {
			q := constraintQuery(t, `
				package test {
					enum def Finish { enum polished; enum brushed; }
					part def Ring { attribute finish : Finish;
						assert constraint chosen { finish == Finish::polished }
					}
				}`, "test::Ring::chosen")
			return wantSat(solver, q, "test::Ring::finish")
		},
	}, {
		feature: "named assertions and an unsat core",
		needs:   []Capability{CapUnsatCores},
		run: func(t *testing.T, solver *Solver) error {
			q := constraintQuery(t, `
				package test {
					private import ScalarValues::*;
					constraint def Conflict {
						in start : Integer;
						assert constraint { start >= 5 }
						assert constraint { start <= 2 }
					}
				}`, "test::Conflict")
			result, err := solver.Explain(context.Background(), q)
			if err != nil {
				return err
			}
			if result.Status != StatusUnsat {
				return fmt.Errorf("answered %s, want unsat", result.Status)
			}
			if result.Core == nil || len(result.Core.Members) == 0 {
				return errors.New("answered unsat with no core")
			}
			return nil
		},
	}, {
		feature: "an incremental dialogue (configuration enumeration)",
		needs:   []Capability{CapModels, CapIncremental, CapDatatypes, CapNonStandardLogic},
		run: func(t *testing.T, solver *Solver) error {
			q, _ := variantQuery(t, "test::ringFamily::variantsAgree")
			result, err := solver.Configurations(context.Background(), q, 0)
			if err != nil {
				return err
			}
			if result.Status != StatusSat || result.Truncated {
				return fmt.Errorf("answered %s (truncated %t), want every configuration", result.Status, result.Truncated)
			}
			if len(result.Solutions) != 3 {
				return fmt.Errorf("reported %d configurations, want 3", len(result.Solutions))
			}
			return nil
		},
	}, {
		feature: "a datatype the query declares itself (nodes of a flow)",
		needs:   []Capability{CapModels, CapDatatypes, CapNonStandardLogic},
		run: func(t *testing.T, solver *Solver) error {
			q, vars := stepQuery()
			result, err := solver.Solve(context.Background(), q)
			if err != nil {
				return err
			}
			if result.Status != StatusSat {
				return fmt.Errorf("answered %s, want sat", result.Status)
			}
			for _, a := range result.Model {
				if a.Var != vars[0] {
					continue
				}
				at, err := DecodeValue(a)
				if err != nil {
					return fmt.Errorf("the node read back as %s: %v", a.Raw, err)
				}
				if at.Text != "middle" && at.Text != "end" {
					return fmt.Errorf("the node read back as %s, want a value of the datatype", at.Text)
				}
				return nil
			}
			return fmt.Errorf("the model assigns no node: %+v", result.Model)
		},
	}, {
		feature: "an incremental dialogue over named variables (enumerating moves)",
		needs:   []Capability{CapModels, CapIncremental, CapDatatypes, CapNonStandardLogic},
		run: func(t *testing.T, solver *Solver) error {
			q, vars := stepQuery()
			result, err := solver.Enumerate(context.Background(), q, vars, 10)
			if err != nil {
				return err
			}
			if result.Status != StatusSat || result.Truncated {
				return fmt.Errorf("answered %s (truncated %t), want every move", result.Status, result.Truncated)
			}
			if len(result.Solutions) != 2 {
				return fmt.Errorf("reported %d moves, want 2", len(result.Solutions))
			}
			return nil
		},
	}, {
		feature: "objective optimization (minimize with :opt.priority)",
		needs:   []Capability{CapModels, CapOptimization, CapOptimizationPriority},
		run: func(t *testing.T, solver *Solver) error {
			q := analysisQuery(t, "MassBudget")
			result, err := solver.Optimize(context.Background(), q)
			if err != nil {
				return err
			}
			if result.Status != StatusSat {
				return fmt.Errorf("answered %s, want sat", result.Status)
			}
			// The lightest mass the conditions permit is 10 kg, in the base
			// magnitude the query is translated to.
			if len(result.Optima) != 1 || result.Optima[0].Status != OptimumAttained {
				return fmt.Errorf("reported %d optima, want one attained", len(result.Optima))
			}
			if got := result.Optima[0].Value; got != "10000.0 [gram]" {
				return fmt.Errorf("the optimum is %q, want 10000.0 [gram]", got)
			}
			return nil
		},
	}}
}

// wantSat asks for a verdict of sat with the named variable assigned in the
// notation's own terms, which is what a model is read for.
func wantSat(solver *Solver, q *Query, assigned string) error {
	result, err := solver.Solve(context.Background(), q)
	if err != nil {
		return err
	}
	if result.Status != StatusSat {
		return fmt.Errorf("answered %s (reason %q), want sat", result.Status, result.Reason)
	}
	for _, a := range result.Model {
		if a.Var.Name != assigned {
			continue
		}
		if !a.Rendered {
			return fmt.Errorf("assigned %s the raw SMT-LIB term %s", assigned, a.Raw)
		}
		return nil
	}
	return fmt.Errorf("no model value for %s", assigned)
}

// TestPortability reports how the backend OPENSYSML_SMT names fares on each
// feature of the subset. A capability it lacks is a reported refusal; a script it
// rejects, or an answer that is wrong, fails.
func TestPortability(t *testing.T) {
	solver := requireSolver(t)
	caps, err := solver.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("probe %s: %v", solver.Name, err)
	}
	t.Logf("%s (%s) probed in %s", solver.Name, solver.Path, caps.Elapsed)
	for _, capability := range AllCapabilities {
		switch {
		case caps.Supports(capability):
			t.Logf("  capability %-22s supported", capability)
		case caps.Refuses(capability):
			t.Logf("  capability %-22s refused: %s", capability, caps.Detail(capability))
		default:
			t.Logf("  capability %-22s undetermined: %s", capability, caps.Detail(capability))
		}
	}

	report := map[string]outcome{}
	for _, c := range portabilityCases() {
		t.Run(c.feature, func(t *testing.T) {
			got, detail := runPortabilityCase(t, solver, caps, c)
			report[c.feature] = got
			switch got {
			case outcomePass:
				t.Logf("pass: %s", c.feature)
			case outcomeRefuse:
				t.Logf("refuse: %s lacks a capability this needs: %s", solver.Name, detail)
			case outcomeFail:
				t.Errorf("fail: %s", detail)
			}
		})
	}
	logPortabilityReport(t, solver, report)
}

// runPortabilityCase runs one case and classifies what came back: a refusal the
// probe predicted, an answer, or our own non-conformance.
func runPortabilityCase(t *testing.T, solver *Solver, caps *Capabilities, c portabilityCase) (outcome, string) {
	t.Helper()
	refused := caps.Missing(c.needs)
	err := c.run(t, solver)
	switch {
	case err == nil && len(refused) == 0:
		return outcomePass, ""
	case err == nil:
		// The probe said the backend refuses something the case needs, yet the case
		// was answered: the check is wrong about the backend, which is ours to fix.
		return outcomeFail, fmt.Sprintf("%s answered although its check reported it refuses %v", solver.Name, refused)
	case reportedRefusal(err):
		if len(refused) == 0 {
			return outcomeFail, fmt.Sprintf("refused although its checks reported every capability the case needs: %v", err)
		}
		return outcomeRefuse, err.Error()
	case len(refused) > 0:
		return outcomeFail, fmt.Sprintf("%s refuses %v, which should have stopped the query before it ran: %v", solver.Name, refused, err)
	}
	return outcomeFail, err.Error()
}

// reportedRefusal reports whether the error is a backend refusing a feature it
// does not implement; optimization refuses with its own error, being no SMT-LIB
// feature at all.
func reportedRefusal(err error) bool {
	return Unsupported(err) || errors.Is(err, ErrNoOptimization)
}

// logPortabilityReport prints the per-feature table, so a run against a new
// backend is readable without reading the failures.
func logPortabilityReport(t *testing.T, solver *Solver, report map[string]outcome) {
	t.Helper()
	features := make([]string, 0, len(report))
	for feature := range report {
		features = append(features, feature)
	}
	sort.Strings(features)
	var b strings.Builder
	fmt.Fprintf(&b, "portability of %s (%s):\n", solver.Name, solver.Path)
	for _, feature := range features {
		fmt.Fprintf(&b, "  %-6s %s\n", report[feature], feature)
	}
	t.Log(strings.TrimSuffix(b.String(), "\n"))
}

// TestPortabilityGateIsRequired: with a solver declared mandatory the harness
// cannot pass by not running, which is how CI keeps it from silently skipping.
func TestPortabilityGateIsRequired(t *testing.T) {
	if os.Getenv(solverRequiredEnv) == "" {
		t.Skipf("%s is unset, so the harness may skip for want of a solver", solverRequiredEnv)
	}
	if _, err := Discover(); err != nil {
		t.Fatalf("%s is set but no solver was found: %v", solverRequiredEnv, err)
	}
	if len(portabilityCases()) == 0 {
		t.Fatal("the portability subset is empty")
	}
}
