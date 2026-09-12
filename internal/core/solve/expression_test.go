package solve

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// actionBody lowers the named action and returns the translator for its subject
// and its body's statements, which is how an encoding of a behavior reads them.
func actionBody(t *testing.T, src, name string) (*Translator, *lower.ActionGraph, []lower.Statement) {
	t.Helper()
	ctx, idx := fixture(t, "<test>", src)
	sym := symbolNamed(t, idx, name)
	graph, err := lower.ToActionGraph(sym.Decl, sym.Scope)
	if err != nil {
		t.Fatalf("lower %s: %v", name, err)
	}
	var body []lower.Statement
	for _, node := range graph.Nodes {
		body = append(body, graph.Bodies[node]...)
	}
	x, err := NewTranslator(ctx, Subject{Kind: "action", Name: name, Symbol: sym})
	if err != nil {
		t.Fatalf("start translating %s: %v", name, err)
	}
	return x, graph, body
}

// assignments are the assignments among the statements, in order.
func assignments(body []lower.Statement) []lower.Assign {
	var out []lower.Assign
	for _, stmt := range body {
		if a, ok := stmt.(lower.Assign); ok {
			out = append(out, a)
		}
	}
	return out
}

// TestTranslatorReadsFeaturesAsOneVariableEach: the expressions of a body read
// each feature as one variable named and sorted as a condition's translation
// names it, with a datatype sort declared once for an enumeration read.
func TestTranslatorReadsFeaturesAsOneVariableEach(t *testing.T) {
	x, _, body := actionBody(t, `
		package test {
			private import ScalarValues::*;
			enum def Mode { enum idle; enum busy; }
			action def Count {
				attribute n : Natural;
				attribute total : Integer;
				attribute mode : Mode;
				attribute done : Boolean;
				action step {
					assign total := total + n * 2;
					assign done := mode == Mode::busy and total > 10;
				}
			}
		}`, "test::Count")
	assigns := assignments(body)
	if len(assigns) != 2 {
		t.Fatalf("lowered %d assignments, want 2", len(assigns))
	}
	sum, err := x.Expression(assigns[0].Value, assigns[0].Scope, "assign total")
	if err != nil {
		t.Fatalf("translate the sum: %v", err)
	}
	if !sum.Term.Sort.Equal(Int) || len(sum.Defined) != 2 {
		t.Fatalf("the sum is %s with %d side conditions, want Int within int64 for the product and the sum",
			sum.Term.Sort.Name, len(sum.Defined))
	}
	flag, err := x.Boolean(assigns[1].Value, assigns[1].Scope, "assign done")
	if err != nil {
		t.Fatalf("translate the flag: %v", err)
	}
	if got := writeTerm(flag.Term); !strings.Contains(got, "|test::Mode::busy|") {
		t.Errorf("the flag reads the literal by its qualified name, got %s", got)
	}
	var names []string
	for _, v := range x.Vars() {
		names = append(names, v.Name+":"+v.Sort.Name)
	}
	want := "test::Count::mode:test::Mode test::Count::n:Int test::Count::total:Int"
	if got := strings.Join(names, " "); got != want {
		t.Errorf("variables %s, want %s", got, want)
	}
	if sorts := x.Sorts(); len(sorts) != 1 || sorts[0].Name != "test::Mode" || len(sorts[0].Values) != 2 {
		t.Errorf("sorts %+v, want test::Mode with two values", sorts)
	}
	domains := x.Domains()
	if len(domains) != 1 || domains[0].From.Element != "test::Count::n" {
		t.Fatalf("domains %+v, want one, the Natural's", domains)
	}
	// A variable read twice is one variable, so a substitution keyed by it
	// replaces every read.
	steps := map[*Var]*Term{}
	for _, v := range x.Vars() {
		steps[v] = VarTerm(&Var{Name: v.Name + "@1", Sort: v.Sort, Symbol: v.Symbol})
	}
	next := Substitute(sum.Term, func(v *Var) *Term { return steps[v] })
	if got := writeTerm(next); got != "(+ |test::Count::total@1| (* |test::Count::n@1| 2))" {
		t.Errorf("substituted %s", got)
	}
	if writeTerm(sum.Term) != "(+ |test::Count::total| (* |test::Count::n| 2))" {
		t.Errorf("substitution changed the original: %s", writeTerm(sum.Term))
	}
}

