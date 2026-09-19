package libs

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// bodyLibrary declares what only a declaration exposes: members, a declared
// value, a condition body, a stated [0..1] multiplicity, and abstractness.
const bodyLibrary = `package Lib {
	abstract part def Volume;
	part def Space :> Volume {
		part voids [0..*];
		attribute isSolid = isEmpty(voids);
		assert constraint { outerDimension >= innerDimension }
		attribute innerDimension [0..1];
		attribute outerDimension;
	}
	part def Room :> Space;
}`

// factRenderers renders each family of derived facts a record persists, keyed by
// the factRecord field that persists it. It is the equality test's comparator
// set, and TestEveryPersistedFactFamilyIsCompared holds it complete: a later
// slice that persists a new family without rendering it here fails that test
// rather than silently narrowing the proof.
var factRenderers = map[string]func(*symbols.LibraryFacts) string{
	"Supers": func(f *symbols.LibraryFacts) string { return fmt.Sprint(f.Supers) },
	"Unit": func(f *symbols.LibraryFacts) string {
		if f.Unit == nil {
			return "none"
		}
		var factors []string
		for _, factor := range f.Unit.Factors {
			factors = append(factors, fmt.Sprintf("%s^%g", factor.FQN, factor.Exponent))
		}
		sort.Strings(factors)
		return fmt.Sprintf("scale=%g/%g irreducible=%v factors=%v",
			f.Unit.ScaleNum, f.Unit.ScaleDen, f.Unit.Irreducible, factors)
	},
	"Abstract": func(f *symbols.LibraryFacts) string { return fmt.Sprint(f.Abstract) },
	"Dimension": func(f *symbols.LibraryFacts) string {
		if f.Dimension == nil {
			return "none"
		}
		var factors []string
		for _, factor := range f.Dimension.Factors {
			factors = append(factors, fmt.Sprintf("%s^%g", factor.FQN, factor.Exponent))
		}
		sort.Strings(factors)
		return fmt.Sprintf("factors=%v", factors)
	},
}

// A record is the only thing a cache hit contributes, so the equality proof has
// to cover every fact family it holds. Enumerating the record's fields makes the
// coverage a property of the format rather than of when the test was written.
func TestEveryPersistedFactFamilyIsCompared(t *testing.T) {
	rec := reflect.TypeFor[factRecord]()
	for i := range rec.NumField() {
		name := rec.Field(i).Name
		if name == "FQN" {
			continue // the key the facts are installed under, not a fact
		}
		if _, ok := factRenderers[name]; !ok {
			t.Errorf("factRecord.%s is persisted but no factRenderers entry compares it, "+
				"so a cache hit could differ from a miss in it unnoticed", name)
		}
	}
	for name := range factRenderers {
		if _, ok := rec.FieldByName(name); !ok {
			t.Errorf("factRenderers has an entry for %s, which factRecord no longer persists", name)
		}
	}
}

// The facts a load installs are what a cache hit replaces derivation with, so
// they must be identical whether they were derived or restored — over the
// bundled library every session loads, not only over a fixture.
func TestLibraryFactsAreEqualColdAndWarm(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  Source
	}{
		{"fixture", NewDirSource(writeLibrary(t, bodyLibrary))},
		{"bundled", DefaultSource()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cacheDir := t.TempDir()
			cold := factsSnapshot(t, loadSource(t, tc.src, cacheDir))
			warm := factsSnapshot(t, loadSource(t, tc.src, cacheDir))
			if len(cold) == 0 {
				t.Fatal("the cold load installed no facts, so the comparison proves nothing")
			}
			diffSnapshots(t, "cold", cold, "warm", warm)
		})
	}
}

