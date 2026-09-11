package lsp

import (
	"bytes"
	"fmt"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/core/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/quickfix"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/interop/reposync"
)

// identityActionKind is the kind of the opt-in identity annotation actions.
const identityActionKind = protocol.RefactorRewrite

// placeholderProjectID is what a minted ProjectRef binds to until the user
// fills in the repository project.
const placeholderProjectID = "<projectId>"

// identityActions offers, for the declaration whose header the range touches,
// minting an ElementId it lacks and binding an unbound root to a project.
func (s *Server) identityActions(name string, doc *model.Document, want source.Span) ([]protocol.CodeAction, error) {
	sym := s.declarationAt(doc, want)
	if sym == nil {
		return nil, nil
	}
	info, ok := s.ws.IdentityOf(name, sym)
	if !ok {
		return nil, nil
	}
	root := identity.Root(info.Symbol)
	uri := nameToURI(name)
	var out []protocol.CodeAction
	if info.Scope == nil && root == info.Symbol {
		out = append(out, protocol.CodeAction{
			Title: fmt.Sprintf("Bind '%s' to a project", sym.Name),
			Kind:  identityActionKind,
			Edit:  workspaceEdit(uri, doc.Content, annotate(doc.Content, []annotation{projectRef(root)})),
		})
	}
	if info.Annotated {
		return out, nil
	}
	id, err := reposync.MintUUID()
	if err != nil {
		return nil, err
	}
	title := fmt.Sprintf("Annotate '%s' with a minted element id", sym.Name)
	notes := []annotation{elementID(info.Symbol, info.FQN, id)}
	if info.Scope == nil {
		title += fmt.Sprintf(" and bind '%s' to a project", root.Name)
		notes = append([]annotation{projectRef(root)}, notes...)
	}
	out = append(out, protocol.CodeAction{
		Title: title,
		Kind:  identityActionKind,
		Edit:  workspaceEdit(uri, doc.Content, annotate(doc.Content, notes)),
	})
	return out, nil
}

// declarationAt returns the declaration whose header — the text before its
// body, or the whole declaration when it has none — the range lies in, trivia
// and comments at either end of a selection (a whole-line selection) skipped.
func (s *Server) declarationAt(doc *model.Document, want source.Span) *symbols.Symbol {
	at, until := want.Offset, want.End()
	if tokens := trimComments(tokensIn(doc.Content, want)); len(tokens) > 0 {
		at, until = tokens[0].Span.Offset, tokens[len(tokens)-1].Span.End()
	}
	sym := symbolAtOffset(doc.Scope, at)
	if sym == nil || sym.Decl == nil || s.ws.EnclosingMetadataBody(sym.OwnerScope) != nil {
		return nil
	}
	body, hasBody := bodyOf(doc.Content, sym.DeclSpan)
	end := body.End()
	if hasBody {
		end = body.Offset + 1
	}
	if until > end {
		return nil
	}
	return sym
}

// trimComments drops the regular comments at either end of tokens.
func trimComments(tokens []lexer.Token) []lexer.Token {
	for len(tokens) > 0 && tokens[0].Kind == lexer.RegularComment {
		tokens = tokens[1:]
	}
	for len(tokens) > 0 && tokens[len(tokens)-1].Kind == lexer.RegularComment {
		tokens = tokens[:len(tokens)-1]
	}
	return tokens
}

// annotation is one metadata annotation to write: inline in the target's body
// when it has one, else standalone (about-form) at the end of the file.
type annotation struct {
	target *symbols.Symbol
	inline string
	about  string
}

func elementID(target *symbols.Symbol, fqn, id string) annotation {
	path := lexer.QualifiedNameText(fqn)
	return annotation{target: target, inline: identity.ElementIdInline(id), about: identity.ElementIdAbout(path, id)}
}

// projectRef binds a root declaration, so its qualified name is its own.
func projectRef(target *symbols.Symbol) annotation {
	path := lexer.NameText(target.Name)
	return annotation{
		target: target,
		inline: identity.ProjectRefInline(placeholderProjectID),
		about:  identity.ProjectRefAbout(path, placeholderProjectID),
	}
}