// TestTranslatorReportsDefinednessPerExpression: a computed divisor is a side
// condition of the expression that reads it, not of the whole translation, and
// two expressions dividing by the same feature each carry it.
func TestTranslatorReportsDefinednessPerExpression(t *testing.T) {
	x, _, body := actionBody(t, `
		package test {
			private import ScalarValues::*;
			action def Ratio {
				attribute a : Integer;
				attribute b : Integer;
				attribute q : Integer;
				attribute r : Integer;
				action step {
					assign q := a / b;
					assign r := a % b;
				}
			}
		}`, "test::Ratio")
	assigns := assignments(body)
	for i, a := range assigns {
		expr, err := x.Expression(a.Value, a.Scope, "assign "+a.Target)
		if err != nil {
			t.Fatalf("translate assignment %d: %v", i, err)
		}
		if len(expr.Defined) != 1 {
			t.Fatalf("assignment %d carries %d side conditions, want the divisor's", i, len(expr.Defined))
		}
		if got := writeTerm(expr.Defined[0]); got != "(distinct |test::Ratio::b| 0)" {
			t.Errorf("assignment %d is defined where %s", i, got)
		}
	}
}

// TestTranslatorDefinesIntegerArithmeticWithinInt64: a sum, difference,
// product or negation of Integers is defined where its result is an int64, as
// the evaluator reports overflow; a Real one and a literal carry no such condition.
func TestTranslatorDefinesIntegerArithmeticWithinInt64(t *testing.T) {
	x, _, body := actionBody(t, `
		package test {
			private import ScalarValues::*;
			action def Arith {
				attribute a : Integer;
				attribute b : Integer;
				attribute r : Real;
				action step {
					assign a := a + b;
					assign a := a - b;
					assign a := a * b;
					assign a := -a;
					assign a := 5;
					assign r := r + 1.0;
				}
			}
		}`, "test::Arith")
	assigns := assignments(body)
	if len(assigns) != 6 {
		t.Fatalf("lowered %d assignments, want 6", len(assigns))
	}
	const within = "(<= (- 9223372036854775808) %s 9223372036854775807)"
	want := []string{
		"(+ |test::Arith::a| |test::Arith::b|)",
		"(- |test::Arith::a| |test::Arith::b|)",
		"(* |test::Arith::a| |test::Arith::b|)",
		"(- |test::Arith::a|)",
	}
	for i, result := range want {
		expr, err := x.Expression(assigns[i].Value, assigns[i].Scope, "assign a")
		if err != nil {
			t.Fatalf("translate assignment %d: %v", i, err)
		}
		if len(expr.Defined) != 1 {
			t.Fatalf("assignment %d carries %d side conditions, want the int64 range", i, len(expr.Defined))
		}
		if expr.Defined[0].Op != OpInt64 {
			t.Errorf("assignment %d is defined by %v, want OpInt64", i, expr.Defined[0].Op)
		}
		if got := writeTerm(expr.Defined[0]); got != fmt.Sprintf(within, result) {
			t.Errorf("assignment %d is defined where %s", i, got)
		}
	}
	for i := 4; i < 6; i++ {
		expr, err := x.Expression(assigns[i].Value, assigns[i].Scope, "assign")
		if err != nil {
			t.Fatalf("translate assignment %d: %v", i, err)
		}
		if len(expr.Defined) != 0 {
			t.Errorf("assignment %d carries %d side conditions, want none", i, len(expr.Defined))
		}
	}
}

