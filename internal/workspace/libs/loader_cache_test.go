package libs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// countingSource wraps a Source and counts how often each file is read.
type countingSource struct {
	inner Source
	reads map[string]int
}

func (c *countingSource) List() []string { return c.inner.List() }
func (c *countingSource) Read(name string) ([]byte, error) {
	if c.reads == nil {
		c.reads = map[string]int{}
	}
	c.reads[name]++
	return c.inner.Read(name)
}

func TestLoaderCacheMissThenHit(t *testing.T) {
	cacheDir := t.TempDir()
	cache := &Cache{dir: cacheDir}
	cs := &countingSource{inner: DefaultSource()}
	ld := NewLoader(cs, cache)

	idx1 := symbols.NewIndex()
	if err := ld.load("Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml", idx1); err != nil {
		t.Fatalf("first Load: %v", err)
	}
	if len(ld.loaded) != 1 || ld.loaded[0].rec != nil {
		t.Fatalf("documents loaded on a cold cache = %+v, want one with no record", ld.loaded)
	}
	// The key covers the whole library set, so every file of it was digested.
	if len(cs.reads) != len(cs.List()) {
		t.Fatalf("files read = %d, want the whole set of %d", len(cs.reads), len(cs.List()))
	}
	if len(idx1.LookupQualified("ScalarValues::Boolean")) != 1 {
		t.Fatal("first load did not index ScalarValues::Boolean")
	}
	ld.installFacts(idx1, true)
	entries, _ := os.ReadDir(cacheDir)
	found := false
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".idx" {
			found = true
		}
	}
	if !found {
		t.Fatal("no .idx file written after cache miss")
	}

	idx2 := symbols.NewIndex()
	if err := ld.load("Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml", idx2); err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if len(ld.loaded) != 1 || ld.loaded[0].rec == nil {
		t.Fatalf("documents loaded on a warm cache = %+v, want one with its record", ld.loaded)
	}
	if len(idx2.LookupQualified("ScalarValues")) != 1 ||
		len(idx2.LookupQualified("ScalarValues::Boolean")) != 1 {
		t.Fatal("cached load did not repopulate index")
	}
	ld.installFacts(idx2, false)

	// A hit restores the derived supertype edges onto the parsed symbol, which
	// keeps its declaration either way.
	boolean := idx2.LookupQualified("ScalarValues::Boolean")[0]
	if boolean.Decl == nil {
		t.Fatal("the cached load left ScalarValues::Boolean without its declaration")
	}
	if boolean.Facts == nil || len(boolean.Facts.Supers) != 1 ||
		boolean.Facts.Supers[0].FQN != "ScalarValues::ScalarValue" || len(boolean.Facts.Supers[0].Path) != 0 {
		t.Fatalf("restored supertype facts of Boolean = %+v, want [ScalarValues::ScalarValue]", boolean.Facts)
	}
}

