package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// The custom methods a diagram client speaks. They are not in the protocol, so
// they are dispatched ahead of the library's own handler.
const (
	// MethodRender renders a view of a document, or an element directly.
	MethodRender = "opensysml/render"
	// MethodViews lists the views a document declares.
	MethodViews = "opensysml/views"
	// MethodRenderChanged tells a client the renderings of a document are out of
	// date. It carries no artifact: the client pulls a fresh one.
	MethodRenderChanged = "opensysml/renderChanged"
)

// CrossDocumentCapability is the experimental capability a client and the server
// each advertise when they speak the cross-document diagram contract: renderings
// naming other documents' declarations and layouts pinned with declaredIn.
const CrossDocumentCapability = "openSysmlCrossDocumentLayout"

// RenderPaletteCapability is the experimental capability the server advertises
// when a render request's palette colours the result's nodes with fill and border.
const RenderPaletteCapability = "openSysmlRenderPalette"

// RenderFormsCapability is the experimental capability whose value lists the
// forms opensysml/render writes, so a client offers exactly those.
const RenderFormsCapability = "openSysmlRenderForms"

// renderFormNames lists the forms in the order the writer defines them.
func renderFormNames() []string {
	names := make([]string, 0, len(view.Forms()))
	for _, form := range view.Forms() {
		names = append(names, string(form))
	}
	return names
}

// renderParams asks for one rendering. View names a view the document declares,
// or a supported pseudo-view (`#<kind>` or `#<kind>:<fqn>`); empty renders the
// document's own view. Form is the artifact written, defaulting to the machine
// form of the rendering's kind. Palette names the palette that fills the nodes by
// keyword family, in the artifact and as each node's Fill and Border; empty is black and white.
type renderParams struct {
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
	View         string                          `json:"view,omitempty"`
	Form         string                          `json:"form,omitempty"`
	Palette      string                          `json:"palette,omitempty"`
}

// renderResult is one rendering: the artifact a client draws, plus the nodes and
// edges it is made of, each located in the source it was declared in.
type renderResult struct {
	View     string        `json:"view"`
	Kind     string        `json:"kind"`
	Stated   string        `json:"stated"`
	Form     string        `json:"form"`
	Artifact string        `json:"artifact"`
	Nodes    []renderNode  `json:"nodes"`
	Edges    []renderEdge  `json:"edges"`
	Rows     []renderRow   `json:"rows,omitempty"`
	Columns  []string      `json:"columns,omitempty"`
	Notices  []string      `json:"notices"`
	Canvas   *renderCanvas `json:"canvas,omitempty"`
	Palette  *editPalette  `json:"palette,omitempty"`
	Version  int           `json:"version"`
}

// renderNode is one node of a rendering, with the range of the declaration it
// was built from when there is one, and its position when a Layout gives one.
// FQN names that declaration the way opensysml/applyModelEdit targets it, and
// Owners the namespaces declaring it, nearest first, drawn or not. Declaration
// stands in for FQN when no qualified name reaches the node: a layout operation
// targets the declaration at that range of the document Origin names. Both are
// given for a declaration of a workspace document alone, a library's being
// beyond every operation; DeclaredHere marks the requested document's own, the
// only ones the operations besides a layout reach. Fill and Border are the
// `#RRGGBB` colours the palette gives the node, as the DOT and PlantUML forms draw it; absent
// for a node left black and white, and for every node when no palette is asked for.
type renderNode struct {
	ID              string          `json:"id"`
	Kind            string          `json:"kind"`
	Name            string          `json:"name"`
	NameSynthesized bool            `json:"nameSynthesized,omitempty"`
	Type            string          `json:"type"`
	Detail          string          `json:"detail"`
	Parent          string          `json:"parent,omitempty"`
	Fill            string          `json:"fill,omitempty"`
	Border          string          `json:"border,omitempty"`
	FQN             string          `json:"fqn,omitempty"`
	DeclaredHere    bool            `json:"declaredHere,omitempty"`
	Notation        string          `json:"notation,omitempty"`
	Owners          []renderOwner   `json:"owners,omitempty"`
	Declaration     *protocol.Range `json:"declaration,omitempty"`
	Origin          *renderOrigin   `json:"origin,omitempty"`
	X               *float64        `json:"x,omitempty"`
	Y               *float64        `json:"y,omitempty"`
	Width           *float64        `json:"width,omitempty"`
	Height          *float64        `json:"height,omitempty"`
	Collapsed       bool            `json:"collapsed,omitempty"`
}

// renderOwner is a namespace declaring a node: its qualified name, and whether
// it is a feature, which an end path chains through with `.` rather than `::`.
type renderOwner struct {
	FQN     string `json:"fqn"`
	Feature bool   `json:"feature"`
}

