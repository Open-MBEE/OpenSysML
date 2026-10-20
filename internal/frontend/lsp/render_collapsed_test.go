package lsp

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var updateCollapsedPorts = flag.Bool("update-collapsed-ports", false, "rewrite the collapsed-ports rendering fixture the diagram panel's tests read")

// TestRenderCollapsedKeepsBoundaryPorts pins what the diagram panel receives for
// a collapsed part: its ports stay on its boundary and every connector still ends
// at them, while the part it nests, and that part's port, are not in the rendering.
// The result is the fixture the webview's tests lay out and draw, so the panel is
// measured against the same rendering the CLI writes.
func TestRenderCollapsedKeepsBoundaryPorts(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "editors", "vscode", "src", "webview", "testdata")
	src, err := os.ReadFile(filepath.Join(dir, "collapsed-ports.sysml"))
	if err != nil {
		t.Fatalf("reading the fixture model: %v", err)
	}
	// The document is opened under a fixed path, so the origins the fixture pins
	// do not depend on where the tree is checked out.
	s, docURI := renderServer(t, "/collapsed-ports.sysml", string(src))
	fixture := map[string]*RenderResult{}
	for _, view := range []string{"tree", "wiring"} {
		fixture[view] = render(t, s, docURI, "FoldedPorts::"+view)
	}

	for view, result := range fixture {
		names := map[string]string{}
		kinds := map[string]string{}
		parents := map[string]string{}
		var engine *RenderNode
		for i := range result.Nodes {
			n := &result.Nodes[i]
			names[n.ID] = n.Name
			kinds[n.ID] = n.Kind
			parents[n.ID] = n.Parent
			if n.Name == "engine" {
				engine = n
			}
		}
		if engine == nil || !engine.Collapsed {
			t.Fatalf("%s: engine = %+v, want a collapsed node", view, engine)
		}
		var kept []string
		for _, n := range result.Nodes {
			if n.Parent == engine.ID {
				kept = append(kept, n.Kind+" "+n.Name)
			}
		}
		if want := []string{"port fuelIn", "port shaftOut"}; !reflect.DeepEqual(kept, want) {
			t.Errorf("%s: under collapsed engine: got %v, want %v", view, kept, want)
		}
		for _, name := range []string{"injector", "feed", "rate", "mass"} {
			for id, got := range names {
				if got == name {
					t.Errorf("%s: %s %s is drawn under %s, though engine is collapsed", view, kinds[id], name, names[parents[id]])
				}
			}
		}
		if view != "wiring" {
			continue
		}
		var ends []string
		for _, e := range result.Edges {
			ends = append(ends, names[e.From]+" -- "+names[e.To])
		}
		if want := []string{"fuelOut -- fuelIn", "shaftOut -- hub", "engine -- fuelOut"}; !reflect.DeepEqual(ends, want) {
			t.Errorf("%s: edges: got %v, want %v", view, ends, want)
		}
	}

	path := filepath.Join(dir, "collapsed-ports.json")
	data, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		t.Fatalf("encoding the fixture: %v", err)
	}
	data = append(data, '\n')
	if *updateCollapsedPorts {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
		t.Logf("updated %s", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v; run go test -count=1 ./internal/frontend/lsp -run '^TestRenderCollapsedKeepsBoundaryPorts$' -update-collapsed-ports", path, err)
	}
	var got, saved any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decoding the current rendering: %v", err)
	}
	if err := json.Unmarshal(want, &saved); err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
	if !reflect.DeepEqual(got, saved) {
		t.Fatalf("%s is stale; run go test -count=1 ./internal/frontend/lsp -run '^TestRenderCollapsedKeepsBoundaryPorts$' -update-collapsed-ports", path)
	}
}
