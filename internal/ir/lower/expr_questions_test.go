package lower

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestExprEntrySegments pins the names a reference and a chain step through, and
// that any other expression answers none.
func TestExprEntrySegments(t *testing.T) {
	values, scope, resolver := exprFixture(t)
	for name, want := range map[string][]string{
		"neg":     nil,
		"name":    {"P", "Q", "x"},
		"chain":   {"v", "m", "m"},
		"missing": {"nowhere"},
		"sum":     nil,
		"call":    nil,
		"nested":  nil,
	} {
		table, id := LowerExpr(values[name], scope, resolver)
		entry := table.Entry(id)
		if name == "neg" {
			entry = entry.Operands[0]
			want = []string{"x"}
		}
		got, ok := entry.Segments()
		if ok != (want != nil) || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: Segments = %v, %v; want %v", name, got, ok, want)
		}
	}
	if got, ok := (*ExprEntry)(nil).Segments(); ok || got != nil {
		t.Errorf("nil entry: Segments = %v, %v", got, ok)
	}
}

// TestExprEntryInvoked pins the names an invocation's function is written with, which
// no reference or chain answers.
func TestExprEntryInvoked(t *testing.T) {
	values, scope, resolver := exprFixture(t)
	for name, want := range map[string][]string{
		"call":   {"f"},
		"name":   nil,
		"chain":  nil,
		"nested": nil,
	} {
		table, id := LowerExpr(values[name], scope, resolver)
		got, ok := table.Entry(id).Invoked()
		if ok != (want != nil) || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: Invoked = %v, %v; want %v", name, got, ok, want)
		}
	}
}

// TestExprEntryReads pins what a value is read through: each name once, a chain
// whole, and a quantity's unit not at all.
func TestExprEntryReads(t *testing.T) {
	values, scope, resolver := exprFixture(t)
	type read struct {
		segments string
		chain    bool
	}
	reads := func(name string) []read {
		table, id := LowerExpr(values[name], scope, resolver)
		var out []read
		table.Entry(id).Reads(func(segments []string, chain, ok bool) bool {
			if !ok {
				t.Errorf("%s: a read names nothing", name)
			}
			text := ""
			for i, s := range segments {
				if i > 0 {
					text += "."
				}
				text += s
			}
			out = append(out, read{text, chain})
			return true
		})
		return out
	}
	for name, want := range map[string][]read{
		"sum":   {{"x", false}},
		"chain": {{"v.m.m", true}},
		"seq":   {{"x", false}},
		"at":    {{"seq", false}},
		"each":  {{"seq", false}, {"e", false}},
		"huge":  nil,
	} {
		if got := reads(name); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: reads %v, want %v", name, got, want)
		}
	}
	unitScope, unit := attributeExpression(t, `attribute m; part def T { attribute x = 1; attribute y = x [m]; }`, "y")
	table, id := LowerExpr(unit, unitScope, nil)
	var got []string
	table.Entry(id).Reads(func(segments []string, _, _ bool) bool {
		got = append(got, segments...)
		return true
	})
	if !reflect.DeepEqual(got, []string{"x"}) {
		t.Errorf("x [m]: reads %v, want only x", got)
	}
	stopped := 0
	table, id = LowerExpr(values["labelled"], scope, resolver)
	table.Entry(id).Reads(func([]string, bool, bool) bool { stopped++; return false })
	if stopped != 1 {
		t.Errorf("a visit answering false was called %d times, want 1", stopped)
	}
	(*ExprEntry)(nil).Reads(func([]string, bool, bool) bool { t.Error("nil entry read"); return true })
}

// TestExprEntryReferenceAndWritten pins which expressions read a feature by name and
// how one outside the records' kinds is named.
func TestExprEntryReferenceAndWritten(t *testing.T) {
	values, scope, resolver := exprFixture(t)
	for name, want := range map[string]struct {
		reference bool
		written   string
	}{
		"missing": {true, "*ast.FeatureReference"},
		"chain":   {true, "*ast.FeatureChainExpr"},
		"sum":     {false, "*ast.OperatorExpr"},
		"meta":    {false, "*ast.MetadataAccessExpr"},
	} {
		table, id := LowerExpr(values[name], scope, resolver)
		entry := table.Entry(id)
		if entry.Reference() != want.reference || entry.Written() != want.written {
			t.Errorf("%s: Reference %v Written %q, want %v %q", name, entry.Reference(), entry.Written(), want.reference, want.written)
		}
	}
	var none *ExprEntry
	if none.Reference() || none.Written() != "<nil>" {
		t.Errorf("nil entry: Reference %v Written %q", none.Reference(), none.Written())
	}
}

// TestLowerExprRecordsBodyShape pins what a body expression records: its parameters
// by name, how many other members it declares, and its result lowered in its own scope.
func TestLowerExprRecordsBodyShape(t *testing.T) {
	values, scope, resolver := exprFixture(t)
	table, id := LowerExpr(values["some"], scope, resolver)
	body := table.Entry(id).Operands[1]
	if body.Kind != program.ExprBody || len(body.Params) != 1 || body.Params[0].Name != "e" || body.Locals != 0 {
		t.Fatalf("body records %+v", body.Expr)
	}
	if body.Result == 0 || body.Answer != table.Entry(body.Result) || body.Answer.Kind != program.ExprBool {
		t.Fatalf("body result %d answers %+v", body.Result, body.Answer)
	}
	if body.Answer.Scope == scope {
		t.Errorf("the result was lowered in the enclosing scope, not the body's")
	}
	if len(body.Args) != 0 {
		t.Errorf("the result is recorded under the body: %v", body.Args)
	}
}

// TestDeclared pins the entry standing for a symbol's declaration.
func TestDeclared(t *testing.T) {
	_, scope, _ := exprFixture(t)
	sym := scope.Members()[0]
	entry := Declared(sym)
	if entry.Tree != sym.Decl || entry.Scope != sym.OwnerScope || entry.Origin.Doc != sym.DocName || entry.Origin.Span != sym.Decl.Span() {
		t.Errorf("Declared(%s) = %+v", sym.Name, entry)
	}
	if Declared(nil) != nil {
		t.Errorf("Declared(nil) is not nil")
	}
}

// TestExprEntrySpellsTheEmptyName pins that a reference or an invocation written
// with the empty unrestricted name names something: its one segment is "".
func TestExprEntrySpellsTheEmptyName(t *testing.T) {
	p := parser.New(source.New("empty.sysml", []byte("package P {\n\tattribute '' = 1;\n\tattribute read = '';\n\tattribute call = ''(1);\n}\n")))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc("empty.sysml", root)
	pkg := idx.LookupQualified("P")
	if len(pkg) != 1 || pkg[0].Scope == nil {
		t.Fatalf("P not indexed: %v", pkg)
	}
	scope, resolver := pkg[0].Scope, resolve.New(idx)
	valueOf := func(name string) *ExprEntry {
		t.Helper()
		sym, ok := scope.LookupLocal(name)
		if !ok {
			t.Fatalf("%s not declared", name)
		}
		table, id := LowerExpr(sym.Decl.(*ast.Usage).Value, scope, resolver)
		return table.Entry(id)
	}
	if got, ok := valueOf("read").Segments(); !ok || !reflect.DeepEqual(got, []string{""}) {
		t.Errorf("read: Segments = %q, %v; want [\"\"], true", got, ok)
	}
	if got, ok := valueOf("call").Invoked(); !ok || !reflect.DeepEqual(got, []string{""}) {
		t.Errorf("call: Invoked = %q, %v; want [\"\"], true", got, ok)
	}
}
