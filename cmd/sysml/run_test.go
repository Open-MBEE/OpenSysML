package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// uncleanModel states a condition that holds and, after it, a declaration that
// does not analyse, so a check made on it would report success about a model the
// tool could not read.
const uncleanModel = `package Rover {
    constraint MassBudget { 180.0 <= 200.0 }
    part def Battery { attribute capacity = ; }
}
`

// behaviorModel states the behavior the run flags carry out: a calculation, an
// action that assigns, and a machine whose transitions are timed.
const behaviorModel = `package Mission {
    calc def Fall {
        in t;
        in g;
        g * t * t
    }

    action tally {
        attribute total = 0;
        first start;
        action accumulate {
            assign total := total + 5;
        }
        done;
        succession first start then accumulate;
        succession first accumulate then done;
    }

    state Cycle {
        entry; then init;
        state init;
        state waiting {
            accept after 10 [SI::s] then working;
        }
        state working {
            accept after 5 [SI::s] then done;
        }
        succession first init then waiting;
    }
}
`

// TestCheckGatesOnModelDiagnostics checks the contract a build step depends on:
// a model that did not analyse cleanly answers nothing, so a check made on it
// reports the diagnostics and exits 2 however the named condition came out.
func TestCheckGatesOnModelDiagnostics(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, uncleanModel, "-constraint", "Rover::MassBudget")
	wantReport(t, got, 2, "expected an expression", "did not analyse cleanly")

	// Redirecting the verdicts must not hide why there are none: the diagnostics
	// belong with the other reports of a run that could not be made.
	if strings.Contains(got.stdout, "expected an expression") {
		t.Errorf("diagnostics were reported on stdout:\n%s", got.stdout)
	}
	if got.stdout != "" {
		t.Errorf("stdout carries output for a check that was never made:\n%s", got.stdout)
	}
	if strings.Contains(got.output(), "Constraint Rover::MassBudget passed") {
		t.Errorf("an unclean model reported a verdict:\n%s", got.output())
	}
}

// TestCheckGatesOnEvaluationFailure checks that an expression the model could not
// evaluate stops the run, rather than being printed and passed over.
func TestCheckGatesOnEvaluationFailure(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, checkModel, "-constraint", "Rover::MassBudget", "-e", "nosuchfeature + 1")
	wantReport(t, got, 2, "unresolved reference: nosuchfeature")
	if !strings.Contains(got.stderr, "unresolved reference") {
		t.Errorf("a failed evaluation was not reported on stderr:\n%s", got.output())
	}
	if strings.Contains(got.stdout, "unresolved reference") {
		t.Errorf("a failed evaluation was reported on stdout:\n%s", got.stdout)
	}
}

// TestCheckGatesOnLiteralEvaluationFailure checks that an expression of literals
// alone that the prompt answers with its failure — an index naming no position —
// stops a check too, rather than being printed as though it evaluated.
func TestCheckGatesOnLiteralEvaluationFailure(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, checkModel, "-constraint", "Rover::MassBudget", "-e", "(1, 2, 3)#(7)")
	wantReport(t, got, 2, "evaluation failed")
	if strings.Contains(got.stdout, "Constraint Rover::MassBudget passed") {
		t.Errorf("a verdict was reported after an expression failed:\n%s", got.output())
	}
}

