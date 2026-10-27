package grpc

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
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
		for _, port := range node.Ports {
			if !port.Inherited || strings.HasPrefix(port.Name, "^") {
				t.Errorf("port %+v of %q, want the pin the part has from its definition marked inherited under its bare name", port, node.Name)
			}
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

func TestRenderViewRendersRelationshipMatrices(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "wasm", "testdata", "render-view.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := parseContent(t, srv, string(content))
	if len(parsed.Diagnostics) != 0 {
		t.Fatalf("ParseFile diagnostics = %+v, want none", parsed.Diagnostics)
	}

	const source = "RenderDemo::Sender::out1"
	const target = "RenderDemo::Receiver::in1"
	for _, viewName := range []string{"RenderViews::relationshipMatrix", "#matrix:RenderDemo::Link"} {
		t.Run(viewName, func(t *testing.T) {
			rendered, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{
				ModelHash: parsed.ModelHash, View: viewName,
			})
			if err != nil {
				t.Fatalf("RenderView(%q): %v", viewName, err)
			}
			if rendered.Kind != "matrix" {
				t.Fatalf("RenderView(%q) kind = %q, want matrix", viewName, rendered.Kind)
			}
			if len(rendered.Columns) == 0 || rendered.Columns[0] != "Source / Target" {
				t.Fatalf("RenderView(%q) columns = %v, want Source / Target first", viewName, rendered.Columns)
			}
			targetColumn := -1
			for i, column := range rendered.Columns {
				if column == target {
					targetColumn = i
					break
				}
			}
			if targetColumn < 0 {
				t.Fatalf("RenderView(%q) columns = %v, want target %q", viewName, rendered.Columns, target)
			}
			for _, row := range rendered.Rows {
				if len(row.Cells) > 0 && row.Cells[0] == source {
					if targetColumn >= len(row.Cells) || row.Cells[targetColumn] != "connect" {
						t.Fatalf("RenderView(%q) source row = %v, want connect at target %q", viewName, row.Cells, target)
					}
					return
				}
			}
			t.Fatalf("RenderView(%q) rows = %+v, want source %q", viewName, rendered.Rows, source)
		})
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
	part def Rover {
		part wheels[4] : Vehicle;
	}

	view structure {
		expose Rover;
		expose Vehicle;
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
	requireEdgeKinds(mixedView, "perform", "succession")
	var telemetry, perform bool
	for _, node := range mixedView.Nodes {
		telemetry = telemetry || node.Kind == "port" && node.Name == "telemetry"
	}
	for _, edge := range mixedView.Edges {
		perform = perform || edge.Kind == "perform" && edge.Label == "«perform»"
	}
	if !telemetry {
		t.Errorf("mixed view nodes do not include the Lander telemetry port: %+v", mixedView.Nodes)
	}
	if !perform {
		t.Errorf("mixed view has no perform reference edge: %+v", mixedView.Edges)
	}

	// An association end's name — a part's, listed in a compartment of its
	// owner, on the owner's edge to the part's type — reaches the wire as the
	// edge's label, the response placing no label; multiplicities are an
	// optional element, off by default.
	structure, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{
		ModelHash: parsed.ModelHash, View: "RenderViewKinds::structure", Compartments: "most",
	})
	if err != nil {
		t.Fatalf("RenderView structure: %v", err)
	}
	var wheels bool
	for _, edge := range structure.Edges {
		wheels = wheels || edge.Kind == "composition" && edge.Label == "wheels"
	}
	if !wheels {
		t.Errorf("structure view has no composition labelled wheels: %+v", structure.Edges)
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
	parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: []*pb.SourceDocument{{
		Source: &pb.SourceDocument_Content{Content: string(content)}, Name: "opensysml-stack.sysml",
	}}})
	if err != nil {
		t.Fatalf("gRPC ParseSources: %v", err)
	}

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
	var engineView pb.RenderViewResponse
	if err := protojson.Unmarshal(engineBody, &engineView); err != nil {
		t.Fatalf("decode engine RenderView as protojson: %v\n%s", err, engineBody)
	}
	grpcView, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{
		ModelHash: parsed.ModelHash, View: viewName, Ports: "minimal",
	})
	if err != nil {
		t.Fatalf("gRPC RenderView: %v", err)
	}
	if !proto.Equal(grpcView, &engineView) {
		t.Errorf("engine RenderView JSON differs from the gRPC answer:\nengine: %s\ngRPC: %s",
			engineBody, protojson.Format(grpcView))
	}
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

