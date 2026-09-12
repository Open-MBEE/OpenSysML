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
	// Nodes are every node of the rendering, parents before children, each
	// naming its parent.
	Nodes []NodeData
	// Edges join nodes by ID.
	Edges []EdgeData
	// Columns and Rows are the tabular rendering, empty for every other kind.
	Columns []string
	Rows    []RowData
	// Canvas is the drawing surface the view states, nil for none.
	Canvas *Canvas
	// Notices are what the rendering could not represent.
	Notices []string
}

// NodeData is one node of a rendering, with the node it is nested in.
type NodeData struct {
	ID     string
	Kind   string
	Name   string
	Detail string
	// Parent is the ID of the node this one is nested in, "" for a root.
	Parent string
	Origin Origin
	// Geometry is where the node is drawn, nil when no Layout positions it.
	Geometry *Geometry
}

// EdgeData is one edge of a rendering, joining two node IDs.
type EdgeData struct {
	From   string
	To     string
	Label  string
	Kind   EdgeKind
	Origin Origin
	// Route is the waypoints the edge follows, empty when no Route gives any.
	Route []Point
}

// RowData is one row of a tabular rendering: its cells, one per column, and
// where the element it reports was declared.
type RowData struct {
	Cells  []string
	Origin Origin
}

// Data is the rendering in machine-consumable form.
func (r *Rendering) Data() Data {
	out := Data{
		View:    r.View,
		Kind:    r.Kind,
		Stated:  r.Stated,
		Columns: r.Columns,
		Canvas:  r.Canvas,
		Notices: r.Notices,
	}
	for _, root := range r.Roots {
		out.Nodes = appendNodeData(out.Nodes, root, "")
	}
	for _, edge := range r.Edges {
		out.Edges = append(out.Edges, EdgeData{
			From: edge.From, To: edge.To, Label: edge.Label, Kind: edge.Kind, Origin: edge.Origin, Route: edge.Route,
		})
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
func appendNodeData(out []NodeData, node *Node, parent string) []NodeData {
	if node == nil {
		return out
	}
	out = append(out, NodeData{
		ID: node.ID, Kind: node.Kind, Name: node.Name, Detail: node.Detail, Parent: parent, Origin: node.Origin,
		Geometry: node.Geometry,
	})
	for _, child := range node.Children {
		out = appendNodeData(out, child, node.ID)
	}
	return out
}

// appendRow adds a row of a tabular rendering and where its element was
// declared, keeping the two in step.
func (r *Rendering) appendRow(cells []string, origin Origin) {
	r.Rows = append(r.Rows, cells)
	r.RowOrigins = append(r.RowOrigins, origin)
}
