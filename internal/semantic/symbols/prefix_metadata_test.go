package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// prefixMembers is the metadata usages a `#M` prefix declares among sym's members.
func prefixMembers(sym *Symbol) []*Symbol {
	var out []*Symbol
	for _, member := range sym.Scope.AllMembers() {
		if member.IsPrefixMetadata() {
			out = append(out, member)
		}
	}
	return out
}

// A `#M` prefix on a definition, usage, parameter or package declares a
// MetadataUsage the annotated element owns, a member of its scope like `@M;`.
func TestPrefixMetadataIsAMemberOfTheAnnotatedElement(t *testing.T) {
	root := build(t, `#Tag package P {
		metadata def Tag { attribute level; }
		#Tag part def D { part a; }
		#Tag part d : D;
		action def Act {
			in #Tag x;
		}
		#Tag state def S;
		state def M { #Tag state s; }
		part def Body { @Tag; }
	}`)
	p, _ := root.LookupLocal("P")
	if got := prefixMembers(p); len(got) != 1 || got[0].Decl.(*ast.PrefixMetadata) == nil {
		t.Fatalf("P prefix members = %d, want the one #Tag", len(got))
	}
	for _, name := range []string{"D", "d", "S"} {
		sym, ok := p.Scope.LookupLocal(name)
		if !ok {
			t.Fatalf("P members = %v, want %s", p.Scope.MemberNames(), name)
		}
		members := sym.Scope.AllMembers()
		if len(members) == 0 {
			t.Fatalf("%s owns no members, want its #Tag last", name)
		}
		last := members[len(members)-1]
		if !last.IsPrefixMetadata() || last.Kind != SymbolMetadataUsage || last.Name != "" {
			t.Errorf("%s's last member = %v %q, want the anonymous #Tag metadata usage", name, last.Kind, last.Name)
		}
		if last.OwnerScope != sym.Scope || last.Owner() != sym {
			t.Errorf("%s's #Tag is owned by %v, want %s", name, last.Owner(), name)
		}
		if last.Scope == nil || last.Scope.Annotated() != sym.Decl {
			t.Errorf("%s's #Tag body scope names %v as annotated, want %s's declaration", name, last.Scope, name)
		}
		if _, isPrefix := last.Decl.(*ast.PrefixMetadata); !isPrefix {
			t.Errorf("%s's #Tag is declared by %T, want *ast.PrefixMetadata", name, last.Decl)
		}
		if got := prefixMembers(sym); len(got) != 1 {
			t.Errorf("%s prefix members = %d, want 1", name, len(got))
		}
	}
	d, _ := p.Scope.LookupLocal("D")
	if names := d.Scope.MemberNames(); len(names) != 1 || names[0] != "a" {
		t.Errorf("D named members = %v, want a alone: the prefix is anonymous", names)
	}
	act, _ := p.Scope.LookupLocal("Act")
	x, _ := act.Scope.LookupLocal("x")
	if got := prefixMembers(x); len(got) != 1 || got[0].Owner() != x {
		t.Errorf("parameter x prefix members = %d, want its one #Tag", len(got))
	}
	m, _ := p.Scope.LookupLocal("M")
	s, _ := m.Scope.LookupLocal("s")
	if got := prefixMembers(s); len(got) != 1 || got[0].Owner() != s {
		t.Errorf("substate s prefix members = %d, want its one #Tag", len(got))
	}
	body, _ := p.Scope.LookupLocal("Body")
	anon := body.Scope.AnonymousMembers()
	if len(anon) != 1 || anon[0].Kind != SymbolMetadataUsage || anon[0].IsPrefixMetadata() {
		t.Errorf("Body's @Tag = %v, want one anonymous metadata usage that is no prefix", anon)
	}
}

// An unnamed control node declares an element once a prefix annotates it.
func TestPrefixMetadataOnAnUnnamedControlNodeDeclaresIt(t *testing.T) {
	root := build(t, `package P {
		metadata def Tag;
		action def A {
			#Tag fork;
			merge;
		}
	}`)
	p, _ := root.LookupLocal("P")
	a, _ := p.Scope.LookupLocal("A")
	anon := a.Scope.AnonymousMembers()
	if len(anon) != 1 {
		t.Fatalf("anonymous members of A = %d, want the tagged fork alone", len(anon))
	}
	if _, ok := anon[0].Decl.(*ast.ForkNode); !ok {
		t.Fatalf("anonymous member declared by %T, want *ast.ForkNode", anon[0].Decl)
	}
	if got := prefixMembers(anon[0]); len(got) != 1 {
		t.Errorf("fork prefix members = %d, want 1", len(got))
	}
}
