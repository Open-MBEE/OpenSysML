package lsp

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// The custom methods a document-preview client speaks, alongside the diagram
// methods in render.go.
const (
	// MethodDocuments lists the document definitions the workspace holds.
	MethodDocuments = "opensysml/documents"
	// MethodRenderDocument renders a named document definition as Markdown.
	MethodRenderDocument = "opensysml/renderDocument"
)

// documentsResult lists the document definitions of the workspace, each by
// qualified name and the file declaring it.
type documentsResult struct {
	Documents []documentInfo `json:"documents"`
}

// documentInfo is one document definition a client may ask to render.
type documentInfo struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
}

// renderDocumentParams asks for the Markdown rendering of the document
// definition Name names. DiagramForm is the source its graph-shaped diagrams
// are written as, mermaid or dot; empty is mermaid.
type renderDocumentParams struct {
	Name        string `json:"name"`
	DiagramForm string `json:"diagramForm,omitempty"`
}

// renderDocumentResult is the rendered document.
type renderDocumentResult struct {
	Name     string `json:"name"`
	Markdown string `json:"markdown"`
}

// Documents answers opensysml/documents: the document definitions declared
// across the workspace, in qualified-name order.
func (s *Server) Documents() *documentsResult {
	out := &documentsResult{Documents: []documentInfo{}}
	for _, def := range s.ws.DocumentDefinitions() {
		out.Documents = append(out.Documents, documentInfo{
			Name: def.FQN,
			URI:  string(s.documentURI(def.Doc)),
		})
	}
	return out
}

// RenderDocument answers opensysml/renderDocument: the named document compiled,
// evaluated and rendered as Markdown, or the typed error stopping it.
func (s *Server) RenderDocument(params *renderDocumentParams) (*renderDocumentResult, error) {
	opts := docrender.MarkdownOptions{DiagramForm: view.Form(params.DiagramForm)}
	markdown, err := s.ws.RenderDocumentMarkdown(params.Name, opts)
	if err != nil {
		return nil, err
	}
	return &renderDocumentResult{Name: params.Name, Markdown: markdown}, nil
}
