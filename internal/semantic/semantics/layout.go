package semantics

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Qualified names of the DiagramLayout metadata definitions in the bundled
// library.
const (
	LayoutFQN  = "DiagramLayout::Layout"
	RouteFQN   = "DiagramLayout::Route"
	CanvasFQN  = "DiagramLayout::Canvas"
	StyleFQN   = "DiagramLayout::Style"
	NoteFQN    = "DiagramLayout::Note"
	PictureFQN = "DiagramLayout::Picture"
)

// LayoutFQNs lists every DiagramLayout metadata definition.
var LayoutFQNs = []string{LayoutFQN, RouteFQN, CanvasFQN, StyleFQN, NoteFQN, PictureFQN}

// IsLayoutFQN reports whether fqn names a DiagramLayout metadata definition.
func IsLayoutFQN(fqn string) bool {
	switch fqn {
	case LayoutFQN, RouteFQN, CanvasFQN, StyleFQN, NoteFQN, PictureFQN:
		return true
	}
	return false
}

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

// Canvas is the drawing surface one Canvas annotation binds: its unit, and its
// extent when both width and height were given.
type Canvas struct {
	Unit          string
	Width, Height float64
	HasSize       bool
}

// Style is how one Style annotation says its element is drawn: colours as
// "#RRGGBB", each "" when unstated; Font "" and FontSize 0 likewise.
type Style struct {
	Fill, Line, Text string
	Font             string
	FontSize         float64
	Bold, Italic     bool
}

// Empty reports whether the style states nothing.
func (s Style) Empty() bool { return s == Style{} }

// Note is one Note annotation: a text box at X, Y, sized when HasSize, drawn
// anchored to the annotated element, or free when it annotates the view.
type Note struct {
	Text          string
	X, Y          float64
	Width, Height float64
	HasSize       bool
}

// Picture is one Picture annotation: a picture read from the file at Location,
// relative to the file the annotation is written in, filling the box at X, Y
// of Width by Height, with an alternative text and drawn over the elements it
// overlaps when Above, else under them.
type Picture struct {
	Location      string
	X, Y          float64
	Width, Height float64
	Alt           string
	Above         bool
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
	// TypeFQN is LayoutFQN, RouteFQN, CanvasFQN, StyleFQN, NoteFQN or PictureFQN.
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
	// Exactly one of Layout, Route, Canvas, Style, Note and Picture is set when
	// the annotation reads; all are nil when a binding it needs has a Problem.
	Layout  *Layout
	Route   *Route
	Canvas  *Canvas
	Style   *Style
	Note    *Note
	Picture *Picture
	// PointCount is the number of values a Route binds to points, whether or
	// not they read as waypoints.
	PointCount int
	// Bindings are the features the annotation's body binds, in declaration order.
	Bindings []MetadataBinding
	// Problems are the bindings that could not be read: a value that is not a
	// constant of the feature's type, a Route with an odd number of values.
	Problems []LayoutProblem
}

// Applies reports whether the site carries geometry a rendering can use.
func (s *LayoutSite) Applies() bool {
	return s != nil && (s.Layout != nil || s.Route != nil || s.Canvas != nil || s.Style != nil || s.Note != nil || s.Picture != nil)
}

// InView reports whether the site positions its element in view: an `about`
// annotation stated in the view's body, or one applying in every view.
func (s *LayoutSite) InView(view *symbols.Symbol) bool {
	return s.View == nil || sameElement(s.View, view)
}

// StatedInBodyOf reports whether the annotation is declared in the body of
// view or a namespace nested in it, inline or with an `about` clause.
func (s *LayoutSite) StatedInBodyOf(view *symbols.Symbol) bool {
	if s == nil || view == nil {
		return false
	}
	enclosing := enclosingView(s.Scope)
	return enclosing != nil && sameElement(enclosing, view)
}