// Restored supertype facts must match what the live-parsed AST yields, so the
// typing edge of a feature survives the round trip too.
func TestLoaderCacheKeepsTypingEdge(t *testing.T) {
	dir := t.TempDir()
	src := "package Lib { part def Engine; part e : Engine; }"
	if err := os.WriteFile(filepath.Join(dir, "lib.sysml"), []byte(src), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	ld := NewLoader(NewDirSource(dir), &Cache{dir: t.TempDir()})

	idx1 := symbols.NewIndex()
	if err := ld.load("lib.sysml", idx1); err != nil {
		t.Fatalf("first Load: %v", err)
	}
	ld.installFacts(idx1, true)

	idx2 := symbols.NewIndex()
	if err := ld.load("lib.sysml", idx2); err != nil {
		t.Fatalf("second Load: %v", err)
	}
	ld.installFacts(idx2, false)
	e := idx2.LookupQualified("Lib::e")
	if len(e) != 1 {
		t.Fatalf("cached load did not register Lib::e")
	}
	if e[0].Facts == nil || len(e[0].Facts.Supers) != 1 || e[0].Facts.Supers[0].FQN != "Lib::Engine" {
		t.Fatalf("restored supertype facts of e = %+v, want [Lib::Engine]", e[0].Facts)
	}
}

func TestLoaderCachePreservesRedefinitionMemberEdges(t *testing.T) {
	dir := t.TempDir()
	src := `package P {
		classifier A { feature f { feature g; } }
		classifier B specializes A { feature redefines f { feature h; } }
	}`
	if err := os.WriteFile(filepath.Join(dir, "lib.kerml"), []byte(src), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	cacheDir := t.TempDir()
	loadWholeLibrary(t, dir, cacheDir)
	idx := loadWholeLibrary(t, dir, cacheDir)
	r := resolve.New(idx)
	m := semantics.NewModel(r)
	b := idx.LookupQualified("P::B")
	if len(b) != 1 {
		t.Fatal("cached B not found")
	}
	f, ok := m.LookupMember(b[0], "f")
	if !ok {
		t.Fatal("cached B does not expose f")
	}
	if _, ok := m.LookupMember(f, "g"); !ok {
		t.Fatal("cached redefined f does not expose inherited g")
	}
}

func TestLibraryBehaviorParametersRedefinedByPosition(t *testing.T) {
	const behaviorSource = `package ScalarFunctions {
		calc '+' {
			in x;
			in y;
			return result;
		}
	}`
	const modelSource = `package P {
		calc c : ScalarFunctions::'+' {
			in x : ScalarValues::Real;
			in y : ScalarValues::Real;
			return r : ScalarValues::Real;
		}
	}`
	assertParameters := func(t *testing.T, idx *symbols.Index) {
		t.Helper()
		doc := parser.New(source.New("model.sysml", []byte(modelSource))).ParseFile()
		idx.AddDocument("model.sysml", doc)
		r := resolve.New(idx)
		m := semantics.NewModel(r)
		r.SetModel(m)
		candidates := idx.LookupQualified("P::c")
		if len(candidates) != 1 {
			t.Fatalf("P::c symbols = %d, want 1", len(candidates))
		}
		c := candidates[0]
		params := m.BehaviorParametersOf(c)
		if len(params) != 3 {
			t.Fatalf("BehaviorParametersOf(c) = %d, want 3", len(params))
		}
		want := []string{"x", "y", "result"}
		owned := []string{"x", "y", "r"}
		for i, param := range params {
			if got := param.Symbol.Name; got != owned[i] {
				t.Errorf("parameter %d = %q, want %s", i, got, owned[i])
			}
		}
		for i, name := range []string{"x", "y", "r"} {
			member, ok := c.Scope.LookupLocal(name)
			if !ok {
				t.Fatalf("missing c.%s", name)
			}
			targets := m.ImplicitParameterRedefinitions(member)
			if len(targets) != 1 {
				t.Errorf("implicit targets of c.%s = %v, want one target named %s", name, targets, want[i])
			} else if i < 2 && targets[0].Name != want[i] {
				t.Errorf("implicit target of c.%s = %q (%s), want %s", name, targets[0].Name, symbols.FQNOf(targets[0]), want[i])
			} else if i == 2 {
				result, ok := targets[0].Decl.(*ast.Usage)
				if !ok || !result.IsResult {
					t.Errorf("implicit target of c.%s = %q (%s), want the general result parameter", name, targets[0].Name, symbols.FQNOf(targets[0]))
				}
			}
		}
	}

	t.Run("cold load", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "library.sysml"), []byte(behaviorSource), 0o600); err != nil {
			t.Fatal(err)
		}
		idx := loadWholeLibrary(t, dir, t.TempDir())
		assertParameters(t, idx)
	})
	t.Run("warm cache", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "library.sysml"), []byte(behaviorSource), 0o600); err != nil {
			t.Fatal(err)
		}
		cacheDir := t.TempDir()
		loadWholeLibrary(t, dir, cacheDir)
		idx := loadWholeLibrary(t, dir, cacheDir)
		assertParameters(t, idx)
	})
	t.Run("snapshot", func(t *testing.T) {
		assertParameters(t, NewModelIndex())
	})
}

