package runtime

import (
	"reflect"
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
	outputs, err := run.IterationOutputs()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, out := range outputs {
		names = append(names, out.Name)
	}
	// The statistics are bound over the sample, so the one output this
	// iteration establishes is the observed value.
	want := []string{"observed"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("outputs %v, want %v", names, want)
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
	if _, err := run.IterationOutputs(); err == nil {
		t.Error("an output erroring for its own reason vanished")
	}
	if run.Observed.Kind != ValConst {
		t.Errorf("the run's observed is invalid: %v", run.Observed)
	}
}

// IterationOutputs evaluates nothing into the run it is read of: called twice
// it reports the same values, and the conclusion over the sample is the one a
// sample no iteration's outputs were ever read of makes.
func TestMonteCarloIterationOutputsMemoizeNothing(t *testing.T) {
	const model = `
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
				out Again : Real = uniform(0.0, 1.0);
			}
		}`
	sample := func(read func(*MonteCarloRun)) (AnalysisResult, error) {
		ctx, scope := analysisFixture(t, model)
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
			if read != nil {
				read(run)
			}
			runs = append(runs, run)
		}
		stats, err := MonteCarloSample(runs)
		if err != nil {
			t.Fatal(err)
		}
		return ConcludeMonteCarlo(runs, stats)
	}

	first, err := sample(nil)
	if err != nil {
		t.Fatal(err)
	}
	read, err := sample(func(r *MonteCarloRun) {
		a, err := r.IterationOutputs()
		if err != nil {
			t.Fatal(err)
		}
		b, err := r.IterationOutputs()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(a, b) {
			t.Errorf("two reads differ: %v vs %v", a, b)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read.Outputs, first.Outputs) || !reflect.DeepEqual(read.Verdicts, first.Verdicts) {
		t.Errorf("reading the iterations' outputs moved the conclusion:\n%v\nwant\n%v", read, first)
	}
}
