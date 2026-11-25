package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const boundSubjectModel = `
package P {
    private import ScalarValues::*;
    part def Boiler { attribute speed : Real default = 1.2; }
    part boilerAsMaintained : Boiler;
    part config { part target : Boiler; }
    verification def Check { subject s : Boiler; }
    verification v1 : Check { subject :>> s = P::boilerAsMaintained; }
    verification v2 : Check { subject s = P::boilerAsMaintained; }
    verification v3 : Check;
    verification casted : Check { subject s = P::boilerAsMaintained as Boiler; }
    verification standaloneCast : Check { subject s = (as Boiler); }
    verification chained : Check { subject s = P::config.target; }
    verification defaulted : Check { subject s : Boiler default P::boilerAsMaintained; }
    verification initial : Check { subject s : Boiler := P::boilerAsMaintained; }
    verification def BoundDef { subject s : Boiler = P::boilerAsMaintained; }
    verification redefNoValue : BoundDef { subject :>> s; }
}
`

func boundSubjectSym(t *testing.T, root *symbols.Scope, pkg, name string) *symbols.Symbol {
	t.Helper()
	p := sym(t, root, pkg)
	found, ok := p.Scope.LookupLocal(name)
	if !ok {
		t.Fatalf("%s::%s not found", pkg, name)
	}
	return found
}

// TestBoundSubjectOfNamesTheBoundDeclaration pins both binding forms and a cast,
// and absence on an unbound case, a parameter, a standalone cast and an unrelated element.
func TestBoundSubjectOfNamesTheBoundDeclaration(t *testing.T) {
	m, root := buildModel(t, boundSubjectModel)
	for _, tc := range []struct {
		name string
		want string
	}{
		{"v1", "P::boilerAsMaintained"},
		{"v2", "P::boilerAsMaintained"},
		{"v3", ""},
		{"casted", "P::boilerAsMaintained"},
		{"standaloneCast", ""},
		{"Check", ""},
		{"boilerAsMaintained", ""},
	} {
		target, ok := m.BoundSubjectOf(boundSubjectSym(t, root, "P", tc.name))
		if tc.want == "" {
			if ok || target != nil {
				t.Errorf("%s: BoundSubjectOf = %v, %v; want absence", tc.name, target, ok)
			}
			continue
		}
		if !ok || target == nil {
			t.Errorf("%s: BoundSubjectOf answered no target", tc.name)
			continue
		}
		if got := symbols.FQNOf(target); got != tc.want {
			t.Errorf("%s: BoundSubjectOf = %s, want %s", tc.name, got, tc.want)
		}
	}
	// The subject parameter itself is never the declaration it binds.
	if subject, ok := boundSubjectSym(t, root, "P", "v1").Scope.LookupLocal("s"); ok {
		if target, ok := m.BoundSubjectOf(subject); ok || target != nil {
			t.Errorf("subject parameter answered %v, %v; want absence", target, ok)
		}
	}
}

// TestBoundSubjectOfReadsChainsDefaultsInitialAndInheritedBindings pins a
// chain, absent defaults/initial values, and an inherited binding.
func TestBoundSubjectOfReadsChainsDefaultsInitialAndInheritedBindings(t *testing.T) {
	m, root := buildModel(t, boundSubjectModel)
	for _, tc := range []struct {
		name string
		want string
	}{
		{"chained", "P::config::target"},
		{"defaulted", ""},
		{"initial", ""},
		{"redefNoValue", "P::boilerAsMaintained"},
		{"BoundDef", "P::boilerAsMaintained"},
	} {
		target, ok := m.BoundSubjectOf(boundSubjectSym(t, root, "P", tc.name))
		if tc.want == "" {
			if ok || target != nil {
				t.Errorf("%s: BoundSubjectOf = %v, %v; want absence", tc.name, target, ok)
			}
			continue
		}
		if !ok || target == nil {
			t.Errorf("%s: BoundSubjectOf answered no target", tc.name)
			continue
		}
		if got := symbols.FQNOf(target); got != tc.want {
			t.Errorf("%s: BoundSubjectOf = %s, want %s", tc.name, got, tc.want)
		}
	}
}

// TestBoundSubjectOfResolvesEachBindingOnce counts resolutions: a second read
// of every case's subject adds nothing to the resolver's memo.
func TestBoundSubjectOfResolvesEachBindingOnce(t *testing.T) {
	m, _, resolver, _ := buildUnresolvedModel(t, "t.sysml", source.KindSysML, boundSubjectModel)
	root := m.resolver.Index().DocumentRoot("t.sysml")
	cases := []*symbols.Symbol{
		boundSubjectSym(t, root, "P", "v1"),
		boundSubjectSym(t, root, "P", "v2"),
		boundSubjectSym(t, root, "P", "v3"),
	}
	before := resolver.MemoSize()
	for _, c := range cases {
		m.BoundSubjectOf(c)
	}
	afterFirst := resolver.MemoSize()
	for _, c := range cases {
		m.BoundSubjectOf(c)
	}
	afterSecond := resolver.MemoSize()
	if afterFirst <= before {
		t.Errorf("resolving %d cases added no memo entries (%d to %d), so the bindings were never read", len(cases), before, afterFirst)
	}
	if afterSecond != afterFirst {
		t.Errorf("re-reading bound subjects grew the resolver memo from %d to %d", afterFirst, afterSecond)
	}
}
