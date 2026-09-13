package runtime

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func parseAndBuildModel(t *testing.T, code string) (*semantics.Model, *resolve.Resolver, *symbols.Scope) {
	t.Helper()
	src := source.New("test.sysml", []byte(code))
	p := parser.New(src)
	root := p.ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("test.sysml", root)
	rootScope := idx.DocumentRoot("test.sysml")
	if rootScope == nil {
		t.Fatal("rootScope nil")
	}
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	return model, resolver, rootScope
}

// parseAndBuildLibraryModel is parseAndBuildModel over the standard library, for
// a model whose imports name library packages.
func parseAndBuildLibraryModel(t *testing.T, code string) (*semantics.Model, *resolve.Resolver, *symbols.Scope) {
	t.Helper()
	root := parser.New(source.New("test.sysml", []byte(code))).ParseFile()
	idx := libs.NewModelIndex()
	idx.AddDocument("test.sysml", root)
	idx.ExpandWildcardImports()
	rootScope := idx.DocumentRoot("test.sysml")
	if rootScope == nil {
		t.Fatal("rootScope nil")
	}
	resolver := resolve.New(idx)
	return semantics.NewModel(resolver), resolver, rootScope
}

func resolveSymbol(t *testing.T, rootScope *symbols.Scope, name string) *symbols.Symbol {
	t.Helper()
	sym, ok := rootScope.LookupLocal(name)
	if !ok || sym == nil {
		t.Fatalf("symbol %q not found", name)
	}
	return sym
}

func TestEval_Literals(t *testing.T) {
	tests := []struct {
		src      string
		expected Value
	}{
		{"42", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 42}}},
		{"3.14", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 3.14}}},
		{"true", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: true}}},
		{`"hello"`, NewStringValue("hello")},
		{"null", Value{Kind: ValNull}},
	}

	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			// Wrap expression in attribute default
			model, resolver, root := parseAndBuildModel(t, "attribute test = "+tt.src+";")
			ctx := NewContext(NewModel(model, resolver), 1000)

			// Extract expression from attribute value
			attrSym := resolveSymbol(t, root, "test")
			attrDecl := attrSym.Decl.(*ast.Usage)
			expr := attrDecl.Value

			result, err := ctx.Eval(expr)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}

			if result.Kind != tt.expected.Kind {
				t.Errorf("expected Kind %v, got %v", tt.expected.Kind, result.Kind)
			}
		})
	}
}

func TestEval_Arithmetic(t *testing.T) {
	src := `attribute test = 1 + 2;`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 1000)

	attrSym := resolveSymbol(t, root, "test")
	attrDecl := attrSym.Decl.(*ast.Usage)
	expr := attrDecl.Value

	result, err := ctx.Eval(expr)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	if result.Kind != ValConst || result.Const.Int != 3 {
		t.Errorf("expected 3, got %v", result)
	}
}

func TestEval_SequenceExpr(t *testing.T) {
	// Test (1, 2, 3) sequence construction
	src := `attribute test = (1, 2, 3);`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 1000)

	attrSym := resolveSymbol(t, root, "test")
	attrDecl := attrSym.Decl.(*ast.Usage)
	expr := attrDecl.Value

	result, err := ctx.Eval(expr)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	if result.Kind != ValSequence {
		t.Fatalf("expected ValSequence, got %v", result.Kind)
	}
	if result.Sequence().Size() != 3 {
		t.Errorf("expected size 3, got %d", result.Sequence().Size())
	}

	// Check elements
	elem0, _ := result.Sequence().At(0)
	if elem0.Kind != ValConst || elem0.Const.Int != 1 {
		t.Errorf("elem[0] expected 1, got %v", elem0)
	}
}

