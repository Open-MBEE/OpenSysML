package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// tradeStudyModel declares trade studies run by TestRunTradeStudy: one that
// selects the lightest of three engines, two of them equally light, and one whose
// evaluation fails for an alternative.
const tradeStudyModel = `package Trade {
    private import ScalarValues::*;
    private import TradeStudies::*;
    part def Engine { attribute mass : Real; attribute cylinders : Integer; }
    part a : Engine { attribute :>> mass = 30.0; attribute :>> cylinders = 6; }
    part b : Engine { attribute :>> mass = 10.0; attribute :>> cylinders = 4; }
    part c : Engine { attribute :>> mass = 10.0; attribute :>> cylinders = 0; }
    analysis lightest : TradeStudy {
        subject : Engine[1..*] = (a, b, c);
        objective : MinimizeObjective;
        calc :>> evaluationFunction {
            in part e :>> alternative : Engine;
            return :>> result : Real = e.mass;
        }
        return part :>> selectedAlternative : Engine;
    }
    analysis perCylinder : TradeStudy {
        subject : Engine[1..*] = (a, c);
        objective : MinimizeObjective;
        calc :>> evaluationFunction {
            in part e :>> alternative : Engine;
            return :>> result : Real = e.mass / e.cylinders;
        }
        return part :>> selectedAlternative : Engine;
    }
    analysis perOffset : TradeStudy {
        subject : Engine[1..*] = (a, b);
        in attribute offset : Integer;
        objective : MinimizeObjective;
        calc :>> evaluationFunction {
            in part e :>> alternative : Engine;
            return :>> result : Real = e.mass / (e.cylinders - offset);
        }
        return part :>> selectedAlternative : Engine;
    }
}`

// tradeStudyEvaluation is one evaluation in the JSON report.
type tradeStudyEvaluation struct {
	Function  string   `json:"function"`
	Arguments []string `json:"arguments"`
	Result    string   `json:"result"`
	Error     string   `json:"error"`
	Selected  bool     `json:"selected"`
	Tied      bool     `json:"tied"`
}

