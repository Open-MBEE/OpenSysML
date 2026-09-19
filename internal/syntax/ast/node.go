package ast

import "github.com/Open-MBEE/OpenSysML/internal/syntax/source"

// Node is the common interface for every AST/CST node. The tree is
// syntax-only and immutable after parsing; all semantic information
// (resolved references, types) lives in side tables keyed by node,
// added in later plans.
type Node interface {
	Span() source.Span
	LeadingTrivia() []Trivia
	TrailingTrivia() []Trivia
}

// TriviaKind classifies a trivia token attached to a node.
type TriviaKind int

const (
	TriviaComment   TriviaKind = iota // REGULAR_COMMENT that is not a comment/doc/rep body
	TriviaLineNote                    // SL_NOTE
	TriviaBlockNote                   // ML_NOTE
)

// Trivia is a non-semantic token (notes/free comments) attached to a node for
// features like doc-comment hover and future formatting. Whitespace carries
// nothing a consumer reads and is not recorded.
type Trivia struct {
	Kind TriviaKind
	Span source.Span
}

// NodeBase is embedded by every concrete node to provide span + trivia
// storage and satisfy the Node interface's accessor methods.
type NodeBase struct {
	NodeSpan source.Span
	leading  []Trivia
	trailing []Trivia
}

func (b *NodeBase) Span() source.Span            { return b.NodeSpan }
func (b *NodeBase) LeadingTrivia() []Trivia      { return b.leading }
func (b *NodeBase) TrailingTrivia() []Trivia     { return b.trailing }
func (b *NodeBase) SetLeadingTrivia(t []Trivia)  { b.leading = t }
func (b *NodeBase) SetTrailingTrivia(t []Trivia) { b.trailing = t }

// ErrorNode represents a span of source the parser could not parse into a
// valid construct. The parser always produces a tree; unparseable regions
// become ErrorNodes so downstream tooling (LSP) still gets a partial tree.
type ErrorNode struct {
	NodeBase
	Message string
}