// renderEdge is one edge of a rendering, located at the connector, transition,
// succession or flow it was written as, with the waypoints a Route gives it.
// FQN and Declaration identify that declaration to a setRoute as a node's do.
type renderEdge struct {
	From        string          `json:"from"`
	To          string          `json:"to"`
	Label       string          `json:"label"`
	Kind        string          `json:"kind"`
	FQN         string          `json:"fqn,omitempty"`
	Declaration *protocol.Range `json:"declaration,omitempty"`
	Origin      *renderOrigin   `json:"origin,omitempty"`
	Route       []renderPoint   `json:"route,omitempty"`
}

// renderPoint is one waypoint of an edge, in the canvas's pixels, y down.
type renderPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// renderCanvas is the drawing surface the view states with a Canvas annotation.
type renderCanvas struct {
	Unit   string   `json:"unit,omitempty"`
	Width  *float64 `json:"width,omitempty"`
	Height *float64 `json:"height,omitempty"`
}

// renderRow is one row of a table rendering, located at the element it reports.
type renderRow struct {
	Cells  []string      `json:"cells"`
	Origin *renderOrigin `json:"origin,omitempty"`
}

// renderOrigin is where an element was declared: Range is the whole declaration,
// SelectionRange the declared identifier alone, which is where a client goes.
// Digest fingerprints the text the ranges are of; an operation naming a range
// of another document hands it back, so a range of text since changed is
// refused rather than misread.
type renderOrigin struct {
	URI            uri.URI         `json:"uri"`
	Range          protocol.Range  `json:"range"`
	SelectionRange *protocol.Range `json:"selectionRange,omitempty"`
	Digest         string          `json:"digest"`
}

// viewsParams asks for the views a document declares.
type viewsParams struct {
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
}

// viewsResult lists a document's views, unsupported ones included so a client
// can say why they cannot be drawn.
type viewsResult struct {
	Views       []viewInfo `json:"views"`
	PseudoViews []string   `json:"pseudoViews"`
}

// viewInfo is one view a document declares. Range is its whole declaration and
// SelectionRange its name, so a client can tell which view the cursor is in.
type viewInfo struct {
	Name           string          `json:"name"`
	Kind           string          `json:"kind"`
	Supported      bool            `json:"supported"`
	Reason         string          `json:"reason,omitempty"`
	Range          *protocol.Range `json:"range,omitempty"`
	SelectionRange *protocol.Range `json:"selectionRange,omitempty"`
}

// renderChangedParams tells a client which document's renderings went stale, and
// at which version.
type renderChangedParams struct {
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
	Version      int                             `json:"version"`
}

// renderHandler dispatches the custom render methods, which the protocol library
// does not know, and passes everything else on.
func (s *Server) renderHandler(inner jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		switch req.Method() {
		case MethodRender:
			var params renderParams
			if err := json.Unmarshal(req.Params(), &params); err != nil {
				return reply(ctx, nil, fmt.Errorf("%s: %w", jsonrpc2.ErrParse, err))
			}
			result, err := s.Render(&params)
			if err != nil {
				return reply(ctx, nil, err)
			}
			return reply(ctx, result, nil)
		case MethodViews:
			var params viewsParams
			if err := json.Unmarshal(req.Params(), &params); err != nil {
				return reply(ctx, nil, fmt.Errorf("%s: %w", jsonrpc2.ErrParse, err))
			}
			return reply(ctx, s.Views(&params), nil)
		case MethodDocuments:
			return reply(ctx, s.Documents(), nil)
		case MethodRenderDocument:
			var params renderDocumentParams
			if err := json.Unmarshal(req.Params(), &params); err != nil {
				return reply(ctx, nil, fmt.Errorf("%s: %w", jsonrpc2.ErrParse, err))
			}
			result, err := s.RenderDocument(&params)
			if err != nil {
				return reply(ctx, nil, err)
			}
			return reply(ctx, result, nil)
		}
		return inner(ctx, reply, req)
	}
}

// Views answers opensysml/views: the views the document declares, with the
// rendering kind each states and why an unsupported one cannot be drawn.
func (s *Server) Views(params *viewsParams) *viewsResult {
	name := uriToName(params.TextDocument.URI)
	out := &viewsResult{Views: []viewInfo{}, PseudoViews: view.PseudoViewSpecs()}
	views, doc := s.ws.Views(name)
	for _, info := range views {
		listed := viewInfo{
			Name:      info.Name,
			Kind:      string(info.Kind),
			Supported: info.Supported,
			Reason:    info.Reason,
		}
		if origin := s.originOf(doc, info.Origin); origin != nil {
			listed.Range = &origin.Range
			listed.SelectionRange = origin.SelectionRange
		}
		out.Views = append(out.Views, listed)
	}
	return out
}

