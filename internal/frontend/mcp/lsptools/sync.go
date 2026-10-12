package lsptools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// OpenDocumentInput names a document and the text to hold open for it.
type OpenDocumentInput struct {
	Path string `json:"path" jsonschema:"the document, absolute or relative to the workspace root; it need not exist on disk yet"`
	Text string `json:"text" jsonschema:"the full text to hold as the document's content"`
}

// UpdateDocumentInput replaces the text of an open document.
type UpdateDocumentInput struct {
	Path string `json:"path" jsonschema:"an open document, absolute or relative to the workspace root"`
	Text string `json:"text" jsonschema:"the full new text of the document"`
}

// CloseDocumentInput names the open document to release.
type CloseDocumentInput struct {
	Path string `json:"path" jsonschema:"an open document, absolute or relative to the workspace root"`
}

// DocumentState reports an open document's version and diagnostics after a
// change.
type DocumentState struct {
	Path        string       `json:"path"`
	Version     int          `json:"version" jsonschema:"the buffer's version, incremented by each update"`
	Open        bool         `json:"open" jsonschema:"whether the server now holds an open buffer for the document"`
	Diagnostics []Diagnostic `json:"diagnostics" jsonschema:"the diagnostics of the text now held"`
}

// ReloadInput optionally restricts a reload to named documents.
type ReloadInput struct {
	Paths []string `json:"paths,omitempty" jsonschema:"documents to re-read from disk; omitted, the whole root is re-scanned"`
}

// ReloadResult lists what a reload found changed.
type ReloadResult struct {
	Documents int      `json:"documents" jsonschema:"model documents indexed after the reload"`
	Added     []string `json:"added,omitempty" jsonschema:"documents found on disk that were not indexed before"`
	Changed   []string `json:"changed,omitempty" jsonschema:"documents re-read because their file changed"`
	Removed   []string `json:"removed,omitempty" jsonschema:"documents dropped because their file is gone"`
}

func registerSync(s *mcp.Server, h *Host) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "open_document",
		Description: "Hold a document open with the given text, as an editor buffer the server analyzes instead of the file on disk, " +
			"so text can be checked (diagnostics, hover, render) before it is written. Nothing is written to disk; close_document reverts to the file. " +
			"Returns the diagnostics of the text.",
		Annotations: toolkit.Mutating("Open document", true),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in OpenDocumentInput) (*mcp.CallToolResult, DocumentState, error) {
		ref, err := h.resolve(in.Path)
		if err != nil || ref.library {
			return nil, DocumentState{}, orLibrary(err, "a standard library document cannot be opened for editing")
		}
		err = h.Server.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{
			URI: ref.uri(), LanguageID: "sysml", Version: 1, Text: in.Text,
		}})
		if err != nil {
			return nil, DocumentState{}, err
		}
		return h.state(ref)
	})
	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_document",
		Description: "Replace the text of a document held open by open_document and re-analyze it. Nothing is written to disk. Returns the diagnostics of the new text.",
		Annotations: toolkit.Mutating("Update document", true),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateDocumentInput) (*mcp.CallToolResult, DocumentState, error) {
		ref, err := h.resolve(in.Path)
		if err != nil || ref.library {
			return nil, DocumentState{}, orLibrary(err, "a standard library document cannot be edited")
		}
		if !slices(h.Workspace.OpenNames(), ref.name) {
			return nil, DocumentState{}, fmt.Errorf("%s is not open; call open_document first", h.display(ref.uri()))
		}
		if err := h.replace(ctx, ref.name, in.Text); err != nil {
			return nil, DocumentState{}, err
		}
		return h.state(ref)
	})
	mcp.AddTool(s, &mcp.Tool{
		Name:        "close_document",
		Description: "Release a document held open by open_document: the server goes back to the file on disk (or forgets the document when there is no file). Nothing is written.",
		Annotations: toolkit.Mutating("Close document", true),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in CloseDocumentInput) (*mcp.CallToolResult, DocumentState, error) {
		ref, err := h.resolve(in.Path)
		if err != nil || ref.library {
			return nil, DocumentState{}, orLibrary(err, "a standard library document is never open")
		}
		if !slices(h.Workspace.OpenNames(), ref.name) {
			return nil, DocumentState{}, fmt.Errorf("%s is not open", h.display(ref.uri()))
		}
		if err := h.Server.DidClose(ctx, &protocol.DidCloseTextDocumentParams{TextDocument: protocol.TextDocumentIdentifier{URI: ref.uri()}}); err != nil {
			return nil, DocumentState{}, err
		}
		h.watched(ctx, ref.name, protocol.FileChangeTypeChanged)
		state := DocumentState{Path: h.display(ref.uri()), Diagnostics: []Diagnostic{}}
		if doc := h.Workspace.Document(ref.name); doc != nil {
			state.Version = doc.Version
			state.Diagnostics = h.diagnosticsOf(ref)
		}
		return text(renderDiagnostics(state.Diagnostics, state.Path+" closed")), state, nil
	})
	mcp.AddTool(s, &mcp.Tool{
		Name: "reload",
		Description: "Re-read model files from disk after they were changed with other tools: re-scans the root for new, changed and deleted .sysml/.kerml files, " +
			"or re-reads only the paths given. Every tool also re-reads its own document when the file changed, so this is needed mainly after creating or deleting files. " +
			"Documents held open by open_document are left as they are.",
		Annotations: toolkit.Mutating("Reload workspace", true),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ReloadInput) (*mcp.CallToolResult, ReloadResult, error) {
		if len(in.Paths) == 0 {
			res, err := h.rescan(ctx)
			if err != nil {
				return nil, ReloadResult{}, err
			}
			return text(fmt.Sprintf("%d documents indexed: %d added, %d changed, %d removed", res.Documents, len(res.Added), len(res.Changed), len(res.Removed))), res, nil
		}
		var res ReloadResult
		for _, path := range in.Paths {
			ref, err := h.resolve(path)
			if err != nil || ref.library {
				return nil, ReloadResult{}, orLibrary(err, "a standard library document is not read from disk")
			}
			switch h.refresh(ctx, ref.name) {
			case removed:
				res.Removed = append(res.Removed, h.display(ref.uri()))
			case added:
				res.Added = append(res.Added, h.display(ref.uri()))
			case changed:
				res.Changed = append(res.Changed, h.display(ref.uri()))
			}
		}
		for _, name := range h.Workspace.DocumentNames() {
			if !h.Workspace.IsLibraryDocument(name) {
				res.Documents++
			}
		}
		return text(fmt.Sprintf("%d documents indexed: %d added, %d re-read, %d removed", res.Documents, len(res.Added), len(res.Changed), len(res.Removed))), res, nil
	})
}

// orLibrary is err when set, else an error with the library message.
func orLibrary(err error, msg string) error {
	if err != nil {
		return err
	}
	return fmt.Errorf("%s", msg)
}

// state reports an open document's version and current diagnostics.
func (h *Host) state(ref docRef) (*mcp.CallToolResult, DocumentState, error) {
	doc := h.Workspace.Document(ref.name)
	if doc == nil {
		return nil, DocumentState{}, fmt.Errorf("%s was not indexed", h.display(ref.uri()))
	}
	state := DocumentState{Path: h.display(ref.uri()), Version: doc.Version, Open: true, Diagnostics: h.diagnosticsOf(ref)}
	return text(renderDiagnostics(state.Diagnostics, state.Path+" (version "+fmt.Sprint(doc.Version)+")")), state, nil
}
