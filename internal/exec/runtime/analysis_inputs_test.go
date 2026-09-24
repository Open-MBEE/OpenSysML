package runtime

import (
	"reflect"
	"strings"
	"testing"
)

// The inputs a run reports are the values its parameters were bound to, in
// declaration order: arguments by position or by name, a default evaluated
// where none was given, and the subject parameter never among them.
func TestAnalysisResultInputsAreTheBoundParameters(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			part def Probe;
			individual def probe :> Probe;
			analysis def Check {
				subject s : Probe;
				in burnTime : Real;
				in margin : Real = 1.5;
				in flag : Boolean = true;
				out fuelUsed : Real = burnTime + margin;
			}
		}`)
	sym := requirementNamed(t, scope, "Check")
	probe, err := ctx.Instantiate(requirementNamed(t, scope, "probe"))
	if err != nil {
		t.Fatal(err)
	}

	textOf := func(inputs []InputBinding) []string {
		out := make([]string, len(inputs))
		for i, in := range inputs {
			out[i] = in.Name + "=" + FormatValue(in.Value)
		}
		return out
	}

	result, err := ctx.RunAnalysis(sym, AnalysisArgs{
		Subject:    probe,
		Positional: []Value{constValue(drawnReal(3.0))},
	}, scope, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"burnTime=3.0", "margin=1.5", "flag=true"}
	if got := textOf(result.Inputs); !reflect.DeepEqual(got, want) {
		t.Errorf("inputs %v, want %v", got, want)
	}

	result, err = ctx.RunAnalysis(sym, AnalysisArgs{
		Subject: probe,
		Named:   map[string]Value{"margin": constValue(drawnReal(0.25)), "burnTime": constValue(drawnReal(4.0))},
	}, scope, nil)
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"burnTime=4.0", "margin=0.25", "flag=true"}
	if got := textOf(result.Inputs); !reflect.DeepEqual(got, want) {
		t.Errorf("inputs %v, want %v", got, want)
	}
}

// A sweep row carries the inputs its run bound, the row's own overlay included.
func TestSweepRowCarriesTheRunsInputs(t *testing.T) {
	ctx, _ := analysisFixture(t, `package test { calc def Idle { return k : Integer = 0; } }`)
	bound := []InputBinding{{Name: "n", Value: constValue(drawnReal(7.0))}}
	row := runSweepRow(ctx, []SweepBinding{{Param: "n", Value: constValue(drawnReal(7.0))}},
		func(*Context, []SweepBinding) (SweepRunResult, error) {
			return SweepRunResult{Inputs: bound}, nil
		})
	if !reflect.DeepEqual(row.Inputs, bound) {
		t.Errorf("row inputs %v, want %v", row.Inputs, bound)
	}
}

// A Monte Carlo run reports the inputs its iteration bound and every declared
// output of that iteration, `observed` appended when it declares no output.
func TestMonteCarloRunInputsAndIterationOutputs(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import RandomFunctions::*;
			part def Probe {
				attribute t : Real;
				action settle { first start; then assign t := uniform(1.0, 5.0); then done; }
			}
			individual def probe :> Probe;
			analysis def Mc :> Simulation::MonteCarlo {
				subject analysed : Probe;
				in gain : Real = 2.0;
				perform action run ::> analysed.settle;
				attribute :>> observed : Real = analysed.t;
				return Mean : Real = mean;
			}
		}`)
	sym := requirementNamed(t, scope, "Mc")
	probe, err := ctx.Instantiate(requirementNamed(t, scope, "probe"))
	if err != nil {
		t.Fatal(err)
	}
	ctx.SetModelSeed(RunSeed(1, 1))
	run, err := ctx.ObserveMonteCarlo(sym, AnalysisArgs{Subject: probe}, scope, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Inputs) != 1 || run.Inputs[0].Name != "gain" {
		t.Fatalf("inputs %+v, want the one binding of gain", run.Inputs)
	}
	var names []string
	for _, out := range run.Outputs {
		names = append(names, out.Name)
	}
	// The statistics are bound over the sample, so the one output this
	// iteration establishes is the observed value; the return reading a
	// statistic is unread until the conclusion supplies it.
	want := []string{"observed"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("outputs %v, want %v", names, want)
	}
	if run.Unread["Mean"] == nil {
		t.Errorf("the stat-bound return is not in Unread: %v", run.Unread)
	}
}

