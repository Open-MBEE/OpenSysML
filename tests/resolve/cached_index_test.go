package resolve_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A library symbol carries its declaration on either load path, so every rule
// reading a library element is asked about this library the same way.
const cachedLibrary = `standard library package Cycles {
	attribute def Mass;
	part def Cycle {
		attribute weight : Mass;
		part wheel : Wheel;
	}
	part def Wheel {
		attribute spokes : Mass;
	}
	part def Tandem :> Cycle;
	alias Bike for Cycle;
	part def AliasedCopy :> Bike;
	private part def Frame;
	alias HiddenFrame for Frame;
}`

// cachedUser reaches into the library: a typing, a specialization, a redefinition,
// a feature chain through a library type and an alias.
const cachedUser = `package App {
	private import Cycles::*;
	part def Racer :> Cycle {
		attribute :>> weight = 9;
		attribute chainMass = wheel.spokes;
	}
	part def Copy :> Bike;
	part racer : Racer;
}`

// What resolution answers about a library element must not depend on the cache:
// inherited features, redefinitions, chains, aliases.
func TestResolutionOfALibraryIsTheSameColdAndWarm(t *testing.T) {
	dir, cacheDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cycles.sysml"), []byte(cachedLibrary), 0o600); err != nil {
		t.Fatalf("write library: %v", err)
	}

	coldIdx := loadLibrary(t, dir, cacheDir) // cache miss: parses, then reduces
	warmIdx := loadLibrary(t, dir, cacheDir) // cache hit: restores
	// A library is parsed either way, so both answer from declarations.
	for path, idx := range map[string]*symbols.Index{"cold": coldIdx, "warm": warmIdx} {
		if sym := libSymbol(t, idx, "Cycles::Cycle"); sym.Decl == nil {
			t.Fatalf("the %s load leaves the library without its declarations", path)
		}
	}

	coldAnswers := resolveAgainstLibrary(t, coldIdx)
	warmAnswers := resolveAgainstLibrary(t, warmIdx)
	for question, want := range coldAnswers {
		// Every question has an answer, so agreeing on "unresolved" is no pass.
		if want == "unresolved" || want == "false" {
			t.Errorf("%s = %q on the cold path, which is not an answer", question, want)
		}
		if got := warmAnswers[question]; got != want {
			t.Errorf("%s = %q warm, %q cold", question, got, want)
		}
	}
}

// resolveAgainstLibrary resolves cachedUser over idx and reports, for each
// question a consumer asks of the library, the fully-qualified name it answers.
func resolveAgainstLibrary(t *testing.T, idx *symbols.Index) map[string]string {
	t.Helper()
	const name = "app.sysml"
	p := parser.New(source.New(name, []byte(cachedUser)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	m := semantics.NewModel(r)
	r.SetModel(m)
	r.ResolveDocument(name, root)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("the document must resolve against the library: %v", r.Diagnostics)
	}

	racer := libSymbol(t, idx, "App::Racer")
	out := map[string]string{}
	fqnOf := func(sym *symbols.Symbol, ok bool) string {
		if !ok || sym == nil {
			return "unresolved"
		}
		if fqn := idx.GetFQN(sym); fqn != "" {
			return fqn
		}
		return sym.Name
	}

	// An inherited feature, and the one a redefinition in the document names.
	out["Racer inherits weight"] = fqnOf(m.LookupContributedMember(racer, "weight"))
	out["Racer.wheel"] = fqnOf(m.LookupMember(racer, "wheel"))
	// A chain through a library type: `wheel.spokes` names a member of Wheel.
	if wheel, ok := m.LookupMember(racer, "wheel"); ok {
		out["Racer.wheel.spokes"] = fqnOf(m.LookupMember(typeOf(t, m, wheel), "spokes"))
	}
	// An alias of a library element, and one whose target the library keeps
	// private: reachable through the alias, not by a qualified reference.
	out["Cycles::Bike"] = fqnOf(r.ResolveAliasTarget(libSymbol(t, idx, "Cycles::Bike")))
	out["Cycles::HiddenFrame"] = fqnOf(r.ResolveAliasTarget(libSymbol(t, idx, "Cycles::HiddenFrame")))
	out["App::Copy super"] = fqnOf(firstSuper(m, libSymbol(t, idx, "App::Copy")))
	out["Cycles::AliasedCopy super"] = fqnOf(firstSuper(m, libSymbol(t, idx, "Cycles::AliasedCopy")))
	// Conformance across a library specialization, read from the recorded
	// supertype facts.
	out["Tandem conforms to Cycle"] = boolText(m.Conforms(
		libSymbol(t, idx, "Cycles::Tandem"), libSymbol(t, idx, "Cycles::Cycle")))
	out["Racer conforms to Cycle"] = boolText(m.Conforms(racer, libSymbol(t, idx, "Cycles::Cycle")))
	return out
}

// loadLibrary indexes every file of the library in dir through a loader backed
// by cacheDir, persisting its records exactly as the stdlib load does.
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

// libSymbol is the single symbol idx registers under fqn.
func libSymbol(t *testing.T, idx *symbols.Index, fqn string) *symbols.Symbol {
	t.Helper()
	syms := idx.LookupQualified(fqn)
	if len(syms) != 1 {
		t.Fatalf("%s names %d symbols, want 1", fqn, len(syms))
	}
	return syms[0]
}

// typeOf is the type a usage is typed by, the owner of the members a chain
// segment after it names.
func typeOf(t *testing.T, m *semantics.Model, sym *symbols.Symbol) *symbols.Symbol {
	t.Helper()
	supers := m.DirectSupertypes(sym)
	if len(supers) == 0 {
		t.Fatalf("%s has no type", sym.Name)
	}
	return supers[0]
}

// firstSuper is the first supertype of sym, or unresolved when it has none.
func firstSuper(m *semantics.Model, sym *symbols.Symbol) (*symbols.Symbol, bool) {
	supers := m.DirectSupertypes(sym)
	if len(supers) == 0 {
		return nil, false
	}
	return supers[0], true
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
