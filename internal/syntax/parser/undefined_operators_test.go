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
	p := newParser("~x + ~y")
	if p.parseUnary(); len(p.undefinedOps) != 1 {
		t.Fatalf("after ~x: len = %d, want 1", len(p.undefinedOps))
	}
	cp := p.checkpoint()
	p.advance() // '+'
	if p.parseUnary(); len(p.undefinedOps) != 2 {
		t.Fatalf("after ~y: len = %d, want 2", len(p.undefinedOps))
	}
	p.restore(cp)
	p.release()
	if len(p.undefinedOps) != 1 || p.undefinedOps[0].Span().Offset != 0 {
		t.Fatalf("after restore: %d ops, want only the ~ at offset 0", len(p.undefinedOps))
	}
	p.advance() // '+'
	if p.parseUnary(); len(p.undefinedOps) != 2 || p.undefinedOps[1].Span().Offset != 5 {
		t.Fatalf("after reparse: %d ops, want the ~ at offset 5 second", len(p.undefinedOps))
	}
}
