package view

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Point is one position on a rendering's canvas, in pixels from the top-left
// corner, y increasing downward.
type Point struct {
	X, Y float64
}

// Geometry is where a node is drawn, from the DiagramLayout::Layout positioning
// it in the rendered view; HasSize says whether Width and Height were given.
type Geometry struct {
	X, Y          float64
	Width, Height float64
	HasSize       bool
	Collapsed     bool
}

// Canvas is the drawing surface a view states with DiagramLayout::Canvas: its
// unit, and its extent when HasSize.
type Canvas struct {
	Unit          string
	Width, Height float64
	HasSize       bool
}

// geometryOf is the Geometry positioning elem in view (nil view: inline Layout
// only), nil when none does; a Layout that does not read as geometry is noticed.
func (r *Renderer) geometryOf(view, elem *symbols.Symbol, out *Rendering) *Geometry {
	out.drawn.note(elem, false)
	site, ok := r.model.LayoutOf(view, elem)
	if !ok {
		return nil
	}
	r.noteLayoutProblems(site, elem, out)
	if site.Layout == nil {
		return nil
	}
	l := site.Layout
	return &Geometry{X: l.X, Y: l.Y, Width: l.Width, Height: l.Height, HasSize: l.HasSize, Collapsed: l.Collapsed}
}

// routeOf is the waypoints the edge declared as elem follows in view, nil when
// no Route annotation gives any, resolved as geometryOf resolves a Layout.
func (r *Renderer) routeOf(view, elem *symbols.Symbol, out *Rendering) []Point {
	out.drawn.note(elem, true)
	site, ok := r.model.RouteOf(view, elem)
	if !ok {
		return nil
	}
	r.noteLayoutProblems(site, elem, out)
	if site.Route == nil {
		return nil
	}
	route := make([]Point, 0, len(site.Route.Points))
	for _, p := range site.Route.Points {
		route = append(route, Point{X: p.X, Y: p.Y})
	}
	return route
}

// declaredRouteOf is the route of the edge lowered from decl, a transition,
// succession or flow declared under elem; nil when decl declares no element.
func (r *Renderer) declaredRouteOf(view, elem *symbols.Symbol, decl ast.Node, out *Rendering) []Point {
	edge, ok := r.model.SymbolDeclaring(documentScope(elem), decl)
	if !ok {
		return nil
	}
	return r.routeOf(view, edge, out)
}

// declaredGeometryOf is the Geometry of the node lowered from decl, a state,
// region or action node declared under elem; nil when decl declares no element.
func (r *Renderer) declaredGeometryOf(view, elem *symbols.Symbol, decl ast.Node, out *Rendering) *Geometry {
	node, ok := r.model.SymbolDeclaring(documentScope(elem), decl)
	if !ok {
		return nil
	}
	return r.geometryOf(view, node, out)
}

// canvasOf is the Canvas view states, nil for a view stating none and for a
// rendering outside any view.
func (r *Renderer) canvasOf(view *symbols.Symbol, out *Rendering) *Canvas {
	site, ok := r.model.CanvasOf(view)
	if !ok {
		return nil
	}
	r.noteLayoutProblems(site, view, out)
	if site.Canvas == nil {
		return nil
	}
	return &Canvas{Unit: site.Canvas.Unit, Width: site.Canvas.Width, Height: site.Canvas.Height, HasSize: site.Canvas.HasSize}
}

// noteLayoutProblems reports, once, the bindings of a DiagramLayout annotation
// that could not be read as geometry, so the annotation is not silently dropped.
func (r *Renderer) noteLayoutProblems(site *semantics.LayoutSite, elem *symbols.Symbol, out *Rendering) {
	for _, problem := range site.Problems {
		notice := fmt.Sprintf("%s annotation of %s %s is not applied: %s",
			simpleName(site.TypeFQN), declKind(elem), r.notationName(elem), problem.Message)
		if !slices.Contains(out.Notices, notice) {
			out.Notices = append(out.Notices, notice)
		}
	}
}

// formatCoord writes a coordinate in its shortest exact form: 120, not 120.0.
func formatCoord(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// documentScope is the outermost scope enclosing sym's declaration, under which
// every declaration of its document registers a symbol.
func documentScope(sym *symbols.Symbol) *symbols.Scope {
	scope := declScope(sym)
	for scope != nil && scope.Parent() != nil {
		scope = scope.Parent()
	}
	return scope
}
