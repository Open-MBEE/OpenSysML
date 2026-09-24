package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The parser records every `~` operator expression on the root in source
// order, wherever the expression sits.
func TestUndefinedOperatorsRecordsEveryTilde(t *testing.T) {
	src := "package p {\n" +
		"attribute a = ~1;\n" +
		"calc def C { return ~x; }\n" +
		"attribute b = f(~y, 2);\n" +
		"}"
	root := New(source.New("t.sysml", []byte(src))).ParseFile()
	ops := root.UndefinedOperators
	if len(ops) != 3 {
		t.Fatalf("UndefinedOperators len = %d, want 3", len(ops))
	}
	for i := 1; i < len(ops); i++ {
		if ops[i].Span().Offset <= ops[i-1].Span().Offset {
			t.Fatalf("UndefinedOperators not in source order at %d", i)
		}
	}
	for _, e := range ops {
		if e.Operator != ast.OpBitNot {
			t.Fatalf("recorded operator = %v, want OpBitNot", e.Operator)
		}
	}
}

// `~~x` records both the inner and the outer `~` expression.
func TestUndefinedOperatorsRecordsNestedTildes(t *testing.T) {
	src := "package p { attribute a = ~~x; }"
	root := New(source.New("t.sysml", []byte(src))).ParseFile()
	ops := root.UndefinedOperators
	if len(ops) != 2 {
		t.Fatalf("UndefinedOperators len = %d, want 2 for ~~x", len(ops))
	}
	outer := strings.Index(src, "~~")
	if got := []int{ops[0].Span().Offset, ops[1].Span().Offset}; got[0] != outer || got[1] != outer+1 {
		t.Fatalf("offsets = %v, want outer %d then inner %d", got, outer, outer+1)
	}
}

// A checkpoint restore drops the `~` sites the abandoned attempt recorded.
func TestUndefinedOperatorsFollowsRestore(t *testing.T) {
	// `attribute a = ~1` inside the first package parses; a second top-level
	// member cannot start mid-expression, so anything the abandoned try-parse
	// of a mistaken shape recorded must not leak into the root list.
	src := "package p { attribute a = ~1; } package q { attribute b = ~2; }"
	root := New(source.New("t.sysml", []byte(src))).ParseFile()
	if len(root.UndefinedOperators) != 2 {
		t.Fatalf("UndefinedOperators len = %d, want 2", len(root.UndefinedOperators))
	}
}
