package lsp

import (
	"github.com/Open-MBEE/OpenSysML/internal/doc/docpdf"
	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/modeldoc"
)

// The custom methods a document-preview client speaks, alongside the diagram
// methods in render.go.
const (
	// MethodDocuments lists the document definitions the workspace holds.
	MethodDocuments = "opensysml/documents"
	// MethodRenderDocument renders a named document definition as Markdown.
	MethodRenderDocument = "opensysml/renderDocument"
)

// DocumentsResult lists the document definitions of the workspace, each by
// qualified name and the file declaring it.
type DocumentsResult struct {
	Documents []DocumentInfo `json:"documents"`
}

// DocumentInfo is one document definition a client may ask to render.
type DocumentInfo struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
}

// RenderDocumentParams asks for the Markdown rendering of the document
// definition Name names. DiagramForm is the source its graph-shaped diagrams
// are written as, mermaid, dot, plantuml or d2; empty is mermaid.
type RenderDocumentParams struct {
	Name         string   `json:"name"`
	DiagramForm  string   `json:"diagramForm,omitempty"`
	Show         []string `json:"show,omitempty"`
	LinkTemplate string   `json:"linkTemplate,omitempty"`
}

// RenderDocumentResult is the rendered document.
type RenderDocumentResult struct {
	Name     string `json:"name"`
	Markdown string `json:"markdown"`
}

// Documents answers opensysml/documents: the document definitions declared
// across the workspace, in qualified-name order.
func (s *Server) Documents() *DocumentsResult {
	out := &DocumentsResult{Documents: []DocumentInfo{}}
	for _, def := range s.ws.DocumentDefinitions() {
		out.Documents = append(out.Documents, DocumentInfo{
			Name: def.FQN,
			URI:  string(s.documentURI(def.Doc)),
		})
	}
	return out
}

// RenderDocument answers opensysml/renderDocument: the named document compiled,
// evaluated and rendered as Markdown, or the typed error stopping it.
func (s *Server) RenderDocument(params *RenderDocumentParams) (*RenderDocumentResult, error) {
	shown, err := view.ShownOf(params.Show)
	if err != nil {
		return nil, err
	}
	opts := docrender.MarkdownOptions{
		DiagramForm: view.Form(params.DiagramForm), Show: shown, Drawer: docpdf.Graphviz{}, LinkTemplate: params.LinkTemplate,
	}
	markdown, err := modeldoc.RenderDocumentMarkdown(s.ws, params.Name, opts)
	if err != nil {
		return nil, err
	}
	return &RenderDocumentResult{Name: params.Name, Markdown: markdown}, nil
}
