package view

// Data is the machine-consumable shape of a Rendering: the nodes flattened out
// of the containment tree, the edges between them, the rows of a tabular
// rendering and what the rendering could not represent. It carries no protocol
// or wire concern — a caller speaking one converts it.
type Data struct {
	// View is the rendered view by qualified name, "" for a rendering of exposed
	// elements alone (RenderExposed).
	View string
	// Kind is the rendering produced, and Stated how the kind was decided.
	Kind   Kind
	Stated string
	// Run marks a rendering of a run's trace rather than of a view.
	Run bool
	// RunUntil is the clock instant through which a run rendering was recorded.
	RunUntil float64
	// Nodes are every node of the rendering, parents before children, each
	// naming its parent.
	Nodes []NodeData
	// Edges join nodes by ID.
	Edges []EdgeData
	// Columns and Rows are the tabular rendering, empty for every other kind.
	Columns []string
	Rows    []RowData
	// Lanes are a run timeline's object machines.
	Lanes []Lane
	// Canvas is the drawing surface the view states, nil for none.
	Canvas *Canvas
	// Notes are the note boxes drawn on the canvas, anchored to a node ID or free.
	Notes []Note
	// Notices are what the rendering could not represent.
	Notices []string
}

// NodeData is one node of a rendering, with the node it is nested in.
type NodeData struct {
	ID   string
	Kind string
	Name string
	// NameSynthesized marks a Name the model did not give: a name to key by, not one a picture shows.
	NameSynthesized bool
	Type            string
	Detail          string
	// Text heads a node whose name is not shown, in place of its kind: a value
	// specification's literal, an accept's event, a send's message.
	Text string
	// StandIn marks a node a migration made up that stands for no element of its
	// source, which a positioned drawing may elide.
	StandIn bool
	// Ports are the pins drawn on the node's border, which an edge may end at.
	Ports []Port
	// Parent is the ID of the node this one is nested in, "" for a root.
	Parent string
	Origin Origin
	// Geometry is where the node is drawn, nil when no Layout positions it.
	Geometry *Geometry
	// Style is how the node is drawn, nil when no Style colours it.
	Style *Style
	// Verdict is the worst verification verdict overlaid on a requirement, ""
	// when none is.
	Verdict string `json:",omitempty"`
}

// EdgeData is one edge of a rendering, joining two node IDs.
type EdgeData struct {
	From string
	To   string
	// FromPort and ToPort are the ports the edge ends at, "" for the node itself.
	FromPort string
	ToPort   string
	Label    string
	// Name is the edge's own name, "" for one anonymous or named by a migration.
	Name   string
	Kind   EdgeKind
	Origin Origin
	// Route is the waypoints the edge follows, empty when no Route gives any.
	Route []Point
	// Style is how the edge is drawn, nil when no Style colours it.
	Style *Style
}

// RowData is one row of a tabular rendering: its cells, one per column, and
// where the element it reports was declared.
type RowData struct {
	Cells  []string
	Origin Origin
}

// Data is the rendering in machine-consumable form.
func (r *Rendering) Data() Data {
	return r.data(nil)
}

// DataFor is the rendering in machine-consumable form with each node's ports
// filtered for the requested display.
func (r *Rendering) DataFor(display Ports) Data {
	ports := r.portView(display)
	return r.data(&ports)
}

func (r *Rendering) data(ports *portView) Data {
	out := Data{
		View:     r.View,
		Kind:     r.Kind,
		Stated:   r.Stated,
		Run:      r.Run,
		RunUntil: r.RunUntil,
		Columns:  r.Columns,
		Lanes:    r.Lanes,
		Canvas:   r.Canvas,
		Notes:    r.Notes,
		Notices:  r.Notices,
	}
	if refusals := refusedPictureNotices(r.Pictures, r.pictureRefusals()); len(refusals) > 0 {
		out.Notices = append(append([]string(nil), r.Notices...), refusals...)
	}
	for _, root := range r.Roots {
		out.Nodes = appendNodeData(out.Nodes, root, "", ports)
	}
	for _, edge := range r.Edges {
		out.Edges = append(out.Edges, EdgeData{From: edge.From, To: edge.To, FromPort: edge.FromPort, ToPort: edge.ToPort,
			Label: edge.Label, Name: edge.Name, Kind: edge.Kind, Origin: edge.Origin, Route: edge.Route, Style: edge.Style})
	}
	for i, cells := range r.Rows {
		var origin Origin
		if i < len(r.RowOrigins) {
			origin = r.RowOrigins[i]
		}
		out.Rows = append(out.Rows, RowData{Cells: cells, Origin: origin})
	}
	return out
}

// appendNodeData flattens a node and what is nested in it, parents first.
func appendNodeData(out []NodeData, node *Node, parent string, ports *portView) []NodeData {
	if node == nil {
		return out
	}
	nodePorts := node.Ports
	if ports != nil {
		nodePorts = ports.of(node)
	}
	out = append(out, NodeData{
		ID: node.ID, Kind: node.Kind, Name: node.Name, NameSynthesized: node.NameSynthesized, Type: node.Type, Detail: node.Detail,
		Text: node.Text, StandIn: node.StandIn, Ports: nodePorts, Parent: parent, Origin: node.Origin, Geometry: node.Geometry, Style: node.Style,
		Verdict: node.Verdict,
	})
	for _, child := range node.Children {
		out = appendNodeData(out, child, node.ID, ports)
	}
	return out
}

// appendRow adds a row of a tabular rendering and where its element was
// declared, keeping the two in step.
func (r *Rendering) appendRow(cells []string, origin Origin) {
	r.Rows = append(r.Rows, cells)
	r.RowOrigins = append(r.RowOrigins, origin)
}
