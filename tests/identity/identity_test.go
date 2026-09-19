package identity_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// buildTable analyzes src over the bundled libraries and computes its table.
func buildTable(t *testing.T, src string) (*identity.Table, *symbols.Index) {
	t.Helper()
	p := parser.New(source.New("<t>", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument("<t>", root)
	idx.ExpandWildcardImports()
	res := resolve.New(idx)
	model := semantics.NewModel(res)
	res.SetModel(model)
	return identity.Build(model, res, idx.DocumentRoot("<t>")), idx
}

func infoOf(t *testing.T, table *identity.Table, idx *symbols.Index, fqn string) *identity.Info {
	t.Helper()
	matches := idx.LookupQualified(fqn)
	if len(matches) == 0 {
		t.Fatalf("%s: no symbol", fqn)
	}
	info, ok := table.Info(matches[0])
	if !ok {
		t.Fatalf("%s: no identity info", fqn)
	}
	return info
}

func TestAnnotatedElementCarriesItsDeclaredID(t *testing.T) {
	table, idx := buildTable(t, `package Vehicles {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "main"; }
	part def Vehicle {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
}
`)
	info := infoOf(t, table, idx, "Vehicles::Vehicle")
	if !info.Annotated || info.DeclaredID != "8f3a41d0" || info.EffectiveID != "8f3a41d0" {
		t.Fatalf("annotated=%v declared=%q effective=%q, want annotated 8f3a41d0",
			info.Annotated, info.DeclaredID, info.EffectiveID)
	}
	if info.Scope == nil || info.Scope.ProjectID != "proj-1" || info.Scope.Branch != "main" || info.Scope.Org != "" {
		t.Fatalf("scope = %+v, want projectId proj-1, branch main, empty org", info.Scope)
	}
}

func TestUnannotatedElementDerivesItsID(t *testing.T) {
	table, idx := buildTable(t, `package Vehicles {
	part def Vehicle;
}
`)
	info := infoOf(t, table, idx, "Vehicles::Vehicle")
	want := rdf.EncodeElementID("Vehicles::Vehicle")
	if info.Annotated || info.EffectiveID != want {
		t.Fatalf("annotated=%v effective=%q, want derived %q", info.Annotated, info.EffectiveID, want)
	}
	if info.Scope != nil {
		t.Fatalf("scope = %+v, want unbound", info.Scope)
	}
}

func TestDeclaredIDEqualToDerivedIDIsStillAnnotated(t *testing.T) {
	derived := rdf.EncodeElementID("P::e")
	table, idx := buildTable(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def e {
		@IdentityMetadata::ElementId { id = "`+derived+`"; }
	}
}
`)
	info := infoOf(t, table, idx, "P::e")
	if !info.Annotated || info.DeclaredID != derived || info.EffectiveID != derived {
		t.Fatalf("annotated=%v declared=%q effective=%q, want the derived id %q kept explicit",
			info.Annotated, info.DeclaredID, info.EffectiveID, derived)
	}
}

func TestNestedProjectRefScopesResolveToTheNearest(t *testing.T) {
	table, idx := buildTable(t, `package Outer {
	@IdentityMetadata::ProjectRef { projectId = "outer-proj"; org = "org-a"; }
	part def A;
	package Inner {
		@IdentityMetadata::ProjectRef { projectId = "inner-proj"; }
		part def B;
	}
}
`)
	a := infoOf(t, table, idx, "Outer::A")
	if a.Scope == nil || a.Scope.ProjectID != "outer-proj" || a.Scope.Org != "org-a" {
		t.Fatalf("A scope = %+v, want outer-proj/org-a", a.Scope)
	}
	b := infoOf(t, table, idx, "Outer::Inner::B")
	if b.Scope == nil || b.Scope.ProjectID != "inner-proj" || b.Scope.Org != "" {
		t.Fatalf("B scope = %+v, want inner-proj", b.Scope)
	}
	inner := infoOf(t, table, idx, "Outer::Inner")
	if inner.Scope == nil || inner.Scope.ProjectID != "inner-proj" {
		t.Fatalf("Inner scope = %+v, want its own inner-proj", inner.Scope)
	}
	if a.Scope.Key() == b.Scope.Key() {
		t.Fatal("distinct projects must have distinct scope keys")
	}
}

func TestAboutFormElementIdCarriesItsDeclaredID(t *testing.T) {
	table, idx := buildTable(t, `package Vehicles {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def Vehicle;
	metadata vid : IdentityMetadata::ElementId about Vehicle {
		id = "8f3a41d0";
	}
}
`)
	info := infoOf(t, table, idx, "Vehicles::Vehicle")
	if !info.Annotated || info.DeclaredID != "8f3a41d0" || info.EffectiveID != "8f3a41d0" {
		t.Fatalf("annotated=%v declared=%q effective=%q, want the about-form id 8f3a41d0",
			info.Annotated, info.DeclaredID, info.EffectiveID)
	}
	if len(info.Declarations) != 1 || !info.Declarations[0].About {
		t.Fatalf("declarations = %+v, want one about-form declaration", info.Declarations)
	}
}

func TestAboutFormProjectRefBindsTheScope(t *testing.T) {
	table, idx := buildTable(t, `package Vehicles {
	part def Vehicle;
}
package Meta {
	metadata pref : IdentityMetadata::ProjectRef about Vehicles {
		projectId = "proj-1";
		org = "org-a";
	}
}
`)
	info := infoOf(t, table, idx, "Vehicles::Vehicle")
	if info.Scope == nil || info.Scope.ProjectID != "proj-1" || info.Scope.Org != "org-a" {
		t.Fatalf("scope = %+v, want proj-1/org-a from the about-form ProjectRef", info.Scope)
	}
}

func TestUnnamedAboutFormElementIdCarriesItsDeclaredID(t *testing.T) {
	table, idx := buildTable(t, `package Vehicles {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def Vehicle;
	metadata : IdentityMetadata::ElementId about Vehicle {
		id = "8f3a41d0";
	}
}
`)
	info := infoOf(t, table, idx, "Vehicles::Vehicle")
	if !info.Annotated || info.DeclaredID != "8f3a41d0" || info.EffectiveID != "8f3a41d0" {
		t.Fatalf("annotated=%v declared=%q effective=%q, want the unnamed about-form id 8f3a41d0",
			info.Annotated, info.DeclaredID, info.EffectiveID)
	}
	if len(info.Declarations) != 1 || !info.Declarations[0].About {
		t.Fatalf("declarations = %+v, want one about-form declaration", info.Declarations)
	}
}

func TestUnnamedAboutFormProjectRefBindsTheScope(t *testing.T) {
	table, idx := buildTable(t, `package Vehicles {
	part def Vehicle;
}
package Meta {
	metadata : IdentityMetadata::ProjectRef about Vehicles {
		projectId = "proj-1";
		org = "org-a";
	}
}
`)
	info := infoOf(t, table, idx, "Vehicles::Vehicle")
	if info.Scope == nil || info.Scope.ProjectID != "proj-1" || info.Scope.Org != "org-a" {
		t.Fatalf("scope = %+v, want proj-1/org-a from the unnamed about-form ProjectRef", info.Scope)
	}
}

func TestInlineAndAboutFormDeclarationsAreBothRecorded(t *testing.T) {
	table, idx := buildTable(t, `package Vehicles {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def Vehicle {
		@IdentityMetadata::ElementId { id = "inline-id"; }
	}
	metadata vid : IdentityMetadata::ElementId about Vehicle {
		id = "about-id";
	}
}
`)
	info := infoOf(t, table, idx, "Vehicles::Vehicle")
	if len(info.Declarations) != 2 {
		t.Fatalf("declarations = %+v, want the inline and the about-form one", info.Declarations)
	}
	if info.Declarations[0].About || info.Declarations[0].ID != "inline-id" {
		t.Fatalf("first declaration = %+v, want the inline id", info.Declarations[0])
	}
	if !info.Declarations[1].About || info.Declarations[1].ID != "about-id" {
		t.Fatalf("second declaration = %+v, want the about-form id", info.Declarations[1])
	}
	if info.EffectiveID != "inline-id" {
		t.Fatalf("effective = %q, want the inline id first", info.EffectiveID)
	}
}

func TestAboutFormAnnotationTargetingLibraryElementIsTabled(t *testing.T) {
	table, idx := buildTable(t, `package Meta {
	metadata sid : IdentityMetadata::ElementId about ScalarValues::Boolean {
		id = "bool-id";
	}
}
`)
	info := infoOf(t, table, idx, "ScalarValues::Boolean")
	if !info.Annotated || info.DeclaredID != "bool-id" || info.EffectiveID != "bool-id" {
		t.Fatalf("annotated=%v declared=%q effective=%q, want the about-form id bool-id on the library element",
			info.Annotated, info.DeclaredID, info.EffectiveID)
	}
}

func TestLibraryDocumentAboutFormAnnotationsAreIndexed(t *testing.T) {
	lib := `package LibMeta {
	metadata pref : IdentityMetadata::ProjectRef about Vehicles {
		projectId = "proj-lib";
		org = "org-lib";
	}
}
`
	p := parser.New(source.New("<lib>", []byte(lib)))
	libRoot := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	src := `package Vehicles {
	part def Vehicle;
}
`
	q := parser.New(source.New("<t>", []byte(src)))
	root := q.ParseFile()
	if len(q.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", q.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument("<lib>", libRoot)
	idx.MarkLibrary("<lib>")
	idx.AddDocument("<t>", root)
	idx.ExpandWildcardImports()
	res := resolve.New(idx)
	model := semantics.NewModel(res)
	res.SetModel(model)
	table := identity.Build(model, res, idx.DocumentRoot("<t>"))
	info := infoOf(t, table, idx, "Vehicles::Vehicle")
	if info.Scope == nil || info.Scope.ProjectID != "proj-lib" || info.Scope.Org != "org-lib" {
		t.Fatalf("scope = %+v, want proj-lib/org-lib from the library document's about-form ProjectRef", info.Scope)
	}
}

func TestLibraryElementCarriesItsNormativeID(t *testing.T) {
	_, idx := buildTable(t, `package Use { import ScalarValues::Real; }`)
	// Build tables only the document's own symbols; library ones are computed directly.
	res := resolve.New(idx)
	model := semantics.NewModel(res)
	res.SetModel(model)
	for _, tc := range []struct{ fqn, id, lang, membership string }{
		{"ScalarValues", "40bb440c-5036-58e1-8675-5afccb8b8f1d", "KerML", "1cecd337-acef-524c-847a-1f3e7a2e65e9"},
		{"ScalarValues::Real", "14c0aa22-5489-59b5-b438-ded26e83ba31", "KerML", "ab72a695-5fe9-58a3-9d48-9e9a8711862d"},
	} {
		syms := idx.LookupQualified(tc.fqn)
		if len(syms) == 0 {
			t.Fatalf("%s: no symbol", tc.fqn)
		}
		info, ok := identity.Of(model, res, syms[0])
		if !ok {
			t.Fatalf("%s: no identity", tc.fqn)
		}
		if info.Source != identity.SourceNormative || !info.Normative() || info.Language.String() != tc.lang {
			t.Errorf("%s: source=%v language=%v, want normative %s", tc.fqn, info.Source, info.Language, tc.lang)
		}
		if info.EffectiveID != tc.id {
			t.Errorf("%s: effective=%q, want %q", tc.fqn, info.EffectiveID, tc.id)
		}
		if got := info.OwningMembershipID(); got != tc.membership {
			t.Errorf("%s: owning membership=%q, want %q", tc.fqn, got, tc.membership)
		}
		if info.Annotated || info.Declared {
			t.Errorf("%s: normative id reported as an annotation", tc.fqn)
		}
	}
}

func TestSystemsLibraryElementIsMintedUnderSysML(t *testing.T) {
	_, idx := buildTable(t, `package Use { import Parts::*; }`)
	res := resolve.New(idx)
	model := semantics.NewModel(res)
	res.SetModel(model)
	syms := idx.LookupQualified("Parts::Part")
	if len(syms) == 0 {
		t.Fatal("Parts::Part: no symbol")
	}
	info, ok := identity.Of(model, res, syms[0])
	if !ok || info.Language.String() != "SysML" || info.Source != identity.SourceNormative {
		t.Fatalf("Parts::Part: info=%+v, want SysML normative", info)
	}
	if info.EffectiveID == rdf.EncodeElementID("Parts::Part") || info.EffectiveID == "" {
		t.Fatalf("Parts::Part: effective=%q, want a UUID", info.EffectiveID)
	}
}

func TestUserElementIsDerivedAndOpenSysMLLibraryIsNotNormative(t *testing.T) {
	table, idx := buildTable(t, `package Vehicles { part def Vehicle; }`)
	info := infoOf(t, table, idx, "Vehicles::Vehicle")
	if info.Source != identity.SourceDerived || info.Normative() || info.OwningMembershipID() != "" {
		t.Fatalf("Vehicles::Vehicle: source=%v, want derived", info.Source)
	}
	res := resolve.New(idx)
	model := semantics.NewModel(res)
	res.SetModel(model)
	syms := idx.LookupQualified("IdentityMetadata::ElementId")
	if len(syms) == 0 {
		t.Fatal("IdentityMetadata::ElementId: no symbol")
	}
	ext, ok := identity.Of(model, res, syms[0])
	if !ok || ext.Source != identity.SourceDerived {
		t.Fatalf("IdentityMetadata::ElementId: source=%v, want derived (an OpenSysML extension is not normative)", ext.Source)
	}
}

func TestDeclaredIDOverridesTheNormativeID(t *testing.T) {
	table, idx := buildTable(t, `package Meta {
	metadata sid : IdentityMetadata::ElementId about ScalarValues::Boolean {
		id = "bool-id";
	}
}
`)
	info := infoOf(t, table, idx, "ScalarValues::Boolean")
	if info.Source != identity.SourceDeclared || info.EffectiveID != "bool-id" || info.Language != 0 {
		t.Fatalf("source=%v effective=%q language=%v, want declared bool-id", info.Source, info.EffectiveID, info.Language)
	}
}

func TestLibraryCatalogNamesEveryNormativeIDAndIsSharedByOverlays(t *testing.T) {
	_, idx := buildTable(t, `package Use;`)
	catalog := identity.LibraryCatalog(idx)
	if identity.LibraryCatalog(libs.NewModelIndex()) != catalog {
		t.Fatal("two overlays over one library base should share one catalog")
	}
	el, ok := catalog.Element("14c0aa22-5489-59b5-b438-ded26e83ba31")
	if !ok || el.FQN != "ScalarValues::Real" || el.Language.String() != "KerML" {
		t.Fatalf("Real by id: %+v, %v", el, ok)
	}
	om, ok := catalog.OwningMembership("ab72a695-5fe9-58a3-9d48-9e9a8711862d")
	if !ok || om != el {
		t.Fatalf("Real's owning membership by id: %+v, %v", om, ok)
	}
	derived := rdf.EncodeElementID("ScalarValues::Real")
	if _, ok := catalog.Element(derived); ok {
		t.Fatal("a derived id is not a normative one")
	}
	if _, ok := catalog.OwningMembership(derived); ok {
		t.Fatal("a derived id is not a normative membership either")
	}
	edges, ok := catalog.Element("6749b419-719a-51d9-8e13-d993bb953e80")
	if !ok || edges.FQN != "ShapeItems::RectangularPyramid::base::edges" {
		t.Fatalf("%+v: an effectively named member is catalogued under its effective name", edges)
	}
	res := resolve.New(idx)
	model := semantics.NewModel(res)
	res.SetModel(model)
	seen := map[string]bool{}
	for _, el := range catalog.Elements() {
		if seen[el.ID] {
			t.Fatalf("%s: id %s catalogued twice", el.FQN, el.ID)
		}
		seen[el.ID] = true
		info, ok := identity.Of(model, res, el.Symbol)
		if !ok || info.EffectiveID != el.ID || info.OwningMembershipID() != el.OwningMembershipID || info.Language != el.Language {
			t.Fatalf("%s: catalog and identity.Of disagree: %+v vs %+v", el.FQN, el, info)
		}
	}
	if len(seen) < 5000 {
		t.Fatalf("catalogued %d elements, want the whole named library", len(seen))
	}
}

// An overlay that shadows, removes or adds a library document is judged by the
// library it shows, not by its frozen base's; one that shows the base's library
// as the base does shares the base's catalog. The judged document is no part of
// that library: indexed under a library file's own name, it is judged against the file.
func TestLibraryVersionOverAnOverlayThatChangesTheLibrary(t *testing.T) {
	const lib = "lib/tanks.sysml"
	oldText := []byte("standard library package OldTanks {\n    part def Tank;\n}\n")
	newText := []byte("standard library package Tanks {\n    part def Tank;\n}\n")
	hullsText := []byte("standard library package Hulls {\n    part def Hull;\n}\n")
	library := func(idx *symbols.Index, name string, text []byte) {
		idx.AddDocumentWithKind(name, parser.New(source.New(name, text)).ParseFile(), source.KindSysML)
		idx.MarkLibraryDocument(name, symbols.LibraryDocument{Tier: symbols.TierSystems, Digest: symbols.TextDigest(text)})
	}
	versionUnder := func(t *testing.T, idx *symbols.Index, name string, text []byte) string {
		t.Helper()
		idx.AddDocumentWithKind(name, parser.New(source.New(name, text)).ParseFile(), source.KindSysML)
		idx.ExpandWildcardImports()
		res := resolve.New(idx)
		model := semantics.NewModel(res)
		res.SetModel(model)
		return identity.LibraryVersion(model, res, name)
	}
	versionOf := func(t *testing.T, idx *symbols.Index, text []byte) string {
		t.Helper()
		return versionUnder(t, idx, "copy.sysml", text)
	}
	base := symbols.NewIndex()
	library(base, lib, oldText)
	base.Freeze()

	shown := symbols.NewOverlay(base)
	if identity.LibraryCatalog(shown) != identity.LibraryCatalog(base) {
		t.Error("an overlay showing the base's library as the base does should share its catalog")
	}
	library(shown, lib, newText)
	if got := versionOf(t, shown, newText); got != lib {
		t.Errorf("a copy of the package the overlay shows: version %q, want %q", got, lib)
	}
	if got := versionOf(t, shown, oldText); got != "" {
		t.Errorf("a copy of the package the overlay shadows: version %q, want none", got)
	}

	removed := symbols.NewOverlay(base)
	removed.RemoveDocument(lib)
	if got := versionOf(t, removed, oldText); got != "" {
		t.Errorf("a copy of the package the overlay removed: version %q, want none", got)
	}

	added := symbols.NewOverlay(base)
	library(added, "lib/hulls.sysml", hullsText)
	if got := versionOf(t, added, hullsText); got != "lib/hulls.sysml" {
		t.Errorf("a copy of the package the overlay adds: version %q, want lib/hulls.sysml", got)
	}
	if got := versionOf(t, added, oldText); got != lib {
		t.Errorf("a copy of the base's package under an overlay that adds one: version %q, want %q", got, lib)
	}

	edited := []byte("standard library package OldTanks {\n    part def Tank;\n    part def Lid;\n}\n")
	underName := symbols.NewOverlay(base)
	if got := versionUnder(t, underName, lib, edited); got != lib {
		t.Errorf("an edited copy under the library file's own name: version %q, want %q", got, lib)
	}
	if got := versionUnder(t, underName, lib, hullsText); got != "" {
		t.Errorf("another package under the library file's own name: version %q, want none", got)
	}
	underNameAdded := symbols.NewOverlay(base)
	library(underNameAdded, "lib/hulls.sysml", hullsText)
	if got := versionUnder(t, underNameAdded, lib, edited); got != lib {
		t.Errorf("an edited copy under the library file's own name, over an overlay that adds one: version %q, want %q", got, lib)
	}
	if got := versionUnder(t, underNameAdded, lib, hullsText); got != "lib/hulls.sysml" {
		t.Errorf("a copy of the added package under the base file's name: version %q, want lib/hulls.sysml", got)
	}
}