// TestAdvanceTakesADuration checks that a value that is not a duration to run
// for is reported, rather than running the machine for no time and holding.
func TestAdvanceTakesADuration(t *testing.T) {
	binary := buildCLI(t)

	// A flag written with no value at all is a misuse too, not no advance.
	wantReport(t, check(t, binary, behaviorModel, "-state", "Mission::Cycle", "-advance", ""), 2,
		"-advance takes a number of time units")

	for _, value := range []string{"NaN", "Inf", "-1"} {
		got := check(t, binary, behaviorModel, "-state", "Mission::Cycle", "-advance", value)
		wantReport(t, got, 2, "-advance takes a duration")
		if strings.Contains(got.stdout, "Started state machine executor") {
			t.Errorf("-advance %s ran the machine anyway:\n%s", value, got.output())
		}
	}

	// The misuse is reported in the form the caller asked for, so a build step
	// reading the JSON document has something to parse.
	got := check(t, binary, behaviorModel, "-state", "Mission::Cycle", "-advance", "soon", "-json")
	var report struct {
		Status string   `json:"status"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	if report.Status != "unresolved" || len(report.Errors) == 0 {
		t.Errorf("report does not say why the check was never made:\n%s", got.stdout)
	}
}

// TestValidate checks the diagnostics gate on its own: a model that analyses
// cleanly succeeds, and one that does not exits 2 with what analysis found.
func TestValidate(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, checkModel, "-validate"), 0, "no errors")
	wantReport(t, check(t, binary, uncleanModel, "-validate"), 2, "expected an expression")
}

// TestRunCalc checks that a calculation is invoked with the arguments given and
// reports what it computed, and that one that could not be invoked exits 2.
func TestRunCalc(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, behaviorModel, "-calc", "Mission::Fall(3, 2)"), 0, "= 18")
	// The prompt's spelling, arguments separated by spaces, is accepted too.
	wantReport(t, check(t, binary, behaviorModel, "-calc", "Mission::Fall 3 2"), 0, "= 18")
	wantReport(t, check(t, binary, behaviorModel, "-calc", "Mission::nosuch(1)"), 2, "unresolved reference: Mission::nosuch")
	wantReport(t, check(t, binary, behaviorModel, "-calc", "Mission::Fall(3)"), 2, `parameter "g" has no argument`)
}

// analysisModel declares an analysis case run by TestRunAnalysis: a usage binding
// its own subject, and a definition whose subject and inputs a run supplies.
const analysisModel = `package An {
    private import ScalarValues::*;
    part def Ship { attribute cost : Real default = 5.0; attribute other : Real = 7.0; }
    calc def Sum { in a : Real; in b : Real; return : Real = a + b; }
    analysis def Priced {
        subject s : Ship;
        in tax : Real;
        in limit : Real default = 100.0;
        objective { require constraint { total <= limit } }
        out total : Real = Sum(s.cost, s.other) * (1.0 + tax);
    }
    part ship : Ship;
    analysis shipCost : Priced { subject s = ship; in tax = 0.0; }
    part def Holder { analysis inner : Priced { subject s = h; in tax = 0.0; } part h : Ship; }
    requirement def Affordable { subject it : Ship; require constraint { it.cost < 6.0 } }
    part dear : Ship { attribute :>> cost = 9.0; }
    analysis def Checked { subject s : Ship; objective : Affordable { subject = s; } return r : Real = s.cost; }
    analysis cheap : Checked { subject s = ship; }
    analysis pricey : Checked { subject s = dear; }
    analysis def Unbound { subject s : Ship; objective : Affordable; return r : Real = s.cost; }
    analysis unbound : Unbound { subject s = ship; }
    requirement def CostCap { subject c : Real; require constraint { c < 6.0 } }
    analysis def Capped { subject s : Ship; objective : CostCap { subject = Capped::result; } s.cost }
    analysis capped : Capped { subject s = dear; }
}`

// TestRunAnalysis checks that an analysis case runs outside the prompt: from
// its own bindings, on an object as its subject with arguments for its inputs,
// as a feature of an object holding it; and that its objective's verdict is the
// run's, while one that could not be run exits 2.
func TestRunAnalysis(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, analysisModel, "-analysis", "An::shipCost"), 0,
		"✓ An::shipCost", "total = 12.0", "objective obj: satisfied")
	wantReport(t, check(t, binary, analysisModel, "-instantiate", "An::ship", "-analysis", "An::Priced(0.5) An::ship"), 0,
		`✓ An::Priced(0.5) on object #1 of "An::ship"`, "total = 18.0")
	wantReport(t, check(t, binary, analysisModel, "-instantiate", "An::ship", "-analysis", "An::Priced(tax = 0.5, limit = 10.0) An::ship"), 1,
		"✗ An::Priced(tax = 0.5, limit = 10.0)", "total = 18.0", "objective obj: not satisfied: total <= limit")
	wantReport(t, check(t, binary, analysisModel, "-instantiate", "An::Holder", "-analysis", "An::Holder::inner"), 0,
		"✓ An::Holder::inner", "total = 12.0")

	// An objective typed by a requirement def binds the def's subject by keyword
	// alone, and one binding none defaults to the case's result, of the wrong type here.
	wantReport(t, check(t, binary, analysisModel, "-analysis", "An::cheap"), 0,
		"✓ An::cheap", "r = 5.0", "objective obj: satisfied")
	wantReport(t, check(t, binary, analysisModel, "-analysis", "An::pricey"), 1,
		"✗ An::pricey", "r = 9.0", "objective obj: not satisfied: it.cost < 6.0")
	wantReport(t, check(t, binary, analysisModel, "-analysis", "An::unbound"), 2,
		"? An::unbound", "r = 5.0", "objective obj: undecided: objective obj: subject it defaults to the case's result (Cases::Case::obj): type mismatch: 5.0 (a Real) is not a Ship")
	wantReport(t, check(t, binary, analysisModel, "-analysis", "An::capped"), 1,
		"✗ An::capped", "result = 9.0", "objective obj: not satisfied: c < 6.0")

	wantReport(t, check(t, binary, analysisModel, "-analysis", "An::Priced(0.5)"), 2, "subject is unbound")
	wantReport(t, check(t, binary, analysisModel, "-analysis", "An::Sum"), 2, "not an analysis case")
	wantReport(t, check(t, binary, analysisModel, "-calc", "An::shipCost"), 2, "run it as an analysis case")
	wantReport(t, check(t, binary, analysisModel, "-analysis", "An::nosuch"), 2, "unresolved reference: An::nosuch")

	// The JSON report carries each output and verdict as a named value.
	got := check(t, binary, analysisModel, "-json", "-analysis", "An::shipCost")
	var report struct {
		Status string `json:"status"`
		Checks []struct {
			Values []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"values"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	if report.Status != "holds" || len(report.Checks) != 1 || len(report.Checks[0].Values) != 2 ||
		report.Checks[0].Values[0].Name != "total" || report.Checks[0].Values[0].Value != "12.0" ||
		report.Checks[0].Values[1].Name != "objective obj" || report.Checks[0].Values[1].Value != "satisfied" {
		t.Errorf("report does not carry the outputs and verdict:\n%s", got.stdout)
	}
}

// TestRunAction checks that an action runs to completion outside the prompt and
// reports the values it produced, and that a name that is not an action exits 2.
func TestRunAction(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, behaviorModel, "-action", "Mission::tally")
	wantReport(t, got, 0, "✓ Action completed", "total = 5")
	if strings.Contains(got.output(), "%step") {
		t.Errorf("a non-interactive run advertised prompt commands:\n%s", got.output())
	}

	wantReport(t, check(t, binary, behaviorModel, "-action", "Mission::Cycle"), 2, "is not an action")
	wantReport(t, check(t, binary, behaviorModel, "-action", "Mission::nosuch"), 2, "unresolved reference: Mission::nosuch")
}

// forkModel forks into two branches the library leaves unordered, so the
// scheduling policy decides which steps first.
const forkModel = `package Mission {
    private import ScalarValues::*;
    action race {
        attribute x : Integer = 0;
        first start;
        fork split;
        action left { assign x := 1; }
        action right { assign x := 2; }
        join sync;
        done;
        succession first start then split;
        succession first split then left;
        succession first split then right;
        succession first left then sync;
        succession first right then sync;
        succession first sync then done;
    }
}
`

// TestRunActionUnderSchedule checks that -schedule decides the order a run
// takes at a choice point, that the trace reports the order actually taken,
// and that a spelling naming no policy is refused at startup.
func TestRunActionUnderSchedule(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, forkModel, "-trace", "-action", "Mission::race")
	wantReport(t, got, 0, "took 3@right first", "x = 1")

	got = check(t, binary, forkModel, "-trace", "-schedule", "declared", "-action", "Mission::race")
	wantReport(t, got, 0, "took 2@left first", "x = 2")

	got = check(t, binary, forkModel, "-trace", "-schedule", "reverse", "-action", "Mission::race")
	wantReport(t, got, 0, "took 3@right first", "x = 1")

	seeded := check(t, binary, forkModel, "-trace", "-schedule", "seed:1", "-action", "Mission::race")
	wantReport(t, seeded, 0, "✓ Action completed")
	if again := check(t, binary, forkModel, "-trace", "-schedule", "seed:1", "-action", "Mission::race"); again.output() != seeded.output() {
		t.Errorf("seed:1 ran\n%s\nthen\n%s", seeded.output(), again.output())
	}

	for _, bad := range []string{"random", "seed:", "seed:-1", "seed:abc"} {
		got := check(t, binary, forkModel, "-schedule", bad, "-action", "Mission::race")
		wantReport(t, got, 2, `invalid scheduling policy "`+bad+`"`)
		if strings.Contains(got.output(), "Action completed") {
			t.Errorf("-schedule %s ran the action:\n%s", bad, got.output())
		}
	}
}

// monitorModel has two change-triggered transitions rising on one write, which
// the library leaves unordered: two runs, two final states.
const monitorModel = `package Watch {
    private import ScalarValues::*;
    state Monitor {
        attribute temp : Integer = 0;
        attribute route : Integer = 0;
        entry; then armed;
        state armed;
        state watching;
        state cool { entry { assign route := 1; } }
        state hot { entry { assign route := 2; } }
        transition first armed do assign temp := 30 then watching;
        transition first watching accept when temp > 20 then cool;
        transition first watching accept when temp > 25 then hot;
    }
}
`

// TestRunUnderExplore checks the sorted outcome table -schedule explore prints,
// its status line, exit code 2 on a budget hit, and the witness traces under -trace.
func TestRunUnderExplore(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, forkModel, "-schedule", "explore", "-action", "Mission::race")
	wantReport(t, got, 0, "✓ explored Mission::race: 2 outcomes",
		"outcome | linearizations | witness",
		"x = 1   | 1              | step 3: 3@right first of 2@left, 3@right",
		"x = 2   | 1              | step 3: 2@left first of 2@left, 3@right",
		"complete (2 runs)")
	if again := check(t, binary, forkModel, "-schedule", "explore", "-action", "Mission::race"); again.output() != got.output() {
		t.Errorf("explore ran\n%s\nthen\n%s", got.output(), again.output())
	}

	traced := check(t, binary, forkModel, "-trace", "-schedule", "explore:runs=8,depth=4", "-action", "Mission::race")
	wantReport(t, traced, 0, "complete (2 runs)", "trace of outcome 1's witness (run 2):", "took 3@right first",
		"trace of outcome 2's witness (run 1):", "took 2@left first")
	if before, _, _ := strings.Cut(traced.stdout, "complete (2 runs)"); strings.Contains(before, "[trace]") {
		t.Errorf("trace lines printed ahead of the table:\n%s", traced.output())
	}

	got = check(t, binary, forkModel, "-schedule", "explore:runs=1", "-action", "Mission::race")
	wantReport(t, got, 2, "? explored Mission::race: 1 outcome", "x = 2   | 1", "incomplete: runs budget 1 hit after 1 runs")
	got = check(t, binary, forkModel, "-schedule", "depth=0", "-action", "Mission::race")
	wantReport(t, got, 2, `invalid scheduling policy "depth=0"`)
	got = check(t, binary, forkModel, "-schedule", "explore:depth=0", "-action", "Mission::race")
	wantReport(t, got, 2, "incomplete: depth budget 0 hit after 1 runs")

	got = check(t, binary, monitorModel, "-schedule", "explore", "-state", "Watch::Monitor", "-advance", "0")
	wantReport(t, got, 0, "✓ explored Watch::Monitor: 2 outcomes",
		"finalState cool; visits armed, watching, cool; route = 1; temp = 30 | 1              | state watching on change -> 1->cool",
		"finalState hot; visits armed, watching, hot; route = 2; temp = 30   | 1              | state watching on change -> 2->hot",
		"complete (2 runs)")

	got = check(t, binary, behaviorModel, "-schedule", "explore", "-action", "Mission::tally")
	wantReport(t, got, 0, "✓ explored Mission::tally: 1 outcome", "total = 5 | 1              | no choice points", "complete (1 runs)")
	got = check(t, binary, behaviorModel, "-schedule", "explore", "-calc", "Mission::Fall(3, 2)")
	wantReport(t, got, 0, "✓ explored Mission::Fall: 1 outcome", "no choice points", "complete (1 runs)")
	got = check(t, binary, analysisModel, "-schedule", "explore", "-analysis", "An::shipCost")
	wantReport(t, got, 0, "✓ explored An::shipCost: 1 outcome", `objective obj = "satisfied"; total = 12.0`, "complete (1 runs)")

	for _, bad := range []string{"explore:", "explore:runs=0", "explore:depth=-1", "explore:bogus", "explore:runs=1,runs=2"} {
		got := check(t, binary, forkModel, "-schedule", bad, "-action", "Mission::race")
		wantReport(t, got, 2, `invalid scheduling policy "`+bad+`"`)
		if strings.Contains(got.output(), "explored") {
			t.Errorf("-schedule %s ran the action:\n%s", bad, got.output())
		}
	}
}

