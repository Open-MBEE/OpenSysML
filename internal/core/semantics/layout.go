package semantics

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Qualified names of the DiagramLayout metadata definitions in the bundled
// library.
const (
	LayoutFQN = "DiagramLayout::Layout"
	RouteFQN  = "DiagramLayout::Route"
	CanvasFQN = "DiagramLayout::Canvas"
)

// Layout is the geometry one Layout annotation binds: the top-left corner of
// the annotated element in pixels, y down, and its size when both were given.
type Layout struct {
	X, Y          float64
	Width, Height float64
	HasSize       bool
	Collapsed     bool
}

// Waypoint is one point of a Route, in the units of Layout.
type Waypoint struct {
	X, Y float64
}

// Route is the waypoints one Route annotation binds, in the order written.
type Route struct {
	Points []Waypoint
}

// Canvas is the drawing surface one Canvas annotation binds; a zero Width or
// Height is one the annotation left unbound.
type Canvas struct {
	Unit          string
	Width, Height float64
}

// LayoutProblem is a binding of a DiagramLayout annotation that could not be
// read as the geometry it stands for, located at the node stating it.
type LayoutProblem struct {
	Node    ast.Node
	Message string
}

// LayoutSite is one DiagramLayout annotation of an element: where it was
// stated, the geometry it binds, and the bindings that could not be read.
type LayoutSite struct {
	// TypeFQN is LayoutFQN, RouteFQN or CanvasFQN.
	TypeFQN string
	// Node states the annotation; Scope is where it is declared.
	Node  ast.Node
	Scope *symbols.Scope
	// About marks an annotation stated away from the element with an `about`
	// clause.
	About bool
	// View is the view whose body states an `about` annotation, so the geometry
	// applies in that view alone; nil for an inline annotation and for an `about`
	// one stated outside every view, which apply in every view.
	View *symbols.Symbol
	// Exactly one of Layout, Route and Canvas is set when the annotation reads
	// as geometry; all are nil when a binding it needs has a Problem.
	Layout *Layout
	Route  *Route
	Canvas *Canvas
	// PointCount is the number of values a Route binds to points, whether or
	// not they read as waypoints.
	PointCount int
	// Problems are the bindings that could not be read: a value that is not a
	// constant of the feature's type, a Route with an odd number of values.
	Problems []LayoutProblem
}

// Applies reports whether the site carries geometry a rendering can use.
func (s *LayoutSite) Applies() bool {
	return s != nil && (s.Layout != nil || s.Route != nil || s.Canvas != nil)
}

// InView reports whether the site positions its element in view: an `about`
// annotation stated in the view's body, or one applying in every view.
func (s *LayoutSite) InView(view *symbols.Symbol) bool {
	return s.View == nil || sameElement(s.View, view)
}

// LayoutSitesOf returns the DiagramLayout annotations of sym in the order
// AnnotationSitesOf lists them, memoized: inline ones first, then `about`
// ones, each in declaration order.
func (m *Model) LayoutSitesOf(sym *symbols.Symbol) []*LayoutSite {
	if m == nil || sym == nil {
		return nil
	}
	if cached, ok := m.layoutSites[sym]; ok {
		return cached
	}
	var out []*LayoutSite
	for _, a := range m.annotationsOf(sym) {
		if a.typ == nil || a.node == nil {
			continue
		}
		fqn := m.fqnOf(a.typ)
		switch fqn {
		case LayoutFQN, RouteFQN, CanvasFQN:
		default:
			continue
		}
		site := &LayoutSite{TypeFQN: fqn, Node: a.node, Scope: a.scope, About: a.about}
		if a.about {
			site.View = enclosingView(a.scope)
		}
		bindings := metadataBindings(valueScope(a.scope, a.node), metadataBody(a.node))
		switch fqn {
		case LayoutFQN:
			m.readLayout(site, bindings)
		case RouteFQN:
			m.readRoute(site, bindings)
		case CanvasFQN:
			m.readCanvas(site, bindings)
		}
		out = append(out, site)
	}
	m.layoutSites[sym] = out
	return out
}

// LayoutOf resolves the Layout of elem as drawn in view: the first Layout
// annotation stated in the view's body, else the first one applying in every
// view. With no view — a rendering of elements outside any view — only the
// latter apply. The site is returned whether or not it reads as geometry, so
// what it could not read is reported rather than dropped.
func (m *Model) LayoutOf(view, elem *symbols.Symbol) (*LayoutSite, bool) {
	return m.resolveSite(view, elem, LayoutFQN)
}

