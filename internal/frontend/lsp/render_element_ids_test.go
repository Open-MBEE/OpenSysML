package lsp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// TestRenderCarriesDurableElementIDs pins the element id on the LSP rendering
// payload, including the JSON name a canvas reads.
func TestRenderCarriesDurableElementIDs(t *testing.T) {
	const src = `package Demo {
    port def Signal;
    part def Sender { port out1 : Signal; }
    part def Receiver { port in1 : ~Signal; }
    part def Link {
        part sender : Sender;
        part receiver : Receiver;
        interface wire connect sender.out1 to receiver.in1;
    }
    view link { expose Demo::Link::*; render Views::asInterconnectionDiagram; }
}
`
	s, docURI := renderServer(t, "element-ids.sysml", src)
	if _, err := s.Initialize(context.Background(), &protocol.InitializeParams{}); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	raw, err := call(t, s, MethodRender, &RenderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "Demo::link", Form: string(view.FormD2), Ports: "full",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var out RenderResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode render result: %v", err)
	}
	var nodes, ports, edges int
	for _, node := range out.Nodes {
		nodes++
		if node.ElementID == "" || node.ElementID == node.ID {
			t.Errorf("node %q has element id %q, render-local id %q", node.Name, node.ElementID, node.ID)
		}
		for _, port := range node.Ports {
			ports++
			if port.ElementID == "" || port.ElementID == port.ID {
				t.Errorf("port %q of %q has element id %q, render-local id %q", port.Name, node.Name, port.ElementID, port.ID)
			}
		}
	}
	for _, edge := range out.Edges {
		edges++
		if edge.ElementID == "" {
			t.Errorf("edge %q has no element id", edge.Kind)
		}
	}
	if nodes == 0 || ports == 0 || edges == 0 {
		t.Fatalf("rendered %d nodes, %d ports, %d edges; want all three", nodes, ports, edges)
	}
	if !strings.Contains(string(raw), `"elementId":"`) {
		t.Errorf("render result has no elementId field on the wire: %s", raw)
	}
}
