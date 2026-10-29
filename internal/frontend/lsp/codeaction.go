package lsp

import (
	"context"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// CodeAction answers the code actions for a range: the quick fixes attached to
// its diagnostics and the opt-in identity rewrites of the declaration under it.
func (s *Server) CodeAction(ctx context.Context, params *protocol.CodeActionParams) ([]protocol.CommandOrCodeAction, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.ws.Document(name)
	if doc == nil {
		return nil, nil
	}
	want := rangeToSpan(doc.Content, params.Range)
	var actions []protocol.CodeAction
	if wantsKind(params.Context.Only, protocol.CodeActionKindQuickFix) {
		actions = append(actions, s.quickFixes(name, doc, want)...)
	}
	if wantsKind(params.Context.Only, identityActionKind) {
		identity, err := s.identityActions(name, doc, want)
		if err != nil {
			return nil, err
		}
		actions = append(actions, identity...)
	}
	out := make([]protocol.CommandOrCodeAction, len(actions))
	for i := range actions {
		out[i] = &actions[i]
	}
	return out, nil
}

// quickFixes returns the fixes attached to the diagnostics overlapping want.
func (s *Server) quickFixes(name string, doc *model.Document, want source.Span) []protocol.CodeAction {
	uri := nameToURI(name)
	pos := positionsOf(doc)
	var out []protocol.CodeAction
	for _, diag := range s.ws.Diagnostics(name) {
		if len(diag.Fixes) == 0 || !overlaps(diag.Span, want) {
			continue
		}
		reported := protocolDiagnostic(pos, diag)
		for _, fix := range diag.Fixes {
			action := protocol.CodeAction{
				Title:       fix.Title,
				Kind:        new(protocol.CodeActionKindQuickFix),
				Diagnostics: []protocol.Diagnostic{reported},
				Edit:        workspaceEdit(uri, pos, fix.Edits),
			}
			if fix.Preferred {
				action.IsPreferred = new(true)
			}
			out = append(out, action)
		}
	}
	return out
}

// wantsKind reports whether a kind filter asks for kind, prefixes included
// (`refactor` asks for `refactor.rewrite`); no filter asks for everything.
func wantsKind(only []protocol.CodeActionKind, kind protocol.CodeActionKind) bool {
	if len(only) == 0 {
		return true
	}
	for _, want := range only {
		if want == "" || want == kind || strings.HasPrefix(string(kind), string(want)+".") {
			return true
		}
	}
	return false
}

// overlaps reports whether two spans share any position, an empty span counting
// where it sits: a cursor on a diagnostic asks for its fixes.
func overlaps(a, b source.Span) bool {
	if a.Len == 0 || b.Len == 0 {
		return a.Offset <= b.End() && b.Offset <= a.End()
	}
	return a.Offset < b.End() && b.Offset < a.End()
}

// workspaceEdit renders a fix's edits as an edit of the document it applies to.
func workspaceEdit(docURI uri.URI, pos positions, edits []diag.Edit) *protocol.WorkspaceEdit {
	out := make([]protocol.TextEdit, 0, len(edits))
	for _, edit := range edits {
		span, text := edit.Render(pos.content)
		out = append(out, protocol.TextEdit{
			Range:   pos.rangeOf(span),
			NewText: text,
		})
	}
	return &protocol.WorkspaceEdit{
		Changes: map[uri.URI][]protocol.TextEdit{docURI: out},
	}
}