// tradeStudyReport is the part of the JSON report a trade-study run fills.
type tradeStudyReport struct {
	Status string `json:"status"`
	Checks []struct {
		Status string `json:"status"`
		Values []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"values"`
		Evaluations []tradeStudyEvaluation `json:"evaluations"`
		Rows        []struct {
			Inputs []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"inputs"`
			Verdicts []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"verdicts"`
			Evaluations []tradeStudyEvaluation `json:"evaluations"`
			Error       string                 `json:"error"`
		} `json:"rows"`
	} `json:"checks"`
	Errors []string `json:"errors"`
}

// TestRunTradeStudy checks that a trade study run outside the prompt evaluates
// every alternative in subject order, reports the one selected and the tie, and
// keeps the evaluations made beside the error of one whose evaluation failed.
func TestRunTradeStudy(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, tradeStudyModel, "-analysis", "Trade::lightest"), 0,
		"✓ Trade::lightest",
		"selectedAlternative = Trade::b (object #2)",
		"objective tradeStudyObjective: satisfied",
		"evaluationFunction(Trade::a (object #1)) = 30.0",
		"evaluationFunction(Trade::b (object #2)) = 10.0 [selected]",
		"evaluationFunction(Trade::c (object #3)) = 10.0 [tied]")
	wantReport(t, check(t, binary, tradeStudyModel, "-analysis", "Trade::perCylinder"), 2,
		"sysml: analysis run failed: analysis Trade::perCylinder",
		"objective tradeStudyObjective: undecided",
		"evaluationFunction(Trade::a (object #1)) = 5.0",
		"evaluationFunction(Trade::c (object #2)): error: calc Trade::perCylinder::evaluationFunction: evaluating the returned expression: division by zero")

	got := check(t, binary, tradeStudyModel, "-json", "-analysis", "Trade::lightest")
	var report tradeStudyReport
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	if report.Status != "holds" || len(report.Checks) != 1 {
		t.Fatalf("report does not hold with one check:\n%s", got.stdout)
	}
	evals := report.Checks[0].Evaluations
	if len(evals) != 3 {
		t.Fatalf("report carries %d evaluations, want the three alternatives:\n%s", len(evals), got.stdout)
	}
	for i, want := range []struct {
		arg, result    string
		selected, tied bool
	}{
		{"Trade::a (object #1)", "30.0", false, false},
		{"Trade::b (object #2)", "10.0", true, false},
		{"Trade::c (object #3)", "10.0", false, true},
	} {
		e := evals[i]
		if e.Function != "Trade::lightest::evaluationFunction" || len(e.Arguments) != 1 || e.Arguments[0] != want.arg ||
			e.Result != want.result || e.Error != "" || e.Selected != want.selected || e.Tied != want.tied {
			t.Errorf("evaluation %d = %+v, want %+v", i, e, want)
		}
	}
	if v := report.Checks[0].Values[0]; v.Name != "selectedAlternative" || v.Value != "Trade::b (object #2)" {
		t.Errorf("selectedAlternative = %+v, want Trade::b", v)
	}

	got = check(t, binary, tradeStudyModel, "-json", "-analysis", "Trade::perCylinder")
	report = tradeStudyReport{}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	if report.Status != "unresolved" || len(report.Checks) != 1 || report.Checks[0].Status != "unresolved" {
		t.Fatalf("a failing alternative did not leave the run unresolved:\n%s", got.stdout)
	}
	evals = report.Checks[0].Evaluations
	if len(evals) != 2 || evals[0].Result != "5.0" || evals[0].Error != "" ||
		evals[1].Result != "" || !strings.HasSuffix(evals[1].Error, "division by zero") || evals[1].Selected || evals[1].Tied {
		t.Errorf("evaluations = %+v, want the first computed and the second failed", evals)
	}
}

// TestSweepTradeStudy checks that a swept trade study reports each run's
// evaluations on its row, in text and in JSON, and that a row whose run failed
// on one alternative keeps the evaluations made before it beside its error.
func TestSweepTradeStudy(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, tradeStudyModel, "-analysis", "Trade::perOffset", "-sweep", "offset=3..4")
	wantReport(t, got, 1,
		"sweep Trade::perOffset — 2 run(s)",
		"| evaluations ",
		"evaluationFunction(Trade::a (object #1)) = 10.0 [selected]; evaluationFunction(Trade::b (object #2)) = 10.0 [tied]",
		"tradeStudyObjective: undecided",
		"evaluationFunction(Trade::a (object #1)) = 15.0; evaluationFunction(Trade::b (object #2)): error: calc Trade::perOffset::evaluationFunction: evaluating the returned expression: division by zero")

	got = check(t, binary, tradeStudyModel, "-json", "-analysis", "Trade::perOffset", "-sweep", "offset=3..4")
	var report tradeStudyReport
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	if report.Status != "fails" || len(report.Checks) != 1 || len(report.Checks[0].Rows) != 2 {
		t.Fatalf("a table with a failed run does not fail with two rows:\n%s", got.stdout)
	}
	rows := report.Checks[0].Rows
	if ok := rows[0]; ok.Error != "" || len(ok.Inputs) != 1 || ok.Inputs[0].Value != "3" || len(ok.Evaluations) != 2 ||
		!ok.Evaluations[0].Selected || ok.Evaluations[0].Result != "10.0" || !ok.Evaluations[1].Tied {
		t.Errorf("completed row = %+v, want a selected at 10.0 and b tied", ok)
	}
	failed := rows[1]
	if !strings.HasSuffix(failed.Error, "division by zero") || len(failed.Verdicts) != 1 || failed.Verdicts[0].Value != "undecided" {
		t.Errorf("failed row = %+v, want its error and the objective undecided", failed)
	}
	if len(failed.Evaluations) != 2 || failed.Evaluations[0].Result != "15.0" || failed.Evaluations[0].Selected ||
		failed.Evaluations[1].Result != "" || !strings.HasSuffix(failed.Evaluations[1].Error, "division by zero") {
		t.Errorf("failed row evaluations = %+v, want a = 15.0 then b's division by zero", failed.Evaluations)
	}
}