// A cache is a performance choice, so a load with no cache at all must install
// the same facts a restored load does: disabling persistence is not a way to
// change the semantic state, and neither is enabling it.
func TestRestoredFactsEqualDerivedFacts(t *testing.T) {
	dir := writeLibrary(t, bodyLibrary)
	cacheDir := t.TempDir()

	loadWholeLibrary(t, dir, cacheDir) // cold: derives the facts and persists them
	restored := loadWholeLibrary(t, dir, cacheDir)
	derived := loadSource(t, NewDirSource(dir), "") // no cache: derives them again

	diffSnapshots(t, "derived", factsSnapshot(t, derived), "restored", factsSnapshot(t, restored))
	diffSnapshots(t, "derived", semanticSnapshot(t, derived), "restored", semanticSnapshot(t, restored))

	// A stale record is a miss, so an edited library derives afresh rather than
	// restoring facts that describe the file as it was.
	if err := os.WriteFile(filepath.Join(dir, "lib.sysml"),
		[]byte(strings.Replace(bodyLibrary, "part def Room :> Space;", "part def Room;", 1)), 0o600); err != nil {
		t.Fatalf("rewrite the library: %v", err)
	}
	edited := loadWholeLibrary(t, dir, cacheDir)
	room := lookupOne(t, edited, "Lib::Room")
	if room.Facts != nil && len(room.Facts.Supers) != 0 {
		t.Errorf("Lib::Room kept the supertype facts of the previous content: %v", room.Facts.Supers)
	}
}

// Members, declared values, condition bodies and stated multiplicities are what
// an index-only library used to drop. They now follow from the declaration, so
// every load path has them.
func TestLibraryBodiesArePresentOnEveryPath(t *testing.T) {
	dir := writeLibrary(t, bodyLibrary)
	cacheDir := t.TempDir()
	paths := []struct {
		name string
		idx  *symbols.Index
	}{
		{"cold", loadWholeLibrary(t, dir, cacheDir)},
		{"warm", loadWholeLibrary(t, dir, cacheDir)},
		{"no cache", loadSource(t, NewDirSource(dir), "")},
	}
	for _, path := range paths {
		t.Run(path.name, func(t *testing.T) {
			idx := path.idx
			space := lookupOne(t, idx, "Lib::Space")
			if !idx.Library(space) {
				t.Error("the loaded symbol is not marked library content")
			}
			if space.Decl == nil {
				t.Fatal("Lib::Space carries no declaration")
			}
			r := resolve.New(idx)
			m := semantics.NewModel(r)
			r.SetModel(m)

			// Members, inherited ones included.
			if got := memberNames(m, lookupOne(t, idx, "Lib::Room")); len(got) == 0 {
				t.Error("Lib::Room exposes no members, so the library body was dropped")
			}
			// A declared value, still an expression that resolves.
			isSolid := lookupOne(t, idx, "Lib::Space::isSolid")
			value := declaredValue(isSolid)
			if value == nil {
				t.Fatal("Lib::Space::isSolid carries no declared value")
			}
			if call, ok := value.(*ast.InvocationExpr); ok {
				if _, resolved := m.Eval(call); !resolved {
					t.Log("Eval declines to fold the library value expression, which is not a resolution failure")
				}
			}
			// A condition body.
			if conditionBodies(space) == 0 {
				t.Error("Lib::Space exposes no condition body")
			}
			// The declared multiplicity, not the assumed 1..1.
			inner := lookupOne(t, idx, "Lib::Space::innerDimension")
			got := m.EffectiveMultiplicityOf(inner)
			if got.Lower.Value != 0 || !got.Lower.Known || got.Upper.Value != 1 || !got.Upper.Known {
				t.Errorf("multiplicity of innerDimension = %v, want the declared 0..1", got)
			}
			// Abstractness, which a rule on a library metaclass reads (p24).
			if !symbols.IsAbstract(lookupOne(t, idx, "Lib::Volume")) {
				t.Error("Lib::Volume does not read as abstract")
			}
			if symbols.IsAbstract(space) {
				t.Error("Lib::Space reads as abstract, which it is not")
			}
		})
	}
}

