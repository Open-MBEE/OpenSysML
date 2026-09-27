package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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

// A Canvas extent is a pair: an explicit zero is a size, an unbound pair is
// none, and one of the two is reported.
func TestCanvasExtentNeedsBothWidthAndHeight(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		part engine;
		view zero { expose engine; @Canvas { width = 0; height = 0; } }
		view unit { expose engine; @Canvas { unit = "mm"; } }
		view half { expose engine; @Canvas { unit = "px"; width = 640; } }
	`)
	if site, ok := m.CanvasOf(sym(t, p, "zero")); !ok || site.Canvas == nil || !site.Canvas.HasSize || site.Canvas.Width != 0 || site.Canvas.Height != 0 || len(site.Problems) != 0 {
		t.Fatalf("CanvasOf(zero) = %+v, %v, want a 0×0 extent", site, ok)
	}
	if site, ok := m.CanvasOf(sym(t, p, "unit")); !ok || site.Canvas == nil || site.Canvas.HasSize || site.Canvas.Unit != "mm" || len(site.Problems) != 0 {
		t.Fatalf("CanvasOf(unit) = %+v, %v, want a unit and no extent", site, ok)
	}
	site, ok := m.CanvasOf(sym(t, p, "half"))
	if !ok || site.Canvas == nil || site.Canvas.HasSize || site.Canvas.Unit != "px" {
		t.Fatalf("CanvasOf(half) = %+v, %v, want the unit and no extent", site, ok)
	}
	if len(site.Problems) != 1 || site.Problems[0].Message != "Canvas binds one of width and height; an extent needs both" {
		t.Fatalf("problems of half = %+v", site.Problems)
	}
}

// A Canvas about a view stated outside its body is listed as a site of the view,
// so the layout pass reports it, but sizes nothing.
func TestCanvasOfIgnoresACanvasStatedOutsideTheViewBody(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		part engine;
		view a { expose engine; }
		metadata Canvas about a { width = 400; }
		view b { expose engine; metadata Canvas about a { width = 800; } }
		view c { expose engine; metadata Canvas about c { width = 100; } }
	`)
	a := sym(t, p, "a")
	if got := len(m.LayoutSitesOf(a)); got != 2 {
		t.Fatalf("LayoutSitesOf(a) has %d sites, want 2", got)
	}
	for _, site := range m.LayoutSitesOf(a) {
		if site.StatedInBodyOf(a) {
			t.Fatalf("site %+v is stated in the body of a", site)
		}
	}
	if site, ok := m.CanvasOf(a); ok {
		t.Fatalf("CanvasOf(a) = %+v, want none", site)
	}
	c := sym(t, p, "c")
	if site, ok := m.CanvasOf(c); !ok || site.Canvas == nil || site.Canvas.Width != 100 || !site.StatedInBodyOf(c) {
		t.Fatalf("CanvasOf(c) = %+v, %v", site, ok)
	}
}

func TestStyleOfPrefersTheViewBodyAndNormalisesColours(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		part engine { @Style { fill = "#ffe8bd"; font = "Arial"; fontSize = 11; bold = true; } }
		view a { expose engine; metadata Style about engine { line = "#336699"; italic = true; } }
		view b { expose engine; }
	`)
	engine := sym(t, p, "engine")
	site, ok := m.StyleOf(sym(t, p, "a"), engine)
	if !ok || site.Style == nil || !site.About {
		t.Fatalf("StyleOf(a, engine) = %+v, %v", site, ok)
	}
	if want := (Style{Line: "#336699", Italic: true}); *site.Style != want {
		t.Fatalf("view-local style = %+v, want %+v", *site.Style, want)
	}
	site, ok = m.StyleOf(sym(t, p, "b"), engine)
	if !ok || site.Style == nil {
		t.Fatalf("StyleOf(b, engine) = %+v, %v", site, ok)
	}
	if want := (Style{Fill: "#FFE8BD", Font: "Arial", FontSize: 11, Bold: true}); *site.Style != want {
		t.Fatalf("inline style = %+v, want %+v", *site.Style, want)
	}
}

func TestStyleOfReportsAMalformedColour(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		part engine { @Style { fill = "orange"; fontSize = -2; } }
	`)
	site, ok := m.StyleOf(nil, sym(t, p, "engine"))
	if !ok || site.Style != nil {
		t.Fatalf("StyleOf(nil, engine) = %+v, %v", site, ok)
	}
	if len(site.Problems) != 2 ||
		site.Problems[0].Message != `fill of Style is "orange", not a colour written #RRGGBB` ||
		site.Problems[1].Message != "fontSize of Style is negative" {
		t.Fatalf("problems = %+v", site.Problems)
	}
}

