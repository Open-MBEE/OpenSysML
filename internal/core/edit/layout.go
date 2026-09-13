package edit

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// layoutBinding is one feature a DiagramLayout annotation body binds, spelled
// as the writer puts it.
type layoutBinding struct {
	feature string
	literal string
}

// layoutFeatures are the features each DiagramLayout metadata definition
// declares, in the order the writer binds them.
var layoutFeatures = map[string][]string{
	semantics.LayoutFQN: {"x", "y", "width", "height", "collapsed"},
	semantics.RouteFQN:  {"points"},
	semantics.CanvasFQN: {"unit", "width", "height"},
}

// layoutSplices turns a set-layout operation into the bytes it rewrites: the
// values of an annotation already there, a new annotation where the notation
// puts one, or the removal of the one being cleared.
func (m Model) layoutSplices(i int, op Operation) ([]splice, error) {
	features, known := layoutFeatures[op.Annotation]
	if !known {
		return nil, &Error{Failure: FailureInvalidValue, OperationIndex: i,
			Message: fmt.Sprintf("%q is no DiagramLayout annotation; the annotations are %s, %s and %s",
				op.Annotation, semantics.LayoutFQN, semantics.RouteFQN, semantics.CanvasFQN)}
	}
	bindings, err := layoutBindings(i, op)
	if err != nil {
		return nil, err
	}
	sym, viewSym, err := m.layoutTargets(i, op)
	if err != nil {
		return nil, err
	}
	r, sem := m.resolver()
	renderer := view.NewRenderer(sem, r, nil)
	if err := m.checkPlaceable(i, op, renderer, sem, sym, viewSym); err != nil {
		return nil, err
	}
	site := m.layoutSite(sem, sym, viewSym, op.Annotation)
	if bindings == nil {
		if site == nil {
			return nil, &Error{Failure: FailureNotAnnotated, OperationIndex: i,
				Message: fmt.Sprintf("%s carries no %s%s to clear", op.Target,
					layoutTypeName(op.Annotation), inView(op.View))}
		}
		return []splice{m.clearAnnotation(i, op, site, sym)}, nil
	}
	if site == nil {
		return []splice{m.insertAnnotation(i, op, bindings, sym, viewSym)}, nil
	}
	return m.updateAnnotation(i, op, site, bindings, features), nil
}

// layoutBindings spells the geometry an operation writes, feature by feature,
// or nil when the operation clears the annotation.
func layoutBindings(i int, op Operation) ([]layoutBinding, error) {
	switch op.Annotation {
	case semantics.LayoutFQN:
		if op.Layout == nil {
			return nil, nil
		}
		out := []layoutBinding{{"x", realLiteral(op.Layout.X)}, {"y", realLiteral(op.Layout.Y)}}
		if op.Layout.HasSize {
			out = append(out, layoutBinding{"width", realLiteral(op.Layout.Width)}, layoutBinding{"height", realLiteral(op.Layout.Height)})
		}
		if op.Layout.Collapsed {
			out = append(out, layoutBinding{"collapsed", "true"})
		}
		return out, nil
	case semantics.RouteFQN:
		if op.Route == nil {
			return nil, nil
		}
		if len(op.Route.Points) == 0 {
			return nil, &Error{Failure: FailureInvalidValue, OperationIndex: i,
				Message: fmt.Sprintf("a Route of %s needs at least one waypoint; clear the Route to route it straight", op.Target)}
		}
		values := make([]string, 0, 2*len(op.Route.Points))
		for _, p := range op.Route.Points {
			values = append(values, realLiteral(p.X), realLiteral(p.Y))
		}
		return []layoutBinding{{"points", "(" + strings.Join(values, ", ") + ")"}}, nil
	case semantics.CanvasFQN:
		if op.Canvas == nil {
			return nil, nil
		}
		var out []layoutBinding
		if op.Canvas.Unit != "" {
			out = append(out, layoutBinding{"unit", stringLiteral(op.Canvas.Unit)})
		}
		if op.Canvas.HasSize {
			out = append(out, layoutBinding{"width", realLiteral(op.Canvas.Width)}, layoutBinding{"height", realLiteral(op.Canvas.Height)})
		}
		if len(out) == 0 {
			return nil, &Error{Failure: FailureInvalidValue, OperationIndex: i,
				Message: fmt.Sprintf("a Canvas of %s binds neither a unit nor a size; clear the Canvas to drop it", op.Target)}
		}
		return out, nil
	}
	return nil, nil
}