// annotate computes the edits writing the annotations, in order, without
// touching any other text of the file.
func annotate(content []byte, notes []annotation) []quickfix.Edit {
	var edits []quickfix.Edit
	var appended []string
	inline := make(map[*symbols.Symbol][]string)
	var order []*symbols.Symbol
	for _, note := range notes {
		if _, hasBody := bodyOf(content, note.target.DeclSpan); !hasBody {
			appended = append(appended, note.about)
			continue
		}
		if _, seen := inline[note.target]; !seen {
			order = append(order, note.target)
		}
		inline[note.target] = append(inline[note.target], note.inline)
	}
	for _, target := range order {
		body, _ := bodyOf(content, target.DeclSpan)
		edits = append(edits, insertInBody(content, body, inline[target])...)
	}
	if len(appended) > 0 {
		text := strings.Join(appended, "\n") + "\n"
		if len(content) > 0 && content[len(content)-1] != '\n' {
			text = "\n" + text
		}
		edits = append(edits, quickfix.Insert(len(content), text))
	}
	return edits
}

// insertInBody places texts at the head of a body: on their own lines when the
// members have theirs, before the first member otherwise, alone if none.
func insertInBody(content []byte, body source.Span, texts []string) []quickfix.Edit {
	open, closeAt := body.Offset, body.End()-1
	anchor, ownLine, hasMember := bodyAnchor(content, open, closeAt)
	if !hasMember {
		interior := source.Span{Offset: open + 1, Len: closeAt - open - 1}
		joined := " " + strings.Join(texts, " ") + " "
		if strings.TrimSpace(string(content[open+1:closeAt])) == "" {
			return []quickfix.Edit{quickfix.Replace(interior, joined)}
		}
		return []quickfix.Edit{quickfix.Insert(open+1, strings.TrimRight(joined, " "))}
	}
	edits := make([]quickfix.Edit, 0, len(texts))
	for _, text := range texts {
		if ownLine {
			edits = append(edits, quickfix.InsertLine(anchor, text))
		} else {
			edits = append(edits, quickfix.Insert(anchor, text+" "))
		}
	}
	return edits
}

// bodyAnchor is where a member-bearing body starts: the first token (notes
// included) opening its own line, else the first member on the brace's line.
func bodyAnchor(content []byte, open, closeAt int) (anchor int, ownLine, hasMember bool) {
	first := firstTokenOffset(content, open+1, closeAt)
	if first < 0 {
		return 0, false, false
	}
	lx := lexer.New(source.New("", content[open+1:first]))
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Kind == lexer.Whitespace {
			continue
		}
		if off := open + 1 + tok.Span.Offset; bytes.Contains(content[open+1:off], []byte{'\n'}) {
			return off, true, true
		}
	}
	return first, bytes.Contains(content[open+1:first], []byte{'\n'}), true
}

// bodyOf spans the braces of the body a declaration ends in; hasBody is false
// for one ending in a semicolon or whose body is not closed.
func bodyOf(content []byte, decl source.Span) (body source.Span, hasBody bool) {
	tokens := tokensIn(content, decl)
	if len(tokens) == 0 {
		return source.Span{}, false
	}
	last := tokens[len(tokens)-1]
	if last.Kind != lexer.RBrace {
		return source.Span{Offset: decl.Offset, Len: last.Span.End() - decl.Offset}, false
	}
	depth := 0
	for i := len(tokens) - 1; i >= 0; i-- {
		switch tokens[i].Kind {
		case lexer.RBrace:
			depth++
		case lexer.LBrace:
			depth--
			if depth == 0 {
				open := tokens[i].Span.Offset
				return source.Span{Offset: open, Len: last.Span.End() - open}, true
			}
		}
	}
	return source.Span{}, false
}

// firstTokenOffset is the offset of the first token in [from, to), or -1 when
// only trivia lies there.
func firstTokenOffset(content []byte, from, to int) int {
	tokens := tokensIn(content, source.Span{Offset: from, Len: to - from})
	if len(tokens) == 0 {
		return -1
	}
	return tokens[0].Span.Offset
}

// tokensIn lexes the text of span, trivia dropped, with spans in content offsets.
func tokensIn(content []byte, span source.Span) []lexer.Token {
	end := span.End()
	if span.Offset < 0 || end > len(content) || span.Offset >= end {
		return nil
	}
	lx := lexer.New(source.New("", content[span.Offset:end]))
	var out []lexer.Token
	for {
		tok := lx.Next()
		if tok.Kind == lexer.EOF {
			return out
		}
		if tok.IsTrivia() {
			continue
		}
		tok.Span.Offset += span.Offset
		out = append(out, tok)
	}
}
