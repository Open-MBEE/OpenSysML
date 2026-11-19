package opensysml_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// sweepSource declares calcs over whole and real parameters, an analysis case
// on a subject, and a trade study whose every run evaluates its alternatives.
const sweepSource = `package Sw {
	private import ScalarValues::*;
	private import TradeStudies::*;

	part def Ship { attribute cost : Real default = 5.0; }
	calc def Twice { in n : Integer[1]; return : Integer = n * 2; }
	calc def Plus { in a : Integer[1]; in b : Integer[1]; return : Integer = a + b; }
	calc def Ratio { in a : Real[1]; in b : Real[1]; return : Real = a / b; }

	analysis def CostAnalysis {
		subject s : Ship;
		in limit : Real = 20.0;
		out total : Real = s.cost * limit;
		objective affordable { require constraint { total <= 100.0 } }
	}
	part ship : Ship;

	part def Engine { attribute mass : Real; }
	part light : Engine { attribute :>> mass = 10.0; }
	part heavy : Engine { attribute :>> mass = 30.0; }
	analysis def Lightest :> TradeStudy {
		in margin : Real = 0.0;
		subject : Engine[1..*] = (light, heavy);
		objective : MinimizeObjective;
		calc :>> evaluationFunction {
			in part e :>> alternative : Engine;
			return :>> result : Real = e.mass + margin;
		}
		return part :>> selectedAlternative : Engine;
	}
}
`

// sweepLines renders a table as one line per row: inputs, outputs, failure.
func sweepLines(sweep *opensysml.Sweep) string {
	lines := make([]string, 0, len(sweep.Rows))
	for _, row := range sweep.Rows {
		var parts []string
		for _, in := range row.Inputs {
			parts = append(parts, fmt.Sprintf("%s=%v", in.Name, in.Value))
		}
		for _, out := range row.Outputs {
			parts = append(parts, fmt.Sprintf("%s -> %v", out.Name, out.Value))
		}
		if row.Error != "" {
			parts = append(parts, "error")
		}
		lines = append(lines, strings.Join(parts, " "))
	}
	return strings.Join(lines, "\n")
}

// A stepped sweep runs the product of its ranges, the first range varying
// slowest, with the arguments every row shares, over both transports.
func TestSweepSteppedAgainstService(t *testing.T) {
	for name, connect := range analysisCallers(t) {
		t.Run(name, func(t *testing.T) {
			client := connect(t)
			model := parse(t, client, sweepSource)
			ctx := context.Background()

			sweep, err := client.Sweep(ctx, model, "Sw::Plus", []opensysml.SweepRange{
				{Parameter: "a", Start: opensysml.Int(1), End: opensysml.Int(2)},
				{Parameter: "b", Start: opensysml.Int(10), End: opensysml.Int(20), Step: opensysml.Int(10)},
			})
			if err != nil {
				t.Fatalf("Sweep: %v", err)
			}
			want := "a=1 b=10 result -> [11]\na=1 b=20 result -> [21]\na=2 b=10 result -> [12]\na=2 b=20 result -> [22]"
			if got := sweepLines(sweep); got != want {
				t.Errorf("rows are\n%s\nwant\n%s", got, want)
			}
			if fmt.Sprint(sweep.Parameters) != "[a b]" || sweep.Sampled || sweep.Engine == "" || sweep.Strength == "" {
				t.Errorf("sweep = %+v, want parameters [a b], stepped, under a named engine", sweep)
			}
			for i, row := range sweep.Rows {
				if row.Elapsed < 0 || row.Reason != 0 {
					t.Errorf("row %d = %+v, want a run that took time and did not fail", i, row)
				}
			}

			named, err := client.Sweep(ctx, model, "Sw::Plus",
				[]opensysml.SweepRange{{Parameter: "b", Start: opensysml.Int(1), End: opensysml.Int(2)}},
				opensysml.SweepArgument("a", opensysml.Int(100)))
			if err != nil || sweepLines(named) != "b=1 result -> [101]\nb=2 result -> [102]" {
				t.Errorf("named argument: %v\n%s", err, sweepLines(named))
			}
			positional, err := client.Sweep(ctx, model, "Sw::Plus",
				[]opensysml.SweepRange{{Parameter: "b", Start: opensysml.Int(1), End: opensysml.Int(2)}},
				opensysml.SweepArguments(opensysml.Int(100)))
			if err != nil || sweepLines(positional) != sweepLines(named) {
				t.Errorf("positional argument: %v\n%s", err, sweepLines(positional))
			}
		})
	}
}

