package resolve

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// `$::pick(v)` names the root: every top-level declaration of the name, in the
// calling document and the others, is a candidate, not only the first indexed.
func TestInvocationCandidatesAtTheGlobalRoot(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "calc def pick { in x : ScalarValues::Integer; } package App { attribute v = 1; }",
		"b.sysml": "calc def pick { in x : ScalarValues::String; }",
		"c.sysml": "package Other { calc def pick { in x : ScalarValues::Boolean; } }",
	})
	r := New(idx)
	app := scopeOf(t, idx.DocumentRoot("a.sysml"), "App")

	cands := r.InvocationCandidates(app, qn(true, "pick"))
	if len(cands) != 2 {
		t.Fatalf("$::pick has %d candidates, want the two root-level calcs: %v", len(cands), fqnsOf(idx, cands))
	}
	for _, c := range cands {
		if c.Kind != symbols.SymbolCalcDef || idx.GetFQN(c) != "pick" {
			t.Errorf("candidate %s (%v) is not a root-level pick", idx.GetFQN(c), c.Kind)
		}
	}
	if sym, ok := r.ResolveQualified(app, qn(true, "pick")); !ok || sym != cands[0] {
		t.Errorf("the name alone resolves to %v, want the first candidate", sym)
	}
	if first := r.InvocationCandidates(idx.DocumentRoot("b.sysml"), qn(true, "pick")); len(first) != 2 {
		t.Errorf("from b.sysml $::pick has %d candidates, want 2", len(first))
	}
}

// A bare `pick(v)` that only other documents' root declarations answer has each of
// them as a candidate, while a root declaring its own still hides the rest.
func TestUnqualifiedInvocationCandidatesAcrossDocuments(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "calc def pick { in x : ScalarValues::Integer; } package App { attribute v = 1; }",
		"b.sysml": "calc def pick { in x : ScalarValues::String; }",
		"c.sysml": "package Other { calc def pick { in x : ScalarValues::Boolean; } }",
		"d.sysml": "package Caller { attribute v = 1; }",
	})
	r := New(idx)

	caller := scopeOf(t, idx.DocumentRoot("d.sysml"), "Caller")
	cands := r.InvocationCandidates(caller, qn(false, "pick"))
	if len(cands) != 2 {
		t.Fatalf("pick from Caller has %d candidates, want the two root-level calcs: %v", len(cands), fqnsOf(idx, cands))
	}
	for _, c := range cands {
		if c.Kind != symbols.SymbolCalcDef || idx.GetFQN(c) != "pick" {
			t.Errorf("candidate %s (%v) is not a root-level pick", idx.GetFQN(c), c.Kind)
		}
	}
	if sym, ok := r.ResolveQualified(caller, qn(false, "pick")); !ok || sym != cands[0] {
		t.Errorf("the name alone resolves to %v, want the first candidate", sym)
	}

	aRoot := idx.DocumentRoot("a.sysml")
	aPick, _ := aRoot.LookupLocal("pick")
	if own := r.InvocationCandidates(scopeOf(t, aRoot, "App"), qn(false, "pick")); len(own) != 1 || own[0] != aPick {
		t.Errorf("pick from App has candidates %v, want a.sysml's own only", fqnsOf(idx, own))
	}
}

// A root namespace has no owner for a visibility to hide its members from, so every
// root-level declaration is a member of the global namespace whatever its keyword
// (KerML 8.2.3.5; the pilot resolves them alike): a private or protected root
// overload in another document is a candidate exactly as a public one is.
func TestRootLevelVisibilityDoesNotGateInvocationCandidates(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "calc def pick { in x : ScalarValues::Integer; }",
		"b.sysml": "private calc def pick { in x : ScalarValues::String; }",
		"c.sysml": "protected calc def pick { in x : ScalarValues::Boolean; }",
		"d.sysml": "package Caller { attribute v = 1; }",
	})
	r := New(idx)

	caller := scopeOf(t, idx.DocumentRoot("d.sysml"), "Caller")
	cands := r.InvocationCandidates(caller, qn(false, "pick"))
	if len(cands) != 3 {
		t.Fatalf("pick from Caller has %d candidates, want the three root-level calcs: %v", len(cands), fqnsOf(idx, cands))
	}
	for _, doc := range []string{"a.sysml", "b.sysml", "c.sysml"} {
		own, _ := idx.DocumentRoot(doc).LookupLocal("pick")
		if !slices.Contains(cands, own) {
			t.Errorf("%s's root pick is not a candidate from Caller", doc)
		}
	}
	if sym, ok := r.ResolveQualified(caller, qn(false, "pick")); !ok || sym != cands[0] {
		t.Errorf("the name alone resolves to %v, want the first candidate", sym)
	}
}

func fqnsOf(idx *symbols.Index, syms []*symbols.Symbol) []string {
	out := make([]string, len(syms))
	for i, s := range syms {
		out[i] = idx.GetFQN(s)
	}
	return out
}
