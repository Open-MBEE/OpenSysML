package lsptools

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/lsp"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// FileEdits is the edits a change makes to one document.
type FileEdits struct {
	Path  string     `json:"path"`
	Edits []TextEdit `json:"edits" jsonschema:"edits to the document's current text, non-overlapping, in document order"`
}

// FormatInput names the document, and optionally the lines, to format.
type FormatInput struct {
	Path  string `json:"path" jsonschema:"the document to format, absolute or relative to the workspace root"`
	Range *Range `json:"range,omitempty" jsonschema:"format only the lines this range touches; omitted, the whole document"`
	Apply bool   `json:"apply,omitempty" jsonschema:"write the formatted text to the file, replacing it atomically (its mode is kept, other inode metadata is not); false (the default) only returns the edits"`
}

// FormatResult is the formatting of a document.
type FormatResult struct {
	Path    string     `json:"path"`
	Edits   []TextEdit `json:"edits" jsonschema:"the edits that format the document; empty when it is already formatted"`
	Text    string     `json:"text" jsonschema:"the document's text with the edits applied"`
	Applied bool       `json:"applied" jsonschema:"whether the text was written to disk"`
}

// RenameInput names the symbol at a position and its new name.
type RenameInput struct {
	Path     string           `json:"path" jsonschema:"the document, absolute or relative to the workspace root"`
	Position toolkit.Position `json:"position" jsonschema:"a position on the name to rename (1-based line and column)"`
	NewName  string           `json:"newName" jsonschema:"the new name"`
	Apply    bool             `json:"apply,omitempty" jsonschema:"write every affected file, all or none, each replaced atomically (its mode is kept, other inode metadata is not); false (the default) only returns the edits"`
}

// WorkspaceEditResult is a change spanning documents.
type WorkspaceEditResult struct {
	Changes []FileEdits `json:"changes" jsonschema:"the edits, per document"`
	Applied bool        `json:"applied" jsonschema:"whether the edits were written to disk"`
}

// ApplyModelEditInput carries model-level operations against a document.
type ApplyModelEditInput struct {
	Path       string                   `json:"path" jsonschema:"the document the operations address, absolute or relative to the workspace root"`
	Operations []lsp.ModelEditOperation `json:"operations"`
	Apply      bool                     `json:"apply,omitempty" jsonschema:"write every affected file, all or none, each replaced atomically (its mode is kept, other inode metadata is not); false (the default) only returns the edits"`
}

// ModelEditResult is the outcome of apply_model_edit.
type ModelEditResult struct {
	Changes []FileEdits            `json:"changes" jsonschema:"the text edits the operations amount to, per document; empty when every operation was refused"`
	Refused []lsp.ModelEditRefusal `json:"refused,omitempty" jsonschema:"operations not carried out, by index, with the failure code and message (ranges inside are LSP ranges, 0-based)"`
	Version int                    `json:"version" jsonschema:"the version of the document the edits apply to"`
	Applied bool                   `json:"applied" jsonschema:"whether the edits were written to disk"`
}

var formattingOptions = protocol.FormattingOptions{TabSize: 4, InsertSpaces: true}