// A request's depth bounds how many levels are drawn below each exposed
// element; a negative one is refused as an invalid argument.
func TestRenderViewDepth(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	parsed, _ := parseContent(t, srv, `package Deep {
    private import Views::*;
    part vehicle {
        part wheel { part hub { part bearing; } }
        part engine { part piston; }
    }
    view structure { expose vehicle; render asTreeDiagram; }
}
`)
	whole, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{ModelHash: parsed.ModelHash, View: "Deep::structure"})
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}
	bounded, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{ModelHash: parsed.ModelHash, View: "Deep::structure", Depth: 1})
	if err != nil {
		t.Fatalf("RenderView depth 1: %v", err)
	}
	if len(whole.Nodes) != 6 || len(bounded.Nodes) != 3 {
		t.Errorf("nodes = %d whole, %d at depth 1; want 6 and 3", len(whole.Nodes), len(bounded.Nodes))
	}
	if !strings.Contains(strings.Join(bounded.Notices, "\n"), "2 elements nested deeper than 1 level not shown") {
		t.Errorf("notices = %v, want the depth noticed", bounded.Notices)
	}
	_, err = srv.RenderView(context.Background(), &pb.RenderViewRequest{ModelHash: parsed.ModelHash, View: "Deep::structure", Depth: -1})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("depth -1: err = %v, want InvalidArgument", err)
	}
}

// An interconnection rendering of a bodiless usage shows the parts it has from
// its definition, each expanded through its own type.
func TestRenderViewExpandsUsagesThroughTheirTypes(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "examples", "runtime-showcase", "mass-rollup.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := parseContent(t, srv, string(content))
	rendered, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{
		ModelHash: parsed.ModelHash, View: "#interconnection:MassRollup::saturnV", Ports: "minimal", Depth: 3,
	})
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}
	children := map[string][]string{}
	for _, node := range rendered.Nodes {
		children[node.Parent] = append(children[node.Parent], node.Name)
	}
	if roots := children[""]; len(roots) != 1 || roots[0] != "MassRollup::saturnV" {
		t.Fatalf("roots = %v, want saturnV alone", roots)
	}
	root := rendered.Nodes[0]
	want := []string{"mass", "stage1", "stage2", "stage3", "subcomponents", "totalMass"}
	if got := slices.Sorted(slices.Values(children[root.Id])); !slices.Equal(got, want) {
		t.Errorf("saturnV holds %v, want %v", got, want)
	}
	for _, node := range rendered.Nodes {
		if node.Parent != root.Id || !strings.HasPrefix(node.Name, "stage") {
			continue
		}
		for _, name := range []string{"engine1", "engines", "propellantMass", "mass", "totalMass", "subcomponents"} {
			if !slices.Contains(children[node.Id], name) {
				t.Errorf("%s lacks %s from its type; holds %v", node.Name, name, children[node.Id])
			}
		}
	}
}

// A Layout binding only collapsed reaches gRPC and engine clients as geometry
// that is collapsed and unplaced, so they can fold the node without a place.
func TestRenderViewCarriesFoldOnlyLayoutUnplaced(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "internal", "ir", "view", "testdata", "layout-collapsed.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := parseContent(t, srv, string(content))
	const viewName = "FoldedViews::foldOnly"
	rendered, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{ModelHash: parsed.ModelHash, View: viewName})
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}
	checkFoldOnly := func(label string, nodes []*pb.RenderNode) {
		t.Helper()
		var engineGeometry *pb.RenderGeometry
		for _, node := range nodes {
			switch node.Name {
			case "engine":
				engineGeometry = node.Geometry
			case "piston":
				t.Errorf("%s: piston is drawn under the collapsed engine", label)
			}
		}
		if engineGeometry == nil || !engineGeometry.Collapsed || !engineGeometry.Unplaced || engineGeometry.X != 0 || engineGeometry.Y != 0 {
			t.Errorf("%s: engine geometry = %+v, want collapsed and unplaced at no position", label, engineGeometry)
		}
	}
	checkFoldOnly("gRPC", rendered.Nodes)

	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	params, err := json.Marshal(map[string]any{
		"documents": []map[string]string{{"name": "layout-collapsed.sysml", "content": string(content)}},
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
	engineRequest, err := json.Marshal(map[string]string{"modelHash": engineParsed.ModelHash, "view": viewName})
	if err != nil {
		t.Fatal(err)
	}
	engineBody, err := eng.Call(context.Background(), "RenderView", engineRequest)
	if err != nil {
		t.Fatalf("engine RenderView: %v", err)
	}
	var engineView pb.RenderViewResponse
	if err := protojson.Unmarshal(engineBody, &engineView); err != nil {
		t.Fatalf("decode engine RenderView as protojson: %v\n%s", err, engineBody)
	}
	checkFoldOnly("engine", engineView.Nodes)
}
