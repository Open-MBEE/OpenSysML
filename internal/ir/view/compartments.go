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
	"attributes", "enums", "occurrences", "individuals", "items", "parts", "ports", "ends",
	"connections", "interfaces", "flows", "allocations", "actions", "states", "state actions", "calcs",
	"constraints", "requirements", "satisfies", "cases", "analyses", "verifications", "verifies",
	"use cases", "include use cases", "subject", "objective", "views", "viewpoints", "rendering", "metadata",
}

// stateActionRoles are the kinds the `state actions` compartment gathers.
var stateActionRoles = map[string]bool{entryActionKind: true, doActionKind: true, exitActionKind: true}

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
	// The figures' two-word keywords (view.go figureKeyword).
	"satisfy requirement": "satisfies",
	"verify requirement":  "verifies",
	"frame concern":       "frames",
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

// compartmentName is the compartment a member of kind lists in, as the figures head it: the
// heading or the keyword's plural; `end` features list under "ends", individual usages under "individuals".
func compartmentName(kind string) string {
	if individualUsage(kind) {
		return "individuals"
	}
	prefix, keyword := splitRowPrefix(kind)
	if keyword == "" {
		return ""
	}
	if slices.Contains(kindWords(prefix), "end") {
		return "ends"
	}
	return keywordCompartment(keyword)
}

// keywordCompartment is the compartment a keyword's own members list in: the
// figure's heading, or the keyword's plural.
func keywordCompartment(keyword string) string {
	if heading, ok := compartmentHeadings[keyword]; ok {
		return heading
	}
	return keyword + "s"
}

// rowCompartments names the compartment a row of kind lists in and, when a
// modifier moves it there (`ends`, `individuals`), its keyword's own compartment,
// whose hiding hides the row too: `end attribute` goes with the attributes.
func rowCompartments(kind string) []string {
	listed := compartmentName(kind)
	_, keyword := splitRowPrefix(kind)
	if own := keywordCompartment(keyword); keyword != "" && own != listed {
		return []string{listed, own}
	}
	return []string{listed}
}

// splitRowPrefix splits a kind into the modifiers a row writes before its
// name and the keyword that names its compartment.
func splitRowPrefix(kind string) (prefix, keyword string) {
	words := kindWords(kind)
	n := 0
	for n < len(words)-1 && isRowPrefix(words[n]) {
		n++
	}
	return strings.Join(words[:n], " "), strings.Join(words[n:], " ")
}

// kindWords splits a kind at its spaces, a quoted name (`#'Safety Level'`) kept
// whole, its backslash escapes (`'Owner\'s Level'`) read as the notation does.
func kindWords(kind string) []string {
	var words []string
	start, quoted, escaped := -1, false, false
	for i, r := range kind {
		switch {
		case escaped:
			escaped = false
		case quoted && r == '\\':
			escaped = true
		case r == '\'':
			quoted = !quoted
			if start < 0 {
				start = i
			}
		case r == ' ' && !quoted:
			if start >= 0 {
				words = append(words, kind[start:i])
				start = -1
			}
		case start < 0:
			start = i
		}
	}
	if start >= 0 {
		words = append(words, kind[start:])
	}
	return words
}

func isRowPrefix(word string) bool {
	if strings.HasPrefix(word, "#") {
		return true
	}
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
	for _, compartment := range rowCompartments(row.Kind) {
		if node.Geometry.Hidden.Hides(compartment) {
			return true
		}
	}
	return false
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
			for _, text := range strings.Split(l.rowText(row), "\n") {
				lines = append(lines, labelLine{Text: text, Greyed: row.Greyed})
			}
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
