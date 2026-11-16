package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// No SysML production declares a namespace (SysML 8.2.2.5.1, 8.2.2.5.2), so the
// finding does not follow the mode: one error, the same in both, silent in KerML.
func TestNamespaceInSysMLIsAnErrorInEitherMode(t *testing.T) {
	for _, src := range []string{
		"namespace N { part def Q; }",
		"namespace N;",
		"package P { namespace N; }",
		"package P { part def D { namespace N { part def Q; } } }",
		"package P { metadata def M; #M namespace N; }",
		"package P { private namespace N; }",
	} {
		def := notationDiags(t, "a.sysml", src, diag.ConformanceDefault)
		strict := notationDiags(t, "a.sysml", src, diag.ConformanceStrict)
		if len(def) != 1 || def[0].Severity != diag.SeverityError || def[0].Code != CodeKerMLNotation {
			t.Errorf("%s: default: got %+v, want one kerml-notation error", src, def)
			continue
		}
		if len(strict) != 1 || strict[0].Severity != diag.SeverityError || strict[0].Span != def[0].Span {
			t.Errorf("%s: strict: got %+v, want the default finding %+v", src, strict, def[0])
		}
	}
	for _, src := range []string{"namespace N { class C; }", "namespace N;", "package P { private namespace N; }"} {
		if got := notationDiags(t, "a.kerml", src, diag.ConformanceStrict); len(got) != 0 {
			t.Errorf("%s: a KerML file declares namespaces: got %+v, want silence", src, got)
		}
	}
}

// The error points at the keyword, past a visibility, prefix metadata or a
// comment that spells it.
func TestNamespaceErrorSpansTheKeyword(t *testing.T) {
	for _, src := range []string{
		"namespace N;",
		"package P { private namespace N; }",
		"package P { metadata def M; #M namespace N { part def Q; } }",
		"package P { private /* namespace */ namespace N; }",
	} {
		_, _, all := notationDiagnostics(t, "a.sysml", src)
		want := source.Span{Offset: strings.LastIndex(src, "namespace"), Len: len("namespace")}
		if len(all) != 1 || all[0].Code != CodeKerMLNotation || all[0].Span != want {
			t.Errorf("%s: got %+v, want one kerml-notation error at %+v", src, all, want)
		}
	}
}

// Each nested declaration is one more error, and a namespace inside a KerML
// declaration that is itself an error is still reported.
func TestNamespaceIsRejectedWhereverItIsNested(t *testing.T) {
	got := notationDiags(t, "a.sysml", "namespace A { namespace B; }", diag.ConformanceDefault)
	if len(got) != 2 || got[0].Severity != diag.SeverityError || got[1].Severity != diag.SeverityError {
		t.Errorf("nested namespaces: got %+v, want two errors", got)
	}
	got = notationDiags(t, "a.sysml", "package P { class C { namespace N; } }", diag.ConformanceDefault)
	if len(got) != 2 || got[0].Severity != diag.SeverityError || got[1].Severity != diag.SeverityError {
		t.Fatalf("namespace in a class: got %+v, want the class error and the namespace error", got)
	}
}

// `namespace` is no reserved word of SysML (8.2.2.1.2), so where no name
// follows it is the name of a keywordless usage, which is legal SysML.
func TestNamespaceAsAUsageNameIsSilent(t *testing.T) {
	for _, src := range []string{
		"package P { namespace { } }",
		"package P { namespace; }",
		"package P { part def T; namespace : T; }",
	} {
		if got := notationDiags(t, "a.sysml", src, diag.ConformanceStrict); len(got) != 0 {
			t.Errorf("%s: got %+v, want no finding", src, got)
		}
	}
}

// The error describes the writing, so it gates no higher tier: the members of
// the rejected namespace are still resolved and reported.
func TestNamespaceErrorDoesNotGateNameResolution(t *testing.T) {
	root, pd, idx := analyzeInputs(t, "a.sysml", "namespace N { part x : NoSuchType; }")
	var notation, unresolved bool
	for _, d := range Analyze("a.sysml", root, pd, idx) {
		switch d.Code {
		case CodeKerMLNotation:
			notation = d.Severity == diag.SeverityError && d.Notation && !d.Blocking()
		case "unresolved":
			unresolved = true
		}
	}
	if !notation || !unresolved {
		t.Fatalf("notation error = %v, unresolved reference reported = %v, want both", notation, unresolved)
	}
}