func registerEdits(s *mcp.Server, h *Host) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "format",
		Description: "Format a SysML v2 / KerML document, or only the lines of a range, in the canonical style. " +
			"Returns the edits and the resulting text; writes the file only with apply=true. A document that does not parse is left unchanged.",
		Annotations: toolkit.Mutating("Format document", true),
	}, writing(h, func(ctx context.Context, _ *mcp.CallToolRequest, in FormatInput) (*mcp.CallToolResult, FormatResult, error) {
		ref, content, err := h.editable(ctx, in.Path)
		if err != nil {
			return nil, FormatResult{}, err
		}
		var edits []protocol.TextEdit
		if in.Range == nil {
			edits, err = h.Server.Formatting(ctx, &protocol.DocumentFormattingParams{TextDocument: protocol.TextDocumentIdentifier{URI: ref.uri()}, Options: formattingOptions})
		} else {
			var r protocol.Range
			if r, err = rangeToProtocol(content, *in.Range); err != nil {
				return nil, FormatResult{}, err
			}
			edits, err = h.Server.RangeFormatting(ctx, &protocol.DocumentRangeFormattingParams{TextDocument: protocol.TextDocumentIdentifier{URI: ref.uri()}, Range: r, Options: formattingOptions})
		}
		if err != nil {
			return nil, FormatResult{}, err
		}
		formatted, err := applyTextEdits(content, edits)
		if err != nil {
			return nil, FormatResult{}, err
		}
		res := FormatResult{Path: h.display(ref.uri()), Edits: convertEdits(content, edits), Text: string(formatted)}
		if in.Apply && len(edits) > 0 {
			if err := h.writeAll(ctx, []pendingWrite{{name: ref.name, content: formatted, base: content}}); err != nil {
				return nil, FormatResult{}, err
			}
			res.Applied = true
		}
		summary := fmt.Sprintf("%s: %d edits", res.Path, len(res.Edits))
		if res.Applied {
			summary += ", written"
		}
		return text(summary + "\n" + res.Text), res, nil
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name: "rename",
		Description: "Rename the element declared or referenced at a position, everywhere in the workspace. " +
			"Returns the edits per document; writes the files only with apply=true.",
		Annotations: toolkit.Mutating("Rename element", false),
	}, writing(h, func(ctx context.Context, _ *mcp.CallToolRequest, in RenameInput) (*mcp.CallToolResult, WorkspaceEditResult, error) {
		if strings.TrimSpace(in.NewName) == "" {
			return nil, WorkspaceEditResult{}, errors.New("newName is required")
		}
		ref, content, err := h.editable(ctx, in.Path)
		if err != nil {
			return nil, WorkspaceEditResult{}, err
		}
		pos, err := toProtocol(content, in.Position)
		if err != nil {
			return nil, WorkspaceEditResult{}, err
		}
		edit, err := h.Server.Rename(ctx, &protocol.RenameParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: ref.uri()}, Position: pos},
			NewName:                    in.NewName,
		})
		if err != nil {
			return nil, WorkspaceEditResult{}, err
		}
		if edit == nil {
			return nil, WorkspaceEditResult{}, fmt.Errorf("nothing to rename at %s:%d:%d", h.display(ref.uri()), in.Position.Line, in.Position.Column)
		}
		return h.workspaceEdit(ctx, edit, in.Apply, "rename")
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name: "apply_model_edit",
		Description: "Edit a model by operations on its elements rather than by text: setValue, rename, addMember, addConnection, addSequence (action-body items), delete, move, " +
			"and the diagram-layout annotations setLayout, setRoute, setCanvas, setStyle. Targets and owners are qualified names as render_view reports them in nodes[].fqn. " +
			"Returns the text edits the operations amount to, per document, and the operations refused with why; writes the files only with apply=true.",
		Annotations: toolkit.Mutating("Apply model edit", false),
		InputSchema: applyModelEditSchema(),
	}, writing(h, func(ctx context.Context, _ *mcp.CallToolRequest, in ApplyModelEditInput) (*mcp.CallToolResult, ModelEditResult, error) {
		if len(in.Operations) == 0 {
			return nil, ModelEditResult{}, errors.New("operations is required")
		}
		ref, _, err := h.editable(ctx, in.Path)
		if err != nil {
			return nil, ModelEditResult{}, err
		}
		doc := h.Workspace.Document(ref.name)
		result, err := h.Server.ApplyModelEdit(&lsp.ApplyModelEditParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: ref.uri()},
			Version:      doc.Version,
			Operations:   in.Operations,
		})
		if err != nil {
			return nil, ModelEditResult{}, err
		}
		if result.Stale {
			return nil, ModelEditResult{}, fmt.Errorf("%s changed while the edit was computed; call the tool again", h.display(ref.uri()))
		}
		res := ModelEditResult{Changes: []FileEdits{}, Refused: result.Refused, Version: result.Version}
		if result.Edit != nil {
			_, edited, err := h.workspaceEdit(ctx, result.Edit, in.Apply, "model edit")
			if err != nil {
				return nil, ModelEditResult{}, err
			}
			res.Changes, res.Applied = edited.Changes, edited.Applied
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%d operations: %d refused, edits to %d documents", len(in.Operations), len(res.Refused), len(res.Changes))
		if res.Applied {
			b.WriteString(", written")
		}
		for _, r := range res.Refused {
			fmt.Fprintf(&b, "\noperation %d refused (%s): %s", r.Operation, r.Failure, r.Message)
		}
		return text(b.String()), res, nil
	}))
}

