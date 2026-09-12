package lsp

import (
	"context"
	"encoding/json"
	"fmt"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/view"
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

// renderParams asks for one rendering. View names a view the document declares,
// or a supported pseudo-view (`#<kind>` or `#<kind>:<fqn>`); empty renders the
// document's own view. Form is the artifact written, defaulting to the machine
// form of the rendering's kind.
type renderParams struct {
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
	View         string                          `json:"view,omitempty"`
	Form         string                          `json:"form,omitempty"`
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
	Version  int           `json:"version"`
}

// renderNode is one node of a rendering, with the range of the declaration it
// was built from when there is one, and its position when a Layout gives one.
type renderNode struct {
	ID        string        `json:"id"`
	Kind      string        `json:"kind"`
	Name      string        `json:"name"`
	Detail    string        `json:"detail"`
	Parent    string        `json:"parent,omitempty"`
	Origin    *renderOrigin `json:"origin,omitempty"`
	X         *float64      `json:"x,omitempty"`
	Y         *float64      `json:"y,omitempty"`
	Width     *float64      `json:"width,omitempty"`
	Height    *float64      `json:"height,omitempty"`
	Collapsed bool          `json:"collapsed,omitempty"`
}

// renderEdge is one edge of a rendering, located at the connector, transition,
// succession or flow it was written as, with the waypoints a Route gives it.
type renderEdge struct {
	From   string        `json:"from"`
	To     string        `json:"to"`
	Label  string        `json:"label"`
	Kind   string        `json:"kind"`
	Origin *renderOrigin `json:"origin,omitempty"`
	Route  []renderPoint `json:"route,omitempty"`
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
type renderOrigin struct {
	URI            uri.URI         `json:"uri"`
	Range          protocol.Range  `json:"range"`
	SelectionRange *protocol.Range `json:"selectionRange,omitempty"`
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

// viewInfo is one view a document declares.
type viewInfo struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Supported bool   `json:"supported"`
	Reason    string `json:"reason,omitempty"`
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
	for _, info := range s.ws.Views(name) {
		out.Views = append(out.Views, viewInfo{
			Name:      info.Name,
			Kind:      string(info.Kind),
			Supported: info.Supported,
			Reason:    info.Reason,
		})
	}
	return out
}

// Render answers opensysml/render: the rendering of the view or element asked
// for, in the form asked for, at the version of the document it was made from.
func (s *Server) Render(params *renderParams) (*renderResult, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.ws.Document(name)
	if doc == nil {
		return nil, fmt.Errorf("%s: no such document", name)
	}
	rendering, err := s.ws.RenderView(name, params.View)
	if err != nil {
		return nil, err
	}
	form, err := renderForm(rendering, params.Form)
	if err != nil {
		return nil, err
	}
	artifact, err := rendering.Write(form)
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
	for _, node := range data.Nodes {
		n := renderNode{
			ID:     node.ID,
			Kind:   node.Kind,
			Name:   node.Name,
			Detail: node.Detail,
			Parent: node.Parent,
			Origin: s.origin(node.Origin),
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
	for _, edge := range data.Edges {
		e := renderEdge{
			From:   edge.From,
			To:     edge.To,
			Label:  edge.Label,
			Kind:   edge.Kind.String(),
			Origin: s.origin(edge.Origin),
		}
		for _, p := range edge.Route {
			e.Route = append(e.Route, renderPoint{X: p.X, Y: p.Y})
		}
		out.Edges = append(out.Edges, e)
	}
	for _, row := range data.Rows {
		out.Rows = append(out.Rows, renderRow{Cells: row.Cells, Origin: s.origin(row.Origin)})
	}
	return out, nil
}

// renderForm is the form to write: the one asked for, else the machine form of
// the rendering's kind. A form the writer does not know is refused rather than
// silently replaced.
func renderForm(rendering *view.Rendering, asked string) (view.Form, error) {
	if asked == "" {
		return rendering.Kind.MachineForm(), nil
	}
	form := view.Form(asked)
	switch form {
	case view.FormText, view.FormMermaid, view.FormMarkdown:
		return form, nil
	}
	return "", fmt.Errorf("%q is no rendering form: write %q, %q or %q", asked, view.FormMermaid, view.FormText, view.FormMarkdown)
}

// origin is a core origin as a client navigates to it, nil for an element with
// no locatable declaration and for one declared in a document the session does
// not hold. A standard library declaration is located in its sysml-stdlib
// document.
func (s *Server) origin(o view.Origin) *renderOrigin {
	if !o.Located() {
		return nil
	}
	doc := s.document(o.Doc)
	if doc == nil {
		return nil
	}
	out := &renderOrigin{URI: s.documentURI(o.Doc), Range: spanToRange(doc.Content, o.Span)}
	if o.Name.Len > 0 {
		name := spanToRange(doc.Content, o.Name)
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