// Abstractness of a bundled metaclass is what p24's rule judges. It is both
// persisted and declared, so every load path answers it and a restored answer
// is the derived one.
func TestAbstractnessOfBundledMetaclassesOnEveryPath(t *testing.T) {
	cacheDir := t.TempDir()
	cold := NewLoader(DefaultSource(), &Cache{dir: cacheDir})
	warm := NewLoader(DefaultSource(), &Cache{dir: cacheDir})
	paths := []struct {
		name   string
		loader *Loader
	}{
		{"cold", cold},
		{"warm", warm},
		{"no cache", NewLoader(DefaultSource(), nil)},
	}
	for _, path := range paths {
		t.Run(path.name, func(t *testing.T) {
			idx := symbols.NewIndex()
			if err := path.loader.LoadAll(idx); err != nil {
				t.Fatalf("load the library: %v", err)
			}
			meta := lookupOne(t, idx, "Metaobjects::Metaobject")
			if meta.Facts == nil || !meta.Facts.Abstract {
				t.Error("Metaobjects::Metaobject carries no abstractness fact")
			}
			if !symbols.IsAbstract(meta) {
				t.Error("Metaobjects::Metaobject does not read as abstract, so p24's rule cannot fire")
			}
			if symbols.IsAbstract(lookupOne(t, idx, "KerML::Comment")) {
				t.Error("KerML::Comment reads as abstract, which it is not")
			}
		})
	}
	if cold.Hits() != 0 {
		t.Errorf("the cold load hit the cache %d time(s), so it proves nothing", cold.Hits())
	}
	if warm.Hits() == 0 {
		t.Error("the warm load hit no record, so it did not exercise the restored path")
	}
}

// factsSnapshot renders every persisted fact family of every library symbol.
func factsSnapshot(t *testing.T, idx *symbols.Index) map[string]string {
	t.Helper()
	families := make([]string, 0, len(factRenderers))
	for name := range factRenderers {
		families = append(families, name)
	}
	sort.Strings(families)

	out := map[string]string{}
	for _, fqn := range idx.FQNs() {
		var descs []string
		for _, sym := range idx.LookupQualified(fqn) {
			var parts []string
			for _, family := range families {
				render := "unset"
				if sym.Facts != nil {
					render = factRenderers[family](sym.Facts)
				}
				parts = append(parts, family+"="+render)
			}
			descs = append(descs, strings.Join(parts, " "))
		}
		sort.Strings(descs)
		out[fqn] = strings.Join(descs, " | ")
	}
	return out
}

// semanticSnapshot renders what consumers ask of a library symbol: the answers
// that used to depend on cache warmth.
func semanticSnapshot(t *testing.T, idx *symbols.Index) map[string]string {
	t.Helper()
	r := resolve.New(idx)
	m := semantics.NewModel(r)
	r.SetModel(m)

	out := map[string]string{}
	for _, fqn := range idx.FQNs() {
		var descs []string
		for _, sym := range idx.LookupQualified(fqn) {
			descs = append(descs, describeSemantics(m, sym))
		}
		sort.Strings(descs)
		out[fqn] = strings.Join(descs, " | ")
	}
	return out
}

// describeSemantics renders one symbol's consumer-visible semantics: its
// multiplicity, members, annotations, conformance to its own supertypes, and the
// value of any expression it declares.
func describeSemantics(m *semantics.Model, sym *symbols.Symbol) string {
	var conforms []string
	for _, super := range m.DirectSupertypes(sym) {
		conforms = append(conforms, fmt.Sprintf("%s=%v", super.Name, m.Conforms(sym, super)))
	}
	sort.Strings(conforms)

	var annotations []string
	for _, a := range m.AnnotationFactsOf(sym) {
		annotations = append(annotations, a.TypeFQN)
	}
	sort.Strings(annotations)

	value := "none"
	if expr := declaredValue(sym); expr != nil {
		if v, ok := m.Eval(expr); ok {
			value = fmt.Sprintf("%+v", v)
		} else {
			value = "unfolded"
		}
	}
	return fmt.Sprintf("kind=%v abstract=%v mult=%v members=%v annotations=%v conforms=%v value=%s bodies=%d",
		sym.Kind, symbols.IsAbstract(sym), m.EffectiveMultiplicityOf(sym), memberNames(m, sym), annotations,
		conforms, value, conditionBodies(sym))
}

