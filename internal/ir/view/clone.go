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
		out.Edges[i].Style = cloneStyle(r.Edges[i].Style)
	}
	out.Notes = append([]Note(nil), r.Notes...)
	out.Pictures = append([]Picture(nil), r.Pictures...)
	if r.Canvas != nil {
		canvas := *r.Canvas
		out.Canvas = &canvas
	}
	out.Columns = append([]string(nil), r.Columns...)
	out.Rows = make([][]string, len(r.Rows))
	for i, row := range r.Rows {
		out.Rows[i] = append([]string(nil), row...)
	}
	out.Header = append([]HeaderField(nil), r.Header...)
	out.RowOrigins = append([]Origin(nil), r.RowOrigins...)
	out.Notices = append([]string(nil), r.Notices...)
	out.Frame = cloneFrame(r.Frame)
	return &out
}

func cloneFrame(f *Frame) *Frame {
	if f == nil {
		return nil
	}
	copied := *f
	copied.Exposes = append([]string(nil), f.Exposes...)
	copied.Filters = append([]string(nil), f.Filters...)
	copied.Annotations = append([]string(nil), f.Annotations...)
	return &copied
}

func cloneNodes(nodes []*Node) []*Node {
	if nodes == nil {
		return nil
	}
	out := make([]*Node, len(nodes))
	for i, node := range nodes {
		copied := *node
		copied.Children = cloneNodes(node.Children)
		copied.Compartments = cloneCompartments(node.Compartments)
		copied.Ports = append([]Port(nil), node.Ports...)
		for j, port := range copied.Ports {
			if port.Geometry != nil {
				geometry := *port.Geometry
				copied.Ports[j].Geometry = &geometry
			}
		}
		if node.Geometry != nil {
			geometry := *node.Geometry
			copied.Geometry = &geometry
		}
		copied.Style = cloneStyle(node.Style)
		out[i] = &copied
	}
	return out
}

func cloneStyle(s *Style) *Style {
	if s == nil {
		return nil
	}
	copied := *s
	return &copied
}

func cloneCompartments(compartments []Compartment) []Compartment {
	if compartments == nil {
		return nil
	}
	out := make([]Compartment, len(compartments))
	for i, compartment := range compartments {
		out[i] = Compartment{Name: compartment.Name, Rows: cloneNodes(compartment.Rows)}
	}
	return out
}
