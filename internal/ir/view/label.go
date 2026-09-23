package view

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// labeller heads the nodes of one rendering the way a diagram names them: a
// node by its name under the namespace every root shares, a type by its own name.
type labeller struct {
	context []string // the shared namespace's names, outermost first
}

// labelsOf finds the namespace the named roots share: the longest run of
// leading names common to their qualifiers.
func labelsOf(roots []*Node) labeller {
	var context []string
	found := false
	for _, root := range roots {
		if root.Name == "" {
			continue
		}
		names, ok := source.QualifiedNameSegments(root.Name)
		if !ok {
			continue
		}
		qualifier := names[:len(names)-1]
		if !found {
			context, found = qualifier, true
			continue
		}
		n := 0
		for n < len(context) && n < len(qualifier) && context[n] == qualifier[n] {
			n++
		}
		context = context[:n]
	}
	return labeller{context: context}
}

// name is a node's name with the shared namespace left off the front.
func (l labeller) name(node *Node) string {
	if len(l.context) == 0 {
		return node.Name
	}
	names, ok := source.QualifiedNameSegments(node.Name)
	if !ok || len(names) <= len(l.context) || !slices.Equal(names[:len(l.context)], l.context) {
		return node.Name
	}
	return source.QualifiedNameOf(names[len(l.context):])
}

// head is the first line of a node's diagram label: its name, followed by
// " : Type" for a typed usage, each type by the name it ends in. A node with no
// name leads with its kind instead.
func (l labeller) head(node *Node) string {
	if node.Name == "" {
		return node.Kind
	}
	if node.Type == "" {
		return l.name(node)
	}
	return l.name(node) + " : " + source.ReferenceEndNames(node.Type)
}

// lines is a node's diagram label in the graphical notation's order: the
// head, the kind in guillemets when the head is a name, then the notes.
func (l labeller) lines(node *Node) []string {
	lines := []string{l.head(node)}
	if node.Name != "" {
		lines = append(lines, "«"+node.Kind+"»")
	}
	if node.Detail != "" {
		lines = append(lines, node.Detail)
	}
	return lines
}
