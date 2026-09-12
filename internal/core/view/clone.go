package view

// Clone returns a deep copy of the rendering, so a holder can hand it out
// without letting its nodes or rows be changed underneath it.
func (r *Rendering) Clone() *Rendering {
	if r == nil {
		return nil
	}
	out := *r
	out.drawn = nil
	out.Roots = cloneNodes(r.Roots)
	out.Edges = append([]Edge(nil), r.Edges...)
	for i := range out.Edges {
		out.Edges[i].Route = append([]Point(nil), r.Edges[i].Route...)
	}
	if r.Canvas != nil {
		canvas := *r.Canvas
		out.Canvas = &canvas
	}
	out.Columns = append([]string(nil), r.Columns...)
	out.Rows = make([][]string, len(r.Rows))
	for i, row := range r.Rows {
		out.Rows[i] = append([]string(nil), row...)
	}
	out.RowOrigins = append([]Origin(nil), r.RowOrigins...)
	out.Notices = append([]string(nil), r.Notices...)
	return &out
}

func cloneNodes(nodes []*Node) []*Node {
	if nodes == nil {
		return nil
	}
	out := make([]*Node, len(nodes))
	for i, node := range nodes {
		copied := *node
		copied.Children = cloneNodes(node.Children)
		if node.Geometry != nil {
			geometry := *node.Geometry
			copied.Geometry = &geometry
		}
		out[i] = &copied
	}
	return out
}