func TestStyleOfRejectsAMalformedBoolean(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		part engine { @Style { fill = "#FF0000"; bold = 7; } }
		part pump { @Style { fill = "#FF0000"; italic = "yes"; } }
	`)
	for _, name := range []string{"engine", "pump"} {
		site, ok := m.StyleOf(nil, sym(t, p, name))
		if !ok || site.Style != nil {
			t.Errorf("StyleOf(nil, %s) = %+v, %v; want the style withheld", name, site, ok)
		}
		if len(site.Problems) != 1 || !strings.HasSuffix(site.Problems[0].Message, "of Style is not a constant boolean") {
			t.Errorf("%s problems = %+v", name, site.Problems)
		}
	}
}

func TestNotesOfListsEveryNoteInTheView(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		part engine { @Note { text = "always"; x = 1; y = 2; } }
		view a {
			expose engine;
			metadata Note about engine { text = "anchored"; x = 10; y = 20; width = 100; height = 40; }
			metadata Note about engine { text = "second"; x = 30; y = 40; }
			@Note { text = "free"; x = 0; y = 0; }
		}
		view b { expose engine; }
	`)
	engine, a := sym(t, p, "engine"), sym(t, p, "a")
	notes := m.NotesOf(a, engine)
	if len(notes) != 3 {
		t.Fatalf("NotesOf(a, engine) has %d notes, want 3", len(notes))
	}
	if n := notes[0].Note; n == nil || n.Text != "always" || notes[0].About {
		t.Fatalf("inline note = %+v", n)
	}
	if n := notes[1].Note; n == nil || n.Text != "anchored" || !n.HasSize || n.Width != 100 || n.Height != 40 {
		t.Fatalf("first about note = %+v", n)
	}
	if n := notes[2].Note; n == nil || n.Text != "second" || n.HasSize {
		t.Fatalf("second about note = %+v", n)
	}
	if got := m.NotesOf(sym(t, p, "b"), engine); len(got) != 1 || got[0].Note.Text != "always" {
		t.Fatalf("NotesOf(b, engine) = %+v", got)
	}
	if free := m.NotesOf(a, a); len(free) != 1 || free[0].Note.Text != "free" {
		t.Fatalf("NotesOf(a, a) = %+v", free)
	}
}

// A note stated in a nested view's body belongs to that view's drawing alone:
// it is listed for the view stating it, not for the view enclosing it.
func TestNotesOfScopesANoteStatedInANestedViewToIt(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		part engine;
		view outer {
			expose engine;
			view inner {
				expose engine;
				@Note { text = "inner"; x = 1; y = 2; }
			}
		}
	`)
	outer := sym(t, p, "outer")
	inner := sym(t, outer.Scope, "inner")
	notes := m.NotesOf(inner, inner)
	if len(notes) != 1 || notes[0].Note == nil || notes[0].Note.Text != "inner" {
		t.Fatalf("NotesOf(inner, inner) = %+v", notes)
	}
	if got := m.NotesOf(outer, inner); len(got) != 0 {
		t.Fatalf("NotesOf(outer, inner) = %+v, want none", got)
	}
}

func TestNotesOfReportsAnIncompleteNote(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		part engine { @Note { x = 1; width = 10; } }
	`)
	notes := m.NotesOf(nil, sym(t, p, "engine"))
	if len(notes) != 1 || notes[0].Note != nil {
		t.Fatalf("NotesOf(nil, engine) = %+v", notes)
	}
	want := []string{
		"Note binds no text to show",
		"Note binds no x and y to place the note at",
		"Note binds one of width and height; a size needs both",
	}
	if len(notes[0].Problems) != len(want) {
		t.Fatalf("problems = %+v", notes[0].Problems)
	}
	for i, p := range notes[0].Problems {
		if p.Message != want[i] {
			t.Errorf("problem %d = %q, want %q", i, p.Message, want[i])
		}
	}
}

