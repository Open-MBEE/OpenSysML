package wasm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/combined"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
	"google.golang.org/protobuf/encoding/protojson"
)

// TestLandingMissionPhasesModel pins what the landing page's clocked scene reads from
// examples/runtime-showcase/mission-sequence.sysml through sysml-engine: the MissionPhases
// state view, the mission instantiated, and the two clocked runs the scene offers.
func TestLandingMissionPhasesModel(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "examples", "runtime-showcase", "mission-sequence.sysml"))
	if err != nil {
		t.Fatalf("reading the mission sequence model: %v", err)
	}
	server, err := combined.New("landing-test")
	if err != nil {
		t.Fatalf("combined.New: %v", err)
	}
	call := func(method string, params any, into any) []byte {
		t.Helper()
		raw, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("encoding %s params: %v", method, err)
		}
		body, err := server.Call(context.Background(), method, raw)
		if err != nil {
			t.Fatalf("%s refused: %v", method, err)
		}
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
		{"content": string(src), "name": "mission-sequence.sysml"},
	}}, &parsed)
	if len(parsed.Diagnostics) != 0 {
		t.Fatalf("the mission sequence model has diagnostics: %s", parsed.Diagnostics)
	}

	// The page lifts the states out of the machine's box and keys each by the name the
	// trace records carry, so the view must draw every state under the one state def.
	var diagram engine.JRenderViewResponse
	renderBody := call("RenderView", map[string]any{
		"modelHash": parsed.ModelHash,
		"view":      "#state:MissionSequence::MissionPhases",
		"ports":     "minimal",
		"show":      []string{"edge-labels"},
	}, &diagram)
	if diagram.Kind != "state" {
		t.Fatalf("phases diagram kind = %q, want state", diagram.Kind)
	}
	if err := protojson.Unmarshal(renderBody, &pb.RenderViewResponse{}); err != nil {
		t.Fatalf("phases diagram is not a protojson RenderViewResponse: %v", err)
	}
	var machine engine.JRenderNode
	kinds := map[string]string{}
	for _, node := range diagram.Nodes {
		if node.Parent == "" {
			if machine.ID != "" {
				t.Fatalf("phases diagram draws two top-level nodes, %+v and %+v", machine, node)
			}
			machine = node
			continue
		}
		kinds[node.Name] = node.Kind
	}
	if machine.Kind != "state def" || machine.Name != "MissionSequence::MissionPhases" {
		t.Fatalf("phases diagram root = %+v, want the MissionPhases state def", machine)
	}
	wantKinds := map[string]string{
		"": "start", "prelaunch": "state", "translunarCoast": "state", "lunarOrbit": "state",
		"onSurface": "state", "transEarthCoast": "state", "recovered": "state", "done": "final",
	}
	if !reflect.DeepEqual(kinds, wantKinds) {
		t.Errorf("phases diagram states: got %v, want %v", kinds, wantKinds)
	}
	for _, node := range diagram.Nodes {
		if node.Parent != "" && node.Parent != machine.ID {
			t.Errorf("state %s is drawn under %s, want under the state def", node.Name, node.Parent)
		}
		if node.Name == "prelaunch" && node.Detail != "initial" {
			t.Errorf("prelaunch detail = %q, want initial", node.Detail)
		}
	}
	nodesByID := make(map[string]engine.JRenderNode, len(diagram.Nodes))
	for _, node := range diagram.Nodes {
		nodesByID[node.ID] = node
	}
	labels := map[string]string{}
	for _, edge := range diagram.Edges {
		if edge.Kind != "transition" {
			t.Errorf("phases diagram wires %s %s -> %s, want only transitions", edge.Kind, edge.From, edge.To)
		}
		labels[nodesByID[edge.From].Name+"->"+nodesByID[edge.To].Name] = edge.Label
	}
	wantLabels := map[string]string{
		"->prelaunch": "", "prelaunch->translunarCoast": "after 9000 [s]", "translunarCoast->lunarOrbit": "after 270000 [s]",
		"lunarOrbit->onSurface": "after 100000 [s]", "onSurface->transEarthCoast": "after 78000 [s]",
		"transEarthCoast->recovered": "after 245000 [s]", "recovered->done": "",
	}
	if !reflect.DeepEqual(labels, wantLabels) {
		t.Errorf("phases diagram transitions: got %v, want %v", labels, wantLabels)
	}

	// Instantiate materializes the mission and the machine it exhibits under `phases`.
	type value struct {
		InstanceID string `json:"instanceId"`
	}
	type featureValue struct {
		Value *value `json:"value"`
	}
	type instance struct {
		ID            string                  `json:"id"`
		TypeSymbolID  string                  `json:"typeSymbolId"`
		FeatureValues map[string]featureValue `json:"featureValues"`
	}
	var built struct {
		Error     string     `json:"error"`
		Instances []instance `json:"instances"`
	}
	instantiateBody := call("Instantiate", map[string]string{"modelHash": parsed.ModelHash, "symbolId": "MissionSequence::mission"}, &built)
	if built.Error != "" {
		t.Fatalf("Instantiate MissionSequence::mission: %s", built.Error)
	}
	pinLandingFixture(t, "phases.json", "TestLandingMissionPhasesModel", parsed.ModelHash, renderBody, instantiateBody)
	byID := map[string]instance{}
	for _, in := range built.Instances {
		byID[in.ID] = in
	}
	if len(built.Instances) == 0 || built.Instances[0].TypeSymbolID != "MissionSequence::mission" {
		t.Fatalf("Instantiate answered %d instances, want the mission first", len(built.Instances))
	}
	phases := built.Instances[0].FeatureValues["phases"]
	if phases.Value == nil || byID[phases.Value.InstanceID].TypeSymbolID != "MissionSequence::MissionPhases" {
		t.Errorf("mission.phases = %+v, want an instance of MissionPhases", phases)
	}

	// The scene's Run and its variant: the machine performed by the mission on the clock.
	type quantity struct {
		RealMagnitude float64 `json:"realMagnitude"`
		Unit          string  `json:"unit"`
	}
	type record struct {
		Kind string `json:"kind"`
		To   string `json:"to"`
		Time struct {
			Quantity *quantity `json:"quantity"`
		} `json:"time"`
	}
	for _, tc := range []struct {
		advance   float64
		wantLast  string
		wantFired map[string]float64
	}{
		{400000, "onSurface", map[string]float64{"translunarCoast": 9000, "lunarOrbit": 279000, "onSurface": 379000}},
		{800000, "done", map[string]float64{"translunarCoast": 9000, "lunarOrbit": 279000, "onSurface": 379000, "transEarthCoast": 457000, "recovered": 702000, "done": 702000}},
	} {
		var ran struct {
			Error         string   `json:"error"`
			StatesVisited []string `json:"statesVisited"`
			FinalTime     float64  `json:"finalTime"`
			Trace         []record `json:"trace"`
		}
		call("ExecuteState", map[string]any{
			"modelHash": parsed.ModelHash, "stateMachineSymbolId": "MissionSequence::MissionPhases",
			"performerSymbolId": "MissionSequence::mission", "advance": tc.advance, "trace": true,
		}, &ran)
		if ran.Error != "" {
			t.Fatalf("ExecuteState advance %g: %s", tc.advance, ran.Error)
		}
		if last := ran.StatesVisited[len(ran.StatesVisited)-1]; last != tc.wantLast || ran.FinalTime != tc.advance {
			t.Errorf("advance %g ended in %s at t=%g, want %s at t=%g", tc.advance, last, ran.FinalTime, tc.wantLast, tc.advance)
		}
		fired := map[string]float64{}
		for _, rec := range ran.Trace {
			if rec.Time.Quantity == nil || rec.Time.Quantity.Unit != "s" {
				t.Errorf("advance %g: %s record timed %+v, want a quantity in seconds", tc.advance, rec.Kind, rec.Time)
				continue
			}
			if rec.Kind == "transition" {
				fired[rec.To] = rec.Time.Quantity.RealMagnitude
			}
		}
		if !reflect.DeepEqual(fired, tc.wantFired) {
			t.Errorf("advance %g transition instants: got %v, want %v", tc.advance, fired, tc.wantFired)
		}
	}
}
