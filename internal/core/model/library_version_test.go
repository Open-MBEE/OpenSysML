package model

import (
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/edit"
	"github.com/Open-MBEE/OpenSysML/internal/core/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const scalarValues = "Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml"

// realOf is the one ScalarValues::Real the workspace resolves, and the document
// declaring it.
func realOf(t *testing.T, ws *Workspace) (*symbols.Symbol, string) {
	t.Helper()
	syms := ws.LookupQualified("ScalarValues::Real")
	if len(syms) != 1 {
		t.Fatalf("ScalarValues::Real = %d symbols, want 1", len(syms))
	}
	return syms[0], syms[0].DocName
}

// A workspace document rooted at a library's top-level package stands in for
// the bundled file: it alone declares the library's names, under the norm's ids.
func TestWorkspaceLibraryVersionStandsInForBundledFile(t *testing.T) {
	ws := NewWorkspace()
	lib := ws.LibraryDocument(scalarValues)
	if lib == nil {
		t.Fatalf("%s not bundled", scalarValues)
	}
	ws.Open("copy.kerml", lib.Content, 1)
	if got := ws.StandsInFor("copy.kerml"); got != scalarValues {
		t.Fatalf("StandsInFor = %q, want %q", got, scalarValues)
	}
	sym, doc := realOf(t, ws)
	if doc != "copy.kerml" {
		t.Fatalf("ScalarValues::Real declared in %q, want the copy", doc)
	}
	info, ok := ws.IdentityOf("copy.kerml", sym)
	if !ok || !info.Normative() || info.EffectiveID != "14c0aa22-5489-59b5-b438-ded26e83ba31" {
		t.Errorf("identity of the copy's Real = %+v, want the norm's", info)
	}
	if ws.IsLibraryDocument("copy.kerml") {
		t.Error("the copy is the workspace's own file, not a bundled one")
	}
	if !ws.IsLibraryDocument(scalarValues) {
		t.Error("the displaced bundled file is still a library document")
	}
	var bundledReal *symbols.Symbol
	walkScope(lib.Scope, func(s *symbols.Symbol) {
		if s.Name == "Real" {
			bundledReal = s
		}
	})
	if info, ok := ws.IdentityOf(scalarValues, bundledReal); !ok || !info.Normative() ||
		info.EffectiveID != "14c0aa22-5489-59b5-b438-ded26e83ba31" {
		t.Errorf("identity of the displaced bundled Real = %+v, want the norm's", info)
	}
	for _, d := range ws.Diagnostics("copy.kerml") {
		if d.Code == "library-package" || d.Code == "duplicate-name" {
			t.Errorf("the copy is judged as a user file: %s: %s", d.Code, d.Message)
		}
	}

	// A second version of the same file stands in beside the first.
	ws.Open("other.kerml", lib.Content, 1)
	if syms := ws.LookupQualified("ScalarValues::Real"); len(syms) != 2 {
		t.Fatalf("ScalarValues::Real = %d symbols with two versions open, want 2", len(syms))
	}
	ws.Remove("other.kerml")
	if _, doc := realOf(t, ws); doc != "copy.kerml" {
		t.Fatalf("ScalarValues::Real declared in %q after one version left, want the copy", doc)
	}

	// Removing the last version brings the bundled file back.
	ws.Remove("copy.kerml")
	if _, doc := realOf(t, ws); doc != scalarValues {
		t.Fatalf("ScalarValues::Real declared in %q after the copy left, want the bundled file", doc)
	}
	if !ws.IsLibraryDocument(scalarValues) {
		t.Error("the restored bundled file is a library document")
	}
}

// The library's identity follows its text, not the names its files are held under: a
// byte-identical version standing in leaves it as it was, an edited one moves it.
func TestWorkspaceLibraryVersionKeepsLibraryIdentity(t *testing.T) {
	ws := NewWorkspace()
	lib := ws.LibraryDocument(scalarValues)
	if lib == nil {
		t.Fatalf("%s not bundled", scalarValues)
	}
	identityOf := func(when string) string {
		t.Helper()
		ws.mu.RLock()
		defer ws.mu.RUnlock()
		id, known := ws.index.LibraryIdentity()
		if !known {
			t.Fatalf("%s: the library's identity is unknown", when)
		}
		return id
	}
	bundled := identityOf("before any version is open")

	ws.Open("copy.kerml", lib.Content, 1)
	if got := ws.StandsInFor("copy.kerml"); got != scalarValues {
		t.Fatalf("StandsInFor = %q, want %q", got, scalarValues)
	}
	if got := identityOf("with an unchanged version open"); got != bundled {
		t.Errorf("an unchanged version of %s changed the library's identity", scalarValues)
	}

	edited := strings.Replace(string(lib.Content), "datatype Real", "datatype Real // edited", 1)
	if edited == string(lib.Content) {
		t.Fatal("Real is not declared where expected")
	}
	ws.Open("copy.kerml", []byte(edited), 2)
	if got := ws.StandsInFor("copy.kerml"); got != scalarValues {
		t.Fatalf("StandsInFor = %q after the edit, want %q", got, scalarValues)
	}
	if got := identityOf("with an edited version open"); got == bundled {
		t.Errorf("an edited version of %s left the library's identity as it was", scalarValues)
	}

	ws.Remove("copy.kerml")
	if got := identityOf("after the version closed"); got != bundled {
		t.Errorf("the library's identity did not return with the bundled file")
	}
}

// An edit moving the roots off the library's package makes the document the
// workspace's own, with derived ids, and restores the bundled file; moving them
// back makes it the library again.
func TestWorkspaceLibraryVersionFollowsRootEdits(t *testing.T) {
	ws := NewWorkspace()
	lib := ws.LibraryDocument(scalarValues)
	if lib == nil {
		t.Fatalf("%s not bundled", scalarValues)
	}
	src := string(lib.Content)
	ws.Open("copy.kerml", lib.Content, 1)
	for _, tc := range []struct {
		name, text string
	}{
		{"renamed root", strings.Replace(src, "standard library package ScalarValues", "standard library package MyValues", 1)},
		{"dropped keywords", strings.Replace(src, "standard library package ScalarValues", "package ScalarValues", 1)},
		{"foreign id", strings.Replace(src, "standard library package ScalarValues {",
			"standard library package ScalarValues {\n\t@IdentityMetadata::ElementId { id = \"not-the-norm\"; }", 1)},
	} {
		if tc.text == src {
			t.Fatalf("%s: ScalarValues is not declared where expected", tc.name)
		}
		ws.Update("copy.kerml", []byte(tc.text), 2)
		if got := ws.StandsInFor("copy.kerml"); got != "" {
			t.Errorf("%s: StandsInFor = %q, want nothing", tc.name, got)
		}
		if !ws.IsLibraryDocument(scalarValues) {
			t.Errorf("%s: the bundled file did not come back", tc.name)
		}
		root := ws.index.DocumentRoot("copy.kerml")
		if root == nil || len(root.Members()) == 0 {
			t.Fatalf("%s: the copy is not indexed", tc.name)
		}
		info, ok := ws.IdentityOf("copy.kerml", root.Members()[0])
		if !ok || info.Normative() {
			t.Errorf("%s: identity of the copy's root = %+v, want the user's", tc.name, info)
		}
		ws.Update("copy.kerml", lib.Content, 3)
		if got := ws.StandsInFor("copy.kerml"); got != scalarValues {
			t.Errorf("%s: restored copy stands in for %q, want %q", tc.name, got, scalarValues)
		}
		if _, doc := realOf(t, ws); doc != "copy.kerml" {
			t.Errorf("%s: ScalarValues::Real declared in %q after restoring, want the copy", tc.name, doc)
		}
	}
	// A root stating the norm's id is the library's whatever its keywords.
	el, ok := identity.LibraryCatalog(ws.index).ElementNamed("ScalarValues")
	if !ok {
		t.Fatal("ScalarValues not catalogued")
	}
	stated := strings.Replace(src, "standard library package ScalarValues {",
		"package ScalarValues {\n\t@IdentityMetadata::ElementId { id = \""+el.ID+"\"; }", 1)
	ws.Update("copy.kerml", []byte(stated), 4)
	if got := ws.StandsInFor("copy.kerml"); got != scalarValues {
		t.Errorf("root stating the norm's id stands in for %q, want %q", got, scalarValues)
	}
}

// Closing a version whose on-disk text is the user's own reverts to that text
// and the bundled file; closing one that is on disk keeps it standing in.
func TestWorkspaceLibraryVersionCloseFollowsDisk(t *testing.T) {
	ws := NewWorkspace()
	lib := ws.LibraryDocument(scalarValues)
	if lib == nil {
		t.Fatalf("%s not bundled", scalarValues)
	}
	ws.SetOnDisk("copy.kerml", []byte("package Mine { datatype Real; }"))
	ws.Open("copy.kerml", lib.Content, 1)
	if got := ws.StandsInFor("copy.kerml"); got != scalarValues {
		t.Fatalf("open buffer stands in for %q, want %q", got, scalarValues)
	}
	ws.Close("copy.kerml")
	if got := ws.StandsInFor("copy.kerml"); got != "" {
		t.Errorf("closed to the user's text, still stands in for %q", got)
	}
	if _, doc := realOf(t, ws); doc != scalarValues {
		t.Errorf("ScalarValues::Real declared in %q after closing, want the bundled file", doc)
	}
	if syms := ws.LookupQualified("Mine::Real"); len(syms) != 1 {
		t.Errorf("Mine::Real = %d after closing, want the on-disk text indexed", len(syms))
	}

	ws.SetOnDisk("copy.kerml", lib.Content)
	if got := ws.StandsInFor("copy.kerml"); got != scalarValues {
		t.Fatalf("on-disk version stands in for %q, want %q", got, scalarValues)
	}
	ws.DeleteOnDisk("copy.kerml")
	if got := ws.StandsInFor("copy.kerml"); got != "" {
		t.Errorf("deleted, still stands in for %q", got)
	}
	if _, doc := realOf(t, ws); doc != scalarValues {
		t.Errorf("ScalarValues::Real declared in %q after deletion, want the bundled file", doc)
	}
}

// A document opened under a bundled file's own name takes its place, standing in
// for it when it qualifies and as the user's file when it does not; closing it
// puts the bundled file back under its library mark either way.
func TestWorkspaceLibraryVersionUnderTheBundledName(t *testing.T) {
	ws := NewWorkspace()
	lib := ws.LibraryDocument(scalarValues)
	if lib == nil {
		t.Fatalf("%s not bundled", scalarValues)
	}
	ws.Open(scalarValues, lib.Content, 1)
	if got := ws.StandsInFor(scalarValues); got != scalarValues {
		t.Fatalf("StandsInFor = %q, want %q", got, scalarValues)
	}
	sym, _ := realOf(t, ws)
	if info, ok := ws.IdentityOf(scalarValues, sym); !ok || !info.Normative() {
		t.Errorf("identity of Real under the bundled name = %+v, want the norm's", info)
	}
	if ws.IsLibraryDocument(scalarValues) {
		t.Error("a workspace document under the bundled name is the workspace's own")
	}
	ws.Close(scalarValues)
	if !ws.IsLibraryDocument(scalarValues) || !ws.index.IsLibraryDocument(scalarValues) {
		t.Error("the bundled file did not come back under its library mark")
	}
	if _, doc := realOf(t, ws); doc != scalarValues {
		t.Errorf("ScalarValues::Real declared in %q after closing, want the bundled file", doc)
	}

	ws.Open(scalarValues, []byte("package Mine { datatype Real; }"), 1)
	if got := ws.StandsInFor(scalarValues); got != "" {
		t.Errorf("user text under the bundled name stands in for %q, want nothing", got)
	}
	if syms := ws.LookupQualified("ScalarValues::Real"); len(syms) != 0 {
		t.Errorf("ScalarValues::Real = %d symbols while user text holds the name, want 0", len(syms))
	}
	ws.Close(scalarValues)
	if !ws.index.IsLibraryDocument(scalarValues) {
		t.Error("the bundled file did not come back under its library mark")
	}
	if _, doc := realOf(t, ws); doc != scalarValues {
		t.Errorf("ScalarValues::Real declared in %q after closing, want the bundled file", doc)
	}
}

// A version opened beside a workspace document that holds the bundled file's
// name leaves that document in the index, whether it is itself a version or the
// user's text, and closing either leaves the other; the bundled file comes back
// only once both are gone.
func TestWorkspaceLibraryVersionBesideTheBundledName(t *testing.T) {
	ws := NewWorkspace()
	lib := ws.LibraryDocument(scalarValues)
	if lib == nil {
		t.Fatalf("%s not bundled", scalarValues)
	}
	declaring := func(qname string) []string {
		var docs []string
		for _, s := range ws.LookupQualified(qname) {
			docs = append(docs, s.DocName)
		}
		sort.Strings(docs)
		return docs
	}
	for _, tc := range []struct {
		name  string
		text  []byte
		qname string
		holds []string
	}{
		{"a version", lib.Content, "ScalarValues::Real", []string{scalarValues, "copy.kerml"}},
		{"the user's text", []byte("package Mine { datatype Real; }"), "Mine::Real", []string{scalarValues}},
	} {
		ws.Open(scalarValues, tc.text, 1)
		ws.Open("copy.kerml", lib.Content, 1)
		if got := ws.StandsInFor("copy.kerml"); got != scalarValues {
			t.Fatalf("%s under the bundled name: the copy stands in for %q, want %q", tc.name, got, scalarValues)
		}
		if ws.index.DocumentRoot(scalarValues) == nil {
			t.Errorf("%s under the bundled name left the index when the copy opened", tc.name)
		}
		if got := declaring(tc.qname); !slices.Equal(got, tc.holds) {
			t.Errorf("%s under the bundled name: %s declared in %q, want %q", tc.name, tc.qname, got, tc.holds)
		}
		ws.Close("copy.kerml")
		if ws.index.DocumentRoot(scalarValues) == nil || ws.IsLibraryDocument(scalarValues) {
			t.Errorf("%s under the bundled name did not survive the copy closing", tc.name)
		}
		if got := declaring(tc.qname); !slices.Equal(got, []string{scalarValues}) {
			t.Errorf("%s under the bundled name: %s declared in %q after the copy closed", tc.name, tc.qname, got)
		}
		ws.Open("copy.kerml", lib.Content, 2)
		ws.Close(scalarValues)
		if ws.index.DocumentRoot(scalarValues) != nil {
			t.Errorf("%s: the bundled file came back while the copy stands in for it", tc.name)
		}
		if _, doc := realOf(t, ws); doc != "copy.kerml" {
			t.Errorf("%s: ScalarValues::Real declared in %q with only the copy open, want the copy", tc.name, doc)
		}
		ws.Close("copy.kerml")
		if !ws.IsLibraryDocument(scalarValues) {
			t.Errorf("%s: the bundled file did not come back once both closed", tc.name)
		}
		if _, doc := realOf(t, ws); doc != scalarValues {
			t.Errorf("%s: ScalarValues::Real declared in %q once both closed, want the bundled file", tc.name, doc)
		}
	}
}

// The bundled file a version displaced comes back with the names it surfaces
// through wildcard imports: a package importing it re-exports them again.
func TestWorkspaceLibraryVersionRestoresImports(t *testing.T) {
	ws := NewWorkspace()
	lib := ws.LibraryDocument(scalarValues)
	if lib == nil {
		t.Fatalf("%s not bundled", scalarValues)
	}
	ws.Open("mine.sysml", []byte("package Mine {\n    public import ScalarValues::*;\n}\n"), 1)
	ws.Open("car.sysml", []byte("part def Car {\n    attribute mass : Mine::Real;\n}\n"), 1)
	imported := func(when string, want string) {
		t.Helper()
		syms := ws.LookupQualified("Mine::Real")
		if len(syms) != 1 || syms[0].DocName != want {
			var docs []string
			for _, s := range syms {
				docs = append(docs, s.DocName)
			}
			t.Errorf("%s: Mine::Real declared in %q, want %q", when, docs, want)
		}
		for _, d := range ws.Diagnostics("car.sysml") {
			if d.Severity == passes.SeverityError {
				t.Errorf("%s: car.sysml: %s: %s", when, d.Code, d.Message)
			}
		}
	}
	imported("before any version", scalarValues)
	ws.Open("copy.kerml", lib.Content, 1)
	imported("with the version open", "copy.kerml")
	ws.Remove("copy.kerml")
	imported("after the version left", scalarValues)

	ws.Open("copy.kerml", lib.Content, 2)
	ws.Update("copy.kerml", []byte(strings.Replace(string(lib.Content),
		"standard library package ScalarValues", "standard library package MyValues", 1)), 3)
	imported("after the version moved off the library's root", scalarValues)
}

// An edit inside a version is judged with the workspace document holding the
// bundled file's name still present, so references to its names stay resolved.
func TestWorkspaceLibraryVersionEditsBesideTheBundledName(t *testing.T) {
	ws := NewWorkspace()
	lib := ws.LibraryDocument(scalarValues)
	if lib == nil {
		t.Fatalf("%s not bundled", scalarValues)
	}
	ws.Open(scalarValues, []byte("package Mine { datatype Real; }"), 1)
	ws.Open("copy.kerml", lib.Content, 1)
	ws.Open("car.sysml", []byte("part def Car {\n    attribute mass : ScalarValues::Real;\n    attribute mine : Mine::Real;\n}\n"), 1)
	result, _, ok, err := ws.ApplyEdit("copy.kerml", []edit.Operation{edit.Rename("ScalarValues::Real", "Reel")})
	if !ok || err != nil {
		t.Fatalf("ok %v, err %v", ok, err)
	}
	edited := map[string]string{}
	for _, doc := range result.Documents {
		edited[doc.Name] = string(doc.Content)
	}
	if want := "attribute mass : ScalarValues::Reel;"; !strings.Contains(edited["car.sysml"], want) {
		t.Errorf("the reference to the version is not renamed:\n%s", edited["car.sysml"])
	}
	if want := "attribute mine : Mine::Real;"; !strings.Contains(edited["car.sysml"], want) {
		t.Errorf("the reference beside it changed:\n%s", edited["car.sysml"])
	}
}

// An edit to a document beside a version resolves the library's names to the
// version, as the workspace does, so it is not refused as ambiguous.
func TestWorkspaceLibraryVersionResolvesEdits(t *testing.T) {
	ws := NewWorkspace()
	lib := ws.LibraryDocument(scalarValues)
	if lib == nil {
		t.Fatalf("%s not bundled", scalarValues)
	}
	ws.Open("copy.kerml", lib.Content, 1)
	ws.Open("car.sysml", []byte("part def Car {\n    attribute mass : ScalarValues::Real;\n}\n"), 1)
	op := edit.AddMember("Car", "attribute", "speed")
	op.Type = "ScalarValues::Real"
	result, _, ok, err := ws.ApplyEdit("car.sysml", []edit.Operation{op})
	if !ok || err != nil {
		t.Fatalf("ok %v, err %v", ok, err)
	}
	want := "part def Car {\n    attribute mass : ScalarValues::Real;\n    attribute speed : ScalarValues::Real;\n}\n"
	if string(result.Documents[0].Content) != want {
		t.Fatalf("content:\n%s\nwant:\n%s", result.Documents[0].Content, want)
	}
}

// An edit inside a version is judged with the version in the bundled file's
// place, as the workspace holds it: the file it displaced does not come back to
// declare the library's names a second time and refuse the edit.
func TestWorkspaceLibraryVersionEditsInsideVersion(t *testing.T) {
	ws := NewWorkspace()
	lib := ws.LibraryDocument(scalarValues)
	if lib == nil {
		t.Fatalf("%s not bundled", scalarValues)
	}
	ws.Open("copy.kerml", lib.Content, 1)
	ws.Open("car.sysml", []byte("part def Car {\n    attribute mass : ScalarValues::Real;\n}\n"), 1)
	result, _, ok, err := ws.ApplyEdit("copy.kerml", []edit.Operation{edit.Rename("ScalarValues::Real", "Reel")})
	if !ok || err != nil {
		t.Fatalf("ok %v, err %v", ok, err)
	}
	edited := map[string]string{}
	for _, doc := range result.Documents {
		edited[doc.Name] = string(doc.Content)
	}
	if !strings.Contains(edited["copy.kerml"], "datatype Reel specializes") {
		t.Errorf("the version's Real is not renamed:\n%s", edited["copy.kerml"])
	}
	if want := "attribute mass : ScalarValues::Reel;"; !strings.Contains(edited["car.sysml"], want) {
		t.Errorf("the reference beside the version is not renamed:\n%s", edited["car.sysml"])
	}
}

// The index an edit is judged in follows the edited roots as the workspace
// does: a version still rooted at the library's package displaces the bundled
// file and carries its tier; one that has moved off it is the user's own
// beside the bundled file, which declares the library's names again.
func TestWorkspaceLibraryVersionEditIndexFollowsRoots(t *testing.T) {
	ws := NewWorkspace()
	lib := ws.LibraryDocument(scalarValues)
	if lib == nil {
		t.Fatalf("%s not bundled", scalarValues)
	}
	src := string(lib.Content)
	ws.Open("copy.kerml", lib.Content, 1)
	for _, tc := range []struct {
		name, text string
		library    bool
		real       []string // documents declaring ScalarValues::Real, sorted
	}{
		{"member added", strings.Replace(src, "datatype Real specializes", "datatype Furlong;\n\tdatatype Real specializes", 1),
			true, []string{"copy.kerml"}},
		{"root renamed", strings.Replace(src, "standard library package ScalarValues", "standard library package MyValues", 1),
			false, []string{scalarValues}},
		{"keywords dropped", strings.Replace(src, "standard library package ScalarValues", "package ScalarValues", 1),
			false, []string{scalarValues, "copy.kerml"}},
	} {
		if tc.text == src {
			t.Fatalf("%s: the text is not declared where expected", tc.name)
		}
		edited := newDocument("copy.kerml", []byte(tc.text), 2)
		ws.mu.Lock()
		ei := ws.editIndexLocked("copy.kerml")
		idx := ei.build()
		idx.AddDocumentWithKind("copy.kerml", edited.AST, edited.sf.Kind())
		ei.indexed(idx, edited.sf, edited.AST)
		ws.mu.Unlock()
		if got := idx.IsLibraryDocument("copy.kerml"); got != tc.library {
			t.Errorf("%s: the edited copy is a library document: %v, want %v", tc.name, got, tc.library)
		}
		if got := idx.DocumentRoot(scalarValues) != nil; got == tc.library {
			t.Errorf("%s: the bundled file is indexed: %v, want %v", tc.name, got, !tc.library)
		}
		var real []string
		for _, sym := range idx.LookupQualified("ScalarValues::Real") {
			real = append(real, sym.DocName)
		}
		sort.Strings(real)
		if !slices.Equal(real, tc.real) {
			t.Errorf("%s: ScalarValues::Real declared in %q, want %q", tc.name, real, tc.real)
		}
	}
}

// One edit's index sees each rewrite in turn: a copy that a first operation
// roots at the library's package displaces the bundled file, and a second that
// moves it off again brings the file back, whatever the workspace holds.
func TestWorkspaceLibraryVersionEditIndexFollowsSequence(t *testing.T) {
	ws := NewWorkspace()
	lib := ws.LibraryDocument(scalarValues)
	if lib == nil {
		t.Fatalf("%s not bundled", scalarValues)
	}
	src := string(lib.Content)
	rooted := func(name string) *Document {
		text := strings.Replace(src, "standard library package ScalarValues", "standard library package "+name, 1)
		if text == src && name != "ScalarValues" {
			t.Fatalf("the root is not declared where expected")
		}
		return newDocument("copy.kerml", []byte(text), 2)
	}
	ws.Open("copy.kerml", rooted("Mine").Content, 1)
	if got := ws.StandsInFor("copy.kerml"); got != "" {
		t.Fatalf("the copy rooted at Mine stands in for %q", got)
	}
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ei := ws.editIndexLocked("copy.kerml")
	idx := ei.build()
	for i, step := range []struct {
		doc     *Document
		library bool
		real    []string
	}{
		{rooted("ScalarValues"), true, []string{"copy.kerml"}},
		{rooted("MineAgain"), false, []string{scalarValues}},
		{rooted("ScalarValues"), true, []string{"copy.kerml"}},
	} {
		idx.AddDocumentWithKind("copy.kerml", step.doc.AST, step.doc.sf.Kind())
		ei.indexed(idx, step.doc.sf, step.doc.AST)
		if got := idx.IsLibraryDocument("copy.kerml"); got != step.library {
			t.Errorf("step %d: the copy is a library document: %v, want %v", i, got, step.library)
		}
		if got := idx.DocumentRoot(scalarValues) != nil; got == step.library {
			t.Errorf("step %d: the bundled file is indexed: %v, want %v", i, got, !step.library)
		}
		var real []string
		for _, sym := range idx.LookupQualified("ScalarValues::Real") {
			real = append(real, sym.DocName)
		}
		sort.Strings(real)
		if !slices.Equal(real, step.real) {
			t.Errorf("step %d: ScalarValues::Real declared in %q, want %q", i, real, step.real)
		}
	}
	if got := ws.standIns["copy.kerml"]; got != "" {
		t.Errorf("the edit's index changed what the workspace's copy stands in for: %q", got)
	}
}

// A document rooted inside a library, or at a user package beside a library
// one, is the workspace's own however its text reads.
func TestWorkspaceLibraryVersionNeedsLibraryRoots(t *testing.T) {
	ws := NewWorkspace()
	lib := ws.LibraryDocument(scalarValues)
	if lib == nil {
		t.Fatalf("%s not bundled", scalarValues)
	}
	for _, tc := range []struct{ name, text string }{
		{"nested element", "datatype Real;"},
		{"library and own root", string(lib.Content) + "\npackage Mine;\n"},
		{"two libraries", string(lib.Content) + "\nstandard library package Base;\n"},
	} {
		ws.Open("own.kerml", []byte(tc.text), 1)
		if got := ws.StandsInFor("own.kerml"); got != "" {
			t.Errorf("%s: StandsInFor = %q, want nothing", tc.name, got)
		}
		if !ws.IsLibraryDocument(scalarValues) {
			t.Errorf("%s: the bundled file was displaced", tc.name)
		}
		if root := ws.index.DocumentRoot("own.kerml"); root != nil {
			for _, sym := range root.Members() {
				if info, ok := ws.IdentityOf("own.kerml", sym); ok && info.Source == identity.SourceNormative {
					t.Errorf("%s: %s carries a normative id", tc.name, sym.Name)
				}
			}
		}
		ws.Remove("own.kerml")
	}
}

// A library the caller indexed and marked has the same lifecycle as the bundled
// one: a workspace document under a library file's name displaces it and closing
// puts it back under its mark, and a version rooted at its package stands in.
func TestWorkspaceLibraryVersionOverCallerBuiltIndex(t *testing.T) {
	const tanks = "lib/tanks.sysml"
	text := []byte("standard library package Tanks {\n    part def Tank;\n}\n")
	src := &scratchSource{files: map[string]string{tanks: string(text)}}
	idx := symbols.NewIndex()
	idx.AddDocumentWithKind(tanks, parser.New(source.New(tanks, text)).ParseFile(), source.KindSysML)
	idx.MarkLibraryDocument(tanks, symbols.LibraryDocument{Tier: symbols.TierSystems, Digest: symbols.TextDigest(text)})
	idx.ExpandWildcardImports()
	ws := NewWorkspaceWithIndex(idx, WithLibrarySource(src))
	tankIn := func(when string, want string) {
		t.Helper()
		syms := ws.LookupQualified("Tanks::Tank")
		if len(syms) != 1 || syms[0].DocName != want {
			var docs []string
			for _, s := range syms {
				docs = append(docs, s.DocName)
			}
			t.Errorf("%s: Tanks::Tank declared in %q, want %q", when, docs, want)
		}
	}
	tankIn("before anything opens", tanks)

	ws.Open(tanks, []byte("package Mine { part def Tank; }"), 1)
	if ws.IsLibraryDocument(tanks) || ws.index.IsLibraryDocument(tanks) {
		t.Error("a workspace document under the library file's name is the workspace's own")
	}
	if syms := ws.LookupQualified("Tanks::Tank"); len(syms) != 0 {
		t.Errorf("Tanks::Tank = %d symbols while user text holds the name, want 0", len(syms))
	}
	ws.Close(tanks)
	tankIn("after closing the document under its name", tanks)
	if !ws.IsLibraryDocument(tanks) || !ws.index.IsLibraryDocument(tanks) {
		t.Error("the library file did not come back under its mark")
	}
	if lib := ws.LibraryDocument(tanks); lib == nil || string(lib.Content) != string(text) {
		t.Error("the restored library file is not served from the library source")
	}

	ws.Open("copy.sysml", text, 1)
	if got := ws.StandsInFor("copy.sysml"); got != tanks {
		t.Fatalf("StandsInFor = %q, want %q", got, tanks)
	}
	tankIn("with a version open", "copy.sysml")
	if !ws.index.IsLibraryDocument("copy.sysml") || ws.index.LibraryDocumentOf("copy.sysml").Tier != symbols.TierSystems {
		t.Error("the version does not carry the library file's tier")
	}
	ws.Open("car.sysml", []byte("part def Car {\n    part tank : Tanks::Tank;\n}\n"), 1)
	for _, d := range ws.Diagnostics("car.sysml") {
		if d.Code == "unresolved" {
			t.Errorf("car.sysml beside the version: %s: %s", d.Code, d.Message)
		}
	}
	op := edit.AddMember("Car", "part", "spare")
	op.Type = "Tanks::Tank"
	if _, _, ok, err := ws.ApplyEdit("car.sysml", []edit.Operation{op}); !ok || err != nil {
		t.Errorf("an edit beside the version: ok %v, err %v", ok, err)
	}
	ws.Update("copy.sysml", []byte("package Other { part def Tank; }"), 2)
	if got := ws.StandsInFor("copy.sysml"); got != "" {
		t.Errorf("moved off the library's root, still stands in for %q", got)
	}
	tankIn("after the version moved off the library's root", tanks)
	ws.Update("copy.sysml", text, 3)
	tankIn("after the version returned", "copy.sysml")
	ws.Remove("copy.sysml")
	tankIn("after the version left", tanks)
	if !ws.IsLibraryDocument(tanks) {
		t.Error("the library file did not come back once the version left")
	}
}

// A file MarkLibrary marked at the generic tier has no normative language, so
// its elements carry no ids; a copy rooted at its package is still its version.
func TestWorkspaceLibraryVersionOfGenericTierFile(t *testing.T) {
	const tanks = "tanks.sysml"
	text := []byte("standard library package Tanks {\n    part def Tank;\n}\n")
	src := &scratchSource{files: map[string]string{tanks: string(text)}}
	idx := symbols.NewIndex()
	idx.AddDocumentWithKind(tanks, parser.New(source.New(tanks, text)).ParseFile(), source.KindSysML)
	idx.MarkLibrary(tanks)
	idx.ExpandWildcardImports()
	ws := NewWorkspaceWithIndex(idx, WithLibrarySource(src))
	if _, ok := identity.LibraryCatalog(ws.index).ElementNamed("Tanks"); ok {
		t.Fatal("a generic-tier library file's package was given a normative id")
	}

	ws.Open("copy.sysml", text, 1)
	if got := ws.StandsInFor("copy.sysml"); got != tanks {
		t.Fatalf("StandsInFor = %q, want %q", got, tanks)
	}
	syms := ws.LookupQualified("Tanks::Tank")
	if len(syms) != 1 || syms[0].DocName != "copy.sysml" {
		t.Fatalf("Tanks::Tank = %v, want the one copy.sysml declares", syms)
	}
	if ws.index.LibraryDocumentOf("copy.sysml").Tier != symbols.TierLibrary {
		t.Error("the version does not carry the generic tier")
	}
	ws.Close("copy.sysml")
	if syms := ws.LookupQualified("Tanks::Tank"); len(syms) != 1 || syms[0].DocName != tanks {
		t.Errorf("after the version closed: Tanks::Tank = %v, want the one %s declares", syms, tanks)
	}
	if !ws.IsLibraryDocument(tanks) {
		t.Error("the generic-tier file did not come back under its mark")
	}
}

// A caller's overlay may hold documents the frozen base does not, unmarked or
// marked as a library; an edit's temporary index keeps both, so the edited
// document still resolves what it did.
func TestWorkspaceLibraryVersionEditIndexKeepsOverlayDocuments(t *testing.T) {
	const hulls, tanks = "lib/hulls.sysml", "lib/tanks.sysml"
	hullText := []byte("package Hulls {\n    part def Hull;\n}\n")
	tankText := []byte("standard library package Tanks {\n    part def Tank;\n}\n")
	base := symbols.NewIndex()
	base.AddDocumentWithKind("lib/base.sysml", parser.New(source.New("lib/base.sysml", []byte("package Base;\n"))).ParseFile(), source.KindSysML)
	base.MarkLibrary("lib/base.sysml")
	base.Freeze()
	idx := symbols.NewOverlay(base)
	idx.AddDocumentWithKind(hulls, parser.New(source.New(hulls, hullText)).ParseFile(), source.KindSysML)
	idx.AddDocumentWithKind(tanks, parser.New(source.New(tanks, tankText)).ParseFile(), source.KindSysML)
	idx.MarkLibraryDocument(tanks, symbols.LibraryDocument{Tier: symbols.TierSystems, Digest: symbols.TextDigest(tankText)})
	idx.ExpandWildcardImports()
	ws := NewWorkspaceWithIndex(idx)

	ws.Open("boat.sysml", []byte("part def Boat {\n    part hull : Hulls::Hull;\n    part tank : Tanks::Tank;\n}\n"), 1)
	op := edit.AddMember("Boat", "part", "spare")
	op.Type = "Tanks::Tank"
	result, _, ok, err := ws.ApplyEdit("boat.sysml", []edit.Operation{op})
	if !ok || err != nil {
		t.Fatalf("ApplyEdit beside the overlay's documents: ok %v, err %v", ok, err)
	}
	if len(result.Documents) != 1 || !strings.Contains(string(result.Documents[0].Content), "spare : Tanks::Tank") {
		t.Fatalf("ApplyEdit result = %+v, want the edited boat.sysml alone", result.Documents)
	}
	edited := ws.editIndexLocked("boat.sysml").build()
	for _, name := range []string{"lib/base.sysml", hulls, tanks} {
		if edited.DocumentRoot(name) == nil {
			t.Errorf("the edit index dropped %s", name)
		}
	}
	if edited.DocumentKind(hulls) != source.KindSysML || edited.IsLibraryDocument(hulls) {
		t.Error("the unmarked overlay document lost its kind or gained a mark")
	}
	if got := edited.LibraryDocumentOf(tanks); got.Tier != symbols.TierSystems || got.Digest != symbols.TextDigest(tankText) {
		t.Errorf("the marked overlay document's mark = %+v in the edit index", got)
	}
	if syms := edited.LookupQualified("Hulls::Hull"); len(syms) != 1 || syms[0].DocName != hulls {
		t.Errorf("Hulls::Hull in the edit index = %v, want the one %s declares", syms, hulls)
	}
}
