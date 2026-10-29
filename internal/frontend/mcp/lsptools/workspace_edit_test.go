package lsptools

import (
	"context"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// A document the server read at a version and left as it was arrives as a
// TextDocumentEdit with no edits; the tool result lists it with none rather
// than dropping it, as the pin is what a client refuses a stale read by.
func TestWorkspaceEditKeepsVersionOnlyDocuments(t *testing.T) {
	f := newFixture(t)
	docURI := uri.File(filepath.Join(f.root, "a.sysml"))
	edit := &protocol.WorkspaceEdit{DocumentChanges: []protocol.DocumentChange{
		&protocol.TextDocumentEdit{TextDocument: protocol.OptionalVersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: docURI},
			Version:                new(int32(1)),
		}},
	}}
	_, res, err := f.host.workspaceEdit(context.Background(), edit, false, "test")
	if err != nil {
		t.Fatalf("workspaceEdit: %v", err)
	}
	if len(res.Changes) != 1 || res.Changes[0].Path != "a.sysml" || len(res.Changes[0].Edits) != 0 {
		t.Fatalf("changes = %+v, want a.sysml listed with no edits", res.Changes)
	}
}