// editable resolves a workspace document that may be edited, with its text.
func (h *Host) editable(ctx context.Context, path string) (docRef, []byte, error) {
	ref, err := h.document(ctx, path)
	if err != nil {
		return docRef{}, nil, err
	}
	if ref.library {
		return docRef{}, nil, fmt.Errorf("%s is a standard library document and cannot be edited", h.display(ref.uri()))
	}
	content, err := h.content(ref)
	if err != nil {
		return docRef{}, nil, err
	}
	return ref, content, nil
}

// workspaceEdit converts a protocol workspace edit into per-document edits in
// 1-based positions, writing each document when apply is set.
func (h *Host) workspaceEdit(ctx context.Context, edit *protocol.WorkspaceEdit, apply bool, what string) (*mcp.CallToolResult, WorkspaceEditResult, error) {
	changes := map[uri.URI][]protocol.TextEdit{}
	for u, edits := range edit.Changes {
		changes[u] = append(changes[u], edits...)
	}
	for _, change := range edit.DocumentChanges {
		edit, ok := change.(*protocol.TextDocumentEdit)
		if !ok {
			continue
		}
		for _, element := range edit.Edits {
			if textEdit, ok := element.(*protocol.TextEdit); ok {
				changes[edit.TextDocument.URI] = append(changes[edit.TextDocument.URI], *textEdit)
			}
		}
	}
	uris := make([]string, 0, len(changes))
	for u := range changes {
		uris = append(uris, string(u))
	}
	sort.Strings(uris)
	res := WorkspaceEditResult{Changes: []FileEdits{}}
	var b strings.Builder
	var writes []pendingWrite
	for _, u := range uris {
		ref := h.refOf(uri.URI(u))
		if !ref.library {
			if _, err := h.resolve(ref.name); err != nil {
				return nil, WorkspaceEditResult{}, fmt.Errorf("%s: %w", what, err)
			}
		}
		content := h.contentOf(uri.URI(u))
		if content == nil {
			return nil, WorkspaceEditResult{}, fmt.Errorf("%s edits %s, which the workspace does not hold", what, h.display(ref.uri()))
		}
		edits := changes[uri.URI(u)]
		res.Changes = append(res.Changes, FileEdits{Path: h.display(ref.uri()), Edits: convertEdits(content, edits)})
		fmt.Fprintf(&b, "%s: %d edits\n", h.display(ref.uri()), len(edits))
		if !apply {
			continue
		}
		if ref.library {
			return nil, WorkspaceEditResult{}, fmt.Errorf("%s edits the standard library document %s, which is read-only", what, ref.name)
		}
		edited, err := applyTextEdits(content, edits)
		if err != nil {
			return nil, WorkspaceEditResult{}, fmt.Errorf("%s: %w", h.display(ref.uri()), err)
		}
		writes = append(writes, pendingWrite{name: ref.name, content: edited, base: content})
	}
	if len(writes) > 0 {
		if err := h.writeAll(ctx, writes); err != nil {
			return nil, WorkspaceEditResult{}, err
		}
		res.Applied = true
		b.WriteString("written to disk")
	}
	return text(strings.TrimSpace(b.String())), res, nil
}

