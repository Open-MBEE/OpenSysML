package lsp

import (
	"context"
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/modelrt"
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

// RenderStylesCapability is the experimental capability whose value lists the
// drawing styles a render request's `style` draws the DOT or Mermaid form in, the first
// the default; a server without it draws the Pilot look alone.
const RenderStylesCapability = "openSysmlRenderStyles"

// RenderPortsCapability is the experimental capability whose value lists the
// port displays a render request's `ports` draws an interconnection or mixed
// rendering's parts with, the first the default; a server without it draws
// every port.
const RenderPortsCapability = "openSysmlRenderPorts"

// RenderLinesCapability is the experimental capability whose value lists the
// edge routings a render request's `lines` draws the DOT or PlantUML form with,
// the first the default; a server without it routes every edge as splines.
const RenderLinesCapability = "openSysmlRenderLines"

// RenderCompartmentsCapability is the experimental capability whose value lists
// the foldings a render request's `compartments` folds a tree's owned members
// into compartment rows with, the first the default; a server without it folds
// only the layout-less members of a positioned tree.
const RenderCompartmentsCapability = "openSysmlRenderCompartments"

// RenderElementsCapability is the experimental capability whose value lists the
// optional notation elements a render request's `show` names, in the order the
// writer defines them, then "all", every one of them.
const RenderElementsCapability = "openSysmlRenderElements"

// RenderDepthCapability is the experimental capability a server advertises when
// a render request's depth bounds how many levels are drawn below each exposed element.
const RenderDepthCapability = "openSysmlRenderDepth"

// RenderIncludeCapability is the capability a server advertises when a render
// request's `include` draws elements besides the view's own content, marking
// each node drawn that way `included`.
const RenderIncludeCapability = "openSysmlRenderInclude"

// renderElementNames lists the optional notation elements, then "all".
func renderElementNames() []string {
	names := make([]string, 0, len(view.Elements())+1)
	for _, element := range view.Elements() {
		names = append(names, string(element))
	}
	return append(names, view.ElementAll)
}

// RenderOverlaysCapability is the experimental capability whose value lists the
// overlays a render request's `overlay` draws over a requirement rendering.
const RenderOverlaysCapability = "openSysmlRenderOverlays"

// renderOverlayNames lists the overlays in the order the writer defines them.
func renderOverlayNames() []string {
	names := make([]string, 0, len(view.Overlays()))
	for _, overlay := range view.Overlays() {
		names = append(names, string(overlay))
	}
	return names
}

// renderCompartmentsNames lists the compartment foldings in the order the writer defines them.
func renderCompartmentsNames() []string {
	names := make([]string, 0, len(view.CompartmentsChoices()))
	for _, compartments := range view.CompartmentsChoices() {
		names = append(names, string(compartments))
	}
	return names
}

// renderLinesNames lists the edge routings in the order the writer defines them.
func renderLinesNames() []string {
	names := make([]string, 0, len(view.LinesChoices()))
	for _, lines := range view.LinesChoices() {
		names = append(names, string(lines))
	}
	return names
}

// renderPortsNames lists the port displays in the order the writer defines them.
func renderPortsNames() []string {
	names := make([]string, 0, len(view.PortsChoices()))
	for _, ports := range view.PortsChoices() {
		names = append(names, string(ports))
	}
	return names
}

// renderStyleNames lists the drawing styles in the order the writer defines them.
func renderStyleNames() []string {
	names := make([]string, 0, len(view.DrawingStyles()))
	for _, style := range view.DrawingStyles() {
		names = append(names, string(style))
	}
	return names
}

// renderFormNames lists the forms a view is written in, in the order the writer defines them.
func renderFormNames() []string {
	names := make([]string, 0, len(view.ViewForms()))
	for _, form := range view.ViewForms() {
		names = append(names, string(form))
	}
	return names
}

// RenderParams asks for one rendering. View names a view the document declares,
// or a supported pseudo-view (`#<kind>` or `#<kind>:<fqn>`); empty renders the
// document's own view. Form is the artifact written, defaulting to the machine
// form of the rendering's kind. Palette names the palette that fills the nodes by
// keyword family, in the artifact and as each node's Fill and Border; empty is black and white.
// Style names the drawing style the DOT and Mermaid forms draw in; empty is the default, pilot.
// Ports names how much of a part's ports an interconnection or mixed rendering
// draws; empty is the default, minimal. Lines names how the DOT and PlantUML
// forms route their edges; empty is the default, splines. Show names the
// optional notation elements drawn besides the ones the view's rendering
// shows, or "all".
type RenderParams struct {
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
	View         string                          `json:"view,omitempty"`
	Form         string                          `json:"form,omitempty"`
	Palette      string                          `json:"palette,omitempty"`
	Style        string                          `json:"style,omitempty"`
	Ports        string                          `json:"ports,omitempty"`
	Lines        string                          `json:"lines,omitempty"`
	Compartments string                          `json:"compartments,omitempty"`
	Show         []string                        `json:"show,omitempty"`
	Depth        int                             `json:"depth,omitzero"`
	// Include names elements drawn besides what the view exposes, each by
	// qualified name, as if the view exposed them; nothing in the model changes.
	Include      []string `json:"include,omitempty"`
	Overlay      string   `json:"overlay,omitempty"`
	LinkTemplate string   `json:"linkTemplate,omitempty"`
}

// RenderResult is one rendering: the artifact a client draws, plus the nodes and
// edges it is made of, each located in the source it was declared in. Style is
// the drawing style the artifact was drawn in, the default named.
type RenderResult struct {
	View     string        `json:"view"`
	Kind     string        `json:"kind"`
	Style    string        `json:"style"`
	Stated   string        `json:"stated"`
	Shown    []string      `json:"shown"`
	Form     string        `json:"form"`
	Artifact string        `json:"artifact"`
	Nodes    []RenderNode  `json:"nodes"`
	Edges    []RenderEdge  `json:"edges"`
	Rows     []RenderRow   `json:"rows,omitempty"`
	Columns  []string      `json:"columns,omitempty"`
	Notices  []string      `json:"notices"`
	Canvas   *RenderCanvas `json:"canvas,omitzero"`
	Palette  *EditPalette  `json:"palette,omitzero"`
	Version  int           `json:"version"`
}

// RenderNode is one node of a rendering, with the range of the declaration it
// was built from when there is one, and its position when a Layout gives one.
// FQN names that declaration the way opensysml/applyModelEdit targets it, and
// Owners the namespaces declaring it, nearest first, drawn or not. Declaration
// stands in for FQN when no qualified name reaches the node: a layout operation
// targets the declaration at that range of the document Origin names. Both are
// given for a declaration of a workspace document alone, a library's being
// beyond every operation; DeclaredHere marks the requested document's own, the
// only ones the operations besides a layout reach. Fill and Border are the
// `#RRGGBB` colours the palette gives the node, as the DOT, Mermaid, PlantUML and D2 forms draw it; absent
// for a node left black and white, and for every node when no palette is asked for.
// Style is the node's own Style annotation, which wins over the palette and the drawing style.
type RenderNode struct {
	ID              string          `json:"id"`
	Kind            string          `json:"kind"`
	Name            string          `json:"name"`
	NameSynthesized bool            `json:"nameSynthesized,omitzero"`
	Inherited       bool            `json:"inherited,omitzero"`
	Type            string          `json:"type"`
	Detail          string          `json:"detail"`
	Parent          string          `json:"parent,omitempty"`
	Compartment     string          `json:"compartment,omitempty"`
	Fill            string          `json:"fill,omitempty"`
	Border          string          `json:"border,omitempty"`
	Style           *RenderStyle    `json:"style,omitzero"`
	Ports           []RenderPort    `json:"ports,omitempty"`
	FQN             string          `json:"fqn,omitempty"`
	DeclaredHere    bool            `json:"declaredHere,omitzero"`
	Notation        string          `json:"notation,omitempty"`
	Owners          []RenderOwner   `json:"owners,omitempty"`
	Declaration     *protocol.Range `json:"declaration,omitempty"`
	Origin          *RenderOrigin   `json:"origin,omitzero"`
	X               *float64        `json:"x,omitzero"`
	Y               *float64        `json:"y,omitzero"`
	Width           *float64        `json:"width,omitzero"`
	Height          *float64        `json:"height,omitzero"`
	Collapsed       bool            `json:"collapsed,omitzero"`
	Verdict         string          `json:"verdict,omitempty"`
	// Included marks a node the request's `include` drew rather than the view.
	Included bool `json:"included,omitzero"`
}

// RenderOwner is a namespace declaring a node: its qualified name, and whether
// it is a feature, which an end path chains through with `.` rather than `::`.
type RenderOwner struct {
	FQN     string `json:"fqn"`
	Feature bool   `json:"feature"`
}

type RenderPort struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`
	Direction string `json:"direction,omitempty"`
	Inherited bool   `json:"inherited,omitzero"`
}

// RenderEdge is one edge of a rendering, located at the connector, transition,
// succession or flow it was written as, with the waypoints a Route gives it.
// FQN and Declaration identify that declaration to a setRoute as a node's do.
// Style is the edge's own Style annotation.
type RenderEdge struct {
	From        string          `json:"from"`
	To          string          `json:"to"`
	Label       string          `json:"label"`
	Kind        string          `json:"kind"`
	FromPort    string          `json:"fromPort,omitempty"`
	ToPort      string          `json:"toPort,omitempty"`
	Style       *RenderStyle    `json:"style,omitzero"`
	FQN         string          `json:"fqn,omitempty"`
	Declaration *protocol.Range `json:"declaration,omitempty"`
	Origin      *RenderOrigin   `json:"origin,omitzero"`
	Route       []RenderPoint   `json:"route,omitempty"`
}

// RenderPoint is one waypoint of an edge, in the canvas's pixels, y down.
type RenderPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// RenderCanvas is the drawing surface the view states with a Canvas annotation.
type RenderCanvas struct {
	Unit   string   `json:"unit,omitempty"`
	Width  *float64 `json:"width,omitzero"`
	Height *float64 `json:"height,omitzero"`
}

// RenderStyle is how a node or edge is drawn where a Style annotation says so:
// `#RRGGBB` colours of fill, line and text, and the face, size in points and
// weight of its text; each empty or zero when unstated.
type RenderStyle struct {
	Fill     string  `json:"fill,omitempty"`
	Line     string  `json:"line,omitempty"`
	Text     string  `json:"text,omitempty"`
	Font     string  `json:"font,omitempty"`
	FontSize float64 `json:"fontSize,omitzero"`
	Bold     bool    `json:"bold,omitzero"`
	Italic   bool    `json:"italic,omitzero"`
}

// RenderRow is one row of a table rendering, located at the element it reports.
type RenderRow struct {
	Cells  []string      `json:"cells"`
	Origin *RenderOrigin `json:"origin,omitzero"`
}

// RenderOrigin is where an element was declared: Range is the whole declaration,
// SelectionRange the declared identifier alone, which is where a client goes.
// Digest fingerprints the text the ranges are of; an operation naming a range
// of another document hands it back, so a range of text since changed is
// refused rather than misread.
type RenderOrigin struct {
	URI            uri.URI         `json:"uri"`
	Range          protocol.Range  `json:"range"`
	SelectionRange *protocol.Range `json:"selectionRange,omitempty"`
	Digest         string          `json:"digest"`
}

// ViewsParams asks for the views a document declares.
type ViewsParams struct {
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
}

// ViewsResult lists a document's views, unsupported ones included so a client
// can say why they cannot be drawn.
type ViewsResult struct {
	Views       []ViewInfo `json:"views"`
	PseudoViews []string   `json:"pseudoViews"`
}

// ViewInfo is one view a document declares. Range is its whole declaration and
// SelectionRange its name, so a client can tell which view the cursor is in.
type ViewInfo struct {
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
	return func(ctx context.Context, req *jsonrpc2.Request) (any, error) {
		switch req.Method() {
		case MethodRender:
			params, err := decodeParams[RenderParams](req)
			if err != nil {
				return nil, err
			}
			return s.Render(params)
		case MethodViews:
			params, err := decodeParams[ViewsParams](req)
			if err != nil {
				return nil, err
			}
			return s.Views(params), nil
		case MethodRelatedElements:
			params, err := decodeParams[RelatedElementsParams](req)
			if err != nil {
				return nil, err
			}
			return s.RelatedElements(params)
		case MethodDocuments:
			return s.Documents(), nil
		case MethodRenderDocument:
			params, err := decodeParams[RenderDocumentParams](req)
			if err != nil {
				return nil, err
			}
			return s.RenderDocument(params)
		}
		return inner(ctx, req)
	}
}

// Views answers opensysml/views: the views the document declares, with the
// rendering kind each states and why an unsupported one cannot be drawn.
func (s *Server) Views(params *ViewsParams) *ViewsResult {
	name := uriToName(params.TextDocument.URI)
	out := &ViewsResult{Views: []ViewInfo{}, PseudoViews: view.PseudoViewSpecs()}
	views, doc := s.ws.Views(name)
	for _, info := range views {
		listed := ViewInfo{
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
func (s *Server) Render(params *RenderParams) (*RenderResult, error) {
	name := uriToName(params.TextDocument.URI)
	overlay, ok := view.ParseOverlay(params.Overlay)
	if !ok {
		return nil, fmt.Errorf("unknown overlay %q; overlay takes %s", params.Overlay, view.OverlayNames())
	}
	var (
		rendering  *view.Rendering
		snapshot   *model.Snapshot
		firstError *diag.Diagnostic
	)
	if err := s.ws.Read(func(r *model.Reading) (err error) {
		var verdicts view.Verdicts
		if overlay == view.OverlayVerdicts {
			rt, err := modelrt.New(r)
			if err != nil {
				return err
			}
			verdicts = rt.RequirementVerdicts()
		}
		rendering, snapshot, err = r.RenderOverlaidView(name, params.View, overlay, verdicts, params.Depth, params.Include)
		if err == nil && (params.LinkTemplate != "" || view.Form(params.Form) == view.FormDrawio) {
			r.LinkSites(rendering, snapshot)
		}
		if err == nil {
			firstError = firstBlocking(r.Diagnostics(name))
		}
		return err
	}); err != nil {
		return nil, err
	}
	doc := snapshot.Rendered
	origin := func(o view.Origin) *RenderOrigin { return s.originIn(snapshot, o) }
	form, err := renderForm(rendering, params.Form)
	if err != nil {
		return nil, err
	}
	colors, err := renderPalette(params.Palette)
	if err != nil {
		return nil, err
	}
	style, err := renderDrawingStyle(params.Style)
	if err != nil {
		return nil, err
	}
	ports, err := renderPorts(params.Ports)
	if err != nil {
		return nil, err
	}
	lines, err := renderLines(params.Lines)
	if err != nil {
		return nil, err
	}
	compartments, err := renderCompartments(params.Compartments)
	if err != nil {
		return nil, err
	}
	shown, err := renderShown(params.Show)
	if err != nil {
		return nil, err
	}
	if params.Depth < 0 {
		return nil, &view.DepthError{Value: fmt.Sprint(params.Depth)}
	}
	artifact, err := rendering.WriteWith(form, view.Options{
		Palette: colors, Style: style, Ports: ports, Lines: lines, Compartments: compartments, Show: shown,
		Links: view.Links{Template: params.LinkTemplate, Sites: snapshot.Sites()},
	})
	if err != nil {
		return nil, err
	}
	fills, err := rendering.Fills(colors)
	if err != nil {
		return nil, err
	}
	data := rendering.DataWith(view.Options{Ports: ports, Compartments: compartments, Show: shown})
	out := &RenderResult{
		View:     data.View,
		Kind:     string(data.Kind),
		Style:    string(style),
		Stated:   data.Stated,
		Shown:    data.Shown,
		Form:     string(form),
		Artifact: artifact,
		Nodes:    make([]RenderNode, 0, len(data.Nodes)),
		Edges:    make([]RenderEdge, 0, len(data.Edges)),
		Columns:  data.Columns,
		Notices:  data.Notices,
		Palette:  palette(data.Kind, source.KindOf(name)),
		Version:  doc.Version,
	}
	if out.Notices == nil {
		out.Notices = []string{}
	}
	if c := data.Canvas; c != nil {
		out.Canvas = &RenderCanvas{Unit: c.Unit}
		if c.HasSize {
			w, h := c.Width, c.Height
			out.Canvas.Width, out.Canvas.Height = &w, &h
		}
	}
	s.renderNodes(out, snapshot, data.Nodes, fills, rendering.Included)
	s.renderEdges(out, snapshot, data.Edges)
	for _, row := range data.Rows {
		out.Rows = append(out.Rows, RenderRow{Cells: row.Cells, Origin: origin(row.Origin)})
	}
	if len(out.Nodes) == 0 && len(out.Edges) == 0 && len(out.Rows) == 0 && firstError != nil {
		out.Notices = append([]string{analysisNotice(name, doc, *firstError)}, out.Notices...)
	}
	return out, nil
}

// firstBlocking is the first error among the diagnostics, nil when none blocks.
func firstBlocking(diagnostics []diag.Diagnostic) *diag.Diagnostic {
	for i := range diagnostics {
		if diagnostics[i].Blocking() {
			return &diagnostics[i]
		}
	}
	return nil
}

// analysisNotice is what a rendering with nothing to draw says when the document
// it was made from has an error: the error itself, in the words the CLI uses,
// since an element the parser could not read is an element no view exposes.
// The diagnostic and doc are of one reading, so the position is of the text rendered.
func analysisNotice(name string, doc *model.Document, d diag.Diagnostic) string {
	at := spanToRange(doc.Content, d.Span).Start
	return fmt.Sprintf("%s did not analyse cleanly; nothing was rendered: %d:%d: %s",
		filepath.Base(name), at.Line+1, at.Character+1, d.Message)
}

// renderNodes converts the rendering's nodes into out, each coloured as fills
// says; a node the document declares confines the palette to its notation and
// is admitted once all are known.
func (s *Server) renderNodes(out *RenderResult, snapshot *model.Snapshot, nodes []view.NodeData, fills map[string]view.Fill, included []view.Origin) {
	name := snapshot.Rendered.Name
	var declared []declaredNode
	for _, node := range nodes {
		n := RenderNode{
			ID:              node.ID,
			Kind:            node.Kind,
			Name:            node.Name,
			NameSynthesized: node.NameSynthesized,
			Inherited:       node.Inherited,
			Type:            node.Type,
			Detail:          node.Detail,
			Parent:          node.Parent,
			Compartment:     node.Compartment,
			Fill:            fills[node.ID].Fill,
			Border:          fills[node.ID].Border,
			Style:           wireStyle(node.Style),
			Verdict:         node.Verdict,
			Included:        node.Parent == "" && slices.Contains(included, node.Origin),
		}
		for _, port := range node.Ports {
			n.Ports = append(n.Ports, RenderPort{
				ID: port.ID, Name: port.Name, Type: port.Type, Direction: port.Direction.String(), Inherited: port.Inherited,
			})
		}
		if node.Style != nil {
			if node.Style.Fill != "" {
				n.Fill = node.Style.Fill
			}
			if node.Style.Line != "" {
				n.Border = node.Style.Line
			}
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
			n.Collapsed = g.Collapsed
		}
		if g := node.Geometry; g != nil && !g.Unplaced {
			x, y := g.X, g.Y
			n.X, n.Y = &x, &y
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
func (s *Server) renderEdges(out *RenderResult, snapshot *model.Snapshot, edges []view.EdgeData) {
	for _, edge := range edges {
		e := RenderEdge{
			From: edge.From, To: edge.To, FromPort: edge.FromPort, ToPort: edge.ToPort,
			Label: edge.Text(), Kind: edge.Kind.String(), Style: wireStyle(edge.Style),
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
			e.Route = append(e.Route, RenderPoint{X: p.X, Y: p.Y})
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
	if slices.Contains(view.ViewForms(), form) {
		return form, nil
	}
	names := make([]string, 0, len(view.ViewForms()))
	for _, form := range view.ViewForms() {
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

// renderDrawingStyle is the drawing style a request names, the default when it
// names none, and an error listing the styles there are when it names something else.
func renderDrawingStyle(asked string) (view.DrawingStyle, error) {
	style, ok := view.ParseDrawingStyle(asked)
	if !ok {
		return "", &view.UnknownDrawingStyleError{Name: asked}
	}
	return style, nil
}

// renderPorts is the port display a request names, the default when it names
// none, and an error listing the displays there are when it names something else.
func renderPorts(asked string) (view.Ports, error) {
	ports, ok := view.ParsePorts(asked)
	if !ok {
		return "", &view.UnknownPortsError{Name: asked}
	}
	return ports, nil
}

// renderCompartments is the compartment folding a request names, the default
// when it names none, and an error listing the foldings when it names something else.
func renderCompartments(asked string) (view.Compartments, error) {
	compartments, ok := view.ParseCompartments(asked)
	if !ok {
		return "", &view.UnknownCompartmentsError{Name: asked}
	}
	return compartments, nil
}

// renderLines is the edge routing a request names, the default when it names
// none, and an error listing the routings there are when it names something else.
func renderLines(asked string) (view.Lines, error) {
	lines, ok := view.ParseLines(asked)
	if !ok {
		return "", &view.UnknownLinesError{Name: asked}
	}
	return lines, nil
}

// renderShown is the optional notation elements a request's show names,
// each of which must be one there is; nil when it names none.
func renderShown(asked []string) (view.Shown, error) {
	return view.ShownOf(asked)
}

// wireStyle is a node's or edge's Style as the wire carries it; nil for none.
func wireStyle(style *view.Style) *RenderStyle {
	if style == nil {
		return nil
	}
	return &RenderStyle{Fill: style.Fill, Line: style.Line, Text: style.Text, Font: style.Font, FontSize: style.FontSize, Bold: style.Bold, Italic: style.Italic}
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
func (s *Server) originIn(snapshot *model.Snapshot, o view.Origin) *RenderOrigin {
	return s.originOf(s.declaring(snapshot, o), o)
}

// originOf places o in doc, the document declaring it; a standard library
// declaration is located in its sysml-stdlib document. Nil for no document.
func (s *Server) originOf(doc *model.Document, o view.Origin) *RenderOrigin {
	if doc == nil {
		return nil
	}
	pos := positionsOf(doc)
	out := &RenderOrigin{URI: s.documentURI(o.Doc), Range: pos.rangeOf(o.Span), Digest: doc.Digest()}
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
	// The notification outlives the request, whose context is recycled on return.
	ctx = jsonrpc2.DetachContext(ctx)
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