// TestJSONReportsExploration checks that the JSON report carries the outcomes
// an exploration reached and how it ended.
func TestJSONReportsExploration(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, forkModel, "-json", "-schedule", "explore:runs=1", "-action", "Mission::race")
	var report struct {
		Status string `json:"status"`
		Checks []struct {
			Status   string `json:"status"`
			Outcomes []struct {
				Values []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"values"`
				Linearizations int      `json:"linearizations"`
				Witness        []string `json:"witness"`
			} `json:"outcomes"`
			Exploration struct {
				Complete   bool     `json:"complete"`
				Runs       int      `json:"runs"`
				BudgetsHit []string `json:"budgetsHit"`
			} `json:"exploration"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	if got.status != 2 || report.Status != "unresolved" || len(report.Checks) != 1 {
		t.Fatalf("status = %d %q, checks = %d\n%s", got.status, report.Status, len(report.Checks), got.output())
	}
	c := report.Checks[0]
	if len(c.Outcomes) != 1 || c.Outcomes[0].Linearizations != 1 ||
		len(c.Outcomes[0].Values) != 1 || c.Outcomes[0].Values[0].Name != "x" || c.Outcomes[0].Values[0].Value != "2" ||
		strings.Join(c.Outcomes[0].Witness, ";") != "step 3: 2@left first of 2@left, 3@right" ||
		c.Exploration.Complete || c.Exploration.Runs != 1 || strings.Join(c.Exploration.BudgetsHit, ",") != "runs" {
		t.Errorf("report does not carry the exploration:\n%s", got.stdout)
	}
}

// TestRunStateMachine checks that a machine runs for the simulated time asked
// for, reporting the configuration it settled in.
func TestRunStateMachine(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, behaviorModel, "-state", "Mission::Cycle", "-advance", "20"),
		0, "✓ State machine completed", "Current state: done")

	// Without -advance the machine only takes its initial transition, so it is
	// reported where it starts rather than run to completion.
	started := check(t, binary, behaviorModel, "-state", "Mission::Cycle")
	wantReport(t, started, 0, "Current state: init")
	if strings.Contains(started.output(), "State machine completed") {
		t.Errorf("a machine ran without being advanced:\n%s", started.output())
	}

	// -advance 0 is a run to the current time, as %advance 0 is: what is due at
	// the start is dispatched, which the initial transition alone does not do.
	zero := check(t, binary, behaviorModel, "-state", "Mission::Cycle", "-advance", "0")
	wantReport(t, zero, 0, "✓ Advanced to 0.0")
	if zero.stdout == started.stdout {
		t.Errorf("-advance 0 reported the same run as no -advance at all:\n%s", zero.output())
	}

	wantReport(t, check(t, binary, behaviorModel, "-state", "Mission::tally", "-advance", "5"),
		2, "is not a state machine")
	wantReport(t, check(t, binary, behaviorModel, "-state", "Mission::Cycle", "-advance", "soon"),
		2, "-advance takes a number of time units")
}

