package symbols

import (
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

func TestScopeMemberNamesInOrder(t *testing.T) {
	s := NewScope(nil, nil)
	s.Define("Beta", &Symbol{Name: "Beta", Kind: SymbolPackage})
	s.Define("Alpha", &Symbol{Name: "Alpha", Kind: SymbolNamespace})
	// Duplicate key must not add a second MemberNames entry.
	s.Define("Beta", &Symbol{Name: "Beta", Kind: SymbolNamespace})

	names := s.MemberNames()
	if len(names) != 2 || names[0] != "Beta" || names[1] != "Alpha" {
		t.Fatalf("MemberNames = %v, want [Beta Alpha]", names)
	}
}

func TestScopeForEachMemberMatchesAllMembers(t *testing.T) {
	s := NewScope(nil, nil)
	first := &Symbol{Name: "first"}
	second := &Symbol{Name: "second"}
	anonymous := &Symbol{}
	s.Define("first", first)
	s.Define("second", second)
	s.DefineAnonymous(anonymous)

	want := s.AllMembers()
	var got []*Symbol
	s.ForEachMember(func(sym *Symbol) bool {
		got = append(got, sym)
		return true
	})
	if len(got) != len(want) {
		t.Fatalf("ForEachMember visited %d symbols, AllMembers returned %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ForEachMember[%d] = %p, AllMembers[%d] = %p", i, got[i], i, want[i])
		}
	}
}

func TestScopeForEachMemberStopsWhenYieldReturnsFalse(t *testing.T) {
	s := NewScope(nil, nil)
	s.Define("first", &Symbol{Name: "first"})
	s.Define("second", &Symbol{Name: "second"})
	s.DefineAnonymous(&Symbol{})

	count := 0
	s.ForEachMember(func(*Symbol) bool {
		count++
		return false
	})
	if count != 1 {
		t.Fatalf("ForEachMember visited %d symbols after stop, want 1", count)
	}
}

func TestScopeLookupPromotesAtThreshold(t *testing.T) {
	for _, count := range []int{memberIndexThreshold, memberIndexThreshold + 1} {
		t.Run(fmt.Sprintf("members=%d", count), func(t *testing.T) {
			s := NewScope(nil, nil)
			first := &Symbol{Name: "first"}
			second := &Symbol{Name: "second"}
			for i := 0; i < count-2; i++ {
				s.Define(fmt.Sprintf("member%d", i), &Symbol{Name: fmt.Sprintf("member%d", i)})
			}
			s.Define("target", first)
			s.Define("target", second)

			got := s.LookupLocalAll("target")
			if len(got) != 2 || got[0] != first || got[1] != second {
				t.Fatalf("LookupLocalAll(target) = %v, want [%p %p]", got, first, second)
			}
			if count > memberIndexThreshold && s.builtNameIndex() == nil {
				t.Fatal("LookupLocalAll did not build the large-scope index")
			}
		})
	}
}

func TestScopeLookupRepeatedKeyOrderAndMemberNameDeduplication(t *testing.T) {
	s := NewScope(nil, nil)
	first := &Symbol{Name: "first"}
	second := &Symbol{Name: "second"}
	third := &Symbol{Name: "third"}
	s.Define("x", first)
	s.Define("y", second)
	s.Define("x", third)

	got := s.LookupLocalAll("x")
	if len(got) != 2 || got[0] != first || got[1] != third {
		t.Fatalf("LookupLocalAll(x) = %v, want [%p %p]", got, first, third)
	}
	names := s.MemberNames()
	if len(names) != 2 || names[0] != "x" || names[1] != "y" {
		t.Fatalf("MemberNames = %v, want [x y]", names)
	}
}

