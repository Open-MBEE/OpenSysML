package semantics

import (
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const featuringFixture = `package F {
	private import Occurrences::*;
	feature top;
	class A { feature x; member feature m; member feature fm featured by B; }
	class B;
	class C specializes A {
		feature declared featured by B;
		feature h { feature k; }
		feature i subsets h { feature viaSubset redefines k; }
		feature j { feature unrelated; }
		feature chained chains h.k;
		class Inner { feature n; }
	}
	class Occ specializes Occurrence { var feature v; feature plain; }
	class Sub specializes Occ { var feature w { feature underVariable; } }
}
`

// featuringSymbol resolves a `::`-separated path below the fixture package.
func featuringSymbol(t *testing.T, root *symbols.Scope, path string) *symbols.Symbol {
	t.Helper()
	scope := root
	var sym *symbols.Symbol
	for _, name := range strings.Split("F::"+path, "::") {
		if scope == nil {
			t.Fatalf("%s: no scope at %s", path, name)
		}
		found, ok := scope.LookupLocal(name)
		if !ok {
			t.Fatalf("%s: %s not found", path, name)
		}
		sym, scope = found, found.Scope
	}
	return sym
}

func featuringNames(types []FeaturingType) string {
	names := make([]string, 0, len(types))
	for _, t := range types {
		switch {
		case t.IsAnything():
			names = append(names, "Anything")
		case t.Snapshots:
			names = append(names, "snapshots("+t.Type.Name+")")
		default:
			names = append(names, t.Type.Name)
		}
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

// deriveFeatureFeaturingType (KerML 1.1 §8.3.3.3.4): the declared featuring types, the type a
// FeatureMembership implies (its snapshots for a variable feature) and the first chaining feature's.
func TestDerivedFeaturingTypes(t *testing.T) {
	want := map[string]string{
		"top":                   "",
		"A::x":                  "A",
		"A::m":                  "",
		"A::fm":                 "B",
		"C::declared":           "B,C",
		"C::h::k":               "h",
		"C::chained":            "C",
		"C::Inner::n":           "Inner",
		"Occ::v":                "snapshots(Occ)",
		"Occ::plain":            "Occ",
		"Sub::w::underVariable": "w",
	}
	for i, fixture := range reflectiveFixtures(t, "featuring.kerml", source.KindKerML, featuringFixture) {
		for path, names := range want {
			got := featuringNames(fixture.model.DerivedFeaturingTypes(featuringSymbol(t, fixture.root, path)))
			if got != names {
				t.Errorf("fixture %d: featuringType of %s = {%s}, want {%s}", i, path, got, names)
			}
		}
		if types := fixture.model.DerivedFeaturingTypes(featuringSymbol(t, fixture.root, "A")); types != nil {
			t.Errorf("fixture %d: a class has featuring types %v", i, types)
		}
	}
}

// Feature::canAccess (KerML 1.1 §8.3.3.3.4) walks the featuring types of featuring features, so a
// nested feature reaches what its enclosing types inherit and nothing a nested classifier's owner does;
// a variable feature reaches its owning type through the snapshots feature featuring it.
func TestCanAccess(t *testing.T) {
	cases := []struct {
		feature, target string
		want            bool
	}{
		{"C::declared", "A::x", true},
		{"C::h::k", "A::x", true},
		{"C::i::viaSubset", "C::h::k", true},
		{"C::j::unrelated", "C::h::k", false},
		{"C::Inner::n", "A::x", false},
		{"C::Inner::n", "top", true},
		{"top", "A::x", false},
		{"top", "A::m", true},
		{"A::fm", "A::x", false},
		{"Sub::w", "Occ::v", true},
		{"Sub::w", "Occ::plain", true},
		{"Sub::w::underVariable", "Occ::v", true},
		{"Sub::w::underVariable", "Occ::plain", true},
		{"C::chained", "A::x", true},
		{"C::j::unrelated", "C::chained", true},
	}
	for i, fixture := range reflectiveFixtures(t, "featuring.kerml", source.KindKerML, featuringFixture) {
		for _, c := range cases {
			feature, target := featuringSymbol(t, fixture.root, c.feature), featuringSymbol(t, fixture.root, c.target)
			if got := fixture.model.CanAccess(feature, target); got != c.want {
				t.Errorf("fixture %d: %s canAccess %s = %v, want %v", i, c.feature, c.target, got, c.want)
			}
		}
	}
}

// validateRedefinitionFeaturingTypes (KerML 1.1 §8.3.3.3.8) compares the featuring types with
// Anything included, a variable feature's read as its owning type.
func TestRedefinitionFeaturingTypesDiffer(t *testing.T) {
	cases := []struct {
		redefining, redefined string
		want                  bool
	}{
		{"C::h", "A::x", true},
		{"A::m", "top", false},
		{"C::declared", "A::fm", true},
		{"A::fm", "A::fm", false},
		{"Sub::w", "Occ::v", true},
		{"Occ::v", "Occ::plain", true},
		{"Occ::plain", "Occ::v", true},
	}
	for i, fixture := range reflectiveFixtures(t, "featuring.kerml", source.KindKerML, featuringFixture) {
		for _, c := range cases {
			redefining, redefined := featuringSymbol(t, fixture.root, c.redefining), featuringSymbol(t, fixture.root, c.redefined)
			if got := fixture.model.RedefinitionFeaturingTypesDiffer(redefining, redefined); got != c.want {
				t.Errorf("fixture %d: %s vs %s differ = %v, want %v", i, c.redefining, c.redefined, got, c.want)
			}
		}
	}
}