// fleetModel states a part exhibiting a machine, both as a top-level usage and as
// a part nested in another, so -state has objects to reach by path and by id.
const fleetModel = `package Fleet {
    part def Rover {
        attribute level = 0;
        attribute log = "";
        exhibit state modes {
            entry; then waiting;
            state waiting {
                entry action w {
                    assign log := log + "W";
                    assign level := level + 10;
                }
                accept after 5 [SI::s] then moving;
            }
            state moving {
                entry action m {
                    assign log := log + "M";
                }
            }
        }
    }
    part def Driver {
        part r : Rover;
    }
    part rover : Rover;
    part driver : Driver;
}
`

// TestStateAttachesToTheExhibitedMachine checks that -state naming the machine an
// object exhibits attaches to the running machine instead of starting a second
// performance of it, and says so in the flag's spelling.
func TestStateAttachesToTheExhibitedMachine(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, fleetModel, "-instantiate", "Fleet::rover", "-state", "Fleet::Rover::modes Fleet::rover", "-advance", "5")
	wantReport(t, got, 0, `Debugging state machine "modes" exhibited by object #`, `already exhibits "Fleet::Rover::modes"`,
		"attaches to that running machine", "`%state Fleet::rover`", "Current state: moving")
	if strings.Contains(got.output(), "Started state machine executor") {
		t.Errorf("the exhibited machine was performed a second time:\n%s", got.output())
	}
}

