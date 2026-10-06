package grpc

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
)

func TestRenderViewRendersInterconnectionAndPorts(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "conformance", "fixtures", "views.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := parseContent(t, srv, string(content))

	minimal, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{
		ModelHash: parsed.ModelHash, View: "RenderViewDemo::connections",
	})
	if err != nil {
		t.Fatalf("RenderView minimal: %v", err)
	}
	if minimal.Kind != "interconnection" || len(minimal.Nodes) != 3 || len(minimal.Edges) != 1 {
		t.Fatalf("RenderView = kind %q, %d nodes, %d edges; want interconnection, 3 and 1",
			minimal.Kind, len(minimal.Nodes), len(minimal.Edges))
	}
	var ports int
	for _, node := range minimal.Nodes {
		ports += len(node.Ports)
		if node.Origin == nil || node.Origin.StartLine == 0 {
			t.Errorf("node %q has no source origin: %+v", node.Name, node.Origin)
		}
	}
	if ports != 2 {
		t.Errorf("minimal ports = %d, want the two connected ports", ports)
	}
	edge := minimal.Edges[0]
	if edge.FromPort == "" || edge.ToPort == "" {
		t.Errorf("edge endpoint ports = %q -> %q, want both set", edge.FromPort, edge.ToPort)
	}
	if edge.Origin == nil || edge.Origin.StartLine == 0 {
		t.Errorf("edge has no source origin: %+v", edge.Origin)
	}

	full, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{
		ModelHash: parsed.ModelHash, View: "RenderViewDemo::connections", Ports: "full",
	})
	if err != nil {
		t.Fatalf("RenderView full: %v", err)
	}
	ports = 0
	for _, node := range full.Nodes {
		ports += len(node.Ports)
	}
	if ports != 3 {
		t.Errorf("full ports = %d, want all three declared ports", ports)
	}
}

func TestRenderViewRendersCaseAndMixed(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	parsed, _ := parseContent(t, srv, `package RenderViewKinds {
	private import OpenSysMLRenderings::*;

	part def Vehicle;
	part def Pilot;
	use case def CrewOperation {
		subject vehicle : Vehicle;
		actor pilot : Pilot;
	}

	port def Signal;
	action def Descend {
		first start;
		action land;
		done;
		succession first start then land;
		succession first land then done;
	}
	part def Lander {
		port telemetry : Signal;
		perform action descend : Descend;
	}

	view useCases {
		expose CrewOperation;
		render asCaseDiagram;
	}
	view mixedOverview {
		expose Lander;
		expose Descend;
		render asMixedDiagram;
	}
}`)

	requireNodeKinds := func(response *pb.RenderViewResponse, want ...string) {
		t.Helper()
		kinds := make(map[string]bool)
		for _, node := range response.Nodes {
			kinds[node.Kind] = true
		}
		for _, kind := range want {
			if !kinds[kind] {
				t.Errorf("view %q node kinds = %v, want %q", response.View, kinds, kind)
			}
		}
	}
	requireEdgeKinds := func(response *pb.RenderViewResponse, want ...string) {
		t.Helper()
		kinds := make(map[string]bool)
		for _, edge := range response.Edges {
			kinds[edge.Kind] = true
		}
		for _, kind := range want {
			if !kinds[kind] {
				t.Errorf("view %q edge kinds = %v, want %q", response.View, kinds, kind)
			}
		}
	}

	caseView, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{
		ModelHash: parsed.ModelHash, View: "RenderViewKinds::useCases",
	})
	if err != nil {
		t.Fatalf("RenderView case: %v", err)
	}
	if caseView.Kind != "case" {
		t.Errorf("case view kind = %q, want case", caseView.Kind)
	}
	requireNodeKinds(caseView, "use case def", "actor", "subject")
	requireEdgeKinds(caseView, "association")

	mixedView, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{
		ModelHash: parsed.ModelHash, View: "RenderViewKinds::mixedOverview", Ports: "full",
	})
	if err != nil {
		t.Fatalf("RenderView mixed: %v", err)
	}
	if mixedView.Kind != "mixed" {
		t.Errorf("mixed view kind = %q, want mixed", mixedView.Kind)
	}
	requireNodeKinds(mixedView, "part def", "port", "action def")
	requireEdgeKinds(mixedView, "reference", "succession")
	var telemetry, perform bool
	for _, node := range mixedView.Nodes {
		telemetry = telemetry || node.Kind == "port" && node.Name == "telemetry"
	}
	for _, edge := range mixedView.Edges {
		perform = perform || edge.Kind == "reference" && edge.Label == "«perform»"
	}
	if !telemetry {
		t.Errorf("mixed view nodes do not include the Lander telemetry port: %+v", mixedView.Nodes)
	}
	if !perform {
		t.Errorf("mixed view has no perform reference edge: %+v", mixedView.Edges)
	}

	casePseudo, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{
		ModelHash: parsed.ModelHash, View: "#case:RenderViewKinds::CrewOperation",
	})
	if err != nil {
		t.Fatalf("RenderView case pseudo-view: %v", err)
	}
	if casePseudo.Kind != "case" {
		t.Errorf("case pseudo-view kind = %q, want case", casePseudo.Kind)
	}
}

