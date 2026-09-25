package xpect

import (
	"strings"
	"testing"
)

const exported = `//*
XPECT_SETUP a.B
	ResourceSet {
		ThisFile {}
		File {from ="/library/Base.kerml"}
	}
END_SETUP
*/
//*
XPECT exportedObjects ---
sysml::Feature: NameEscape::..::parameter
sysml::Function: NameEscape::..
sysml::Package: NameEscape
--- */

package NameEscape {
	function '..' {
		in parameter: Base::Anything;
		return : Base::Anything;
	}
}
`

// An exportedObjects note declares one object per line, and adjudicates against
// what the document contributes to the index: a named element each, an unnamed
// one none.
func TestW8FExportedObjectsAgree(t *testing.T) {
	dir := writeSuite(t, map[string]string{"a/e.kerml.xt": exported, "library/Base.kerml": baseLib})
	res := compareOne(dir, "a/e.kerml.xt", newLibraryCache())
	if len(res.Problems) != 0 {
		t.Fatalf("problems = %v", res.Problems)
	}
	for _, r := range res.Rows {
		if r.Kind != kindExportedObjects {
			continue
		}
		if r.Verdict != verdictAgree {
			t.Fatalf("%s: %s (%s)", r.Verdict, r.Actual, r.Declared)
		}
		return
	}
	t.Fatal("no exportedObjects row")
}

// A declared object we do not export is a disagreement, counted per half.
func TestW8FExportedObjectsDisagree(t *testing.T) {
	fixture := strings.Replace(exported, "sysml::Package: NameEscape", "sysml::Package: NameEscape\nsysml::Package: Absent", 1)
	dir := writeSuite(t, map[string]string{"a/e.kerml.xt": fixture, "library/Base.kerml": baseLib})
	res := compareOne(dir, "a/e.kerml.xt", newLibraryCache())
	for _, r := range res.Rows {
		if r.Kind != kindExportedObjects {
			continue
		}
		if r.Verdict != verdictDisagree || r.Names == nil || r.Names.Missing != 1 {
			t.Fatalf("%s: %s names=%+v", r.Verdict, r.Actual, r.Names)
		}
		return
	}
	t.Fatal("no exportedObjects row")
}