// TestStateNamingTheMachineAloneDrivesItsExhibitor checks that -state naming only
// the machine one object exhibits drives that object's performance, and refuses
// when no object exhibits it rather than performing it detached.
func TestStateNamingTheMachineAloneDrivesItsExhibitor(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, fleetModel, "-instantiate", "Fleet::rover", "-state", "Fleet::Rover::modes", "-advance", "5")
	wantReport(t, got, 0, `Debugging state machine "modes" exhibited by object #`, `of "Fleet::rover"`, "Current state: moving")
	if strings.Contains(got.output(), "Started state machine executor") {
		t.Errorf("the exhibited machine was performed detached from its object:\n%s", got.output())
	}

	wantReport(t, check(t, binary, fleetModel, "-state", "Fleet::Rover::modes", "-advance", "5"),
		2, `no object of this session exhibits "Fleet::Rover::modes"`, "%state <object>", "%state Fleet::Rover::modes <object>")
}

// TestStateAddressesANestedPart checks that -state reaches a part nested in a
// top-level object by a feature path, by its qualified name and by the id the
// report prints, and that a path reaching no object names the segment.
func TestStateAddressesANestedPart(t *testing.T) {
	binary := buildCLI(t)

	byPath := check(t, binary, fleetModel, "-instantiate", "Fleet::driver", "-state", "Fleet::driver.r", "-advance", "5")
	wantReport(t, byPath, 0, `exhibited by object #`, `of "Fleet::driver.r"`, "Current state: moving")
	id := byPath.stdout[strings.Index(byPath.stdout, "object #")+len("object #"):]
	id = id[:strings.IndexAny(id, " \n")]

	wantReport(t, check(t, binary, fleetModel, "-instantiate", "Fleet::driver", "-state", "#"+id),
		0, `exhibited by object #`+id+"\n", "Current state: waiting")
	wantReport(t, check(t, binary, fleetModel, "-instantiate", "Fleet::driver", "-state", "Fleet::driver::r"),
		0, `exhibited by object #`+id+` of "Fleet::driver.r"`)
	wantReport(t, check(t, binary, fleetModel, "-instantiate", "Fleet::driver", "-state", "Fleet::Rover::modes Fleet::driver.r"),
		0, `exhibited by object #`+id+` of "Fleet::driver.r"`, "attaches to that running machine", "`%state Fleet::driver.r`")

	wantReport(t, check(t, binary, fleetModel, "-instantiate", "Fleet::driver", "-state", "Fleet::driver.x"),
		2, `Fleet::driver has no feature "x" (its features are r, and 13 more the library declares)`)
	wantReport(t, check(t, binary, fleetModel, "-instantiate", "Fleet::driver", "-state", "Fleet::driver.r.level"),
		2, "level of Fleet::driver.r holds a value (10), not an object")
	wantReport(t, check(t, binary, fleetModel, "-instantiate", "Fleet::driver", "-state", "#99"),
		2, "no object #99 in this session: nothing materialized has that identity (the objects are #1, #2")
}

// TestStateQualifiedPathDenotesTheUsageTyped checks that with both a definition and
// its usage instantiated, -state over the usage's qualified path reaches the usage's
// part, not the definition's, which the path's feature resolves to.
func TestStateQualifiedPathDenotesTheUsageTyped(t *testing.T) {
	binary := buildCLI(t)
	both := []string{"-instantiate", "Fleet::Driver", "-instantiate", "Fleet::driver"}

	usage := check(t, binary, fleetModel, append(both, "-state", "Fleet::driver::r")...)
	wantReport(t, usage, 0, `exhibited by object #`, `of "Fleet::driver.r"`)
	definition := check(t, binary, fleetModel, append(both, "-state", "Fleet::Driver::r")...)
	wantReport(t, definition, 0, `exhibited by object #`, `of "Fleet::Driver.r"`)
	if strings.Contains(usage.stdout, `of "Fleet::Driver.r"`) {
		t.Errorf("the usage's path reached the definition's part:\n%s", usage.output())
	}
}

// TestStateNamesTheUsageToInstantiate checks that -state asked about a usage whose
// definition alone was instantiated says so, naming the usage to instantiate, in
// the words the prompt uses for it.
func TestStateNamesTheUsageToInstantiate(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, fleetModel, "-state", "Fleet::Rover::modes Fleet::rover"),
		2, `no instance of "Fleet::rover" (use %instantiate first)`)

	got := check(t, binary, fleetModel, "-instantiate", "Fleet::Rover", "-state", "Fleet::Rover::modes Fleet::rover")
	wantReport(t, got, 2, `sysml: no instance of the usage "Fleet::rover"`,
		`of "Fleet::Rover" is of its definition "Fleet::Rover", not of the usage`,
		"use %instantiate Fleet::rover to create the usage's object", "or name Fleet::Rover to address it")

	wantReport(t, check(t, binary, fleetModel, "-instantiate", "Fleet::rover", "-state", "Fleet::Rover::modes Fleet::Rover"),
		2, `no instance of the definition "Fleet::Rover" itself`, `of "Fleet::rover" is typed by it`, "name Fleet::rover to address it")
}