func TestScopeMemberNamesLargeScopeBuildsIndex(t *testing.T) {
	names := []string{"first", "repeat", "second", "repeat"}
	for i := len(names); i < memberIndexThreshold*3; i++ {
		names = append(names, fmt.Sprintf("member%d", i))
	}

	small := NewScope(nil, nil)
	for _, name := range names[:4] {
		small.Define(name, &Symbol{Name: name})
	}
	smallNames := small.MemberNames()

	large := NewScope(nil, nil)
	for _, name := range names {
		large.Define(name, &Symbol{Name: name})
	}
	if large.builtNameIndex() != nil {
		t.Fatal("large scope unexpectedly had an index before MemberNames")
	}
	largeNames := large.MemberNames()
	if large.builtNameIndex() == nil {
		t.Fatal("large MemberNames did not build the lookup index")
	}
	if len(largeNames) < len(smallNames) {
		t.Fatalf("large MemberNames returned %d names, want at least %d", len(largeNames), len(smallNames))
	}
	for i, name := range smallNames {
		if largeNames[i] != name {
			t.Fatalf("large MemberNames[%d] = %q, want %q", i, largeNames[i], name)
		}
	}
	if largeNames[1] != "repeat" {
		t.Fatalf("large MemberNames = %v, want repeat deduplicated in first-seen order", largeNames[:4])
	}
}

func TestScopeMembersPreserveInterleavedDeclarationOrder(t *testing.T) {
	s := NewScope(nil, nil)
	first := &Symbol{Name: "first"}
	anonymous := &Symbol{}
	third := &Symbol{Name: "third"}
	s.Define("a", first)
	s.DefineAnonymous(anonymous)
	s.Define("a", third)

	want := []*Symbol{first, anonymous, third}
	for i, got := range s.AllMembers() {
		if got != want[i] {
			t.Fatalf("AllMembers()[%d] = %p, want %p", i, got, want[i])
		}
	}
	var got []*Symbol
	s.ForEachMember(func(sym *Symbol) bool {
		got = append(got, sym)
		return true
	})
	if len(got) != len(want) {
		t.Fatalf("ForEachMember visited %d symbols, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ForEachMember()[%d] = %p, want %p", i, got[i], want[i])
		}
	}
	wantNamed := []*Symbol{first, third}
	named := s.Members()
	if len(named) != len(wantNamed) {
		t.Fatalf("Members() returned %d symbols, want %d", len(named), len(wantNamed))
	}
	for i, got := range named {
		if got != wantNamed[i] {
			t.Fatalf("Members()[%d] = %p, want %p", i, got, wantNamed[i])
		}
	}
}

func TestScopeLookupSingleResultHasCappedCapacity(t *testing.T) {
	s := NewScope(nil, nil)
	first := &Symbol{Name: "first"}
	second := &Symbol{Name: "second"}
	s.Define("first", first)
	s.Define("second", second)

	got := s.LookupLocalAll("first")
	got = append(got, &Symbol{Name: "appended"})
	if len(got) != 2 || got[0] != first {
		t.Fatalf("appended lookup result = %v, want first followed by appended", got)
	}
	if next := s.LookupLocalAll("second"); len(next) != 1 || next[0] != second {
		t.Fatalf("LookupLocalAll(second) = %v, want [%p]", next, second)
	}
}

func TestScopeForEachMemberUsesEntryLengthWhileDefining(t *testing.T) {
	s := NewScope(nil, nil)
	first := &Symbol{Name: "first"}
	second := &Symbol{Name: "second"}
	third := &Symbol{Name: "third"}
	s.Define("first", first)
	s.Define("second", second)

	var got []*Symbol
	s.ForEachMember(func(sym *Symbol) bool {
		got = append(got, sym)
		if sym == first {
			s.Define("third", third)
		}
		return true
	})
	if len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("ForEachMember visited %v, want [%p %p]", got, first, second)
	}
	if found, ok := s.LookupLocal("third"); !ok || found != third {
		t.Fatalf("LookupLocal(third) = %p, %v; want %p, true", found, ok, third)
	}
}

func TestNewScopeHasNoMemberIndex(t *testing.T) {
	s := NewScope(nil, nil)
	if s.builtNameIndex() != nil {
		t.Fatal("empty scope has a member index")
	}
	if got := s.LookupLocalAll("missing"); got != nil {
		t.Fatalf("LookupLocalAll(missing) = %v, want nil", got)
	}
	if s.builtNameIndex() != nil {
		t.Fatal("empty scope built a member index")
	}
}

