package model

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

const geometryViewModel = `package GeomViews {
	private import StandardViewDefinitions::*;
	part def Widget;

	view geometryView : GeometryView {
		expose Widget;
	}
}
`

// The views of every document are listed together in qualified-name order, each
// naming the document declaring it and the line it starts on.
func TestAllViewsListsEveryDocumentsViewsInOrder(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("kit.sysml", []byte(twoViewModel), 1)
	ws.Open("geom.sysml", []byte(geometryViewModel), 1)

	views := ws.AllViews()
	var names []string
	for _, info := range views {
		names = append(names, info.Name)
	}
	want := []string{"GeomViews::geometryView", "KitViews::widgetTable", "KitViews::widgetTree"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("listed %v, want %v", names, want)
	}
	docs := map[string]string{"GeomViews::geometryView": "geom.sysml", "KitViews::widgetTable": "kit.sysml", "KitViews::widgetTree": "kit.sysml"}
	lines := map[string]int{"GeomViews::geometryView": 5, "KitViews::widgetTable": 16, "KitViews::widgetTree": 12}
	for _, info := range views {
		if info.Doc != docs[info.Name] {
			t.Errorf("%s: Doc = %q, want %q", info.Name, info.Doc, docs[info.Name])
		}
		if info.Line != lines[info.Name] {
			t.Errorf("%s: Line = %d, want %d", info.Name, info.Line, lines[info.Name])
		}
		if info.Notation != info.Name {
			t.Errorf("%s: Notation = %q, want the name unquoted names need no quoting", info.Name, info.Notation)
		}
	}
}

// A view of a kind this build does not produce is listed with its kind and the
// reason, as Views lists it.
func TestAllViewsCarriesUnsupportedReasons(t *testing.T) {
	ws := openDoc(t, "geom.sysml", geometryViewModel)
	views := ws.AllViews()
	if len(views) != 1 {
		t.Fatalf("listed %d views, want 1: %+v", len(views), views)
	}
	info := views[0]
	if info.Supported || info.Kind != view.KindGeometry || !strings.Contains(info.Reason, "not supported") {
		t.Errorf("geometry view listed as %+v, want unsupported geometry with a reason", info)
	}
	one, _ := ws.Views("geom.sysml")
	if len(one) != 1 || one[0] != info {
		t.Errorf("Views lists %+v, AllViews %+v; want the same entry", one, info)
	}
}

// Views the library declares are not the workspace's own and are not listed.
func TestAllViewsExcludesLibraryViews(t *testing.T) {
	const lib = "lib/views.sysml"
	text := []byte("package LibViews { part def Thing; view libraryView { expose Thing; } }")
	idx := symbols.NewIndex()
	idx.AddDocumentWithKind(lib, parser.New(source.New(lib, text)).ParseFile(), source.KindSysML)
	idx.MarkLibraryDocument(lib, symbols.LibraryDocument{Tier: symbols.TierLibrary, Digest: symbols.TextDigest(text)})
	idx.ExpandWildcardImports()
	if len(DeclaredViews(idx.DocumentRoot(lib))) == 0 {
		t.Fatal("the library declares no view, so the test proves nothing")
	}
	ws := NewWorkspaceWithIndex(idx)
	ws.Open("own.sysml", []byte("package Own { part def P; view ownView { expose P; } }"), 1)

	views := ws.AllViews()
	if len(views) != 1 || views[0].Name != "Own::ownView" || views[0].Doc != "own.sysml" {
		t.Errorf("listed %+v, want Own::ownView of own.sysml alone", views)
	}
}

// A name needing quotes is written as the notation writes it, each segment
// quoted on its own, so a segment holding `::` reads back whole.
func TestAllViewsNotationQuotesSegments(t *testing.T) {
	ws := openDoc(t, "q.sysml", `package 'My Pkg' {
	part def W;
	view 'a::b view' { expose W; }
	view 'line\nbreak' { expose W; }
}
`)
	got := map[string]bool{}
	for _, info := range ws.AllViews() {
		got[info.Notation] = true
	}
	for _, want := range []string{`'My Pkg'::'a::b view'`, `'My Pkg'::'line\nbreak'`} {
		if !got[want] {
			t.Errorf("no view listed as %s: %v", want, got)
		}
	}
}

// Document definitions carry the notation name and the line they start on.
func TestDocumentDefinitionsLocateAndQuote(t *testing.T) {
	ws := openDoc(t, "d.sysml", `package 'My Pkg' {
	private import DocumentQueries::*;

	part def 'Report "A"' :> Document {
		attribute redefines title = "A";
	}
}
`)
	defs := ws.DocumentDefinitions()
	if len(defs) != 1 {
		t.Fatalf("listed %+v, want one document", defs)
	}
	if defs[0].Notation != `'My Pkg'::'Report "A"'` || defs[0].Line != 4 || defs[0].Doc != "d.sysml" {
		t.Errorf("document listed as %+v", defs[0])
	}
}

const renderedViewModel = `package Rendered {
	part def W { part p : W; }

	view asText { expose W; render Views::asTextualNotation; }
	view asWiring { expose W; render Views::asInterconnectionDiagram; }
	view asTable { expose W; render Views::asElementTable; }
}
`

// A document held as its record lists the kinds its views' render members state,
// as the parsed document does: the listing reads the bodies, so it hydrates.
func TestAllViewsReadsRecordedDocumentsKinds(t *testing.T) {
	cache, err := libs.NewCacheIn(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	inputs := []Input{{Name: "rendered.sysml", Content: []byte(renderedViewModel), Version: 1}}
	cold := NewWorkspace(WithRecordCache(cache))
	cold.OpenAll(inputs)
	cold.DiagnosticsAll([]string{"rendered.sysml"})
	want := cold.AllViews()

	for _, list := range []func(*Workspace) []ViewInfo{
		(*Workspace).AllViews,
		func(w *Workspace) []ViewInfo { views, _ := w.Views("rendered.sysml"); return views },
	} {
		warm := NewWorkspace(WithRecordCache(cache))
		warm.OpenAll(inputs)
		if !warm.Recorded("rendered.sysml") {
			t.Fatal("rendered.sysml is not recorded on a warm cache: the test proves nothing")
		}
		got := list(warm)
		if len(got) != len(want) {
			t.Fatalf("warm listing %+v, cold %+v", got, want)
		}
		for i := range got {
			if got[i].Name != want[i].Name || got[i].Kind != want[i].Kind || got[i].Line != want[i].Line {
				t.Errorf("warm listing %+v, cold %+v", got[i], want[i])
			}
		}
	}
	kinds := []view.Kind{view.KindTable, view.KindTextual, view.KindInterconnection}
	if len(want) != len(kinds) {
		t.Fatalf("cold listing %+v, want %v", want, kinds)
	}
	for i, kind := range kinds {
		if want[i].Kind != kind {
			t.Errorf("cold listing %+v, want kinds %v", want, kinds)
		}
	}
}
