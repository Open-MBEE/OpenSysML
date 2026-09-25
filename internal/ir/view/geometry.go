package view

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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

// Style is how a node or edge is drawn, from the DiagramLayout::Style annotating
// it: colours as "#RRGGBB" and a font face, size in points and weight. Each
// field is optional; the empty value leaves it to the drawing style.
type Style struct {
	Fill, Line, Text string
	Font             string
	FontSize         float64
	Bold, Italic     bool
}

// Note is a note box drawn on the canvas, from a DiagramLayout::Note: its text,
// the node it is anchored to (or the edge, by its end nodes; neither for one
// free on the surface), the top-left corner of its box and its size when HasSize.
type Note struct {
	Text          string
	Anchor        string
	EdgeFrom      string
	EdgeTo        string
	X, Y          float64
	Width, Height float64
	HasSize       bool
}

// Picture is a picture drawn on the canvas, from a DiagramLayout::Picture:
// the file it is read from as the view states it (relative to the view's
// file), the directory of that file ("" when the view is in no file), the box
// it fills, an alternative text, and whether it is drawn over the nodes it
// overlaps rather than under them.
type Picture struct {
	Location      string
	Dir           string
	X, Y          float64
	Width, Height float64
	Alt           string
	Above         bool
}

// Path is the picture's file as a path from the working directory: Location
// under Dir, or Location itself when it is absolute, a URL, or Dir is unknown.
func (p Picture) Path() string {
	if p.Dir == "" || filepath.IsAbs(p.Location) || strings.Contains(p.Location, "://") {
		return p.Location
	}
	return filepath.Join(p.Dir, filepath.FromSlash(p.Location))
}

// picturesOf adds to out the Pictures drawn on view, in the order the view
// states them; one that does not read is noticed.
func (r *Renderer) picturesOf(view *symbols.Symbol, out *Rendering) {
	dir := source.Dir(view.Origin().Doc)
	for _, site := range r.model.PicturesOf(view) {
		r.noteLayoutProblems(site, view, out)
		if site.Picture == nil {
			continue
		}
		p := site.Picture
		out.Pictures = append(out.Pictures, Picture{Location: p.Location, Dir: dir, X: p.X, Y: p.Y, Width: p.Width, Height: p.Height, Alt: p.Alt, Above: p.Above})
	}
}

// styleOf is the Style colouring elem in view (nil view: inline Style only),
// nil when none does; a Style that does not read as one is noticed.
func (r *Renderer) styleOf(view, elem *symbols.Symbol, out *Rendering) *Style {
	site, ok := r.model.StyleOf(view, elem)
	if !ok {
		return nil
	}
	r.noteLayoutProblems(site, elem, out)
	if site.Style == nil {
		return nil
	}
	s := site.Style
	return &Style{Fill: s.Fill, Line: s.Line, Text: s.Text, Font: s.Font, FontSize: s.FontSize, Bold: s.Bold, Italic: s.Italic}
}

// notesOf adds to out the Notes annotating elem in view, anchored to the node
// with ID anchor (empty: free on the canvas); one that does not read is noticed.
func (r *Renderer) notesOf(view, elem *symbols.Symbol, anchor string, out *Rendering) {
	for _, site := range r.model.NotesOf(view, elem) {
		r.noteLayoutProblems(site, elem, out)
		if site.Note == nil {
			continue
		}
		n := site.Note
		out.Notes = append(out.Notes, Note{Text: n.Text, Anchor: anchor, X: n.X, Y: n.Y, Width: n.Width, Height: n.Height, HasSize: n.HasSize})
	}
}

// edgeNotesOf adds to out the Notes annotating elem in view, anchored to the
// edge from one node to another.
func (r *Renderer) edgeNotesOf(view, elem *symbols.Symbol, from, to string, out *Rendering) {
	before := len(out.Notes)
	r.notesOf(view, elem, "", out)
	for i := before; i < len(out.Notes); i++ {
		out.Notes[i].EdgeFrom, out.Notes[i].EdgeTo = from, to
	}
}

// edgeDress is the Style of the edge elem from one node to another in view,
// adding the Notes anchored to it.
func (r *Renderer) edgeDress(view, elem *symbols.Symbol, from, to string, out *Rendering) *Style {
	r.edgeNotesOf(view, elem, from, to, out)
	return r.styleOf(view, elem, out)
}

// declaredEdgeDress is edgeDress for the edge lowered from decl, declared under elem.
func (r *Renderer) declaredEdgeDress(view, elem *symbols.Symbol, decl ast.Node, from, to string, out *Rendering) *Style {
	sym, ok := r.model.SymbolDeclaring(documentScope(elem), decl)
	if !ok {
		return nil
	}
	return r.edgeDress(view, sym, from, to, out)
}

// dress gives node the Style of elem in view and adds the Notes anchored to it.
func (r *Renderer) dress(view, elem *symbols.Symbol, node *Node, out *Rendering) *Node {
	node.Style = r.styleOf(view, elem, out)
	r.notesOf(view, elem, node.ID, out)
	return node
}

// declaredDress is dress for the node lowered from decl, declared under elem.
func (r *Renderer) declaredDress(view, elem *symbols.Symbol, decl ast.Node, node *Node, out *Rendering) *Node {
	sym, ok := r.model.SymbolDeclaring(documentScope(elem), decl)
	if !ok {
		return node
	}
	return r.dress(view, sym, node, out)
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

// declaredNameSynthesized reports whether a migration made up the name of the
// element decl declares under elem, as declaredRouteOf finds that element.
func (r *Renderer) declaredNameSynthesized(elem *symbols.Symbol, decl ast.Node) bool {
	sym, ok := r.model.SymbolDeclaring(documentScope(elem), decl)
	return ok && r.model.NameSynthesized(sym)
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