// LayoutSitesOf returns the DiagramLayout annotations of sym in the order
// AnnotationSitesOf lists them, memoized: inline ones first, then `about`
// ones, each in declaration order.
func (m *Model) LayoutSitesOf(sym *symbols.Symbol) []*LayoutSite {
	if m == nil || sym == nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.layoutSites[sym]; ok {
		return cached
	}
	var out []*LayoutSite
	for _, a := range m.annotationsOf(sym) {
		if a.typ == nil || a.node == nil {
			continue
		}
		fqn := m.fqnOf(a.typ)
		if !IsLayoutFQN(fqn) {
			continue
		}
		site := &LayoutSite{TypeFQN: fqn, Node: a.node, Scope: a.scope, About: a.about}
		if a.about {
			site.View = enclosingView(a.scope)
		}
		bindings := metadataBindings(valueScope(a.scope, a.node), metadataBody(a.node))
		site.Bindings = bindings
		switch fqn {
		case LayoutFQN:
			m.readLayout(site, bindings)
		case RouteFQN:
			m.readRoute(site, bindings)
		case CanvasFQN:
			m.readCanvas(site, bindings)
		case StyleFQN:
			m.readStyle(site, bindings)
		case NoteFQN:
			m.readNote(site, bindings)
		case PictureFQN:
			m.readPicture(site, bindings)
		}
		out = append(out, site)
	}
	journal(m, m.layoutSites, sym, sym.Decl)
	m.layoutSites[sym] = out
	return out
}

// IsLayoutAnnotation reports whether sym is a metadata usage typed by one of
// the DiagramLayout definitions: a statement about a picture, not model content.
func (m *Model) IsLayoutAnnotation(sym *symbols.Symbol) bool {
	return IsLayoutFQN(m.annotationTypeFQN(sym))
}

