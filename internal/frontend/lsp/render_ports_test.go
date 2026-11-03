package lsp

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// A request's `ports` chooses how much of a part's ports an interconnection or
// mixed view draws — minimal, the default, the ports a connector ends at, named
// alone, or full, every port typed — the displays listed under the capability;
// a name that is neither is refused with the two there are.
func TestRenderTakesAPortDisplay(t *testing.T) {
	const ported = `package Demo {
    port def Signal;
    part def Sender { port out1 : Signal; }
    part def Receiver { port in1 : ~Signal; port spare : Signal; }
    part def Link {
        part sender : Sender;
        part receiver : Receiver;
        interface wire connect sender.out1 to receiver.in1;
    }
    view link { expose Demo::Link::*; render Views::asInterconnectionDiagram; }
}
`
	s, docURI := renderServer(t, "ported.sysml", ported)
	res, err := s.Initialize(context.Background(), &protocol.InitializeParams{})
	if err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
	experimental := experimentalCaps(t, res)
	if advertised, _ := experimental[RenderPortsCapability].([]string); !slices.Equal(advertised, []string{"minimal", "full"}) {
		t.Fatalf("%s = %#v, want minimal then full", RenderPortsCapability, experimental[RenderPortsCapability])
	}
	if advertised, _ := experimental[RenderLinesCapability].([]string); !slices.Equal(advertised, []string{"splines", "ortho", "polyline"}) {
		t.Fatalf("%s = %#v, want splines, ortho then polyline", RenderLinesCapability, experimental[RenderLinesCapability])
	}
	if advertised, _ := experimental[RenderCompartmentsCapability].([]string); !slices.Equal(advertised, []string{"positioned", "most", "ports"}) {
		t.Fatalf("%s = %#v, want positioned, most then ports", RenderCompartmentsCapability, experimental[RenderCompartmentsCapability])
	}
	if advertised, _ := experimental[RenderOverlaysCapability].([]string); !slices.Equal(advertised, []string{"verdicts"}) {
		t.Fatalf("%s = %#v, want verdicts", RenderOverlaysCapability, experimental[RenderOverlaysCapability])
	}
	render := func(t *testing.T, ports string) *RenderResult {
		t.Helper()
		raw, err := call(t, s, MethodRender, &RenderParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			View:         "Demo::link", Form: string(view.FormPlantUML), Ports: ports, Show: []string{"typings"},
		})
		if err != nil {
			t.Fatalf("render ports=%q: %v", ports, err)
		}
		var out RenderResult
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode render result: %v", err)
		}
		return &out
	}
	minimalResult := render(t, "")
	minimal := minimalResult.Artifact
	for _, want := range []string{`port "^out1" as `, `port "^in1" as `} {
		if !strings.Contains(minimal, want) {
			t.Errorf("the default lacks %q:\n%s", want, minimal)
		}
	}
	minimalPorts := map[string]RenderPort{}
	for _, node := range minimalResult.Nodes {
		for _, port := range node.Ports {
			minimalPorts[port.Name] = port
		}
	}
	if len(minimalPorts) != 2 || minimalPorts["out1"].ID == "" || minimalPorts["in1"].ID == "" {
		t.Errorf("default node ports = %+v, want only connected out1 and in1 pins", minimalPorts)
	}
	if !minimalPorts["out1"].Inherited || !minimalPorts["in1"].Inherited {
		t.Errorf("default node ports = %+v, want the pins the parts have from their definitions marked inherited", minimalPorts)
	}
	if len(minimalResult.Edges) != 1 || minimalResult.Edges[0].FromPort != minimalPorts["out1"].ID ||
		minimalResult.Edges[0].ToPort != minimalPorts["in1"].ID {
		t.Errorf("default edges = %+v, want endpoints at out1 and in1", minimalResult.Edges)
	}
	if strings.Contains(minimal, "spare") || strings.Contains(minimal, "Signal") {
		t.Errorf("the default drew an unconnected port or a type:\n%s", minimal)
	}
	if render(t, "minimal").Artifact != minimal {
		t.Errorf("ports=minimal differs from the default")
	}
	fullResult := render(t, "full")
	full := fullResult.Artifact
	for _, want := range []string{`port "^out1 : Signal" as `, `port "^in1 : <U+007E>Signal" as `, `port "^spare : Signal" as `} {
		if !strings.Contains(full, want) {
			t.Errorf("ports=full lacks %q:\n%s", want, full)
		}
	}
	fullPorts := map[string]RenderPort{}
	for _, node := range fullResult.Nodes {
		for _, port := range node.Ports {
			fullPorts[port.Name] = port
		}
	}
	if len(fullPorts) != 3 || fullPorts["spare"].Type != "Signal" ||
		fullPorts["in1"].Type != "~Signal" || fullPorts["out1"].Type != "Signal" {
		t.Errorf("full node ports = %+v, want all three typed pins", fullPorts)
	}
	if len(fullResult.Edges) != 1 || fullResult.Edges[0].FromPort != fullPorts["out1"].ID ||
		fullResult.Edges[0].ToPort != fullPorts["in1"].ID {
		t.Errorf("full edges = %+v, want endpoints at out1 and in1", fullResult.Edges)
	}
	assertEngineRenderMatchesLSP(t, ported, minimalResult, "", []string{"typings"})
	assertEngineRenderMatchesLSP(t, ported, fullResult, "full", []string{"typings"})
	_, err = call(t, s, MethodRender, &RenderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "Demo::link", Form: string(view.FormDot), Ports: "all",
	})
	want := `unknown port display "all"; the displays are minimal, full`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v, want it to refuse the port display by name", err)
	}
}
