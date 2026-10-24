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
	Path         string   `json:"path" jsonschema:"the document, absolute or relative to the workspace root"`
	View         string   `json:"view,omitempty" jsonschema:"the qualified name of a view the document declares, a pseudo-view such as #tree, #interconnection, #state, #action, #sequence or #tree:Pkg::Part (see the views tool), or omitted for the one view the document declares (a document declaring none needs a pseudo-view)"`
	Form         string   `json:"form,omitempty" jsonschema:"the artifact's notation: mermaid, dot (Graphviz), plantuml, d2, text, markdown, csv or tsv; omitted, markdown for a table or matrix and mermaid otherwise"`
	Palette      string   `json:"palette,omitempty" jsonschema:"fill colors by keyword family for dot/mermaid/plantuml/d2: okabe-ito, tol-bright, tol-muted, tol-light, brewer-set2, brewer-dark2, viridis or cividis; omitted draws black and white"`
	Style        string   `json:"style,omitempty" jsonschema:"the drawing style of dot/mermaid: pilot (the default, the Pilot visualizer's look) or cameo (Cameo Systems Modeler's)"`
	Ports        string   `json:"ports,omitempty" jsonschema:"ports drawn on an interconnection or mixed rendering: minimal (the default, only connected ports) or full (every port, labelled name : Type)"`
	Lines        string   `json:"lines,omitempty" jsonschema:"how dot and plantuml route edges: splines (the default), ortho or polyline"`
	Compartments string   `json:"compartments,omitempty" jsonschema:"how a tree rendering folds a node's owned members into compartment rows of its box: positioned (the default, only the layout-less members of a tree some Layout positions), most (every owned usage no Layout, nested definition or package keeps a node) or ports (the port usages with the ports nested under them)"`
	Show         []string `json:"show,omitempty" jsonschema:"optional notation elements drawn besides the ones the view's rendering shows: parameter-symbols, port-symbols, compartments, edge-labels, keywords, multiplicities, typings, qualifiers, or all; port symbols, compartments and keywords are drawn by default"`
	Depth        int      `json:"depth,omitempty" jsonschema:"how many levels of nested elements a tree or interconnection rendering draws below each exposed element, overriding the view's depth attribute; omitted or 0 keeps the view's own depth"`
	Overlay      string   `json:"overlay,omitempty" jsonschema:"verdicts, on a requirement rendering: runs the verification cases and colors each requirement by its verdict; omitted runs nothing"`
	LinkTemplate string   `json:"linkTemplate,omitempty" jsonschema:"a template for source links on elements, with {file}, {line}, {col}, {qname} and {id} placeholders"`
}

// RenderDocumentInput names a document definition to render.
type RenderDocumentInput struct {
	Name         string   `json:"name" jsonschema:"the qualified name of a document definition (doc def), as documents lists"`
	DiagramForm  string   `json:"diagramForm,omitempty" jsonschema:"the notation of every graph-shaped diagram block: mermaid, dot, plantuml or d2; omitted, chosen per diagram as the CLI chooses it"`
	Show         []string `json:"show,omitempty" jsonschema:"optional notation elements every diagram draws besides those its view and diagram block show: parameter-symbols, port-symbols, compartments, edge-labels, keywords, multiplicities, typings, qualifiers, or all"`
	LinkTemplate string   `json:"linkTemplate,omitempty" jsonschema:"a template for source links on elements, with {file}, {line}, {col}, {qname} and {id} placeholders"`
}

// NoInput takes no arguments.
type NoInput struct{}

func registerRendering(s *mcp.Server, h *Host) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "render_view",
		Description: "Render a diagram or table of a document: a declared view or a pseudo-view, as Mermaid, Graphviz dot, PlantUML, D2, text, Markdown, CSV or TSV. " +
			"The result also lists the nodes and edges drawn with their qualified names (for apply_model_edit) and their declaration ranges in LSP form (0-based).",
		Annotations: toolkit.ReadOnly("Render view"),
	}, reading(h, func(ctx context.Context, _ *mcp.CallToolRequest, in RenderViewInput) (*mcp.CallToolResult, *lsp.RenderResult, error) {
		ref, err := h.document(ctx, in.Path)
		if err != nil {
			return nil, nil, err
		}
		if _, err := h.content(ref); err != nil {
			return nil, nil, err
		}
		res, err := h.Server.Render(&lsp.RenderParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: ref.uri()},
			View:         in.View, Form: in.Form, Palette: in.Palette, Style: in.Style, Ports: in.Ports, Lines: in.Lines, Compartments: in.Compartments, Show: in.Show, Depth: in.Depth, Overlay: in.Overlay, LinkTemplate: in.LinkTemplate,
		})
		if err != nil {
			return nil, nil, err
		}
		return text(res.Artifact), res, nil
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "views",
		Description: "List the views a document declares and the pseudo-views (#tree, #interconnection, ...) render_view can draw of it.",
		Annotations: toolkit.ReadOnly("Views"),
	}, reading(h, func(ctx context.Context, _ *mcp.CallToolRequest, in PathInput) (*mcp.CallToolResult, *lsp.ViewsResult, error) {
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
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "documents",
		Description: "List the document definitions (doc def) declared across the workspace, which render_document renders to Markdown.",
		Annotations: toolkit.ReadOnly("Document definitions"),
	}, reading(h, func(ctx context.Context, _ *mcp.CallToolRequest, _ NoInput) (*mcp.CallToolResult, *lsp.DocumentsResult, error) {
		h.Refresh(ctx)
		res := h.Server.Documents()
		var b strings.Builder
		for _, d := range res.Documents {
			fmt.Fprintf(&b, "%s in %s\n", d.Name, h.display(protocol.DocumentURI(d.URI)))
		}
		if b.Len() == 0 {
			b.WriteString("no document definitions")
		}
		return text(strings.TrimSpace(b.String())), res, nil
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "render_document",
		Description: "Render a document definition (doc def) to Markdown, its queries evaluated over the workspace and its diagrams embedded.",
		Annotations: toolkit.ReadOnly("Render document"),
	}, reading(h, func(ctx context.Context, _ *mcp.CallToolRequest, in RenderDocumentInput) (*mcp.CallToolResult, *lsp.RenderDocumentResult, error) {
		if strings.TrimSpace(in.Name) == "" {
			return nil, nil, fmt.Errorf("name is required")
		}
		h.Refresh(ctx)
		res, err := h.Server.RenderDocument(&lsp.RenderDocumentParams{Name: in.Name, DiagramForm: in.DiagramForm, Show: in.Show, LinkTemplate: in.LinkTemplate})
		if err != nil {
			return nil, nil, err
		}
		return text(res.Markdown), res, nil
	}))
}
