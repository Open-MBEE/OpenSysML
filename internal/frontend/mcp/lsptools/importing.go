package lsptools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/lsp"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// ImportDiagramInput names the document whose view a draw.io file draws and
// the file, edited in draw.io after render_view wrote it with form drawio.
type ImportDiagramInput struct {
	Path    string `json:"path" jsonschema:"the document declaring the view, absolute or relative to the workspace root"`
	File    string `json:"file,omitempty" jsonschema:"the draw.io file to import, absolute or relative to the workspace root; alternative to content"`
	Content string `json:"content,omitempty" jsonschema:"the draw.io file's text, as render_view form drawio wrote it and draw.io saved it, compressed or not; alternative to file"`
	View    string `json:"view,omitempty" jsonschema:"the view to import when the file has several pages; omitted, the one page's view"`
	Inline  bool   `json:"inline,omitempty" jsonschema:"write the annotations beside the elements rather than in the view's body"`
	Apply   bool   `json:"apply,omitempty" jsonschema:"write the edits to disk; omitted, a dry run answering the edits and the text changes they amount to"`
}

// ImportDiagramResult is what the import plans and leaves alone, and the text
// edits the plan amounts to, written when apply was asked.
type ImportDiagramResult struct {
	View      string                  `json:"view" jsonschema:"the view the file draws"`
	Edits     []lsp.ImportDiagramEdit `json:"edits" jsonschema:"the DiagramLayout annotations the import plans, each from one cell"`
	Unmatched []string                `json:"unmatched,omitempty" jsonschema:"elements the file names that the rendering lacks; their cells are not imported"`
	Unknown   []string                `json:"unknown,omitempty" jsonschema:"cells naming no element; nothing is created for them"`
	Deleted   []string                `json:"deleted,omitempty" jsonschema:"elements whose cells the file lost; nothing is deleted for them"`
	Notices   []string                `json:"notices,omitempty" jsonschema:"changes the file shows that no annotation carries yet"`
	Refused   []lsp.ModelEditRefusal  `json:"refused,omitempty" jsonschema:"the planned edit the model editor refused, with why"`
	Changes   []FileEdits             `json:"changes" jsonschema:"the text edits, per document"`
	Applied   bool                    `json:"applied" jsonschema:"whether the edits were written to disk"`
}

func registerImporting(s *mcp.Server, h *Host) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "import_diagram",
		Description: "Import a draw.io (diagrams.net) file that render_view wrote with form drawio and a user edited in draw.io: what moved, resized, folded, re-routed or recoloured, and the page size, " +
			"become DiagramLayout annotations (Layout, Route, Style, Canvas, Note, Picture) in the view's body, applied as apply_model_edit applies its operations. " +
			"Cells the rendering lacks, cells naming no element and deleted cells are reported, never created or deleted. Answers the planned edits and text changes; writes the files only with apply=true.",
		Annotations: toolkit.Mutating("Import diagram", false),
	}, writing(h, func(ctx context.Context, _ *mcp.CallToolRequest, in ImportDiagramInput) (*mcp.CallToolResult, ImportDiagramResult, error) {
		if in.File == "" && in.Content == "" {
			return nil, ImportDiagramResult{}, errors.New("file or content is required")
		}
		ref, _, err := h.editable(ctx, in.Path)
		if err != nil {
			return nil, ImportDiagramResult{}, err
		}
		file := ""
		if in.File != "" {
			res, err := h.root.Resolve(in.File)
			if err != nil {
				return nil, ImportDiagramResult{}, err
			}
			file = res.Real
		}
		result, err := h.Server.ImportDiagram(&lsp.ImportDiagramParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: ref.uri()},
			Path:         file, Content: in.Content, View: in.View, Inline: in.Inline, DryRun: !in.Apply,
		})
		if err != nil {
			return nil, ImportDiagramResult{}, err
		}
		res := ImportDiagramResult{View: result.View, Edits: result.Edits, Unmatched: result.Unmatched, Unknown: result.Unknown,
			Deleted: result.Deleted, Notices: result.Notices, Refused: result.Refused, Changes: []FileEdits{}}
		if result.Edit != nil {
			_, edited, err := h.workspaceEdit(ctx, result.Edit, in.Apply, "diagram import")
			if err != nil {
				return nil, ImportDiagramResult{}, err
			}
			res.Changes, res.Applied = edited.Changes, edited.Applied
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%s: %d edits planned, %d refused, edits to %d documents", res.View, len(res.Edits), len(res.Refused), len(res.Changes))
		if res.Applied {
			b.WriteString(", written")
		}
		for _, e := range res.Edits {
			fmt.Fprintf(&b, "\n%s about %s", e.Kind, e.Target)
		}
		for _, r := range res.Refused {
			fmt.Fprintf(&b, "\nrefused (%s): %s", r.Failure, r.Message)
		}
		for _, n := range res.Notices {
			b.WriteString("\n" + n)
		}
		return text(b.String()), res, nil
	}))
}
