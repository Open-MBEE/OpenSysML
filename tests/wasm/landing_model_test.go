package wasm

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
)

var updateLanding = flag.Bool("update-landing", false, "rewrite editors/vscode/src/landing/stack.json")

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
	call := func(method string, params any, into any) []byte {
		t.Helper()
		raw, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("encoding %s params: %v", method, err)
		}
		body := mustCall(t, eng, method, string(raw))
		if err := json.Unmarshal(body, into); err != nil {
			t.Fatalf("decoding %s: %v\n%s", method, err, body)
		}
		return body
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
	instantiateBody := call("Instantiate", map[string]string{"modelHash": parsed.ModelHash, "symbolId": "OpenSysMLStack::stack"}, &built)
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
	renderBody := call("RenderView", map[string]string{
		"modelHash": parsed.ModelHash,
		"view":      "#interconnection:OpenSysMLStack::stack",
		"ports":     "minimal",
	}, &diagram)
	pinLandingFixture(t, parsed.ModelHash, renderBody, instantiateBody)
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
		StatesVisited []string             `json:"statesVisited"`
		Trace         []engine.JTraceEvent `json:"trace"`
		Error         string               `json:"error"`
	}
	call("ExecuteState", map[string]any{
		"modelHash":            parsed.ModelHash,
		"stateMachineSymbolId": "OpenSysMLStack::ModelJourney",
		"events":               []string{"Commit", "Pull", "Push", "Check"},
	}, &journey)
	if journey.Error != "" {
		t.Fatalf("ExecuteState: %s", journey.Error)
	}
	if len(journey.Trace) != 0 {
		t.Fatalf("unrequested trace has %d records", len(journey.Trace))
	}
	if want := []string{"start", "opensysml", "flexo", "toolkit", "flexo", "pilot"}; !reflect.DeepEqual(journey.StatesVisited, want) {
		t.Errorf("journey: got %v, want %v", journey.StatesVisited, want)
	}

	var tracedJourney struct {
		Trace []engine.JTraceEvent `json:"trace"`
		Error string               `json:"error"`
	}
	call("ExecuteState", map[string]any{
		"modelHash":            parsed.ModelHash,
		"stateMachineSymbolId": "OpenSysMLStack::ModelJourney",
		"events":               []string{"Commit", "Pull", "Push", "Check"},
		"trace":                true,
	}, &tracedJourney)
	if tracedJourney.Error != "" {
		t.Fatalf("traced ExecuteState: %s", tracedJourney.Error)
	}
	wantTrace := []engine.JTraceEvent{
		{Kind: "entry", State: "start"},
		{Kind: "choice", Alternatives: []string{"1->opensysml", "2->flexo"}, Taken: "1->opensysml"},
		{Kind: "exit", State: "start"},
		{Kind: "entry", State: "opensysml"},
		{Kind: "transition", From: "start", To: "opensysml"},
		{Kind: "accept", Event: "Commit"},
		{Kind: "exit", State: "opensysml"},
		{Kind: "entry", State: "flexo"},
		{Kind: "transition", From: "opensysml", To: "flexo", Event: "accept Commit"},
		{Kind: "accept", Event: "Pull"},
		{Kind: "choice", Alternatives: []string{"1->toolkit", "2->pilot", "3->opensysml"}, Taken: "1->toolkit"},
		{Kind: "exit", State: "flexo"},
		{Kind: "entry", State: "toolkit"},
		{Kind: "transition", From: "flexo", To: "toolkit", Event: "accept Pull"},
		{Kind: "accept", Event: "Push"},
		{Kind: "exit", State: "toolkit"},
		{Kind: "entry", State: "flexo"},
		{Kind: "transition", From: "toolkit", To: "flexo", Event: "accept Push"},
		{Kind: "accept", Event: "Check"},
		{Kind: "exit", State: "flexo"},
		{Kind: "entry", State: "pilot"},
		{Kind: "transition", From: "flexo", To: "pilot", Event: "accept Check"},
	}
	if len(tracedJourney.Trace) != len(wantTrace) {
		t.Fatalf("traced journey has %d records, want %d: %+v", len(tracedJourney.Trace), len(wantTrace), tracedJourney.Trace)
	}
	for i, want := range wantTrace {
		got := tracedJourney.Trace[i]
		if got.Kind != want.Kind || got.State != want.State || got.From != want.From ||
			got.To != want.To || got.Event != want.Event ||
			!reflect.DeepEqual(got.Alternatives, want.Alternatives) || got.Taken != want.Taken {
			t.Errorf("trace[%d] = %+v, want kind=%q state=%q from=%q to=%q event=%q alternatives=%v taken=%q",
				i, got, want.Kind, want.State, want.From, want.To, want.Event, want.Alternatives, want.Taken)
		}
	}

	var seedTwo struct {
		StatesVisited []string             `json:"statesVisited"`
		Trace         []engine.JTraceEvent `json:"trace"`
		Error         string               `json:"error"`
	}
	call("ExecuteState", map[string]any{
		"modelHash":            parsed.ModelHash,
		"stateMachineSymbolId": "OpenSysMLStack::ModelJourney",
		"events":               []string{"Commit", "Pull", "Push", "Check"},
		"schedule":             "seed:2",
		"trace":                true,
	}, &seedTwo)
	if seedTwo.Error != "" {
		t.Fatalf("seed:2 ExecuteState: %s", seedTwo.Error)
	}
	if want := []string{"start", "flexo", "opensysml"}; !reflect.DeepEqual(seedTwo.StatesVisited, want) {
		t.Errorf("seed:2 journey: got %v, want %v", seedTwo.StatesVisited, want)
	}
	commitIndex := -1
	for i, event := range seedTwo.Trace {
		if event.Kind == "accept" && event.Event == "Commit" {
			commitIndex = i
			break
		}
	}
	if commitIndex < 0 || commitIndex+1 >= len(seedTwo.Trace) ||
		seedTwo.Trace[commitIndex+1].Kind != "accept" || seedTwo.Trace[commitIndex+1].Event != "Pull" {
		t.Errorf("seed:2 expected consecutive Commit and Pull accepts, got %+v", seedTwo.Trace)
	}
	var lastTransition *engine.JTraceEvent
	for i := range seedTwo.Trace {
		if seedTwo.Trace[i].Kind == "transition" {
			lastTransition = &seedTwo.Trace[i]
		}
	}
	if lastTransition == nil || lastTransition.From != "flexo" || lastTransition.To != "opensysml" ||
		lastTransition.Event != "accept Pull" {
		t.Errorf("seed:2 last transition = %+v, want flexo -> opensysml on accept Pull", lastTransition)
	}

	var seedFive struct {
		StatesVisited []string `json:"statesVisited"`
		Error         string   `json:"error"`
	}
	call("ExecuteState", map[string]any{
		"modelHash":            parsed.ModelHash,
		"stateMachineSymbolId": "OpenSysMLStack::ModelJourney",
		"events":               []string{"Commit", "Pull", "Push", "Check"},
		"schedule":             "seed:5",
	}, &seedFive)
	if seedFive.Error != "" {
		t.Fatalf("seed:5 ExecuteState: %s", seedFive.Error)
	}
	if want := []string{"start", "flexo", "pilot", "flexo", "pilot"}; !reflect.DeepEqual(seedFive.StatesVisited, want) {
		t.Errorf("seed:5 journey: got %v, want %v", seedFive.StatesVisited, want)
	}
}

