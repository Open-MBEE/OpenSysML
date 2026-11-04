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
	"attributes", "enums", "occurrences", "items", "parts", "ports", "ends",
	"connections", "interfaces", "flows", "allocations", "actions", "states", "calcs",
	"constraints", "requirements", "satisfies", "cases", "analyses", "verifications", "verifies",
	"use cases", "include use cases", "subject", "objective", "views", "viewpoints", "rendering", "metadata",
}

// compartmentHeadings are the compartment names the figures of SysML v2 §8.2.3
// head otherwise than by the plain plural of the members' keyword.
var compartmentHeadings = map[string]string{
	"calc":     "calcs",
	"enum":     "enums",
	"analysis": "analyses",
	"metadata": "metadata",
	// A metadata usage with no keyword of its own is named by its classification.
	"metadata usage": "metadata",
	"render":         "rendering",
	"rendering":      "rendering",
	"subject":        "subject",
	"objective":      "objective",
	"satisfy":        "satisfies",
	"verify":         "verifies",
}

// rowPrefixes are the modifier keywords a kind may begin with, which the row
// keeps before its name while the compartment is named by the kind after them.
var rowPrefixes = []string{"ref", "abstract", "variation", "individual", "readonly", "derived", "end"}

// compartmentName is the compartment a member of kind lists in, as the figures head
// it: the figure's heading or the keyword's plural; `end` features list under "ends".
func compartmentName(kind string) string {
	prefix, keyword := splitRowPrefix(kind)
	if keyword == "" {
		return ""
	}
	if slices.Contains(strings.Fields(prefix), "end") {
		return "ends"
	}
	if heading, ok := compartmentHeadings[keyword]; ok {
		return heading
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
	prefix, _ := splitRowPrefix(node.Kind)
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
	return node.Compartments
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

// ellipsisMark is the `…` the notation sets at the lower left of a node whose
// compartments leave rows out (§8.2.3.2 ellipsis-at-lower-left-corner).
const ellipsisMark = "…"

// labelLine is one line of a node's compartments: its text, and whether the
// notation greys it as the element inside a textual compartment an edge ends at.
type labelLine struct {
	Text   string
	Greyed bool
}

// compartmentEntries are a node's compartments as label lines: each one's name
// then its rows, and the ellipsis when the node's Layout hid rows.
func (l labeller) compartmentEntries(node *Node) []labelLine {
	var lines []labelLine
	for _, compartment := range l.compartments(node) {
		lines = append(lines, labelLine{Text: compartment.Name})
		for _, row := range compartment.Rows {
			lines = append(lines, labelLine{Text: l.rowText(row), Greyed: row.Greyed})
		}
	}
	if l.elided(node) {
		lines = append(lines, labelLine{Text: ellipsisMark})
	}
	return lines
}

// elided reports whether the node's box ends with the ellipsis: its Layout hid
// rows, and the rendering shows compartments at all.
func (l labeller) elided(node *Node) bool {
	return node.Elided && l.shown.Has(ElementCompartments)
}

// compartmentLines are compartmentEntries as plain text.
func (l labeller) compartmentLines(node *Node) []string {
	var lines []string
	for _, entry := range l.compartmentEntries(node) {
		lines = append(lines, entry.Text)
	}
	return lines
}
