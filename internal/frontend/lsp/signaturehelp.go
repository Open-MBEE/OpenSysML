package lsp

import (
	"context"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// SignatureHelp answers the signature of the call the cursor's argument list
// belongs to: one signature per declaration the call may invoke, each listing
// the parameters a positional argument binds in order, with the parameter the
// cursor's argument binds active.
func (s *Server) SignatureHelp(ctx context.Context, params *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.document(name)
	if doc == nil || doc.AST == nil || doc.Scope == nil {
		return nil, nil
	}
	offset := positionToOffset(doc.Content, params.Position)
	call, open := enclosingCall(doc.AST, doc.Content, offset)
	if call == nil {
		return nil, nil
	}
	ref := callReference(collectRefs(doc.AST, doc.Scope), call)
	if ref == nil {
		return nil, nil
	}
	sigs := s.ws.CallSignaturesInDoc(name, *ref)
	if len(sigs) == 0 {
		return nil, nil
	}
	help := &protocol.SignatureHelp{}
	for i, sig := range sigs {
		help.Signatures = append(help.Signatures, signatureInformation(sig))
		if sig.Selected {
			help.ActiveSignature = uint32(i) // #nosec G115 -- a handful of overloads.
		}
	}
	help.ActiveParameter = activeParameter(doc.Content, call, open, offset, sigs[help.ActiveSignature])
	return help, nil
}

// enclosingCall is the innermost invocation whose argument list the cursor is
// in, with the offset of the list's opening parenthesis; nil when the cursor is
// on no argument list, the called name included.
func enclosingCall(root *ast.RootNamespace, content []byte, offset int) (*ast.InvocationExpr, int) {
	var found *ast.InvocationExpr
	foundOpen := -1
	ast.Inspect(root, func(n ast.Node) bool {
		sp := n.Span()
		if offset < sp.Offset || offset > sp.End() {
			return false
		}
		e, ok := n.(*ast.InvocationExpr)
		if !ok || e.Type == nil {
			return true
		}
		open := argumentListStart(content, e.Type.Span().End(), sp.End())
		if open < 0 || offset <= open {
			return true
		}
		if offset == sp.End() && sp.End() > 0 && content[sp.End()-1] == ')' {
			return true // just past the closing parenthesis: outside the list
		}
		if found == nil || sp.Len <= found.Span().Len {
			found, foundOpen = e, open
		}
		return true
	})
	return found, foundOpen
}

// argumentListStart is the offset of the `(` opening an argument list written
// between from and to, -1 when the call has none (`x->f { … }`).
func argumentListStart(content []byte, from, to int) int {
	if to > len(content) {
		to = len(content)
	}
	for i := from; i < to; i++ {
		switch content[i] {
		case ' ', '\t', '\r', '\n':
			continue
		case '(':
			return i
		default:
			return -1
		}
	}
	return -1
}

// callReference is the reference naming what call invokes, nil when the
// document's references list none for it.
func callReference(refs []resolve.Reference, call *ast.InvocationExpr) *resolve.Reference {
	for i := range refs {
		if refs[i].Invocation == call {
			return &refs[i]
		}
	}
	return nil
}

// signatureInformation writes one signature as `Callee(p : T, [q : U]) : R`, a
// bracketed parameter being one with a default the call may leave to it.
func signatureInformation(sig model.CallSignature) protocol.SignatureInformation {
	info := protocol.SignatureInformation{}
	labels := make([]string, 0, len(sig.Params))
	for _, p := range sig.Params {
		label := parameterLabel(p.Name, typeName(p.Type))
		doc := ""
		if p.Default {
			doc = "has a default"
		}
		info.Parameters = append(info.Parameters, protocol.ParameterInformation{Label: label, Documentation: doc})
		if p.Default {
			label = "[" + label + "]"
		}
		labels = append(labels, label)
	}
	var b strings.Builder
	b.WriteString(sig.Callee.Name)
	b.WriteByte('(')
	b.WriteString(strings.Join(labels, ", "))
	b.WriteByte(')')
	if result := typeName(sig.Result); result != "" {
		b.WriteString(" : ")
		b.WriteString(result)
	}
	info.Label = b.String()
	return info
}

// parameterLabel is `name : Type`, or the name alone for an untyped parameter.
func parameterLabel(name, typ string) string {
	if typ == "" {
		return name
	}
	return name + " : " + typ
}

// activeParameter is the index of the parameter the argument under the cursor
// binds: a named argument's by its name, a positional one by its position,
// counting the `->` operand as the first argument.
func activeParameter(content []byte, call *ast.InvocationExpr, open, offset int, sig model.CallSignature) uint32 {
	for _, arg := range call.NamedArgs {
		if arg.Name == nil || len(arg.Name.Parts) == 0 {
			continue
		}
		end := arg.Name.Span().End()
		if arg.Value != nil {
			end = arg.Value.Span().End()
		}
		if offset < arg.Name.Span().Offset || offset > end {
			continue
		}
		name := arg.Name.Parts[len(arg.Name.Parts)-1].Text
		for i, p := range sig.Params {
			if p.Name == name {
				return uint32(i) // #nosec G115 -- a parameter list's length.
			}
		}
	}
	index := topLevelCommas(content, open+1, offset)
	if call.Operand != nil {
		index++
	}
	return uint32(index) // #nosec G115 -- an argument list's length.
}

// topLevelCommas counts the commas between from and to that separate the
// arguments of the list opened just before from, skipping nested lists, bodies,
// strings and comments.
func topLevelCommas(content []byte, from, to int) int {
	if to > len(content) {
		to = len(content)
	}
	depth, count := 0, 0
	for i := from; i < to; i++ {
		switch content[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case '"':
			for i++; i < to && content[i] != '"'; i++ {
				if content[i] == '\\' {
					i++
				}
			}
		case '/':
			if i+1 < to && content[i+1] == '/' {
				for i += 2; i < to && content[i] != '\n'; i++ {
				}
			} else if i+1 < to && content[i+1] == '*' {
				for i += 3; i < to && (content[i-1] != '*' || content[i] != '/'); i++ {
				}
			}
		case ',':
			if depth == 0 {
				count++
			}
		}
	}
	return count
}

// typeName is the name a hint or signature writes a type as, empty for none.
func typeName(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	return sym.Name
}
