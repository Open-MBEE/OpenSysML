package lower

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestChainedAssignmentBindsNoCalcOutput locks that a chained target writing another
// object's feature does not count as the calc computing its own output of that name.
func TestChainedAssignmentBindsNoCalcOutput(t *testing.T) {
	outputs := []program.CalcOutput{{Name: "x"}}
	chained := []Statement{Assign{Target: "x", Chain: &AssignTarget{Text: "foo.x"}}}
	if assigned := assignedOutputs(chained, outputs, nil); len(assigned) != 0 {
		t.Errorf("assignedOutputs(chained) = %v; want none, since foo.x is another object's feature", assigned)
	}
	plain := []Statement{Assign{Target: "x"}}
	if assigned := assignedOutputs(plain, outputs, nil); !assigned["x"] {
		t.Errorf("assignedOutputs(plain) = %v; want x, which the body does bind", assigned)
	}
}

const calcIdentitySrc = `
	package P {
		calc def Base { in a : Real = 1.0; in b : Real; return : Real = a + b; }
		calc def Derived :> Base { in c : Real = 2.0; out d : Real = a * c; }
		analysis def Study { subject s = 3; in n = 4; return : Real = n; }
	}
`

// identityCalc lowers the member of P named name in src, with the environment a
// runtime would hand the lowering.
func identityCalc(t *testing.T, src, name string) (*program.Calc, *CalcTrees) {
	t.Helper()
	p := parser.New(source.New("calc.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc("calc.sysml", root)
	resolver := resolve.New(idx)
	sym, _ := idx.DocumentRoot("calc.sysml").ChildFor(root.Members[0].(*ast.Membership).Member).LookupLocal(name)
	if sym == nil {
		t.Fatalf("P::%s not indexed", name)
	}
	return ToCalc(sym, CalcEnv{Semantics: semantics.NewModel(resolver), Resolver: resolver})
}

// TestToCalcResolvesTheChain pins what ToCalc records for an inherited interface:
// parameters most general first, redeclared outputs, the result and the body owner.
func TestToCalcResolvesTheChain(t *testing.T) {
	calc, trees := identityCalc(t, calcIdentitySrc, "Derived")
	if calc.Name != "P::Derived" || calc.Kind != "calc" || calc.Label != "calc P::Derived" {
		t.Fatalf("identity = %q %q %q", calc.Name, calc.Kind, calc.Label)
	}
	if calc.Origin.Path != "P::Derived" || calc.Origin.Doc != "calc.sysml" {
		t.Fatalf("origin = %+v", calc.Origin)
	}
	if got := calc.ParamNames; !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("ParamNames = %v", got)
	}
	if len(calc.Chain) != 2 || calc.Chain[1] != calc.Sym {
		t.Fatalf("Chain = %v", calc.Chain)
	}
	if calc.Params[0].Default == 0 || calc.Params[1].Default != 0 || calc.Params[2].Default == 0 {
		t.Fatalf("defaults = %v %v %v", calc.Params[0].Default, calc.Params[1].Default, calc.Params[2].Default)
	}
	if trees.Expr(calc.Params[0].Default) == nil || trees.Expr(calc.Params[1].Default) != nil {
		t.Fatal("Expr does not answer the defaults' ids")
	}
	if len(calc.Params[0].Declarations) != 1 || calc.Params[0].Declarations[0].Owner != calc.Chain[0] {
		t.Fatalf("Declarations of a = %+v", calc.Params[0].Declarations)
	}
	if at := calc.ResultOutputIndex(); at < 0 || calc.Outputs[at].Value == 0 || calc.Outputs[at].Owner != calc.Chain[0] {
		t.Fatalf("result output = %d %+v", at, calc.Outputs)
	}
	// The `out d = a * c` binding is a return of the body, not a step the invocation runs.
	if !calc.Returns || calc.StepsReturn || calc.Result != 0 || calc.BodyOwner != calc.Sym {
		t.Fatalf("Returns=%v StepsReturn=%v Result=%v BodyOwner=%v", calc.Returns, calc.StepsReturn, calc.Result, calc.BodyOwner)
	}
}

// TestToCalcIDsDenseAndDeterministic pins the expression table: ids dense from 1
// in the order the record states them, the same across two lowerings.
func TestToCalcIDsDenseAndDeterministic(t *testing.T) {
	first, trees := identityCalc(t, calcIdentitySrc, "Study")
	second, _ := identityCalc(t, calcIdentitySrc, "Study")
	var ids []program.ExprID
	for _, p := range first.Params {
		ids = append(ids, p.Default)
	}
	for _, out := range first.Outputs {
		ids = append(ids, out.Value)
	}
	ids = append(ids, first.Result)
	seen := map[program.ExprID]bool{}
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if seen[id] || int(id) >= len(trees.exprs) || trees.exprs[id] == nil {
			t.Fatalf("id %d is not a distinct table index: %v", id, ids)
		}
		seen[id] = true
	}
	if len(seen) != len(trees.exprs)-1 {
		t.Fatalf("%d ids for %d expressions", len(seen), len(trees.exprs)-1)
	}
	if first.Kind != "analysis" || !first.Params[0].IsSubject || first.ParamNames[0] != "s" {
		t.Fatalf("subject not first: %+v", first.ParamNames)
	}
	first.Sym, second.Sym, first.Chain, second.Chain = nil, nil, nil, nil
	for i := range first.Params {
		first.Params[i].Sym, first.Params[i].Owner, first.Params[i].Declarations = nil, nil, nil
		second.Params[i].Sym, second.Params[i].Owner, second.Params[i].Declarations = nil, nil, nil
	}
	for i := range first.Outputs {
		first.Outputs[i].Owner, first.Outputs[i].Declarations = nil, nil
		second.Outputs[i].Owner, second.Outputs[i].Declarations = nil, nil
	}
	first.BodyOwner, second.BodyOwner = nil, nil
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("two lowerings differ:\n%+v\n%+v", first, second)
	}
}
