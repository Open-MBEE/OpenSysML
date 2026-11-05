package view

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The kinds of the annotation nodes of SysML v2 §8.2.3.4: a comment, a
// documentation and a textual representation, each drawn as a note-shaped box
// joined to what it annotates by a dashed annotation link.
const (
	commentKind = "comment"
	docKind     = "doc"
	repKind     = "rep"
)

// annotationKind reports whether a node is an annotation node.
func annotationKind(kind string) bool {
	return kind == commentKind || kind == docKind || kind == repKind
}

// annotatingSymbol reports whether sym declares an annotation node: a comment,
// a documentation or a textual representation.
func annotatingSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch sym.Kind {
	case symbols.SymbolComment, symbols.SymbolDocumentation, symbols.SymbolTextualRepresentation:
		return true
	}
	return false
}

// annotatingMembers are the comments, documentation and textual
// representations declared in sym's own body, in declaration order.
func (r *Renderer) annotatingMembers(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || sym.Scope == nil {
		return nil
	}
	var out []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{}
	for _, list := range [][]*symbols.Symbol{sym.Scope.Members(), sym.Scope.AnonymousMembers()} {
		for _, member := range list {
			if member == sym || seen[member] || member.Implied() || !annotatingSymbol(member) {
				continue
			}
			seen[member] = true
			out = append(out, member)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DeclSpan.Offset < out[j].DeclSpan.Offset })
	return out
}

// annotationNode is the node drawing an annotating element: its kind heads it
// (`«comment»`, `«doc»`, `«rep»`), its name when it has one, its locale or
// language as the detail, and its body as the text. Nil when the body cannot
// be read and nothing else would be shown.
func (r *Renderer) annotationNode(ids *nodeIDs, view, sym *symbols.Symbol, out *Rendering) *Node {
	node := &Node{Kind: declKind(sym), Name: localName(sym), NameSynthesized: r.model.NameSynthesized(sym), Origin: symbolOrigin(sym)}
	switch decl := sym.Decl.(type) {
	case *ast.Comment:
		node.Text = r.annotationBody(sym, decl.BodySpan)
		node.Detail = localeDetail(decl.Locale)
	case *ast.Documentation:
		node.Text = r.annotationBody(sym, decl.BodySpan)
		node.Detail = localeDetail(decl.Locale)
	case *ast.TextualRepresentation:
		node.Text = r.annotationBody(sym, decl.BodySpan)
		if decl.Language != "" {
			node.Detail = "language " + decl.Language
		}
	default:
		return nil
	}
	if node.Text == "" && shown(node) == "" {
		return nil
	}
	node.ID = ids.take()
	node.Geometry = r.geometryOf(view, sym, out)
	node.Style = r.styleOf(view, sym, out)
	out.noteTreeSite(node, view, sym)
	r.notesOf(view, sym, node.ID, out)
	return node
}

// annotationBody is the prose of an annotation's body, its comment delimiters
// and leading asterisks off; "" when the source cannot be read.
func (r *Renderer) annotationBody(sym *symbols.Symbol, span source.Span) string {
	if r.text == nil || sym.DocName == "" {
		return ""
	}
	return strings.TrimSpace(source.CommentProse(r.text(sym.DocName, span)))
}

// localeDetail is the `locale "xx"` line of a comment or documentation stating
// one; the locale is kept as written, its quotes included.
func localeDetail(locale string) string {
	if locale == "" {
		return ""
	}
	return "locale " + locale
}

// annotatedElements are the elements an annotating element annotates: the ones
// a comment's `about` names and resolves, else its owning namespace (KerML
// §7.2.5: an annotating element with no annotation annotates its owner).
func (r *Renderer) annotatedElements(sym *symbols.Symbol) []*symbols.Symbol {
	if comment, ok := sym.Decl.(*ast.Comment); ok && len(comment.About) > 0 {
		var out []*symbols.Symbol
		for _, about := range comment.About {
			if about == nil {
				continue
			}
			if target, ok := r.resolveEnd(sym.OwnerScope, about); ok && target != nil {
				out = append(out, target)
			}
		}
		return out
	}
	if owner := sym.Owner(); owner != nil {
		return []*symbols.Symbol{owner}
	}
	return nil
}

// annotationRowText is the text of an annotation listed as a compartment row:
// its name, its locale or language, then its body, a line each.
func annotationRowText(node *Node) string {
	var lines []string
	if name := shown(node); name != "" {
		lines = append(lines, name)
	}
	if node.Detail != "" {
		lines = append(lines, node.Detail)
	}
	if node.Text != "" {
		lines = append(lines, node.Text)
	}
	return strings.Join(lines, "\n")
}
