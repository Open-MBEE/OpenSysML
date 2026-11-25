package grpc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

var shapeModelPath = filepath.Join("..", "..", "ir", "view", "testdata", "shape-nodes.sysml")

// The structured render carries each node's notation shape, so a canvas need
// not parse a written form to draw a case, requirement or actor.
func TestRenderViewCarriesNodeShape(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	content, err := os.ReadFile(shapeModelPath)
	if err != nil {
		t.Fatal(err)
	}
	hash := mustParse(t, srv, string(content))
	response, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{ModelHash: hash, View: "ShapeExamples::shapes"})
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}
	want := map[string]string{
		"Inspection": "stadium",
		"driver":     "actor",
		"Ready":      "rectangle",
	}
	for _, node := range response.Nodes {
		if shape, ok := want[node.Name]; ok {
			if node.Shape != shape {
				t.Errorf("node %q shape = %q, want %q", node.Name, node.Shape, shape)
			}
			delete(want, node.Name)
		}
	}
	if len(want) > 0 {
		t.Errorf("nodes with no shape: %v", want)
	}
	found := false
	for _, name := range Capabilities() {
		found = found || name == CapabilityRenderNodeShape
	}
	if !found {
		t.Errorf("capabilities = %v, want one named %q", Capabilities(), CapabilityRenderNodeShape)
	}
}

// A client without the capability gets no shapes at all, as a server before
// the field answered none.
func TestRenderViewWithholdsNodeShapeWithoutCapability(t *testing.T) {
	srv := mustNewServiceWithout(t, CapabilityRenderNodeShape)
	t.Cleanup(srv.Close)
	content, err := os.ReadFile(shapeModelPath)
	if err != nil {
		t.Fatal(err)
	}
	hash := mustParse(t, srv, string(content))
	response, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{ModelHash: hash, View: "ShapeExamples::shapes"})
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}
	if len(response.Nodes) == 0 {
		t.Fatal("RenderView returned no nodes")
	}
	for _, node := range response.Nodes {
		if node.Shape != "" {
			t.Errorf("node %q shape = %q, want withheld", node.Name, node.Shape)
		}
	}
}