// sharedMachineModel states a part exhibiting one state definition as two usages.
const sharedMachineModel = `package Shared {
    state def Blink {
        entry; then dark;
        state dark { accept after 2 [SI::s] then lit; }
        state lit;
    }
    part def Lamp {
        exhibit state front : Blink;
        exhibit state rear : Blink;
    }
    part lamp : Lamp;
}
`

// unmaterializablePartModel states a part whose value does not materialize.
const unmaterializablePartModel = `package Shared {
    part def Bulb;
    part def Lamp { part spare : Bulb = null; }
    part lamp : Lamp;
}
`

// TestStateOverASharedDefinitionNamesTheUsages checks that -state naming a
// definition the object exhibits twice refuses, naming the usages that address
// one machine, and that a segment failing to materialize reports the runtime's
// reason rather than a missing feature.
func TestStateOverASharedDefinitionNamesTheUsages(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, sharedMachineModel, "-instantiate", "Shared::lamp", "-state", "Shared::Blink Shared::lamp")
	wantReport(t, got, 2, `exhibits "Shared::Blink" as 2 machines`, "name the exhibited usage instead",
		"Shared::Lamp::front or Shared::Lamp::rear")
	for _, attached := range []string{"Debugging state machine", "Started state machine executor"} {
		if strings.Contains(got.output(), attached) {
			t.Errorf("a machine was selected despite the ambiguity:\n%s", got.output())
		}
	}
	wantReport(t, check(t, binary, sharedMachineModel, "-instantiate", "Shared::lamp", "-state", "Shared::Lamp::rear Shared::lamp", "-advance", "2"),
		0, `Debugging state machine "rear"`, "Current state: lit")

	got = check(t, binary, unmaterializablePartModel, "-instantiate", "Shared::lamp", "-state", "Shared::lamp.spare")
	wantReport(t, got, 2, "spare of Shared::lamp could not be materialized", "multiplicity violation")
	if strings.Contains(got.output(), `has no feature "spare"`) {
		t.Errorf("a feature that failed to materialize was reported missing:\n%s", got.output())
	}
}

// TestReportKeepsModelTextVerbatim checks that a value the model produced is
// reported byte for byte, in text and in JSON, even where it spells a prompt
// command the flags have another name for.
func TestReportKeepsModelTextVerbatim(t *testing.T) {
	binary := buildCLI(t)
	const model = `package Notes {
    private import ScalarValues::*;
    calc def Hint { "%state m %action a %instantiate x" }
}`
	const text = `"%state m %action a %instantiate x"`

	wantReport(t, check(t, binary, model, "-calc", "Notes::Hint()"), 0, "= "+text)

	got := check(t, binary, model, "-calc", "Notes::Hint()", "-json")
	var report struct {
		Checks []struct {
			Values []struct {
				Value string `json:"value"`
			} `json:"values"`
			Lines []string `json:"lines"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.stdout)
	}
	if len(report.Checks) != 1 || len(report.Checks[0].Values) != 1 {
		t.Fatalf("report carries no single value:\n%s", got.stdout)
	}
	if v := report.Checks[0].Values[0].Value; v != text {
		t.Errorf("value = %q, want %q", v, text)
	}
	if !strings.Contains(strings.Join(report.Checks[0].Lines, "\n"), text) {
		t.Errorf("lines rewrite the value:\n%s", strings.Join(report.Checks[0].Lines, "\n"))
	}
}

// TestJSONReport checks the document a build step parses: the status it exits
// with, every verdict decided, and the values a run produced.
func TestJSONReport(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, checkModel, "-satisfy", "-json")
	if got.status != 1 {
		t.Fatalf("exit status = %d, want 1\n%s", got.status, got.output())
	}
	var report struct {
		Status string `json:"status"`
		Exit   int    `json:"exit"`
		Checks []struct {
			Subject string `json:"subject"`
			Status  string `json:"status"`
			Values  []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"values"`
			Lines []string `json:"lines"`
		} `json:"checks"`
		Diagnostics []struct {
			Severity string `json:"severity"`
			Message  string `json:"message"`
			Line     int    `json:"line"`
		} `json:"diagnostics"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.stdout)
	}
	if report.Status != "fails" || report.Exit != 1 {
		t.Errorf("report status = %q exit = %d, want fails and 1", report.Status, report.Exit)
	}
	if len(report.Checks) != 2 {
		t.Fatalf("report carries %d checks, want the model's 2:\n%s", len(report.Checks), got.stdout)
	}
	if report.Checks[0].Status != "holds" || report.Checks[1].Status != "fails" {
		t.Errorf("verdicts = %q and %q, want holds then fails", report.Checks[0].Status, report.Checks[1].Status)
	}
	if len(report.Checks[0].Lines) == 0 {
		t.Error("a verdict carries no report of itself")
	}

	// A run's values are what a caller reads instead of the printed lines.
	got = check(t, binary, behaviorModel, "-calc", "Mission::Fall(3, 2)", "-json")
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.stdout)
	}
	if len(report.Checks) != 1 || len(report.Checks[0].Values) == 0 {
		t.Fatalf("the calculation reported no value:\n%s", got.stdout)
	}
	if value := report.Checks[0].Values[0]; value.Name != "result" || value.Value != "18" {
		t.Errorf("value = %s = %s, want result = 18", value.Name, value.Value)
	}

	// An unclean model is reported as data too, so a build step need not read the
	// printed diagnostics to know what stopped the check.
	got = check(t, binary, uncleanModel, "-constraint", "Rover::MassBudget", "-json")
	if got.status != 2 {
		t.Fatalf("exit status = %d, want 2\n%s", got.status, got.output())
	}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.stdout)
	}
	if report.Status != "unresolved" || len(report.Errors) == 0 {
		t.Errorf("report does not say the check was never made:\n%s", got.stdout)
	}
	if len(report.Diagnostics) == 0 || report.Diagnostics[0].Severity != "error" {
		t.Fatalf("report carries no error diagnostic:\n%s", got.stdout)
	}
	if report.Diagnostics[0].Line != 3 {
		t.Errorf("diagnostic reported at line %d, want the declaration's line 3", report.Diagnostics[0].Line)
	}
}

// warningModel analyses cleanly but states an expose where the spec constrains
// one, which analysis reports as a warning rather than an error.
const warningModel = `package Rover {
    constraint MassBudget { 180.0 <= 200.0 }
    part p;
    view def V { expose Rover::**; }
}
`

// TestJSONReportsWarningsOfACleanModel checks that what analysis found is
// reported as data whatever was checked, so a caller parsing the report reads
// the warnings the printed run shows.
func TestJSONReportsWarningsOfACleanModel(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, warningModel, "-constraint", "Rover::MassBudget", "-json")
	if got.status != 0 {
		t.Fatalf("exit status = %d, want 0 for a model whose findings are warnings\n%s", got.status, got.output())
	}
	var report struct {
		Diagnostics []struct {
			Severity string `json:"severity"`
			Message  string `json:"message"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.stdout)
	}
	if len(report.Diagnostics) == 0 {
		t.Fatalf("report carries no warning of a model analysis warned about:\n%s", got.stdout)
	}
	if report.Diagnostics[0].Severity != "warning" {
		t.Errorf("diagnostic severity = %q, want warning", report.Diagnostics[0].Severity)
	}
}

