package lower

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const exprSrc = `
	package P {
		attribute def V { attribute m = 2; }
		part def Q {
			attribute x = 1;
			attribute v : V;
			attribute sum = x + 2.5;
			attribute name = P::Q::x;
			attribute chain = v.m.m;
			attribute seq = (x, true, "a\n", null, *);
			attribute at = seq#(1);
			attribute neg = -x;
			attribute cast = x as V;
			attribute call = f(x);
			attribute missing = nowhere;
			attribute huge = 99999999999999999999999999;
		}
	}
`

// exprFixture is the value of each attribute of P::Q in exprSrc, with Q's scope.
func exprFixture(t *testing.T) (map[string]ast.Node, *symbols.Scope, *resolve.Resolver) {
	t.Helper()
	p := parser.New(source.New("expr.sysml", []byte(exprSrc)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc("expr.sysml", root)
	q := idx.LookupQualified("P::Q")
	if len(q) != 1 || q[0].Scope == nil {
		t.Fatalf("P::Q not indexed: %v", q)
	}
	values := map[string]ast.Node{}
	for _, sym := range q[0].Scope.Members() {
		if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Value != nil {
			values[sym.Name] = usage.Value
		}
	}
	return values, q[0].Scope, resolve.New(idx)
}

// TestLowerExprRecordsEachKind pins what LowerExpr records for the kinds it
// lowers: operands by id, names resolved in the scope, constants read once.
func TestLowerExprRecordsEachKind(t *testing.T) {
	values, scope, resolver := exprFixture(t)
	lowerOf := func(name string) (*ExprTable, *program.Expr) {
		t.Helper()
		table, id := LowerExpr(values[name], scope, resolver)
		expr := table.At(id)
		if expr == nil || table.Tree(id) != values[name] || table.Entry(id).Scope != scope {
			t.Fatalf("%s: record %v does not answer for its tree and scope", name, expr)
		}
		if expr.Origin.Doc != "expr.sysml" || expr.Origin.Span != values[name].Span() {
			t.Fatalf("%s: origin = %+v", name, expr.Origin)
		}
		return table, expr
	}
	x, _ := scope.LookupLocal("x")

	table, sum := lowerOf("sum")
	if sum.Kind != program.ExprOp || sum.Op != program.OpAdd || len(sum.Args) != 2 {
		t.Fatalf("sum = %+v", sum)
	}
	left, right := table.At(sum.Args[0]), table.At(sum.Args[1])
	if left.Kind != program.ExprRef || left.Ref.Name != "x" || left.Ref.Sym != x || len(left.Path) != 0 {
		t.Fatalf("x = %+v", left)
	}
	want, _ := semantics.ParseRational("2.5", semantics.DefaultMaxIntegerBits)
	if right.Kind != program.ExprReal || right.Text != "2.5" || !right.Const.Equal(want) {
		t.Fatalf("2.5 = %+v", right)
	}

	_, name := lowerOf("name")
	if name.Kind != program.ExprRef || name.Ref.Name != "P::Q::x" || name.Ref.Sym != x || len(name.Path) != 3 {
		t.Fatalf("P::Q::x = %+v", name)
	}
	if name.Path[0].Name != "P" || name.Path[0].Sym == nil || name.Path[1].Sym != scope.Owner() || name.Path[2].Sym != x {
		t.Fatalf("path of P::Q::x = %+v", name.Path)
	}

	table, chain := lowerOf("chain")
	if chain.Kind != program.ExprChain || len(chain.Args) != 1 || len(chain.Chain) != 2 {
		t.Fatalf("v.m.m = %+v", chain)
	}
	// A member is declared by what holds it at run time, so it is recorded by its spelling.
	if base := table.At(chain.Args[0]); base.Kind != program.ExprRef || base.Ref.Name != "v" ||
		chain.Chain[0].Name != "m" || chain.Chain[1].Name != "m" || chain.Chain[0].Sym != nil {
		t.Fatalf("v.m.m reads %+v through %+v", base, chain.Chain)
	}

	table, seq := lowerOf("seq")
	kinds := []program.ExprKind{program.ExprRef, program.ExprBool, program.ExprString, program.ExprNull, program.ExprInfinity}
	if seq.Kind != program.ExprSequence || len(seq.Args) != len(kinds) {
		t.Fatalf("seq = %+v", seq)
	}
	for i, kind := range kinds {
		if got := table.At(seq.Args[i]).Kind; got != kind {
			t.Fatalf("element %d is kind %d, want %d", i, got, kind)
		}
	}
	if truth, text := table.At(seq.Args[1]), table.At(seq.Args[2]); !truth.Const.Bool || text.Text != "a\n" {
		t.Fatalf("true = %+v, string = %q", truth.Const, text.Text)
	}

	if _, at := lowerOf("at"); at.Kind != program.ExprIndex || at.Bracket || len(at.Args) != 2 {
		t.Fatalf("seq#(1) = %+v", at)
	}
	if _, neg := lowerOf("neg"); neg.Kind != program.ExprOp || neg.Op != program.OpNeg || len(neg.Args) != 1 {
		t.Fatalf("-x = %+v", neg)
	}
	v, _ := scope.Parent().LookupLocal("V")
	if _, cast := lowerOf("cast"); cast.Op != program.OpAs || len(cast.Args) != 1 || cast.Ref.Name != "V" || cast.Ref.Sym != v {
		t.Fatalf("x as V = %+v", cast)
	}
}

// TestLowerExprLeavesTheRestToTheRun pins what lowering does not decide: a tree-evaluated
// kind is a leaf, and a name or literal that is no value is recorded as written.
func TestLowerExprLeavesTheRestToTheRun(t *testing.T) {
	values, scope, resolver := exprFixture(t)
	table, id := LowerExpr(values["call"], scope, resolver)
	if call := table.At(id); call.Kind != program.ExprInvoke || len(call.Args) != 0 || table.At(id+1) != nil {
		t.Fatalf("f(x) = %+v, with %v beside it", call, table.At(id+1))
	}
	table, id = LowerExpr(values["missing"], scope, resolver)
	if missing := table.At(id); missing.Kind != program.ExprRef || missing.Ref.Name != "nowhere" || missing.Ref.Sym != nil {
		t.Fatalf("nowhere = %+v", missing)
	}
	table, id = LowerExpr(values["huge"], scope, resolver)
	want, _ := semantics.ParseInteger("99999999999999999999999999")
	if huge := table.At(id); huge.Kind != program.ExprInt || !huge.Const.Equal(want) {
		t.Fatalf("huge = %+v", huge)
	}
	// Without a resolver nothing is resolved, and the table still answers by id.
	table, id = LowerExpr(values["sum"], scope, nil)
	if x := table.At(table.At(id).Args[0]); x.Ref.Name != "x" || x.Ref.Sym != nil {
		t.Fatalf("x without a resolver = %+v", x)
	}
	if table.At(0) != nil || table.Tree(0) != nil || table.Entry(0) != nil || table.At(id+3) != nil {
		t.Fatal("the table answers for an id it does not hold")
	}
	var none *ExprTable
	if none.At(1) != nil || none.Tree(1) != nil || none.Entry(1) != nil {
		t.Fatal("a nil table answers")
	}
}

// TestOperatorMirrorsSyntax pins the IR's operators to the syntax tree's, one
// by one: the two convert by number, so a ladder reordered on one side fails here.
func TestOperatorMirrorsSyntax(t *testing.T) {
	mirror := map[ast.OperatorKind]program.Operator{
		ast.OpInvalid: program.OpInvalid, ast.OpConditional: program.OpConditional,
		ast.OpNullCoalesce: program.OpNullCoalesce, ast.OpImplies: program.OpImplies,
		ast.OpOr: program.OpOr, ast.OpConditionalOr: program.OpConditionalOr,
		ast.OpXor: program.OpXor, ast.OpAnd: program.OpAnd,
		ast.OpConditionalAnd: program.OpConditionalAnd, ast.OpEq: program.OpEq,
		ast.OpNeq: program.OpNeq, ast.OpEqEqEq: program.OpEqEqEq,
		ast.OpNeqEqEq: program.OpNeqEqEq, ast.OpHasType: program.OpHasType,
		ast.OpIsType: program.OpIsType, ast.OpAt: program.OpAt, ast.OpMetaAt: program.OpMetaAt,
		ast.OpAs: program.OpAs, ast.OpMeta: program.OpMeta, ast.OpLt: program.OpLt,
		ast.OpGt: program.OpGt, ast.OpLe: program.OpLe, ast.OpGe: program.OpGe,
		ast.OpRange: program.OpRange, ast.OpAdd: program.OpAdd, ast.OpSub: program.OpSub,
		ast.OpMul: program.OpMul, ast.OpDiv: program.OpDiv, ast.OpMod: program.OpMod,
		ast.OpPow: program.OpPow, ast.OpNeg: program.OpNeg, ast.OpPos: program.OpPos,
		ast.OpBitNot: program.OpBitNot, ast.OpNot: program.OpNot, ast.OpAll: program.OpAll,
		ast.OpIndex: program.OpIndex,
	}
	for kind := ast.OpInvalid; kind <= ast.OpIndex; kind++ {
		op, named := mirror[kind]
		if !named || OperatorOf(kind) != op || OperatorKindOf(op) != kind {
			t.Errorf("operator %q (%d) is %d in the IR, mirrored by %d (named %v)", kind, kind, OperatorOf(kind), op, named)
		}
	}
	if next := ast.OpIndex + 1; !strings.HasPrefix(next.String(), "OperatorKind(") {
		t.Errorf("the syntax tree has an operator %q the IR does not mirror", next)
	}
}
