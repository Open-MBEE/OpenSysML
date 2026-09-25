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

func realValue(f float64) runtime.Value {
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
		Package: "Records", Case: "P::scoutBudget", Provenance: provenance(KindRun),
		Runs: []Run{{
			Spell:   sp,
			Subject: Subject{Usage: "P::scout", Text: "P::scout"},
			Inputs: []runtime.InputBinding{
				{Name: "burnTime", Value: realValue(3.0)},
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
				{Name: "fuelUsed", Value: realValue(12.5)},
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
		Package: "P::Records", Case: "P::choose", Provenance: provenance(KindTrade),
		Runs: []Run{{
			Spell:   spell(),
			Subject: Subject{Text: "P::fleet"},
			Outputs: []runtime.CalcOutputValue{{Name: "best", Value: realValue(1.0)}},
			Evaluations: []runtime.AnalysisEvaluation{
				{Function: "P::score", Arguments: []runtime.Value{realValue(1.0)}, Result: realValue(0.75), Selected: true},
				{Function: "P::score", Arguments: []runtime.Value{realValue(2.0)}, Result: realValue(0.75), Tied: true},
				{Function: "P::score", Arguments: []runtime.Value{realValue(3.0)}, Result: runtime.Value{Kind: runtime.ValNull}, Error: failed},
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
		Package: "Records", Case: "P::check", Provenance: provenance(KindSweep),
		Existing: Existing{
			Package: true, Definition: true, Taken: map[int]bool{1: true, 2: true, 3: true},
			Attributes: map[string]Feature{
				"load": {TypeFQN: "ScalarValues::Real"},
				"done": {TypeFQN: "ScalarValues::Boolean"},
			},
		},
		Runs: []Run{
			{Iteration: 1, Spell: spell(), Inputs: []runtime.InputBinding{{Name: "load", Value: realValue(1.0)}}, Outputs: []runtime.CalcOutputValue{{Name: "done", Value: boolean(true)}}},
			{Iteration: 2, Spell: spell(), Inputs: []runtime.InputBinding{{Name: "load", Value: realValue(3.0)}}, Outputs: []runtime.CalcOutputValue{{Name: "done", Value: boolean(false)}}},
			{Iteration: 3, Spell: spell(), Inputs: []runtime.InputBinding{{Name: "load", Value: realValue(5.0)}}, Outputs: []runtime.CalcOutputValue{{Name: "done", Value: boolean(true)}}},
		},
	})
	want := []string{"Records::check_run4", "Records::check_run5", "Records::check_run6"}
	if fmt.Sprint(res.Records) != fmt.Sprint(want) {
		t.Errorf("records %v, want %v", res.Records, want)
	}
}

// A run that reached external tools records each call's tool line in call order.
func TestGenerateToolRun(t *testing.T) {
	prov := provenance(KindRun)
	prov.Tools = []string{
		"ThermalSolver 2.3 from /etc/opensysml/tools/thermal.json: /usr/bin/python3 solve.py --mass 12.5",
		"EchoTool: /opt/bin/echo < {\"toolName\":\"EchoTool\"}",
	}
	res := golden(t, "tool_run.sysml.golden", Request{
		Package: "Records", Case: "P::heatCheck", Provenance: prov,
		Runs: []Run{{
			Spell:   spell(),
			Subject: Subject{Text: "P::block"},
			Outputs: []runtime.CalcOutputValue{{Name: "tMax", Value: realValue(87.2)}},
		}},
	})
	if len(res.Records) != 1 {
		t.Fatalf("records %v", res.Records)
	}
}

// Generate refuses the shapes it cannot record.
func TestGenerateErrors(t *testing.T) {
	base := func() Request {
		return Request{Package: "Records", Case: "P::check", Provenance: provenance(KindRun)}
	}
	empty := Request{Package: "Records", Case: "P::check"}
	if _, err := Generate(empty); err == nil {
		t.Error("no runs: want an error")
	}
	req := base()
	req.Runs = []Run{{Spell: spell(), Inputs: []runtime.InputBinding{{Name: "x", Value: realValue(1)}}}}
	if _, err := Generate(req); err == nil {
		t.Error("a run with no outputs, verdicts or evaluations: want an error")
	}
	req = base()
	req.Runs = []Run{{Spell: spell(), Outputs: []runtime.CalcOutputValue{{Name: "kind", Value: realValue(1)}}}}
	if _, err := Generate(req); err == nil {
		t.Error("member colliding with an AnalysisRun feature: want an error")
	}
	req = base()
	req.Runs = []Run{{
		Spell:   spell(),
		Outputs: []runtime.CalcOutputValue{{Name: "x", Value: realValue(2)}, {Name: "x", Value: realValue(3)}},
	}}
	if _, err := Generate(req); err == nil {
		t.Error("one side of the run listing a name twice: want an error")
	}
	req = base()
	req.Runs = []Run{{
		Spell:   spell(),
		Inputs:  []runtime.InputBinding{{Name: "x", Value: realValue(1)}, {Name: "xIn", Value: realValue(0)}},
		Outputs: []runtime.CalcOutputValue{{Name: "x", Value: realValue(2)}},
	}}
	if _, err := Generate(req); err == nil {
		t.Error("member colliding with an inout's in companion: want an error")
	}
	req = base()
	req.Existing = Existing{Package: true, Definition: true, Attributes: map[string]Feature{
		"load": {TypeFQN: "ScalarValues::String"},
	}}
	req.Runs = []Run{{
		Spell:   spell(),
		Inputs:  []runtime.InputBinding{{Name: "load", Value: realValue(1)}},
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
		Spell:   spell(),
		Inputs:  []runtime.InputBinding{{Name: "load", Value: realValue(1)}},
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
	req.Runs = []Run{{Spell: spell(), Outputs: []runtime.CalcOutputValue{
		{Name: "dose", Value: dose},
		{Name: "doseUnit", Value: realValue(1)},
	}}}
	if _, err := Generate(req); err == nil {
		t.Error("a member named as a quantity's unit companion: want an error")
	}
	req = base()
	req.Runs = []Run{{Spell: spell(), Outputs: []runtime.CalcOutputValue{
		{Name: "doseUnit", Value: realValue(1)},
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
		Package: "Records", Case: "P::check", Provenance: provenance(KindSweep),
		Runs: []Run{
			{Iteration: 1, Spell: spell(), Outputs: []runtime.CalcOutputValue{
				{Name: "x", Value: runtime.Value{Kind: runtime.ValNull}},
				{Name: "a", Value: realValue(1)},
				{Name: "b", Value: realValue(2)},
				{Name: "c", Value: realValue(3)},
			}},
			{Iteration: 2, Spell: spell(), Outputs: []runtime.CalcOutputValue{
				{Name: "x", Value: realValue(3.0)},
				{Name: "a", Value: realValue(1)},
				{Name: "b", Value: realValue(2)},
				{Name: "c", Value: realValue(3)},
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
		Package: "Records", Case: "P::check", Provenance: provenance(KindRun),
		Runs: []Run{{Spell: spell(), Outputs: []runtime.CalcOutputValue{
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
	for _, name := range []string{"single_run.sysml.golden", "trade_run.sysml.golden", "sweep_runs.sysml.golden", "tool_run.sysml.golden"} {
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

// A member unset in an early row and a quantity in a later one still gains
// its unit companion, settled to Real.
func TestGenerateSettlesAnUnsetMemberToQuantity(t *testing.T) {
	res, err := Generate(Request{
		Package: "Records", Case: "P::check", Provenance: provenance(KindSweep),
		Runs: []Run{
			{Iteration: 1, Spell: spell(), Outputs: []runtime.CalcOutputValue{
				{Name: "x", Value: runtime.Value{Kind: runtime.ValNull}},
				{Name: "a", Value: realValue(1)},
				{Name: "b", Value: realValue(2)},
			}},
			{Iteration: 2, Spell: spell(), Outputs: []runtime.CalcOutputValue{
				{Name: "x", Value: runtime.NewQuantityValue(&runtime.Quantity{
					Num:  semantics.Value{Kind: semantics.ValReal, Real: 2.0},
					Unit: semantics.Unit{Text: "kg"},
				})},
				{Name: "a", Value: realValue(1)},
				{Name: "b", Value: realValue(2)},
			}},
		},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{
		"attribute x : ScalarValues::Real;",
		"attribute xUnit : ScalarValues::String;",
		"attribute :>> x = 2.0;",
		`attribute :>> xUnit = "kg";`,
	} {
		if !strings.Contains(res.Source, want) {
			t.Errorf("generated source is missing %q:\n%s", want, res.Source)
		}
	}
	if _, err := format.Source("<test>", []byte(res.Source), format.DefaultOptions); err != nil {
		t.Errorf("generated source does not parse: %v", err)
	}
}

// A member Real in one row and a quantity in a later one declares the unit
// companion, which only the rows with a unit redefine.
func TestGenerateSettlesARealMemberToQuantity(t *testing.T) {
	res, err := Generate(Request{
		Package: "Records", Case: "P::check", Provenance: provenance(KindSweep),
		Runs: []Run{
			{Iteration: 1, Spell: spell(), Outputs: []runtime.CalcOutputValue{{Name: "x", Value: realValue(1)}}},
			{Iteration: 2, Spell: spell(), Outputs: []runtime.CalcOutputValue{
				{Name: "x", Value: runtime.NewQuantityValue(&runtime.Quantity{
					Num:  semantics.Value{Kind: semantics.ValReal, Real: 2.0},
					Unit: semantics.Unit{Text: "kg"},
				})},
			}},
		},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{
		"attribute x : ScalarValues::Real;",
		"attribute xUnit : ScalarValues::String;",
		"attribute :>> x = 1.0;",
		`attribute :>> xUnit = "kg";`,
	} {
		if !strings.Contains(res.Source, want) {
			t.Errorf("generated source is missing %q:\n%s", want, res.Source)
		}
	}
	if strings.Count(res.Source, "xUnit = ") != 1 {
		t.Errorf("only the row with a unit should redefine xUnit:\n%s", res.Source)
	}
	if _, err := format.Source("<test>", []byte(res.Source), format.DefaultOptions); err != nil {
		t.Errorf("generated source does not parse: %v", err)
	}
}

// Record numbers fill the gaps a package's earlier records leave.
func TestGenerateNumbersIntoTheGaps(t *testing.T) {
	res, err := Generate(Request{
		Package: "Records", Case: "P::check", Provenance: provenance(KindSweep),
		Existing: Existing{
			Package: true, Definition: true, Taken: map[int]bool{2: true},
			Attributes: map[string]Feature{"load": {TypeFQN: "ScalarValues::Real"}},
		},
		Runs: []Run{
			{Iteration: 1, Spell: spell(), Outputs: []runtime.CalcOutputValue{{Name: "load", Value: realValue(1)}}},
			{Iteration: 2, Spell: spell(), Outputs: []runtime.CalcOutputValue{{Name: "load", Value: realValue(2)}}},
		},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	want := []string{"Records::check_run1", "Records::check_run3"}
	if fmt.Sprint(res.Records) != fmt.Sprint(want) {
		t.Errorf("records %v, want %v", res.Records, want)
	}
}

// A case name that needs quoting is carried through quoted record names, and
// the def marks the case it records.
func TestGenerateQuotedName(t *testing.T) {
	res, err := Generate(Request{
		Package: "Records", Case: "Demo::fuel budget", CaseName: "fuel budget",
		Provenance: provenance(KindRun),
		Runs:       []Run{{Spell: spell(), Outputs: []runtime.CalcOutputValue{{Name: "y", Value: realValue(3)}}}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{
		"part def 'Fuel budgetRun' :> AnalysisRecords::AnalysisRun",
		`attribute :>> caseName default = "Demo::fuel budget";`,
		"part 'fuel budget_run1' : 'Fuel budgetRun'",
	} {
		if !strings.Contains(res.Source, want) {
			t.Errorf("generated source is missing %q:\n%s", want, res.Source)
		}
	}
	if _, err := format.Source("<test>", []byte(res.Source), format.DefaultOptions); err != nil {
		t.Errorf("generated source does not parse: %v", err)
	}
}

// An owner-prefixed stem names the definition and the records of a case whose
// short name a sibling's definition already took.
func TestGenerateOwnerStem(t *testing.T) {
	res, err := Generate(Request{
		Package: "Records", Case: "Demo::B::check",
		Provenance: provenance(KindRun),
		Existing:   Existing{Stem: "B_check"},
		Runs:       []Run{{Spell: spell(), Outputs: []runtime.CalcOutputValue{{Name: "y", Value: realValue(2)}}}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{
		"part def B_checkRun :> AnalysisRecords::AnalysisRun",
		"part B_check_run1 : B_checkRun",
	} {
		if !strings.Contains(res.Source, want) {
			t.Errorf("generated source is missing %q:\n%s", want, res.Source)
		}
	}
}

// A member declared ScalarValue by an earlier, unset run accepts a concrete
// type the next run settles it to.
func TestGenerateExistingScalarValueAcceptsASettledType(t *testing.T) {
	res, err := Generate(Request{
		Package: "Records", Case: "P::check", Provenance: provenance(KindRun),
		Existing: Existing{
			Package: true, Definition: true,
			Attributes: map[string]Feature{"x": {TypeFQN: "ScalarValues::ScalarValue"}},
			Stem:       "check",
		},
		Runs: []Run{{Spell: spell(), Outputs: []runtime.CalcOutputValue{{Name: "x", Value: realValue(2)}}}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(res.Source, "attribute :>> x = 2.0;") {
		t.Errorf("generated source is missing the redefinition:\n%s", res.Source)
	}
}

// A verification run records what its body and subcases decided: the body's
// verdict beside the case's features, each verdict a VerdictRecord row.
func TestGenerateVerificationRun(t *testing.T) {
	res, err := Generate(Request{
		Package: "P::Records", Case: "P::fire", Provenance: provenance(KindRun),
		Runs: []Run{{
			Outputs:  []runtime.CalcOutputValue{{Name: "margin", Value: realValue(-200.0)}},
			Verdicts: []runtime.AnalysisVerdict{{Kind: "objective", Name: "thrust", Status: runtime.VerdictNotSatisfied}},
			Verifications: []runtime.VerificationVerdict{
				{Case: "P::fire", Kind: runtime.VerdictFail},
				{Case: "P::cold", Kind: runtime.VerdictPass, Subcase: true},
				{Case: "P::hot", Kind: runtime.VerdictInconclusive, Subcase: true, Detail: "no data"},
			},
			Spell: spell(),
		}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{
		`attribute :>> verdict = "fail";`,
		`part verification1 : AnalysisRecords::VerdictRecord :> verdicts {`,
		`attribute :>> kind = "verification";`,
		`attribute :>> name = "P::fire";`,
		`attribute :>> status = "fail";`,
		`part verification2`,
		`attribute :>> kind = "subcase";`,
		`attribute :>> name = "P::cold";`,
		`attribute :>> status = "pass";`,
		`part verification3`,
		`attribute :>> status = "inconclusive";`,
		`attribute :>> detail = "no data";`,
	} {
		if !strings.Contains(res.Source, want) {
			t.Errorf("source is missing %q:\n%s", want, res.Source)
		}
	}
	reparsed := parser.New(source.New("<record>", []byte(res.Source)))
	reparsed.ParseFile()
	if len(reparsed.Diagnostics) > 0 {
		t.Fatalf("generated source does not parse: %v\n%s", reparsed.Diagnostics[0], res.Source)
	}
}

// A run that decided only verification verdicts is still a record worth
// making: verdicts count toward "nothing to record".
func TestGenerateVerificationOnlyRun(t *testing.T) {
	res, err := Generate(Request{
		Package: "P::Records", Case: "P::fire", Provenance: provenance(KindRun),
		Runs: []Run{{
			Verifications: []runtime.VerificationVerdict{{Case: "P::fire", Kind: runtime.VerdictPass}},
			Spell:         spell(),
		}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(res.Source, `attribute :>> verdict = "pass";`) {
		t.Errorf("source is missing the verdict:\n%s", res.Source)
	}
}

// An inout records as one member carrying the value the run left in it,
// plus an <name>In companion carrying the value it was bound with.
func TestGenerateInoutRun(t *testing.T) {
	res, err := Generate(Request{
		Package: "P::Records", Case: "P::count", Provenance: provenance(KindRun),
		Runs: []Run{{
			Inputs:  []runtime.InputBinding{{Name: "counter", Value: integer(3)}},
			Outputs: []runtime.CalcOutputValue{{Name: "counter", Value: integer(6)}, {Name: "doubled", Value: integer(6)}},
			Spell:   spell(),
		}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{
		"attribute counter : ScalarValues::Integer;",
		"attribute counterIn : ScalarValues::Integer;",
		"attribute doubled : ScalarValues::Integer;",
		"attribute :>> counter = 6;",
		"attribute :>> counterIn = 3;",
		"attribute :>> doubled = 6;",
	} {
		if !strings.Contains(res.Source, want) {
			t.Errorf("source is missing %q:\n%s", want, res.Source)
		}
	}
	// The In companion follows its value member.
	if strings.Index(res.Source, "attribute counterIn") < strings.Index(res.Source, "attribute counter :") {
		t.Errorf("the In companion precedes its member:\n%s", res.Source)
	}
	reparsed := parser.New(source.New("<record>", []byte(res.Source)))
	reparsed.ParseFile()
	if len(reparsed.Diagnostics) > 0 {
		t.Fatalf("generated source does not parse: %v\n%s", reparsed.Diagnostics[0], res.Source)
	}
}

// A scalar-valued enumeration literal records as the literal it is, not the
// scalar it equals.
func TestGenerateScalarValuedEnumLiteral(t *testing.T) {
	file := parser.New(source.New("<test>", []byte(
		`package P { enum def Grade { enum high = 3; enum low = 1; } }`))).ParseFile()
	scope := symbols.Build(file)
	p, _ := scope.LookupLocal("P")
	grade, _ := p.Scope.LookupLocal("Grade")
	high, ok := grade.Scope.LookupLocal("high")
	if !ok {
		t.Fatal("enum literal not built")
	}
	res, err := Generate(Request{
		Package: "P::Records", Case: "P::mix", Provenance: provenance(KindRun),
		Runs: []Run{{
			Outputs: []runtime.CalcOutputValue{
				{Name: "g", Value: runtime.EnumeratedValue(high, integer(3))},
				{Name: "half", Value: realValue(1.5)},
			},
			Spell: spell(),
		}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{
		"attribute g : P::Grade;",
		"attribute :>> g = P::Grade::high;",
	} {
		if !strings.Contains(res.Source, want) {
			t.Errorf("source is missing %q:\n%s", want, res.Source)
		}
	}
}

// Integer and Real are one numeric family for the record definition: rows of
// either settle the member to Real, each literal staying its own.
func TestGenerateNumericFamilySettlesToReal(t *testing.T) {
	res, err := Generate(Request{
		Package: "P::Records", Case: "P::mix", Provenance: provenance(KindSweep),
		Runs: []Run{
			{Outputs: []runtime.CalcOutputValue{{Name: "half", Value: integer(3)}}, Spell: spell()},
			{Outputs: []runtime.CalcOutputValue{{Name: "half", Value: realValue(3.5)}}, Spell: spell()},
		},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{
		"attribute half : ScalarValues::Real;",
		"attribute :>> half = 3;",
		"attribute :>> half = 3.5;",
	} {
		if !strings.Contains(res.Source, want) {
			t.Errorf("source is missing %q:\n%s", want, res.Source)
		}
	}
}

// A declared Real member accepts Integer values — Integer specializes it —
// while a declared Integer member cannot take a Real back.
func TestGenerateExistingRealAcceptsAnInteger(t *testing.T) {
	req := Request{
		Package: "P::Records", Case: "P::mix", Provenance: provenance(KindRun),
		Existing: Existing{
			Package: true, Definition: true,
			Attributes: map[string]Feature{"half": {TypeFQN: "ScalarValues::Real"}},
		},
		Runs: []Run{{
			Outputs: []runtime.CalcOutputValue{{Name: "half", Value: integer(3)}},
			Spell:   spell(),
		}},
	}
	if _, err := Generate(req); err != nil {
		t.Fatalf("an Integer under a declared Real: %v", err)
	}
	req.Existing.Attributes["half"] = Feature{TypeFQN: "ScalarValues::Integer"}
	req.Runs[0].Outputs[0].Value = realValue(3.5)
	if _, err := Generate(req); err == nil {
		t.Error("a Real under a declared Integer: want an error")
	}
}

// A quantity member takes a plain-number row in either order: the row keeps
// its literal and takes no unit.
func TestGenerateQuantityAndPlainNumbers(t *testing.T) {
	kg := runtime.NewQuantityValue(&runtime.Quantity{
		Num:  semantics.Value{Kind: semantics.ValReal, Real: 3.0},
		Unit: semantics.Unit{Text: "kg"},
	})
	orders := map[string][]runtime.Value{
		"integer then quantity": {integer(2), kg},
		"quantity then integer": {kg, integer(2)},
	}
	for name, values := range orders {
		var runs []Run
		for _, v := range values {
			runs = append(runs, Run{
				Outputs: []runtime.CalcOutputValue{{Name: "x", Value: v}},
				Spell:   spell(),
			})
		}
		res, err := Generate(Request{
			Package: "P::Records", Case: "P::mix", Provenance: provenance(KindSweep), Runs: runs,
		})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, want := range []string{
			"attribute x : ScalarValues::Real;",
			"attribute xUnit : ScalarValues::String;",
			"attribute :>> x = 2;",
			"attribute :>> x = 3.0;",
			`attribute :>> xUnit = "kg";`,
		} {
			if !strings.Contains(res.Source, want) {
				t.Errorf("%s: source is missing %q:\n%s", name, want, res.Source)
			}
		}
		if strings.Count(res.Source, "xUnit = ") != 1 {
			t.Errorf("%s: a plain-number row wrote a unit:\n%s", name, res.Source)
		}
	}
}

// An inout whose two sides mix a quantity with a plain number settles both
// members to the quantity shape, each with its unit companion declared.
func TestGenerateInoutQuantityAndPlainNumber(t *testing.T) {
	kg := runtime.NewQuantityValue(&runtime.Quantity{
		Num:  semantics.Value{Kind: semantics.ValReal, Real: 3.0},
		Unit: semantics.Unit{Text: "kg"},
	})
	sides := map[string][2]runtime.Value{
		"integer out, quantity in": {integer(2), kg},
		"real out, quantity in":    {realValue(2.5), kg},
		"quantity out, integer in": {kg, integer(2)},
		"quantity out, real in":    {kg, realValue(2.5)},
	}
	for name, v := range sides {
		res, err := Generate(Request{
			Package: "P::Records", Case: "P::mix", Provenance: provenance(KindRun),
			Runs: []Run{{
				Inputs:  []runtime.InputBinding{{Name: "x", Value: v[1]}},
				Outputs: []runtime.CalcOutputValue{{Name: "x", Value: v[0]}},
				Spell:   spell(),
			}},
		})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, want := range []string{
			"attribute x : ScalarValues::Real;",
			"attribute xUnit : ScalarValues::String;",
			"attribute xIn : ScalarValues::Real;",
			"attribute xInUnit : ScalarValues::String;",
			`Unit = "kg";`,
		} {
			if !strings.Contains(res.Source, want) {
				t.Errorf("%s: source is missing %q:\n%s", name, want, res.Source)
			}
		}
		if strings.Count(res.Source, `Unit = "kg";`) != 1 {
			t.Errorf("%s: want exactly one unit redefinition:\n%s", name, res.Source)
		}
	}
}
