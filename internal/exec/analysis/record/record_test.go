package record

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/format"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

var update = flag.Bool("update", false, "rewrite the golden files from the current generator")

func real(f float64) runtime.Value {
	return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: f}}
}

func integer(n int64) runtime.Value {
	return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
}

func boolean(b bool) runtime.Value {
	return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: b}}
}

func enumLiteral(t *testing.T) runtime.Value {
	file := parser.New(source.New("<test>", []byte(
		`package P { enum def Fuel { enum leaded; enum unleaded; } }`))).ParseFile()
	scope := symbols.Build(file)
	p, ok := scope.LookupLocal("P")
	if !ok {
		t.Fatal("package not built")
	}
	fuel, ok := p.Scope.LookupLocal("Fuel")
	if !ok {
		t.Fatal("enum def not built")
	}
	lit, ok := fuel.Scope.LookupLocal("leaded")
	if !ok {
		t.Fatal("enum literal not built")
	}
	return runtime.NewEnumLiteral(lit)
}

// spell renders values the way a session would: no object resolves to a usage.
func spell() Spelling {
	return Spelling{
		ObjectUsage: func(runtime.Value) string { return "" },
		Text:        runtime.FormatValue,
		Unset:       func(v runtime.Value) bool { return v.Kind == runtime.ValNull },
	}
}

func provenance(kind Kind) Provenance {
	return Provenance{
		RunAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Tool:    "sysml test",
		Command: "%record P::check",
		Kind:    kind,
	}
}

// golden runs Generate and compares Source against the golden at name.
func golden(t *testing.T, name string, req Request) Result {
	t.Helper()
	res, err := Generate(req)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, []byte(res.Source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s missing; run with -update", path)
	}
	if res.Source != string(want) {
		t.Errorf("%s differs from the generated text:\n%s", name, res.Source)
	}
	return res
}

// A single run records every value spelling, its verdicts and its provenance.
func TestGenerateSingleRun(t *testing.T) {
	obj := runtime.Value{Kind: runtime.ValInstance, Instance: 7}
	sp := spell()
	sp.ObjectUsage = func(v runtime.Value) string {
		if v == obj {
			return "P::scout"
		}
		return ""
	}
	res := golden(t, "single_run.sysml.golden", Request{
		Package: "Records", Case: "P::scoutBudget", Provenance: provenance(KindRun), Spell: sp,
		Runs: []Run{{
			Subject: Subject{Usage: "P::scout", Text: "P::scout"},
			Inputs: []runtime.InputBinding{
				{Name: "burnTime", Value: real(3.0)},
				{Name: "trials", Value: integer(8)},
				{Name: "crewOk", Value: boolean(true)},
				{Name: "label", Value: runtime.NewStringValue(`a "b"`)},
				{Name: "grade", Value: enumLiteral(t)},
				{Name: "tank", Value: runtime.NewQuantityValue(&runtime.Quantity{
					Num:  semantics.Value{Kind: semantics.ValReal, Real: 12.5},
					Unit: semantics.Unit{Text: "kg"},
				})},
				{Name: "target", Value: obj},
			},
			Outputs: []runtime.CalcOutputValue{
				{Name: "fuelUsed", Value: real(12.5)},
				{Name: "memo", Value: runtime.Value{Kind: runtime.ValNull}},
				{Name: "plan", Value: runtime.NewSequenceValue(nil)},
			},
			Verdicts: []runtime.AnalysisVerdict{
				{Kind: "objective", Name: "fuelFits", Status: runtime.VerdictSatisfied, Detail: "holds"},
				{Kind: "assertion", Name: "crewReady", Status: runtime.VerdictNotSatisfied},
			},
		}},
	})
	if res.Definition != "Records::ScoutBudgetRun" {
		t.Errorf("definition %q", res.Definition)
	}
	if len(res.Records) != 1 || res.Records[0] != "Records::scoutBudget_run1" {
		t.Errorf("records %v", res.Records)
	}
}

