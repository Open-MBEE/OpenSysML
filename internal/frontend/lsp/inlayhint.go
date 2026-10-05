package lsp

import (
	"context"
	"encoding/json"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// methodInlayHint is textDocument/inlayHint (LSP 3.17), which the protocol
// library predates; its shapes are declared here.
const methodInlayHint = "textDocument/inlayHint"

// inlayHintParams asks for the hints of one range of a document.
type inlayHintParams struct {
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
	Range        protocol.Range                  `json:"range"`
}

// inlayHint is text the editor shows inline at a position without it being in
// the document: a type after a name declared without one, a value after an
// expression evaluated from the model alone.
type inlayHint struct {
	Position     protocol.Position `json:"position"`
	Label        string            `json:"label"`
	Kind         int               `json:"kind,omitempty"`
	Tooltip      string            `json:"tooltip,omitempty"`
	PaddingLeft  bool              `json:"paddingLeft,omitempty"`
	PaddingRight bool              `json:"paddingRight,omitempty"`
}

// inlayHintKindType is the InlayHintKind of a type annotation.
const inlayHintKindType = 1

// inlayHintProvider is the capability advertised for the method.
var inlayHintProvider = map[string]any{"resolveProvider": false}

// InlayHint answers the hints of a range: the inferred type of each feature
// declared without one, and the constant each non-literal feature value
// evaluates to from the model alone. Only the declarations in the range are
// read, so hints cost nothing until an editor asks for them.
func (s *Server) InlayHint(params *inlayHintParams) ([]inlayHint, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.document(name)
	if doc == nil || doc.Scope == nil {
		return nil, nil
	}
	want := rangeToSpan(doc.Content, params.Range)
	syms := usagesIn(doc.Scope, want)
	if len(syms) == 0 {
		return []inlayHint{}, nil
	}
	pos := positionsOf(doc)
	out := []inlayHint{}
	// A declaration spanning several lines may put a hint outside the range.
	add := func(hint inlayHint) {
		if inRange(hint.Position, params.Range) {
			out = append(out, hint)
		}
	}
	for _, hint := range s.ws.FeatureHintsInDoc(name, syms) {
		usage := hint.Symbol.Decl.(*ast.Usage)
		if hint.Type != nil {
			if at, ok := typeHintOffset(hint.Symbol, usage); ok {
				add(inlayHint{
					Position:    pos.position(at),
					Label:       ": " + typeName(hint.Type),
					Kind:        inlayHintKindType,
					Tooltip:     "inferred type " + s.ws.FQNOf(hint.Type),
					PaddingLeft: true,
				})
			}
		}
		if text, ok := constantText(hint.Value); hint.HasValue && ok {
			add(inlayHint{
				Position:    pos.position(usage.Value.Span().End()),
				Label:       "= " + text,
				Tooltip:     "value evaluated from the model",
				PaddingLeft: true,
			})
		}
	}
	return out, nil
}

// inRange reports whether p lies in r, its end included: an editor asking for
// the lines on screen is owed the hint at the end of the last one.
func inRange(p protocol.Position, r protocol.Range) bool {
	before := func(a, b protocol.Position) bool {
		return a.Line < b.Line || (a.Line == b.Line && a.Character <= b.Character)
	}
	return before(r.Start, p) && before(p, r.End)
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

// inlayHintHandler serves textDocument/inlayHint, which the protocol library
// does not dispatch, and adds the method's capability to the initialize result,
// which the library's capability type has no field for.
func (s *Server) inlayHintHandler(inner jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		switch req.Method() {
		case methodInlayHint:
			var params inlayHintParams
			if err := json.Unmarshal(req.Params(), &params); err != nil {
				return reply(ctx, nil, err)
			}
			hints, err := s.InlayHint(&params)
			return reply(ctx, hints, err)
		case protocol.MethodInitialize:
			var params protocol.InitializeParams
			if err := json.Unmarshal(req.Params(), &params); err != nil {
				return reply(ctx, nil, err)
			}
			result, err := s.Initialize(ctx, &params)
			if err != nil {
				return reply(ctx, nil, err)
			}
			full, err := withInlayHintCapability(result)
			return reply(ctx, full, err)
		}
		return inner(ctx, reply, req)
	}
}

// withInlayHintCapability is the initialize result with inlayHintProvider set.
func withInlayHintCapability(result *protocol.InitializeResult) (map[string]any, error) {
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	caps, _ := out["capabilities"].(map[string]any)
	if caps == nil {
		caps = map[string]any{}
	}
	caps["inlayHintProvider"] = inlayHintProvider
	out["capabilities"] = caps
	return out, nil
}
