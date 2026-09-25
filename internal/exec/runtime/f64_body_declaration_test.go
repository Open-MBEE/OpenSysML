package runtime

import "testing"

// F64: the parser keeps the features a body expression declares, and the
// evaluator resolves those declarations lazily when the result reads them.
func TestF64BodyDeclarationEvaluates(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want []int64
	}{
		{"xs->select { in i; private attribute k = i * 2; k > 2 }", []int64{2, 3}},
		{"xs->collect { in i; attribute k = i + 1; k }", []int64{2, 3, 4}},
		{"xs->reject { in i; private attribute k = i; k > 1 }", []int64{1}},
	} {
		t.Run(tt.expr, func(t *testing.T) {
			got, err := evalCollectionExpr(t, tt.expr)
			if err != nil {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if ints := intsOf(t, got); !equalInts(ints, tt.want) {
				t.Errorf("result = %v, want %v", ints, tt.want)
			}
		})
	}
}

// A body's opening documentation is kept in the AST and is not a declaration
// to execute, so a documented body evaluates as the same body undocumented.
func TestDocumentedBodyEvaluates(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want []int64
	}{
		{"xs->select { doc /* keep the large ones */ in i; i > 1 }", []int64{2, 3}},
		{"xs->collect { doc /* double */ in i; private attribute k = i * 2; k }", []int64{2, 4, 6}},
	} {
		t.Run(tt.expr, func(t *testing.T) {
			got, err := evalCollectionExpr(t, tt.expr)
			if err != nil {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if ints := intsOf(t, got); !equalInts(ints, tt.want) {
				t.Errorf("result = %v, want %v", ints, tt.want)
			}
		})
	}
}

// A body that declares nothing still evaluates.
func TestF64BodyWithoutDeclarationStillEvaluates(t *testing.T) {
	got, err := evalCollectionExpr(t, "xs->select { in i; i > 1 }")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Sequence() == nil || len(got.Sequence().Elements()) != 2 {
		t.Errorf("result = %v, want the two elements above 1", got)
	}
}
