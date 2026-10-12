package lsptools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/lsp"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// RenderViewInput asks for one rendering of a document's view.
type RenderViewInput struct {
	Path         string `json:"path" jsonschema:"the document, absolute or relative to the workspace root"`
	View         string `json:"view,omitempty" jsonschema:"the qualified name of a view the document declares, a pseudo-view such as #tree, #interconnection, #state, #action, #sequence or #tree:Pkg::Part (see the views tool), or omitted for the one view the document declares (a document declaring none needs a pseudo-view)"`
	Form         string `json:"form,omitempty" jsonschema:"the artifact's notation: dot (the default), mermaid, plantuml, d2, svg, or json"`
	Palette      string `json:"palette,omitempty" jsonschema:"the colors: pilot (default) or cameo"`
	Style        string `json:"style,omitempty" jsonschema:"the drawing style of dot/mermaid: pilot (default) or cameo"`
	Ports        string `json:"ports,omitempty" jsonschema:"how ports are drawn: shown, hidden or nested"`
	Overlay      string `json:"overlay,omitempty" jsonschema:"an overlay of analysis results on the diagram, as the LSP's opensysml/render accepts"`
	LinkTemplate string `json:"linkTemplate,omitempty" jsonschema:"a URL template for node links, with {fqn} substituted"`
}

// RenderDocumentInput names a document definition to render.
type RenderDocumentInput struct {
	Name         string `json:"name" jsonschema:"the qualified name of a document definition (doc def), as documents lists"`
	DiagramForm  string `json:"diagramForm,omitempty" jsonschema:"the notation of embedded diagrams: mermaid (default), dot, plantuml, d2 or svg"`
	LinkTemplate string `json:"linkTemplate,omitempty" jsonschema:"a URL template for element links, with {fqn} substituted"`
}

// NoInput takes no arguments.
type NoInput struct{}

func registerRendering(s *mcp.Server, h *Host) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "render_view",
		Description: "Render a diagram of a document: a declared view or a pseudo-view, as Graphviz dot, Mermaid, PlantUML, D2, SVG or JSON text. " +
			"The result also lists the nodes and edges drawn with their qualified names (for apply_model_edit) and their declaration ranges in LSP form (0-based).",
		Annotations: toolkit.ReadOnly("Render view"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in RenderViewInput) (*mcp.CallToolResult, *lsp.RenderResult, error) {
		ref, err := h.document(ctx, in.Path)
		if err != nil {
			return nil, nil, err
		}
		if _, err := h.content(ref); err != nil {
			return nil, nil, err
		}
		res, err := h.Server.Render(&lsp.RenderParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: ref.uri()},
			View:         in.View, Form: in.Form, Palette: in.Palette, Style: in.Style, Ports: in.Ports, Overlay: in.Overlay, LinkTemplate: in.LinkTemplate,
		})
		if err != nil {
			return nil, nil, err
		}
		return text(res.Artifact), res, nil
	})
	mcp.AddTool(s, &mcp.Tool{
		Name:        "views",
		Description: "List the views a document declares and the pseudo-views (#tree, #interconnection, ...) render_view can draw of it.",
		Annotations: toolkit.ReadOnly("Views"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in PathInput) (*mcp.CallToolResult, *lsp.ViewsResult, error) {
		ref, err := h.document(ctx, in.Path)
		if err != nil {
			return nil, nil, err
		}
		if _, err := h.content(ref); err != nil {
			return nil, nil, err
		}
		res := h.Server.Views(&lsp.ViewsParams{TextDocument: protocol.TextDocumentIdentifier{URI: ref.uri()}})
		var b strings.Builder
		for _, v := range res.Views {
			fmt.Fprintf(&b, "%s (%s)\n", v.Name, v.Kind)
		}
		if len(res.Views) == 0 {
			b.WriteString("no declared views\n")
		}
		fmt.Fprintf(&b, "pseudo-views: %s", strings.Join(res.PseudoViews, ", "))
		return text(strings.TrimSpace(b.String())), res, nil
	})
	mcp.AddTool(s, &mcp.Tool{
		Name:        "documents",
		Description: "List the document definitions (doc def) declared across the workspace, which render_document renders to Markdown.",
		Annotations: toolkit.ReadOnly("Document definitions"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ NoInput) (*mcp.CallToolResult, *lsp.DocumentsResult, error) {
		res := h.Server.Documents()
		var b strings.Builder
		for _, d := range res.Documents {
			fmt.Fprintf(&b, "%s in %s\n", d.Name, h.display(protocol.DocumentURI(d.URI)))
		}
		if b.Len() == 0 {
			b.WriteString("no document definitions")
		}
		return text(strings.TrimSpace(b.String())), res, nil
	})
	mcp.AddTool(s, &mcp.Tool{
		Name:        "render_document",
		Description: "Render a document definition (doc def) to Markdown, its queries evaluated over the workspace and its diagrams embedded.",
		Annotations: toolkit.ReadOnly("Render document"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in RenderDocumentInput) (*mcp.CallToolResult, *lsp.RenderDocumentResult, error) {
		if strings.TrimSpace(in.Name) == "" {
			return nil, nil, fmt.Errorf("name is required")
		}
		res, err := h.Server.RenderDocument(&lsp.RenderDocumentParams{Name: in.Name, DiagramForm: in.DiagramForm, LinkTemplate: in.LinkTemplate})
		if err != nil {
			return nil, nil, err
		}
		return text(res.Markdown), res, nil
	})
}