func TestRenderViewMapsDiagramLayoutNotes(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "internal", "ir", "view", "testdata", "shared-notes.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := parseContent(t, srv, string(content))
	rendered, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{
		ModelHash: parsed.ModelHash, View: "SharedViews::rigView",
	})
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}
	for _, note := range rendered.Notes {
		if note.Text != "calibrated" {
			continue
		}
		if note.Anchor == "" || note.Origin == nil || note.Origin.StartLine == 0 {
			t.Errorf("note = %+v, want anchor and source origin", note)
		}
		if note.X != 400 || note.Y != 20 || note.Width != 120 || note.Height != 40 || !note.HasSize {
			t.Errorf("note geometry = %+v, want the stated note rectangle", note)
		}
		return
	}
	t.Fatalf("notes = %+v, want the calibrated note", rendered.Notes)
}

func TestRenderViewMatchesEngineJSON(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "assets", "opensysml-stack.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := parseContent(t, srv, string(content))

	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	params, err := json.Marshal(map[string]any{
		"documents": []map[string]string{{"name": "opensysml-stack.sysml", "content": string(content)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := eng.Call(context.Background(), "ParseSources", params)
	if err != nil {
		t.Fatalf("engine ParseSources: %v", err)
	}
	var engineParsed engine.JParseSourcesResponse
	if err := json.Unmarshal(body, &engineParsed); err != nil {
		t.Fatalf("decode engine ParseSources: %v", err)
	}
	const viewName = "#interconnection:OpenSysMLStack::stack"
	engineRequest, err := json.Marshal(map[string]string{
		"modelHash": engineParsed.ModelHash, "view": viewName, "ports": "minimal",
	})
	if err != nil {
		t.Fatal(err)
	}
	engineBody, err := eng.Call(context.Background(), "RenderView", engineRequest)
	if err != nil {
		t.Fatalf("engine RenderView: %v", err)
	}
	var engineView engine.JRenderViewResponse
	if err := json.Unmarshal(engineBody, &engineView); err != nil {
		t.Fatalf("decode engine RenderView: %v", err)
	}
	grpcView, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{
		ModelHash: parsed.ModelHash, View: viewName, Ports: "minimal",
	})
	if err != nil {
		t.Fatalf("gRPC RenderView: %v", err)
	}
	if got := engineProjection(grpcView); !reflect.DeepEqual(got, engineView) {
		t.Errorf("shared engine/gRPC render fields differ:\nengine JSON: %+v\ngRPC projection: %+v",
			engineView, got)
	}
}

func engineProjection(response *pb.RenderViewResponse) engine.JRenderViewResponse {
	projected := engine.JRenderViewResponse{
		View: response.View, Kind: response.Kind, Stated: response.Stated,
		Notices: response.Notices, Columns: response.Columns,
		Nodes: make([]engine.JRenderNode, 0, len(response.Nodes)),
		Edges: make([]engine.JRenderEdge, 0, len(response.Edges)),
	}
	if response.Canvas != nil {
		projected.Canvas = &engine.JRenderCanvas{Unit: response.Canvas.Unit}
		if response.Canvas.HasSize {
			width, height := response.Canvas.Width, response.Canvas.Height
			projected.Canvas.Width, projected.Canvas.Height = &width, &height
		}
	}
	for _, node := range response.Nodes {
		item := engine.JRenderNode{
			ID: node.Id, Kind: node.Kind, Name: node.Name, NameSynthesized: node.NameSynthesized,
			Type: node.Type, Detail: node.Detail, Parent: node.Parent,
		}
		if node.Style != nil {
			item.Style = &engine.JRenderStyle{
				Fill: node.Style.Fill, Line: node.Style.Line, Text: node.Style.Text,
				Font: node.Style.Font, FontSize: node.Style.FontSize, Bold: node.Style.Bold,
				Italic: node.Style.Italic,
			}
			item.Fill, item.Border = node.Style.Fill, node.Style.Line
		}
		if node.Geometry != nil {
			x, y := node.Geometry.X, node.Geometry.Y
			item.X, item.Y, item.Collapsed = &x, &y, node.Geometry.Collapsed
			if node.Geometry.HasSize {
				width, height := node.Geometry.Width, node.Geometry.Height
				item.Width, item.Height = &width, &height
			}
		}
		for _, port := range node.Ports {
			item.Ports = append(item.Ports, engine.JRenderPort{
				ID: port.Id, Name: port.Name, Type: port.Type, Direction: port.Direction,
			})
		}
		projected.Nodes = append(projected.Nodes, item)
	}
	for _, edge := range response.Edges {
		item := engine.JRenderEdge{
			From: edge.From, To: edge.To, FromPort: edge.FromPort, ToPort: edge.ToPort,
			Label: edge.Label, Kind: edge.Kind,
		}
		if edge.Style != nil {
			item.Style = &engine.JRenderStyle{
				Fill: edge.Style.Fill, Line: edge.Style.Line, Text: edge.Style.Text,
				Font: edge.Style.Font, FontSize: edge.Style.FontSize, Bold: edge.Style.Bold,
				Italic: edge.Style.Italic,
			}
		}
		for _, point := range edge.Route {
			item.Route = append(item.Route, engine.JRenderPoint{X: point.X, Y: point.Y})
		}
		projected.Edges = append(projected.Edges, item)
	}
	for _, row := range response.Rows {
		projected.Rows = append(projected.Rows, engine.JRenderRow{Cells: row.Cells})
	}
	return projected
}

func TestRenderViewErrorsMatchEngineSelection(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "conformance", "fixtures", "views.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := parseContent(t, srv, string(content))

	for _, test := range []struct {
		name string
		req  *pb.RenderViewRequest
		code connect.Code
		want string
	}{
		{"unknown model", &pb.RenderViewRequest{ModelHash: "missing", View: "RenderViewDemo::connections"}, connect.CodeNotFound, "model not found"},
		{"empty view", &pb.RenderViewRequest{ModelHash: parsed.ModelHash}, connect.CodeInvalidArgument, "view is required"},
		{"bad ports", &pb.RenderViewRequest{ModelHash: parsed.ModelHash, View: "RenderViewDemo::connections", Ports: "all"}, connect.CodeInvalidArgument, "unknown port display"},
		{"unknown view", &pb.RenderViewRequest{ModelHash: parsed.ModelHash, View: "RenderViewDemo::Missing"}, connect.CodeNotFound, "no view named"},
		{"malformed pseudo-view", &pb.RenderViewRequest{ModelHash: parsed.ModelHash, View: "#unknown:RenderViewDemo::assembly"}, connect.CodeInvalidArgument, "is no pseudo-view"},
		{"untargeted pseudo-view", &pb.RenderViewRequest{ModelHash: parsed.ModelHash, View: "#tree"}, connect.CodeInvalidArgument, "is untargeted"},
		{"missing pseudo-view target", &pb.RenderViewRequest{ModelHash: parsed.ModelHash, View: "#tree:RenderViewDemo::Missing"}, connect.CodeNotFound, "names nothing in this model"},
		{"not a view", &pb.RenderViewRequest{ModelHash: parsed.ModelHash, View: "RenderViewDemo::Assembly"}, connect.CodeInvalidArgument, "not a view"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := srv.RenderView(context.Background(), test.req)
			if connect.CodeOf(err) != test.code || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("RenderView error = %v (%s), want %s containing %q", err, connect.CodeOf(err), test.code, test.want)
			}
		})
	}
}

func TestRenderViewRequiresCapability(t *testing.T) {
	srv := mustNewServiceWithout(t, CapabilityRenderView)
	_, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{ModelHash: "unknown", View: "x"})
	if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityRenderView) {
		t.Fatalf("RenderView error = %v, want UNIMPLEMENTED naming %s", err, CapabilityRenderView)
	}
}
