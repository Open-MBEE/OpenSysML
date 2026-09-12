package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// layoutModel resolves src against the standard library, where DiagramLayout
// is declared, and returns the model and the root of package P.
func layoutModel(t *testing.T, src string) (*Model, *symbols.Scope) {
	t.Helper()
	m, root := stdlibModelWithDoc(t, "layout.sysml", "package P {\n"+src+"\n}\n")
	return m, sym(t, root, "P").Scope
}

func TestLayoutOfPrefersTheViewBodyOverTheInlineAnnotation(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		part def Engine;
		part engine : Engine { @Layout { x = 10; y = 20; width = 100; height = 50; } }
		view a { expose engine; metadata Layout about engine { x = 300; y = 40; collapsed = true; } }
		view b { expose engine; }
	`)
	engine := sym(t, p, "engine")

	site, ok := m.LayoutOf(sym(t, p, "a"), engine)
	if !ok || site.Layout == nil {
		t.Fatalf("LayoutOf(a, engine) = %+v, %v", site, ok)
	}
	if site.Layout.X != 300 || site.Layout.Y != 40 || site.Layout.HasSize || !site.Layout.Collapsed || !site.About {
		t.Fatalf("view-local layout = %+v", site.Layout)
	}

	site, ok = m.LayoutOf(sym(t, p, "b"), engine)
	if !ok || site.Layout == nil || site.About {
		t.Fatalf("LayoutOf(b, engine) = %+v, %v", site, ok)
	}
	if site.Layout.X != 10 || site.Layout.Y != 20 || !site.Layout.HasSize || site.Layout.Width != 100 || site.Layout.Height != 50 {
		t.Fatalf("inline layout = %+v", site.Layout)
	}

	site, ok = m.LayoutOf(nil, engine)
	if !ok || site.Layout == nil || site.Layout.X != 10 {
		t.Fatalf("LayoutOf(nil, engine) = %+v, %v", site, ok)
	}
}

func TestLayoutOfPlacesAnElementInOneViewOnly(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		part engine;
		view a { expose engine; metadata Layout about engine { x = 1; y = 2; } }
		view b { expose engine; }
	`)
	engine := sym(t, p, "engine")
	if site, ok := m.LayoutOf(sym(t, p, "a"), engine); !ok || site.Layout == nil || site.Layout.X != 1 {
		t.Fatalf("LayoutOf(a, engine) = %+v, %v", site, ok)
	}
	if site, ok := m.LayoutOf(sym(t, p, "b"), engine); ok {
		t.Fatalf("LayoutOf(b, engine) = %+v, want none", site)
	}
	if site, ok := m.LayoutOf(nil, engine); ok {
		t.Fatalf("LayoutOf(nil, engine) = %+v, want none", site)
	}
}

func TestLayoutOfReadsAnAboutAnnotationNestedInTheViewBody(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		part engine;
		view a { expose engine; package Positions { metadata Layout about engine { x = 5; y = 6; } } }
	`)
	site, ok := m.LayoutOf(sym(t, p, "a"), sym(t, p, "engine"))
	if !ok || site.Layout == nil || site.Layout.X != 5 || site.Layout.Y != 6 {
		t.Fatalf("LayoutOf(a, engine) = %+v, %v", site, ok)
	}
}

func TestLayoutOfFirstViewLocalAnnotationWins(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		part engine;
		view a {
			expose engine;
			metadata Layout about engine { x = 1; y = 1; }
			metadata Layout about engine { x = 2; y = 2; }
		}
	`)
	site, ok := m.LayoutOf(sym(t, p, "a"), sym(t, p, "engine"))
	if !ok || site.Layout == nil || site.Layout.X != 1 {
		t.Fatalf("LayoutOf(a, engine) = %+v, %v", site, ok)
	}
	sites := m.LayoutSitesOf(sym(t, p, "engine"))
	if len(sites) != 2 {
		t.Fatalf("LayoutSitesOf(engine) has %d sites, want 2", len(sites))
	}
}

func TestRouteOfReadsWaypointPairs(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		part a; part b;
		connection c connect a to b;
		view v { expose a; expose b; expose c; metadata Route about c { points = (10, 20, 30.5, 40); } }
	`)
	site, ok := m.RouteOf(sym(t, p, "v"), sym(t, p, "c"))
	if !ok || site.Route == nil {
		t.Fatalf("RouteOf(v, c) = %+v, %v", site, ok)
	}
	want := []Waypoint{{10, 20}, {30.5, 40}}
	if len(site.Route.Points) != 2 || site.Route.Points[0] != want[0] || site.Route.Points[1] != want[1] {
		t.Fatalf("route points = %v, want %v", site.Route.Points, want)
	}
}

func TestRouteOfReportsAnOddPointCount(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		part a; part b;
		connection c connect a to b { @Route { points = (1, 2, 3); } }
	`)
	site, ok := m.RouteOf(nil, sym(t, p, "c"))
	if !ok || site.Route != nil || site.PointCount != 3 {
		t.Fatalf("RouteOf(nil, c) = %+v, %v", site, ok)
	}
	if len(site.Problems) != 1 || !strings.Contains(site.Problems[0].Message, "3 values") {
		t.Fatalf("problems = %+v", site.Problems)
	}
}

func TestLayoutOfReportsANonConstantBinding(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		attribute pos : ScalarValues::Real;
		part engine { @Layout { x = pos; y = 2; } }
	`)
	site, ok := m.LayoutOf(nil, sym(t, p, "engine"))
	if !ok || site.Layout != nil {
		t.Fatalf("LayoutOf(nil, engine) = %+v, %v", site, ok)
	}
	if len(site.Problems) != 1 || site.Problems[0].Message != "x of Layout is not a constant number" {
		t.Fatalf("problems = %+v", site.Problems)
	}
}

func TestCanvasOfReadsTheViewsCanvas(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		part engine;
		view a { expose engine; @Canvas { unit = "px"; width = 1200; height = 800; } }
		view b { expose engine; }
	`)
	site, ok := m.CanvasOf(sym(t, p, "a"))
	if !ok || site.Canvas == nil {
		t.Fatalf("CanvasOf(a) = %+v, %v", site, ok)
	}
	if site.Canvas.Unit != "px" || site.Canvas.Width != 1200 || site.Canvas.Height != 800 {
		t.Fatalf("canvas = %+v", site.Canvas)
	}
	if site, ok := m.CanvasOf(sym(t, p, "b")); ok {
		t.Fatalf("CanvasOf(b) = %+v, want none", site)
	}
}

func TestSymbolDeclaringFindsATransitionByItsDeclaration(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		state def S {
			entry; then off;
			state off;
			state on;
			transition off_on first off then on { @Route { points = (0, 0, 10, 10); } }
		}
	`)
	s := sym(t, p, "S")
	trans := sym(t, s.Scope, "off_on")
	got, ok := m.SymbolDeclaring(s.Scope, trans.Decl)
	if !ok || got != trans {
		t.Fatalf("SymbolDeclaring = %v, %v, want %v", got, ok, trans)
	}
	if site, ok := m.RouteOf(nil, got); !ok || site.Route == nil || len(site.Route.Points) != 2 {
		t.Fatalf("RouteOf(nil, off_on) = %+v, %v", site, ok)
	}
}