func TestPicturesOfListsEveryPictureOfTheView(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		part engine;
		view a {
			expose engine;
			@Picture { location = "images/bench.png"; x = 0; y = 0; width = 823; height = 577; alt = "the bench"; }
			@Picture { location = "images/logo.png"; x = 700; y = 20; width = 80; height = 40; above = true; }
		}
		view b { expose engine; }
	`)
	a := sym(t, p, "a")
	pics := m.PicturesOf(a)
	if len(pics) != 2 {
		t.Fatalf("PicturesOf(a) has %d pictures, want 2", len(pics))
	}
	if pic := pics[0].Picture; pic == nil || pic.Location != "images/bench.png" || pic.X != 0 || pic.Y != 0 ||
		pic.Width != 823 || pic.Height != 577 || pic.Alt != "the bench" || pic.Above {
		t.Fatalf("first picture = %+v", pic)
	}
	if pic := pics[1].Picture; pic == nil || pic.Location != "images/logo.png" || pic.X != 700 || pic.Y != 20 ||
		pic.Width != 80 || pic.Height != 40 || pic.Alt != "" || !pic.Above {
		t.Fatalf("second picture = %+v", pic)
	}
	if got := m.PicturesOf(sym(t, p, "b")); len(got) != 0 {
		t.Fatalf("PicturesOf(b) = %+v", got)
	}
	if got := m.PicturesOf(nil); got != nil {
		t.Fatalf("PicturesOf(nil) = %+v", got)
	}
}

func TestPicturesOfReportsAnIncompletePicture(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		view a {
			@Picture { x = 1; width = 10; }
			@Picture { location = ""; x = 0; y = 0; width = 0; height = 10; }
			@Picture { location = "https://example.org/a.png"; x = 0; y = 0; width = 10; height = 10; }
			@Picture { location = "data:image/png;base64,iVBORw0KGgo="; x = 0; y = 0; width = 10; height = 10; }
		}
	`)
	pics := m.PicturesOf(sym(t, p, "a"))
	if len(pics) != 4 || pics[0].Picture != nil || pics[1].Picture != nil || pics[2].Picture != nil || pics[3].Picture != nil {
		t.Fatalf("PicturesOf(a) = %+v", pics)
	}
	for i, want := range [][]string{{
		"Picture binds no location to read the picture from",
		"Picture binds no x and y to place the picture at",
		"Picture binds no width and height to size the picture to",
	}, {
		"location of Picture is empty",
		"width and height of Picture must be positive",
	}, {
		"location of Picture is a URL, not the path of a file the drawing tools can read",
	}, {
		"location of Picture is a URL, not the path of a file the drawing tools can read",
	}} {
		if len(pics[i].Problems) != len(want) {
			t.Fatalf("picture %d problems = %+v", i, pics[i].Problems)
		}
		for j, p := range pics[i].Problems {
			if p.Message != want[j] {
				t.Errorf("picture %d problem %d = %q, want %q", i, j, p.Message, want[j])
			}
		}
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

// A declaration inherited from a definition in another document is traced to
// its symbol from the scope of the usage's document.
func TestSymbolDeclaringSearchesEveryDocument(t *testing.T) {
	m, p := layoutModel(t, `
		private import DiagramLayout::*;
		state def S {
			entry; then off;
			state off { @Layout { x = 1; y = 2; } }
		}
	`)
	other := parser.New(source.New("other.sysml", []byte("package Q { state s : P::S; }\n")))
	otherRoot := other.ParseFile()
	if len(other.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", other.Diagnostics)
	}
	idx := m.resolver.Index()
	idx.AddDocument("other.sysml", otherRoot)
	off := sym(t, sym(t, p, "S").Scope, "off")
	got, ok := m.SymbolDeclaring(idx.DocumentRoot("other.sysml"), off.Decl)
	if !ok || got != off {
		t.Fatalf("SymbolDeclaring from other.sysml = %v, %v, want %v", got, ok, off)
	}
	if site, ok := m.LayoutOf(nil, got); !ok || site.Layout == nil || site.Layout.X != 1 {
		t.Fatalf("LayoutOf(nil, off) = %+v, %v", site, ok)
	}
}
