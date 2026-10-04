package wasm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
)

// TestLandingStackModel pins what the landing page reads from
// docs/assets/opensysml-stack.sysml through sysml-engine: a clean parse, the
// stack diagram rendered through RenderView, each part's attributes and API
// port, one wire per interface, and the state sequence animated along them.
func TestLandingStackModel(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "docs", "assets", "opensysml-stack.sysml"))
	if err != nil {
		t.Fatalf("reading the landing model: %v", err)
	}
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	call := func(method string, params any, into any) {
		t.Helper()
		raw, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("encoding %s params: %v", method, err)
		}
		body := mustCall(t, eng, method, string(raw))
		if err := json.Unmarshal(body, into); err != nil {
			t.Fatalf("decoding %s: %v\n%s", method, err, body)
		}
	}

	var parsed struct {
		ModelHash   string            `json:"modelHash"`
		Diagnostics []json.RawMessage `json:"diagnostics"`
	}
	call("ParseSources", map[string]any{"documents": []map[string]string{
		{"content": string(src), "name": "opensysml-stack.sysml"},
	}}, &parsed)
	if len(parsed.Diagnostics) != 0 {
		t.Fatalf("the landing model has diagnostics: %s", parsed.Diagnostics)
	}

	type value struct {
		StringValue *string `json:"stringValue"`
		InstanceID  string  `json:"instanceId"`
	}
	type instance struct {
		ID            string `json:"id"`
		TypeSymbolID  string `json:"typeSymbolId"`
		FeatureValues map[string]struct {
			Value *value `json:"value"`
		} `json:"featureValues"`
	}
	var built struct {
		Error     string     `json:"error"`
		Instances []instance `json:"instances"`
	}
	call("Instantiate", map[string]string{"modelHash": parsed.ModelHash, "symbolId": "OpenSysMLStack::stack"}, &built)
	if built.Error != "" {
		t.Fatalf("Instantiate: %s", built.Error)
	}
	byID := map[string]instance{}
	for _, in := range built.Instances {
		byID[in.ID] = in
	}
	str := func(in instance, name string) string {
		if fv, ok := in.FeatureValues[name]; ok && fv.Value != nil && fv.Value.StringValue != nil {
			return *fv.Value.StringValue
		}
		return ""
	}
	ref := func(in instance, name string) string {
		if fv, ok := in.FeatureValues[name]; ok && fv.Value != nil {
			return fv.Value.InstanceID
		}
		return ""
	}

	var root instance
	for _, in := range built.Instances {
		if in.TypeSymbolID == "OpenSysMLStack::stack" {
			root = in
		}
	}
	labels := map[string]string{}
	portOwner := map[string]string{}
	var interfaces []string
	for name := range root.FeatureValues {
		in, ok := byID[ref(root, name)]
		if !ok {
			continue
		}
		switch {
		case str(in, "label") != "":
			for _, attr := range []string{"label", "kind", "lang", "role", "repo"} {
				if str(in, attr) == "" {
					t.Errorf("part %s has no %s", name, attr)
				}
			}
			labels[name] = str(in, "label")
			portOwner[ref(in, "api")] = name
		case ref(in, "client") != "":
			interfaces = append(interfaces, name)
		}
	}
	wantLabels := map[string]string{
		"opensysml": "OpenSysML",
		"toolkit":   "sysml-toolkit",
		"pilot":     "SysML v2 Pilot Implementation",
		"flexo":     "Flexo MMS",
	}
	if !reflect.DeepEqual(labels, wantLabels) {
		t.Errorf("parts: got %v, want %v", labels, wantLabels)
	}
	var wires []string
	for _, name := range interfaces {
		in := byID[ref(root, name)]
		wires = append(wires, portOwner[ref(in, "client")]+","+portOwner[ref(in, "store")])
	}
	sort.Strings(wires)
	if want := []string{"opensysml,flexo", "pilot,flexo", "toolkit,flexo"}; !reflect.DeepEqual(wires, want) {
		t.Errorf("wires: got %v, want %v", wires, want)
	}

	var diagram engine.JRenderViewResponse
	call("RenderView", map[string]string{
		"modelHash": parsed.ModelHash,
		"view":      "#interconnection:OpenSysMLStack::stack",
	}, &diagram)
	if diagram.Kind != "interconnection" {
		t.Fatalf("landing diagram kind = %q, want interconnection", diagram.Kind)
	}
	if diagram.Notices == nil || diagram.Nodes == nil || diagram.Edges == nil {
		t.Fatalf("landing diagram arrays are null: notices=%#v nodes=%#v edges=%#v",
			diagram.Notices, diagram.Nodes, diagram.Edges)
	}
	nodesByID := make(map[string]engine.JRenderNode, len(diagram.Nodes))
	partsByName := make(map[string]engine.JRenderNode)
	for _, node := range diagram.Nodes {
		nodesByID[node.ID] = node
		if node.Kind == "part" && node.Type == "Project" {
			partsByName[node.Name] = node
		}
	}
	var diagramRoot engine.JRenderNode
	for _, node := range diagram.Nodes {
		if node.Name == "stack" || node.Name == "OpenSysMLStack::stack" {
			diagramRoot = node
			break
		}
	}
	if diagramRoot.ID == "" || diagramRoot.Parent != "" {
		t.Fatalf("stack diagram root = %+v, want a root node", diagramRoot)
	}
	wantParts := []string{"opensysml", "toolkit", "pilot", "flexo"}
	if len(partsByName) != len(wantParts) {
		t.Errorf("Project nodes = %+v, want four children of stack", partsByName)
	}
	portIDs := map[string]string{}
	for _, name := range wantParts {
		part, ok := partsByName[name]
		if !ok {
			t.Errorf("landing diagram has no Project part %q: %+v", name, partsByName)
			continue
		}
		if part.Parent != diagramRoot.ID {
			t.Errorf("part %s parent = %q, want stack %q", name, part.Parent, diagramRoot.ID)
		}
		if len(part.Ports) != 1 || part.Ports[0].Name != "api" {
			t.Errorf("part %s ports = %+v, want exactly one api port", name, part.Ports)
			continue
		}
		portIDs[name] = part.Ports[0].ID
	}
	edgesByLabel := make(map[string]engine.JRenderEdge, len(diagram.Edges))
	for _, edge := range diagram.Edges {
		edgesByLabel[edge.Label] = edge
	}
	wantEdges := map[string]string{
		"opensysml_flexo": "opensysml",
		"toolkit_flexo":   "toolkit",
		"pilot_flexo":     "pilot",
	}
	if len(diagram.Edges) != len(wantEdges) {
		t.Errorf("landing diagram edges = %+v, want three connector edges", diagram.Edges)
	}
	for label, client := range wantEdges {
		edge, ok := edgesByLabel[label]
		if !ok {
			t.Errorf("landing diagram has no edge %q: %+v", label, edgesByLabel)
			continue
		}
		if edge.From != partsByName[client].ID || edge.FromPort != portIDs[client] ||
			edge.ToPort != portIDs["flexo"] {
			t.Errorf("edge %s = %+v, want %s api -> flexo api", label, edge, client)
		}
		target := nodesByID[edge.To]
		for target.ID != "" && target.ID != partsByName["flexo"].ID {
			if target.Parent == "" {
				break
			}
			target = nodesByID[target.Parent]
		}
		if target.ID != partsByName["flexo"].ID {
			t.Errorf("edge %s ends at %s, which is not flexo or nested in flexo", label, edge.To)
		}
	}
	t.Logf("RenderView landing nodes (id kind name parent):")
	for _, node := range diagram.Nodes {
		t.Logf("  %s %s %s %s", node.ID, node.Kind, node.Name, node.Parent)
	}
	t.Logf("RenderView landing edges: %+v; API port ids: %+v", diagram.Edges, portIDs)

	var journey struct {
		StatesVisited []string `json:"statesVisited"`
		Error         string   `json:"error"`
	}
	call("ExecuteState", map[string]any{
		"modelHash":            parsed.ModelHash,
		"stateMachineSymbolId": "OpenSysMLStack::ModelJourney",
		"events":               []string{"commit", "pull", "push", "check"},
	}, &journey)
	if journey.Error != "" {
		t.Fatalf("ExecuteState: %s", journey.Error)
	}
	if want := []string{"opensysml", "flexo", "toolkit", "flexo", "pilot"}; !reflect.DeepEqual(journey.StatesVisited, want) {
		t.Errorf("journey: got %v, want %v", journey.StatesVisited, want)
	}
}