func TestEval_StepLimit(t *testing.T) {
	// Verify step counter triggers on deep recursion
	// (Step counter already wired in Context.incrementStep + eval.go)
	src := `part def Simple {}`
	model, resolver, _ := parseAndBuildModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 5) // very low limit

	// The budget bounds one run, so the literals are evaluated within one rather
	// than starting a run each.
	defer ctx.beginRun()()

	// Eval 6 literals → should exceed 5 steps
	for i := 0; i < 6; i++ {
		_, err := ctx.Eval(&ast.LiteralInteger{Value: "1"})
		if err != nil {
			if errors.Is(err, ErrStepLimitExceeded) {
				return // success — limit triggered
			}
			t.Fatalf("unexpected error: %v", err)
		}
	}
	t.Error("expected ErrStepLimitExceeded but got none")
}

func TestEval_QualifiedNameLookup(t *testing.T) {
	tests := []struct {
		src      string
		expected int64
	}{
		// Qualified name: nested namespace
		{"package A { attribute x = 42; } attribute test = A::x;", 42},
		// Qualified name: nested definition
		{"part def Vehicle { attribute speed = 100; } attribute test = Vehicle::speed;", 100},
		// Multi-level qualified name
		{"package A { package B { attribute val = 7; } } attribute test = A::B::val;", 7},
	}

	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			model, resolver, root := parseAndBuildModel(t, tt.src)
			ctx := NewContext(NewModel(model, resolver), 1000)

			testSym := resolveSymbol(t, root, "test")
			testDecl := testSym.Decl.(*ast.Usage)

			result, err := ctx.Eval(testDecl.Value)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}

			if result.Kind != ValConst || result.Const.Kind != semantics.ValInt || result.Const.Int != tt.expected {
				t.Errorf("expected int %d, got %v", tt.expected, result)
			}
		})
	}
}

func TestEval_EqualityConst(t *testing.T) {
	tests := []struct {
		src      string
		expected bool
	}{
		{"42 == 42", true},
		{"42 == 43", false},
		{"3.14 == 3.14", true},
		{"3.14 != 3.15", true},
		{"true == true", true},
		{"true == false", false},
		{"false != true", true},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			model, resolver, root := parseAndBuildModel(t, "attribute test = "+tt.src+";")
			ctx := NewContext(NewModel(model, resolver), 1000)
			attrSym := resolveSymbol(t, root, "test")
			attrDecl := attrSym.Decl.(*ast.Usage)
			result, err := ctx.Eval(attrDecl.Value)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}
			if result.Kind != ValConst || result.Const.Kind != semantics.ValBool {
				t.Fatalf("expected bool, got %v", result)
			}
			if result.Const.Bool != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result.Const.Bool)
			}
		})
	}
}

func TestEval_EqualityString(t *testing.T) {
	tests := []struct {
		src      string
		expected bool
	}{
		{`"hello" == "hello"`, true},
		{`"hello" == "world"`, false},
		{`"foo" != "bar"`, true},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			model, resolver, root := parseAndBuildModel(t, "attribute test = "+tt.src+";")
			ctx := NewContext(NewModel(model, resolver), 1000)
			attrSym := resolveSymbol(t, root, "test")
			attrDecl := attrSym.Decl.(*ast.Usage)
			result, err := ctx.Eval(attrDecl.Value)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}
			if result.Kind != ValConst || result.Const.Kind != semantics.ValBool {
				t.Fatalf("expected bool, got %v", result)
			}
			if result.Const.Bool != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result.Const.Bool)
			}
		})
	}
}

func TestEval_EqualityNull(t *testing.T) {
	tests := []struct {
		src      string
		expected bool
	}{
		{"null == null", true},
		{"null != null", false},
		{"null == ()", true},
		{"() == null", true},
		{"() != null", false},
		{"null === ()", true},
		{"() !== null", false},
		{"() == 0", false},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			model, resolver, root := parseAndBuildModel(t, "attribute test = "+tt.src+";")
			ctx := NewContext(NewModel(model, resolver), 1000)
			attrSym := resolveSymbol(t, root, "test")
			attrDecl := attrSym.Decl.(*ast.Usage)
			result, err := ctx.Eval(attrDecl.Value)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}
			if result.Kind != ValConst || result.Const.Kind != semantics.ValBool {
				t.Fatalf("expected bool, got %v", result)
			}
			if result.Const.Bool != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result.Const.Bool)
			}
		})
	}
}

