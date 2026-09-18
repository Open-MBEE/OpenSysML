// Exercised from outside the package: building an index of records needs the loader,
// which imports semantics.
package semantics_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A library symbol keeps its declaration on either load path, so every rule it
// takes part in must answer the same whichever path loaded it.
const semanticsLibrary = `standard library package Kit {
	attribute def Mass;
	metadata def Safety;
	part def Fastener {
		attribute mass : Mass;
	}
	part def Bolt :> Fastener;
	part def HexBolt :> Bolt;
	part def Nut :> Fastener {
		attribute mass : Mass [0..1];
	}
	package Critical {
		filter @Safety;
		#Safety part def Guard;
		part def Plain;
	}
}`

const inheritedImplicitLibrary = `package Occurrences {
	class Occurrence { feature endShot; }
}
package Objects {
	struct Object specializes Occurrences::Occurrence;
}
struct Wheel;
struct MyWheel specializes Wheel {
	feature redefines endShot : EndShot;
}
struct EndShot;`

// Conformance follows the specialization graph, which a library load records as
// supertype facts: it must answer the same cold and warm, and transitively.
func TestConformanceOverALibraryIsTheSameColdAndWarm(t *testing.T) {
	cold, warm := bothPaths(t)
	for _, tc := range []struct {
		sub, super string
		want       bool
	}{
		{"Kit::Bolt", "Kit::Fastener", true},
		{"Kit::HexBolt", "Kit::Fastener", true}, // transitive
		{"Kit::Fastener", "Kit::Bolt", false},   // conformance is not symmetric
		{"Kit::Nut", "Kit::Bolt", false},        // siblings do not conform
	} {
		for path, lib := range map[string]library{"cold": cold, "warm": warm} {
			got := lib.model.Conforms(lib.symbol(t, tc.sub), lib.symbol(t, tc.super))
			if got != tc.want {
				t.Errorf("%s: Conforms(%s, %s) = %v, want %v", path, tc.sub, tc.super, got, tc.want)
			}
		}
	}
}

func TestInheritedImplicitBaseIsTheSameColdAndWarm(t *testing.T) {
	dir, cacheDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wheel.kerml"), []byte(inheritedImplicitLibrary), 0o600); err != nil {
		t.Fatalf("write library: %v", err)
	}
	cold := modelOver(t, loadLibrary(t, dir, cacheDir))
	warm := modelOver(t, loadLibrary(t, dir, cacheDir))

	for path, lib := range map[string]library{"cold": cold, "warm": warm} {
		myWheel := lib.symbol(t, "MyWheel")
		if myWheel.Decl == nil {
			t.Fatalf("the %s load leaves MyWheel without its declaration", path)
		}
		inherited, ok := lib.model.LookupContributedMember(myWheel, "endShot")
		if !ok {
			t.Errorf("%s: MyWheel does not inherit endShot through Wheel's implicit base", path)
			continue
		}
		if got := lib.idx.GetFQN(inherited); got != "Occurrences::Occurrence::endShot" {
			t.Errorf("%s: inherited endShot = %q", path, got)
		}
	}
}

// The members of a library type are the same on both paths, including the
// inherited ones and the masking a closer declaration applies.
func TestMembersOfALibraryTypeAreTheSameColdAndWarm(t *testing.T) {
	cold, warm := bothPaths(t)
	for _, tc := range []struct{ owner, member, want string }{
		// Declared, inherited, and inherited-but-masked by a redeclaration.
		{"Kit::Fastener", "mass", "Kit::Fastener::mass"},
		{"Kit::Bolt", "mass", "Kit::Fastener::mass"},
		{"Kit::HexBolt", "mass", "Kit::Fastener::mass"},
		{"Kit::Nut", "mass", "Kit::Nut::mass"},
	} {
		for path, lib := range map[string]library{"cold": cold, "warm": warm} {
			member, ok := lib.model.LookupMember(lib.symbol(t, tc.owner), tc.member)
			if !ok {
				t.Errorf("%s: %s has no member %s", path, tc.owner, tc.member)
				continue
			}
			if got := lib.idx.GetFQN(member); got != tc.want {
				t.Errorf("%s: %s::%s = %s, want %s", path, tc.owner, tc.member, got, tc.want)
			}
		}
	}
}

// A member reached only through what a type specializes must not be reported as
// one of its own, whichever path the library came from.
func TestContributedMembersOfALibraryTypeAreTheSameColdAndWarm(t *testing.T) {
	cold, warm := bothPaths(t)
	for path, lib := range map[string]library{"cold": cold, "warm": warm} {
		nut := lib.symbol(t, "Kit::Nut")
		contributed, ok := lib.model.LookupContributedMember(nut, "mass")
		if !ok {
			t.Errorf("%s: Nut inherits no mass from Fastener", path)
			continue
		}
		if got := lib.idx.GetFQN(contributed); got != "Kit::Fastener::mass" {
			t.Errorf("%s: the mass Nut inherits = %s, want Kit::Fastener::mass", path, got)
		}
	}
}

