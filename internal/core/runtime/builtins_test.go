package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

func TestBuiltin_SequenceSize(t *testing.T) {
	seq := NewSequence()
	seq.Append(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}})
	seq.Append(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 2}})

	args := []Value{NewSequenceValue(seq)}

	fn := builtins["SequenceFunctions::size"]
	result, err := fn(nil, args)
	if err != nil {
		t.Fatalf("size failed: %v", err)
	}

	if result.Const.Int != 2 {
		t.Errorf("expected size 2, got %d", result.Const.Int)
	}
}

func TestSequenceFunctions_Includes(t *testing.T) {
	tests := []struct {
		src      string
		expected bool
	}{
		{`attribute result = SequenceFunctions::includes((1, 2, 3), 2);`, true},
		{`attribute result = SequenceFunctions::includes((1, 2, 3), 4);`, false},
		{`attribute result = SequenceFunctions::includes((), 1);`, false},
	}
	for _, tt := range tests {
		model, resolver, root := parseAndBuildLibraryModel(t, tt.src)
		ctx := NewContext(NewModel(model, resolver), 1000)
		sym := resolveSymbol(t, root, "result")
		decl := sym.Decl.(*ast.Usage)
		result, err := ctx.Eval(decl.Value)
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if result.Kind != ValConst || result.Const.Kind != semantics.ValBool {
			t.Fatalf("expected bool, got %+v", result)
		}
		if result.Const.Bool != tt.expected {
			t.Errorf("includes(%s) = %v, expected %v", tt.src, result.Const.Bool, tt.expected)
		}
	}
}

func TestBuiltin_ControlSelect(t *testing.T) {
	tests := []struct {
		src      string
		expected []int64
	}{
		{
			`attribute result = ControlFunctions::select((1, 2, 3, 4), { in x; x > 2 });`,
			[]int64{3, 4},
		},
		{
			`attribute result = ControlFunctions::select((1, 2, 3), { in x; x == 2 });`,
			[]int64{2},
		},
		{
			`attribute result = ControlFunctions::select((), { in x; x > 0 });`,
			[]int64{},
		},
		{
			`attribute result = ControlFunctions::select((10, 20, 30), { in x; x >= 20 });`,
			[]int64{20, 30},
		},
	}

	for _, tt := range tests {
		model, resolver, root := parseAndBuildLibraryModel(t, tt.src)
		ctx := NewContext(NewModel(model, resolver), 1000)
		sym := resolveSymbol(t, root, "result")
		decl := sym.Decl.(*ast.Usage)
		result, err := ctx.Eval(decl.Value)
		if err != nil {
			t.Fatalf("eval failed for %s: %v", tt.src, err)
		}

		if result.Kind != ValSequence {
			t.Fatalf("expected sequence, got %v", result.Kind)
		}

		elements := result.Sequence().Elements()
		if len(elements) != len(tt.expected) {
			t.Fatalf("expected %d elements, got %d: %+v", len(tt.expected), len(elements), elements)
		}

		for i, expectedInt := range tt.expected {
			elem := elements[i]
			if elem.Kind != ValConst || elem.Const.Kind != semantics.ValInt || elem.Const.Int != expectedInt {
				t.Errorf("element[%d] expected %d, got %+v", i, expectedInt, elem)
			}
		}
	}
}

func TestBuiltin_ControlCollect(t *testing.T) {
	tests := []struct {
		src      string
		expected []int64
	}{
		{
			`attribute result = ControlFunctions::collect((1, 2, 3), { in x; x * 2 });`,
			[]int64{2, 4, 6},
		},
		{
			`attribute result = ControlFunctions::collect((5, 10), { in x; x + 1 });`,
			[]int64{6, 11},
		},
		{
			`attribute result = ControlFunctions::collect((), { in x; x * 2 });`,
			[]int64{},
		},
	}

	for _, tt := range tests {
		model, resolver, root := parseAndBuildLibraryModel(t, tt.src)
		ctx := NewContext(NewModel(model, resolver), 1000)
		sym := resolveSymbol(t, root, "result")
		decl := sym.Decl.(*ast.Usage)
		result, err := ctx.Eval(decl.Value)
		if err != nil {
			t.Fatalf("eval failed for %s: %v", tt.src, err)
		}

		if result.Kind != ValSequence {
			t.Fatalf("expected sequence, got %v", result.Kind)
		}

		elements := result.Sequence().Elements()
		if len(elements) != len(tt.expected) {
			t.Fatalf("expected %d elements, got %d: %+v", len(tt.expected), len(elements), elements)
		}

		for i, expectedInt := range tt.expected {
			elem := elements[i]
			if elem.Kind != ValConst || elem.Const.Kind != semantics.ValInt || elem.Const.Int != expectedInt {
				t.Errorf("element[%d] expected %d, got %+v", i, expectedInt, elem)
			}
		}
	}
}

func TestBuiltin_ControlSelect_NonBooleanPredicate(t *testing.T) {
	src := `attribute result = ControlFunctions::select((1, 2, 3), { in x; x * 2 });`
	model, resolver, root := parseAndBuildLibraryModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 1000)
	sym := resolveSymbol(t, root, "result")
	decl := sym.Decl.(*ast.Usage)
	_, err := ctx.Eval(decl.Value)
	if err == nil || !strings.Contains(err.Error(), "predicate must return boolean") {
		t.Errorf("expected predicate type error, got: %v", err)
	}
}

func TestBuiltin_Track2Integration(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		expected bool
	}{
		{
			name: "includes + select",
			src: `attribute test = SequenceFunctions::includes(
				ControlFunctions::select((1, 2, 3, 4, 5), { in x; x > 2 }),
				4
			);`,
			expected: true,
		},
		{
			name: "collect + includes",
			src: `attribute test = SequenceFunctions::includes(
				ControlFunctions::collect((1, 2, 3), { in x; x * 2 }),
				6
			);`,
			expected: true,
		},
		{
			name: "select + collect + includes",
			src: `attribute test = SequenceFunctions::includes(
				ControlFunctions::collect(
					ControlFunctions::select((1, 2, 3, 4, 5), { in x; x > 2 }),
					{ in x; x * 10 }
				),
				30
			);`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, resolver, root := parseAndBuildLibraryModel(t, tt.src)
			ctx := NewContext(NewModel(model, resolver), 1000)

			testSym := resolveSymbol(t, root, "test")
			testDecl := testSym.Decl.(*ast.Usage)

			result, err := ctx.Eval(testDecl.Value)
			if err != nil {
				t.Fatalf("Eval() error: %v", err)
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
