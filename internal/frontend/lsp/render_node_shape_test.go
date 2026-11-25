package lsp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// The LSP render result carries the same node shape as the gRPC response, so
// the editor's canvas draws notation from the structured result.
func TestRenderNodesCarryNotationShape(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "ir", "view", "testdata", "shape-nodes.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	s, docURI := renderServer(t, "shape-nodes.sysml", string(content))
	res, err := call(t, s, MethodRender, &RenderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "ShapeExamples::shapes",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var out RenderResult
	if err := json.Unmarshal(res, &out); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"Inspection": "stadium", "driver": "actor", "Ready": "rectangle"}
	for _, node := range out.Nodes {
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
}

func TestInitializeAdvertisesTheRenderNodeShapeCapability(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	res, err := s.Initialize(context.Background(), &protocol.InitializeParams{})
	if err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
	experimental := experimentalCaps(t, res)
	if experimental[RenderNodeShapeCapability] != true {
		t.Errorf("%s = %#v, want true", RenderNodeShapeCapability, experimental[RenderNodeShapeCapability])
	}
}