// realLiteral spells a coordinate as the notation's Real literal.
func realLiteral(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// stringLiteral spells s as the notation's String literal.
func stringLiteral(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// layoutTargets resolves the element an operation lays out and the view whose
// body states the layout. Whatever the operation writes into must be declared
// in this document: the element for an inline annotation, the view for one in
// its body; an element another document declares may still be placed by a view
// of this one.
func (m Model) layoutTargets(i int, op Operation) (sym, viewSym *symbols.Symbol, err error) {
	if op.Annotation == semantics.CanvasFQN || op.View == "" {
		sym, err = m.target(i, op)
		if err != nil {
			return nil, nil, err
		}
	} else {
		sym, err = m.declaredOnce(i, op.Target)
		if err != nil {
			return nil, nil, err
		}
	}
	if op.Annotation == semantics.CanvasFQN {
		if op.View != "" {
			return nil, nil, &Error{Failure: FailureInvalidValue, OperationIndex: i,
				Message: "a Canvas is stated in the body of the view it sizes; name that view as the target and no other"}
		}
		if !semantics.IsView(sym) {
			return nil, nil, &Error{Failure: FailureNotAView, OperationIndex: i,
				Message: fmt.Sprintf("%s is no view; a Canvas sizes the drawing surface of a view", op.Target)}
		}
		return sym, nil, nil
	}
	if op.View == "" {
		return sym, nil, nil
	}
	viewSym, err = m.target(i, Operation{Target: op.View})
	if err != nil {
		return nil, nil, err
	}
	if !semantics.IsView(viewSym) {
		return nil, nil, &Error{Failure: FailureNotAView, OperationIndex: i,
			Message: fmt.Sprintf("%s is no view; a %s applies in the view whose body states it",
				op.View, layoutTypeName(op.Annotation))}
	}
	return sym, viewSym, nil
}

// checkPlaceable refuses a Layout or Route of an element the rendering it
// applies in does not draw as a node or an edge: the view's rendering for a
// view-local one, any rendering for an inline one.
func (m Model) checkPlaceable(i int, op Operation, renderer *view.Renderer, sem *semantics.Model, sym, viewSym *symbols.Symbol) error {
	if op.Annotation == semantics.CanvasFQN {
		return nil
	}
	asLayout := op.Annotation == semantics.LayoutFQN
	role := "an edge a Route steers"
	if asLayout {
		role = "a node a Layout positions"
	}
	if viewSym == nil {
		node, edge := renderer.DrawsAnywhere(sym)
		if (asLayout && !node) || (!asLayout && !edge) {
			return &Error{Failure: FailureNotDrawn, OperationIndex: i,
				Message: fmt.Sprintf("no rendering draws %s as %s", op.Target, role)}
		}
		return nil
	}
	if !m.exposes(sem, viewSym, sym) {
		return &Error{Failure: FailureNotExposed, OperationIndex: i,
			Message: fmt.Sprintf("%s does not expose %s, so a %s in its body would place nothing",
				op.View, op.Target, layoutTypeName(op.Annotation))}
	}
	drawn, err := renderer.DrawnIn(viewSym)
	if err != nil {
		return &Error{Failure: FailureNotDrawn, OperationIndex: i,
			Message: fmt.Sprintf("%s does not render: %v", op.View, err)}
	}
	if (asLayout && !drawn.Node(sym)) || (!asLayout && !drawn.Edge(sym)) {
		return &Error{Failure: FailureNotDrawn, OperationIndex: i,
			Message: fmt.Sprintf("the rendering of %s does not draw %s as %s", op.View, op.Target, role)}
	}
	return nil
}

// exposes reports whether view exposes sym or an element sym is nested in.
func (m Model) exposes(sem *semantics.Model, viewSym, sym *symbols.Symbol) bool {
	exposed, err := sem.ExposedElements(viewSym)
	if err != nil {
		return false
	}
	for cur := sym; cur != nil; cur = ownerOf(cur) {
		for _, e := range exposed {
			if e == cur || (e.Decl != nil && e.Decl == cur.Decl) {
				return true
			}
		}
	}
	return false
}

// ownerOf is the element declaring sym, or nil at the root.
func ownerOf(sym *symbols.Symbol) *symbols.Symbol {
	if sym.OwnerScope == nil {
		return nil
	}
	return sym.OwnerScope.Owner()
}

// layoutSite is the annotation of type typeFQN the operation rewrites: the one
// stated in viewSym's body about sym, or, for an inline operation, the one
// applying in every view; nil when none is stated in this document.
func (m Model) layoutSite(sem *semantics.Model, sym, viewSym *symbols.Symbol, typeFQN string) *semantics.LayoutSite {
	for _, site := range sem.LayoutSitesOf(sym) {
		if site.TypeFQN != typeFQN || symbols.DocNameOf(site.Scope) != m.Source.Name() {
			continue
		}
		if viewSym == nil && site.View == nil {
			return site
		}
		if viewSym != nil && site.View != nil && site.View.Decl == viewSym.Decl {
			return site
		}
	}
	return nil
}

// insertAnnotation is the insertion of a new annotation: inline in the body of
// sym, or stated about it in the body of viewSym.
func (m Model) insertAnnotation(i int, op Operation, bindings []layoutBinding, sym, viewSym *symbols.Symbol) splice {
	owner := sym
	text := "@" + op.Annotation
	if viewSym != nil {
		owner = viewSym
		text = "metadata " + op.Annotation + " about " + notationName(sym)
	}
	span, insertion := m.memberInsertion(owner.Decl, text+" "+writeBindings(bindings))
	return splice{span: span, text: insertion, opIndex: i, target: op.Target}
}

// writeBindings spells an annotation body on one line.
func writeBindings(bindings []layoutBinding) string {
	parts := make([]string, len(bindings))
	for j, b := range bindings {
		parts[j] = b.feature + " = " + b.literal + ";"
	}
	return "{ " + strings.Join(parts, " ") + " }"
}

// updateAnnotation rewrites an annotation in place: the value of each feature
// it already binds, a removal of each it binds and the operation drops, and an
// insertion of each the operation adds, after the last binding kept.
func (m Model) updateAnnotation(i int, op Operation, site *semantics.LayoutSite, bindings []layoutBinding, features []string) []splice {
	wanted := map[string]string{}
	for _, b := range bindings {
		wanted[b.feature] = b.literal
	}
	known := map[string]bool{}
	for _, f := range features {
		known[f] = true
	}
	var out []splice
	bound := map[string]bool{}
	var lastKept *ast.Usage
	for _, b := range site.Bindings {
		if !known[b.Feature] || b.Node == nil {
			continue
		}
		literal, keep := wanted[b.Feature]
		if !keep {
			out = append(out, splice{span: m.bindingSpan(b.Node), opIndex: i, target: op.Target})
			continue
		}
		bound[b.Feature] = true
		lastKept = b.Node
		if b.Value == nil {
			continue
		}
		out = append(out, splice{span: m.tokenSpan(b.Value.Span()), text: literal, opIndex: i, target: op.Target})
	}
	var missing []layoutBinding
	for _, b := range bindings {
		if !bound[b.feature] {
			missing = append(missing, b)
		}
	}
	if len(missing) > 0 {
		out = append(out, m.bindingInsertion(i, op, site.Node, lastKept, missing))
	}
	return out
}

// bindingSpan is the bytes a binding member's removal takes: the member, its
// `;`, and the whitespace before it on its line.
func (m Model) bindingSpan(member *ast.Usage) source.Span {
	content := m.Source.Bytes()
	start := member.Span().Offset
	end := m.tokenSpan(member.Span()).End()
	if semi, ok := m.terminator(member); ok && semi.End() > end {
		end = semi.End()
	}
	for start > 0 && (content[start-1] == ' ' || content[start-1] == '\t') {
		start--
	}
	if start > 0 && content[start-1] == '\n' && onlyWhitespace(content[start:member.Span().Offset]) {
		start--
		if start > 0 && content[start-1] == '\r' {
			start--
		}
	}
	return source.Span{Offset: start, Len: end - start}
}

// bindingInsertion inserts bindings into an annotation body after the last
// binding kept, or at the body's opening brace when none is; each on the line
// of the last binding when the body is written on one line, else on its own.
func (m Model) bindingInsertion(i int, op Operation, node ast.Node, after *ast.Usage, bindings []layoutBinding) splice {
	content := m.Source.Bytes()
	lbrace, rbrace := m.bodyBraces(node.Span())
	at := lbrace.End()
	if after != nil {
		at = m.tokenSpan(after.Span()).End()
		if semi, ok := m.terminator(after); ok && semi.End() > at {
			at = semi.End()
		}
	}
	var sep string
	if strings.IndexByte(string(content[lbrace.Offset:rbrace.End()]), '\n') < 0 {
		sep = " "
	} else if after != nil {
		sep = "\n" + lineIndent(content, after.Span().Offset)
	} else {
		sep = "\n" + m.memberIndent(node.Span())
	}
	var text strings.Builder
	for _, b := range bindings {
		text.WriteString(sep + b.feature + " = " + b.literal + ";")
	}
	return splice{span: source.Span{Offset: at}, text: text.String(), opIndex: i, target: op.Target}
}

// bodyBraces are the braces of the outermost body written within span: the last
// `}` and the `{` it closes.
func (m Model) bodyBraces(span source.Span) (lbrace, rbrace source.Span) {
	var opens []source.Span
	lx := lexer.New(m.Source)
	for tok := lx.Next(); tok.Kind != lexer.EOF && tok.Span.Offset < span.End(); tok = lx.Next() {
		if tok.Span.Offset < span.Offset {
			continue
		}
		switch tok.Kind {
		case lexer.LBrace:
			opens = append(opens, tok.Span)
		case lexer.RBrace:
			if len(opens) == 1 {
				lbrace, rbrace = opens[0], tok.Span
			}
			if len(opens) > 0 {
				opens = opens[:len(opens)-1]
			}
		}
	}
	return lbrace, rbrace
}

// clearAnnotation removes an annotation with its owned trivia. An inline
// annotation that was the whole body of sym's declaration takes the body with
// it, so the declaration reads `…;` as it did before it was placed.
func (m Model) clearAnnotation(i int, op Operation, site *semantics.LayoutSite, sym *symbols.Symbol) splice {
	removed := m.deleteSpan(deletion{node: site.Node, span: site.Node.Span()})
	if !site.About && sym.Decl != nil && sym.DocName == m.Source.Name() {
		if body, ok := m.emptiedBody(sym.Decl, removed); ok {
			return splice{span: body, text: ";", opIndex: i, target: op.Target}
		}
	}
	return splice{span: removed, opIndex: i, target: op.Target}
}

// emptiedBody is the span of decl's body, with the space before its `{`, when
// removing removed leaves nothing but whitespace in it.
func (m Model) emptiedBody(decl ast.Node, removed source.Span) (source.Span, bool) {
	body, hasBody := bodyInfo(decl)
	if !hasBody {
		return source.Span{}, false
	}
	lbrace, rbrace := m.bodyBraces(body)
	if rbrace.Len == 0 || !encloses(source.Span{Offset: lbrace.End(), Len: rbrace.Offset - lbrace.End()}, removed) {
		return source.Span{}, false
	}
	content := m.Source.Bytes()
	rest := string(content[lbrace.End():removed.Offset]) + string(content[removed.End():rbrace.Offset])
	if strings.TrimSpace(rest) != "" {
		return source.Span{}, false
	}
	start := lbrace.Offset
	for start > 0 && isSpace(content[start-1]) {
		start--
	}
	return source.Span{Offset: start, Len: rbrace.End() - start}, true
}

// layoutTypeName is the simple name of a DiagramLayout metadata definition.
func layoutTypeName(fqn string) string {
	return fqn[strings.LastIndex(fqn, "::")+2:]
}

// inView says where a view-local annotation applies, for a message.
func inView(viewName string) string {
	if viewName == "" {
		return ""
	}
	return " in " + viewName
}
