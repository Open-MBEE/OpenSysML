package passes

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// DiagramLayoutPass validates the DiagramLayout annotations of a document: a
// Layout or Route the rendering it applies to cannot draw, a binding that does
// not read as geometry, a Canvas stated outside the body of the view it
// annotates, and two `about` annotations of one kind for one element in one
// view, of which the first applies.
type DiagramLayoutPass struct{}

// Diagnostic codes of the pass.
const (
	layoutUnplacedCode  = "diagram-layout-unplaced"
	layoutValueCode     = "diagram-layout-value"
	layoutCanvasCode    = "diagram-layout-canvas"
	layoutDuplicateCode = "diagram-layout-duplicate"
)

func (DiagramLayoutPass) Level() PassLevel { return LevelConstraint }

func (DiagramLayoutPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &layoutChecker{
		model:    ctx.Model(),
		renderer: view.NewRenderer(ctx.Model(), ctx.Resolver(), nil),
		docRoot:  rootScope,
		fqn:      func(sym *symbols.Symbol) string { return ctx.Index.GetFQN(sym) },
		drawn:    map[ast.Node]*view.Drawn{},
	}
	// An `about` annotation stated in this document may annotate an element of
	// another, so the annotated elements of the whole workspace are visited too.
	seen := map[*symbols.Symbol]bool{}
	w8dWalkSymbols(ctx, rootScope, func(sym *symbols.Symbol) {
		seen[sym] = true
		c.check(sym)
	})
	for _, sym := range c.model.AboutAnnotatedSymbols() {
		if !seen[sym] {
			seen[sym] = true
			c.check(sym)
		}
	}
	return c.diags
}

type layoutChecker struct {
	model    *semantics.Model
	renderer *view.Renderer
	docRoot  *symbols.Scope
	fqn      func(*symbols.Symbol) string
	diags    []Diagnostic
	// drawn is what each view's rendering draws, by the view's declaration.
	drawn map[ast.Node]*view.Drawn
}

// inDoc reports whether a scope lies in the document under validation, so each
// document reports the annotations it states and no other.
func (c *layoutChecker) inDoc(scope *symbols.Scope) bool {
	for sc := scope; sc != nil; sc = sc.Parent() {
		if sc == c.docRoot {
			return true
		}
	}
	return false
}

// check validates every DiagramLayout annotation of sym stated in the document.
func (c *layoutChecker) check(sym *symbols.Symbol) {
	sites := c.model.LayoutSitesOf(sym)
	if len(sites) == 0 {
		return
	}
	firstInView := map[viewKey]*semantics.LayoutSite{}
	for _, site := range sites {
		if site.View != nil {
			key := viewKey{view: site.View.Decl, typeFQN: site.TypeFQN}
			if _, dup := firstInView[key]; !dup {
				firstInView[key] = site
			} else if c.inDoc(site.Scope) {
				c.warnf(site.Node.Span(), layoutDuplicateCode,
					"%s about %s is already stated in %s; the first stated applies",
					layoutTypeName(site.TypeFQN), c.describe(sym), c.describe(site.View))
			}
		}
		if !c.inDoc(site.Scope) {
			continue
		}
		for _, problem := range site.Problems {
			c.errorf(problem.Node.Span(), layoutValueCode, "%s: %s", c.describe(sym), problem.Message)
		}
		switch site.TypeFQN {
		case semantics.CanvasFQN:
			if !semantics.IsView(sym) {
				c.errorf(site.Node.Span(), layoutCanvasCode,
					"Canvas annotates %s, which is no view; a Canvas belongs in the body of the view it sizes", c.describe(sym))
			} else if !site.StatedInBodyOf(sym) {
				c.errorf(site.Node.Span(), layoutCanvasCode,
					"Canvas about %s is stated outside its body and sizes nothing; a Canvas belongs in the body of the view it sizes", c.describe(sym))
			}
		case semantics.LayoutFQN, semantics.RouteFQN:
			c.checkPlaced(site, sym)
		}
	}
}

// viewKey identifies the annotations of one kind stated in one view's body; the
// view's declaration stands for it across re-indexed symbols.
type viewKey struct {
	view    ast.Node
	typeFQN string
}

// checkPlaced warns when the rendering a Layout or Route applies to draws no
// node, or no edge, for the element: the rendering of the view an `about`
// annotation is stated in, any kind for an annotation applying in every view.
func (c *layoutChecker) checkPlaced(site *semantics.LayoutSite, sym *symbols.Symbol) {
	asLayout := site.TypeFQN == semantics.LayoutFQN
	if site.View == nil {
		node, edge := c.renderer.DrawsAnywhere(sym)
		if asLayout && !node {
			c.warnf(site.Node.Span(), layoutUnplacedCode,
				"Layout positions %s, which no rendering draws as a node", c.describe(sym))
		}
		if !asLayout && !edge {
			c.warnf(site.Node.Span(), layoutUnplacedCode,
				"Route steers %s, which no rendering draws as an edge", c.describe(sym))
		}
		return
	}
	kind, _, err := c.renderer.KindOf(site.View)
	if err != nil {
		return
	}
	drawn, ok := c.drawnIn(site.View)
	if !ok {
		return
	}
	if asLayout && !drawn.Node(sym) {
		c.warnf(site.Node.Span(), layoutUnplacedCode,
			"Layout positions %s, which the %s rendering of %s does not draw as a node",
			c.describe(sym), kind, c.describe(site.View))
	}
	if !asLayout && !drawn.Edge(sym) {
		c.warnf(site.Node.Span(), layoutUnplacedCode,
			"Route steers %s, which the %s rendering of %s does not draw as an edge",
			c.describe(sym), kind, c.describe(site.View))
	}
}

// drawnIn is what the rendering of v draws, rendered once per view; false when
// the view does not render, which other passes report.
func (c *layoutChecker) drawnIn(v *symbols.Symbol) (*view.Drawn, bool) {
	if drawn, ok := c.drawn[v.Decl]; ok {
		return drawn, drawn != nil
	}
	drawn, err := c.renderer.DrawnIn(v)
	if err != nil {
		drawn = nil
	}
	c.drawn[v.Decl] = drawn
	return drawn, drawn != nil
}

// layoutTypeName is the simple name of a DiagramLayout metadata definition.
func layoutTypeName(fqn string) string {
	return fqn[strings.LastIndex(fqn, "::")+2:]
}

// describe names an element as the notation declares it: "part def Kit::Cog",
// or "an unnamed transition of state def Kit::Motor".
func (c *layoutChecker) describe(sym *symbols.Symbol) string {
	if sym.Name == "" {
		if sym.OwnerScope != nil && sym.OwnerScope.Owner() != nil {
			return "an unnamed " + sym.Notation() + " of " + c.describe(sym.OwnerScope.Owner())
		}
		return "an unnamed " + sym.Notation()
	}
	name := c.fqn(sym)
	if name == "" {
		name = sym.Name
	}
	return sym.Notation() + " " + name
}

func (c *layoutChecker) errorf(span source.Span, code, format string, args ...any) {
	c.diags = append(c.diags, Diagnostic{
		Severity: SeverityError, Span: span, Message: fmt.Sprintf(format, args...), Code: code, Source: "constraint",
	})
}

func (c *layoutChecker) warnf(span source.Span, code, format string, args ...any) {
	c.diags = append(c.diags, Diagnostic{
		Severity: SeverityWarning, Span: span, Message: fmt.Sprintf(format, args...), Code: code, Source: "constraint",
	})
}