func TestEval_EqualityCrossKind(t *testing.T) {
	tests := []struct {
		src      string
		expected bool
	}{
		{"42 == null", false},
		{`"hello" == 42`, false},
		{`"hello" != 42`, true},
		{"null != 42", true},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			model, resolver, root := parseAndBuildModel(t, "attribute test = "+tt.src+";")
			ctx := NewContext(NewModel(model, resolver), 1000)
			attrSym := resolveSymbol(t, root, "test")
			attrDecl := attrSym.Decl.(*ast.Usage)
			result, err := ctx.Eval(attrDecl.Value)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}
			if result.Kind != ValConst || result.Const.Kind != semantics.ValBool {
				t.Fatalf("expected bool, got %v", result)
			}
			if result.Const.Bool != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result.Const.Bool)
			}
		})
	}
}

func TestEval_LogicalAnd(t *testing.T) {
	tests := []struct {
		src      string
		expected bool
	}{
		{"true & true", true},
		{"true & false", false},
		{"false & true", false},
		{"false & false", false},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			model, resolver, root := parseAndBuildModel(t, "attribute test = "+tt.src+";")
			ctx := NewContext(NewModel(model, resolver), 1000)
			attrSym := resolveSymbol(t, root, "test")
			attrDecl := attrSym.Decl.(*ast.Usage)
			result, err := ctx.Eval(attrDecl.Value)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}
			if result.Kind != ValConst || result.Const.Kind != semantics.ValBool {
				t.Fatalf("expected bool, got %v", result)
			}
			if result.Const.Bool != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result.Const.Bool)
			}
		})
	}
}

func TestEval_LogicalOr(t *testing.T) {
	tests := []struct {
		src      string
		expected bool
	}{
		{"true | true", true},
		{"true | false", true},
		{"false | true", true},
		{"false | false", false},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			model, resolver, root := parseAndBuildModel(t, "attribute test = "+tt.src+";")
			ctx := NewContext(NewModel(model, resolver), 1000)
			attrSym := resolveSymbol(t, root, "test")
			attrDecl := attrSym.Decl.(*ast.Usage)
			result, err := ctx.Eval(attrDecl.Value)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}
			if result.Kind != ValConst || result.Const.Kind != semantics.ValBool {
				t.Fatalf("expected bool, got %v", result)
			}
			if result.Const.Bool != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result.Const.Bool)
			}
		})
	}
}

func TestEval_LogicalNot(t *testing.T) {
	tests := []struct {
		src      string
		expected bool
	}{
		{"not true", false},
		{"not false", true},
		{"not not true", true},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			model, resolver, root := parseAndBuildModel(t, "attribute test = "+tt.src+";")
			ctx := NewContext(NewModel(model, resolver), 1000)
			attrSym := resolveSymbol(t, root, "test")
			attrDecl := attrSym.Decl.(*ast.Usage)
			result, err := ctx.Eval(attrDecl.Value)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}
			if result.Kind != ValConst || result.Const.Kind != semantics.ValBool {
				t.Fatalf("expected bool, got %v", result)
			}
			if result.Const.Bool != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result.Const.Bool)
			}
		})
	}
}

func TestEval_NegationArithmetic(t *testing.T) {
	tests := []struct {
		src      string
		expected int64
	}{
		{"-42", -42},
		{"-(-5)", 5},
		{"-(3 + 2)", -5},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			model, resolver, root := parseAndBuildModel(t, "attribute test = "+tt.src+";")
			ctx := NewContext(NewModel(model, resolver), 1000)
			attrSym := resolveSymbol(t, root, "test")
			attrDecl := attrSym.Decl.(*ast.Usage)
			result, err := ctx.Eval(attrDecl.Value)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}
			if result.Kind != ValConst || result.Const.Kind != semantics.ValInt {
				t.Fatalf("expected int, got %v", result)
			}
			if result.Const.Int != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result.Const.Int)
			}
		})
	}
}

