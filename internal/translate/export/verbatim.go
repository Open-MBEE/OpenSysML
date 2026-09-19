package export

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// Source text is printed only while it states what the graph states: the
// candidate notation is converted back and each disagreement demotes its text.

// render prints the roots from source text where it agrees with the graph and
// canonically where it does not; every pass demotes more text or is the last.
// A normative id is left implied only while the notation reads as the library
// document that implies it; otherwise the pass is redone with the ids written.
func (d *decoder) render(roots []*element) (string, error) {
	for {
		d.printed = map[*element]bool{}
		d.rebuilt = map[*element]bool{}
		d.prefixed = map[*element]bool{}
		d.usedExpr = map[string]bool{}
		d.written = nil
		d.implied = 0
		if err := d.resolveExpressions(); err != nil {
			return "", err
		}
		var b strings.Builder
		for _, root := range roots {
			if err := d.print(&b, root, 0); err != nil {
				return "", err
			}
		}
		if d.implied > 0 && !d.libraryNotation(b.String(), roots) {
			d.explicit = true
			continue
		}
		if len(d.printed)+len(d.usedExpr) == 0 {
			return b.String(), nil
		}
		before := len(d.demoted) + len(d.demotedExpr)
		d.demoteStale(b.String(), roots)
		if len(d.demoted)+len(d.demotedExpr) == before {
			return b.String(), nil
		}
	}
}

// libraryNotation reports whether the encoder reads notation as the bundled
// library document the graph is a version of, which derives the norm's ids on its own.
func (d *decoder) libraryNotation(notation string, roots []*element) bool {
	name, ok := d.candidateName(roots)
	if !ok || d.library == "" {
		return false
	}
	file := source.New(name, []byte(notation))
	p := parser.New(file)
	root := p.ParseFile()
	return len(p.Diagnostics) == 0 && documentLibrary(file, root) == d.library
}

// verbatim returns the source text an element is printed as, if it carries
// one it has not been demoted from.
func (d *decoder) verbatim(el *element) (string, bool) {
	if d.demoted[el] {
		return "", false
	}
	return d.graph.Lexical(rdf.IRI(el.iri), rdf.OpenSysML+xSourceText)
}

// expressionText returns the source text an expression node is written as, if
// it carries one it has not been demoted from; an Expression element's text is
// its lines, which verbatim prints, so its structure is rebuilt here.
func (d *decoder) expressionText(node rdf.Term) (string, bool) {
	if _, isElement := d.byIRI[node.Value]; isElement || d.demotedExpr[node.Value] {
		return "", false
	}
	text, ok := d.graph.Lexical(node, rdf.OpenSysML+xSourceText)
	if ok && text != "" {
		d.usedExpr[node.Value] = true
	}
	return text, ok && text != ""
}

// demoteStale demotes whatever verbatim text contradicts the graph; notation
// that does not convert, or whose disagreement cannot be placed, demotes all.
func (d *decoder) demoteStale(notation string, roots []*element) {
	name, ok := d.candidateName(roots)
	if !ok {
		d.demoteAll()
		return
	}
	file := source.New(name, []byte(notation))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		d.demoteAll()
		return
	}
	check, err := encodeDocument(file, root, d.library)
	if err != nil {
		d.demoteAll()
		return
	}
	for _, t := range disagreeingTriples(d.graph, check.graph) {
		// A reference to an element only one graph has disagrees about that
		// element's identity, not about the element referring to it.
		var el *element
		var expr string
		if t.Object.IsIRI() && d.graph.HasProperty(t.Object, rdf.RDFType) != check.graph.HasProperty(t.Object, rdf.RDFType) {
			el, expr = d.blame(t.Object.Value, check)
		}
		if el == nil && expr == "" {
			el, expr = d.blame(t.Subject.Value, check)
		}
		if el == nil && expr == "" && t.Object.IsIRI() {
			el, expr = d.blame(t.Object.Value, check)
		}
		// One landing on notation already rebuilt is the graph's own, not its text's.
		switch {
		case el != nil && d.printed[el]:
			d.demoted[el] = true
		case expr != "":
			d.demotedExpr[expr] = true
		case el == nil:
			d.demoteAll()
			return
		}
	}
}

