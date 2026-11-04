package view

import (
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Compartment is one compartment of a node's box: the rows of the members of
// one kind under the compartment's name, in declaration order.
type Compartment struct {
	// Name is the compartment's name, the plural of the members' keyword as
	// the graphical notation heads it: "attributes", "parts", "ports".
	Name string
	// Rows are the members the compartment lists, each the node it would
	// otherwise be drawn as.
	Rows []*Node
}

// compartmentOrder is the order the compartments are stacked in, that of the
// chapters of the language specification introducing each kind; a kind not
// named here follows them in the order its members were declared.
var compartmentOrder = []string{
	"attributes", "enumerations", "occurrences", "individuals", "items", "parts", "ports",
	"connections", "interfaces", "allocations", "actions", "states", "state actions", "calculations",
	"constraints", "requirements", "cases", "analyses", "verifications", "use cases",
	"views", "viewpoints", "renderings", "metadata",
}

// compartmentPlurals are the compartment names whose keyword does not take
// a plain "s".
// stateActionRoles are the kinds the `state actions` compartment gathers.
var stateActionRoles = map[string]bool{entryActionKind: true, doActionKind: true, exitActionKind: true}

var compartmentPlurals = map[string]string{
	"calc":           "calculations",
	"enum":           "enumerations",
	"analysis":       "analyses",
	"metadata":       "metadata",
	entryActionKind:  "state actions",
	doActionKind:     "state actions",
	exitActionKind:   "state actions",
	sendActionKind:   "actions",
	acceptActionKind: "actions",
	assignKind:       "actions",
	loopKind:         "actions",
	ifKind:           "actions",
	terminateKind:    "actions",
	"perform":        "perform actions",
	docKind:          docKind, // the `doc` compartment of §8.2.3.4 (documentation-compartment)
	repKind:          repKind,
}

// rowPrefixes are the modifier keywords a kind may begin with, which the row
// keeps before its name while the compartment is named by the kind after them.
var rowPrefixes = []string{"ref", "abstract", "variation", "individual", "readonly", "derived", "end", "event"}

// individualPrefix leads the kind of an individual usage, which the
// `individuals` compartment collects whatever its keyword (SysML v2.1 §8.2.3.9).
const individualPrefix = "individual "

// individualUsage reports the kind of an individual usage, not of an `individual def`.
func individualUsage(kind string) bool {
	return strings.HasPrefix(kind, individualPrefix) && !strings.HasSuffix(kind, " def")
}

// compartmentName is the name of the compartment a member of kind belongs to:
// the plural of its keyword after any modifier, "part defs" for a nested
// definition, "individuals" for an individual usage of any keyword.
func compartmentName(kind string) string {
	if individualUsage(kind) {
		return "individuals"
	}
	_, keyword := splitRowPrefix(kind)
	if keyword == "" {
		return ""
	}
	if plural, ok := compartmentPlurals[keyword]; ok {
		return plural
	}
	return keyword + "s"
}

// splitRowPrefix splits a kind into the modifiers a row writes before its
// name and the keyword that names its compartment.
func splitRowPrefix(kind string) (prefix, keyword string) {
	words := strings.Fields(kind)
	n := 0
	for n < len(words)-1 && isRowPrefix(words[n]) {
		n++
	}
	return strings.Join(words[:n], " "), strings.Join(words[n:], " ")
}

func isRowPrefix(word string) bool {
	for _, prefix := range rowPrefixes {
		if word == prefix {
			return true
		}
	}
	return false
}

// compartmentsOf groups rows into compartments by kind, the compartments in
// the notation's order and the rows of each in the order given. It is nil
// when there are no rows.
func compartmentsOf(rows []*Node) []Compartment {
	if len(rows) == 0 {
		return nil
	}
	index := map[string]int{}
	var out []Compartment
	for _, row := range rows {
		name := compartmentName(row.Kind)
		i, ok := index[name]
		if !ok {
			i = len(out)
			index[name] = i
			out = append(out, Compartment{Name: name})
		}
		out[i].Rows = append(out[i].Rows, row)
	}
	rank := func(name string) int {
		for i, known := range compartmentOrder {
			if known == name {
				return i
			}
		}
		return len(compartmentOrder)
	}
	slices.SortStableFunc(out, func(a, b Compartment) int { return rank(a.Name) - rank(b.Name) })
	return out
}

// rowText is a compartment row's text, the member's declaration as the
// textual notation writes it without the keyword its compartment names:
// `ref wheel : Wheel`, `gain : dB = 15.0`, `: Sensor` for an anonymous one.
func (l labeller) rowText(node *Node) string {
	if annotationKind(node.Kind) {
		return l.text(annotationRowText(node))
	}
	if node.Kind == statementRowKind {
		return node.Text
	}
	prefix, _ := splitRowPrefix(node.Kind)
	if _, ok := stateActionRoles[node.Kind]; ok {
		prefix = node.Kind // a `state actions` row keeps its entry/do/exit role
	}
	// The individuals compartment says `individual`; its row keeps the keyword instead.
	if individualUsage(node.Kind) {
		prefix = strings.TrimPrefix(node.Kind, individualPrefix)
	}
	text := shown(node)
	if text != "" {
		text = usagePrefix(node.Inherited) + text
	}
	if text == "" && node.Type == "" {
		text = node.Text
	}
	if node.Type != "" {
		text = strings.TrimSpace(text + " : " + source.ReferenceEndNames(node.Type))
	}
	if text == "" {
		text, prefix = node.Kind, ""
	}
	if node.Value != "" {
		text += " = " + node.Value
	}
	if prefix != "" {
		text = strings.TrimSpace(prefix + " " + text)
	}
	return l.text(text)
}

// compartments are the compartments a node's label shows: its own, when the
// rendering shows compartments.
func (l labeller) compartments(node *Node) []Compartment {
	if !l.shown.Has(ElementCompartments) {
		return nil
	}
	if !l.skin.cameo {
		return node.Compartments
	}
	// Cameo writes a state's behaviors as `entry / a` lines under its name.
	var kept []Compartment
	for _, compartment := range node.Compartments {
		if !stateActionCompartment(compartment.Name) {
			kept = append(kept, compartment)
		}
	}
	return kept
}

// referenceRow reports whether sym lists as a reference row, the `ref part`
// a tool's references compartment holds: a structural usage the model derives
// as non-composite, one that refers to its value rather than owning it. An
// attribute or enumeration usage, referential by definition, is a row of its
// own kind.
func (r *Renderer) referenceRow(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolAttributeUsage, symbols.SymbolEnumerationUsage:
		return false
	}
	return structuralUsage(sym) && !r.model.UsageIsComposite(sym)
}

// hiddenRow reports whether node's Layout hides the compartment row would
// list in: a hidden "references" hides every referential row, a hidden kind
// the compartment of that name.
func hiddenRow(node, row *Node) bool {
	if node.Geometry == nil || !node.Geometry.Hidden.Any() {
		return false
	}
	if row.Referential && !row.StandIn {
		return node.Geometry.Hidden.Hides("references")
	}
	return node.Geometry.Hidden.Hides(compartmentName(row.Kind))
}

// rowCount is how many rows a node's compartments hold.
func (l labeller) rowCount(node *Node) int {
	n := 0
	for _, compartment := range l.compartments(node) {
		n += len(compartment.Rows)
	}
	return n
}

// compartmentLines are a node's compartments as label lines: each one's name
// then its rows.
func (l labeller) compartmentLines(node *Node) []string {
	var lines []string
	for _, compartment := range l.compartments(node) {
		lines = append(lines, compartment.Name)
		for _, row := range compartment.Rows {
			lines = append(lines, strings.Split(l.rowText(row), "\n")...)
		}
	}
	return lines
}