// annotationTypeFQN is the qualified name of the metadata definition sym
// states as a metadata usage or prefix metadata, "" for any other symbol.
func (m *Model) annotationTypeFQN(sym *symbols.Symbol) string {
	if m == nil || sym == nil || m.resolver == nil {
		return ""
	}
	var a annotation
	var ok bool
	switch decl := sym.Decl.(type) {
	case *ast.Usage:
		if decl.Kind != ast.UsageMetadata {
			return ""
		}
		a, ok = m.usageAnnotation(sym.OwnerScope, decl)
	case *ast.PrefixMetadata:
		a, ok = m.prefixAnnotation(sym.OwnerScope, decl)
	}
	if !ok || a.typ == nil {
		return ""
	}
	return m.fqnOf(a.typ)
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

// StyleOf resolves the Style of elem as drawn in view, as LayoutOf resolves a
// Layout.
func (m *Model) StyleOf(view, elem *symbols.Symbol) (*LayoutSite, bool) {
	return m.resolveSite(view, elem, StyleFQN)
}

// NotesOf lists every Note of elem drawn in view, in declaration order: those
// stated in the view's body and those applying in every view. Unlike a Layout,
// every Note applies; an element may carry several. With a nil view only the
// latter apply. A view's own notes are free notes.
func (m *Model) NotesOf(view, elem *symbols.Symbol) []*LayoutSite {
	var out []*LayoutSite
	for _, site := range m.LayoutSitesOf(elem) {
		if site.TypeFQN == NoteFQN && site.InView(view) {
			out = append(out, site)
		}
	}
	return out
}

// PicturesOf lists every Picture of view, in declaration order: those stated
// in the view's body and those applying in every view. Every Picture is
// drawn; a view may carry several. A Picture annotating anything but a view
// draws nothing; the layout pass reports it.
func (m *Model) PicturesOf(view *symbols.Symbol) []*LayoutSite {
	if view == nil {
		return nil
	}
	var out []*LayoutSite
	for _, site := range m.LayoutSitesOf(view) {
		if site.TypeFQN == PictureFQN && site.InView(view) {
			out = append(out, site)
		}
	}
	return out
}

// CanvasOf resolves the Canvas of view: the first Canvas annotation stated in
// the view's body. One stated elsewhere sizes nothing; the layout pass
// reports it.
func (m *Model) CanvasOf(view *symbols.Symbol) (*LayoutSite, bool) {
	if view == nil {
		return nil, false
	}
	for _, site := range m.LayoutSitesOf(view) {
		if site.TypeFQN == CanvasFQN && site.StatedInBodyOf(view) {
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

// SymbolDeclaring returns the symbol registered for decl, so a node of a
// lowered graph is traced back to the element whose annotations position it.
// The subtree of scope is searched first, then every other document of the
// model: a lowered graph inherits content from definitions declared anywhere.
func (m *Model) SymbolDeclaring(scope *symbols.Scope, decl ast.Node) (*symbols.Symbol, bool) {
	if m == nil || decl == nil {
		return nil, false
	}
	if sym, ok := m.symbolDeclaringUnder(scope, decl); ok {
		return sym, true
	}
	if m.resolver == nil || m.resolver.Index() == nil {
		return nil, false
	}
	idx := m.resolver.Index()
	for _, name := range idx.Documents() {
		if root := idx.DocumentRoot(name); root != scope {
			if sym, ok := m.symbolDeclaringUnder(root, decl); ok {
				return sym, true
			}
		}
	}
	return nil, false
}

// symbolDeclaringUnder is the symbol scope's subtree registers for decl,
// memoized per scope.
func (m *Model) symbolDeclaringUnder(scope *symbols.Scope, decl ast.Node) (*symbols.Symbol, bool) {
	if scope == nil {
		return nil, false
	}
	defer m.ownScope(scope).LeaveDoc()
	index, ok := m.declSymbols[scope]
	if !ok {
		index = make(map[ast.Node]*symbols.Symbol)
		indexDeclarations(scope, index, make(map[*symbols.Scope]bool))
		journal(m, m.declSymbols, scope, scope.Node())
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

// readCanvas reads a Canvas body: an optional unit string and an optional
// width and height pair.
func (m *Model) readCanvas(site *LayoutSite, bindings []MetadataBinding) {
	canvas := &Canvas{}
	ok := true
	var hasWidth, hasHeight bool
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
			hasWidth = m.readReal(site, b, &canvas.Width)
			ok = hasWidth && ok
		case "height":
			hasHeight = m.readReal(site, b, &canvas.Height)
			ok = hasHeight && ok
		}
	}
	if bindsFeature(bindings, "width") != bindsFeature(bindings, "height") {
		site.Problems = append(site.Problems, LayoutProblem{Node: site.Node, Message: "Canvas binds one of width and height; an extent needs both"})
	}
	if ok {
		canvas.HasSize = hasWidth && hasHeight
		site.Canvas = canvas
	}
}

// readStyle reads a Style body: colours are "#RRGGBB" strings, font a
// string, fontSize a real, bold and italic booleans, every one optional.
func (m *Model) readStyle(site *LayoutSite, bindings []MetadataBinding) {
	style := &Style{}
	ok := true
	for _, b := range bindings {
		switch b.Feature {
		case "fill":
			ok = m.readColor(site, b, &style.Fill) && ok
		case "line":
			ok = m.readColor(site, b, &style.Line) && ok
		case "text":
			ok = m.readColor(site, b, &style.Text) && ok
		case "font":
			if b.Value != nil {
				ok = m.readString(site, b, &style.Font) && ok
			}
		case "fontSize":
			if b.Value != nil {
				ok = m.readReal(site, b, &style.FontSize) && ok
			}
		case "bold":
			ok = m.readBool(site, b, &style.Bold) && ok
		case "italic":
			ok = m.readBool(site, b, &style.Italic) && ok
		}
	}
	if style.FontSize < 0 {
		site.Problems = append(site.Problems, LayoutProblem{Node: site.Node, Message: "fontSize of Style is negative"})
		ok = false
	}
	if ok {
		site.Style = style
	}
}

// readNote reads a Note body: text is a required string, x and y required
// reals, width and height an optional pair.
func (m *Model) readNote(site *LayoutSite, bindings []MetadataBinding) {
	note := &Note{}
	var hasText, hasX, hasY, hasWidth, hasHeight bool
	for _, b := range bindings {
		switch b.Feature {
		case "text":
			hasText = m.readString(site, b, &note.Text)
		case "x":
			hasX = m.readReal(site, b, &note.X)
		case "y":
			hasY = m.readReal(site, b, &note.Y)
		case "width":
			hasWidth = m.readReal(site, b, &note.Width)
		case "height":
			hasHeight = m.readReal(site, b, &note.Height)
		}
	}
	if !bindsFeature(bindings, "text") {
		site.Problems = append(site.Problems, LayoutProblem{Node: site.Node, Message: "Note binds no text to show"})
	}
	if !bindsFeature(bindings, "x") || !bindsFeature(bindings, "y") {
		site.Problems = append(site.Problems, LayoutProblem{Node: site.Node, Message: "Note binds no x and y to place the note at"})
	}
	if hasWidth != hasHeight {
		site.Problems = append(site.Problems, LayoutProblem{Node: site.Node, Message: "Note binds one of width and height; a size needs both"})
	}
	if !hasText || !hasX || !hasY {
		return
	}
	note.HasSize = hasWidth && hasHeight
	site.Note = note
}

// readPicture reads a Picture body: location a required string, x, y, width
// and height required reals, alt an optional string, above an optional boolean.
func (m *Model) readPicture(site *LayoutSite, bindings []MetadataBinding) {
	pic := &Picture{}
	ok := true
	read := map[string]bool{}
	for _, b := range bindings {
		switch b.Feature {
		case "location":
			read[b.Feature] = m.readString(site, b, &pic.Location)
		case "x":
			read[b.Feature] = m.readReal(site, b, &pic.X)
		case "y":
			read[b.Feature] = m.readReal(site, b, &pic.Y)
		case "width":
			read[b.Feature] = m.readReal(site, b, &pic.Width)
		case "height":
			read[b.Feature] = m.readReal(site, b, &pic.Height)
		case "alt":
			if b.Value != nil {
				ok = m.readString(site, b, &pic.Alt) && ok
			}
		case "above":
			ok = m.readBool(site, b, &pic.Above) && ok
		}
	}
	if !bindsFeature(bindings, "location") {
		site.Problems = append(site.Problems, LayoutProblem{Node: site.Node, Message: "Picture binds no location to read the picture from"})
	}
	if !bindsFeature(bindings, "x") || !bindsFeature(bindings, "y") {
		site.Problems = append(site.Problems, LayoutProblem{Node: site.Node, Message: "Picture binds no x and y to place the picture at"})
	}
	if !bindsFeature(bindings, "width") || !bindsFeature(bindings, "height") {
		site.Problems = append(site.Problems, LayoutProblem{Node: site.Node, Message: "Picture binds no width and height to size the picture to"})
	}
	for _, feature := range []string{"location", "x", "y", "width", "height"} {
		ok = ok && read[feature]
	}
	if read["location"] && pic.Location == "" {
		site.Problems = append(site.Problems, LayoutProblem{Node: site.Node, Message: "location of Picture is empty"})
		ok = false
	}
	if (read["width"] && pic.Width <= 0) || (read["height"] && pic.Height <= 0) {
		site.Problems = append(site.Problems, LayoutProblem{Node: site.Node, Message: "width and height of Picture must be positive"})
		ok = false
	}
	if ok {
		site.Picture = pic
	}
}

// readString reads one binding as a string, reporting a value that is not a
// constant string; false then and for a binding with no value.
func (m *Model) readString(site *LayoutSite, b MetadataBinding, into *string) bool {
	if b.Value == nil {
		return false
	}
	v := m.annotationValue(b.Scope, b.Value)
	if v.Kind != symbols.FilterValueString {
		site.Problems = append(site.Problems, LayoutProblem{Node: b.Value,
			Message: fmt.Sprintf("%s of %s is not a constant string", b.Feature, simpleName(site.TypeFQN))})
		return false
	}
	*into = v.Str
	return true
}

// readColor reads one binding as a "#RRGGBB" colour; true for a binding with
// no value, which states no colour.
func (m *Model) readColor(site *LayoutSite, b MetadataBinding, into *string) bool {
	if b.Value == nil {
		return true
	}
	var s string
	if !m.readString(site, b, &s) {
		return false
	}
	if !IsHexColor(s) {
		site.Problems = append(site.Problems, LayoutProblem{Node: b.Value,
			Message: fmt.Sprintf("%s of %s is %q, not a colour written #RRGGBB", b.Feature, simpleName(site.TypeFQN), s)})
		return false
	}
	*into = strings.ToUpper(s)
	return true
}

// IsHexColor reports whether s is a colour written #RRGGBB.
func IsHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
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
func (m *Model) readBool(site *LayoutSite, b MetadataBinding, into *bool) bool {
	if b.Value == nil {
		return true
	}
	v := m.annotationValue(b.Scope, b.Value)
	if v.Kind != symbols.FilterValueBool {
		site.Problems = append(site.Problems, LayoutProblem{Node: b.Value,
			Message: fmt.Sprintf("%s of %s is not a constant boolean", b.Feature, simpleName(site.TypeFQN))})
		return false
	}
	*into = v.Bool
	return true
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