// A library `filter` reaches the evaluator as its parsed condition on both
// paths, and must classify the same either way.
func TestNamespaceFilterOverALibraryIsTheSameColdAndWarm(t *testing.T) {
	cold, warm := bothPaths(t)
	for path, lib := range map[string]library{"cold": cold, "warm": warm} {
		if f := filterOn(t, lib, "Kit::Critical"); f.IsZero() {
			t.Errorf("the %s load does not carry the condition", path)
		}
	}
	for _, tc := range []struct {
		cand string
		want bool
	}{
		{"Kit::Critical::Guard", true},
		{"Kit::Critical::Plain", false},
		{"Kit::Bolt", false},
	} {
		for path, lib := range map[string]library{"cold": cold, "warm": warm} {
			got := lib.model.SatisfiesElementFilter(filterOn(t, lib, "Kit::Critical"), lib.symbol(t, tc.cand))
			if got != tc.want {
				t.Errorf("%s: @Safety admits %s = %v, want %v", path, tc.cand, got, tc.want)
			}
		}
	}
}

// The metadata annotating a library element reads the same on both paths, so a
// filter classifies it the same way.
func TestAnnotationFactsOfALibraryElementSurviveTheCache(t *testing.T) {
	cold, warm := bothPaths(t)
	for path, lib := range map[string]library{"cold": cold, "warm": warm} {
		facts := lib.model.AnnotationFactsOf(lib.symbol(t, "Kit::Critical::Guard"))
		if len(facts) != 1 || facts[0].TypeFQN != "Kit::Safety" {
			t.Errorf("%s: Guard is annotated %+v, want one Kit::Safety", path, facts)
		}
		if facts := lib.model.AnnotationFactsOf(lib.symbol(t, "Kit::Critical::Plain")); len(facts) != 0 {
			t.Errorf("%s: Plain is annotated %+v, want nothing", path, facts)
		}
	}
}

// filterOn is the single condition the namespace registered under fqn declares.
func filterOn(t *testing.T, lib library, fqn string) symbols.ElementFilter {
	t.Helper()
	filters := lib.idx.NamespaceFiltersOf(fqn)
	if len(filters) != 1 {
		t.Fatalf("%s declares %d filters, want 1", fqn, len(filters))
	}
	return filters[0]
}

// library is one indexed copy of semanticsLibrary with a model over it.
type library struct {
	idx   *symbols.Index
	model *semantics.Model
}

func (l library) symbol(t *testing.T, fqn string) *symbols.Symbol {
	t.Helper()
	syms := l.idx.LookupQualified(fqn)
	if len(syms) != 1 {
		t.Fatalf("%s names %d symbols, want 1", fqn, len(syms))
	}
	return syms[0]
}

// bothPaths loads semanticsLibrary twice through one cache directory: the first
// load derives its facts, the second restores them.
func bothPaths(t *testing.T) (cold, warm library) {
	t.Helper()
	dir, cacheDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "kit.sysml"), []byte(semanticsLibrary), 0o600); err != nil {
		t.Fatalf("write library: %v", err)
	}
	cold = modelOver(t, loadLibrary(t, dir, cacheDir))
	warm = modelOver(t, loadLibrary(t, dir, cacheDir))
	// Both loads parse the library, which is what makes the comparisons below a
	// test of the fact path rather than of two different states.
	if cold.symbol(t, "Kit::Fastener").Decl == nil {
		t.Fatal("the cold load leaves the library without its declarations")
	}
	if warm.symbol(t, "Kit::Fastener").Decl == nil {
		t.Fatal("the warm load leaves the library without its declarations")
	}
	return cold, warm
}

// modelOver is a semantic model over idx, wired as the workspace wires it.
func modelOver(t *testing.T, idx *symbols.Index) library {
	t.Helper()
	r := resolve.New(idx)
	m := semantics.NewModel(r)
	r.SetModel(m)
	return library{idx: idx, model: m}
}

// loadLibrary indexes every file of the library in dir through a loader backed
// by cacheDir, persisting its facts exactly as the stdlib load does.
func loadLibrary(t *testing.T, dir, cacheDir string) *symbols.Index {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	cache, err := libs.NewCache()
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	src := libs.NewDirSource(dir)
	idx := symbols.NewIndex()
	if err := libs.NewLoader(src, cache).LoadAll(idx); err != nil {
		t.Fatalf("load the library: %v", err)
	}
	if len(idx.FQNs()) == 0 {
		t.Fatal("the load registered nothing")
	}
	return idx
}

// A library feature is parsed on both paths, so it takes the multiplicity it
// declares rather than the assumed 1..1 (roadmap L3).
func TestMultiplicityOfALibraryFeatureIsTheSameColdAndWarm(t *testing.T) {
	cold, warm := bothPaths(t)
	got := cold.model.EffectiveMultiplicityOf(cold.symbol(t, "Kit::Nut::mass"))
	if want := warm.model.EffectiveMultiplicityOf(warm.symbol(t, "Kit::Nut::mass")); got != want {
		t.Errorf("Kit::Nut::mass multiplicity = %+v cold, %+v warm", got, want)
	}
	declared := semantics.Range{
		Lower: semantics.Bound{Value: 0, Known: true},
		Upper: semantics.Bound{Value: 1, Known: true},
	}
	if got != declared {
		t.Errorf("Kit::Nut::mass multiplicity = %+v, want the declared %+v", got, declared)
	}
}
