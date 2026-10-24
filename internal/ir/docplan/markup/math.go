package markup

import (
	gmast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// mathNode is inline mathematics: Segment covers the LaTeX between the dollars.
type mathNode struct {
	gmast.BaseInline
	Segment text.Segment
}

// kindMath is the node kind of mathNode.
var kindMath = gmast.NewNodeKind("Math")

// Kind implements ast.Node.
func (*mathNode) Kind() gmast.NodeKind { return kindMath }

// Dump implements ast.Node.
func (n *mathNode) Dump(source []byte, level int) {
	gmast.DumpHelper(n, source, level, map[string]string{"Source": string(n.Segment.Value(source))}, nil)
}

// mathParser reads `$…$` on one line as inline mathematics, the pandoc way:
// the opening dollar is followed by a non-space, the closing one is preceded
// by a non-space, is not escaped and is not followed by a digit, and a `$$`
// neither opens nor closes inline mathematics.
type mathParser struct{}

// Trigger implements parser.InlineParser.
func (mathParser) Trigger() []byte { return []byte{'$'} }

// Parse implements parser.InlineParser.
func (mathParser) Parse(_ gmast.Node, block text.Reader, _ parser.Context) gmast.Node {
	line, segment := block.PeekLine()
	if len(line) < 3 || line[1] == '$' || util.IsSpace(line[1]) || block.PrecendingCharacter() == '$' {
		return nil
	}
	for i := 2; i < len(line); i++ {
		if line[i] != '$' || line[i-1] == '\\' || line[i-1] == '$' || util.IsSpace(line[i-1]) {
			continue
		}
		if i+1 < len(line) && (line[i+1] == '$' || (line[i+1] >= '0' && line[i+1] <= '9')) {
			continue
		}
		node := &mathNode{Segment: text.NewSegment(segment.Start+1, segment.Start+i)}
		block.Advance(i + 1)
		return node
	}
	return nil
}
