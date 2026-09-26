package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestTransitionClausesRejectRepeatedGuard(t *testing.T) {
	src := "state def S { state a; state b; transition first a if true if false then b; }"
	p := New(source.New("transition.sysml", []byte(src)))
	p.ParseFile()

	if len(p.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %v, want one repeated-guard diagnostic", p.Diagnostics)
	}
	diagnostic := p.Diagnostics[0]
	if diagnostic.Message != "a transition has at most one 'if' clause" {
		t.Fatalf("diagnostic = %q, want repeated-guard message", diagnostic.Message)
	}
	secondIf := strings.LastIndex(src, "if")
	if diagnostic.Span.Offset != secondIf || diagnostic.Span.Len != len("if") {
		t.Fatalf("diagnostic span = %+v, want second `if` at %d", diagnostic.Span, secondIf)
	}
}

func TestTransitionClausesAcceptSingleGuard(t *testing.T) {
	src := "state def S { state a; state b; transition first a if true then b; }"
	p := New(source.New("transition.sysml", []byte(src)))
	p.ParseFile()

	if len(p.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %v, want none", p.Diagnostics)
	}
}

func TestTransitionClausesReportOutOfOrderClause(t *testing.T) {
	src := "state def S { state a; state b; transition first a do { } if true then b; }"
	p := New(source.New("transition.sysml", []byte(src)))
	p.ParseFile()

	if len(p.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %v, want one out-of-order diagnostic", p.Diagnostics)
	}
	diagnostic := p.Diagnostics[0]
	if diagnostic.Message != "transition clauses must appear in the order 'accept', 'if', 'do'" {
		t.Fatalf("diagnostic = %q, want clause-order message", diagnostic.Message)
	}
	if got := src[diagnostic.Span.Offset:diagnostic.Span.End()]; got != "if" {
		t.Fatalf("diagnostic sits on %q, want the out-of-order `if` keyword", got)
	}
}
