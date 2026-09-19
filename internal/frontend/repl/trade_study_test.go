package repl

import (
	"strings"
	"testing"
)

// tradeStudyModel is a trade study over two listed engines whose evaluation
// weighs power against mass by a case parameter, so a sweep over the weight
// changes which alternative the library's maximize selects.
const tradeStudyModel = `package Trade {
	private import ScalarValues::*;
	private import TradeStudies::*;

	part def Engine { attribute mass : Real; attribute power : Real; }
	part strong : Engine { attribute :>> mass = 30.0; attribute :>> power = 300.0; }
	part light : Engine { attribute :>> mass = 10.0; attribute :>> power = 50.0; }

	analysis weighted : TradeStudy {
		subject : Engine[1..*] = (strong, light);
		in attribute powerWeight : Real;
		objective : MaximizeObjective;
		calc :>> evaluationFunction {
			in part e :>> alternative : Engine;
			return :>> result : Real = e.power * powerWeight - e.mass;
		}
		return part :>> selectedAlternative : Engine;
	}

	analysis failing : TradeStudy {
		subject : Engine[1..*] = (strong, light);
		in attribute divisor : Real;
		objective : MinimizeObjective;
		calc :>> evaluationFunction {
			in part e :>> alternative : Engine;
			return :>> result : Real = e.mass / (e.power * divisor - 50.0);
		}
		return part :>> selectedAlternative : Engine;
	}
}`

func tradeStudySession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(tradeStudyModel).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	return s
}

// %analysis of a trade study reports the selected alternative, the objective's
// verdict, and one line per evaluation of an alternative in subject order.
func TestAnalysisOfATradeStudyReportsEachAlternative(t *testing.T) {
	s := tradeStudySession(t)
	got := run(t, s, "%analysis Trade::weighted(powerWeight = 0.1)")
	want := strings.Join([]string{
		"✓ Trade::weighted(powerWeight = 0.1)",
		"  selectedAlternative = Trade::strong (object #1)",
		"  objective tradeStudyObjective: satisfied",
		"  evaluationFunction(Trade::strong (object #1)) = 0.0 [selected]",
		"  evaluationFunction(Trade::light (object #2)) = -5.0",
		"  standing: value (observed: 1 run under reverse)",
	}, "\n")
	if got != want {
		t.Errorf("report is\n%s\nwant\n%s", got, want)
	}

	v := s.RunAnalysis("Trade::weighted(powerWeight = 0.0)")
	if v.Status != VerdictHolds {
		t.Fatalf("verdict = %+v", v)
	}
	if len(v.Evaluations) != 2 {
		t.Fatalf("evaluations = %+v", v.Evaluations)
	}
	if e := v.Evaluations[0]; e.Function != "Trade::weighted::evaluationFunction" ||
		len(e.Arguments) != 1 || e.Arguments[0] != "Trade::strong (object #1)" ||
		e.Result != "-30.0" || e.Selected || e.Tied || e.Error != "" {
		t.Errorf("first evaluation = %+v", e)
	}
	if e := v.Evaluations[1]; e.Arguments[0] != "Trade::light (object #2)" || e.Result != "-10.0" || !e.Selected {
		t.Errorf("second evaluation = %+v", e)
	}
}

// An evaluation that fails for one alternative is that evaluation's error, the
// ones made before it stand, and the objective is undecided.
func TestAnalysisOfATradeStudyKeepsEvaluationsBesideAFailure(t *testing.T) {
	s := tradeStudySession(t)
	v := s.RunAnalysis("Trade::failing(divisor = 1.0)")
	if v.Status != VerdictUnresolved {
		t.Fatalf("verdict = %+v", v)
	}
	if len(v.Lines) == 0 || !strings.HasPrefix(v.Lines[0], "error: ") || !strings.Contains(v.Lines[0], "division by zero") {
		t.Errorf("lines = %q, want the failed alternative's division by zero first", v.Lines)
	}
	if len(v.Evaluations) != 2 || v.Evaluations[0].Error != "" || v.Evaluations[0].Result != "0.12" {
		t.Fatalf("evaluations = %+v", v.Evaluations)
	}
	if e := v.Evaluations[1]; !strings.Contains(e.Error, "division by zero") || e.Selected || e.Result != "" {
		t.Errorf("failed evaluation = %+v", e)
	}
	if len(v.Values) != 1 || v.Values[0].Name != "objective tradeStudyObjective" || !strings.HasPrefix(v.Values[0].Value, "undecided") {
		t.Errorf("objective = %+v", v.Values)
	}
}

