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

func registerSync(s *mcp.Server, h *Host) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "open_document",
		Description: "Hold a document open with the given text, as an editor buffer the server analyzes instead of the file on disk, " +
			"so text can be checked (diagnostics, hover, render) before it is written. Nothing is written to disk; close_document reverts to the file. " +
			"format, rename and apply_model_edit with apply=true write the buffer, unless the file changed on disk since it was opened. " +
			"Returns the diagnostics of the text.",
		Annotations: toolkit.Mutating("Open document", true),
	}, writing(h, func(ctx context.Context, _ *mcp.CallToolRequest, in OpenDocumentInput) (*mcp.CallToolResult, DocumentState, error) {
		ref, err := h.resolve(in.Path)
		if err != nil || ref.library {
			return nil, DocumentState{}, orLibrary(err, "a standard library document cannot be opened for editing")
		}
		disk, err := readDisk(ref.name)
		if err != nil {
			return nil, DocumentState{}, err
		}
		// Reopening keeps the snapshot taken when the buffer began, so an edit
		// made on disk meanwhile is not silently accepted as the baseline.
		if seen, open := h.opened[ref.name]; open {
			if !disk.same(seen) {
				return nil, DocumentState{}, fmt.Errorf("%s changed on disk while it was held open; close_document and open it again with the text to keep", h.display(ref.uri()))
			}
			disk = seen
		}
		err = h.Server.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{
			URI: ref.uri(), LanguageID: "sysml", Version: 1, Text: in.Text,
		}})
		if err != nil {
			return nil, DocumentState{}, err
		}
		h.opened[ref.name] = disk
		return h.state(ref)
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_document",
		Description: "Replace the text of a document held open by open_document and re-analyze it. Nothing is written to disk. Returns the diagnostics of the new text.",
		Annotations: toolkit.Mutating("Update document", true),
	}, writing(h, func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateDocumentInput) (*mcp.CallToolResult, DocumentState, error) {
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
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "close_document",
		Description: "Release a document held open by open_document: the server goes back to the file on disk (or forgets the document when there is no file). Nothing is written.",
		Annotations: toolkit.Mutating("Close document", true),
	}, writing(h, func(ctx context.Context, _ *mcp.CallToolRequest, in CloseDocumentInput) (*mcp.CallToolResult, DocumentState, error) {
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
		delete(h.opened, ref.name)
		h.Refresh(ctx)
		state := DocumentState{Path: h.display(ref.uri()), Diagnostics: []Diagnostic{}}
		if doc := h.Workspace.Document(ref.name); doc != nil {
			state.Version = doc.Version
			state.Diagnostics = h.diagnosticsOf(ref)
		}
		return text(renderDiagnostics(state.Diagnostics, state.Path+" closed")), state, nil
	}))
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
