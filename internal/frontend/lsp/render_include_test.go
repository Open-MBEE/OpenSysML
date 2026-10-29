package lsp

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

const includeModel = `package Rel {
    private import Views::*;
    part def Vehicle;
    part def Car :> Vehicle;
    part def Truck :> Vehicle;
    part def Wheel;
    part car : Car { part wheel : Wheel; }
    view structure { expose Car; render asTreeDiagram; }
}
`

func renderWithInclude(t *testing.T, s *Server, docURI uri.URI, include ...string) RenderResult {
	t.Helper()
	raw, err := call(t, s, MethodRender, &RenderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "Rel::structure",
		Form:         string(view.FormText),
		Include:      include,
	})
	if err != nil {
		t.Fatalf("render include %v: %v", include, err)
	}
	var out RenderResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func findNode(out RenderResult, name string) *RenderNode {
	for i := range out.Nodes {
		if out.Nodes[i].FQN == name {
			return &out.Nodes[i]
		}
	}
	return nil
}

// An included element is drawn as if the view exposed it, marked `included`,
// with the edges between it and the view's own nodes; the view's own content
// is never marked, and the document is not touched.
func TestRenderIncludeDrawsElementsBesidesTheView(t *testing.T) {
	s, docURI := renderServer(t, "rel.sysml", includeModel)
	before := renderWithInclude(t, s, docURI)
	if findNode(before, "Rel::Vehicle") != nil || len(before.Nodes) != 1 {
		t.Fatalf("view alone draws %d nodes, want just Car: %+v", len(before.Nodes), before.Nodes)
	}

	out := renderWithInclude(t, s, docURI, "Rel::Vehicle", "Rel::Truck")
	for _, name := range []string{"Rel::Vehicle", "Rel::Truck"} {
		node := findNode(out, name)
		if node == nil {
			t.Fatalf("include %s: not drawn; nodes %+v", name, out.Nodes)
		}
		if !node.Included {
			t.Errorf("include %s: included = false, want true", name)
		}
	}
	car := findNode(out, "Rel::Car")
	if car == nil || car.Included {
		t.Fatalf("Car = %+v, want drawn by the view and not marked included", car)
	}
	vehicle := findNode(out, "Rel::Vehicle")
	edges := 0
	for _, edge := range out.Edges {
		if edge.To == vehicle.ID && (edge.From == car.ID || edge.From == findNode(out, "Rel::Truck").ID) && edge.Kind == view.EdgeSpecialization.String() {
			edges++
		}
	}
	if edges != 2 {
		t.Errorf("specialization edges to Vehicle = %d, want Car and Truck: %+v", edges, out.Edges)
	}
	if out.Version != before.Version {
		t.Errorf("version moved from %d to %d; include must not change the document", before.Version, out.Version)
	}
	if len(out.Notices) != 0 {
		t.Errorf("notices = %v, want none", out.Notices)
	}
}

// Including what the view exposes draws it once, unmarked; an unknown name is
// reported as a notice and the rest of the rendering stands.
func TestRenderIncludeKeepsTheViewsContentAndReportsUnknownNames(t *testing.T) {
	s, docURI := renderServer(t, "rel.sysml", includeModel)
	out := renderWithInclude(t, s, docURI, "Rel::Car", "Rel::Nothing", "Rel::Wheel")
	cars := 0
	for _, node := range out.Nodes {
		if node.FQN == "Rel::Car" {
			cars++
			if node.Included {
				t.Errorf("Car is exposed by the view; included = true")
			}
		}
	}
	if cars != 1 {
		t.Errorf("Car drawn %d times, want once", cars)
	}
	if wheel := findNode(out, "Rel::Wheel"); wheel == nil || !wheel.Included {
		t.Errorf("Wheel = %+v, want drawn and included", wheel)
	}
	want := "included element Rel::Nothing is not drawn: no such element"
	if !slices.Contains(out.Notices, want) {
		t.Errorf("notices = %v, want %q", out.Notices, want)
	}
}

// A nested element the view already draws under an exposed one is not marked
// included when the request names it: it is the view's own content.
func TestRenderIncludeDoesNotMarkWhatTheViewDrawsNested(t *testing.T) {
	s, docURI := renderServer(t, "rel.sysml", strings.Replace(includeModel, "expose Car;", "expose car;", 1))
	out := renderWithInclude(t, s, docURI, "Rel::car::wheel")
	wheels := 0
	for _, node := range out.Nodes {
		if node.FQN == "Rel::car::wheel" {
			wheels++
			if node.Included {
				t.Errorf("wheel is drawn under car by the view; included = true")
			}
		}
	}
	if wheels != 1 {
		t.Errorf("wheel drawn %d times, want once", wheels)
	}
}

func TestInitializeAdvertisesTheRenderIncludeCapability(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	res, err := s.Initialize(context.Background(), &protocol.InitializeParams{})
	if err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
	experimental := res.Capabilities.Experimental.(map[string]any)
	if experimental[RenderIncludeCapability] != true {
		t.Errorf("%s = %#v, want true", RenderIncludeCapability, experimental[RenderIncludeCapability])
	}
}