// A swept trade study reports each run's evaluations as a column, the selected
// alternative marked, so a reader sees why the pick changes along the range.
func TestSweepOfATradeStudyReportsEachRunsEvaluations(t *testing.T) {
	s := tradeStudySession(t)
	got := sweepTable(run(t, s, "%sweep Trade::weighted powerWeight=0.0..0.2:0.1"))
	want := strings.Join([]string{
		"sweep Trade::weighted — 3 run(s)",
		"powerWeight | selectedAlternative       | verdict                        | evaluations                                                                                                            | time",
		"-+-+-+-+-",
		"0.0         | Trade::light (object #2)  | tradeStudyObjective: satisfied | evaluationFunction(Trade::strong (object #1)) = -30.0; evaluationFunction(Trade::light (object #2)) = -10.0 [selected] | <time>",
		"0.1         | Trade::strong (object #1) | tradeStudyObjective: satisfied | evaluationFunction(Trade::strong (object #1)) = 0.0 [selected]; evaluationFunction(Trade::light (object #2)) = -5.0    | <time>",
		"0.2         | Trade::strong (object #1) | tradeStudyObjective: satisfied | evaluationFunction(Trade::strong (object #1)) = 30.0 [selected]; evaluationFunction(Trade::light (object #2)) = 0.0    | <time>",
		"  standing: table (observed: 3 rows)",
	}, "\n")
	if got != want {
		t.Errorf("table is\n%s\nwant\n%s", got, want)
	}

	v := s.RunSweep("Trade::weighted", []string{"powerWeight=0.0..0.2:0.1"})
	if v.Status != VerdictHolds || len(v.Rows) != 3 {
		t.Fatalf("verdict = %+v", v)
	}
	row := v.Rows[0]
	if len(row.Evaluations) != 2 || row.Evaluations[0].Result != "-30.0" || row.Evaluations[0].Selected ||
		row.Evaluations[1].Result != "-10.0" || !row.Evaluations[1].Selected {
		t.Errorf("first row's evaluations = %+v", row.Evaluations)
	}
	if row.Evaluations[1].Function != "Trade::weighted::evaluationFunction" ||
		row.Evaluations[1].Arguments[0] != "Trade::light (object #2)" {
		t.Errorf("first row's selected evaluation = %+v", row.Evaluations[1])
	}
}

// A row whose run failed on one alternative keeps the evaluations made before
// the failure beside its error, and the table reports it as a failed run.
func TestSweepOfATradeStudyKeepsEvaluationsOnAFailedRow(t *testing.T) {
	s := tradeStudySession(t)
	v := s.RunSweep("Trade::failing", []string{"divisor=1.0..2.0:1.0"})
	if v.Status != VerdictFails || len(v.Rows) != 2 {
		t.Fatalf("verdict = %+v", v)
	}
	failed := v.Rows[0]
	if !strings.Contains(failed.Error, "division by zero") {
		t.Errorf("failed row's error = %q", failed.Error)
	}
	if len(failed.Evaluations) != 2 || failed.Evaluations[0].Result != "0.12" ||
		!strings.Contains(failed.Evaluations[1].Error, "division by zero") {
		t.Errorf("failed row's evaluations = %+v", failed.Evaluations)
	}
	if len(failed.Verdicts) != 1 || !strings.HasPrefix(failed.Verdicts[0].Value, "undecided") {
		t.Errorf("failed row's verdicts = %+v", failed.Verdicts)
	}
	if ok := v.Rows[1]; ok.Error != "" || len(ok.Evaluations) != 2 || !ok.Evaluations[0].Selected {
		t.Errorf("completed row = %+v", ok)
	}
	got := sweepTable(run(t, s, "%sweep Trade::failing divisor=1.0..2.0:1.0"))
	if !strings.Contains(got, "division by zero") || !strings.Contains(got, "= 0.12") {
		t.Errorf("table is\n%s\nwant the failed row's error beside the evaluation made before it", got)
	}
}
