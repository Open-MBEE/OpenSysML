package resolve_test

import (
	"strings"
	"testing"
)

// A unit written by its short name without its package imported is diagnosed
// as any unimported library name is: `[km]` without `SI` names the import that
// makes the bare name visible, with a fix writing it.
func TestUnimportedShortNamedUnitNamesTheImport(t *testing.T) {
	src := `package EV {
		public import ScalarValues::*;
		public import ISQ::*;
		attribute targetRange : LengthValue = 400 [km];
	}`
	r := resolveStdlib(t, "d.sysml", src)
	if len(r.Diagnostics) == 0 {
		t.Fatal("no diagnostics")
	}
	first := r.Diagnostics[0].Message
	want := "unresolved reference: km — did you mean SI::km? To use the bare name, import its package: private import SI::*;"
	if first != want {
		t.Fatalf("diagnostic = %q, want %q", first, want)
	}
	imported := false
	for _, fix := range r.Diagnostics[0].Fixes {
		if !strings.Contains(fix.Title, "SI::*") {
			continue
		}
		imported = true
		if len(fix.Edits) != 1 || fix.Edits[0].NewText != "private import SI::*;" {
			t.Errorf("import fix writes %+v", fix.Edits)
		}
	}
	if !imported {
		t.Errorf("no Import 'SI::*' fix in %+v", r.Diagnostics[0].Fixes)
	}
}

// The same holds for a short name the model declares itself.
func TestUnimportedShortNamedDeclarationNamesTheImport(t *testing.T) {
	src := `package P { attribute def <sn> ShortNamed; }
	package Q { attribute a : sn; }`
	r := resolveStdlib(t, "d.sysml", src)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %v, want one", r.Diagnostics)
	}
	want := "unresolved reference: sn — did you mean P::sn? To use the bare name, import its package: private import P::*;"
	if got := r.Diagnostics[0].Message; got != want {
		t.Fatalf("diagnostic = %q, want %q", got, want)
	}
}

// A declaration named as written ranks before one the name only reaches as a
// short name: `min(1, 2)` offers the functions before the minute.
func TestShortNameRanksBehindTheName(t *testing.T) {
	src := `package P { attribute def <sn> ShortNamed; }
	package Q { attribute def sn; }
	package R { attribute a : sn; attribute m = min(1, 2); }`
	r := resolveStdlib(t, "d.sysml", src)
	if len(r.Diagnostics) != 2 {
		t.Fatalf("diagnostics = %v, want two", r.Diagnostics)
	}
	if got, want := r.Diagnostics[0].Message, "unresolved reference: sn — did you mean Q::sn or P::sn?"; !strings.HasPrefix(got, want) {
		t.Errorf("diagnostic = %q, want prefix %q", got, want)
	}
	got := r.Diagnostics[1].Message
	want := "unresolved reference: min — did you mean DataFunctions::min, RealFunctions::min or ScalarFunctions::min?"
	if !strings.HasPrefix(got, want) || strings.Contains(got, "SI::min") {
		t.Errorf("diagnostic = %q, want prefix %q without SI::min", got, want)
	}
}
