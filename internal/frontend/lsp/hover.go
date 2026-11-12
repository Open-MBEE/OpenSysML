package lsp

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Hover returns type/kind information for the declaration under the cursor, in
// a workspace document or a bundled library one.
func (s *Server) Hover(ctx context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.document(name)
	if doc == nil || doc.Scope == nil {
		return nil, nil
	}
	content := doc.Content
	offset := positionToOffset(content, params.Position)

	// A cursor on a reference hovers what it names, not the declaration it sits
	// in: the type a usage declares, the query a document block invokes.
	if ref := refAtOffset(collectRefs(doc.AST, doc.Scope), offset); ref != nil {
		if target, span, ok := s.referencedSegment(name, *ref, offset); ok && target != nil {
			target, span = s.hydratedTarget(name, *ref, offset, target, span)
			signature := target.Notation()
			if target.Name != "" {
				signature += " " + source.NameText(target.Name)
			}
			if t := declaredTypeText(target); t != "" {
				signature += " : " + t
			}
			rng := spanToRange(content, span)
			return &protocol.Hover{
				Contents: s.hoverContents(signature, s.ws.DocumentationOf(target), s.symbolNotes(target), s.identityLine(target.DocName, target)),
				Range:    &rng,
			}, nil
		}
		if hover := s.ambiguousCallHover(name, content, *ref, offset); hover != nil {
			return hover, nil
		}
	}

	sym := symbolAtOffset(doc.Scope, offset)
	if sym == nil {
		return nil, nil
	}

	signature := sym.Notation()
	if sym.Name != "" {
		signature += " " + source.NameText(sym.Name)
	}
	// A metadata body declaration implicitly redefines a feature of the
	// annotation's metadata definition (KerML 7.4.7); name it and its type.
	if target, fqn, ok := s.ws.MetadataBodyRedefines(sym); ok {
		signature += " redefines " + fqn
		if t := declaredTypeText(target); t != "" {
			signature += " : " + t
		}
	}
	notes := leadingNotes(content, sym.LeadingTrivia)

	rng := spanToRange(content, sym.DeclSpan)
	return &protocol.Hover{
		Contents: s.hoverContents(signature, s.ws.DocumentationOf(sym), notes, s.identityLine(name, sym)),
		Range:    &rng,
	}, nil
}

// identityLine states a declared or normative element id; a derived id is the
// encoded name and goes unsaid.
func (s *Server) identityLine(doc string, sym *symbols.Symbol) string {
	info, ok := s.ws.IdentityOf(doc, sym)
	if !ok || info.Source == identity.SourceDerived {
		return ""
	}
	provenance := info.Source.String()
	if info.Fixed() {
		provenance += ", " + info.Language.String()
	}
	return fmt.Sprintf("Element id `%s` (%s)", info.EffectiveID, provenance)
}

// ambiguousCallHover lists the overloads a call's arguments leave tied when the
// cursor is on the called name itself; nil on a qualifier or a `::`. content is the
// document text ref was collected from.
func (s *Server) ambiguousCallHover(doc string, content []byte, ref resolve.Reference, offset int) *protocol.Hover {
	parts := ref.QN.Parts
	if len(parts) == 0 || segmentAt(ref, offset) != len(parts)-1 {
		return nil
	}
	last := parts[len(parts)-1]
	overloads := s.ws.AmbiguousInvocationInDoc(doc, ref)
	if len(overloads) == 0 {
		return nil
	}
	lines := make([]string, len(overloads))
	for i, sym := range overloads {
		lines[i] = sym.Notation() + " " + s.ws.FQNOf(sym)
	}
	rng := spanToRange(content, last.Span)
	return &protocol.Hover{
		Contents: s.hoverContents(strings.Join(lines, "\n"), []string{"Ambiguous call: the arguments fit each of these overloads equally."}, nil, ""),
		Range:    &rng,
	}
}

