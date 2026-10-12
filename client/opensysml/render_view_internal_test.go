package opensysml

import (
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

func TestRenderedViewFromProtoPreservesFieldsAndPresence(t *testing.T) {
	response := &pb.RenderViewResponse{
		View: "Demo::view", Kind: "interconnection", Stated: "rendered",
		Columns: []string{"name"}, Notices: []string{"notice"},
		Canvas: &pb.RenderCanvas{Unit: "px", Width: 800, Height: 400, HasSize: true},
		Nodes: []*pb.RenderNode{{
			Id: "n0", Kind: "part", Name: "root", NameSynthesized: true, Type: "Demo::Part",
			Detail: "detail", Text: "text", StandIn: true, Parent: "parent",
			Ports:    []*pb.RenderPort{{Id: "n0.0", Name: "api", Type: "Demo::API", Direction: "inout"}},
			Origin:   &pb.Span{File: "views.sysml", StartLine: 4},
			Geometry: &pb.RenderGeometry{X: 1, Y: 2, Width: 3, Height: 4, HasSize: true, Collapsed: true},
			Style: &pb.RenderStyle{Fill: "#fff", Line: "#000", Text: "#111", Font: "sans",
				FontSize: 12, Bold: true, Italic: true},
		}},
		Edges: []*pb.RenderEdge{{
			From: "n1", To: "n0", FromPort: "n1.0", ToPort: "n0.0", Label: "wire",
			Name: "wire", Kind: "connection", Origin: &pb.Span{File: "views.sysml"},
			Route: []*pb.RenderPoint{{X: 2, Y: 3}},
			Style: &pb.RenderStyle{Line: "#222"},
		}},
		Rows: []*pb.RenderRow{{Cells: []string{"x"}, Origin: &pb.Span{File: "views.sysml"}}},
		Notes: []*pb.RenderNote{{
			Text: "note", Anchor: "n0", EdgeFrom: "n1", EdgeTo: "n0",
			X: 1, Y: 2, Width: 3, Height: 4, HasSize: true,
			Origin: &pb.Span{File: "views.sysml"},
		}},
	}

	rendered := renderedViewFromProto(response)
	if rendered.View != "Demo::view" || rendered.Kind != "interconnection" || rendered.Stated != "rendered" {
		t.Fatalf("rendered metadata = %+v", rendered)
	}
	node := rendered.Nodes[0]
	if node.ID != "n0" || node.Kind != "part" || node.Name != "root" || !node.NameSynthesized ||
		node.Type != "Demo::Part" || node.Detail != "detail" || node.Text != "text" ||
		!node.StandIn || node.Parent != "parent" || node.Ports[0].Direction != "inout" {
		t.Errorf("rendered node = %+v", node)
	}
	if node.Origin == nil || node.Origin.StartLine != 4 || node.Geometry == nil ||
		!node.Geometry.Collapsed || node.Style == nil || node.Style.FontSize != 12 || !node.Style.Bold {
		t.Errorf("rendered node optional fields = %+v", node)
	}
	if edge := rendered.Edges[0]; edge.From != "n1" || edge.To != "n0" ||
		edge.FromPort != "n1.0" || edge.ToPort != "n0.0" || edge.Name != "wire" ||
		edge.Kind != "connection" || edge.Route[0] != (RenderPoint{X: 2, Y: 3}) ||
		edge.Style == nil || edge.Style.Line != "#222" || edge.Origin == nil {
		t.Errorf("rendered edge = %+v", edge)
	}
	if rendered.Columns[0] != "name" || rendered.Rows[0].Cells[0] != "x" ||
		rendered.Rows[0].Origin == nil || rendered.Canvas == nil || !rendered.Canvas.HasSize ||
		rendered.Notes[0].EdgeTo != "n0" || rendered.Notes[0].Origin == nil ||
		rendered.Notices[0] != "notice" {
		t.Errorf("rendered remaining fields = %+v", rendered)
	}
	empty := renderedViewFromProto(&pb.RenderViewResponse{})
	if empty.Canvas != nil || empty.Nodes != nil || empty.Edges != nil {
		t.Errorf("absent optional/render collections = %+v", empty)
	}
}