// A record whose supertypes are not all reachable yet must not be cached when
// the loader requires resolution: its key does not describe the index it was
// built in, so it would be restored — minus that edge — where the target exists.
func TestLoaderRequireResolvedSkipsUnresolvedRecord(t *testing.T) {
	dir := t.TempDir()
	// Specializes ScalarValues::Real, which this directory does not declare.
	src := filepath.Join(dir, "lib.sysml")
	if err := os.WriteFile(src, []byte("package Lib { attribute def Mass :> ScalarValues::Real; }"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	cacheDir := t.TempDir()
	ld := NewLoader(NewDirSource(dir), &Cache{dir: cacheDir})
	ld.RequireResolved = true

	idx := symbols.NewIndex()
	if err := ld.load("lib.sysml", idx); err != nil {
		t.Fatalf("Load: %v", err)
	}
	ld.installFacts(idx, true)

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("read cache dir: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".idx" {
			t.Fatalf("cached %s despite the unresolved supertype ScalarValues::Real", e.Name())
		}
	}
}

// equivalenceLibrary is a library exercising every piece of index state a load
// path has to produce identically: a chain of wildcard imports across sibling
// packages, a short name, an alias, and a specialization across files.
// It also carries both element-filter forms over an annotated element, whose
// verdict must not depend on which path loaded the library.
const equivalenceLibrary = `package Lib {
	public import Core::*;
	package Root {
		part def Element;
		attribute def <kg> Kilogram;
	}
	package Core {
		public import Root::*;
		part def Type :> Element;
	}
	alias Elem for Root::Element;

	package Meta {
		metadata def Safety {
			attribute level;
		}
	}
	package Annotated {
		#Meta::Safety part def Belt;
		part def Bolt;
	}
	package SafeImport {
		public import Annotated::*[@Meta::Safety];
	}
	package SafeReexport {
		public import Annotated::*;
		filter @Meta::Safety;
	}
}`

// A persistent cache is a performance optimisation, so a load that derives a
// library's facts and a load that restores them must leave the resolver looking
// at the same thing: the same fully-qualified names, and the same supertypes,
// wildcard imports and alias targets under each of them.
func TestParsedAndRestoredIndexesAreEquivalent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lib.sysml"), []byte(equivalenceLibrary), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	cacheDir := t.TempDir()

	parsed := loadWholeLibrary(t, dir, cacheDir)   // cache miss: parses, then persists
	restored := loadWholeLibrary(t, dir, cacheDir) // cache hit: restores the record

	// A pair of equally empty indexes would compare equal, so pin the content
	// that only the full import chain Lib -> Core -> Root can produce.
	for _, fqn := range []string{"Lib::Root::Element", "Lib::Core::Element", "Lib::Element", "Lib::kg"} {
		if len(parsed.LookupQualified(fqn)) != 1 {
			t.Fatalf("the parsed index does not register %s", fqn)
		}
	}

	want := snapshotIndex(parsed)
	got := snapshotIndex(restored)
	for _, fqn := range want.fqns {
		if got.entries[fqn] != want.entries[fqn] {
			t.Errorf("%s:\n  parsed:   %s\n  restored: %s", fqn, want.entries[fqn], got.entries[fqn])
		}
	}
	for _, fqn := range got.fqns {
		if _, ok := want.entries[fqn]; !ok {
			t.Errorf("%s is registered only after a cache restore: %s", fqn, got.entries[fqn])
		}
	}
}