// RouteOf resolves the Route of the element an edge is declared as — a
// connector, flow, transition or succession — as drawn in view, as LayoutOf
// resolves a Layout.
func (m *Model) RouteOf(view, elem *symbols.Symbol) (*LayoutSite, bool) {
	return m.resolveSite(view, elem, RouteFQN)
}

// CanvasOf resolves the Canvas of view: the first Canvas annotation of the view
// itself, whether stated in its body or from elsewhere.
func (m *Model) CanvasOf(view *symbols.Symbol) (*LayoutSite, bool) {
	if view == nil {
		return nil, false
	}
	for _, site := range m.LayoutSitesOf(view) {
		if site.TypeFQN == CanvasFQN {
			return site, true
		}
	}
	return nil, false
}

// resolveSite picks the site of one type that positions elem in view: a
// view-local one first, then one applying in every view, each first-wins.
func (m *Model) resolveSite(view, elem *symbols.Symbol, typeFQN string) (*LayoutSite, bool) {
	sites := m.LayoutSitesOf(elem)
	if view != nil {
		for _, site := range sites {
			if site.TypeFQN == typeFQN && site.View != nil && sameElement(site.View, view) {
				return site, true
			}
		}
	}
	for _, site := range sites {
		if site.TypeFQN == typeFQN && site.View == nil {
			return site, true
		}
	}
	return nil, false
}

// SymbolDeclaring returns the symbol scope's subtree registers for decl, so an
// edge of a lowered graph is traced back to the element whose annotations
// position it. Memoized per scope.
func (m *Model) SymbolDeclaring(scope *symbols.Scope, decl ast.Node) (*symbols.Symbol, bool) {
	if m == nil || scope == nil || decl == nil {
		return nil, false
	}
	index, ok := m.declSymbols[scope]
	if !ok {
		index = make(map[ast.Node]*symbols.Symbol)
		indexDeclarations(scope, index, make(map[*symbols.Scope]bool))
		m.declSymbols[scope] = index
	}
	sym, ok := index[decl]
	return sym, ok
}

// indexDeclarations records the first symbol registered for each declaration
// under scope.
func indexDeclarations(scope *symbols.Scope, index map[ast.Node]*symbols.Symbol, seen map[*symbols.Scope]bool) {
	if scope == nil || seen[scope] {
		return
	}
	seen[scope] = true
	scope.ForEachMember(func(sym *symbols.Symbol) bool {
		if sym.Decl != nil {
			if _, dup := index[sym.Decl]; !dup {
				index[sym.Decl] = sym
			}
		}
		indexDeclarations(sym.Scope, index, seen)
		return true
	})
	for _, child := range scope.Children() {
		indexDeclarations(child, index, seen)
	}
}

// enclosingView is the nearest view whose body scope encloses, nil when no
// view does.
func enclosingView(scope *symbols.Scope) *symbols.Symbol {
	for sc := scope; sc != nil; sc = sc.Parent() {
		if owner := sc.Owner(); owner != nil && IsView(owner) {
			return owner
		}
	}
	return nil
}

// sameElement reports whether two symbols stand for one declaration, which a
// re-indexed twin of a symbol does as well as the symbol itself.
func sameElement(a, b *symbols.Symbol) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a == b || (a.Decl != nil && a.Decl == b.Decl)
}

// readLayout reads a Layout body: x and y are required reals, width and
// height an optional pair, collapsed an optional boolean.
func (m *Model) readLayout(site *LayoutSite, bindings []MetadataBinding) {
	layout := &Layout{}
	var hasX, hasY, hasWidth, hasHeight bool
	for _, b := range bindings {
		switch b.Feature {
		case "x":
			hasX = m.readReal(site, b, &layout.X)
		case "y":
			hasY = m.readReal(site, b, &layout.Y)
		case "width":
			hasWidth = m.readReal(site, b, &layout.Width)
		case "height":
			hasHeight = m.readReal(site, b, &layout.Height)
		case "collapsed":
			m.readBool(site, b, &layout.Collapsed)
		}
	}
	if !bindsFeature(bindings, "x") || !bindsFeature(bindings, "y") {
		site.Problems = append(site.Problems, LayoutProblem{Node: site.Node, Message: "Layout binds no x and y to place the element at"})
	}
	if !hasX || !hasY {
		return
	}
	layout.HasSize = hasWidth && hasHeight
	if hasWidth != hasHeight {
		site.Problems = append(site.Problems, LayoutProblem{Node: site.Node, Message: "Layout binds one of width and height; a size needs both"})
	}
	site.Layout = layout
}

