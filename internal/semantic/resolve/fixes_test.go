package resolve

import (
	"testing"
)

// fixTitles is the titles of the fixes carried by the diagnostics of src.
func fixTitles(t *testing.T, src string) []string {
	t.Helper()
	r := resolveDoc(t, "d.sysml", src)
	var out []string
	for _, d := range r.Diagnostics {
		for _, fix := range d.Fixes {
			out = append(out, fix.Title)
		}
	}
	return out
}

// A near-miss name in scope is offered as the spelling that was meant, with the
// edit replacing the name written.
func TestUnresolvedNameCarriesSpellingFix(t *testing.T) {
	const src = "package P { part def Wheel; part w : Wheeel; }"
	r := resolveDoc(t, "d.sysml", src)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %v, want one", r.Diagnostics)
	}
	d := r.Diagnostics[0]
	if len(d.Fixes) != 1 {
		t.Fatalf("fixes = %+v, want one", d.Fixes)
	}
	fix := d.Fixes[0]
	if fix.Title != "Change 'Wheeel' to 'Wheel'" || !fix.Preferred {
		t.Errorf("fix = %+v", fix)
	}
	if len(fix.Edits) != 1 {
		t.Fatalf("edits = %+v, want one", fix.Edits)
	}
	if got := src[fix.Edits[0].Span.Offset:fix.Edits[0].Span.End()]; got != "Wheeel" {
		t.Errorf("the edit replaces %q, want the name written", got)
	}
	if fix.Edits[0].NewText != "Wheel" {
		t.Errorf("the edit writes %q", fix.Edits[0].NewText)
	}
}

// A name declared elsewhere under a namespace the user can import is offered
// both spellings: qualified, or imported so it resolves as written.
func TestUnresolvedNameCarriesImportFix(t *testing.T) {
	titles := fixTitles(t, "package Lib { part def Wheel; }\npackage P { part w : Wheel; }")
	want := map[string]bool{"Change 'Wheel' to 'Lib::Wheel'": false, "Import 'Lib::*'": false}
	for _, title := range titles {
		if _, ok := want[title]; ok {
			want[title] = true
		}
	}
	for title, seen := range want {
		if !seen {
			t.Errorf("no fix %q in %v", title, titles)
		}
	}
}

// The import a fix writes is private, so applying it does not re-export the
// imported names onward ([SysML, 7.2] over [KerML, 8.2.3.3]).
func TestTheImportFixWritesAPrivateImport(t *testing.T) {
	r := resolveDoc(t, "d.sysml", "package Lib { part def Wheel; }\npackage P { part w : Wheel; }")
	for _, d := range r.Diagnostics {
		for _, fix := range d.Fixes {
			if fix.Title != "Import 'Lib::*'" {
				continue
			}
			if len(fix.Edits) != 1 || fix.Edits[0].NewText != "private import Lib::*;" {
				t.Errorf("the import fix writes %+v", fix.Edits)
			}
			return
		}
	}
	t.Fatal("no import fix")
}

// A name nothing in the workspace resembles carries no fix rather than a guess.
func TestUnresolvedNameWithoutCandidatesCarriesNoFix(t *testing.T) {
	if titles := fixTitles(t, "package P { part w : Zzzqqqxyw; }"); len(titles) != 0 {
		t.Errorf("fixes = %v, want none", titles)
	}
}

// More than one candidate leaves none preferred: the editor must not apply one
// of them silently.
func TestSeveralCandidatesArePreferredNone(t *testing.T) {
	r := resolveDoc(t, "d.sysml", "package P { part def Wheelx; part def Wheely; part w : Wheelz; }")
	var fixes int
	for _, d := range r.Diagnostics {
		for _, fix := range d.Fixes {
			fixes++
			if fix.Preferred {
				t.Errorf("fix %q is preferred while other candidates exist", fix.Title)
			}
		}
	}
	if fixes < 2 {
		t.Errorf("fixes = %d, want one per candidate spelling", fixes)
	}
}
