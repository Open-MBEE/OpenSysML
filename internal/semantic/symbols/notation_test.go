package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func notationOf(t *testing.T, file, src, pkg, name string) string {
	t.Helper()
	root := Build(parser.New(source.New(file, []byte(src))).ParseFile())
	p, ok := root.LookupLocal(pkg)
	if !ok {
		t.Fatalf("package %s not indexed", pkg)
	}
	sym, ok := p.Scope.LookupLocal(name)
	if !ok {
		t.Fatalf("%s not indexed", name)
	}
	return sym.Notation()
}

// A declaration is named by the keywords it was written with, not by the kind
// several spellings share.
func TestNotationKeepsTheWrittenKeywords(t *testing.T) {
	sysml := "package P { part def Wheel; attribute def A; part w : Wheel; state s; " +
		"assert constraint c { true } perform action a; }"
	for name, want := range map[string]string{
		"Wheel": "part def", "A": "attribute def", "w": "part", "s": "state",
		"c": "assert constraint", "a": "perform action",
	} {
		if got := notationOf(t, "p.sysml", sysml, "P", name); got != want {
			t.Errorf("notation of %s = %q, want %q", name, got, want)
		}
	}

	// A KerML classifier is written without the `def` a SysML definition takes,
	// and `datatype`/`feature` are spellings of kinds SysML spells otherwise.
	kerml := "package K { class C; classifier D; struct S; datatype T; feature f : C; " +
		"metaclass M; behavior def BD; specialization Spec subtype C :> S; }"
	for name, want := range map[string]string{
		"C": "class", "D": "classifier", "S": "struct", "T": "datatype", "f": "feature",
		"M": "metaclass", "BD": "behavior def", "Spec": "specialization",
	} {
		if got := notationOf(t, "k.kerml", kerml, "K", name); got != want {
			t.Errorf("notation of %s = %q, want %q", name, got, want)
		}
	}
}