// TestTranslatorRefusesNamingTheStatement: a construct outside the subset refuses
// with ErrNotTranslatable, the refusal naming the statement it appeared in.
func TestTranslatorRefusesNamingTheStatement(t *testing.T) {
	x, _, body := actionBody(t, `
		package test {
			private import ScalarValues::*;
			action def Power {
				attribute a : Integer;
				attribute p : Integer;
				attribute ok : Boolean;
				action step {
					assign p := a ** 2;
					assign ok := a;
				}
			}
		}`, "test::Power")
	assigns := assignments(body)
	_, err := x.Expression(assigns[0].Value, assigns[0].Scope, "assign p := a ** 2")
	var refused *NotTranslatableError
	if !errors.As(err, &refused) || !errors.Is(err, ErrNotTranslatable) {
		t.Fatalf("translating a ** 2: %v, want a NotTranslatableError", err)
	}
	if refused.Condition != "assign p := a ** 2" || refused.Element != "action test::Power" {
		t.Errorf("refusal names %q in %q, want the statement in the action", refused.Condition, refused.Element)
	}
	if refused.Span.Len == 0 || refused.Location == "" {
		t.Errorf("refusal carries no position: %+v", refused)
	}
	_, err = x.Boolean(assigns[1].Value, assigns[1].Scope, "assign ok := a")
	if !errors.As(err, &refused) || !strings.Contains(refused.Reason, "Int rather than a boolean") {
		t.Fatalf("translating a as a condition: %v, want a refusal for its sort", err)
	}
	if _, err := x.Expression(nil, assigns[0].Scope, "empty"); !errors.Is(err, ErrNotTranslatable) {
		t.Errorf("translating no expression: %v, want a refusal", err)
	}
}

// TestTranslatorResolvesBodyLocals: a name a body declares resolves in the
// statement's own scope, as the evaluator resolves it, and reads through a
// feature chain refuse as a condition's do.
func TestTranslatorResolvesBodyLocals(t *testing.T) {
	x, _, body := actionBody(t, `
		package test {
			private import ScalarValues::*;
			action def Local {
				attribute total : Integer;
				action step {
					attribute x : Integer = 3;
					assign total := total + x;
				}
			}
		}`, "test::Local")
	assigns := assignments(body)
	if len(assigns) != 1 {
		t.Fatalf("lowered %d assignments, want 1", len(assigns))
	}
	expr, err := x.Expression(assigns[0].Value, assigns[0].Scope, "assign total")
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if got := writeTerm(expr.Term); got != "(+ |test::Local::total| |test::Local::step::x|)" {
		t.Errorf("translated %s", got)
	}
	var local *Var
	for _, v := range x.Vars() {
		if v.Name == "test::Local::step::x" {
			local = v
		}
	}
	if local == nil || local.Symbol == nil || local.Symbol.Name != "x" {
		t.Fatalf("the local reads no variable standing for its declaration: %+v", local)
	}
}

// TestNewTranslatorNeedsAContext mirrors the condition translations.
func TestNewTranslatorNeedsAContext(t *testing.T) {
	if _, err := NewTranslator(nil, Subject{Kind: "action", Name: "x"}); err == nil {
		t.Fatal("translating with no context succeeded")
	}
	var ctx *runtime.Context
	if _, err := NewTranslator(ctx, Subject{}); err == nil {
		t.Fatal("translating with a nil context succeeded")
	}
}

