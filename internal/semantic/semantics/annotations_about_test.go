package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// buildOverlayModel indexes libSrc as a frozen base — the shape the shared
// standard library index has — and userSrc as the workspace document of an
// overlay over it, returning the model and both document root scopes.
func buildOverlayModel(t *testing.T, libSrc, userSrc string) (*Model, *symbols.Scope, *symbols.Scope) {
	t.Helper()
	base := symbols.NewIndex()
	addTestDoc(t, base, "lib.sysml", libSrc)
	base.MarkLibrary("lib.sysml")
	base.Freeze()

	over := symbols.NewOverlay(base)
	addTestDoc(t, over, "user.sysml", userSrc)
	over.ExpandWildcardImports()

	r := resolve.New(over)
	m := NewModel(r)
	r.SetModel(m)
	return m, over.DocumentRoot("lib.sysml"), over.DocumentRoot("user.sysml")
}

func addTestDoc(t *testing.T, idx *symbols.Index, name, src string) {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics for %s: %v", name, p.Diagnostics)
	}
	idx.AddDocument(name, root)
}

// annotationTypes reports the FQNs of the metadata types annotating sym.
func annotationTypes(m *Model, sym *symbols.Symbol) []string {
	var out []string
	for _, facts := range m.AnnotationFactsOf(sym) {
		out = append(out, facts.TypeFQN)
	}
	return out
}

// An `about` annotation declared by a frozen library document is read from the
// cache its Freeze built and found exactly as walking the document found it.
func TestAboutAnnotationDeclaredByAFrozenLibraryDocument(t *testing.T) {
	m, lib, _ := buildOverlayModel(t, `
		metadata def Safety;
		part def Belt;
		metadata s : Safety about Belt;
	`, "part x;")

	belt := sym(t, lib, "Belt")
	types := annotationTypes(m, belt)
	if len(types) != 1 || types[0] != "Safety" {
		t.Fatalf("annotations of Belt = %v, want [Safety]", types)
	}
	annotated := m.AboutAnnotatedSymbols()
	if len(annotated) != 1 || annotated[0] != belt {
		t.Fatalf("AboutAnnotatedSymbols = %v, want [Belt]", annotated)
	}
}

// A workspace document's `about` annotation targeting a frozen library element
// is still found: the workspace document is walked, cache or no cache.
func TestAboutAnnotationFromWorkspaceTargetsALibraryElement(t *testing.T) {
	m, lib, _ := buildOverlayModel(t, `
		metadata def Safety;
		part def Belt;
	`, "metadata s : Safety about Belt;")

	types := annotationTypes(m, sym(t, lib, "Belt"))
	if len(types) != 1 || types[0] != "Safety" {
		t.Fatalf("annotations of Belt = %v, want [Safety]", types)
	}
}

// A workspace document's `about` annotation targeting another workspace
// element is found over an overlay, and combines with the library's own.
func TestAboutAnnotationsFromLibraryAndWorkspaceCombine(t *testing.T) {
	m, lib, user := buildOverlayModel(t, `
		metadata def Safety;
		part def Belt;
		metadata s : Safety about Belt;
	`, `
		metadata def Comfort;
		part radio;
		metadata c : Comfort about radio;
	`)

	if types := annotationTypes(m, sym(t, user, "radio")); len(types) != 1 || types[0] != "Comfort" {
		t.Fatalf("annotations of radio = %v, want [Comfort]", types)
	}
	if types := annotationTypes(m, sym(t, lib, "Belt")); len(types) != 1 || types[0] != "Safety" {
		t.Fatalf("annotations of Belt = %v, want [Safety]", types)
	}
}

// `.metadata` reads an element's annotations in textual order across the
// documents stating them: document order first ("lib.sysml" sorts before
// "user.sysml"), then source position, an `about` usage written before the
// element's own inline annotation coming first.
func TestElementMetadataOrderedByDocumentThenPosition(t *testing.T) {
	m, _, user := buildOverlayModel(t, `
		metadata def Safety;
		metadata def Comfort;
		metadata def Audit;
		metadata s : Safety about radio;
	`, `
		metadata a : Audit about radio;
		part radio { @Comfort; }
		metadata s2 : Safety about radio;
	`)

	type site struct {
		typ   string
		doc   string
		about bool
	}
	var got []site
	for _, md := range m.ElementMetadataOf(sym(t, user, "radio")) {
		got = append(got, site{m.fqnOf(md.Type), md.Doc, md.About})
	}
	want := []site{
		{"Safety", "lib.sysml", true},
		{"Audit", "user.sysml", true},
		{"Comfort", "user.sysml", false},
		{"Safety", "user.sysml", true},
	}
	if len(got) != len(want) {
		t.Fatalf("metadata of radio = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("metadata[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

// The `@M about x` spelling of an `about` usage annotates what it names, from a
// frozen library document and from the workspace alike, and never the
// namespace it is written in.
func TestAboutAnnotationInThePrefixSpelling(t *testing.T) {
	m, lib, user := buildOverlayModel(t, `
		metadata def Safety;
		part def Belt;
		part def Buckle;
		package Notes {
			@Safety about Belt;
		}
	`, `
		metadata def Comfort;
		part radio;
		part def Seat {
			part cushion;
			@Comfort about cushion, radio;
		}
	`)

	if types := annotationTypes(m, sym(t, lib, "Belt")); len(types) != 1 || types[0] != "Safety" {
		t.Fatalf("annotations of Belt = %v, want [Safety]", types)
	}
	if types := annotationTypes(m, sym(t, lib, "Notes")); len(types) != 0 {
		t.Fatalf("annotations of Notes = %v, want none", types)
	}
	if types := annotationTypes(m, sym(t, user, "radio")); len(types) != 1 || types[0] != "Comfort" {
		t.Fatalf("annotations of radio = %v, want [Comfort]", types)
	}
	seat := sym(t, user, "Seat")
	if types := annotationTypes(m, sym(t, seat.Scope, "cushion")); len(types) != 1 || types[0] != "Comfort" {
		t.Fatalf("annotations of cushion = %v, want [Comfort]", types)
	}
	if types := annotationTypes(m, seat); len(types) != 0 {
		t.Fatalf("annotations of Seat = %v, want none", types)
	}

	var usage *symbols.Symbol
	seat.Scope.ForEachMember(func(s *symbols.Symbol) bool {
		if s.Kind == symbols.SymbolMetadataUsage {
			usage = s
		}
		return true
	})
	if usage == nil {
		t.Fatal("Seat declares no metadata usage")
	}
	annotated := m.AnnotatedElementsOf(usage)
	if len(annotated) != 2 || annotated[0] != sym(t, seat.Scope, "cushion") || annotated[1] != sym(t, user, "radio") {
		t.Fatalf("AnnotatedElementsOf = %v, want [cushion radio]", annotated)
	}
	if facts := m.AboutAnnotationFactsOf(usage); facts == nil || facts.TypeFQN != "Comfort" {
		t.Fatalf("AboutAnnotationFactsOf = %+v, want Comfort", facts)
	}
}