func TestEval_NegationArithmeticReal(t *testing.T) {
	tests := []struct {
		src      string
		expected float64
	}{
		{"-3.14", -3.14},
		{"-(-2.5)", 2.5},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			model, resolver, root := parseAndBuildModel(t, "attribute test = "+tt.src+";")
			ctx := NewContext(NewModel(model, resolver), 1000)
			attrSym := resolveSymbol(t, root, "test")
			attrDecl := attrSym.Decl.(*ast.Usage)
			result, err := ctx.Eval(attrDecl.Value)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}
			if result.Kind != ValConst || result.Const.Kind != semantics.ValReal {
				t.Fatalf("expected real, got %v", result)
			}
			if result.Const.Real != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result.Const.Real)
			}
		})
	}
}

func TestEval_Track1Integration(t *testing.T) {
	// Test combining equality, logical, negation, and qualified names
	tests := []struct {
		src      string
		expected interface{} // bool or int64
	}{
		// Equality + logical operators
		{"attribute test = (42 == 42) & (100 != 99);", true},
		{"attribute test = (10 == 11) | (5 == 5);", true},
		{"attribute test = not (42 == 43);", true},

		// Qualified names + operators
		{"package A { attribute x = 42; } attribute test = A::x == 42;", true},
		{"package A { attribute x = 10; } attribute test = -(A::x);", int64(-10)},

		// Complex nested expression
		{"package A { attribute x = 5; attribute y = 10; } attribute test = (A::x < A::y) & (A::y == 10);", true},
	}

	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			model, resolver, root := parseAndBuildModel(t, tt.src)
			ctx := NewContext(NewModel(model, resolver), 1000)

			testSym := resolveSymbol(t, root, "test")
			testDecl := testSym.Decl.(*ast.Usage)

			result, err := ctx.Eval(testDecl.Value)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}

			switch exp := tt.expected.(type) {
			case bool:
				if result.Kind != ValConst || result.Const.Kind != semantics.ValBool || result.Const.Bool != exp {
					t.Errorf("expected bool %v, got %v", exp, result)
				}
			case int64:
				if result.Kind != ValConst || result.Const.Kind != semantics.ValInt || result.Const.Int != exp {
					t.Errorf("expected int %d, got %v", exp, result)
				}
			}
		})
	}
}

// enumSource declares the enumerations the literal tests evaluate against: one
// of plain literals, one whose literals carry attributes of their own.
const enumSource = `
package D {
	enum def Color { red; green; blue; }
	enum def Level {
		low { attribute n = 1; }
		high { attribute n = 9; }
	}
	enum def Grade { A = 4; B = 3; }
}
attribute test = %s;
`

// evalEnumExpr evaluates expr against the enumerations enumSource declares.
func evalEnumExpr(t *testing.T, expr string) (Value, error) {
	t.Helper()
	model, resolver, root := parseAndBuildModel(t, fmt.Sprintf(enumSource, expr))
	ctx := NewContext(NewModel(model, resolver), 1000)
	testSym := resolveSymbol(t, root, "test")
	return ctx.Eval(testSym.Decl.(*ast.Usage).Value)
}

// TestEval_EnumerationLiteralIsAValue pins that a literal evaluates to the
// identity of that literal, which is what renders and what compares.
func TestEval_EnumerationLiteralIsAValue(t *testing.T) {
	val, err := evalEnumExpr(t, "D::Color::red")
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if val.Kind != ValEnumLiteral {
		t.Fatalf("kind = %v, want ValEnumLiteral", val.Kind)
	}
	if val.Literal() == nil || val.Literal().Name != "red" {
		t.Fatalf("literal = %v, want red", val.Literal())
	}
	if got := val.LiteralText(); got != "Color::red" {
		t.Errorf("LiteralText() = %q, want %q", got, "Color::red")
	}
	if got := FormatTraceValue(val); got != "Color::red" {
		t.Errorf("FormatTraceValue() = %q, want %q", got, "Color::red")
	}
	if got := describeOperand(val); got != "the enumeration literal Color::red" {
		t.Errorf("describeOperand() = %q", got)
	}
}