// stepQuery is a hand-assembled query of the shape an encoding of moves emits: a
// datatype of nodes declared by the query itself rather than by the model, a
// variable per step over it, and an integer the steps count.
func stepQuery() (*Query, []*Var) {
	nodes := Sort{Kind: SortDatatype, Name: "Node", Origin: "Node", Values: []string{"start", "middle", "end"}}
	at0 := &Var{Name: "at@0", Sort: nodes}
	at1 := &Var{Name: "at@1", Sort: nodes}
	count := &Var{Name: "count@1", Sort: Int}
	moved := &Var{Name: "moved@1", Sort: Bool}
	from := Provenance{Kind: "step", Element: "walk", Condition: "one move", Role: RoleRequired}
	q := &Query{
		Kind:    "step",
		Element: "walk",
		Sorts:   []Sort{nodes},
		Vars:    []*Var{at0, at1, count, moved},
		Assertions: []Assertion{
			{Term: Binary(OpEq, Bool, VarTerm(at0), ValueTerm(nodes, "start")), From: from},
			{Term: Binary(OpNe, Bool, VarTerm(at1), ValueTerm(nodes, "start")), From: from},
			{Term: Binary(OpEq, Bool, VarTerm(moved), Binary(OpEq, Bool, VarTerm(at1), ValueTerm(nodes, "end"))), From: from},
			{Term: Binary(OpEq, Bool, VarTerm(count), Ite(VarTerm(moved), IntTerm(2), IntTerm(1))), From: from},
		},
	}
	return q, []*Var{at1, count, moved}
}

// TestEnumerateReportsEveryAssignmentToTheNamedVariables: an enumeration over
// named variables reports each distinct assignment to them once, denying each
// one reported, and stops at unsat having shown there is no other.
func TestEnumerateReportsEveryAssignmentToTheNamedVariables(t *testing.T) {
	solver := requireSolver(t)
	q, vars := stepQuery()
	result, err := solver.Enumerate(context.Background(), q, vars, 10)
	if err != nil {
		t.Fatalf("enumerate: %v", err)
	}
	if result.Status != StatusSat || result.Truncated {
		t.Fatalf("status %s, truncated %v, want every assignment reported", result.Status, result.Truncated)
	}
	if len(result.Solutions) != 2 {
		t.Fatalf("reported %d assignments, want 2: %+v", len(result.Solutions), result.Solutions)
	}
	seen := map[string]bool{}
	for _, solution := range result.Solutions {
		if len(solution) != 3 {
			t.Fatalf("a solution assigns %d variables, want the 3 named", len(solution))
		}
		at, err := DecodeValue(solution[0])
		if err != nil || at.Kind != SortDatatype {
			t.Fatalf("decode %s: %v (%+v)", solution[0].Raw, err, at)
		}
		count, err := DecodeValue(solution[1])
		if err != nil || count.Kind != SortInt {
			t.Fatalf("decode %s: %v (%+v)", solution[1].Raw, err, count)
		}
		moved, err := DecodeValue(solution[2])
		if err != nil || moved.Kind != SortBool {
			t.Fatalf("decode %s: %v (%+v)", solution[2].Raw, err, moved)
		}
		if seen[at.Text] {
			t.Errorf("%s was reported twice", at.Text)
		}
		seen[at.Text] = true
		wantMoved := at.Text == "end"
		wantCount := int64(1)
		if wantMoved {
			wantCount = 2
		}
		if moved.Bool != wantMoved || count.Number.Num().Int64() != wantCount {
			t.Errorf("at %s: moved %v count %s, want %v and %d", at.Text, moved.Bool, count.Number, wantMoved, wantCount)
		}
	}
	if !seen["middle"] || !seen["end"] {
		t.Errorf("reported %v, want middle and end", seen)
	}
}

// TestEnumerateStopsAtItsBound: a limit truncates the enumeration at the bound
// and says so; no limit asks for one assignment.
func TestEnumerateStopsAtItsBound(t *testing.T) {
	solver := requireSolver(t)
	q, vars := stepQuery()
	result, err := solver.Enumerate(context.Background(), q, vars, 0)
	if err != nil {
		t.Fatalf("enumerate: %v", err)
	}
	if result.Status != StatusSat || !result.Truncated || !result.AtBound || len(result.Solutions) != 1 {
		t.Fatalf("status %s, truncated %v, at bound %v, %d solutions; want one at the bound",
			result.Status, result.Truncated, result.AtBound, len(result.Solutions))
	}
}

