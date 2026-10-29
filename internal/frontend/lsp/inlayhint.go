package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// InlayHint answers the hints of a range: the inferred type of each feature
// declared without one, and the constant each non-literal feature value
// evaluates to from the model alone. Only the declarations in the range are
// read, so hints cost nothing until an editor asks for them.
func (s *Server) InlayHint(ctx context.Context, params *protocol.InlayHintParams) ([]protocol.InlayHint, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.document(name)
	if doc == nil || doc.Scope == nil {
		return nil, nil
	}
	want := rangeToSpan(doc.Content, params.Range)
	syms := usagesIn(doc.Scope, want)
	if len(syms) == 0 {
		return []protocol.InlayHint{}, nil
	}
	pos := positionsOf(doc)
	out := []protocol.InlayHint{}
	// A declaration spanning several lines may put a hint outside the range.
	add := func(hint protocol.InlayHint) {
		if inRange(hint.Position, params.Range) {
			out = append(out, hint)
		}
	}
	for _, hint := range s.ws.FeatureHintsInDoc(name, syms) {
		usage := hint.Symbol.Decl.(*ast.Usage)
		if hint.Type != nil {
			if at, ok := typeHintOffset(hint.Symbol, usage); ok {
				add(protocol.InlayHint{
					Position:    pos.position(at),
					Label:       protocol.String(": " + typeName(hint.Type)),
					Kind:        protocol.InlayHintKindType,
					Tooltip:     protocol.String("inferred type " + s.ws.FQNOf(hint.Type)),
					PaddingLeft: new(true),
				})
			}
		}
		if text, ok := constantText(hint.Value); hint.HasValue && ok {
			add(protocol.InlayHint{
				Position:    pos.position(usage.Value.Span().End()),
				Label:       protocol.String("= " + text),
				Tooltip:     protocol.String("value evaluated from the model"),
				PaddingLeft: new(true),
			})
		}
	}
	return out, nil
}

// inRange reports whether p lies in r, which like every LSP range includes its
// start and excludes its end.
func inRange(p protocol.Position, r protocol.Range) bool {
	less := func(a, b protocol.Position) bool {
		return a.Line < b.Line || (a.Line == b.Line && a.Character < b.Character)
	}
	return !less(p, r.Start) && less(p, r.End)
}

// usagesIn lists the usages declared under scope whose declaration overlaps
// want, in declaration order.
func usagesIn(scope *symbols.Scope, want source.Span) []*symbols.Symbol {
	var out []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{}
	var visit func(scope *symbols.Scope)
	visit = func(scope *symbols.Scope) {
		for _, sym := range scope.Members() {
			if seen[sym] || !overlaps(sym.DeclSpan, want) {
				continue
			}
			seen[sym] = true
			if _, ok := sym.Decl.(*ast.Usage); ok {
				out = append(out, sym)
			}
		}
		for _, child := range scope.Children() {
			if node := child.Node(); node != nil && !overlaps(node.Span(), want) {
				continue
			}
			visit(child)
		}
	}
	visit(scope)
	return out
}

// typeHintOffset is where a type hint for the usage goes: after its declared
// name, else after the redefinition or subsetting it takes its name from.
func typeHintOffset(sym *symbols.Symbol, usage *ast.Usage) (int, bool) {
	if usage.Ident.Name != "" && usage.Ident.NameSpan.Len > 0 {
		return usage.Ident.NameSpan.End(), true
	}
	if usage.Ident.ShortName != "" && usage.Ident.ShortNameSpan.Len > 0 {
		return usage.Ident.ShortNameSpan.End(), true
	}
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Target == nil {
			continue
		}
		if rel.Kind == ast.RelRedefines || rel.Kind == ast.RelSubsets {
			return rel.Target.Span().End(), true
		}
	}
	return 0, false
}

// constantText writes a model-level constant as notation.
func constantText(v semantics.Value) (string, bool) {
	switch v.Kind {
	case semantics.ValInt:
		return v.FormatInt(), true
	case semantics.ValReal:
		return semantics.FormatReal(v.Real), true
	case semantics.ValRational:
		return v.FormatRational(), true
	case semantics.ValBool:
		if v.Bool {
			return "true", true
		}
		return "false", true
	case semantics.ValInfinity:
		return "*", true
	}
	return "", false
}