// TestEval_EnumerationLiteralEquality pins that literal equality is identity:
// the same literal is equal to itself, and no other literal is equal to it,
// including one of another enumeration.
func TestEval_EnumerationLiteralEquality(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want bool
	}{
		{"D::Color::red == D::Color::red", true},
		{"D::Color::red != D::Color::red", false},
		{"D::Color::red == D::Color::green", false},
		{"D::Color::red != D::Color::green", true},
		{"D::Color::red == D::Level::low", false},
		{"D::Color::red != D::Level::low", true},
		{"D::Color::red === D::Color::red", true},
		{"D::Color::red === D::Color::blue", false},
		{"D::Color::red == 1", false},
		{"D::Color::red == \"red\"", false},
	} {
		t.Run(tt.expr, func(t *testing.T) {
			val, err := evalEnumExpr(t, tt.expr)
			if err != nil {
				t.Fatalf("Eval: %v", err)
			}
			if val.Kind != ValConst || val.Const.Kind != semantics.ValBool {
				t.Fatalf("value = %v, want a Boolean", val)
			}
			if val.Const.Bool != tt.want {
				t.Errorf("%s = %v, want %v", tt.expr, val.Const.Bool, tt.want)
			}
		})
	}
}

// TestEval_EnumerationLiteralOwnAttributes pins that a literal is an occurrence
// of its enumeration: the attributes it declares are readable through it, and
// each literal holds its own.
func TestEval_EnumerationLiteralOwnAttributes(t *testing.T) {
	for expr, want := range map[string]int64{
		"D::Level::low.n":  1,
		"D::Level::high.n": 9,
	} {
		val, err := evalEnumExpr(t, expr)
		if err != nil {
			t.Fatalf("Eval %s: %v", expr, err)
		}
		if val.Kind != ValConst || val.Const.Kind != semantics.ValInt || val.Const.Int != want {
			t.Errorf("%s = %v, want %d", expr, val, want)
		}
	}
}

// TestEval_EnumerationLiteralReadTwiceIsOneObject pins that the object a literal
// stands for is materialized once, so reading its attributes repeatedly reads
// the same occurrence rather than piling up objects.
func TestEval_EnumerationLiteralReadTwiceIsOneObject(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t,
		fmt.Sprintf(enumSource, "D::Level::high.n == D::Level::high.n"))
	ctx := NewContext(NewModel(model, resolver), 1000)
	testSym := resolveSymbol(t, root, "test")
	val, err := ctx.Eval(testSym.Decl.(*ast.Usage).Value)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if val.Kind != ValConst || !val.Const.Bool {
		t.Fatalf("value = %v, want true", val)
	}
	if len(ctx.instances) != 1 {
		t.Errorf("materialized %d objects, want 1", len(ctx.instances))
	}
}

// TestEval_EnumerationLiteralWithAValue pins that a literal of an enumeration
// giving its literals values evaluates to the value it declares.
func TestEval_EnumerationLiteralWithAValue(t *testing.T) {
	val, err := evalEnumExpr(t, "D::Grade::A")
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if val.Kind != ValConst || val.Const.Kind != semantics.ValInt || val.Const.Int != 4 {
		t.Fatalf("value = %v, want 4", val)
	}
}

// TestEval_EnumerationLiteralInASet pins that a set holds one element per
// literal: the same literal added twice is one member, two literals are two.
func TestEval_EnumerationLiteralInASet(t *testing.T) {
	red, err := evalEnumExpr(t, "D::Color::red")
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	green, err := evalEnumExpr(t, "D::Color::green")
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	set := NewSet()
	set.Add(red)
	set.Add(red)
	set.Add(green)
	if set.Size() != 2 {
		t.Errorf("set size = %d, want 2", set.Size())
	}
	if !set.Contains(red) || !set.Contains(green) {
		t.Errorf("set %v does not hold both literals", set.Elements())
	}
}
