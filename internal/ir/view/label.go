package view

import (
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// labeller heads the nodes of one rendering the way a diagram names them: a
// node by its name under the namespace every root shares, or under the drawn
// owner that holds it, a type by its own name.
type labeller struct {
	context []string         // the shared namespace's names, outermost first
	owned   map[*Node]string // a node's name relative to the nearest drawn owner
	simple  map[*Node]string // a node's distinguishing suffix, when names are drawn simple
	skin    dotSkin          // the DOT skin labels are composed and measured for
}

// labelsOf finds the namespace the named roots share — the longest run of
// leading names common to their qualifiers — and, for every node whose
// qualified name continues that of another drawn node, its name below that owner.
// simple instead heads each node by the minimal suffix of its qualified name
// that distinguishes it among the nodes the drawing declares, as a positioned
// drawing of scattered elements names them; omitted lists the node IDs a
// drawing leaves undeclared (nil counts every node).
func labelsOf(roots []*Node, simple bool, omitted map[string]bool) labeller {
	if simple {
		return labeller{simple: simpleNames(roots, omitted)}
	}
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
			for _, typ := range drawnTypes(node) {
				drawn[typ] = true
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

// simpleNames is each node's minimal distinguishing suffix: its last name
// segment, extended back over the qualifier until no other node of the
// rendering ends in the same segments; a name that does not parse keeps its
// whole spelling, as a node outside the map reports.
func simpleNames(roots []*Node, omitted map[string]bool) map[*Node]string {
	names := map[*Node][]string{}
	groups := map[string][]*Node{}
	var walk func(nodes []*Node)
	walk = func(nodes []*Node) {
		for _, node := range nodes {
			if !omitted[node.ID] {
				if segments, ok := source.QualifiedNameSegments(node.Name); ok && len(segments) > 0 {
					names[node] = segments
					last := segments[len(segments)-1]
					groups[last] = append(groups[last], node)
				}
			}
			walk(node.Children)
		}
	}
	walk(roots)
	out := map[*Node]string{}
	for _, group := range groups {
		for _, node := range group {
			segments := names[node]
			n := 1
			for n < len(segments) && !distinctSuffix(names, group, node, segments, n) {
				n++
			}
			if n == len(segments) {
				out[node] = node.Name
			} else {
				out[node] = source.QualifiedNameOf(segments[len(segments)-n:])
			}
		}
	}
	return out
}

// distinctSuffix reports whether node's trailing n segments end the name of no
// other node in its last-segment group.
func distinctSuffix(names map[*Node][]string, group []*Node, node *Node, segments []string, n int) bool {
	tail := segments[len(segments)-n:]
	for _, other := range group {
		theirs := names[other]
		if other != node && len(theirs) >= n && slices.Equal(theirs[len(theirs)-n:], tail) {
			return false
		}
	}
	return true
}

// drawnTypes are the qualified names of the types a node's box draws the members
// of: the elements its typings resolved to, else the typings as written.
func drawnTypes(node *Node) []string {
	if len(node.Typings) > 0 {
		return node.Typings
	}
	var types []string
	for _, typ := range source.ReferenceQualifiedNames(node.Type) {
		types = append(types, source.QualifiedNameOf(typ))
	}
	return types
}

// name is a node's name below its nearest drawn owner, or else with the
// shared namespace left off the front.
func (l labeller) name(node *Node) string {
	if l.simple != nil {
		if name, ok := l.simple[node]; ok {
			return name
		}
		return node.Name
	}
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

// shown is the name a diagram shows for a node: its Name, unless a migration
// made that up, in which case the node is drawn as its source drew it, unnamed.
func shown(node *Node) string {
	if node.NameSynthesized {
		return ""
	}
	return node.Name
}

// head is the first line of a node's diagram label: its name, followed by
// " : Type" for a typed usage, each type by the name it ends in. A node with no
// shown name leads with " : Type" alone when typed, else with its kind.
func (l labeller) head(node *Node) string {
	switch name, typ := shown(node), node.Type; {
	case name == "" && typ == "":
		return node.Kind
	case name == "":
		return ": " + source.Unescape(source.ReferenceEndNames(typ))
	case typ == "":
		return source.Unescape(l.name(node))
	default:
		return source.Unescape(l.name(node) + " : " + source.ReferenceEndNames(typ))
	}
}

// headLines is the head split at the line breaks its escapes decode to, each
// line headed separately by every form's label writer.
func (l labeller) headLines(node *Node) []string {
	return strings.Split(l.head(node), "\n")
}

// keyworded reports whether a node's label has a keyword line, the kind in
// guillemets after the head: every node whose head is not the kind itself.
func keyworded(node *Node) bool {
	return shown(node) != "" || node.Type != ""
}

// lines is a node's diagram label in the graphical notation's order: the
// head, the keyword line when the node has one, then the notes. Every entry
// is a single line: a name that escapes a line break heads several entries.
func (l labeller) lines(node *Node) []string {
	lines := l.headLines(node)
	if keyworded(node) {
		lines = append(lines, "«"+node.Kind+"»")
	}
	if node.Detail != "" {
		if l.skin.cameo && node.Kind == "state" {
			return append(lines, cameoStateDetails(node.Detail)...)
		}
		lines = append(lines, node.Detail)
	}
	return lines
}

// cameoStateDetails splits a state's detail into Cameo's compartment lines, one
// per behaviour, and drops the `initial` marker the initial dot already draws.
func cameoStateDetails(detail string) []string {
	var lines []string
	for _, part := range strings.Split(detail, ", ") {
		switch {
		case part == "initial":
		case len(lines) > 0 && !stateDetailKeyword(part):
			lines[len(lines)-1] += ", " + part
		default:
			lines = append(lines, part)
		}
	}
	return lines
}

// stateDetailKeyword reports whether a detail part opens a behaviour line.
func stateDetailKeyword(part string) bool {
	word, _, _ := strings.Cut(part, " ")
	switch word {
	case "entry", "do", "exit", "defers":
		return true
	}
	return false
}