// convertEdits turns protocol edits into 1-based edits in document order.
func convertEdits(content []byte, edits []protocol.TextEdit) []TextEdit {
	out := make([]TextEdit, 0, len(edits))
	for _, e := range edits {
		out = append(out, TextEdit{Range: rangeFromProtocol(content, e.Range), NewText: e.NewText})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Range.Start.Line != out[j].Range.Start.Line {
			return out[i].Range.Start.Line < out[j].Range.Start.Line
		}
		return out[i].Range.Start.Column < out[j].Range.Start.Column
	})
	return out
}

// applyModelEditSchema is the input schema of apply_model_edit, written out
// because an operation's body nests operations, which schema inference rejects.
func applyModelEditSchema() *jsonschema.Schema {
	root := schemaWithRecursion(reflect.TypeFor[ApplyModelEditInput](), reflect.TypeFor[lsp.ModelEditOperation](), "operation")
	root.Properties["operations"].Description = "the operations, applied in order against the document's current text"
	describe(root.Defs["operation"], operationFieldDescriptions)
	return root
}

func describe(s *jsonschema.Schema, descriptions map[string]string) {
	for name, desc := range descriptions {
		if p, ok := s.Properties[name]; ok {
			p.Description = desc
		}
	}
}

var operationFieldDescriptions = map[string]string{
	"kind":           "setValue, rename, addMember, addConnection, addSequence, delete, move, setLayout, setRoute, setCanvas or setStyle",
	"target":         "qualified name of the element the operation is about (setValue, rename, delete, move, setCanvas, and the layout operations when no declaration is given)",
	"declaration":    "for setLayout, setRoute, setStyle: the LSP range (0-based) of an unnamed element's declaration, as render_view reports it, in place of target",
	"declaredIn":     "the document URI declaring the target, as render_view reports in origin.uri; omitted, the requested document",
	"digest":         "the origin.digest render_view reported for declaredIn, so a changed document is detected",
	"value":          "setValue: the new value expression; addMember: an initial value; addSequence: the value assigned, sent or iterated over",
	"bodyExpression": "addMember: the body expression of a calculation or constraint member",
	"newName":        "rename: the new name",
	"owner":          "addMember, addConnection, addSequence: the qualified name of the owner, empty for the document; move: the new owner",
	"memberKind":     "addMember: the member's keyword (part, attribute, port, action, state, ...); addConnection: connection, interface, allocation, binding, flow, succession or transition; addSequence: the action-body item (if, for, while, assign, send, accept, perform, ...)",
	"name":           "addMember: the member's name; addConnection: an optional name",
	"type":           "addMember, addConnection: the member's type, by qualified name",
	"multiplicity":   "addMember: a multiplicity such as [0..*]",
	"specializes":    "addMember: qualified names of what the member specializes",
	"from":           "addConnection: the end the connection starts at",
	"to":             "addConnection: the end the connection goes to",
	"cascade":        "delete: also delete what refers to the target instead of refusing",
	"view":           "the view a layout annotation belongs to, as render_view reports in view",
	"keyword":        "addSequence: the succession keyword before the item (then, first)",
	"ref":            "addSequence: the element the keyword refers to, by qualified name",
	"after":          "the member after which to insert, by qualified name; omitted, at the end of the body",
	"condition":      "addConnection (transition, succession): the guard; addSequence (if, while): the condition",
	"via":            "addSequence (send, accept): the port the message goes through",
	"until":          "addSequence (while): the until condition",
	"parameter":      "addSequence (for): the loop variable",
	"body":           "addSequence: the nested action-body items of an if, for or while, as addSequence operations",
	"elseBody":       "addSequence (if): the items of the else branch, as addSequence operations",
	"layout":         "setLayout: {x, y, width?, height?, collapsed?} in diagram coordinates",
	"route":          "setRoute: the waypoints [{x, y}, ...]",
	"canvas":         "setCanvas: {unit?, width?, height?}",
	"style":          "setStyle: {fill?, line?, text?, font?, fontSize?, bold?, italic?}, colors as #RRGGBB",
}
