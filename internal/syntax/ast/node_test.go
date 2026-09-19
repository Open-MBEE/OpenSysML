package ast

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestNodeBaseSpan(t *testing.T) {
	n := &ErrorNode{NodeBase: NodeBase{NodeSpan: source.Span{Offset: 3, Len: 5}}}
	if got := n.Span(); got.Offset != 3 || got.Len != 5 {
		t.Fatalf("Span() = %+v, want {3 5}", got)
	}
}

func TestErrorNodeMessage(t *testing.T) {
	n := &ErrorNode{Message: "unexpected token"}
	if n.Message != "unexpected token" {
		t.Fatalf("Message = %q", n.Message)
	}
}

func TestTriviaAttachment(t *testing.T) {
	n := &ErrorNode{}
	n.SetLeadingTrivia([]Trivia{{Kind: TriviaComment, Span: source.Span{Offset: 0, Len: 4}}})
	if len(n.LeadingTrivia()) != 1 || n.LeadingTrivia()[0].Kind != TriviaComment {
		t.Fatalf("leading trivia not attached: %+v", n.LeadingTrivia())
	}
}
