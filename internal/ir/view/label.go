package view

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// labeller heads the nodes of one rendering the way a diagram names them: a
// node by its name under the namespace every root shares, or under the drawn
// owner that holds it, a type by its own name.
type labeller struct {
	context []string         // the shared namespace's names, outermost first
	owned   map[*Node]string // a node's name relative to the nearest drawn owner
}

// labelsOf finds the namespace the named roots share — the longest run of
// leading names common to their qualifiers — and, for every node whose
// qualified name continues that of another drawn node, its name below that owner.
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
	return labeller{context: context, owned: ownedNames(roots)}
}

// ownedNames names each node relative to the nearest drawn owner its qualified
// name continues: a drawn node, or the type of a drawn usage, whose members the
// usage's box holds. A root's name is its qualified name; a child named with one
// name is named under its parent, one named with a qualified name stands on its
// own, as a nested view's exposed elements do.
func ownedNames(roots []*Node) map[*Node]string {
	drawn := map[string]bool{}
	qualified := map[*Node][]string{} // the nodes named with a qualifier: the ones to shorten
	var walk func(nodes []*Node, root bool, parent []string)
	walk = func(nodes []*Node, root bool, parent []string) {
		for _, node := range nodes {
			var names []string
			if node.Name != "" {
				segments, ok := source.QualifiedNameSegments(node.Name)
				switch {
				case !ok:
				case len(segments) > 1:
					names, qualified[node] = segments, segments
				case root:
					names = segments
				case parent != nil:
					names = append(slices.Clone(parent), segments[0])
				}
			}
			if names != nil {
				drawn[source.QualifiedNameOf(names)] = true
			}
			for _, typ := range source.ReferenceQualifiedNames(node.Type) {
				drawn[source.QualifiedNameOf(typ)] = true
			}
			walk(node.Children, false, names)
		}
	}
	walk(roots, true, nil)
	owned := map[*Node]string{}
	for node, names := range qualified {
		for n := len(names) - 1; n > 0; n-- {
			if drawn[source.QualifiedNameOf(names[:n])] {
				owned[node] = source.QualifiedNameOf(names[n:])
				break
			}
		}
	}
	return owned
}

// name is a node's name below its nearest drawn owner, or else with the
// shared namespace left off the front.
func (l labeller) name(node *Node) string {
	if name, ok := l.owned[node]; ok {
		return name
	}
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
