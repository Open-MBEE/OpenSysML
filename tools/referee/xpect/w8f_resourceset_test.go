package xpect

import (
	"strings"
	"testing"
)

// setupOf wraps a resource list in an XPECT_SETUP header.
func setupOf(resources, body string) string {
	return "//*\nXPECT_SETUP a.B\n\tResourceSet {\n\t\tThisFile {}\n" + resources + "\t}\nEND_SETUP\n*/\n" + body
}

// A fixture declaring nothing but itself is loaded alone: a library name it
// does not ask for is not there.
func TestW8FResourceSetDeclaringNothing(t *testing.T) {
	fixture := setupOf("", "package test {\n\tclassifier A specializes Base::Anything;\n}\n")
	dir := writeSuite(t, map[string]string{
		"a/only.kerml.xt":    fixture,
		"library/Base.kerml": baseLib,
	})
	set := loadResourceSet(dir, parseXT("a/only.kerml.xt", "kerml", []byte(fixture)), newLibraryCache())
	if len(set.libraryRoots) != 0 || len(set.missing) != 0 {
		t.Fatalf("libraryRoots = %v, missing = %v", set.libraryRoots, set.missing)
	}
	if got := errorsOf(t, set, "a/only.kerml"); !strings.Contains(got, "Base") {
		t.Errorf("an undeclared library must not resolve; errors = %q", got)
	}
}

// A fixture declaring another source file sees that file's declarations, and
// only that file's: a sibling it does not declare stays out.
func TestW8FResourceSetDeclaringAnotherFile(t *testing.T) {
	fixture := setupOf("\t\tFile {from =\"/src/Other.kerml\"}\n",
		"package test {\n\tclassifier A specializes Other::B;\n\tclassifier C specializes Third::D;\n}\n")
	dir := writeSuite(t, map[string]string{
		"a/main.kerml.xt":    fixture,
		"src/Other.kerml":    "package Other {\n\tclassifier B;\n}\n",
		"src/Third.kerml":    "package Third {\n\tclassifier D;\n}\n",
		"library/Base.kerml": baseLib,
	})
	set := loadResourceSet(dir, parseXT("a/main.kerml.xt", "kerml", []byte(fixture)), newLibraryCache())
	if len(set.missing) != 0 {
		t.Fatalf("missing = %v", set.missing)
	}
	errs := errorsOf(t, set, "a/main.kerml")
	if strings.Contains(errs, "Other::B") {
		t.Errorf("a declared source must resolve; errors = %q", errs)
	}
	if !strings.Contains(errs, "Third::D") {
		t.Errorf("an undeclared sibling must not resolve; errors = %q", errs)
	}
}

// A fixture declaring library files loads those copies, marked as library
// content so their roots gate the implicit members, and no others.
func TestW8FResourceSetDeclaringLibraryFiles(t *testing.T) {
	fixture := setupOf("\t\tFile {from =\"/library/Base.kerml\"}\n",
		"package test {\n\tclassifier A specializes Base::Anything;\n\tclassifier C specializes Links::Link;\n}\n")
	dir := writeSuite(t, map[string]string{
		"a/lib.kerml.xt":      fixture,
		"library/Base.kerml":  baseLib,
		"library/Links.kerml": "standard library package Links {\n\tabstract classifier Link;\n}\n",
	})
	set := loadResourceSet(dir, parseXT("a/lib.kerml.xt", "kerml", []byte(fixture)), newLibraryCache())
	if len(set.libraryRoots) != 1 || set.libraryRoots[0] != "Base" {
		t.Fatalf("libraryRoots = %v, want [Base]", set.libraryRoots)
	}
	errs := errorsOf(t, set, "a/lib.kerml")
	if strings.Contains(errs, "Base::Anything") {
		t.Errorf("a declared library must resolve; errors = %q", errs)
	}
	if !strings.Contains(errs, "Links::Link") {
		t.Errorf("an undeclared library file must not resolve; errors = %q", errs)
	}
}

// A declared resource the download does not hold is reported, never silently
// treated as loaded: a missing resource must not read as agreement.
func TestW8FResourceSetMissingResourceIsReported(t *testing.T) {
	fixture := setupOf("\t\tFile {from =\"/library/Absent.kerml\"}\n", "package test {\n\tclassifier A;\n}\n")
	dir := writeSuite(t, map[string]string{"a/gone.kerml.xt": fixture})
	res := compareOne(dir, "a/gone.kerml.xt", newLibraryCache())
	if len(res.Missing) != 1 || res.Missing[0] != "/library/Absent.kerml" {
		t.Fatalf("missing = %v, want [/library/Absent.kerml]", res.Missing)
	}
}

// A resource path without a leading slash is resolved beside the .xt file.
func TestW8FResourceSetRelativePath(t *testing.T) {
	if got := resourcePath("a/b/main.kerml.xt", "Other.kerml"); got != "a/b/Other.kerml" {
		t.Errorf("resourcePath(relative) = %q", got)
	}
	if got := resourcePath("a/b/main.kerml.xt", "/src/Other.kerml"); got != "src/Other.kerml" {
		t.Errorf("resourcePath(absolute) = %q", got)
	}
}

// errorsOf is the joined error text of a document in a loaded resource set.
func errorsOf(t *testing.T, set resourceSet, doc string) string {
	t.Helper()
	var msgs []string
	for _, d := range set.ws.Diagnostics(doc) {
		msgs = append(msgs, d.Message)
	}
	return strings.Join(msgs, "; ")
}