// A run that failed is its row's error and reason, and the rows after it run.
func TestSweepFailedRunIsARow(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, sweepSource)
	sweep, err := client.Sweep(context.Background(), model, "Sw::Ratio",
		[]opensysml.SweepRange{{Parameter: "b", Start: opensysml.Int(-1), End: opensysml.Int(1)}},
		opensysml.SweepArgument("a", opensysml.Real(4)))
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(sweep.Rows) != 3 {
		t.Fatalf("ran %d row(s), want 3:\n%s", len(sweep.Rows), sweepLines(sweep))
	}
	failed := sweep.Rows[1]
	if !strings.Contains(failed.Error, "division by zero") || failed.Reason != opensysml.ReasonEvaluation || len(failed.Outputs) != 0 {
		t.Errorf("failed row = %+v, want a division by zero with reason EVALUATION and no outputs", failed)
	}
	for _, i := range []int{0, 2} {
		if row := sweep.Rows[i]; row.Error != "" || row.Reason != 0 || len(row.Outputs) != 1 {
			t.Errorf("row %d = %+v, want one output and no error", i, row)
		}
	}
}

// A sampled sweep draws the same rows from the same seed and echoes the seed.
func TestSweepSampledIsReproducible(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, sweepSource)
	draw := func(seed uint64) *opensysml.Sweep {
		t.Helper()
		sweep, err := client.Sweep(context.Background(), model, "Sw::Twice",
			[]opensysml.SweepRange{{Parameter: "n", Start: opensysml.Int(0), End: opensysml.Int(1_000_000)}},
			opensysml.SweepSamples(8), opensysml.SweepSeed(seed))
		if err != nil {
			t.Fatalf("Sweep seed %d: %v", seed, err)
		}
		return sweep
	}
	first, again, other := draw(7), draw(7), draw(8)
	if !first.Sampled || first.Seed != 7 || len(first.Rows) != 8 {
		t.Fatalf("sweep reports sampled %v, seed %d, %d row(s); want true, 7, 8", first.Sampled, first.Seed, len(first.Rows))
	}
	if sweepLines(first) != sweepLines(again) {
		t.Errorf("seed 7 drew\n%s\nthen\n%s", sweepLines(first), sweepLines(again))
	}
	if sweepLines(first) == sweepLines(other) {
		t.Errorf("seed 8 drew what seed 7 did:\n%s", sweepLines(first))
	}
}

// An analysis case runs on the subject named once per row, each row carrying
// its outputs and its objective's verdict about an instance the sweep reports.
func TestSweepAnalysisCaseOnSubject(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, sweepSource)
	sweep, err := client.Sweep(context.Background(), model, "Sw::CostAnalysis",
		[]opensysml.SweepRange{{Parameter: "limit", Start: opensysml.Real(2), End: opensysml.Real(30), Step: opensysml.Real(14)}},
		opensysml.SweepSubject("Sw::ship"), opensysml.SweepEngine("sweep"))
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(sweep.Rows) != 3 || sweep.Engine != "sweep" {
		t.Fatalf("sweep = %+v, want 3 rows under the sweep engine", sweep)
	}
	for i, want := range []bool{true, true, false} {
		row := sweep.Rows[i]
		if len(row.Verdicts) != 1 || row.Verdicts[0].Kind != "objective" || row.Verdicts[0].Holds != want {
			t.Fatalf("row %d verdicts = %+v, want the objective holding %v", i, row.Verdicts, want)
		}
		if row.Verdicts[0].InstanceTypeID != "Sw::ship" {
			t.Errorf("row %d verdict is about %q, want the subject named", i, row.Verdicts[0].InstanceTypeID)
		}
		if len(row.Outputs) == 0 {
			t.Errorf("row %d reports no outputs", i)
		}
	}
}

