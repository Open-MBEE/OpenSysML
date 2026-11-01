package lsp

import (
	"context"

	"github.com/Open-MBEE/OpenSysML/internal/check/codes"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// publishDiagnostics analyzes the named document and pushes diagnostics to the
// client, withdrawing them for a document the workspace no longer holds. It is a
// no-op when no client is connected.
func (s *Server) publishDiagnostics(ctx context.Context, name string) {
	if s.client == nil {
		return
	}
	// Analysis and send happen together: the coalesced sweep publishes from a
	// timer goroutine, so interleaving them would let an older set land last on
	// a client that cannot tell diagnostics apart by version.
	s.pubMu.Lock()
	defer s.pubMu.Unlock()
	s.sendDiagnosticsLocked(ctx, name)
}

// publishOpenDiagnostics publishes a document's diagnostics only while it is
// still open, tested under the same lock the withdrawal takes, so a didClose
// racing a sweep stays withdrawn.
func (s *Server) publishOpenDiagnostics(ctx context.Context, name string) {
	if s.client == nil {
		return
	}
	s.pubMu.Lock()
	defer s.pubMu.Unlock()
	if !s.ws.IsOpen(name) {
		return
	}
	s.sendDiagnosticsLocked(ctx, name)
}

// sendDiagnosticsLocked analyzes name and pushes the result, then tells a
// diagram client that the renderings of the document are stale. Caller holds
// pubMu, so a client sees the diagnostics of an analysis before the render
// notification of the same one.
func (s *Server) sendDiagnosticsLocked(ctx context.Context, name string) {
	if nb := s.notebooks.named(name); nb != nil {
		s.sendNotebookDiagnosticsLocked(ctx, nb)
		return
	}
	out := []protocol.Diagnostic{}
	if content, diags, ok := s.ws.AnalyzedContent(name); ok {
		for _, f := range s.ws.FindingsOn(name) {
			diags = append(diags, f.Diagnostic)
		}
		out = protocolDiagnostics(content, diags)
	}
	// Best-effort push: a failed notification has no recovery path here and the
	// server currently threads no logger, so the error is intentionally dropped.
	_ = s.client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{
		URI:         nameToURI(name),
		Diagnostics: out,
	})
	s.queueRenderChanged(ctx, name)
}

// protocolDiagnostics converts diagnostics located in content to the protocol's.
func protocolDiagnostics(content []byte, diags []diag.Diagnostic) []protocol.Diagnostic {
	out := make([]protocol.Diagnostic, 0, len(diags))
	pos := positionsFor(content)
	for _, d := range diags {
		out = append(out, protocolDiagnostic(pos, d))
	}
	return out
}

// protocolDiagnostic is one diagnostic located in the content pos indexes, its
// code linking the catalogue entry that explains it.
func protocolDiagnostic(pos positions, d diag.Diagnostic) protocol.Diagnostic {
	ld := protocol.Diagnostic{
		Range:    pos.rangeOf(d.Span),
		Severity: protocol.DiagnosticSeverity(int(d.Severity) + 1), // #nosec G115 -- four severities
		Message:  protocol.String(d.Message),
	}
	if d.Code != "" {
		ld.Code = protocol.String(d.Code)
		if href, err := uri.Parse(codes.URL(d.Code)); err == nil {
			ld.CodeDescription = protocol.CodeDescription{Href: href}
		}
	}
	if d.Source != "" {
		ld.Source = protocol.NewOptional(d.Source)
	}
	return ld
}

// publishFindings publishes the workspace findings reported in documents the
// editor has not open — a file the folder scan indexed, a library file — so a
// library package two documents declare is in the problems list whichever is
// open, and withdraws the ones published last time that no longer hold. A
// publication replaces a URI's whole set, so a document's findings go out
// together. A finding in an open document goes out with that document's diagnostics.
func (s *Server) publishFindings(ctx context.Context) {
	if s.client == nil {
		return
	}
	s.pubMu.Lock()
	defer s.pubMu.Unlock()
	sets := map[uri.URI][]protocol.Diagnostic{}
	var order []uri.URI
	for _, f := range s.ws.WorkspaceFindings() {
		if s.ws.IsOpen(f.Document) {
			continue
		}
		var content []byte
		target := nameToURI(f.Document)
		if f.Library {
			target = libraryURI(f.Document)
			if doc := s.ws.LibraryDocument(f.Document); doc != nil {
				content = doc.Content
			}
		} else if doc := s.ws.Document(f.Document); doc != nil {
			content = doc.Content
		}
		if _, seen := sets[target]; !seen {
			order = append(order, target)
		}
		sets[target] = append(sets[target], protocolDiagnostics(content, []diag.Diagnostic{f.Diagnostic})...)
	}
	for _, target := range order {
		_ = s.client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{URI: target, Diagnostics: sets[target]})
	}
	for target := range s.findingURIs {
		if _, still := sets[target]; !still {
			_ = s.client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{URI: target, Diagnostics: []protocol.Diagnostic{}})
		}
	}
	current := make(map[uri.URI]bool, len(sets))
	for target := range sets {
		current[target] = true
	}
	s.findingURIs = current
}

// clearDiagnostics withdraws the diagnostics published for a document, so
// markers do not outlive the buffer or the file they came from.
func (s *Server) clearDiagnostics(ctx context.Context, name string) {
	if s.client == nil {
		return
	}
	s.pubMu.Lock()
	defer s.pubMu.Unlock()
	_ = s.client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{
		URI:         nameToURI(name),
		Diagnostics: []protocol.Diagnostic{},
	})
}
