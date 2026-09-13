package resolve

import (
	"strings"
	"testing"
)

// diagnosticContaining is the first diagnostic message holding text, "" when
// none does.
func diagnosticContaining(r *Resolver, text string) string {
	for _, d := range r.Diagnostics {
		if strings.Contains(d.Message, text) {
			return d.Message
		}
	}
	return ""
}

// A bare name that is the unquoted start of a declaration the scope reaches is
// offered that declaration, quoted, with the rule that made the quotes needed.
func TestUnresolvedNameSuggestsTheQuotedDeclarationItStarts(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package T {
		part def Rocket;
		individual part def 'SA-506' :> Rocket;
		part c : SA;
	}`)
	want := "unresolved reference: SA — did you mean 'SA-506'? Names containing '-' must be quoted."
	if got := diagnosticContaining(r, "unresolved reference: SA"); got != want {
		t.Errorf("diagnostic = %q, want %q", got, want)
	}
}

// A qualified name whose last segment is the unquoted start of a member of the
// namespace it names is offered that member, qualified and quoted.
func TestUnresolvedMemberSuggestsTheQuotedMemberItStarts(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package T {
		part def Rocket;
		individual part def 'SA-506' :> Rocket;
		requirement def 'HLR-R001';
	}
	package U {
		part d : T::SA;
		requirement r : T::HLR;
	}`)
	for written, want := range map[string]string{
		"T::SA":  "unresolved reference: T::SA — did you mean T::'SA-506'? Names containing '-' must be quoted.",
		"T::HLR": "unresolved reference: T::HLR — did you mean T::'HLR-R001'? Names containing '-' must be quoted.",
	} {
		if got := diagnosticContaining(r, "unresolved reference: "+written); got != want {
			t.Errorf("diagnostic for %s = %q, want %q", written, got, want)
		}
	}
}

// A declaration the scope does not reach is offered by the qualified path that
// declares it, each segment quoted as its spelling needs.
func TestUnresolvedNameSuggestsTheQuotedDeclarationByItsPath(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package 'My Lib' { part def 'SA-506'; }
	package U { part d : SA; }`)
	want := "unresolved reference: SA — did you mean 'My Lib'::'SA-506'? Names containing ' ' or '-' must be quoted."
	if got := diagnosticContaining(r, "unresolved reference: SA"); got != want {
		t.Errorf("diagnostic = %q, want %q", got, want)
	}
}

// Only a declaration going on with a character no basic name holds is offered as
// typed without its quotes: a longer identifier is not, and nothing is made up.
func TestUnresolvedNameOffersNoQuotedSpellingForAnIdentifier(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package T {
		part def SATURN;
		part def SA_506;
		part c : SA;
	}`)
	got := diagnosticContaining(r, "unresolved reference: SA")
	if got == "" || strings.Contains(got, "must be quoted") || strings.Contains(got, "'") {
		t.Errorf("diagnostic = %q, want no quoted spelling offered", got)
	}
}

// A near spelling that itself needs quotes is written with them, so the
// suggestion can be typed back as it reads.
func TestNearSpellingIsOfferedQuoted(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package T {
		part def 'SA-506';
		part c : SA_506;
	}`)
	want := "unresolved reference: SA_506 — did you mean 'SA-506'?"
	if got := diagnosticContaining(r, "unresolved reference: SA_506"); got != want {
		t.Errorf("diagnostic = %q, want %q", got, want)
	}
}

// The fixes an unresolved name carries write the quoted spelling, the
// declaration the name is the unquoted start of first.
func TestUnresolvedNameFixesWriteTheQuotedSpelling(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package T {
		part def 'SA-506';
		part c : SA;
	}`)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %v, want one", r.Diagnostics)
	}
	fixes := r.Diagnostics[0].Fixes
	if len(fixes) == 0 {
		t.Fatal("no fixes")
	}
	fix := fixes[0]
	if fix.Title != "Change 'SA' to 'SA-506'" {
		t.Errorf("fix title = %q", fix.Title)
	}
	if len(fix.Edits) != 1 || fix.Edits[0].NewText != "'SA-506'" {
		t.Errorf("the fix writes %+v, want 'SA-506' quoted", fix.Edits)
	}
}

// The public UnresolvedName reads as the diagnostic does, so callers writing
// their own message get the same hint.
func TestUnresolvedNameHintMatchesTheDiagnostic(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `package T {
		part def 'SA-506';
		part c : SA;
	}`)
	pkgs := r.idx.LookupQualified("T")
	if len(pkgs) != 1 || pkgs[0].Scope == nil {
		t.Fatal("no scope for T")
	}
	want := "SA — did you mean 'SA-506'? Names containing '-' must be quoted."
	if got := r.UnresolvedName(pkgs[0].Scope, "SA", nil); got != want {
		t.Errorf("UnresolvedName = %q, want %q", got, want)
	}
	if got := r.UnresolvedName(pkgs[0].Scope, "Zzz", nil); got != "Zzz" {
		t.Errorf("UnresolvedName(Zzz) = %q, want the name alone", got)
	}
}
