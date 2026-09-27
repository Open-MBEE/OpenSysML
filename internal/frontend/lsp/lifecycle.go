package lsp

import (
	"context"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// Initialize advertises the server capabilities this plan implements and records
// the session's folders, scanned later so the handshake is not delayed.
func (s *Server) Initialize(ctx context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	s.setFolders(initializeFolders(params))
	if params != nil {
		s.applyConformanceSettings(params.InitializationOptions)
		s.setHoverMarkdown(clientRendersMarkdownHover(params.Capabilities))
		s.setCompletionMarkdown(clientRendersMarkdownCompletion(params.Capabilities))
		s.setCrossDocument(clientAdvertisesCrossDocument(params.Capabilities))
	}
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    protocol.TextDocumentSyncKindIncremental,
				// Without this a client sends no didSave, so the save-time
				// cross-document refresh would never run.
				Save: &protocol.SaveOptions{IncludeText: false},
			},
			HoverProvider:           true,
			DefinitionProvider:      true,
			ReferencesProvider:      true,
			DocumentSymbolProvider:  true,
			WorkspaceSymbolProvider: true,
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{":", "."},
			},
			DocumentFormattingProvider:      true,
			DocumentRangeFormattingProvider: true,
			RenameProvider:                  &protocol.RenameOptions{PrepareProvider: true},
			SemanticTokensProvider: &semanticTokensProvider{
				Legend: semanticTokensLegend(),
				Full:   true,
				Range:  true,
			},
			CodeActionProvider: &protocol.CodeActionOptions{
				CodeActionKinds: []protocol.CodeActionKind{protocol.QuickFix, identityActionKind},
			},
			// A client that draws diagrams speaks opensysml/render, which is no
			// protocol method; this is how it learns the server serves it.
			Experimental: map[string]any{
				"openSysmlRender":         true,
				"openSysmlRenderDocument": true,
				"openSysmlApplyModelEdit": true,
				"openSysmlStdlibContent":  true,
				"openSysmlDebug":          true,
				CrossDocumentCapability:   true,
				RenderPaletteCapability:   true,
				RenderFormsCapability:     renderFormNames(),
				RenderStylesCapability:    renderStyleNames(),
			},
			// Folders added mid-session are only indexed if the client reports them.
			Workspace: &protocol.ServerCapabilitiesWorkspace{
				WorkspaceFolders: &protocol.ServerCapabilitiesWorkspaceFolders{
					Supported:           true,
					ChangeNotifications: true,
				},
			},
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    "sysml-lsp",
			Version: "0.1.0",
		},
	}, nil
}

// clientRendersMarkdownHover reports whether the client advertised Markdown
// among the content formats it accepts for hover.
func clientRendersMarkdownHover(caps protocol.ClientCapabilities) bool {
	if caps.TextDocument == nil || caps.TextDocument.Hover == nil {
		return false
	}
	for _, format := range caps.TextDocument.Hover.ContentFormat {
		if format == protocol.Markdown {
			return true
		}
	}
	return false
}

// clientRendersMarkdownCompletion reports whether the client advertised Markdown
// among the formats it accepts for completion item documentation.
func clientRendersMarkdownCompletion(caps protocol.ClientCapabilities) bool {
	if caps.TextDocument == nil || caps.TextDocument.Completion == nil || caps.TextDocument.Completion.CompletionItem == nil {
		return false
	}
	for _, format := range caps.TextDocument.Completion.CompletionItem.DocumentationFormat {
		if format == protocol.Markdown {
			return true
		}
	}
	return false
}

// clientAdvertisesCrossDocument reports whether the client listed the
// cross-document diagram contract among its experimental capabilities.
func clientAdvertisesCrossDocument(caps protocol.ClientCapabilities) bool {
	experimental, ok := caps.Experimental.(map[string]any)
	return ok && experimental[CrossDocumentCapability] == true
}

// Initialized indexes the session's folders, so cross-file names resolve without
// the editor having opened every file.
func (s *Server) Initialized(ctx context.Context, params *protocol.InitializedParams) error {
	s.loadFolders(ctx)
	return nil
}

// Shutdown prepares the server for exit: the session stays readable, but only
// the exit notification is still answered (LSP 3.17 §Shutdown Request). Live
// debug sessions end with it, releasing what they hold.
func (s *Server) Shutdown(ctx context.Context) error {
	s.markShutdown()
	s.debugSessionsEnded(ctx, "the server is shutting down")
	return nil
}

// markShutdown records that the client asked for a shutdown.
func (s *Server) markShutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shutdownReceived = true
}

// Exit ends the session. It records the status the process owes its client and
// releases Run, which owns the stream, rather than closing it from under the
// read loop that dispatched this notification.
func (s *Server) Exit(ctx context.Context) error {
	s.mu.Lock()
	s.exitReceived = true
	s.mu.Unlock()
	s.exitOnce.Do(func() { close(s.exited) })
	return nil
}

// ExitCode is the process status LSP 3.17 asks of a served session: 0 when exit
// followed a shutdown request, 1 when exit arrived without one. A session the
// client ended by closing the stream instead served to its end, so it is 0.
func (s *Server) ExitCode() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.exitReceived && !s.shutdownReceived {
		return 1
	}
	return 0
}

// shutdownRequested reports whether a shutdown request has been served.
func (s *Server) shutdownRequested() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shutdownReceived
}

// lifecycleHandler enforces the shutdown/exit half of the lifecycle. It runs on
// the read loop, ahead of the asynchronous dispatch, so that what arrives after
// a shutdown is judged against the state the client saw when it sent it: a
// request is answered InvalidRequest, a notification other than exit is dropped,
// and exit ends the session.
func (s *Server) lifecycleHandler(inner jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		switch req.Method() {
		case protocol.MethodExit:
			return reply(ctx, nil, s.Exit(ctx))
		case protocol.MethodShutdown:
			// Recorded here, not in the handler: the asynchronous dispatch could
			// otherwise run after the next message has been judged.
			s.markShutdown()
			return inner(ctx, reply, req)
		}
		if !s.shutdownRequested() {
			return inner(ctx, reply, req)
		}
		if _, isRequest := req.(*jsonrpc2.Call); isRequest {
			return reply(ctx, nil, jsonrpc2.ErrInvalidRequest)
		}
		return reply(ctx, nil, nil)
	}
}