// TestWarningsPrintBeforeTheLoadTheyQualify checks the run's output reads in
// order: what analysis found about a model comes before the line reporting it
// loaded, so a success and its caveats are never read out of order.
func TestWarningsPrintBeforeTheLoadTheyQualify(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, warningModel, "-constraint", "Rover::MassBudget", "-json")
	if got.status != 0 {
		t.Fatalf("exit status = %d, want 0\n%s", got.status, got.output())
	}
	var report struct {
		Output []string `json:"output"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.stdout)
	}
	warning, loaded := -1, -1
	for i, line := range report.Output {
		if warning < 0 && strings.Contains(line, "warning:") {
			warning = i
		}
		if loaded < 0 && strings.Contains(line, "✓ package Rover") {
			loaded = i
		}
	}
	if warning < 0 || loaded < 0 {
		t.Fatalf("output misses the warning (%d) or the load line (%d):\n%s", warning, loaded, got.stdout)
	}
	if warning > loaded {
		t.Errorf("the warning prints after the load it qualifies:\n%s", got.stdout)
	}
}

// TestJSONLocatesADiagnosticInItsOwnFile checks that a finding is reported where
// its file has it, rather than at its line in the accumulated session buffer.
func TestJSONLocatesADiagnosticInItsOwnFile(t *testing.T) {
	binary := buildCLI(t)

	dir := t.TempDir()
	first := filepath.Join(dir, "first.sysml")
	second := filepath.Join(dir, "second.sysml")
	if err := os.WriteFile(first, []byte(checkModel), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte(uncleanModel), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binary, "-validate", "-json", first, second)
	out, err := cmd.Output()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("exit = %v, want status 2\n%s", err, out)
	}
	var report struct {
		Diagnostics []struct {
			File string `json:"file"`
			Line int    `json:"line"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, out)
	}
	if len(report.Diagnostics) == 0 {
		t.Fatalf("report carries no diagnostic:\n%s", out)
	}
	got := report.Diagnostics[0]
	if got.File != second {
		t.Errorf("diagnostic reported in %q, want the file it is in, %q", got.File, second)
	}
	if got.Line != 3 {
		t.Errorf("diagnostic reported at line %d, want line 3 of its own file", got.Line)
	}
}

// TestAdvanceWithoutBehavior checks that -advance with nothing to run it for
// is a misuse reported as such, rather than silently having no effect.
func TestAdvanceWithoutBehavior(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, behaviorModel, "-advance", "10"), 2, "-advance is the time a behavior runs for")
	wantReport(t, check(t, binary, behaviorModel, "-advance", "10", "-constraint", "Mission::Fall"), 2, "name one, as -action <name> or -state <name>")
}

