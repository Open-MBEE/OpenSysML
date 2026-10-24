package wasm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
)

// An interconnection rendering of a bodiless usage shows the parts it has from
// its definition, each expanded through its own type: saturnV holds the three
// stages LaunchVehicle declares, and each stage the engines its type declares.
func TestEngineRenderViewExpandsUsagesThroughTheirTypes(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "examples", "runtime-showcase", "mass-rollup.sysml"))
	if err != nil {
		t.Fatalf("reading mass-rollup.sysml: %v", err)
	}
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	parsedBody := mustCall(t, eng, "ParseSources", renderParamsJSON(t, map[string]any{
		"documents": []map[string]string{{"name": "mass-rollup.sysml", "content": string(src)}},
	}))
	var parsed engine.JParseSourcesResponse
	if err := json.Unmarshal(parsedBody, &parsed); err != nil {
		t.Fatalf("decoding ParseSources: %v\n%s", err, parsedBody)
	}
	body := mustCall(t, eng, "RenderView", renderParamsJSON(t, map[string]any{
		"modelHash": parsed.ModelHash, "view": "#interconnection:MassRollup::saturnV", "ports": "minimal", "depth": 3,
	}))
	var response engine.JRenderViewResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decoding RenderView: %v\n%s", err, body)
	}
	byID := map[string]engine.JRenderNode{}
	children := map[string][]string{}
	for _, node := range response.Nodes {
		byID[node.ID] = node
		children[node.Parent] = append(children[node.Parent], node.Name)
	}
	roots := children[""]
	if len(roots) != 1 || roots[0] != "MassRollup::saturnV" {
		t.Fatalf("roots = %v, want saturnV alone", roots)
	}
	root := response.Nodes[0]
	want := []string{"mass", "stage1", "stage2", "stage3", "subcomponents", "totalMass"}
	if got := slices.Sorted(slices.Values(children[root.ID])); !slices.Equal(got, want) {
		t.Errorf("saturnV holds %v, want the features LaunchVehicle declares and inherits %v", got, want)
	}
	engines := map[string][]string{
		"stage1": {"engine1", "engine2", "engine3", "engine4", "engine5"},
		"stage2": {"engine1", "engine2", "engine3", "engine4", "engine5"},
		"stage3": {"engine1"},
	}
	for _, node := range response.Nodes {
		wantEngines, ok := engines[node.Name]
		if !ok || node.Parent != root.ID {
			continue
		}
		var got []string
		for _, child := range children[node.ID] {
			if len(child) > 6 && child[:6] == "engine" && child != "engines" {
				got = append(got, child)
			}
		}
		slices.Sort(got)
		if !slices.Equal(got, wantEngines) {
			t.Errorf("%s holds engines %v, want %v", node.Name, got, wantEngines)
		}
		for _, name := range []string{"mass", "propellantMass", "engines", "subcomponents", "totalMass"} {
			if !slices.Contains(children[node.ID], name) {
				t.Errorf("%s lacks %s from its type; holds %v", node.Name, name, children[node.ID])
			}
		}
	}
	if len(response.Nodes) < 40 {
		t.Errorf("nodes = %d, want the stages and engines expanded", len(response.Nodes))
	}
}