// Render answers opensysml/render: the rendering of the view or element asked
// for, in the form asked for, at the version of the document it was made from:
// version, node FQNs, ranges and digests all come from the one read of the
// workspace the rendering was made under.
func (s *Server) Render(params *renderParams) (*renderResult, error) {
	name := uriToName(params.TextDocument.URI)
	rendering, snapshot, err := s.ws.RenderView(name, params.View)
	if err != nil {
		return nil, err
	}
	doc := snapshot.Rendered
	origin := func(o view.Origin) *renderOrigin { return s.originIn(snapshot, o) }
	form, err := renderForm(rendering, params.Form)
	if err != nil {
		return nil, err
	}
	colors, err := renderPalette(params.Palette)
	if err != nil {
		return nil, err
	}
	artifact, err := rendering.WriteWith(form, view.Options{Palette: colors})
	if err != nil {
		return nil, err
	}
	fills, err := rendering.Fills(colors)
	if err != nil {
		return nil, err
	}
	data := rendering.Data()
	out := &renderResult{
		View:     data.View,
		Kind:     string(data.Kind),
		Stated:   data.Stated,
		Form:     string(form),
		Artifact: artifact,
		Nodes:    make([]renderNode, 0, len(data.Nodes)),
		Edges:    make([]renderEdge, 0, len(data.Edges)),
		Columns:  data.Columns,
		Notices:  data.Notices,
		Palette:  palette(data.Kind, source.KindOf(name)),
		Version:  doc.Version,
	}
	if out.Notices == nil {
		out.Notices = []string{}
	}
	if c := data.Canvas; c != nil {
		out.Canvas = &renderCanvas{Unit: c.Unit}
		if c.HasSize {
			w, h := c.Width, c.Height
			out.Canvas.Width, out.Canvas.Height = &w, &h
		}
	}
	s.renderNodes(out, snapshot, data.Nodes, fills)
	s.renderEdges(out, snapshot, data.Edges)
	for _, row := range data.Rows {
		out.Rows = append(out.Rows, renderRow{Cells: row.Cells, Origin: origin(row.Origin)})
	}
	return out, nil
}

// renderNodes converts the rendering's nodes into out, each coloured as fills
// says; a node the document declares confines the palette to its notation and
// is admitted once all are known.
func (s *Server) renderNodes(out *renderResult, snapshot *model.Snapshot, nodes []view.NodeData, fills map[string]view.Fill) {
	name := snapshot.Rendered.Name
	var declared []declaredNode
	for _, node := range nodes {
		n := renderNode{
			ID:              node.ID,
			Kind:            node.Kind,
			Name:            node.Name,
			NameSynthesized: node.NameSynthesized,
			Type:            node.Type,
			Detail:          node.Detail,
			Parent:          node.Parent,
			Fill:            fills[node.ID].Fill,
			Border:          fills[node.ID].Border,
		}
		declaring := s.declaring(snapshot, node.Origin)
		n.Origin = s.originOf(declaring, node.Origin)
		if sym := nodeSymbol(s.targetable(snapshot, declaring), node.Origin); sym != nil {
			if owners, ok := nodeOwners(sym); ok {
				n.FQN = notationName(sym)
				n.Notation = sym.Notation()
				n.Owners = owners
				n.DeclaredHere = declaring.Name == name
				if out.Palette != nil && n.DeclaredHere {
					out.Palette.confine(n.Notation)
					declared = append(declared, declaredNode{node.ID, sym.Decl})
				}
			} else {
				decl := positionsOf(declaring).rangeOf(sym.DeclSpan)
				n.Declaration = &decl
			}
		}
		if g := node.Geometry; g != nil {
			x, y := g.X, g.Y
			n.X, n.Y, n.Collapsed = &x, &y, g.Collapsed
			if g.HasSize {
				w, h := g.Width, g.Height
				n.Width, n.Height = &w, &h
			}
		}
		out.Nodes = append(out.Nodes, n)
	}
	for _, d := range declared {
		out.Palette.admit(d.id, d.decl)
	}
}

// renderEdges converts the rendering's edges into out, each with its route and
// the FQN or declaration range of the element it comes from.
func (s *Server) renderEdges(out *renderResult, snapshot *model.Snapshot, edges []view.EdgeData) {
	for _, edge := range edges {
		e := renderEdge{
			From:  edge.From,
			To:    edge.To,
			Label: edge.Label,
			Kind:  edge.Kind.String(),
		}
		declaring := s.declaring(snapshot, edge.Origin)
		e.Origin = s.originOf(declaring, edge.Origin)
		if sym := nodeSymbol(s.targetable(snapshot, declaring), edge.Origin); sym != nil {
			if _, ok := nodeOwners(sym); ok {
				e.FQN = notationName(sym)
			} else {
				decl := positionsOf(declaring).rangeOf(sym.DeclSpan)
				e.Declaration = &decl
			}
		}
		for _, p := range edge.Route {
			e.Route = append(e.Route, renderPoint{X: p.X, Y: p.Y})
		}
		out.Edges = append(out.Edges, e)
	}
}

