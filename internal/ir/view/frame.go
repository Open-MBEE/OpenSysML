package view

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Frame is the view frame of SysML v2 §8.2.3.26 (framed-view): the name
// compartment at the frame's top-left corner, `«view» Name : ViewDef`, and the
// view-frame-info-contents of its three info compartments — the view's expose
// statements (top right), its element filters (bottom left) and the comments
// and documentation annotating the view (bottom right). Nil for a rendering
// of exposed elements with no view, which is drawn frameless.
type Frame struct {
	Name string
	// Type is the view definition the view is typed by, "" for an untyped view.
	Type string
	// Exposes are the view's expose statements as the notation writes them.
	Exposes []string
	// Filters are the view's element filter conditions as written.
	Filters []string
	// Annotations are the bodies of the comments and documentation annotating the view.
	Annotations []string
}

// NameLine is the name compartment's text: `«view» Name : Type`.
func (f *Frame) NameLine() string {
	line := "«view» " + f.Name
	if f.Type != "" {
		line += " : " + f.Type
	}
	return line
}

// Compartments are the info compartments drawn, each by its corner (`tr`,
// `bl`, `br`) with its lines; one with nothing to say is not drawn.
func (f *Frame) Compartments() []FrameCompartment {
	var out []FrameCompartment
	if len(f.Exposes) > 0 {
		out = append(out, FrameCompartment{Corner: FrameTopRight, Lines: f.Exposes})
	}
	if len(f.Filters) > 0 {
		out = append(out, FrameCompartment{Corner: FrameBottomLeft, Lines: f.Filters})
	}
	if len(f.Annotations) > 0 {
		out = append(out, FrameCompartment{Corner: FrameBottomRight, Lines: f.Annotations})
	}
	return out
}

// FrameCompartment is one info compartment of a view frame, at its corner.
type FrameCompartment struct {
	Corner string
	Lines  []string
}

// The corners the info compartments hang at, named as the productions are.
const (
	FrameTopRight    = "tr"
	FrameBottomLeft  = "bl"
	FrameBottomRight = "br"
)

// frameOf is the frame of view: its name and type, its expose statements, its
// filters and the annotations of its own body.
func (r *Renderer) frameOf(view *symbols.Symbol) *Frame {
	if view == nil {
		return nil
	}
	f := &Frame{Name: r.notationName(view), Type: r.headType(view)}
	if view.Scope != nil {
		for _, imp := range view.Scope.Imports() {
			if imp == nil || !imp.IsExpose || imp.Imported == nil {
				continue
			}
			text := "expose " + bareNames(ast.QualifiedNameText(imp.Imported)) + importSuffix(imp)
			if imp.FilterExpr != nil {
				if condition := r.nodeText(view.DocName, imp.FilterExpr); condition != "" {
					text += "[" + condition + "]"
				}
			}
			f.Exposes = append(f.Exposes, text)
		}
		for _, filter := range symbols.NamespaceFiltersIn(view.Scope) {
			if condition := r.nodeText(view.DocName, filter.Expr); condition != "" {
				f.Filters = append(f.Filters, "filter "+condition)
			}
		}
	}
	for _, member := range r.annotatingMembers(view) {
		if member.Kind == symbols.SymbolTextualRepresentation {
			continue
		}
		if node := r.annotationNode(&nodeIDs{}, nil, member, &Rendering{}); node != nil {
			f.Annotations = append(f.Annotations, strings.TrimSpace(annotationRowText(node)))
		}
	}
	return f
}
