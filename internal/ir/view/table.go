package view

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// tableColumns are the headings of a tabular rendering: what each row is, what
// the notation calls it, the type it is declared with, and what declares it.
var tableColumns = []string{"Element", "Kind", "Type", "Declared in"}

// TableColumns are the headings a tabular rendering carries when it names none.
func TableColumns() []string { return append([]string(nil), tableColumns...) }

// renderTable renders the exposed elements as rows of a table: one row per
// exposed element, one per element declared in it, and one per view nested in
// the rendered view with its own exposed elements beneath it.
func (r *Renderer) renderTable(view *symbols.Symbol, exposed []*symbols.Symbol, out *Rendering) {
	out.Columns = tableColumns
	for _, elem := range exposed {
		r.tableRows(elem, "", map[*symbols.Symbol]bool{}, 0, out)
	}
	r.nestedViewRows(view, map[*symbols.Symbol]bool{view: true}, out)
}

// tableRows writes an exposed element, by qualified name, and each element
// declared in it. owner is what declares the element, empty for one a view
// exposes.
func (r *Renderer) tableRows(sym *symbols.Symbol, owner string, seen map[*symbols.Symbol]bool, depth int, out *Rendering) {
	out.appendRow([]string{r.notationName(sym), declKind(sym), declType(sym), owner}, symbolOrigin(sym))
	r.memberRowsOf(sym, r.notationName(sym), seen, depth, out)
}

// memberRows writes an element declared in another, named as it is declared
// rather than by qualified name, and what it declares in turn.
func (r *Renderer) memberRows(sym *symbols.Symbol, owner string, seen map[*symbols.Symbol]bool, depth int, out *Rendering) {
	name := localName(sym)
	out.appendRow([]string{name, declKind(sym), declType(sym), owner}, symbolOrigin(sym))
	r.memberRowsOf(sym, name, seen, depth, out)
}

// memberRowsOf writes the elements declared in sym, stopping where the tree
// renderer stops: at a cycle, and at the depth bound.
func (r *Renderer) memberRowsOf(sym *symbols.Symbol, name string, seen map[*symbols.Symbol]bool, depth int, out *Rendering) {
	if seen[sym] || depth >= maxTreeDepth {
		return
	}
	seen[sym] = true
	for _, member := range r.containedMembers(sym) {
		r.memberRows(member, name, seen, depth+1, out)
	}
}

// nestedViewRows writes each view nested in view, and what it exposes, as rows
// of their own.
func (r *Renderer) nestedViewRows(view *symbols.Symbol, rendered map[*symbols.Symbol]bool, out *Rendering) {
	nested, err := r.model.NestedViews(view)
	if err != nil {
		return
	}
	for _, sub := range nested {
		if rendered[sub] {
			out.Notices = append(out.Notices, fmt.Sprintf("nested view %s is nested in itself; listed once", r.notationName(sub)))
			continue
		}
		rendered[sub] = true
		out.appendRow([]string{r.notationName(sub), declKind(sub), declType(sub), r.notationName(view)}, symbolOrigin(sub))
		exposed, err := r.model.ExposedElements(sub)
		if err == nil {
			for _, elem := range exposed {
				r.tableRows(elem, r.notationName(sub), map[*symbols.Symbol]bool{}, 0, out)
			}
		}
		r.nestedViewRows(sub, rendered, out)
	}
}