type landingFixture struct {
	Hash      string          `json:"hash"`
	Render    json.RawMessage `json:"render"`
	Instances json.RawMessage `json:"instances"`
}

func pinLandingFixture(t *testing.T, hash string, render, instantiate []byte) {
	t.Helper()
	var response struct {
		Instances json.RawMessage `json:"instances"`
	}
	if err := json.Unmarshal(instantiate, &response); err != nil {
		t.Fatalf("decoding Instantiate fixture data: %v\n%s", err, instantiate)
	}
	fixture := landingFixture{
		Hash:      hash,
		Render:    json.RawMessage(render),
		Instances: response.Instances,
	}
	path := filepath.Join("..", "..", "editors", "vscode", "src", "landing", "stack.json")
	if *updateLanding {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("creating landing fixture directory: %v", err)
		}
		data, err := json.MarshalIndent(fixture, "", "  ")
		if err != nil {
			t.Fatalf("encoding landing fixture: %v", err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatalf("writing landing fixture: %v", err)
		}
		t.Logf("updated %s", path)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v; run go test -count=1 ./tests/wasm -run '^TestLandingStackModel$' -update-landing", path, err)
	}
	var want landingFixture
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
	var gotRender, wantRender, gotInstances, wantInstances any
	if err := json.Unmarshal(fixture.Render, &gotRender); err != nil {
		t.Fatalf("decoding current RenderView result: %v", err)
	}
	if err := json.Unmarshal(want.Render, &wantRender); err != nil {
		t.Fatalf("decoding fixture RenderView result: %v", err)
	}
	if err := json.Unmarshal(fixture.Instances, &gotInstances); err != nil {
		t.Fatalf("decoding current Instantiate instances: %v", err)
	}
	if err := json.Unmarshal(want.Instances, &wantInstances); err != nil {
		t.Fatalf("decoding fixture Instantiate instances: %v", err)
	}
	if fixture.Hash != want.Hash || !reflect.DeepEqual(gotRender, wantRender) || !reflect.DeepEqual(gotInstances, wantInstances) {
		t.Fatalf("%s is stale; run go test -count=1 ./tests/wasm -run '^TestLandingStackModel$' -update-landing", path)
	}
}
