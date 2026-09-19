package lsp

import (
	"context"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// CodeAction answers the code actions for a range: the quick fixes attached to
// its diagnostics and the opt-in identity rewrites of the declaration under it.
func (s *Server) CodeAction(ctx context.Context, params *protocol.CodeActionParams) ([]protocol.CodeAction, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.ws.Document(name)
	if doc == nil {
		return nil, nil
	}
	want := rangeToSpan(doc.Content, params.Range)
	out := []protocol.CodeAction{}
	if wantsKind(params.Context.Only, protocol.QuickFix) {
		out = append(out, s.quickFixes(name, doc, want)...)
	}
	if wantsKind(params.Context.Only, identityActionKind) {
		actions, err := s.identityActions(name, doc, want)
		if err != nil {
			return nil, err
		}
		out = append(out, actions...)
	}
	return out, nil
}

// quickFixes returns the fixes attached to the diagnostics overlapping want.
func (s *Server) quickFixes(name string, doc *model.Document, want source.Span) []protocol.CodeAction {
	uri := nameToURI(name)
	var out []protocol.CodeAction
	for _, diag := range s.ws.Diagnostics(name) {
		if len(diag.Fixes) == 0 || !overlaps(diag.Span, want) {
			continue
		}
		reported := protocol.Diagnostic{
			Range:    spanToRange(doc.Content, diag.Span),
			Severity: protocol.DiagnosticSeverity(int(diag.Severity) + 1),
			Message:  diag.Message,
			Code:     diag.Code,
			Source:   diag.Source,
		}
		for _, fix := range diag.Fixes {
			out = append(out, protocol.CodeAction{
				Title:       fix.Title,
				Kind:        protocol.QuickFix,
				Diagnostics: []protocol.Diagnostic{reported},
				IsPreferred: fix.Preferred,
				Edit:        workspaceEdit(uri, doc.Content, fix.Edits),
			})
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
func workspaceEdit(uri protocol.DocumentURI, content []byte, edits []diag.Edit) *protocol.WorkspaceEdit {
	out := make([]protocol.TextEdit, 0, len(edits))
	for _, edit := range edits {
		span, text := edit.Render(content)
		out = append(out, protocol.TextEdit{
			Range:   spanToRange(content, span),
			NewText: text,
		})
	}
	return &protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentURI][]protocol.TextEdit{uri: out},
	}
}
