package layout

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The positions-only graph: a leaf a fixed box in inches, a container a
// cluster whose label reserves its header and whose anchor point an edge
// may end at, no routing, the gaps as nodesep and ranksep.
func TestGraphDOT(t *testing.T) {
	g := Graph{Direction: "LR", Gap: 72, Pad: 16,
		Nodes: []Node{
			{ID: "site", Width: 100, Height: 42, Children: []Node{{ID: "s", Width: 54, Height: 37}, {ID: "a", Width: 54, Height: 37}}},
			{ID: "lone", Width: 20, Height: 20},
		},
		Edges: []Edge{{From: "s", To: "a"}, {From: "site", To: "lone"}, {From: "s", To: "nowhere"}},
	}
	want := `digraph layout {
  graph [splines=none, nodesep=1, ranksep=1, rankdir=LR];
  node [shape=box, fixedsize=true, label=""];
  subgraph "cluster_site" {
    label=<<table border="0" cellborder="0" cellspacing="0" cellpadding="0"><tr><td fixedsize="true" width="100" height="42"></td></tr></table>>;
    margin=16;
    "site" [shape=point, style=invis, width=0, height=0];
    "s" [width=0.75, height=0.5138888888888888];
    "a" [width=0.75, height=0.5138888888888888];
  }
  "lone" [width=0.2777777777777778, height=0.2777777777777778];
  "s" -> "a";
  "site" -> "lone";
}
`
	if got := g.dot(); got != want {
		t.Errorf("dot():\n%s\nwant:\n%s", got, want)
	}
}

// dot's answer is read back as whole pixels from the top-left corner: a
// node's centre and inches become a box, a cluster's bb its container's box,
// and the anchor point inside a cluster is not a box of its own.
func TestParse(t *testing.T) {
	answer := `{"name": "layout", "bb": "0,0,210,244.55", "objects": [
	  {"name": "cluster_site", "bb": "8,8,202,236.55"},
	  {"name": "site", "pos": "24,132", "width": "0.01", "height": "0.01"},
	  {"name": "s", "pos": "114,132", "width": "2", "height": "0.8"},
	  {"name": "a", "pos": "114,45.6", "width": "1", "height": "0.59999"}
	]}`
	got, err := parse([]byte(answer), map[string]bool{"site": true})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Box{
		"site": {X: 8, Y: 8, Width: 194, Height: 229},
		"s":    {X: 42, Y: 84, Width: 144, Height: 58},
		"a":    {X: 78, Y: 178, Width: 72, Height: 43},
	}
	if len(got) != len(want) {
		t.Fatalf("parse() = %v, want %v", got, want)
	}
	for id, box := range want {
		if got[id] != box {
			t.Errorf("parse()[%s] = %v, want %v", id, got[id], box)
		}
	}
	for _, bad := range []string{`not json`, `{"bb": "0,0,1"}`, `{"bb": "0,0,1,1", "objects": [{"name": "x", "pos": "1"}]}`, `{"bb": "0,0,1,1", "objects": [{"name": "cluster_x", "bb": "a,b,c,d"}]}`} {
		if _, err := parse([]byte(bad), nil); err == nil {
			t.Errorf("parse(%s) found no fault", bad)
		}
	}
}

// Locate finds the dot OPENSYSML_DOT names ahead of PATH, and names what it
// looked for when there is none to find.
func TestLocate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake dot is a shell script")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "dot")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho 'dot - graphviz version 16.1.0 (20260904.0139)' >&2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvVar, fake)
	path, err := Locate()
	if err != nil || path != fake {
		t.Fatalf("Locate() = %q, %v; want the override %s", path, err, fake)
	}
	version, err := Version(path)
	if err != nil || version != "16.1.0" {
		t.Errorf("Version() = %q, %v; want 16.1.0", version, err)
	}

	t.Setenv(EnvVar, filepath.Join(dir, "missing"))
	_, err = Locate()
	var notFound *NotFoundError
	if !errors.As(err, &notFound) || !strings.Contains(err.Error(), "no Graphviz dot at OPENSYSML_DOT="+filepath.Join(dir, "missing")) {
		t.Errorf("Locate() with a missing override: %v", err)
	}

	t.Setenv(EnvVar, "")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/nonexistent")
	if path, err := Locate(); err != nil || path != fake {
		t.Errorf("Locate() from PATH = %q, %v; want %s", path, err, fake)
	}
	t.Setenv("PATH", "/nonexistent")
	_, err = Locate()
	if !errors.As(err, &notFound) || err.Error() != "no Graphviz dot on PATH; install Graphviz or set OPENSYSML_DOT" {
		t.Errorf("Locate() with no dot: %v", err)
	}

	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho 'something else'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Version(fake); err == nil || !strings.Contains(err.Error(), "not a graphviz version") {
		t.Errorf("Version() of a stranger: %v", err)
	}
}

// A dot that fails is reported with what it said.
func TestPlaceFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake dot is a shell script")
	}
	fake := filepath.Join(t.TempDir(), "dot")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho 'Error: broken' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Place(fake, Graph{Nodes: []Node{{ID: "a", Width: 10, Height: 10}}})
	var failed *Error
	if !errors.As(err, &failed) || !strings.Contains(err.Error(), "Error: broken") {
		t.Errorf("Place() with a failing dot: %v", err)
	}
}
