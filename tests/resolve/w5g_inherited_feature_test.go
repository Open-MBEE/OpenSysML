package resolve_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// w5gLibrary exercises the shapes the inherited-feature walk must follow
// through library symbols: a multi-hop specialization chain, a specialization
// cycle, and a `featured by` edge.
const w5gLibrary = `standard library package Rigs {
	classifier Base {
		feature anchor;
	}
	classifier Mid specializes Base;
	classifier Rig specializes Mid;
	classifier Loop1 specializes Loop2 {
		feature strand;
	}
	classifier Loop2 specializes Loop1;
	classifier Host {
		feature slot;
	}
	feature widget featured by Host;
}`

// resolveW5GUser parses src over idx and returns the resolver's diagnostics.
func resolveW5GUser(t *testing.T, idx *symbols.Index, src string) []string {
	t.Helper()
	const name = "user.kerml"
	p := parser.New(source.New(name, []byte(src)))
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
	var msgs []string
	for _, d := range r.Diagnostics {
		msgs = append(msgs, d.Message)
	}
	return msgs
}

// w5gLibraryIndexes loads w5gLibrary twice against one cache: the first load
// derives its facts, the second restores the persisted ones.
func w5gLibraryIndexes(t *testing.T) (cold, warm *symbols.Index) {
	t.Helper()
	dir, cacheDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rigs.kerml"), []byte(w5gLibrary), 0o600); err != nil {
		t.Fatalf("write library: %v", err)
	}
	cold = loadLibrary(t, dir, cacheDir)
	warm = loadLibrary(t, dir, cacheDir)
	for path, idx := range map[string]*symbols.Index{"cold": cold, "warm": warm} {
		if sym := libSymbol(t, idx, "Rigs::Rig"); sym.Decl == nil {
			t.Fatalf("the %s load leaves the library without its declarations", path)
		}
	}
	return cold, warm
}

// A redefinition whose target sits several specialization hops up a recorded
// chain resolves the same cold and warm.
func TestW5GRedefinitionThroughRecordedChain(t *testing.T) {
	const user = `package App {
	private import Rigs::*;
	classifier Custom specializes Rig {
		feature redefines anchor;
	}
}`
	cold, warm := w5gLibraryIndexes(t)
	if msgs := resolveW5GUser(t, cold, user); len(msgs) != 0 {
		t.Fatalf("a cold library reports %v, want clean", msgs)
	}
	if msgs := resolveW5GUser(t, warm, user); len(msgs) != 0 {
		t.Fatalf("a warm library reports %v, cold was clean", msgs)
	}
}

// A specialization cycle in the library terminates the walk in both cache
// states: a feature on the cycle resolves, a missing one reports cleanly.
func TestW5GWalkIsCycleSafe(t *testing.T) {
	const resolves = `package App {
	private import Rigs::*;
	classifier Cyclic specializes Loop2 {
		feature redefines strand;
	}
}`
	const missing = `package App {
	private import Rigs::*;
	classifier Cyclic specializes Loop2 {
		feature redefines notThere;
	}
}`
	cold, warm := w5gLibraryIndexes(t)
	for _, idx := range []*symbols.Index{cold, warm} {
		if msgs := resolveW5GUser(t, idx, resolves); len(msgs) != 0 {
			t.Errorf("a feature on the cycle reports %v, want clean", msgs)
		}
		if msgs := resolveW5GUser(t, idx, missing); len(msgs) == 0 {
			t.Error("a feature absent from the cycle must stay unresolved")
		}
	}
}

// A `featured by` edge contributes inherited features identically cold and warm.
func TestW5GFeaturedByEdgeSurvivesRestore(t *testing.T) {
	const user = `package App {
	private import Rigs::*;
	feature gadget specializes widget {
		feature redefines slot;
	}
}`
	cold, warm := w5gLibraryIndexes(t)
	if msgs := resolveW5GUser(t, cold, user); len(msgs) != 0 {
		t.Fatalf("a cold library reports %v, want clean", msgs)
	}
	if msgs := resolveW5GUser(t, warm, user); len(msgs) != 0 {
		t.Fatalf("a warm library reports %v, cold was clean", msgs)
	}
}