// An output erroring for its own reason — not the statistics the sample has
// not supplied — is reported on the run, which still completes and observes.
func TestMonteCarloRunReportsAnOutputError(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import RandomFunctions::*;
			part def Probe {
				attribute t : Real;
				action settle { first start; then assign t := uniform(1.0, 5.0); then done; }
			}
			individual def probe :> Probe;
			analysis def Mc :> Simulation::MonteCarlo {
				subject analysed : Probe;
				perform action run ::> analysed.settle;
				attribute :>> observed : Real = analysed.t;
				return Mean : Real = mean;
				out Bad : Real = 1.0 / 0.0;
			}
		}
	`)
	sym := requirementNamed(t, scope, "Mc")
	probe, err := ctx.Instantiate(requirementNamed(t, scope, "probe"))
	if err != nil {
		t.Fatal(err)
	}
	ctx.SetModelSeed(RunSeed(1, 1))
	run, err := ctx.ObserveMonteCarlo(sym, AnalysisArgs{Subject: probe}, scope, nil)
	if err != nil {
		t.Fatal(err)
	}
	if run.Unread["Bad"] == nil {
		t.Error("an output erroring for its own reason vanished")
	}
	if run.Observed.Kind != ValConst {
		t.Errorf("the run's observed is invalid: %v", run.Observed)
	}
}

// The outputs an observation captures are read in a probe: capturing them
// draws nothing, so the conclusion over the sample — and the draws taken to
// make it — are the ones a capture-free observation makes, whether or not an
// output is a random draw of its own.
func TestMonteCarloIterationOutputsMemoizeNothing(t *testing.T) {
	sample := func(extraOutput string) (AnalysisResult, []DrawTaken, error) {
		ctx, scope := analysisFixture(t, `
			package test {
				private import ScalarValues::*;
				private import RandomFunctions::*;
				part def Probe {
					attribute t : Real;
					action settle { first start; then assign t := uniform(1.0, 5.0); then done; }
				}
				individual def probe :> Probe;
				analysis def Mc :> Simulation::MonteCarlo {
					subject analysed : Probe;
					perform action run ::> analysed.settle;
					attribute :>> observed : Real = analysed.t;
					return Mean : Real = mean;`+extraOutput+`
				}
			}`)
		sym := requirementNamed(t, scope, "Mc")
		probe, err := ctx.Instantiate(requirementNamed(t, scope, "probe"))
		if err != nil {
			t.Fatal(err)
		}
		ctx.SetModelSeed(RunSeed(1, 1))
		var runs []*MonteCarloRun
		for i := int64(1); i <= 3; i++ {
			run, err := ctx.ObserveMonteCarlo(sym, AnalysisArgs{Subject: probe}, scope, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(run.run.outputs) != 0 {
				t.Fatalf("capturing the outputs memoized into the run: %v", run.run.outputs)
			}
			runs = append(runs, run)
		}
		stats, err := MonteCarloSample(runs)
		if err != nil {
			t.Fatal(err)
		}
		res, err := ConcludeMonteCarlo(runs, stats)
		var observed []DrawTaken
		for _, d := range ctx.DrawsTaken() {
			if strings.HasPrefix(d.What, "uniform(1.0") {
				observed = append(observed, d)
			}
		}
		return res, observed, err
	}

	plain, drawsPlain, err := sample("")
	if err != nil {
		t.Fatal(err)
	}
	drawn, drawsDrawn, err := sample(`
					out Again : Real = uniform(0.0, 1.0);`)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(drawsDrawn, drawsPlain) {
		t.Errorf("capturing a draw-made output moved the sample's draws: %v vs %v", drawsDrawn, drawsPlain)
	}
	for i, out := range plain.Outputs {
		if i < len(drawn.Outputs) && drawn.Outputs[i].Name == out.Name && FormatValue(drawn.Outputs[i].Value) != FormatValue(out.Value) {
			t.Errorf("output %s changed: %v vs %v", out.Name, drawn.Outputs[i].Value, out.Value)
		}
	}
}

// A return reading a statistic is unread on every run — the conclusion reads
// it over the sample, the rows never carry it — even once concluded.
func TestMonteCarloStatBoundReturnStaysUnread(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import RandomFunctions::*;
			part def Probe {
				attribute t : Real;
				action settle { first start; then assign t := uniform(1.0, 5.0); then done; }
			}
			individual def probe :> Probe;
			analysis def Mc :> Simulation::MonteCarlo {
				subject analysed : Probe;
				perform action run ::> analysed.settle;
				attribute :>> observed : Real = analysed.t;
				return Mean : Real = mean;
			}
		}`)
	sym := requirementNamed(t, scope, "Mc")
	probe, err := ctx.Instantiate(requirementNamed(t, scope, "probe"))
	if err != nil {
		t.Fatal(err)
	}
	ctx.SetModelSeed(RunSeed(1, 1))
	var runs []*MonteCarloRun
	for i := 0; i < 2; i++ {
		run, err := ctx.ObserveMonteCarlo(sym, AnalysisArgs{Subject: probe}, scope, nil)
		if err != nil {
			t.Fatal(err)
		}
		runs = append(runs, run)
	}
	stats, err := MonteCarloSample(runs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ConcludeMonteCarlo(runs, stats); err != nil {
		t.Fatal(err)
	}
	for _, run := range runs {
		if run.Unread["Mean"] == nil {
			t.Errorf("run %d's stat-bound return left Unread", run.Number)
		}
		for _, out := range run.Outputs {
			if out.Name == "Mean" {
				t.Errorf("run %d carries the sample's mean as its own", run.Number)
			}
		}
	}
}