// readRoute reads a Route body: points is a sequence of reals, x and y
// alternating, so an odd count leaves a point without its y.
func (m *Model) readRoute(site *LayoutSite, bindings []MetadataBinding) {
	var values []float64
	var elements []ast.Node
	ok := true
	for _, b := range bindings {
		if b.Feature != "points" || b.Value == nil {
			continue
		}
		for _, elem := range pointValues(b.Value) {
			elements = append(elements, elem)
			var v float64
			if !m.readRealValue(site, b.Scope, elem, "points", &v) {
				ok = false
			}
			values = append(values, v)
		}
	}
	site.PointCount = len(values)
	if len(values)%2 == 1 {
		site.Problems = append(site.Problems, LayoutProblem{Node: elements[len(elements)-1],
			Message: fmt.Sprintf("Route binds %d values to points; waypoints are x, y pairs, so the count must be even", len(values))})
		ok = false
	}
	if !ok {
		return
	}
	route := &Route{Points: make([]Waypoint, 0, len(values)/2)}
	for i := 0; i+1 < len(values); i += 2 {
		route.Points = append(route.Points, Waypoint{X: values[i], Y: values[i+1]})
	}
	site.Route = route
}

// readCanvas reads a Canvas body: an optional unit string and optional width
// and height.
func (m *Model) readCanvas(site *LayoutSite, bindings []MetadataBinding) {
	canvas := &Canvas{}
	ok := true
	for _, b := range bindings {
		switch b.Feature {
		case "unit":
			if b.Value == nil {
				continue
			}
			v := m.annotationValue(b.Scope, b.Value)
			if v.Kind != symbols.FilterValueString {
				site.Problems = append(site.Problems, LayoutProblem{Node: b.Value, Message: "unit of Canvas is not a constant string"})
				ok = false
				continue
			}
			canvas.Unit = v.Str
		case "width":
			ok = m.readReal(site, b, &canvas.Width) && ok
		case "height":
			ok = m.readReal(site, b, &canvas.Height) && ok
		}
	}
	if ok {
		site.Canvas = canvas
	}
}

// readReal reads one binding as a real, reporting a value that is not a
// constant number; ok is false then and for a binding with no value.
func (m *Model) readReal(site *LayoutSite, b MetadataBinding, into *float64) bool {
	if b.Value == nil {
		return false
	}
	return m.readRealValue(site, b.Scope, b.Value, b.Feature, into)
}

func (m *Model) readRealValue(site *LayoutSite, scope *symbols.Scope, value ast.Node, feature string, into *float64) bool {
	v := m.annotationValue(scope, value)
	switch v.Kind {
	case symbols.FilterValueReal:
		*into = v.Real
	case symbols.FilterValueInt:
		*into = float64(v.Int)
	default:
		site.Problems = append(site.Problems, LayoutProblem{Node: value,
			Message: fmt.Sprintf("%s of %s is not a constant number", feature, simpleName(site.TypeFQN))})
		return false
	}
	return true
}

// readBool reads one binding as a boolean, reporting a value that is not a
// constant truth value.
func (m *Model) readBool(site *LayoutSite, b MetadataBinding, into *bool) {
	if b.Value == nil {
		return
	}
	v := m.annotationValue(b.Scope, b.Value)
	if v.Kind != symbols.FilterValueBool {
		site.Problems = append(site.Problems, LayoutProblem{Node: b.Value,
			Message: fmt.Sprintf("%s of %s is not a constant boolean", b.Feature, simpleName(site.TypeFQN))})
		return
	}
	*into = v.Bool
}

// bindsFeature reports whether the body binds feature to a value at all.
func bindsFeature(bindings []MetadataBinding, feature string) bool {
	for _, b := range bindings {
		if b.Feature == feature && b.Value != nil {
			return true
		}
	}
	return false
}

// pointValues is the values a Route binds to points: the sequence's elements,
// nothing for the empty sequence `()`.
func pointValues(value ast.Node) []ast.Node {
	if _, ok := value.(*ast.NullExpr); ok {
		return nil
	}
	return sequenceElements(value)
}
