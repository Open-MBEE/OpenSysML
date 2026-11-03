package lsp

import (
	"encoding/json"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

func TestRenderMixedCarriesConnectorPortData(t *testing.T) {
	const mixed = `package Demo {
    private import OpenSysMLRenderings::*;
    port def Signal;
    part def Sender { port out1 : Signal; }
    part def Receiver { port in1 : Signal; port spare : Signal; }
    part def Plant {
        part sender : Sender;
        part receiver : Receiver;
        connection wire : Connection connect sender.out1 to receiver.in1;
    }
    connection def Connection {
        end part a;
        end part b;
    }
    part def Person;
    use case def Review {
        actor reviewer : Person;
        objective { doc /* Review the plant. */ }
    }
    view link { expose Demo::Plant; render asMixedDiagram; }
    view caseView { expose Demo::Review; render asCaseDiagram; }
}
`
	s, docURI := renderServer(t, "mixed-ports.sysml", mixed)
	renderWithPorts := func(t *testing.T, ports string) *RenderResult {
		t.Helper()
		raw, err := call(t, s, MethodRender, &RenderParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			View:         "Demo::link", Ports: ports,
		})
		if err != nil {
			t.Fatalf("render mixed view with ports=%q: %v", ports, err)
		}
		var out RenderResult
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode mixed render result: %v", err)
		}
		return &out
	}
	for _, display := range []struct {
		name string
		want int
	}{
		{name: "", want: 2},
		{name: "full", want: 3},
	} {
		t.Run(map[string]string{"": "minimal", "full": "full"}[display.name], func(t *testing.T) {
			out := renderWithPorts(t, display.name)
			portsByNode := map[string]map[string]string{}
			portNames := map[string]string{}
			for _, node := range out.Nodes {
				for _, port := range node.Ports {
					if portsByNode[node.ID] == nil {
						portsByNode[node.ID] = map[string]string{}
					}
					portsByNode[node.ID][port.ID] = port.Name
					portNames[port.Name] = port.ID
				}
			}
			if len(portNames) != display.want {
				t.Fatalf("mixed node ports = %v, want %d pins", portNames, display.want)
			}
			if display.name == "" && portNames["spare"] != "" {
				t.Errorf("minimal mixed ports include unconnected spare pin: %v", portNames)
			}
			// The connector, beside the membership edge the mixed rendering
			// draws from Plant to each of its parts.
			var connectors []RenderEdge
			for _, edge := range out.Edges {
				if edge.Kind == "connection" {
					connectors = append(connectors, edge)
				}
			}
			if len(connectors) != 1 || len(out.Edges) != 3 {
				t.Fatalf("mixed edges = %+v, want one connector and two part memberships", out.Edges)
			}
			edge := connectors[0]
			if edge.FromPort == "" || portsByNode[edge.From][edge.FromPort] != "out1" ||
				edge.ToPort == "" || portsByNode[edge.To][edge.ToPort] != "in1" {
				t.Errorf("mixed edge ports do not resolve in endpoint node arrays: edge=%+v ports=%v", edge, portsByNode)
			}
			if !strings.Contains(out.Artifact, edge.FromPort) || !strings.Contains(out.Artifact, edge.ToPort) {
				t.Errorf("mixed artifact does not draw connector pins %s and %s:\n%s", edge.FromPort, edge.ToPort, out.Artifact)
			}
			assertEngineRenderMatchesLSP(t, mixed, out, display.name, nil)
		})
	}

	caseView := render(t, s, docURI, "Demo::caseView")
	for _, node := range caseView.Nodes {
		if len(node.Ports) != 0 {
			t.Errorf("case node %s unexpectedly has ports: %+v", node.ID, node.Ports)
		}
	}
	for _, edge := range caseView.Edges {
		if edge.FromPort != "" || edge.ToPort != "" {
			t.Errorf("case edge unexpectedly refers to ports: %+v", edge)
		}
	}
}