// A trade run records its evaluations: selected, tied and failed alternatives.
func TestGenerateTradeRun(t *testing.T) {
	failed := errors.New("boom")
	res := golden(t, "trade_run.sysml.golden", Request{
		Package: "P::Records", Case: "P::choose", Provenance: provenance(KindTrade), Spell: spell(),
		Runs: []Run{{
			Subject: Subject{Text: "P::fleet"},
			Outputs: []runtime.CalcOutputValue{{Name: "best", Value: real(1.0)}},
			Evaluations: []runtime.AnalysisEvaluation{
				{Function: "P::score", Arguments: []runtime.Value{real(1.0)}, Result: real(0.75), Selected: true},
				{Function: "P::score", Arguments: []runtime.Value{real(2.0)}, Result: real(0.75), Tied: true},
				{Function: "P::score", Arguments: []runtime.Value{real(3.0)}, Result: runtime.Value{Kind: runtime.ValNull}, Error: failed},
			},
		}},
	})
	if res.Definition != "P::Records::ChooseRun" {
		t.Errorf("definition %q", res.Definition)
	}
}

// A sweep reuses the existing definition and numbers its records on.
func TestGenerateSweepRuns(t *testing.T) {
	res := golden(t, "sweep_runs.sysml.golden", Request{
		Package: "Records", Case: "P::check", Provenance: provenance(KindSweep), Spell: spell(),
		Existing: Existing{
			Package: true, Definition: true, NextRun: 4,
			Attributes: map[string]Feature{
				"load": {TypeFQN: "ScalarValues::Real"},
				"done": {TypeFQN: "ScalarValues::Boolean"},
			},
		},
		Runs: []Run{
			{Iteration: 1, Inputs: []runtime.InputBinding{{Name: "load", Value: real(1.0)}}, Outputs: []runtime.CalcOutputValue{{Name: "done", Value: boolean(true)}}},
			{Iteration: 2, Inputs: []runtime.InputBinding{{Name: "load", Value: real(3.0)}}, Outputs: []runtime.CalcOutputValue{{Name: "done", Value: boolean(false)}}},
			{Iteration: 3, Inputs: []runtime.InputBinding{{Name: "load", Value: real(5.0)}}, Outputs: []runtime.CalcOutputValue{{Name: "done", Value: boolean(true)}}},
		},
	})
	want := []string{"Records::check_run4", "Records::check_run5", "Records::check_run6"}
	if fmt.Sprint(res.Records) != fmt.Sprint(want) {
		t.Errorf("records %v, want %v", res.Records, want)
	}
}

// Generate refuses the shapes it cannot record.
func TestGenerateErrors(t *testing.T) {
	base := func() Request {
		return Request{Package: "Records", Case: "P::check", Provenance: provenance(KindRun), Spell: spell()}
	}
	empty := Request{Package: "Records", Case: "P::check"}
	if _, err := Generate(empty); err == nil {
		t.Error("no runs: want an error")
	}
	req := base()
	req.Runs = []Run{{Inputs: []runtime.InputBinding{{Name: "x", Value: real(1)}}}}
	if _, err := Generate(req); err == nil {
		t.Error("a run with no outputs, verdicts or evaluations: want an error")
	}
	req = base()
	req.Runs = []Run{{Outputs: []runtime.CalcOutputValue{{Name: "kind", Value: real(1)}}}}
	if _, err := Generate(req); err == nil {
		t.Error("member colliding with an AnalysisRun feature: want an error")
	}
	req = base()
	req.Runs = []Run{{
		Inputs:  []runtime.InputBinding{{Name: "x", Value: real(1)}},
		Outputs: []runtime.CalcOutputValue{{Name: "x", Value: real(2)}},
	}}
	if _, err := Generate(req); err == nil {
		t.Error("input and output sharing a name: want an error")
	}
	req = base()
	req.Existing = Existing{Package: true, Definition: true, Attributes: map[string]Feature{
		"load": {TypeFQN: "ScalarValues::String"},
	}}
	req.Runs = []Run{{
		Inputs:  []runtime.InputBinding{{Name: "load", Value: real(1)}},
		Outputs: []runtime.CalcOutputValue{{Name: "done", Value: boolean(true)}},
	}}
	if _, err := Generate(req); err == nil {
		t.Error("existing def with a type-mismatched member: want an error")
	}
	req = base()
	req.Existing = Existing{Package: true, Definition: true, Attributes: map[string]Feature{
		"load": {Ref: true},
		"done": {TypeFQN: "ScalarValues::Boolean"},
	}}
	req.Runs = []Run{{
		Inputs:  []runtime.InputBinding{{Name: "load", Value: real(1)}},
		Outputs: []runtime.CalcOutputValue{{Name: "done", Value: boolean(true)}},
	}}
	if _, err := Generate(req); err == nil {
		t.Error("existing ref member fed an attribute value: want an error")
	}
	dose := runtime.NewQuantityValue(&runtime.Quantity{
		Num:  semantics.Value{Kind: semantics.ValReal, Real: 1.5},
		Unit: semantics.Unit{Text: "kg"},
	})
	req = base()
	req.Runs = []Run{{Outputs: []runtime.CalcOutputValue{
		{Name: "dose", Value: dose},
		{Name: "doseUnit", Value: real(1)},
	}}}
	if _, err := Generate(req); err == nil {
		t.Error("a member named as a quantity's unit companion: want an error")
	}
	req = base()
	req.Runs = []Run{{Outputs: []runtime.CalcOutputValue{
		{Name: "doseUnit", Value: real(1)},
		{Name: "dose", Value: dose},
	}}}
	if _, err := Generate(req); err == nil {
		t.Error("a quantity whose unit companion names a member: want an error")
	}
}