// The elements a filtered import and a filtered re-export admit must be the same
// whether the library's facts were derived or restored from its record.
func TestFilteredImportsSurviveCacheRestore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lib.sysml"), []byte(equivalenceLibrary), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	cacheDir := t.TempDir()

	parsed := admittedNames(t, loadWholeLibrary(t, dir, cacheDir))   // cache miss: parses
	restored := admittedNames(t, loadWholeLibrary(t, dir, cacheDir)) // cache hit: restores

	want := map[string]bool{
		// The annotated element passes both filter forms; the unannotated one
		// passes neither, and an unfiltered route admits both.
		"Lib::SafeImport::Belt":   true,
		"Lib::SafeImport::Bolt":   false,
		"Lib::SafeReexport::Belt": true,
		"Lib::SafeReexport::Bolt": false,
		"Lib::Annotated::Belt":    true,
		"Lib::Annotated::Bolt":    true,
	}
	for name, admit := range want {
		if parsed[name] != admit {
			t.Errorf("%s admitted=%v in the parsed library, want %v", name, parsed[name], admit)
		}
		if restored[name] != parsed[name] {
			t.Errorf("%s admitted=%v after a cache restore, but %v when parsed", name, restored[name], parsed[name])
		}
	}
}

// admittedNames reports, for each name the library registers the annotated
// elements under, whether the filters gating that name admit the element there.
// It is the question resolution asks of a filter, asked directly of the index and
// the semantic model.
func admittedNames(t *testing.T, idx *symbols.Index) map[string]bool {
	t.Helper()
	r := resolve.New(idx)
	m := semantics.NewModel(r)
	r.SetModel(m)

	out := map[string]bool{}
	for _, prefix := range []string{"Lib::Annotated", "Lib::SafeImport", "Lib::SafeReexport"} {
		for _, leaf := range []string{"Belt", "Bolt"} {
			fqn := prefix + "::" + leaf
			syms := idx.LookupQualified(fqn)
			if len(syms) != 1 {
				t.Fatalf("%s names %d symbols, want 1", fqn, len(syms))
			}
			routes := idx.ReexportGates("", fqn, syms[0], "")
			admitted := len(routes) == 0 // declared here, or surfaced unconditionally
			for _, route := range routes {
				passes := true
				for _, gate := range route {
					if !m.SatisfiesElementFilter(gate, syms[0]) {
						passes = false
						break
					}
				}
				if passes {
					admitted = true
				}
			}
			out[fqn] = admitted
		}
	}
	return out
}

// loadWholeLibrary indexes every file of the library in dir through a loader
// backed by cacheDir, expanding imports and persisting records exactly as
// model.loadStdlib does.
func loadWholeLibrary(t *testing.T, dir, cacheDir string) *symbols.Index {
	t.Helper()
	ld := NewLoader(NewDirSource(dir), &Cache{dir: cacheDir})
	idx := symbols.NewIndex()
	if err := ld.LoadAll(idx); err != nil {
		t.Fatalf("load the library: %v", err)
	}
	return idx
}

// indexView is a description of an index that does not depend on how it was
// populated: every registered fully-qualified name, and the resolver-visible
// state of the symbols under it.
type indexView struct {
	fqns    []string
	entries map[string]string
}

func snapshotIndex(idx *symbols.Index) indexView {
	r := resolve.New(idx)
	model := semantics.NewModel(r)
	view := indexView{fqns: idx.FQNs(), entries: map[string]string{}}
	for _, fqn := range view.fqns {
		var descs []string
		for _, sym := range idx.LookupQualified(fqn) {
			descs = append(descs, describeSymbol(sym, idx, model))
		}
		sort.Strings(descs)
		view.entries[fqn] = fmt.Sprintf("%v imports=%v", descs, idx.WildcardImportsOf(fqn))
	}
	return view
}

// describeSymbol renders the state a symbol contributes to name resolution,
// which its declaration states on every load path.
func describeSymbol(sym *symbols.Symbol, idx *symbols.Index, model *semantics.Model) string {
	refs, _ := supersOf(sym, idx, model)
	var supers []string
	for _, ref := range refs {
		supers = append(supers, refString(ref))
	}
	sort.Strings(supers)
	return fmt.Sprintf("kind=%v short=%q supers=%v alias=%q name=%q",
		sym.Kind, sym.ShortName, supers, aliasTargetOf(sym.Decl), sym.Name)
}

