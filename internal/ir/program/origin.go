package program

import "github.com/Open-MBEE/OpenSysML/internal/syntax/source"

// Origin is where a lowered record came from, as plain data: what a message, a
// trace or a hover says about it once the syntax tree is gone.
type Origin struct {
	// Doc is the name of the document the declaration was written in, "" outside any.
	Doc string
	// Span is the declaration's extent in Doc.
	Span source.Span
	// Path is unique within the graph: the declaration's qualified name, else its
	// owner's and its position among the owner's unnamed members of its kind (`A#if[2]`).
	Path string
	// Element is the declaration's element id when one is known, else "".
	Element string
	// Kind is the word today's messages name the declaration by when it is unnamed:
	// an action node's declaring type as %T renders it, a vertex's `state` or `pseudostate`.
	Kind string
	// Name is the declaration's name as messages use it, "" when it has none.
	Name string
	// Docs is the declaration's documentation prose, "" when it has none.
	Docs string
}

// String names the record the way a message does: by name when it has one,
// else by kind.
func (o Origin) String() string {
	if o.Name != "" {
		return "node " + o.Name
	}
	return "the " + o.Kind
}
