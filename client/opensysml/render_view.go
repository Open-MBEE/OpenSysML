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
	Parent          string
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

// RenderView renders a named view. Ports are minimal unless WithFullPorts is
// supplied; WithShow adds optional notation elements and WithDepth limits how
// deep nested elements are drawn.
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
		Shown: append([]string(nil), response.Shown...),
	}
	for _, node := range response.Nodes {
		converted := RenderNode{
			ID: node.Id, Kind: node.Kind, Name: node.Name, NameSynthesized: node.NameSynthesized,
			Type: node.Type, Detail: node.Detail, Text: node.Text, StandIn: node.StandIn, Parent: node.Parent, Compartment: node.Compartment,
			Origin: renderSpanFromProto(node.Origin),
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
