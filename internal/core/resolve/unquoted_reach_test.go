package resolve_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// unresolvedMessage resolves src with a semantic model attached and returns the
// unresolved-reference diagnostic for the name written, "" when there is none.
func unresolvedMessage(t *testing.T, src, written string) string {
	t.Helper()
	const name = "d.sysml"
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name, root)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	r.SetModel(semantics.NewModel(r))
	r.ResolveDocument(name, root)
	for _, d := range r.Diagnostics {
		if strings.HasPrefix(d.Message, "unresolved reference: "+written) {
			return d.Message
		}
	}
	return ""
}

const quotingRule = "Names containing '-' must be quoted."

// A quoted member is offered only when the spelling offered resolves from where
// the name was written: a member private to its namespace is not.
func TestUnresolvedMemberOffersNoPrivateMember(t *testing.T) {
	got := unresolvedMessage(t, `package T {
		private part def 'Secret-Key';
		part def Rocket;
	}
	package U { part d : T::Secret; }`, "T::Secret")
	if got != "unresolved reference: T::Secret" {
		t.Errorf("diagnostic = %q, want no spelling offered", got)
	}
}

// A quoted member the namespace inherits resolves under it, so it is offered.
func TestUnresolvedMemberOffersAnInheritedMember(t *testing.T) {
	got := unresolvedMessage(t, `package T {
		part def Base { part 'SA-506' : Base; }
		part def Sub :> Base;
		part d : Sub::SA;
	}`, "Sub::SA")
	want := "unresolved reference: Sub::SA — did you mean Sub::'SA-506'? " + quotingRule
	if got != want {
		t.Errorf("diagnostic = %q, want %q", got, want)
	}
}

// A quoted member a public import surfaces in the namespace is offered too.
func TestUnresolvedMemberOffersAnImportedMember(t *testing.T) {
	got := unresolvedMessage(t, `package Lib { part def 'SA-506'; }
	package T { public import Lib::*; }
	package U { part d : T::SA; }`, "T::SA")
	want := "unresolved reference: T::SA — did you mean T::'SA-506'? " + quotingRule
	if got != want {
		t.Errorf("diagnostic = %q, want %q", got, want)
	}
}

// A `$::`-rooted name is offered rooted: the bare spelling would start at a
// local namespace of the same name.
func TestUnresolvedGlobalMemberIsOfferedRooted(t *testing.T) {
	got := unresolvedMessage(t, `package T { part def 'SA-506'; }
	package U {
		package T;
		part d : $::T::SA;
	}`, "$::T::SA")
	want := "unresolved reference: $::T::SA — did you mean $::T::'SA-506'? " + quotingRule
	if got != want {
		t.Errorf("diagnostic = %q, want %q", got, want)
	}
}

// A name holding `::` is offered quoted whole, bare or under its namespace: the
// spelling typed back must be one segment, not a qualification.
func TestUnresolvedNameQuotesAnEmbeddedSeparatorWhole(t *testing.T) {
	const rule = "Names containing ':' or '-' must be quoted."
	src := `package T {
		part def 'left::right-X';
		part d : left;
	}
	package U { part e : T::left; }`
	if got, want := unresolvedMessage(t, src, "left"),
		"unresolved reference: left — did you mean 'left::right-X'? "+rule; got != want {
		t.Errorf("bare diagnostic = %q, want %q", got, want)
	}
	if got, want := unresolvedMessage(t, src, "T::left"),
		"unresolved reference: T::left — did you mean T::'left::right-X'? "+rule; got != want {
		t.Errorf("qualified diagnostic = %q, want %q", got, want)
	}
}