// Each row of a swept trade study carries the evaluations its run made, their
// alternatives resolving through the instances the sweep reports.
func TestSweepRowsCarryEvaluations(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, sweepSource)
	sweep, err := client.Sweep(context.Background(), model, "Sw::Lightest",
		[]opensysml.SweepRange{{Parameter: "margin", Start: opensysml.Real(0), End: opensysml.Real(1), Step: opensysml.Real(1)}})
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(sweep.Rows) != 2 {
		t.Fatalf("ran %d row(s), want 2:\n%s", len(sweep.Rows), sweepLines(sweep))
	}
	for i, row := range sweep.Rows {
		if row.Error != "" {
			t.Fatalf("row %d failed: %s", i, row.Error)
		}
		var got []string
		for _, evaluation := range row.Evaluations {
			alternative := "?"
			if len(evaluation.Arguments) == 1 {
				if id, ok := evaluation.Arguments[0].(opensysml.InstanceID); ok && sweep.Instance(id) != nil {
					alternative = sweep.Instance(id).TypeSymbolID
				}
			}
			got = append(got, fmt.Sprintf("%s=%v selected=%v", alternative, evaluation.Result, evaluation.Selected))
		}
		margin := float64(i)
		want := []string{
			fmt.Sprintf("Sw::light=%v selected=true", opensysml.Real(10+margin)),
			fmt.Sprintf("Sw::heavy=%v selected=false", opensysml.Real(30+margin)),
		}
		if strings.Join(got, "; ") != strings.Join(want, "; ") {
			t.Errorf("row %d evaluations = %v, want %v", i, got, want)
		}
	}
}

// A sweep the service refuses before running is a VerifyError with its reason.
func TestSweepRefusalsAreVerifyErrors(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, sweepSource)
	ctx := context.Background()
	ints := []opensysml.SweepRange{{Parameter: "n", Start: opensysml.Int(1), End: opensysml.Int(2)}}
	for name, tc := range map[string]struct {
		symbol string
		ranges []opensysml.SweepRange
		opts   []opensysml.SweepOption
		wants  string
		reason opensysml.Reason
	}{
		"another kind of element": {symbol: "Sw::ship", ranges: ints, wants: "Sw::ship", reason: opensysml.ReasonWrongKind},
		"an unknown symbol":       {symbol: "Sw::Nope", ranges: ints, wants: "Sw::Nope"},
		"a calc given a subject":  {symbol: "Sw::Twice", ranges: ints, opts: []opensysml.SweepOption{opensysml.SweepSubject("Sw::ship")}, wants: "subject"},
		"no range":                {symbol: "Sw::Twice", wants: "range"},
		"a parameter bound twice": {symbol: "Sw::Twice", ranges: ints, opts: []opensysml.SweepOption{opensysml.SweepArgument("n", opensysml.Int(3))}, wants: "both an argument"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := client.Sweep(ctx, model, tc.symbol, tc.ranges, tc.opts...)
			var failure *opensysml.VerifyError
			if !errors.As(err, &failure) || failure.Op != "Sweep" || !strings.Contains(failure.Message, tc.wants) {
				t.Fatalf("Sweep: %v, want an in-band VerifyError mentioning %q", err, tc.wants)
			}
			if tc.reason != 0 && failure.Reason != tc.reason {
				t.Errorf("reason = %v, want %v", failure.Reason, tc.reason)
			}
		})
	}
	if _, err := client.Sweep(ctx, nil, "Sw::Twice", ints); err == nil {
		t.Error("Sweep on no model succeeded")
	}
}