// hydratedTarget is the hovered reference's target over its declaring document
// hydrated when the workspace held that as its record: the documentation hover
// shows is read from the tree. A declaration outside the workspace is left as is.
func (s *Server) hydratedTarget(doc string, ref resolve.Reference, offset int, target *symbols.Symbol, span source.Span) (*symbols.Symbol, source.Span) {
	if !target.Recorded() || s.ws.Document(target.DocName) == nil {
		return target, span
	}
	if err := s.ws.Hydrate(target.DocName); err != nil {
		return target, span
	}
	if hydrated, hydratedSpan, ok := s.referencedSegment(doc, ref, offset); ok && hydrated != nil {
		return hydrated, hydratedSpan
	}
	return target, span
}

// symbolNotes returns the notes written directly above a symbol's declaration,
// when the document declaring it is loaded or is a bundled library file.
func (s *Server) symbolNotes(sym *symbols.Symbol) []string {
	if len(sym.LeadingTrivia) == 0 || sym.DocName == "" {
		return nil
	}
	doc := s.document(sym.DocName)
	if doc == nil {
		return nil
	}
	return leadingNotes(doc.Content, sym.LeadingTrivia)
}

// hoverContents renders the signature, owned documentation, notes and identity
// as Markdown when the client supports it, plain text otherwise.
func (s *Server) hoverContents(signature string, docs, notes []string, elementID string) *protocol.MarkupContent {
	if s.wantsMarkdownHover() {
		var b strings.Builder
		b.WriteString("```sysml\n")
		b.WriteString(signature)
		b.WriteString("\n```")
		if prose := documentationProse(docs, notes); prose != "" {
			b.WriteString("\n\n")
			b.WriteString(prose)
		}
		if elementID != "" {
			b.WriteString("\n\n")
			b.WriteString(elementID)
		}
		return &protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: b.String()}
	}

	value := signature
	if doc := documentationText(docs, notes); doc != "" {
		value += "\n\n" + doc
	}
	if elementID != "" {
		value += "\n\n" + strings.ReplaceAll(elementID, "`", "")
	}
	return &protocol.MarkupContent{Kind: protocol.MarkupKindPlainText, Value: value}
}

// documentationProse renders each owned `doc` body, then each note stripped of its
// delimiters, as a Markdown paragraph that keeps the line breaks it was written with.
func documentationProse(docs, notes []string) string {
	var paragraphs []string
	for _, body := range docs {
		paragraphs = append(paragraphs, strings.ReplaceAll(body, "\n", "  \n"))
	}
	for _, note := range notes {
		if prose := source.CommentProse(note); prose != "" {
			paragraphs = append(paragraphs, strings.ReplaceAll(prose, "\n", "  \n"))
		}
	}
	return strings.Join(paragraphs, "\n\n")
}

// documentationText renders an element's documentation as plain text: the
// owned `doc` bodies as prose, then the notes as written.
func documentationText(docs, notes []string) string {
	return strings.Join(append(slices.Clone(docs), notes...), "\n")
}

// symbolAtOffset finds the innermost symbol whose DeclSpan contains offset.
// Nested scopes are searched first, including those an anonymous declaration
// owns and those no symbol owns at all — a loop body or the parameters of a
// body expression.
func symbolAtOffset(scope *symbols.Scope, offset int) *symbols.Symbol {
	for _, child := range scope.Children() {
		node := child.Node()
		if node == nil {
			continue
		}
		sp := node.Span()
		if offset < sp.Offset || offset >= sp.End() {
			continue
		}
		if inner := symbolAtOffset(child, offset); inner != nil {
			return inner
		}
	}
	for _, sym := range scope.Members() {
		sp := sym.DeclSpan
		if offset >= sp.Offset && offset < sp.End() {
			return sym
		}
	}
	return nil
}

// leadingNotes returns the text of each `//` and `//* */` note directly above a
// declaration; a bare `/* */` there is a Comment of the namespace, not a note.
func leadingNotes(content []byte, trivia []ast.Trivia) []string {
	if len(trivia) == 0 {
		return nil
	}
	var parts []string
	for _, tr := range trivia {
		switch tr.Kind {
		case ast.TriviaBlockNote, ast.TriviaLineNote:
			start, end := tr.Span.Offset, tr.Span.End()
			if start < 0 || end > len(content) || start > end {
				continue
			}
			if text := strings.TrimSpace(string(content[start:end])); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return parts
}