// A member unset in one run takes the type the settled run gives it, wherever
// it sits among the members the definition declares.
func TestGenerateSettlesAnUnsetMember(t *testing.T) {
	res, err := Generate(Request{
		Package: "Records", Case: "P::check", Provenance: provenance(KindSweep), Spell: spell(),
		Runs: []Run{
			{Iteration: 1, Outputs: []runtime.CalcOutputValue{
				{Name: "x", Value: runtime.Value{Kind: runtime.ValNull}},
				{Name: "a", Value: real(1)},
				{Name: "b", Value: real(2)},
				{Name: "c", Value: real(3)},
			}},
			{Iteration: 2, Outputs: []runtime.CalcOutputValue{
				{Name: "x", Value: real(3.0)},
				{Name: "a", Value: real(1)},
				{Name: "b", Value: real(2)},
				{Name: "c", Value: real(3)},
			}},
		},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{"attribute x : ScalarValues::Real;", "attribute :>> x = 3.0;"} {
		if !strings.Contains(res.Source, want) {
			t.Errorf("generated source is missing %q:\n%s", want, res.Source)
		}
	}
}

// Infinity has no literal of a typed attribute: it records as a string.
func TestGenerateInfinityValue(t *testing.T) {
	res, err := Generate(Request{
		Package: "Records", Case: "P::check", Provenance: provenance(KindRun), Spell: spell(),
		Runs: []Run{{Outputs: []runtime.CalcOutputValue{
			{Name: "value", Value: runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInfinity}}},
		}}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{"attribute value : ScalarValues::String;", `attribute :>> value = "*";`} {
		if !strings.Contains(res.Source, want) {
			t.Errorf("generated source is missing %q:\n%s", want, res.Source)
		}
	}
	if _, err := format.Source("<test>", []byte(res.Source), format.DefaultOptions); err != nil {
		t.Errorf("generated source does not parse: %v", err)
	}
}

// The generated text is formatter-stable: formatting it changes nothing.
func TestGeneratedSourceIsFormatterStable(t *testing.T) {
	for _, name := range []string{"single_run.sysml.golden", "trade_run.sysml.golden", "sweep_runs.sysml.golden"} {
		src, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("%s missing; run with -update", name)
		}
		out, err := format.Source(name, src, format.DefaultOptions)
		if err != nil {
			t.Fatalf("format %s: %v", name, err)
		}
		if string(out) != string(src) {
			t.Errorf("%s is not formatter-stable:\n%s", name, out)
		}
	}
}
