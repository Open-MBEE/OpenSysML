package lsp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"go.lsp.dev/protocol"
)

// A request's compartments folds a tree's owned members into compartment rows
// of their owner: most folds every usage, ports only the ports, the default
// nothing of an unpositioned tree; a folding there is not is refused by name.
func TestRenderFoldsCompartmentsOnRequest(t *testing.T) {
	const folded = `package Demo {
    attribute def Label; attribute def Reading;
    port def Hub { port bearing : Hub; }
    part def Sensor { attribute label : Label; attribute reading : Reading; }
    part def Controller { part sensor : Sensor; port hub : Hub; }
    view overview { expose Demo::Sensor; expose Demo::Controller; }
}
`
	s, docURI := renderServer(t, "folded.sysml", folded)
	result := func(t *testing.T, compartments string) RenderResult {
		t.Helper()
		raw, err := call(t, s, MethodRender, &RenderParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			View:         "Demo::overview", Form: string(view.FormDot), Compartments: compartments,
		})
		if err != nil {
			t.Fatalf("render compartments=%q: %v", compartments, err)
		}
		var out RenderResult
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode render result: %v", err)
		}
		return out
	}
	render := func(t *testing.T, compartments string) string {
		t.Helper()
		return result(t, compartments).Artifact
	}
	plain := render(t, "")
	if !strings.Contains(plain, "<b>sensor</b>") || strings.Contains(plain, "compartment rows") {
		t.Errorf("the default draws every member as a node:\n%s", plain)
	}
	if got := render(t, "positioned"); got != plain {
		t.Errorf("positioned differs from the default:\n%s", got)
	}
	most := render(t, "most")
	for _, want := range []string{"label : Label<br/>reading : Reading", "sensor : Sensor", "hub : Hub<br/>hub.bearing : Hub", `[headlabel="sensor", dir=back, arrowtail=diamond]`, "// not represented: 5 member(s) drawn as compartment rows of their owner, and 2 node(s) under them left undrawn\n"} {
		if !strings.Contains(most, want) {
			t.Errorf("most lacks %q:\n%s", want, most)
		}
	}
	if strings.Contains(most, "<b>sensor</b>") {
		t.Errorf("most keeps sensor a node:\n%s", most)
	}
	rows := map[string]string{}
	for _, node := range result(t, "most").Nodes {
		if node.Compartment != "" {
			rows[node.Name] = node.Compartment + " of " + node.Parent
		}
	}
	if len(rows) != 5 || rows["sensor"] == "" || rows["hub.bearing"] == "" || !strings.HasPrefix(rows["label"], "attributes of ") {
		t.Errorf("most nodes carry the rows %v, want each folded member as a row of its owner's compartment", rows)
	}
	for _, node := range result(t, "").Nodes {
		if node.Compartment != "" {
			t.Errorf("the default lists %s as a row of %q", node.Name, node.Compartment)
		}
	}
	ports := render(t, "ports")
	if !strings.Contains(ports, "hub : Hub<br/>hub.bearing : Hub") || !strings.Contains(ports, "<b>sensor</b>") {
		t.Errorf("ports folds the ports and keeps the parts nodes:\n%s", ports)
	}
	_, err := call(t, s, MethodRender, &RenderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "Demo::overview", Form: string(view.FormDot), Compartments: "rows",
	})
	want := `unknown compartment folding "rows"; the foldings are positioned, most, ports`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v, want it to refuse the folding by name", err)
	}
}
