package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// queryResultDefinition is the view definition a written query answer is typed by.
const queryResultDefinition = "QVTS::QueryResult"

// queryResultAttributes are what a QueryResult view states about its run, in
// the order the QVTS library declares them.
var queryResultAttributes = []string{"query", "source", "outcome", "provenance"}

// HeaderField is one statement a rendering makes about itself as a whole.
type HeaderField struct {
	Name  string
	Value string
}

// IsQueryResult reports whether view is typed by the QVTS library's QueryResult.
func (r *Renderer) IsQueryResult(view *symbols.Symbol) bool {
	for _, sym := range r.model.AllSupertypes(view) {
		if r.fqn(sym) == queryResultDefinition {
			return true
		}
	}
	return false
}

// renderQueryResult renders a query answer: one row per exposed element and
// the four attributes of its run as the header.
func (r *Renderer) renderQueryResult(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering) {
	out.Columns = tableColumns
	for _, elem := range exposed {
		out.appendRow([]string{r.notationName(elem), declKind(elem), r.headType(elem), r.notationName(elem.Owner())}, symbolOrigin(elem))
	}
	for _, name := range queryResultAttributes {
		value, _ := r.statedValue(view, name)
		out.Header = append(out.Header, HeaderField{Name: name, Value: value})
	}
	if len(exposed) == 0 {
		out.emptyReason = "the query result exposes no element"
	}
}

// statedValue is the value view binds its attribute name to, a string as its
// text and a reference by its last name; false when the view binds none.
func (r *Renderer) statedValue(view *symbols.Symbol, name string) (string, bool) {
	if view == nil || view.Scope == nil {
		return "", false
	}
	members := append(view.Scope.Members(), view.Scope.AnonymousMembers()...)
	sort.SliceStable(members, func(i, j int) bool { return members[i].DeclSpan.Offset < members[j].DeclSpan.Offset })
	for _, member := range members {
		if !member.Kind.IsAttributeLike() || r.model.EffectiveNameOf(member) != name {
			continue
		}
		usage, ok := member.Decl.(*ast.Usage)
		if !ok || usage.Value == nil {
			continue
		}
		if literal, ok := usage.Value.(*ast.LiteralString); ok {
			return source.StringValue(literal.Value), true
		}
		return symbols.LastSegment(referenceText(usage.Value)), true
	}
	return "", false
}

// writeHeaderText writes the header of a query-result rendering, one field a line.
func writeHeaderText(b *strings.Builder, header []HeaderField) {
	if len(header) == 0 {
		return
	}
	b.WriteString("\n")
	for _, field := range header {
		fmt.Fprintf(b, "%s: %s\n", field.Name, field.Value)
	}
}

// markdownText escapes what Markdown would read as markup in running text.
func markdownText(text string) string {
	return strings.NewReplacer("\\", "\\\\", "*", "\\*", "_", "\\_", "`", "\\`", "<", "\\<", "[", "\\[", "\n", " ").Replace(text)
}