// candidateName names the candidate notation for the grammar the roots record
// their text was written in; a root with text recording none was read as a
// buffer with no extension, and the candidate is named so it reads the same
// way. Roots recording different grammars cannot be read as one document; one
// with neither text nor grammar was added to the graph and says nothing, and a
// graph whose roots all say nothing is read in the grammar of the library
// document it is a version of, if it is one.
func (d *decoder) candidateName(roots []*element) (string, bool) {
	language, seen := "", false
	for _, root := range roots {
		recorded, ok := d.stringOf(root, rdf.OpenSysML+xSourceLanguage)
		if _, hasText := d.graph.Lexical(rdf.IRI(root.iri), rdf.OpenSysML+xSourceText); !ok && !hasText {
			continue
		}
		if seen && recorded != language {
			return "", false
		}
		language, seen = recorded, true
	}
	if !seen {
		language = d.libraryLanguage()
	}
	if language == "" {
		return "<converted>", true
	}
	return "<converted>." + language, true
}

// libraryLanguage is the grammar the library document the graph is a version of
// is written in, as languageName records it; "" for a graph that is no library.
func (d *decoder) libraryLanguage() string {
	if d.library == "" {
		return ""
	}
	return languageName(source.KindOf(d.library))
}

// demoteAll demotes every verbatim text and expression printed in this pass.
func (d *decoder) demoteAll() {
	for el := range d.printed {
		d.demoted[el] = true
	}
	for iri := range d.usedExpr {
		d.demotedExpr[iri] = true
	}
}

// disagreeingTriples lists the structural triples only one of the two graphs
// states. Source text differs from canonical notation by design, and a member
// index states an order the notation keeps whatever the numbers, so both skip.
func disagreeingTriples(graph, check *rdf.Graph) []rdf.Triple {
	var out []rdf.Triple
	structural := func(t rdf.Triple) bool {
		switch t.Predicate.Value {
		case rdf.OpenSysML + xSourceText, rdf.OpenSysML + xSourceTail, rdf.OpenSysML + xSourceLanguage,
			rdf.OpenSysML + xMemberIndex:
			return false
		}
		return true
	}
	for _, t := range graph.Triples() {
		if structural(t) && !check.Has(t) {
			out = append(out, t)
		}
	}
	for _, t := range check.Triples() {
		if structural(t) && !graph.Has(t) {
			out = append(out, t)
		}
	}
	return out
}

// blame returns the verbatim element a disagreement over iri falls in, or else
// the outermost expression node written from its text. A subject only the
// candidate has is traced to the element written over where it was parsed.
func (d *decoder) blame(iri string, check *encoder) (*element, string) {
	var expr string
	visited := map[string]bool{}
	for iri != "" && !visited[iri] {
		visited[iri] = true
		if el, ok := d.byIRI[iri]; ok {
			if target, ok := d.folded[el]; ok {
				el = target
			}
			return d.nearestVerbatim(el), expr
		}
		if d.usedExpr[iri] {
			expr = iri
		}
		if offset, ok := check.offsets[iri]; ok {
			return d.writerAt(offset), ""
		}
		holder := holderOf(d.graph, iri)
		if holder == "" {
			holder = holderOf(check.graph, iri)
		}
		iri = holder
	}
	return nil, expr
}

// nearestVerbatim returns the element whose notation a disagreement over el
// falls in: the nearest of el and its owners that was written at all. One
// already rebuilt canonically is returned as is, so the blame stops there.
func (d *decoder) nearestVerbatim(el *element) *element {
	for x := el; x != nil; x = x.owner {
		if d.printed[x] || d.rebuilt[x] {
			return x
		}
	}
	return nil
}

// writerAt returns the innermost element written over an offset of the
// candidate notation: the one whose text the notation parsed there came from.
func (d *decoder) writerAt(offset int) *element {
	var best *writing
	for i := range d.written {
		w := &d.written[i]
		if offset < w.where.start || offset >= w.where.end {
			continue
		}
		if best == nil || w.where.end-w.where.start < best.where.end-best.where.start {
			best = w
		}
	}
	if best == nil {
		return nil
	}
	return best.el
}

// holderOf returns the subject a node hangs off: the namespace or relationship
// owning an element, or the subject that points at a membership or expression node.
func holderOf(g *rdf.Graph, iri string) string {
	subject := rdf.IRI(iri)
	if g.HasProperty(subject, rdf.SysML+pQualifiedName) {
		for _, property := range []string{pOwningNamespace, pOwner} {
			if owner, ok := g.Object(subject, rdf.SysML+property); ok && owner.IsIRI() {
				return owner.Value
			}
		}
		return ""
	}
	if !g.HasProperty(subject, rdf.RDFType) {
		return ""
	}
	for _, t := range g.Triples() {
		if t.Object.IsIRI() && t.Object.Value == iri && t.Subject.Value != iri {
			return t.Subject.Value
		}
	}
	return ""
}