// TestEnumerateRefusesVariablesTheQueryLacks: a variable the query does not
// declare is a typed error before the solver runs, and asking over none is an
// error too.
func TestEnumerateRefusesVariablesTheQueryLacks(t *testing.T) {
	q, _ := stepQuery()
	solver := &Solver{Name: "none", Path: "/nonexistent/solver"}
	stranger := &Var{Name: "elsewhere", Sort: Int}
	_, err := solver.Enumerate(context.Background(), q, []*Var{stranger}, 1)
	var undeclared *NotDeclaredError
	if !errors.As(err, &undeclared) || !errors.Is(err, ErrNotDeclared) || undeclared.Variable != "elsewhere" {
		t.Fatalf("enumerating an undeclared variable: %v, want a NotDeclaredError naming it", err)
	}
	if _, err := solver.Enumerate(context.Background(), q, nil, 1); err == nil {
		t.Fatal("enumerating no variable succeeded")
	}
	if _, err := solver.Enumerate(context.Background(), nil, nil, 1); err == nil {
		t.Fatal("enumerating no query succeeded")
	}
}

// TestDecodeValueReadsEachSortExactly: a model value decodes to its sort's
// representation, exact for numbers, and a value the sort does not hold is an
// error rather than a guess.
func TestDecodeValueReadsEachSortExactly(t *testing.T) {
	nodes := Sort{Kind: SortDatatype, Name: "Node", Origin: "Node", Values: []string{"start", "end"}}
	cases := []struct {
		name string
		a    Assignment
		want string
		err  string
	}{
		{"bool", Assignment{Var: &Var{Name: "b", Sort: Bool}, Raw: "true"}, "Bool:true", ""},
		{"int", Assignment{Var: &Var{Name: "i", Sort: Int}, Raw: "(- 7)"}, "Int:-7", ""},
		{"wide int", Assignment{Var: &Var{Name: "i", Sort: Int}, Raw: "18446744073709551616"}, "Int:18446744073709551616", ""},
		{"real", Assignment{Var: &Var{Name: "r", Sort: Real}, Raw: "(/ 1.0 3.0)"}, "Real:1/3", ""},
		{"string", Assignment{Var: &Var{Name: "s", Sort: String}, Raw: `"x"`}, "String:x", ""},
		{"datatype", Assignment{Var: &Var{Name: "n", Sort: nodes}, Raw: "end"}, "Datatype:end", ""},
		{"foreign datatype value", Assignment{Var: &Var{Name: "n", Sort: nodes}, Raw: "elsewhere"}, "", "elsewhere is not a value of Node"},
		{"fraction for an int", Assignment{Var: &Var{Name: "i", Sort: Int}, Raw: "(/ 1 2)"}, "", "no integer"},
		{"algebraic real", Assignment{Var: &Var{Name: "r", Sort: Real}, Raw: "(root-obj (+ (^ x 2) (- 2)) 2)"}, "", "no rational"},
		{"no variable", Assignment{Raw: "1"}, "", "no variable"},
		{"unquoted string", Assignment{Var: &Var{Name: "s", Sort: String}, Raw: "x"}, "", "unreadable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := DecodeValue(c.a)
			if c.err != "" {
				if err == nil || !strings.Contains(err.Error(), c.err) {
					t.Fatalf("decoded %+v with error %v, want one containing %q", got, err, c.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if rendered := renderDecoded(got); rendered != c.want {
				t.Errorf("decoded %s, want %s", rendered, c.want)
			}
		})
	}
}

// renderDecoded writes a decoded value for a comparison.
func renderDecoded(v ModelValue) string {
	switch v.Kind {
	case SortBool:
		if v.Bool {
			return "Bool:true"
		}
		return "Bool:false"
	case SortInt:
		return "Int:" + v.Number.Num().String()
	case SortReal:
		return "Real:" + v.Number.RatString()
	case SortString:
		return "String:" + v.Text
	}
	return "Datatype:" + v.Text
}

// TestDecodedValueDeniesItself: the literal a decoded value denotes is what a
// blocking clause writes, and an integer no literal holds is reported.
func TestDecodedValueDeniesItself(t *testing.T) {
	wide, err := DecodeValue(Assignment{Var: &Var{Name: "i", Sort: Int}, Raw: "18446744073709551616"})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, err := wide.Literal(Int); err == nil || !strings.Contains(err.Error(), "outside the Integer range") {
		t.Fatalf("a literal for a wide integer: %v, want a range error", err)
	}
	third, err := DecodeValue(Assignment{Var: &Var{Name: "r", Sort: Real}, Raw: "(/ 1.0 3.0)"})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	literal, err := third.Literal(Real)
	if err != nil {
		t.Fatalf("literal: %v", err)
	}
	if got := writeTerm(literal); got != "(/ 1.0 3.0)" {
		t.Errorf("the literal writes as %s", got)
	}
}

// TestSubstituteSharesUnchangedTerms: a substitution shares the subterms it
// leaves alone and keeps a variable no replacement is given for.
func TestSubstituteSharesUnchangedTerms(t *testing.T) {
	a, b := &Var{Name: "a", Sort: Int}, &Var{Name: "b", Sort: Int}
	sum := Binary(OpAdd, Int, VarTerm(a), IntTerm(1))
	whole := Binary(OpLt, Bool, sum, VarTerm(b))
	if Substitute(whole, func(*Var) *Term { return nil }) != whole {
		t.Error("a substitution replacing nothing copied the term")
	}
	if Substitute(nil, func(*Var) *Term { return nil }) != nil {
		t.Error("substituting into no term made one")
	}
	c := &Var{Name: "c", Sort: Int}
	got := Substitute(whole, func(v *Var) *Term {
		if v == b {
			return VarTerm(c)
		}
		return nil
	})
	if got.Args[0] != sum {
		t.Error("the untouched operand was copied")
	}
	if writeTerm(got) != "(< (+ a 1) c)" {
		t.Errorf("substituted %s", writeTerm(got))
	}
}

// TestTranslatorConditionsJudgeTheSetAsTheEvaluatorDoes: the conditions of a
// requirement translate to the conjunction of the required ones, an assumed one
// only adding its side conditions; a negated declaration denies that conjunction,
// and one with nothing required refuses.
func TestTranslatorConditionsJudgeTheSetAsTheEvaluatorDoes(t *testing.T) {
	ctx, idx := fixture(t, "<test>", `
		package test {
			private import ScalarValues::*;
			part def P {
				attribute a : Integer;
				attribute b : Integer;
				requirement safe { assume a / b > 0; require a > 1; require b > 2; }
				assert not constraint unsafe { a > 10 }
				assert not constraint vacuous { assume a > 0; }
			}
		}`)
	translate := func(name string) (*Expression, error) {
		sym := symbolNamed(t, idx, "test::P::"+name)
		x, err := NewTranslator(ctx, Subject{Kind: "condition", Name: name, Symbol: sym, Negated: runtime.NegatedDecl(sym)})
		if err != nil {
			t.Fatalf("start translating %s: %v", name, err)
		}
		return x.Conditions(ctx.ConditionsOf(sym, nil))
	}
	safe, err := translate("safe")
	if err != nil {
		t.Fatalf("safe: %v", err)
	}
	if got := writeTerm(safe.Term); got != "(and (> |test::P::a| 1) (> |test::P::b| 2))" {
		t.Errorf("safe judges %s", got)
	}
	if len(safe.Defined) != 1 || writeTerm(safe.Defined[0]) != "(distinct |test::P::b| 0)" {
		t.Errorf("safe is defined where %v, want the assumption's divisor", safe.Defined)
	}
	unsafe, err := translate("unsafe")
	if err != nil {
		t.Fatalf("unsafe: %v", err)
	}
	if got := writeTerm(unsafe.Term); got != "(not (> |test::P::a| 10))" {
		t.Errorf("not unsafe judges %s", got)
	}
	if _, err := translate("vacuous"); !errors.Is(err, ErrNoConditions) {
		t.Errorf("not vacuous: %v, want ErrNoConditions", err)
	}
}
