package wasm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/combined"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
	"google.golang.org/protobuf/encoding/protojson"
)

// TestLandingLunarMissionModel pins what the landing page's action scene reads from
// examples/runtime-showcase/mission-sequence.sysml through sysml-engine: the LunarMission
// action view, apollo11 instantiated, and the four traced runs the scene and its toggles perform.
func TestLandingLunarMissionModel(t *testing.T) {
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

	// The page lifts the nodes out of the action's box and keys each by the name the trace
	// records carry, so the view must draw every node under the one action def.
	var diagram engine.JRenderViewResponse
	renderBody := call("RenderView", map[string]any{
		"modelHash": parsed.ModelHash,
		"view":      "#action:MissionSequence::LunarMission",
		"ports":     "minimal",
		"show":      []string{"edge-labels"},
	}, &diagram)
	if diagram.Kind != "action" {
		t.Fatalf("mission diagram kind = %q, want action", diagram.Kind)
	}
	if err := protojson.Unmarshal(renderBody, &pb.RenderViewResponse{}); err != nil {
		t.Fatalf("mission diagram is not a protojson RenderViewResponse: %v", err)
	}
	var action engine.JRenderNode
	kinds := map[string]string{}
	for _, node := range diagram.Nodes {
		if node.Parent == "" {
			if action.ID != "" {
				t.Fatalf("mission diagram draws two top-level nodes, %+v and %+v", action, node)
			}
			action = node
			continue
		}
		kinds[node.Name] = node.Kind
	}
	if action.Kind != "action def" || action.Name != "MissionSequence::LunarMission" {
		t.Fatalf("mission diagram root = %+v, want the LunarMission action def", action)
	}
	wantKinds := map[string]string{
		"start": "initial", "translunarInjection": "action", "lunarOrbitInsertion": "action", "landingGo": "decision",
		"awaitGo": "action", "descent": "action", "surfaceOperations": "action", "ascent": "action",
		"transEarthInjection": "action", "splashdown": "action", "done": "final",
	}
	if !reflect.DeepEqual(kinds, wantKinds) {
		t.Errorf("mission diagram nodes: got %v, want %v", kinds, wantKinds)
	}
	for _, node := range diagram.Nodes {
		if node.Parent != "" && node.Parent != action.ID {
			t.Errorf("node %s is drawn under %s, want under the action def", node.Name, node.Parent)
		}
	}
	nodesByID := make(map[string]engine.JRenderNode, len(diagram.Nodes))
	for _, node := range diagram.Nodes {
		nodesByID[node.ID] = node
	}
	labels := map[string]string{}
	for _, edge := range diagram.Edges {
		if edge.Kind != "succession" {
			t.Errorf("mission diagram wires %s %s -> %s, want only successions", edge.Kind, edge.From, edge.To)
		}
		labels[nodesByID[edge.From].Name+"->"+nodesByID[edge.To].Name] = edge.Label
	}
	wantLabels := map[string]string{
		"start->translunarInjection": "", "translunarInjection->lunarOrbitInsertion": "", "lunarOrbitInsertion->landingGo": "",
		"landingGo->awaitGo": "[remaining >= 3970 ['m/s']]", "landingGo->transEarthInjection": "", "awaitGo->descent": "",
		"descent->surfaceOperations": "", "surfaceOperations->ascent": "", "ascent->transEarthInjection": "",
		"transEarthInjection->splashdown": "", "splashdown->done": "",
	}
	if !reflect.DeepEqual(labels, wantLabels) {
		t.Errorf("mission diagram successions: got %v, want %v", labels, wantLabels)
	}

	// Instantiate materializes apollo11, the object the scene performs the action on.
	var built struct {
		Error     string `json:"error"`
		Instances []struct {
			TypeSymbolID string `json:"typeSymbolId"`
		} `json:"instances"`
	}
	instantiateBody := call("Instantiate", map[string]string{"modelHash": parsed.ModelHash, "symbolId": "MissionSequence::apollo11"}, &built)
	if built.Error != "" {
		t.Fatalf("Instantiate MissionSequence::apollo11: %s", built.Error)
	}
	if len(built.Instances) == 0 || built.Instances[0].TypeSymbolID != "MissionSequence::apollo11" {
		t.Fatalf("Instantiate answered %d instances, want apollo11 first", len(built.Instances))
	}
	pinLandingFixture(t, "mission.json", "TestLandingLunarMissionModel", parsed.ModelHash, renderBody, instantiateBody)

	// The scene's Run and its toggles: the action traced on each performer, with and without the event.
	type record struct {
		Kind    string          `json:"kind"`
		State   string          `json:"state"`
		To      string          `json:"to"`
		Event   string          `json:"event"`
		Taken   string          `json:"taken"`
		Feature string          `json:"feature"`
		Value   json.RawMessage `json:"value"`
		Token   string          `json:"token"`
		Text    string          `json:"text"`
	}
	type run struct {
		Error        string         `json:"error"`
		Outputs      map[string]any `json:"outputs"`
		Trace        []record       `json:"trace"`
		TraceDropped int            `json:"traceDropped"`
	}
	perform := func(action, performer string, events ...string) run {
		t.Helper()
		var ran run
		call("ExecuteAction", map[string]any{
			"modelHash": parsed.ModelHash, "actionSymbolId": action, "performerSymbolId": performer,
			"trace": true, "events": events,
		}, &ran)
		if ran.TraceDropped != 0 {
			t.Errorf("%s on %s dropped %d trace records", action, performer, ran.TraceDropped)
		}
		return ran
	}
	assigned := func(ran run) []string {
		var values []string
		for _, rec := range ran.Trace {
			if rec.Kind == "assign" {
				values = append(values, rec.State+": "+rec.Text)
			}
		}
		return values
	}
	stepsTo := func(ran run) []string {
		var nodes []string
		for _, rec := range ran.Trace {
			if rec.Kind == "step" {
				nodes = append(nodes, rec.To)
			}
		}
		return nodes
	}

	// Apollo 11 without the event stops at awaitGo, waiting for GoForDescent, and says so.
	waiting := perform("MissionSequence::LunarMission", "MissionSequence::apollo11")
	if !strings.Contains(waiting.Error, "GoForDescent") || !strings.Contains(waiting.Error, "awaitGo") && !strings.Contains(waiting.Error, "accept go") {
		t.Errorf("apollo11 without events: error %q, want the wait for GoForDescent at awaitGo", waiting.Error)
	}
	if last := waiting.Trace[len(waiting.Trace)-1]; last.Kind != "wait" || last.State != "awaitGo" || last.Event != "GoForDescent" {
		t.Errorf("apollo11 last record = %+v, want wait GoForDescent at awaitGo", last)
	}
	wantPath := []string{"start", "translunarInjection", "lunarOrbitInsertion", "landingGo", "awaitGo"}
	if got := stepsTo(waiting); !reflect.DeepEqual(got, wantPath) {
		t.Errorf("apollo11 steps %v, want %v", got, wantPath)
	}
	wantAssigned := []string{"translunarInjection: assign remaining = 6350 ['m/s']", "lunarOrbitInsertion: assign remaining = 5450 ['m/s']"}
	if got := assigned(waiting); !reflect.DeepEqual(got, wantAssigned) {
		t.Errorf("apollo11 assignments %v, want %v", got, wantAssigned)
	}
	for _, rec := range waiting.Trace {
		if rec.Kind == "decide" && (rec.State != "landingGo" || rec.Taken != "1->awaitGo") {
			t.Errorf("apollo11 decide = %+v, want landingGo taking 1->awaitGo", rec)
		}
		if rec.Kind != "complete" && rec.Token != "1" {
			t.Errorf("apollo11 %s record carries token %q, want the one token 1", rec.Kind, rec.Token)
		}
	}

	// With GoForDescent posted the accept takes it and the run completes, landed and returned.
	landed := perform("MissionSequence::LunarMission", "MissionSequence::apollo11", "GoForDescent")
	if landed.Error != "" {
		t.Fatalf("apollo11 with GoForDescent: %s", landed.Error)
	}
	wantPath = append(wantPath, "descent", "surfaceOperations", "ascent", "transEarthInjection", "splashdown", "done")
	if got := stepsTo(landed); !reflect.DeepEqual(got, wantPath) {
		t.Errorf("apollo11 with GoForDescent steps %v, want %v", got, wantPath)
	}
	wantAssigned = append(wantAssigned,
		"descent: assign remaining = 3350 ['m/s']", "surfaceOperations: assign landed = [true]",
		"ascent: assign remaining = 1480 ['m/s']", "transEarthInjection: assign remaining = 480 ['m/s']",
		"splashdown: assign returned = [true]")
	if got := assigned(landed); !reflect.DeepEqual(got, wantAssigned) {
		t.Errorf("apollo11 with GoForDescent assignments %v, want %v", got, wantAssigned)
	}
	texts := make([]string, 0, len(landed.Trace))
	for _, rec := range landed.Trace {
		texts = append(texts, rec.Text)
	}
	for _, want := range []string{"wait GoForDescent", "accept GoForDescent", "complete"} {
		if !slicesContains(texts, want) {
			t.Errorf("apollo11 with GoForDescent trace lacks %q:\n%s", want, strings.Join(texts, "\n"))
		}
	}
	if landed.Trace[len(landed.Trace)-1].Kind != "complete" {
		t.Errorf("apollo11 with GoForDescent ends on %s, want complete", landed.Trace[len(landed.Trace)-1].Kind)
	}
	for name, want := range map[string]string{"landed": "true", "returned": "true"} {
		if got := boolOutput(t, landed.Outputs, name); got != want {
			t.Errorf("apollo11 output %s = %s, want %s", name, got, want)
		}
	}

	// Apollo 8's 5500 m/s budget fails the guard: the decide takes the trans-Earth branch and no step reaches awaitGo.
	flown := perform("MissionSequence::LunarMission", "MissionSequence::apollo8")
	if flown.Error != "" {
		t.Fatalf("apollo8: %s", flown.Error)
	}
	wantPath = []string{"start", "translunarInjection", "lunarOrbitInsertion", "landingGo", "transEarthInjection", "splashdown", "done"}
	if got := stepsTo(flown); !reflect.DeepEqual(got, wantPath) {
		t.Errorf("apollo8 steps %v, want %v", got, wantPath)
	}
	for _, rec := range flown.Trace {
		if rec.Kind == "decide" && rec.Taken != "2->transEarthInjection" {
			t.Errorf("apollo8 decide took %q, want 2->transEarthInjection", rec.Taken)
		}
	}
	if got := boolOutput(t, flown.Outputs, "landed"); got != "false" {
		t.Errorf("apollo8 output landed = %s, want false", got)
	}

	// LunarMissionUnordered has no first step: the engine starts a token on each of its three
	// actions and completes, recording the order it took them in as a choice.
	unordered := perform("MissionSequence::LunarMissionUnordered", "MissionSequence::apollo11")
	if unordered.Error != "" {
		t.Fatalf("LunarMissionUnordered: %s", unordered.Error)
	}
	tokens := map[string]string{}
	var kindsSeen []string
	for _, rec := range unordered.Trace {
		kindsSeen = append(kindsSeen, rec.Kind)
		if rec.Kind == "step" {
			tokens[rec.Token] = rec.To
		}
	}
	wantTokens := map[string]string{"1": "outbound", "2": "lunarOps", "3": "returnJourney"}
	if !reflect.DeepEqual(tokens, wantTokens) {
		t.Errorf("LunarMissionUnordered started tokens %v, want %v", tokens, wantTokens)
	}
	if !slicesContains(kindsSeen, "complete") || !slicesContains(kindsSeen, "choice") {
		t.Errorf("LunarMissionUnordered records %v, want a choice over the unordered tokens and a completion", kindsSeen)
	}
}

func slicesContains(texts []string, want string) bool {
	for _, text := range texts {
		if text == want {
			return true
		}
	}
	return false
}

// boolOutput reads a protojson SysMLValue output holding one boolean, as the engine answers `landed`.
func boolOutput(t *testing.T, outputs map[string]any, name string) string {
	t.Helper()
	raw, err := json.Marshal(outputs[name])
	if err != nil {
		t.Fatalf("encoding output %s: %v", name, err)
	}
	var value pb.Value
	if err := protojson.Unmarshal(raw, &value); err != nil {
		t.Fatalf("output %s is not a Value: %v\n%s", name, err, raw)
	}
	elements := value.GetSequence().GetElements()
	if len(elements) != 1 {
		t.Fatalf("output %s holds %d values, want one: %s", name, len(elements), raw)
	}
	if b, ok := elements[0].GetKind().(*pb.Value_BoolValue); ok {
		if b.BoolValue {
			return "true"
		}
		return "false"
	}
	return string(raw)
}
