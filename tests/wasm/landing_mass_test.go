package wasm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/combined"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
	"google.golang.org/protobuf/encoding/protojson"
)

// TestLandingMassRollupModel pins what the landing page's mass roll-up scene reads
// from examples/runtime-showcase/mass-rollup.sysml through sysml-engine: a clean
// parse, the Saturn V's composition rendered through RenderView, the instances
// Instantiate materializes with their rolled-up masses, the three evaluations the
// scene reports, the roll-up the instrument unit leaves undetermined, and the one that
// completes once its usage values the mass.
func TestLandingMassRollupModel(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "examples", "runtime-showcase", "mass-rollup.sysml"))
	if err != nil {
		t.Fatalf("reading the mass roll-up model: %v", err)
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
		{"content": string(src), "name": "mass-rollup.sysml"},
	}}, &parsed)
	if len(parsed.Diagnostics) != 0 {
		t.Fatalf("the mass roll-up model has diagnostics: %s", parsed.Diagnostics)
	}

	// The page keys every box by the feature path RenderView draws it under, so the
	// composition must come back nested: the vehicle holds the stages, each its engines.
	var diagram engine.JRenderViewResponse
	renderBody := call("RenderView", map[string]any{
		"modelHash": parsed.ModelHash,
		"view":      "#interconnection:MassRollup::saturnV",
		"ports":     "minimal",
		"depth":     3,
		"show":      []string{"typings", "keywords"},
	}, &diagram)
	if diagram.Kind != "interconnection" {
		t.Fatalf("mass diagram kind = %q, want interconnection", diagram.Kind)
	}
	if err := protojson.Unmarshal(renderBody, &pb.RenderViewResponse{}); err != nil {
		t.Fatalf("mass diagram is not a protojson RenderViewResponse: %v", err)
	}
	nodesByID := make(map[string]engine.JRenderNode, len(diagram.Nodes))
	for _, node := range diagram.Nodes {
		nodesByID[node.ID] = node
	}
	pathOf := func(node engine.JRenderNode) string {
		path := node.Name
		for parent, ok := nodesByID[node.Parent]; ok; parent, ok = nodesByID[parent.Parent] {
			path = parent.Name + "." + path
		}
		return path
	}
	partsByPath := map[string]engine.JRenderNode{}
	for _, node := range diagram.Nodes {
		if node.Kind == "part" {
			partsByPath[pathOf(node)] = node
		}
	}
	// The root is drawn under its qualified name; the parts below it under their feature names.
	const root = "MassRollup::saturnV"
	if node, ok := partsByPath[root]; !ok || node.Parent != "" || node.Type != "LaunchVehicle" {
		t.Fatalf("saturnV root = %+v, want a top-level part typed LaunchVehicle", node)
	}
	wantParts := map[string]string{
		root + ".stage1": "'S-IC'", root + ".stage2": "'S-II'", root + ".stage3": "'S-IVB'",
		root + ".stage1.engine1": "'F-1'", root + ".stage1.engine5": "'F-1'",
		root + ".stage2.engine1": "'J-2'", root + ".stage2.engine5": "'J-2'",
		root + ".stage3.engine1": "'J-2'",
		// The bound collections and the unvalued defaults are drawn too, a redefining one headed
		// by the type it inherits; the page folds them into the instances they refer to.
		root + ".subcomponents": "Component", root + ".stage1.engines": "Engine", root + ".stage1.engine1.subcomponents": "Component",
	}
	for path, typ := range wantParts {
		if node, ok := partsByPath[path]; !ok || node.Type != typ {
			t.Errorf("RenderView part %s = %+v, want type %s", path, node, typ)
		}
	}
	// The only wires are the bindings between the bound collections, which the page folds away.
	for _, edge := range diagram.Edges {
		from, to := nodesByID[edge.From], nodesByID[edge.To]
		if edge.Kind != "binding" || (from.Name != "subcomponents" && from.Name != "engines") {
			t.Errorf("mass diagram wires %s %s -> %s, want only bindings out of the bound collections", edge.Kind, pathOf(from), pathOf(to))
		}
	}

	// Instantiate materializes the vehicle, its three stages and eleven engines, each with
	// its mass and the totalMass the engine rolls up; the page reads every box's number here.
	type quantity struct {
		IntMagnitude string `json:"intMagnitude"`
		Unit         string `json:"unit"`
	}
	type value struct {
		Quantity   *quantity `json:"quantity"`
		InstanceID string    `json:"instanceId"`
	}
	type featureValue struct {
		Value  *value  `json:"value"`
		Values []value `json:"values"`
		Error  string  `json:"error"`
	}
	type instance struct {
		ID            string                  `json:"id"`
		TypeSymbolID  string                  `json:"typeSymbolId"`
		FeatureValues map[string]featureValue `json:"featureValues"`
	}
	instantiate := func(symbol string) ([]byte, []instance) {
		t.Helper()
		var built struct {
			Error     string     `json:"error"`
			Instances []instance `json:"instances"`
		}
		body := call("Instantiate", map[string]string{"modelHash": parsed.ModelHash, "symbolId": symbol}, &built)
		if built.Error != "" {
			t.Fatalf("Instantiate %s: %s", symbol, built.Error)
		}
		return body, built.Instances
	}
	kilograms := func(in instance, feature string) string {
		fv, ok := in.FeatureValues[feature]
		if !ok || fv.Value == nil || fv.Value.Quantity == nil {
			t.Errorf("%s (%s) has no %s quantity: %+v", in.ID, in.TypeSymbolID, feature, fv)
			return ""
		}
		if fv.Value.Quantity.Unit != "kg" {
			t.Errorf("%s.%s unit = %q, want kg", in.TypeSymbolID, feature, fv.Value.Quantity.Unit)
		}
		return fv.Value.Quantity.IntMagnitude
	}
	instantiateBody, instances := instantiate("MassRollup::saturnV")
	pinLandingFixture(t, "mass.json", "TestLandingMassRollupModel", parsed.ModelHash, renderBody, instantiateBody)
	if len(instances) != 15 {
		t.Fatalf("Instantiate saturnV made %d instances, want 15: the vehicle, three stages, eleven engines", len(instances))
	}
	byID := map[string]instance{}
	types := map[string]int{}
	for _, in := range instances {
		byID[in.ID] = in
		types[in.TypeSymbolID]++
	}
	wantTypes := map[string]int{
		"MassRollup::saturnV": 1, "MassRollup::S-IC": 1, "MassRollup::S-II": 1, "MassRollup::S-IVB": 1,
		"MassRollup::F-1": 5, "MassRollup::J-2": 6,
	}
	if !reflect.DeepEqual(types, wantTypes) {
		t.Errorf("instance types: got %v, want %v", types, wantTypes)
	}
	var vehicle instance
	for _, in := range instances {
		if in.TypeSymbolID == "MassRollup::saturnV" {
			vehicle = in
		}
	}
	if got := kilograms(vehicle, "totalMass"); got != "2941728" {
		t.Errorf("saturnV.totalMass on the instance = %s kg, want 2941728", got)
	}
	// The stage features hold the stage instances singly; `subcomponents` lists the same three.
	stageTotals := map[string]string{}
	for _, feature := range []string{"stage1", "stage2", "stage3"} {
		fv := vehicle.FeatureValues[feature]
		if fv.Value == nil || fv.Value.InstanceID == "" {
			t.Fatalf("saturnV.%s holds no instance: %+v", feature, fv)
		}
		stageTotals[feature] = kilograms(byID[fv.Value.InstanceID], "totalMass")
	}
	if want := map[string]string{"stage1": "2332000", "stage2": "488940", "stage3": "120788"}; !reflect.DeepEqual(stageTotals, want) {
		t.Errorf("stage totals: got %v, want %v", stageTotals, want)
	}
	var listed []string
	for _, v := range vehicle.FeatureValues["subcomponents"].Values {
		listed = append(listed, v.InstanceID)
	}
	sort.Strings(listed)
	var held []string
	for _, feature := range []string{"stage1", "stage2", "stage3"} {
		held = append(held, vehicle.FeatureValues[feature].Value.InstanceID)
	}
	sort.Strings(held)
	if !reflect.DeepEqual(listed, held) {
		t.Errorf("saturnV.subcomponents = %v, want the stage instances %v", listed, held)
	}
	for _, in := range instances {
		if in.TypeSymbolID == "MassRollup::F-1" || in.TypeSymbolID == "MassRollup::J-2" {
			if mass, total := kilograms(in, "mass"), kilograms(in, "totalMass"); mass != total {
				t.Errorf("engine %s rolls up %s kg from a mass of %s kg; an engine has no subcomponents", in.ID, total, mass)
			}
		}
	}

	// Evaluate answers the three expressions the scene reports: the vehicle, the first stage,
	// and the vehicle with an instrument unit whose mass the model never values.
	evaluate := func(expression string) json.RawMessage {
		t.Helper()
		var response struct {
			Result json.RawMessage `json:"result"`
			Error  string          `json:"error"`
		}
		call("Evaluate", map[string]any{"modelHash": parsed.ModelHash, "expression": expression}, &response)
		if response.Error != "" {
			t.Fatalf("Evaluate %s: %s", expression, response.Error)
		}
		return response.Result
	}
	for expression, want := range map[string]string{
		"MassRollup::saturnV.totalMass":         "2941728",
		"MassRollup::saturnV.stage1.totalMass":  "2332000",
		"MassRollup::saturnVComplete.totalMass": "2943728",
	} {
		var result struct {
			Quantity *quantity `json:"quantity"`
		}
		raw := evaluate(expression)
		if err := json.Unmarshal(raw, &result); err != nil || result.Quantity == nil {
			t.Fatalf("Evaluate %s = %s, want a quantity", expression, raw)
		}
		if result.Quantity.IntMagnitude != want || result.Quantity.Unit != "kg" {
			t.Errorf("Evaluate %s = %s %s, want %s kg", expression, result.Quantity.IntMagnitude, result.Quantity.Unit, want)
		}
	}
	var undetermined struct {
		Undetermined *struct {
			Reason string `json:"reason"`
		} `json:"undetermined"`
	}
	raw := evaluate("MassRollup::saturnVWithIU.totalMass")
	if err := json.Unmarshal(raw, &undetermined); err != nil || undetermined.Undetermined == nil {
		t.Fatalf("Evaluate saturnVWithIU.totalMass = %s, want undetermined", raw)
	}
	if got, want := undetermined.Undetermined.Reason, "mass has no value in the model"; got != want {
		t.Errorf("saturnVWithIU.totalMass undetermined reason = %q, want %q", got, want)
	}

	// With the instrument unit the instantiation still materializes every object, and the
	// two roll-ups that need its mass come back as errors naming the unvalued feature.
	_, withIU := instantiate("MassRollup::saturnVWithIU")
	if len(withIU) != 16 {
		t.Fatalf("Instantiate saturnVWithIU made %d instances, want 16", len(withIU))
	}
	errors := map[string]string{}
	for _, in := range withIU {
		if fv := in.FeatureValues["totalMass"]; fv.Error != "" {
			errors[in.TypeSymbolID] = fv.Error
		}
	}
	wantErrors := map[string]string{
		"MassRollup::saturnVWithIU":  "feature value saturnVWithIU.totalMass: feature value InstrumentUnit.totalMass: no value for feature mass",
		"MassRollup::InstrumentUnit": "feature value InstrumentUnit.totalMass: no value for feature mass",
	}
	if !reflect.DeepEqual(errors, wantErrors) {
		t.Errorf("unvalued roll-ups: got %v, want %v", errors, wantErrors)
	}
	for _, in := range withIU {
		if in.TypeSymbolID == "MassRollup::InstrumentUnit" {
			if fv := in.FeatureValues["mass"]; fv.Value == nil || fv.Value.Quantity != nil {
				t.Errorf("InstrumentUnit.mass = %+v, want unset", fv)
			}
		}
	}

	// The scene's last step values the mass on the usage, and the same roll-up completes; the
	// redefining usage declares no typing, so its box is headed with the inherited one.
	var completeDiagram engine.JRenderViewResponse
	call("RenderView", map[string]any{
		"modelHash": parsed.ModelHash,
		"view":      "#interconnection:MassRollup::saturnVComplete",
		"ports":     "minimal",
		"depth":     3,
		"show":      []string{"typings", "keywords"},
	}, &completeDiagram)
	var instrumentUnits []engine.JRenderNode
	for _, node := range completeDiagram.Nodes {
		if node.Name == "instrumentUnit" {
			instrumentUnits = append(instrumentUnits, node)
		}
	}
	if len(instrumentUnits) != 1 || instrumentUnits[0].Type != "InstrumentUnit" {
		t.Errorf("saturnVComplete draws instrumentUnit as %+v, want one part typed InstrumentUnit", instrumentUnits)
	}
	_, complete := instantiate("MassRollup::saturnVComplete")
	if len(complete) != 16 {
		t.Fatalf("Instantiate saturnVComplete made %d instances, want 16", len(complete))
	}
	for _, in := range complete {
		if fv := in.FeatureValues["totalMass"]; fv.Error != "" {
			t.Errorf("saturnVComplete leaves %s.totalMass unvalued: %s", in.TypeSymbolID, fv.Error)
		}
		switch in.TypeSymbolID {
		case "MassRollup::saturnVComplete":
			if got := kilograms(in, "totalMass"); got != "2943728" {
				t.Errorf("saturnVComplete.totalMass on the instance = %s kg, want 2943728", got)
			}
		case "MassRollup::InstrumentUnit":
			if mass, total := kilograms(in, "mass"), kilograms(in, "totalMass"); mass != "2000" || total != "2000" {
				t.Errorf("the valued instrument unit has mass %s kg and totalMass %s kg, want 2000 and 2000", mass, total)
			}
		}
	}
}
