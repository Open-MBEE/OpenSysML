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
	var names []string
	for _, out := range run.Outputs {
		names = append(names, out.Name)
	}
	// The statistics are bound over the sample, so the one output this
	// iteration establishes is the observed value.
	want := []string{"observed"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("outputs %v, want %v", names, want)
	}
}