// memberNames names the members sym exposes, its supertypes' included.
func memberNames(m *semantics.Model, sym *symbols.Symbol) []string {
	var names []string
	for _, member := range m.MembersOfIncludingRedefined(sym) {
		names = append(names, member.Name)
	}
	sort.Strings(names)
	return names
}

// declaredValue is the expression sym's declaration assigns it, if any.
func declaredValue(sym *symbols.Symbol) ast.Node {
	if sym == nil {
		return nil
	}
	if u, ok := sym.Decl.(*ast.Usage); ok {
		return u.Value
	}
	return nil
}

// conditionBodies counts the conditions sym's declaration owns, directly or
// through a nested constraint — the `assert constraint { … }` bodies an
// index-only library dropped.
func conditionBodies(sym *symbols.Symbol) int {
	if sym == nil {
		return 0
	}
	count := 0
	for _, member := range declMembers(sym.Decl) {
		switch m := member.(type) {
		case *ast.ConstraintMember:
			if m.Expression != nil || len(m.Body) != 0 {
				count++
			}
		case *ast.Usage:
			if m.Kind == ast.UsageConstraint {
				count += conditionsIn(declMembers(m))
			}
		}
	}
	return count
}

// conditionsIn counts the conditions among a constraint's members.
func conditionsIn(members []ast.Node) int {
	count := 0
	for _, member := range members {
		if c, ok := member.(*ast.ConstraintMember); ok && (c.Expression != nil || len(c.Body) != 0) {
			count++
		}
	}
	return count
}

// declMembers are the members a declaration owns, unwrapped from the memberships
// that hold them.
func declMembers(decl ast.Node) []ast.Node {
	var members []ast.Node
	switch d := decl.(type) {
	case *ast.Usage:
		members = d.Members
	case *ast.Definition:
		members = d.Members
	}
	out := make([]ast.Node, 0, len(members))
	for _, member := range members {
		if m, ok := member.(*ast.Membership); ok {
			member = m.Member
		}
		out = append(out, member)
	}
	return out
}

// diffSnapshots reports every name the two renderings disagree on.
func diffSnapshots(t *testing.T, wantPath string, want map[string]string, gotPath string, got map[string]string) {
	t.Helper()
	for fqn, w := range want {
		g, ok := got[fqn]
		if !ok {
			t.Errorf("%s is registered by the %s load only", fqn, wantPath)
			continue
		}
		if g != w {
			t.Errorf("%s:\n  %s: %s\n  %s: %s", fqn, wantPath, w, gotPath, g)
		}
	}
	for fqn := range got {
		if _, ok := want[fqn]; !ok {
			t.Errorf("%s is registered by the %s load only", fqn, gotPath)
		}
	}
}

// lookupOne returns the single symbol fqn names.
func lookupOne(t *testing.T, idx *symbols.Index, fqn string) *symbols.Symbol {
	t.Helper()
	syms := idx.LookupQualified(fqn)
	if len(syms) != 1 {
		t.Fatalf("%s names %d symbols, want 1", fqn, len(syms))
	}
	return syms[0]
}

// loadSource loads src into a fresh index, with a cache in cacheDir or none when
// cacheDir is empty.
func loadSource(t *testing.T, src Source, cacheDir string) *symbols.Index {
	t.Helper()
	var cache *Cache
	if cacheDir != "" {
		cache = &Cache{dir: cacheDir}
	}
	idx := symbols.NewIndex()
	if err := NewLoader(src, cache).LoadAll(idx); err != nil {
		t.Fatalf("load the library: %v", err)
	}
	return idx
}

// writeLibrary writes src as the only file of a fresh library directory.
func writeLibrary(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lib.sysml"), []byte(src), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	return dir
}
