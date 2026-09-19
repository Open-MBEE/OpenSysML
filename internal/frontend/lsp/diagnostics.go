package lsp

import (
	"context"

	"go.lsp.dev/protocol"
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
	out := []protocol.Diagnostic{}
	if content, diags, ok := s.ws.AnalyzedContent(name); ok {
		out = make([]protocol.Diagnostic, 0, len(diags))
		for _, d := range diags {
			out = append(out, protocol.Diagnostic{
				Range:    spanToRange(content, d.Span),
				Severity: protocol.DiagnosticSeverity(int(d.Severity) + 1),
				Message:  d.Message,
				Code:     d.Code,
				Source:   d.Source,
			})
		}
	}
	// Best-effort push: a failed notification has no recovery path here and the
	// server currently threads no logger, so the error is intentionally dropped.
	_ = s.client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{
		URI:         nameToURI(name),
		Diagnostics: out,
	})
	s.queueRenderChanged(ctx, name)
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