func TestScopeMemberDeclaringAtEitherSideOfThreshold(t *testing.T) {
	for _, count := range []int{memberIndexThreshold, memberIndexThreshold + 1} {
		t.Run(fmt.Sprintf("members=%d", count), func(t *testing.T) {
			s := NewScope(nil, nil)
			decl := &ast.Package{}
			first := &Symbol{Name: "first", Decl: decl}
			second := &Symbol{Name: "second", Decl: decl}
			anonymous := &Symbol{Decl: &ast.Package{}}
			s.Define("first", first)
			s.DefineAnonymous(anonymous)
			s.Define("second", second)
			for i := 3; i < count; i++ {
				s.Define(fmt.Sprintf("member%d", i), &Symbol{Name: fmt.Sprintf("member%d", i), Decl: &ast.Package{}})
			}

			if got := s.MemberDeclaring(decl); got != first {
				t.Fatalf("MemberDeclaring(shared decl) = %v, want the first registration %p", got, first)
			}
			if got := s.MemberDeclaring(anonymous.Decl); got != anonymous {
				t.Fatalf("MemberDeclaring(anonymous decl) = %v, want %p", got, anonymous)
			}
			if got := s.MemberDeclaring(&ast.Package{}); got != nil {
				t.Fatalf("MemberDeclaring(unknown decl) = %v, want nil", got)
			}
			if (s.builtDeclIndex() != nil) != (count > memberIndexThreshold) {
				t.Fatalf("declaration index built = %v for %d members", s.builtDeclIndex() != nil, count)
			}
		})
	}
}

func TestScopeMemberDeclaringSeesMembersDefinedAfterIndexing(t *testing.T) {
	s := NewScope(nil, nil)
	for i := 0; i <= memberIndexThreshold; i++ {
		s.Define(fmt.Sprintf("member%d", i), &Symbol{Name: fmt.Sprintf("member%d", i), Decl: &ast.Package{}})
	}
	if s.MemberDeclaring(&ast.Package{}) != nil || s.builtDeclIndex() == nil {
		t.Fatal("large scope did not build its declaration index on lookup")
	}

	named := &Symbol{Name: "late", Decl: &ast.Package{}}
	s.Define("late", named)
	if got := s.MemberDeclaring(named.Decl); got != named {
		t.Fatalf("MemberDeclaring after Define = %v, want %p", got, named)
	}
	anonymous := &Symbol{Decl: &ast.Package{}}
	s.DefineAnonymous(anonymous)
	if got := s.MemberDeclaring(anonymous.Decl); got != anonymous {
		t.Fatalf("MemberDeclaring after DefineAnonymous = %v, want %p", got, anonymous)
	}
}

func TestScopeForEachAnonymousMemberVisitsInOrderAndStops(t *testing.T) {
	s := NewScope(nil, nil)
	first, second := &Symbol{}, &Symbol{}
	s.DefineAnonymous(first)
	s.Define("named", &Symbol{Name: "named"})
	s.DefineAnonymous(second)

	var got []*Symbol
	s.ForEachAnonymousMember(func(sym *Symbol) bool {
		got = append(got, sym)
		return true
	})
	if len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("ForEachAnonymousMember visited %v, want [%p %p]", got, first, second)
	}
	count := 0
	s.ForEachAnonymousMember(func(*Symbol) bool {
		count++
		return false
	})
	if count != 1 {
		t.Fatalf("ForEachAnonymousMember visited %d symbols after stop, want 1", count)
	}
	var nilScope *Scope
	nilScope.ForEachAnonymousMember(func(*Symbol) bool { t.Fatal("visited a member of a nil scope"); return false })
}

// builtNameIndex is the name index if built, nil otherwise.
func (s *Scope) builtNameIndex() map[string][]int32 {
	if indexes := s.indexes.Load(); indexes != nil {
		return indexes.byName
	}
	return nil
}

// builtDeclIndex is the declaration index if built, nil otherwise.
func (s *Scope) builtDeclIndex() map[ast.Node]*Symbol {
	if indexes := s.indexes.Load(); indexes != nil {
		return indexes.byDecl
	}
	return nil
}
