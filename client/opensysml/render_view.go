package opensysml

import (
	"context"
	"fmt"
	"math"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// RenderViewOption configures a view rendering.
type RenderViewOption func(*renderViewOptions)

type renderViewOptions struct {
	ports        string
	show         []string
	depth        int
	compartments string
	form         string
	palette      string
	style        string
	lines        string
	direction    string
	link         string
	unplaced     string
}

// written reports whether a form or an option only a written form reads is
// set, which a service without CapabilityRenderViewForm would drop unanswered.
func (o *renderViewOptions) written() bool {
	return o.form != "" || o.palette != "" || o.style != "" || o.lines != "" ||
		o.direction != "" || o.link != "" || o.unplaced != ""
}

// The forms WithForm takes, as the CLI's -render-form names them.
const (
	ViewFormText     = "text"
	ViewFormMermaid  = "mermaid"
	ViewFormMarkdown = "markdown"
	ViewFormDOT      = "dot"
	ViewFormPlantUML = "plantuml"
	ViewFormD2       = "d2"
	ViewFormDrawio   = "drawio"
	ViewFormCSV      = "csv"
	ViewFormTSV      = "tsv"
)

// ViewForms lists the forms WithForm takes, in the order the CLI offers them.
func ViewForms() []string {
	return []string{
		ViewFormText, ViewFormMermaid, ViewFormMarkdown, ViewFormDOT, ViewFormPlantUML,
		ViewFormD2, ViewFormDrawio, ViewFormCSV, ViewFormTSV,
	}
}

// WithForm also writes the rendering in form into RenderedView.Text, as `sysml -render-form` writes it.
// An unknown form, or one the view's kind is not written in, is refused; requires CapabilityRenderViewForm.
func WithForm(form string) RenderViewOption {
	return func(options *renderViewOptions) { options.form = form }
}

// WithPalette fills the nodes of a graph form from the named palette, as
// -render-palette names them; none draws in black and white.
func WithPalette(palette string) RenderViewOption {
	return func(options *renderViewOptions) { options.palette = palette }
}

// WithStyle draws the DOT or Mermaid form in the named style, as
// -render-style names them: "pilot" (the default) or "cameo".
func WithStyle(style string) RenderViewOption {
	return func(options *renderViewOptions) { options.style = style }
}

// WithLines routes the edges of the DOT or PlantUML form, as -render-lines
// names the routings: "splines" (the default), "ortho" or "polyline".
func WithLines(lines string) RenderViewOption {
	return func(options *renderViewOptions) { options.lines = lines }
}

// WithDirection draws a graph-shaped form in the flow direction "TB", "LR",
// "RL" or "BT"; none leaves each kind's default, which is what the CLI writes.
func WithDirection(direction string) RenderViewOption {
	return func(options *renderViewOptions) { options.direction = direction }
}

// WithLink links each node and edge of the form to its source through template, as
// -render-link takes it: placeholders {file}, {line}, {col}, {qname} and {id}.
func WithLink(template string) RenderViewOption {
	return func(options *renderViewOptions) { options.link = template }
}

// WithUnplaced says where a graph form of a positioned view puts the nodes no
// layout places, as -render-unplaced names it: "omit" (the default) or "strip".
func WithUnplaced(placement string) RenderViewOption {
	return func(options *renderViewOptions) { options.unplaced = placement }
}

// WithCompartments folds a tree rendering's owned members into compartment
// rows of their owner: "positioned" (the default) folds only the layout-less
// members of a tree some Layout positions, "most" every owned usage no Layout,
// nested definition or package keeps a node, "ports" the port usages with the
// ports nested under them. A row is a node whose Compartment names the
// compartment of its Parent it is drawn in; another name is refused as an
// invalid request naming the foldings there are.
func WithCompartments(folding string) RenderViewOption {
	return func(options *renderViewOptions) { options.compartments = folding }
}

// WithFullPorts includes ports whether or not a connector uses them.
func WithFullPorts() RenderViewOption {
	return func(options *renderViewOptions) { options.ports = "full" }
}

// WithShow draws the named optional notation elements besides those the
// view's rendering shows: "parameter-symbols", "port-symbols", "compartments",
// "edge-labels", "keywords", "multiplicities", "typings", "qualifiers" or
// "all". The result's Shown lists the ones drawn; a name none of them has is
// refused as an invalid request naming the elements there are.
func WithShow(elements ...string) RenderViewOption {
	return func(options *renderViewOptions) { options.show = append(options.show, elements...) }
}

// WithDepth draws this many levels of nested elements below each exposed
// element, overriding the view's own depth; zero draws every level the view
// allows.
func WithDepth(depth int) RenderViewOption {
	return func(options *renderViewOptions) { options.depth = depth }
}

// RenderedView is the lossless diagram data returned for a declared or
// targeted pseudo-view.
type RenderedView struct {
	View    string
	Kind    string
	Stated  string
	Nodes   []RenderNode
	Edges   []RenderEdge
	Columns []string
	Rows    []RenderRow
	Canvas  *RenderCanvas
	Notes   []RenderNote
	Notices []string
	// Shown lists the optional notation elements drawn, the view's own and
	// those WithShow asked for.
	Shown []string
	// Frame is the view frame round a graph-shaped rendering; nil when frameless.
	Frame *RenderFrame
	// Text is the rendering written in the form WithForm named; empty without one.
	Text string
}

// RenderFrame is the view frame of SysML v2 §8.2.3.26: the name compartment's
// name and view definition, and the lines of its three information compartments.
type RenderFrame struct {
	Name        string
	Type        string
	Exposes     []string
	Filters     []string
	Annotations []string
}

// RenderNode is a view node with optional geometry and style.
type RenderNode struct {
	ID              string
	Kind            string
	Name            string
	NameSynthesized bool
	Type            string
	Detail          string
	Text            string
	StandIn         bool
	// Parallel marks a state whose substates run in parallel; the notation
	// writes «parallel» under its name.
	Parallel bool
	Parent   string
	// Compartment names the compartment of Parent this node is a row of
	// ("attributes", "parts", "ports", ...); empty for a node drawn as a box.
	Compartment string
	Ports       []RenderPort
	Origin      *Span
	Geometry    *RenderGeometry
	Style       *RenderStyle
}

// RenderPort is a port shown on a rendered node.
type RenderPort struct {
	ID        string
	Name      string
	Type      string
	Direction string
}

// RenderEdge is a connection between rendered nodes.
type RenderEdge struct {
	From     string
	To       string
	FromPort string
	ToPort   string
	Label    string
	Name     string
	Kind     string
	Origin   *Span
	Route    []RenderPoint
	Style    *RenderStyle
}

// RenderGeometry is a node's position and optional size; an Unplaced one only
// folds or hides the node, its X and Y carrying no position.
type RenderGeometry struct {
	X         float64
	Y         float64
	Width     float64
	Height    float64
	HasSize   bool
	Collapsed bool
	Unplaced  bool
}

// RenderCanvas is the optional canvas bounds for a view.
type RenderCanvas struct {
	Unit    string
	Width   float64
	Height  float64
	HasSize bool
}

// RenderStyle is a rendered node or edge's optional style.
type RenderStyle struct {
	Fill     string
	Line     string
	Text     string
	Font     string
	FontSize float64
	Bold     bool
	Italic   bool
}

// RenderPoint is a point on an edge route.
type RenderPoint struct {
	X float64
	Y float64
}

// RenderRow is one row of a table-style view.
type RenderRow struct {
	Cells  []string
	Origin *Span
}

// RenderNote is an annotation in a rendered view.
type RenderNote struct {
	Text     string
	Anchor   string
	EdgeFrom string
	EdgeTo   string
	X        float64
	Y        float64
	Width    float64
	Height   float64
	HasSize  bool
	Origin   *Span
}

// RenderView renders a named view as data, shaped by WithFullPorts, WithShow, WithDepth
// and WithCompartments; WithForm also writes it as text, under the writer's options.
func (c *client) RenderView(ctx context.Context, model *Model, viewName string, opts ...RenderViewOption) (*RenderedView, error) {
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	if err := c.requireCapabilities(ctx, CapabilityRenderView); err != nil {
		return nil, err
	}
	var options renderViewOptions
	for _, opt := range opts {
		opt(&options)
	}
	if options.written() {
		if err := c.requireCapabilities(ctx, CapabilityRenderViewForm); err != nil {
			return nil, err
		}
	}
	if options.depth < 0 || options.depth > math.MaxInt32 {
		return nil, &StatusError{Code: CodeInvalidArgument, Message: fmt.Sprintf("depth must be between 0 and %d, got %d", math.MaxInt32, options.depth)}
	}
	response, err := c.caller.renderView(ctx, &pb.RenderViewRequest{
		ModelHash:    hash,
		View:         viewName,
		Ports:        options.ports,
		Show:         append([]string(nil), options.show...),
		Depth:        int32(options.depth),
		Compartments: options.compartments,
		Form:         options.form,
		Palette:      options.palette,
		Style:        options.style,
		Lines:        options.lines,
		Direction:    options.direction,
		Link:         options.link,
		Unplaced:     options.unplaced,
	})
	if err != nil {
		return nil, err
	}
	return renderedViewFromProto(response), nil
}

func renderedViewFromProto(response *pb.RenderViewResponse) *RenderedView {
	result := &RenderedView{
		View: response.View, Kind: response.Kind, Stated: response.Stated,
		Columns: append([]string(nil), response.Columns...), Notices: append([]string(nil), response.Notices...),
		Shown: append([]string(nil), response.Shown...), Text: response.Text,
	}
	for _, node := range response.Nodes {
		converted := RenderNode{
			ID: node.Id, Kind: node.Kind, Name: node.Name, NameSynthesized: node.NameSynthesized,
			Type: node.Type, Detail: node.Detail, Text: node.Text, StandIn: node.StandIn, Parallel: node.Parallel, Parent: node.Parent,
			Compartment: node.Compartment, Origin: renderSpanFromProto(node.Origin),
		}
		for _, port := range node.Ports {
			converted.Ports = append(converted.Ports, RenderPort{
				ID: port.Id, Name: port.Name, Type: port.Type, Direction: port.Direction,
			})
		}
		if node.Geometry != nil {
			converted.Geometry = &RenderGeometry{
				X: node.Geometry.X, Y: node.Geometry.Y, Width: node.Geometry.Width, Height: node.Geometry.Height,
				HasSize: node.Geometry.HasSize, Collapsed: node.Geometry.Collapsed, Unplaced: node.Geometry.Unplaced,
			}
		}
		converted.Style = renderStyleFromProto(node.Style)
		result.Nodes = append(result.Nodes, converted)
	}
	for _, edge := range response.Edges {
		converted := RenderEdge{
			From: edge.From, To: edge.To, FromPort: edge.FromPort, ToPort: edge.ToPort,
			Label: edge.Label, Name: edge.Name, Kind: edge.Kind, Origin: renderSpanFromProto(edge.Origin),
			Style: renderStyleFromProto(edge.Style),
		}
		for _, point := range edge.Route {
			converted.Route = append(converted.Route, RenderPoint{X: point.X, Y: point.Y})
		}
		result.Edges = append(result.Edges, converted)
	}
	for _, row := range response.Rows {
		result.Rows = append(result.Rows, RenderRow{Cells: append([]string(nil), row.Cells...), Origin: renderSpanFromProto(row.Origin)})
	}
	for _, note := range response.Notes {
		result.Notes = append(result.Notes, RenderNote{
			Text: note.Text, Anchor: note.Anchor, EdgeFrom: note.EdgeFrom, EdgeTo: note.EdgeTo,
			X: note.X, Y: note.Y, Width: note.Width, Height: note.Height, HasSize: note.HasSize,
			Origin: renderSpanFromProto(note.Origin),
		})
	}
	if response.Canvas != nil {
		result.Canvas = &RenderCanvas{
			Unit: response.Canvas.Unit, Width: response.Canvas.Width, Height: response.Canvas.Height,
			HasSize: response.Canvas.HasSize,
		}
	}
	if f := response.Frame; f != nil {
		result.Frame = &RenderFrame{
			Name: f.Name, Type: f.Type, Exposes: append([]string(nil), f.Exposes...),
			Filters: append([]string(nil), f.Filters...), Annotations: append([]string(nil), f.Annotations...),
		}
	}
	return result
}

func renderStyleFromProto(style *pb.RenderStyle) *RenderStyle {
	if style == nil {
		return nil
	}
	return &RenderStyle{
		Fill: style.Fill, Line: style.Line, Text: style.Text, Font: style.Font, FontSize: style.FontSize,
		Bold: style.Bold, Italic: style.Italic,
	}
}

func renderSpanFromProto(span *pb.Span) *Span {
	if span == nil {
		return nil
	}
	return &Span{
		File: span.File, StartLine: int(span.StartLine), StartCol: int(span.StartCol),
		EndLine: int(span.EndLine), EndCol: int(span.EndCol),
	}
}