// renderForm is the form to write: the one asked for, else the machine form of
// the rendering's kind. A form the writer does not know is refused rather than
// silently replaced.
func renderForm(rendering *view.Rendering, asked string) (view.Form, error) {
	if asked == "" {
		return rendering.Kind.MachineForm(), nil
	}
	form := view.Form(asked)
	if slices.Contains(view.Forms(), form) {
		return form, nil
	}
	names := make([]string, 0, len(view.Forms()))
	for _, form := range view.Forms() {
		names = append(names, strconv.Quote(string(form)))
	}
	return "", fmt.Errorf("%q is no rendering form: write %s or %s", asked, strings.Join(names[:len(names)-1], ", "), names[len(names)-1])
}

// renderPalette is the palette a request names, none when it names none, and
// an error listing the palettes there are when it names something else.
func renderPalette(asked string) (view.Palette, error) {
	if asked == "" {
		return "", nil
	}
	palette, ok := view.ParsePalette(asked)
	if !ok {
		return "", &view.UnknownPaletteError{Name: asked}
	}
	return palette, nil
}

// declaring is the document an origin is located in, as the rendering read it:
// the snapshot's document of that name, else the bundled library file of that
// name, which is never rewritten; nil for an origin with no locatable
// declaration or in a document the session does not hold.
func (s *Server) declaring(snapshot *model.Snapshot, o view.Origin) *model.Document {
	if !o.Located() {
		return nil
	}
	if doc := snapshot.Document(o.Doc); doc != nil {
		return doc
	}
	return s.ws.LibraryDocument(o.Doc)
}

// targetable is declaring when the client may target its declarations: not a
// bundled library file, and not another document than the rendered one for a
// client that would place its declarations by name alone, unpinned.
func (s *Server) targetable(snapshot *model.Snapshot, declaring *model.Document) *model.Document {
	if declaring == nil || s.ws.IsLibraryDocument(declaring.Name) {
		return nil
	}
	if declaring.Name != snapshot.Rendered.Name && !s.clientSpeaksCrossDocument() {
		return nil
	}
	return declaring
}

// originIn is a core origin as a client navigates to it, placed in the text of
// the document declaring it as the rendering read it.
func (s *Server) originIn(snapshot *model.Snapshot, o view.Origin) *renderOrigin {
	return s.originOf(s.declaring(snapshot, o), o)
}

// originOf places o in doc, the document declaring it; a standard library
// declaration is located in its sysml-stdlib document. Nil for no document.
func (s *Server) originOf(doc *model.Document, o view.Origin) *renderOrigin {
	if doc == nil {
		return nil
	}
	pos := positionsOf(doc)
	out := &renderOrigin{URI: s.documentURI(o.Doc), Range: pos.rangeOf(o.Span), Digest: doc.Digest()}
	if o.Name.Len > 0 {
		name := pos.rangeOf(o.Name)
		out.SelectionRange = &name
	}
	return out
}

// queueRenderChanged tells the client the renderings of a document are stale,
// once an editor burst settles: a redraw per keystroke is a rendering of text
// that has already been superseded. It is debounced on the window the
// cross-document sweep uses, and so always follows the diagnostics of the
// analysis it reports.
func (s *Server) queueRenderChanged(ctx context.Context, name string) {
	if s.notifier == nil {
		return
	}
	if s.renderNotify == nil {
		s.notifyRenderChanged(ctx, name)
		return
	}
	// The notification outlives the request whose context is cancelled on return.
	ctx = context.WithoutCancel(ctx)
	s.renderNotify.Trigger(name, func() { s.notifyRenderChanged(ctx, name) })
}

// notifyRenderChanged sends the notification, carrying no artifact: the client
// pulls a fresh one, so nothing is rendered for a hidden panel.
func (s *Server) notifyRenderChanged(ctx context.Context, name string) {
	notifier := s.notifier
	if notifier == nil {
		return
	}
	version := 0
	if doc := s.ws.Document(name); doc != nil {
		version = doc.Version
	}
	// Best-effort push, like diagnostics: a failed notification has no recovery
	// path here, and the client re-renders on the next edit.
	_ = notifier.Notify(ctx, MethodRenderChanged, &renderChangedParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: nameToURI(name)},
		Version:      version,
	})
}
