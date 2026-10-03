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

// TestLandingStackModel pins what the landing page's diagram reads from
// docs/assets/opensysml-stack.sysml through sysml-engine: a clean parse, one
// box per part of `stack` carrying the attributes the page draws, one wire per
// interface, and the state sequence the page animates along those wires.
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
