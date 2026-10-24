package main

import (
	"strings"
	"testing"
)

// Two loaded files declaring the same library package are reported once, as a
// warning on the copy references do not reach, naming the one they do; the
// check still runs, since a warning gates nothing.
func TestValidateReportsALibraryDeclaredTwice(t *testing.T) {
	binary := buildCLI(t)
	files := map[string]string{
		"a.sysml": "library package Foo { part def A; }\n",
		"b.sysml": "library package Foo { part def A; }\n",
		"m.sysml": "package M { part x : Foo::A; }\n",
	}
	got := checkFiles(t, binary, files, "-validate")
	wantReport(t, got, 0, "b.sysml:1:17: warning[library-declared-twice]: library package Foo is declared both here and in ",
		"a.sysml; references resolve to the declaration in ", "and this one is unused", "no errors")
	if n := strings.Count(got.output(), "library-declared-twice"); n != 1 {
		t.Errorf("the warning is reported %d times, want once:\n%s", n, got.output())
	}
	wantReport(t, checkFiles(t, binary, files, "-warnings-as-errors", "-validate"), 2, "warning[library-declared-twice]")
	wantReport(t, checkFiles(t, binary, files, "-validate", "-json"), 0, `"code": "library-declared-twice"`, `"pass": "workspace"`)
}

// One declaration of a user library, and the bundled library alone, are no duplication.
func TestValidateIsQuietAboutALibraryDeclaredOnce(t *testing.T) {
	binary := buildCLI(t)
	got := checkFiles(t, binary, map[string]string{
		"lib.sysml": "library package Foo { part def A; }\n",
		"m.sysml":   "package M { private import ScalarValues::*; part x : Foo::A; attribute r : Real; }\n",
	}, "-validate")
	wantReport(t, got, 0, "no errors")
	if strings.Contains(got.output(), "library-declared-twice") {
		t.Errorf("a library declared once was reported:\n%s", got.output())
	}
}
