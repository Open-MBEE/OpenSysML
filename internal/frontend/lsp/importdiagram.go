package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	modeledit "github.com/Open-MBEE/OpenSysML/internal/check/edit"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// MethodImportDiagram is the request importing a draw.io file the drawio
// rendering form wrote and the user edited, as DiagramLayout metadata edits.
const MethodImportDiagram = "opensysml/importDiagram"

// ImportDiagramParams names the document whose view the file draws, the file
// by path or content, and the view when the file has several pages.
type ImportDiagramParams struct {
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
	Path         string                          `json:"path,omitempty"`
	Content      string                          `json:"content,omitempty"`
	View         string                          `json:"view,omitempty"`
	// Inline writes the annotations beside the elements rather than in the view's body.
	Inline bool `json:"inline,omitempty"`
	// DryRun answers the edits without the client being asked to apply them.
	DryRun bool `json:"dryRun,omitempty"`
}

// ImportDiagramEdit is one edit the import plans: the annotation kind, the
// element it is about, and the file cell it comes from.
type ImportDiagramEdit struct {
	Kind   string `json:"kind"`
	Target string `json:"target,omitempty"`
	Label  string `json:"label,omitempty"`
	Cell   string `json:"cell"`
}

// ImportDiagramResult is what the import plans and what it leaves alone, and
// the WorkspaceEdit the plan amounts to, as opensysml/applyModelEdit answers it.
type ImportDiagramResult struct {
	View      string                  `json:"view"`
	DryRun    bool                    `json:"dryRun"`
	Edits     []ImportDiagramEdit     `json:"edits"`
	Unmatched []string                `json:"unmatched,omitempty"`
	Unknown   []string                `json:"unknown,omitempty"`
	Deleted   []string                `json:"deleted,omitempty"`
	Notices   []string                `json:"notices,omitempty"`
	Refused   []ModelEditRefusal      `json:"refused,omitempty"`
	Version   int                     `json:"version"`
	Edit      *protocol.WorkspaceEdit `json:"edit,omitempty"`
}

// importDiagramHandler dispatches opensysml/importDiagram and passes everything
// else on.
func (s *Server) importDiagramHandler(inner jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		if req.Method() != MethodImportDiagram {
			return inner(ctx, reply, req)
		}
		var params ImportDiagramParams
		if err := json.Unmarshal(req.Params(), &params); err != nil {
			return reply(ctx, nil, fmt.Errorf("%s: %w", jsonrpc2.ErrParse, err))
		}
		result, err := s.ImportDiagram(&params)
		if err != nil {
			return reply(ctx, nil, err)
		}
		return reply(ctx, result, nil)
	}
}

// ImportDiagram answers opensysml/importDiagram: the file is read, the view it
// draws rendered as the export rendered it, and what differs becomes edits.
func (s *Server) ImportDiagram(params *ImportDiagramParams) (*ImportDiagramResult, error) {
	name := uriToName(params.TextDocument.URI)
	if s.ws.Document(name) == nil {
		return nil, fmt.Errorf("%s: no such document", name)
	}
	data := []byte(params.Content)
	switch {
	case params.Path != "" && params.Content != "":
		return nil, fmt.Errorf("%s: path and content are alternatives; give one", jsonrpc2.ErrInvalidParams)
	case params.Path != "":
		var err error
		if data, err = os.ReadFile(params.Path); err != nil {
			return nil, err
		}
	case params.Content == "":
		return nil, fmt.Errorf("%s: the draw.io file is required, by path or content", jsonrpc2.ErrInvalidParams)
	}
	file, err := view.ReadDrawio(data, params.View)
	if err != nil {
		return nil, err
	}
	var (
		rendering *view.Rendering
		snapshot  *model.Snapshot
	)
	if err := s.ws.Read(func(r *model.Reading) (err error) {
		rendering, snapshot, err = r.RenderOverlaidView(name, file.View, "", nil, 0, nil)
		if err == nil {
			r.LinkSites(rendering, snapshot)
		}
		return err
	}); err != nil {
		return nil, err
	}
	imp, err := rendering.ImportDrawio(file, view.Options{Links: view.Links{Sites: snapshot.Sites()}})
	if err != nil {
		return nil, err
	}
	out := &ImportDiagramResult{View: file.View, DryRun: params.DryRun, Edits: []ImportDiagramEdit{},
		Unmatched: imp.Unmatched, Deleted: imp.Deleted, Notices: imp.Notices}
	for _, cell := range imp.Unknown {
		out.Unknown = append(out.Unknown, cell.ID)
	}
	for _, e := range imp.Edits {
		out.Edits = append(out.Edits, ImportDiagramEdit{Kind: e.Kind.String(), Target: importTarget(e), Label: e.Label, Cell: e.Cell})
	}
	if len(imp.Edits) == 0 {
		return out, nil
	}
	result, version, ok, err := s.ws.ApplyEdit(name, modeledit.DrawioOperations(imp, params.Inline), nil)
	if !ok {
		return nil, fmt.Errorf("%s: no such document", name)
	}
	out.Version = version
	if err != nil {
		var refusal *modeledit.Error
		if !errors.As(err, &refusal) {
			return nil, err
		}
		out.Refused = []ModelEditRefusal{s.refusal(refusal, s.ws.Document(name).Content)}
		return out, nil
	}
	changes := make([]protocol.TextDocumentEdit, 0, len(result.Documents))
	for _, edited := range result.Documents {
		change, err := documentChange(edited)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	out.Edit = &protocol.WorkspaceEdit{DocumentChanges: changes}
	return out, nil
}

// importTarget names what an import edit is about: the element, the edge's
// ends, or the page.
func importTarget(e view.DrawioEdit) string {
	switch {
	case e.Kind == view.DrawioEditCanvas:
		return ""
	case e.Edge != nil:
		return e.Edge.String()
	}
	return e.FQN
}