// aliasTargetOf names the target an alias declaration points at, "" for anything
// that is not an alias.
func aliasTargetOf(decl ast.Node) string {
	a, ok := decl.(*ast.Alias)
	if !ok || a.For == nil {
		return ""
	}
	return a.For.Text()
}

// A record persists facts derived from sibling files — a unit's scale follows a
// prefix or reference unit declared elsewhere — so editing any file of a library
// must invalidate every record of that library, not just the edited file's.
func TestLoaderCacheInvalidatesSiblingRecords(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.sysml"), []byte("package A { part def Engine; }"), 0o600); err != nil {
		t.Fatalf("write a.sysml: %v", err)
	}
	sibling := filepath.Join(dir, "b.sysml")
	if err := os.WriteFile(sibling, []byte("package B { part e : A::Engine; }"), 0o600); err != nil {
		t.Fatalf("write b.sysml: %v", err)
	}
	cacheDir := t.TempDir()
	loadWholeLibrary(t, dir, cacheDir) // cold: derives both, then persists them

	if missed := loadLibraryMissCount(t, dir, cacheDir); missed != 0 {
		t.Fatalf("records missing for an unchanged library = %d, want 0", missed)
	}

	if err := os.WriteFile(sibling, []byte("package B { part def Wheel; }"), 0o600); err != nil {
		t.Fatalf("rewrite b.sysml: %v", err)
	}
	if missed := loadLibraryMissCount(t, dir, cacheDir); missed != 2 {
		t.Fatalf("records missing after editing one file = %d, want 2 (the whole library)", missed)
	}

	// The records of the library as it was are now unreachable, so the next
	// persist prunes them once they have gone unused for long enough.
	stale := time.Now().Add(-maxIdleAge - time.Hour)
	for _, name := range idxFiles(t, cacheDir) {
		if err := os.Chtimes(filepath.Join(cacheDir, name), stale, stale); err != nil {
			t.Fatalf("backdate %s: %v", name, err)
		}
	}
	loadWholeLibrary(t, dir, cacheDir)
	if files := idxFiles(t, cacheDir); len(files) != 2 {
		t.Fatalf("records in the cache = %d, want the 2 of the current library: %v", len(files), files)
	}
}

// idxFiles lists the record files in a cache directory.
func idxFiles(t *testing.T, cacheDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("read cache dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".idx" {
			names = append(names, e.Name())
		}
	}
	return names
}

// loadLibraryMissCount indexes the library in dir and reports how many of its
// files found no record in the cache, so their facts have to be derived.
func loadLibraryMissCount(t *testing.T, dir, cacheDir string) int {
	t.Helper()
	src := NewDirSource(dir)
	ld := NewLoader(src, &Cache{dir: cacheDir})
	idx := symbols.NewIndex()
	for _, name := range src.List() {
		if err := ld.load(name, idx); err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
	}
	missed := 0
	for _, doc := range ld.loaded {
		if doc.rec == nil {
			missed++
		}
	}
	return missed
}

// A library document is removable whichever path loaded it, which is what lets a
// workspace drop a library it no longer uses.
func TestLibraryDocumentIsRemovable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lib.kerml"), []byte("package P { namespace N; }"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	cacheDir := t.TempDir()
	for _, path := range []string{"cold", "warm"} {
		idx := loadWholeLibrary(t, dir, cacheDir)
		if len(idx.LookupQualified("P::N")) != 1 {
			t.Fatalf("the %s load did not register P::N", path)
		}
		idx.RemoveDocument("lib.kerml")
		if len(idx.LookupQualified("P::N")) != 0 {
			t.Fatalf("RemoveDocument left P::N after the %s load", path)
		}
	}
}