// timedBehaviorModel states an action parked on the clock that then signals a
// machine, so -action and -state have one clock to share.
const timedBehaviorModel = `package Timed {
    private import ScalarValues::*;

    item def Ping;

    action pinger {
        attribute count : Integer = 0;
        first start;
        then action wait accept after 5 [SI::s];
        then action tick assign count := count + 1;
        then send new Ping() to listener;
        then done;
    }

    state listener {
        entry; then idle;
        state idle;
        transition idle_pinged first idle accept Ping then pinged;
        state pinged;
    }
}
`

// TestAdvanceRunsActionsAndStatesOnOneClock checks that -advance runs an action
// for the simulated time asked for, on its own or with a machine that accepts
// what the action sends, and reports an action the time did not complete.
func TestAdvanceRunsActionsAndStatesOnOneClock(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, timedBehaviorModel, "-action", "Timed::pinger", "-advance", "5"),
		0, "✓ Advanced to 5.0", "Action state: Completed", "✓ Action completed", "count = 1")

	short := check(t, binary, timedBehaviorModel, "-action", "Timed::pinger", "-advance", "2")
	wantReport(t, short, 2, "✓ Advanced to 2.0", "Action state: Waiting",
		"stopped at Waiting at simulation time 2.0 without completing", "for the clock to reach t=5.0", "run it with -advance 5.0 or more")

	both := check(t, binary, timedBehaviorModel, "-action", "Timed::pinger", "-state", "Timed::listener", "-advance", "10")
	wantReport(t, both, 0, "Started action executor", "Started state machine executor",
		"✓ Advanced to 10.0 (1 event(s) processed)", "✓ Action completed", "count = 1",
		"Current state: pinged", "Last event at: 5.0")

	// Without -advance the action runs to completion on its own, moving the clock
	// to its wait, and the machine only takes its initial transition.
	wantReport(t, check(t, binary, timedBehaviorModel, "-action", "Timed::pinger", "-state", "Timed::listener"),
		0, "✓ Action completed", "count = 1", "Current state: idle")
}

// dueTogetherModel has an action token and a state transition due at one instant
// of the clock they share, so which runs first is a choice point.
const dueTogetherModel = `package Due {
    private import SI::*;
    private import ScalarValues::*;

    part def Beacon {
        attribute lit : Boolean = false;
        exhibit state blinking {
            entry; then dark;
            state dark { accept after 5 [s] then shining; }
            state shining { entry assign lit := true; }
        }
    }

    part beacon : Beacon;

    action watcher {
        attribute sawLit : Boolean = false;
        first start;
        then action wait accept after 5 [s];
        then action look assign sawLit := beacon.lit;
        then done;
    }
}
`

// TestExploreAdvanceRunsBehaviorsOnOneClock checks that -schedule explore with
// -advance explores the behaviors named as one run on one clock: every order of
// the executors due at one instant is a linearization, and the outcome of the
// run is the outcomes of every behavior under its name.
func TestExploreAdvanceRunsBehaviorsOnOneClock(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, dueTogetherModel, "-schedule", "explore", "-instantiate", "Due::beacon",
		"-action", "Due::watcher", "-state", "Due::Beacon::blinking Due::beacon", "-advance", "5")
	wantReport(t, got, 0, "✓ explored Due::watcher, Due::Beacon::blinking: 2 outcomes",
		`Due::Beacon::blinking finalState = "shining"; Due::Beacon::blinking visits = "dark, shining"; Due::watcher.sawLit = false | 1              | t=5.0: action watcher first of action watcher, state machine blinking of object #1`,
		`Due::Beacon::blinking finalState = "shining"; Due::Beacon::blinking visits = "dark, shining"; Due::watcher.sawLit = true  | 1              | t=5.0: state machine blinking of object #1 first of action watcher, state machine blinking of object #1`,
		"complete (2 runs)")

	// Advanced short of the instant, neither is due: one outcome, no choice, and
	// the action that did not complete is the run's error.
	short := check(t, binary, dueTogetherModel, "-schedule", "explore", "-instantiate", "Due::beacon",
		"-action", "Due::watcher", "-state", "Due::Beacon::blinking Due::beacon", "-advance", "2")
	wantReport(t, short, 2, "? explored Due::watcher, Due::Beacon::blinking: 1 outcome",
		"action Due::watcher stopped at Waiting at simulation time 2.0 without completing", "no choice points", "complete (1 runs)")

	// One behavior explored with -advance is its own outcome, as without -advance.
	one := check(t, binary, timedBehaviorModel, "-schedule", "explore", "-action", "Timed::pinger", "-advance", "5")
	wantReport(t, one, 0, "✓ explored Timed::pinger: 1 outcome", "count = 1 | 1              | no choice points", "complete (1 runs)")
	both := check(t, binary, timedBehaviorModel, "-schedule", "explore", "-action", "Timed::pinger", "-state", "Timed::listener", "-advance", "10")
	wantReport(t, both, 0, "✓ explored Timed::pinger, Timed::listener: 1 outcome",
		`Timed::listener finalState = "pinged"; Timed::listener visits = "idle, pinged"; Timed::pinger.count = 1 | 1              | no choice points`)
}

// TestJSONWithoutCheck checks that -json alone is a misuse reported as such,
// rather than starting a prompt a build step cannot answer.
func TestJSONWithoutCheck(t *testing.T) {
	binary := buildCLI(t)
	wantReport(t, check(t, binary, checkModel, "-json"), 2, "-json reports a check")
}
