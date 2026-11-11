package symbols

import "testing"

func wantNamed(t *testing.T, idx *Index, fqn string, want int) {
	t.Helper()
	if got := len(idx.LookupQualified(fqn)); got != want {
		t.Errorf("LookupQualified(%s) = %d symbols, want %d", fqn, got, want)
	}
}

// An import takes its target's public memberships, an `import all` every one,
// and what either took has the import's visibility onward (KerML 8.3.2.4.5).
func TestExpandWildcardImportsTakesPrivateMembersOnlyUnderImportAll(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "base.sysml", "package Base { private part def Hidden; part def Shown; }")
	addDoc(t, idx, "mid.sysml", "package Mid { private import Base::*; private part def Own; }")
	addDoc(t, idx, "all.sysml", "package All { public import all Mid::*; }")
	addDoc(t, idx, "plain.sysml", "package Plain { public import Mid::*; }")
	addDoc(t, idx, "onward.sysml", "package Onward { public import All::*; }")
	idx.ExpandWildcardImports()

	wantNamed(t, idx, "All::Own", 1)
	wantNamed(t, idx, "All::Shown", 1)
	wantNamed(t, idx, "All::Hidden", 0)
	wantNamed(t, idx, "Plain::Own", 0)
	wantNamed(t, idx, "Plain::Shown", 0)
	wantNamed(t, idx, "Onward::Own", 1)
	wantNamed(t, idx, "Onward::Shown", 1)
	wantNamed(t, idx, "Onward::Hidden", 0)
}

// A member whose visibility an edit changed is re-derived in its importers.
func TestExpandWildcardImportsFollowsAVisibilityEdit(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "lib.sysml", "package Lib { private part def P; }")
	addDoc(t, idx, "app.sysml", "package App { public import Lib::*; }")
	idx.ExpandWildcardImports()
	wantNamed(t, idx, "App::P", 0)

	addDoc(t, idx, "lib.sysml", "package Lib { public part def P; }")
	idx.ExpandWildcardImports()
	wantNamed(t, idx, "App::P", 1)

	addDoc(t, idx, "lib.sysml", "package Lib { private part def P; }")
	idx.ExpandWildcardImports()
	wantNamed(t, idx, "App::P", 0)
}

// An import edited between ordinary and `import all` re-derives its importer.
func TestExpandWildcardImportsFollowsAnImportAllEdit(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "lib.sysml", "package Lib { private part def P; }")
	addDoc(t, idx, "app.sysml", "package App { public import Lib::*; }")
	idx.ExpandWildcardImports()
	wantNamed(t, idx, "App::P", 0)

	addDoc(t, idx, "app.sysml", "package App { public import all Lib::*; }")
	idx.ExpandWildcardImports()
	wantNamed(t, idx, "App::P", 1)

	addDoc(t, idx, "app.sysml", "package App { public import Lib::*; }")
	idx.ExpandWildcardImports()
	wantNamed(t, idx, "App::P", 0)
}

// The snapshot stream carries an import's `all`, with what it re-exported.
func TestSnapshotKeepsImportAll(t *testing.T) {
	restored := roundTrip(t, frozenBase(t, map[string]string{
		"lib.sysml": "package Lib { private part def P; }",
		"app.sysml": "package App { public import all Lib::*; }",
	}))
	if got := restored.WildcardImportsOf("App"); len(got) != 1 || !got[0].All {
		t.Fatalf("restored WildcardImportsOf(App) = %+v, want one `import all`", got)
	}
	wantNamed(t, restored, "App::P", 1)
}
