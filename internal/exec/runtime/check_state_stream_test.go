package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestCanonicalStateTellsUnreceivedStreamsApart: two performances whose late streamed
// values differ only in pin, queue position, source or count spell differently.
func TestCanonicalStateTellsUnreceivedStreamsApart(t *testing.T) {
	node := func(offset int) ast.Node {
		return &ast.Usage{NodeBase: ast.NodeBase{NodeSpan: source.Span{Offset: offset, Len: 1}}, Kind: ast.UsageAction}
	}
	target, srcA, srcB := node(10), node(20), node(30)
	frameWith := func(streams ...unreceivedStream) *actionFrame {
		return &actionFrame{unreceived: map[ast.Node][]unreceivedStream{target: streams}}
	}
	mark := func(source ast.Node, pin string, at int) unreceivedStream {
		return unreceivedStream{flow: lower.ObjectFlow{SourcePin: pin, Target: target, TargetPin: pin}, source: source, pin: pin, at: at}
	}
	frames := []*actionFrame{
		frameWith(mark(srcA, "v", 0)),
		frameWith(mark(srcA, "w", 0)),
		frameWith(mark(srcA, "v", 1)),
		frameWith(mark(srcB, "v", 0)),
		frameWith(mark(srcA, "v", 0), mark(srcA, "v", 1)),
	}
	s := &stateSpeller{labels: make(map[*actionFrame]string)}
	seen := make(map[string]int)
	for i, perf := range frames {
		s.labels[perf] = "action"
		text := s.frame(perf)
		if j, dup := seen[text]; dup {
			t.Fatalf("frames %d and %d spell alike: %s", j, i, text)
		}
		seen[text] = i
	}
}
